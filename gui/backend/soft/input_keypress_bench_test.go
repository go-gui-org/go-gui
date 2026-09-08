package soft

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// F2: glyph-per-keypress profile. One keystroke in an Input costs an
// insert (rune-slice rebuild), a full Update (generate, arrange,
// render), and — when the caret is visible — a whole-buffer glyph
// reshape, because renderInputCursor needs character boundaries and
// the layout cache keys on the full text. These benches measure the
// whole keypress against the REAL shaper: the gui stubs skip
// LayoutText entirely and would profile the fallback path.
//
// Each iteration re-seeds the same text so size stays fixed (typing
// "x" into the seed, then rebuilding the view from the seed).
// Sizes are byte budgets, because the shaper caps input at 10240
// bytes (go-glyph MaxTextLength) and anything past that measures the
// fallback path instead of shaping.
// keypressWindow returns a window holding a focused Input seeded
// with text, measured by the real shaper, plus a reseed func that
// rebuilds the view from the seed. Each iteration re-seeds so the
// size stays fixed while typing "x" into it.
func keypressWindow(
	tb testing.TB, seed string,
) (*gui.Window, func()) {
	tb.Helper()
	w := gui.NewWindow(gui.WindowCfg{Width: 800, Height: 600})
	tm, err := prepare(w, 1)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { tm.textSys.Free() })
	w.SetTextMeasurer(tm)
	reseed := func() {
		w.UpdateView(func(w *gui.Window) gui.View {
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
		w.EventFn(&x)
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

// The keypress cost scales with the text size (whole-buffer insert
// copies plus one full reshape per keystroke), so the budget is
// pinned at the top of the shaper's byte range: a 10KB field must
// not get heap-hungrier without the bench above showing it first.
// Timing stays advisory (machine variance); allocations do not.
func TestInputKeypressAllocBudget(t *testing.T) {
	w, reseed := keypressWindow(t, strings.Repeat("a", 10_000))
	x := gui.Event{Type: gui.EventChar, CharCode: 'x'}
	keypress := func() {
		reseed()
		w.EventFn(&x)
		w.Update()
	}
	// Discard the first sweep: font and atlas warmth is one-time,
	// the gate is on the steady-state keypress.
	testing.AllocsPerRun(10, keypress)
	if got := testing.AllocsPerRun(50, keypress); got > 400 {
		t.Fatalf("keypress allocs = %v, want <= 400", got)
	}
}
