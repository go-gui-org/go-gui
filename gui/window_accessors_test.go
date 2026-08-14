package gui

import (
	"sync"
	"testing"
	"time"
)

// fontListerMeasurer is a TextMeasurer that also implements FontLister.
// stubTextMeasurer (layout_pipeline_test.go) deliberately does NOT
// implement FontLister, so it doubles as the negative case below.
type fontListerMeasurer struct {
	stubTextMeasurer
	families []string
}

func (m *fontListerMeasurer) ListFontFamilies() []string { return m.families }

func TestListSystemFontsNilMeasurer(t *testing.T) {
	// Before backend init w.textMeasurer is nil — the type assertion
	// must fail cleanly rather than panic.
	w := newTestWindow()
	if got := ListSystemFonts(w); got != nil {
		t.Fatalf("ListSystemFonts with nil measurer = %v, want nil", got)
	}
}

func TestListSystemFontsNilWindow(t *testing.T) {
	// Matches the nil-receiver tolerance of TextWidth and Now: a font
	// picker built before the window exists degrades to an empty list
	// instead of panicking.
	if got := ListSystemFonts(nil); got != nil {
		t.Fatalf("ListSystemFonts(nil) = %v, want nil", got)
	}
}

func TestListSystemFontsMeasurerWithoutFontLister(t *testing.T) {
	// FontLister is an optional capability; a plain measurer opts out.
	w := newTestWindow()
	w.SetTextMeasurer(&stubTextMeasurer{charWidth: 10, fontHeight: 20})
	if got := ListSystemFonts(w); got != nil {
		t.Fatalf("ListSystemFonts without FontLister = %v, want nil", got)
	}
}

