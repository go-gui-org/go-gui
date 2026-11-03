//go:build !wasm

// The whole keypress runs against the real shaper, whose construction
// needs a DOM (syscall/js) the node-based CI harness does not provide
// — same exclusion as the other text-system tests in this package.

package soft

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The legacy benchmark below keeps its pre-chunking cases for historical
// comparison. The large-edit benchmark measures the current chunked path.
//
// Each iteration re-seeds the same text so size stays fixed (typing
// "x" into the seed, then rebuilding the view from the seed).
// keypressWindow returns a window holding a focused Input seeded
// with text, measured by the real shaper, plus a reseed func that
// rebuilds the view from the seed. Each iteration re-seeds so the
// size stays fixed while typing "x" into it.
func keypressWindow(
	tb testing.TB, seed string,
) (*gui.Window, func()) {
	tb.Helper()
	w := gui.NewTestWindow(tb, gui.WindowCfg{Width: 800, Height: 600})
	tm, err := prepare(w, 1)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { tm.textSys.Free() })
	w.SetTextMeasurer(tm)
	reseed := func() {
		w.SetView(func(w *gui.Window) gui.View {
			return gui.Input(gui.InputCfg{
				ID:     "f",
				Text:   seed,
				Sizing: gui.FillFit,
			})
		})
	}
	reseed()
	w.Update()
	w.SetFocus("f")
	w.Update()
	return w, reseed
}

func benchmarkInputKeypress(b *testing.B, seed string) {
	b.Helper()
	w, reseed := keypressWindow(b, seed)
	x := gui.Event{Type: gui.EventChar, CharCode: 'x'}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		reseed()
		e := x // dispatch consumes an Event; every iteration needs a fresh one
		w.EventFn(&e)
		w.Update()
	}
}

func BenchmarkInputKeypress(b *testing.B) {
	scripts := []struct {
		name     string
		unit     string
		unitSize int
	}{
		{"ascii", "a", 1},
		{"cjk", "中", 3},
		{"emoji", "🎉", 4},
	}
	for _, byteBudget := range []int{1_024, 5_120, 10_000} {
		for _, sc := range scripts {
			name := fmt.Sprintf("%s_%dB", sc.name, byteBudget)
			b.Run(name, func(b *testing.B) {
				benchmarkInputKeypress(b, strings.Repeat(
					sc.unit, byteBudget/sc.unitSize))
			})
		}
	}
}

// editPairWindow holds a real-shaper Input whose OnTextChanged callback is
// the application's source of truth. The benchmark alternates insert and
// backspace, so every measured edit sees the same text size (plus or minus
// one byte) without rebuilding the view from stale text between iterations.
func editPairWindow(
	tb testing.TB, seed string, atEnd bool,
) (*gui.Window, *gui.Event, *gui.Event, *string) {
	tb.Helper()
	w := gui.NewTestWindow(tb, gui.WindowCfg{Width: 800, Height: 600})
	tm, err := prepare(w, 1)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { tm.textSys.Free() })
	w.SetTextMeasurer(tm)
	text := seed
	w.SetView(func(*gui.Window) gui.View {
		return gui.Input(gui.InputCfg{
			ID: "f", Text: text, Sizing: gui.FillFit,
			MaxLength:     2 << 20,
			OnTextChanged: func(s string, _ gui.EventCtx) { text = s },
		})
	})
	w.Update()
	w.SetFocus("f")
	w.Update()
	if atEnd {
		if err := w.TestKey("f", gui.KeyEnd, gui.ModNone); err != nil {
			tb.Fatal(err)
		}
	}
	insert := &gui.Event{Type: gui.EventChar, CharCode: 'x'}
	backspace := &gui.Event{Type: gui.EventKeyDown, KeyCode: gui.KeyBackspace}
	return w, insert, backspace, &text
}

func benchmarkInputEditPair(
	b *testing.B, seed string, atEnd bool,
) {
	b.Helper()
	w, insert, backspace, text := editPairWindow(b, seed, atEnd)
	// Warm the layout and shaper scratch in both directions. Without this,
	// a short fixed-iteration run charges whichever case first grows a
	// scratch buffer and identical benchmark repetitions report different
	// allocation counts.
	e := *insert
	w.EventFn(&e)
	w.Update()
	e = *backspace
	w.EventFn(&e)
	w.Update()
	if *text != seed {
		b.Fatalf("warm edit pair left %d bytes, want %d", len(*text), len(seed))
	}
	b.ReportAllocs()
	b.ReportMetric(float64(len(seed)), "seed-B")
	b.ResetTimer()
	add := true
	for b.Loop() {
		if add {
			e = *insert
		} else {
			e = *backspace
		}
		w.EventFn(&e)
		w.Update()
		add = !add
	}
	b.StopTimer()
	want := len(seed)
	if !add {
		want++
	}
	if len(*text) != want {
		b.Fatalf("measured edits left %d bytes, want %d", len(*text), want)
	}
}