func TestListSystemFontsReturnsFamilies(t *testing.T) {
	want := []string{"Helvetica", "Menlo"}
	w := newTestWindow()
	w.SetTextMeasurer(&fontListerMeasurer{families: want})

	got := ListSystemFonts(w)
	if len(got) != len(want) {
		t.Fatalf("ListSystemFonts len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("family[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestListSystemFontsEmptyCatalog(t *testing.T) {
	// A FontLister with nothing registered returns nil, not a panic.
	w := newTestWindow()
	w.SetTextMeasurer(&fontListerMeasurer{families: nil})
	if got := ListSystemFonts(w); got != nil {
		t.Fatalf("ListSystemFonts with empty catalog = %v, want nil", got)
	}
}

func TestWindowTextMeasurerRoundTrip(t *testing.T) {
	w := newTestWindow()
	if w.TextMeasurer() != nil {
		t.Fatal("expected nil measurer on a fresh window")
	}

	tm := &stubTextMeasurer{charWidth: 7, fontHeight: 14}
	w.SetTextMeasurer(tm)
	if w.TextMeasurer() != tm {
		t.Fatal("TextMeasurer did not return the measurer set by SetTextMeasurer")
	}

	// Backends may clear the measurer during teardown.
	w.SetTextMeasurer(nil)
	if w.TextMeasurer() != nil {
		t.Fatal("expected nil measurer after SetTextMeasurer(nil)")
	}
}

func TestWindowTimingsZeroBeforeFirstFrame(t *testing.T) {
	// Timings reports the previous frame's numbers; before any frame
	// has run it must read as the zero value rather than garbage.
	w := newTestWindow()
	if got := (w.Timings()); got != (FrameTimings{}) {
		t.Fatalf("Timings before first frame = %+v, want zero value", got)
	}
}

func TestWindowAppNilInSingleWindowMode(t *testing.T) {
	// A window built without an App parent reports nil; widgets that
	// reach for multi-window services must nil-check.
	w := newTestWindow()
	if got := w.App(); got != nil {
		t.Fatalf("App() = %v, want nil for a standalone window", got)
	}
}

func TestWindowLockUnlockPair(t *testing.T) {
	// Lock/Unlock are a raw mutex passthrough; a matched pair must not
	// deadlock and must leave the mutex reacquirable.
	w := newTestWindow()

	w.Lock()
	w.windowWidth = 640
	w.Unlock()

	w.Lock()
	w.windowHeight = 480
	w.Unlock()

	if gotW, gotH := w.WindowSize(); gotW != 640 || gotH != 480 {
		t.Fatalf("WindowSize = %dx%d, want 640x480", gotW, gotH)
	}
}

func TestWindowLockExcludesSecondHolder(t *testing.T) {
	w := newTestWindow()

	var wg sync.WaitGroup
	acquired := make(chan struct{})

	w.Lock()
	wg.Go(func() {
		w.Lock()
		close(acquired)
		w.Unlock()
	})

	// The goroutine must still be blocked while this side holds the lock.
	select {
	case <-acquired:
		t.Fatal("second Lock succeeded while the window mutex was held")
	case <-time.After(20 * time.Millisecond):
	}

	w.Unlock()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("second Lock never acquired after Unlock")
	}
	wg.Wait()
}

func TestClearDrawCanvasCacheEmptyRegistry(t *testing.T) {
	// Called on every theme change, including before any canvas has
	// been drawn — an uninitialized registry must be a no-op.
	w := newTestWindow()
	w.ClearDrawCanvasCache()
}

func TestClearDrawCanvasCacheDropsEntries(t *testing.T) {
	w := newTestWindow()

	sm := StateMap[string, drawCanvasCache](w, nsDrawCanvas, capModerate)
	sm.Set("canvas-1", drawCanvasCache{Version: 7})
	if _, ok := sm.Get("canvas-1"); !ok {
		t.Fatal("seeded cache entry not readable")
	}

	w.ClearDrawCanvasCache()

	// ClearNamespace drops the whole map, so the namespace reads back nil.
	if got := StateMapRead[string, drawCanvasCache](w, nsDrawCanvas); got != nil {
		t.Fatal("expected nsDrawCanvas namespace to be cleared")
	}
}

func TestSetWakeMainFnAcceptsNil(t *testing.T) {
	// Headless/test windows never get a wake fn; clearing it must not
	// panic. The invocation path is covered by TestQueueCommandWakesMain.
	w := newTestWindow()
	w.SetWakeMainFn(func() {})
	w.SetWakeMainFn(nil)
}

func TestSetStateTyped(t *testing.T) {
	w := newTestWindow()
	type appState struct{ count int }
	w.setState(&appState{count: 7})

	got := State[appState](w)
	if got.count != 7 {
		t.Fatalf("State().count = %d, want 7", got.count)
	}
}

func TestStatePanicsOnTypeMismatch(t *testing.T) {
	w := newTestWindow()
	w.setState("string-state")
	defer func() {
		if recover() == nil {
			t.Fatal("State[int] on a string state must panic")
		}
	}()
	_ = State[int](w)
}

func TestFrameCount(t *testing.T) {
	w := newTestWindow()
	if got := w.FrameCount(); got != 0 {
		t.Fatalf("FrameCount before frames = %d, want 0", got)
	}
	w.FrameFn()
	w.FrameFn()
	w.FrameFn()
	if got := w.FrameCount(); got != 3 {
		t.Fatalf("FrameCount after 3 FrameFn = %d, want 3", got)
	}
}

func TestSetPrimaryGetFn(t *testing.T) {
	w := newTestWindow()
	if got := w.GetPrimary(); got != "" {
		t.Fatalf("GetPrimary without fn = %q, want \"\"", got)
	}
	w.SetPrimaryGetFn(func() string { return "sel" })
	if got := w.GetPrimary(); got != "sel" {
		t.Fatalf("GetPrimary = %q, want sel", got)
	}
	// Clearing the fn falls back to empty.
	w.SetPrimaryGetFn(nil)
	if got := w.GetPrimary(); got != "" {
		t.Fatalf("GetPrimary after nil fn = %q, want \"\"", got)
	}
}