// BenchmarkInputLargeEdit is the steady-state whole-widget cost at the sizes
// the chunked layout is intended to unlock. Start and end are separate: an
// edit at the start reopens the long paragraph after it, while an end edit can
// preserve all preceding chunks. The mixed case keeps bidi shaping visible in
// the performance data without multiplying the 1 MiB setup cost.
func BenchmarkInputLargeEdit(b *testing.B) {
	cases := []struct {
		name string
		text string
	}{
		{"ascii_10KB", strings.Repeat("a", 10_000)},
		{"ascii_100KB", strings.Repeat("a", 100_000)},
		{"ascii_1MiB", strings.Repeat("a", 1<<20)},
		{"mixed_bidi_100KB", strings.Repeat("latin \u05d0\u05d1\u05d2 ",
			100_000/len("latin \u05d0\u05d1\u05d2 "))},
	}
	for _, tc := range cases {
		for _, pos := range []struct {
			name  string
			atEnd bool
		}{{"start", false}, {"end", true}} {
			b.Run(tc.name+"/"+pos.name, func(b *testing.B) {
				benchmarkInputEditPair(b, tc.text, pos.atEnd)
			})
		}
	}
}

func benchmarkInputKeyPair(
	b *testing.B, seed string,
	first gui.KeyCode, firstMods gui.Modifier,
	second gui.KeyCode, secondMods gui.Modifier,
) {
	b.Helper()
	w, _, _, _ := editPairWindow(b, seed, true)
	press := func(key gui.KeyCode, mods gui.Modifier) {
		if err := w.TestKey("f", key, mods); err != nil {
			b.Fatal(err)
		}
	}
	// Warm both states, including the destination chunks and render paths.
	press(first, firstMods)
	press(second, secondMods)
	b.ReportAllocs()
	b.ReportMetric(float64(len(seed)), "seed-B")
	b.ResetTimer()
	useFirst := true
	for b.Loop() {
		if useFirst {
			press(first, firstMods)
		} else {
			press(second, secondMods)
		}
		useFirst = !useFirst
	}
}

// BenchmarkInputLargeNavigate covers the two non-editing paths that can
// accidentally turn a large value back into whole-buffer work. Home/End
// moves the caret and horizontal viewport between distant chunks; select-all
// and Right alternate a full logical selection with its collapsed state while
// rendering only the clipped portion of that selection.
func BenchmarkInputLargeNavigate(b *testing.B) {
	for _, size := range []int{100_000, 1 << 20} {
		seed := strings.Repeat("a", size)
		name := fmt.Sprintf("ascii_%dB", size)
		b.Run(name+"/home_end", func(b *testing.B) {
			benchmarkInputKeyPair(b, seed,
				gui.KeyHome, gui.ModNone, gui.KeyEnd, gui.ModNone)
		})
		b.Run(name+"/select_all_collapse", func(b *testing.B) {
			benchmarkInputKeyPair(b, seed,
				gui.KeyA, gui.ModCtrl, gui.KeyRight, gui.ModNone)
		})
	}
}

// BenchmarkInputLargePaste measures the one-shot path reported in #1190:
// clipboard insertion, controlled-value rebuild, caret placement and the
// scroll-into-view query. Minified JSON is one logical paragraph but still
// has UAX #14 wrap opportunities at some punctuation; the line-broken case
// distinguishes long-paragraph traversal from the immutable string copy.
func BenchmarkInputLargePaste(b *testing.B) {
	const jsonUnit = `{"id":17,"ok":true,"name":"response"},`
	repeatBytes := func(unit string, n int) string {
		return strings.Repeat(unit, (n+len(unit)-1)/len(unit))[:n]
	}
	cases := []struct {
		name string
		text string
	}{
		{"json_74KB", repeatBytes(jsonUnit, 74_000)},
		{"json_1MiB", repeatBytes(jsonUnit, 1<<20)},
		{"lines_1MiB", repeatBytes(strings.Repeat("x", 240)+"\n", 1<<20)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			w := gui.NewTestWindow(b, gui.WindowCfg{Width: 820, Height: 600})
			tm, err := prepare(w, 1)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { tm.textSys.Free() })
			w.SetTextMeasurer(tm)
			text := ""
			w.SetView(func(*gui.Window) gui.View {
				return gui.Input(gui.InputCfg{
					ID: "f", Text: text, Mode: gui.InputMultiline,
					Scrollable: true, Sizing: gui.FillFixed, Height: 220,
					MaxLength:     2 << 20,
					OnTextChanged: func(s string, _ gui.EventCtx) { text = s },
				})
			})
			w.SetClipboardGetFn(func() string { return tc.text })
			w.Update()
			w.SetFocus("f")
			w.Update()
			b.ReportAllocs()
			b.ReportMetric(float64(len(tc.text)), "paste-B")
			b.ResetTimer()
			for b.Loop() {
				if err := w.TestKey("f", gui.KeyV, gui.ModCtrl); err != nil {
					b.Fatal(err)
				}
				if text != tc.text {
					b.Fatalf("paste stored %d bytes, want %d", len(text), len(tc.text))
				}
				b.StopTimer()
				if err := w.TestKey("f", gui.KeyA, gui.ModCtrl); err != nil {
					b.Fatal(err)
				}
				if err := w.TestKey("f", gui.KeyBackspace, gui.ModNone); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
			}
		})
	}
}

// The keypress cost includes a whole-buffer immutable string copy, so the
// budget is pinned near the old shaper limit: a 10KB field must not get
// heap-hungrier without the bench above showing it first.
// Timing stays advisory (machine variance); allocations do not.
func TestInputKeypressAllocBudget(t *testing.T) {
	if inputRaceEnabled {
		t.Skip("AllocsPerRun is process-wide and unstable under race instrumentation")
	}
	w, reseed := keypressWindow(t, strings.Repeat("a", 10_000))
	x := gui.Event{Type: gui.EventChar, CharCode: 'x'}
	keypress := func() {
		reseed()
		e := x // AllocsPerRun invokes this closure repeatedly
		w.EventFn(&e)
		w.Update()
	}
	// Discard the first sweep: font and atlas warmth is one-time,
	// the gate is on the steady-state keypress.
	testing.AllocsPerRun(10, keypress)
	// AllocsPerRun reads process-wide malloc counters. A 50-keypress
	// sample takes seconds under -race, long enough for the caret loop,
	// font-cache warm and unrelated test goroutines to pollute every run.
	// Keep the lowest of short samples: a real per-keypress allocation
	// appears in all of them, while background noise does not.
	const limit = 370
	best := math.Inf(1)
	for range 20 {
		best = min(best, testing.AllocsPerRun(1, keypress))
		if best <= limit {
			break
		}
	}
	if best > limit {
		t.Fatalf("keypress allocs = %v, want <= %d", best, limit)
	}
}

// A large edit necessarily allocates a new immutable Go string, but it must
// not also rebuild a full-size display string or reshape the whole value.
// Count is a poor signal here: one accidental 1 MiB allocation is still one
// allocation, so this gate samples allocated bytes per steady-state edit.
func TestInputLargeEditAllocBudget(t *testing.T) {
	const editsPerSample = 10
	seed := strings.Repeat("a", 1<<20)
	w, insert, backspace, text := editPairWindow(t, seed, false)

	run := func() {
		for i := range editsPerSample {
			e := *insert
			if i%2 != 0 {
				e = *backspace
			}
			w.EventFn(&e)
			w.Update()
		}
		if *text != seed {
			t.Fatalf("edit sample left %d bytes, want %d", len(*text), len(seed))
		}
	}
	// Warm all long-lived shaper and rendering scratch before sampling.
	run()
	best := ^uint64(0)
	for range 5 {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		run()
		runtime.ReadMemStats(&after)
		best = min(best, (after.TotalAlloc-before.TotalAlloc)/editsPerSample)
	}
	const maxBytesPerEdit = 1_300_000
	if best > maxBytesPerEdit {
		t.Fatalf("large edit allocated %d bytes, want <= %d",
			best, maxBytesPerEdit)
	}
	t.Logf("best 1 MiB edit allocation: %d bytes", best)
}

// Moving and selecting may inspect a distant chunk, but neither operation
// creates a new value string. Keep their allocations independent of the
// value size; 32 KiB leaves ample runtime noise above the ~5 KiB baseline
// while still catching a copied 1 MiB value or a full selection layout.
func TestInputLargeNavigateAllocBudget(t *testing.T) {
	seed := strings.Repeat("a", 1<<20)
	w, _, _, _ := editPairWindow(t, seed, true)
	measure := func(
		first gui.KeyCode, firstMods gui.Modifier,
		second gui.KeyCode, secondMods gui.Modifier,
	) uint64 {
		press := func(key gui.KeyCode, mods gui.Modifier) {
			if err := w.TestKey("f", key, mods); err != nil {
				t.Fatal(err)
			}
		}
		press(first, firstMods)
		press(second, secondMods)
		best := ^uint64(0)
		const pairs = 10
		for range 5 {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for range pairs {
				press(first, firstMods)
				press(second, secondMods)
			}
			runtime.ReadMemStats(&after)
			best = min(best, (after.TotalAlloc-before.TotalAlloc)/(2*pairs))
		}
		return best
	}
	for name, got := range map[string]uint64{
		"home/end": measure(gui.KeyHome, gui.ModNone,
			gui.KeyEnd, gui.ModNone),
		"select all/collapse": measure(gui.KeyA, gui.ModCtrl,
			gui.KeyRight, gui.ModNone),
	} {
		const maxBytesPerAction = 32 << 10
		if got > maxBytesPerAction {
			t.Errorf("%s allocated %d bytes, want <= %d",
				name, got, maxBytesPerAction)
		}
		t.Logf("best 1 MiB %s allocation: %d bytes", name, got)
	}
}
