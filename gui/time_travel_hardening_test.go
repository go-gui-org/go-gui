package gui

import (
	"math"
	"strings"
	"testing"
	"time"
)

// TestRestoreTypeMismatchRecovers confirms a type-mismatched
// namespace entry is skipped via recover instead of crashing
// the entire restore. Simulates generic-type drift by replacing
// the receiving BoundedMap with one of a different value type
// after capture.
func TestRestoreTypeMismatchRecovers(t *testing.T) {
	const ns = "test.type.drift"
	t.Cleanup(func() {
		snapshotableNamespaces.mu.Lock()
		delete(snapshotableNamespaces.set, ns)
		snapshotableNamespaces.mu.Unlock()
	})
	registerNamespaceSnapshot(ns)

	w := &Window{state: &testState{}, focused: true}
	w.history = newSnapshotRing(1 << 20)
	origin := StateMap[string, int](w, ns, capFew)
	origin.Set("k", 1)

	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})

	// Swap the namespace's BoundedMap to a different value type
	// so restoreAny will panic; safeRestoreAny must recover.
	w.viewState.registry.maps[ns] = NewBoundedMap[string, string](capFew)

	w.postRestore(0)
	w.flushCommands()
	// No panic and the second namespace (untouched) keeps whatever
	// is in it — restore simply skipped this one.
	other, _ := w.viewState.registry.maps[ns].(*BoundedMap[string, string])
	if other == nil {
		t.Fatal("namespace map unexpectedly replaced")
	}
}

// TestSnapshotRingEntrySizeClamped caps a misbehaving Size()
// return so totalBytes cannot overflow.
func TestSnapshotRingEntrySizeClamped(t *testing.T) {
	r := newSnapshotRing(1 << 40)
	r.push((&oversizedState{n: math.MaxInt}).Snapshot(),
		time.Now(), "big")
	e, _ := r.last()
	if e.bytes != snapshotMaxEntryBytes {
		t.Fatalf("bytes = %d, want %d", e.bytes, snapshotMaxEntryBytes)
	}
	if r.bytes() != snapshotMaxEntryBytes {
		t.Fatalf("totalBytes = %d, want %d",
			r.bytes(), snapshotMaxEntryBytes)
	}
}

type oversizedState struct{ n int }

func (s *oversizedState) Snapshot() any { return &oversizedState{n: s.n} }
func (s *oversizedState) Restore(v any) { *s = *v.(*oversizedState) }
func (s *oversizedState) Size() int     { return s.n }

// TestOnSliderChangeRejectsNaN leaves the cursor unchanged when
// the slider emits NaN or ±Inf (implementation-defined int()).
func TestOnSliderChangeRejectsNaN(t *testing.T) {
	w, _ := newFixtureApp(t, 3)
	c := &timeTravelController{App: w, Cursor: 1}

	c.onSliderChange(float32(math.NaN()), EventCtx{nil, nil, nil})
	w.flushCommands()
	if c.Cursor != 1 {
		t.Fatalf("NaN cursor = %d, want 1", c.Cursor)
	}
	c.onSliderChange(float32(math.Inf(1)), EventCtx{nil, nil, nil})
	w.flushCommands()
	if c.Cursor != 1 {
		t.Fatalf("+Inf cursor = %d, want 1", c.Cursor)
	}
	c.onSliderChange(float32(math.Inf(-1)), EventCtx{nil, nil, nil})
	w.flushCommands()
	if c.Cursor != 1 {
		t.Fatalf("-Inf cursor = %d, want 1", c.Cursor)
	}
	c.onSliderChange(2.0, EventCtx{nil, nil, nil})
	w.flushCommands()
	if c.Cursor != 2 {
		t.Fatalf("normal cursor = %d, want 2", c.Cursor)
	}
}

// TestOnSliderChangeNil tolerates a nil receiver.
func TestOnSliderChangeNil(t *testing.T) {
	var c *timeTravelController
	c.onSliderChange(1.0, EventCtx{nil, nil, nil})
}

// TestOnSliderChangeClampsOutOfRange clamps v < 0 to 0 and
// v > len-1 to len-1 so a misbehaving slider backend cannot
// leave sliderValue outside the track's [Min, Max].
func TestOnSliderChangeClampsOutOfRange(t *testing.T) {
	w, _ := newFixtureApp(t, 4)
	c := &timeTravelController{App: w, Cursor: 2}

	c.onSliderChange(-5, EventCtx{nil, nil, nil})
	w.flushCommands()
	if c.sliderValue != 0 {
		t.Fatalf("negative sliderValue = %v, want 0", c.sliderValue)
	}
	if c.Cursor != 0 {
		t.Fatalf("negative Cursor = %d, want 0", c.Cursor)
	}

	c.onSliderChange(99, EventCtx{nil, nil, nil})
	w.flushCommands()
	maxV := float32(3)
	if c.sliderValue != maxV {
		t.Fatalf("overflow sliderValue = %v, want %v", c.sliderValue, maxV)
	}
	if c.Cursor != 3 {
		t.Fatalf("overflow Cursor = %d, want 3", c.Cursor)
	}
}

// TestOnSliderChangeEmptyRingNoOp early-returns when history is
// empty so a stray OnChange cannot assign a non-zero sliderValue
// to a zero-max slider.
func TestOnSliderChangeEmptyRingNoOp(t *testing.T) {
	w := &Window{state: &testState{}}
	w.enableHistory(0)
	c := &timeTravelController{App: w}
	c.onSliderChange(5, EventCtx{nil, nil, nil})
	if c.sliderValue != 0 {
		t.Fatalf("sliderValue = %v, want 0", c.sliderValue)
	}
	if c.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0", c.Cursor)
	}
}

// TestOnSliderChangeKeepsFractional confirms the rounded int
// commits Cursor while the fractional float survives in
// sliderValue so mid-drag frames display the live mouse
// position instead of snapping to the cursor.
func TestOnSliderChangeKeepsFractional(t *testing.T) {
	w, _ := newFixtureApp(t, 5)
	c := &timeTravelController{App: w, Cursor: 0}
	c.onSliderChange(2.7, EventCtx{nil, nil, nil})
	w.flushCommands()
	if c.Cursor != 2 {
		t.Fatalf("Cursor = %d, want 2 (int of 2.7)", c.Cursor)
	}
	if c.sliderValue != 2.7 {
		t.Fatalf("sliderValue = %v, want 2.7 (fractional preserved)",
			c.sliderValue)
	}
}

// TestJumpSyncsSliderValue asserts that non-slider motion paths
// (keyboard, buttons) re-align the thumb with the integer
// cursor via Jump.
func TestJumpSyncsSliderValue(t *testing.T) {
	w, _ := newFixtureApp(t, 5)
	c := &timeTravelController{App: w, Cursor: 0, sliderValue: 3.7}
	c.jump(1)
	if c.sliderValue != 1 {
		t.Fatalf("sliderValue = %v, want 1 after Jump", c.sliderValue)
	}
}

// TestOpenDebugWindowCloseResumes confirms the queued cfg
// installs an OnCloseRequest that unfreezes the app, so
// closing the scrubber cannot strand the user with input
// permanently blocked.
func TestOpenDebugWindowCloseResumes(t *testing.T) {
	app := NewApp()
	w := &Window{state: &testState{}}
	w.app = app
	w.platformID = 1
	w.enableHistory(0)
	w.Freeze()
	if !w.isFrozen() {
		t.Fatal("precondition: app should be frozen")
	}
	w.openDebugWindow()

	cfg := <-app.PendingOpen()
	if cfg.OnCloseRequest == nil {
		t.Fatal("OnCloseRequest not wired")
	}
	dw := NewWindow(cfg)
	cfg.OnCloseRequest(dw)

	if w.isFrozen() {
		t.Fatal("app still frozen after debug close")
	}
	if !dw.CloseRequested() {
		t.Fatal("debug window close was not requested")
	}
}

// TestOpenDebugWindowDimensions guards the compact 300x150
// sizing so a future edit doesn't silently regress it.
func TestOpenDebugWindowDimensions(t *testing.T) {
	app := NewApp()
	w := &Window{state: &testState{}}
	w.app = app
	w.platformID = 1
	w.enableHistory(0)
	w.openDebugWindow()

	cfg := <-app.PendingOpen()
	if cfg.Width != 300 || cfg.Height != 150 {
		t.Fatalf("size = %dx%d, want 300x150", cfg.Width, cfg.Height)
	}
}

// TestControllerViewNilHost returns an empty view rather than
// panicking when the host window is nil.
func TestControllerViewNilHost(t *testing.T) {
	c := &timeTravelController{}
	if c.View(nil) == nil {
		t.Fatal("View(nil) returned nil")
	}
}

// TestBoundedMapCloneRestoreRoundTrip round-trips entries
// through cloneAny/restoreAny.
func TestBoundedMapCloneRestoreRoundTrip(t *testing.T) {
	src := NewBoundedMap[string, int](capFew)
	src.Set("a", 1)
	src.Set("b", 2)
	src.Set("c", 3)

	clone := src.cloneAny().(*BoundedMap[string, int])
	src.Set("a", 999)
	src.Delete("b")

	if v, _ := clone.Get("a"); v != 1 {
		t.Fatalf("clone decoupled: a = %d, want 1", v)
	}
	if v, _ := clone.Get("b"); v != 2 {
		t.Fatalf("clone decoupled: b = %d, want 2", v)
	}
	if clone.Len() != 3 {
		t.Fatalf("clone len = %d, want 3", clone.Len())
	}

	dst := NewBoundedMap[string, int](capFew)
	dst.Set("zombie", 42)
	dst.restoreAny(clone)
	if _, ok := dst.Get("zombie"); ok {
		t.Fatal("restore did not clear existing entries")
	}
	if v, _ := dst.Get("a"); v != 1 {
		t.Fatalf("restore a = %d, want 1", v)
	}
}

// TestBoundedMapCloneEmpty clones an empty map to an empty map.
func TestBoundedMapCloneEmpty(t *testing.T) {
	src := NewBoundedMap[int, int](capFew)
	clone := src.cloneAny().(*BoundedMap[int, int])
	if clone.Len() != 0 {
		t.Fatalf("empty clone len = %d, want 0", clone.Len())
	}
}

// TestOpenDebugWindowTruncatesLongTitle caps oversize parent
// titles in the composed debug window title.
func TestOpenDebugWindowTruncatesLongTitle(t *testing.T) {
	app := NewApp()
	w := &Window{state: &testState{}}
	w.app = app
	w.platformID = 1
	w.Config.Title = strings.Repeat("x", 4096)
	w.enableHistory(0)
	w.openDebugWindow()

	select {
	case cfg := <-app.PendingOpen():
		// Composed title is "Time Travel — " + up to 256 bytes
		// of parent. The prefix itself is small; assert total
		// stays well under an unsafe length.
		if len(cfg.Title) > 512 {
			t.Fatalf("title len = %d, want <= 512", len(cfg.Title))
		}
		if !strings.HasPrefix(cfg.Title, "Time Travel — ") {
			t.Fatalf("title prefix wrong: %q", cfg.Title)
		}
	default:
		t.Fatal("no WindowCfg queued")
	}
}

// TestShouldSnapshotSkipsAllBookkeeping covers every event type
// that shouldSnapshot must skip.
func TestShouldSnapshotSkipsAllBookkeeping(t *testing.T) {
	skip := []EventType{
		EventMouseMove, EventIMEComposition,
		EventFocused, EventUnfocused, EventResized,
	}
	for _, et := range skip {
		if shouldSnapshot(et) {
			t.Errorf("shouldSnapshot(%v) = true, want false", et)
		}
	}
	for _, et := range []EventType{
		EventKeyDown, EventMouseDown, EventMouseUp,
		EventMouseScroll, EventChar, EventFileDropped,
	} {
		if !shouldSnapshot(et) {
			t.Errorf("shouldSnapshot(%v) = false, want true", et)
		}
	}
}

// TestEventCauseLabels covers the full cause mapping.
func TestEventCauseLabels(t *testing.T) {
	cases := map[EventType]string{
		EventChar:             "char",
		EventKeyDown:          "key",
		EventMouseDown:        "mouse-down",
		EventMouseUp:          "mouse-up",
		EventMouseScroll:      "scroll",
		EventFileDropped:      "file-drop",
		EventTouchesBegan:     "touch-begin",
		EventTouchesMoved:     "touch-move",
		EventTouchesEnded:     "touch-end",
		EventTouchesCancelled: "touch-cancel",
	}
	for et, want := range cases {
		got := eventCause(&Event{Type: et})
		if got != want {
			t.Errorf("%v = %q, want %q", et, got, want)
		}
	}
	// Nil-safe.
	if got := eventCause(nil); got != "" {
		t.Errorf("nil = %q, want empty", got)
	}
	// Unknown event returns fallback.
	if got := eventCause(&Event{Type: EventType(127)}); got != "event" {
		t.Errorf("unknown fallback = %q, want event", got)
	}
}

// TestFreezeLabel returns distinct strings for frozen / live.
func TestFreezeLabel(t *testing.T) {
	if got := freezeLabel(nil); got != "Pause" {
		t.Errorf("nil = %q, want Pause", got)
	}
	w := &Window{}
	w.history = newSnapshotRing(0)
	if got := freezeLabel(w); got != "Pause" {
		t.Errorf("live = %q, want Pause", got)
	}
	w.Freeze()
	if got := freezeLabel(w); got != "Resume" {
		t.Errorf("frozen = %q, want Resume", got)
	}
}

// TestControllerResumeLiveEmpty on a populated-but-history-len-0
// window leaves Cursor untouched.
func TestControllerResumeLiveEmpty(t *testing.T) {
	w := &Window{state: &testState{}}
	w.history = newSnapshotRing(1 << 20)
	c := &timeTravelController{App: w, Cursor: 7}
	c.resumeLive()
	if c.Cursor != 7 {
		t.Fatalf("Cursor = %d, want 7 (empty history leaves cursor)",
			c.Cursor)
	}
}

// TestEnableHistoryShrinkEvicts shrinks the cap and confirms
// oldest entries are evicted to fit.
func TestEnableHistoryShrinkEvicts(t *testing.T) {
	w := &Window{state: &testState{}}
	w.enableHistory(4 * snapshotDefaultSize)
	for range 4 {
		w.history.push((&testState{}).Snapshot(), time.Now(), "e")
	}
	if w.history.len() != 4 {
		t.Fatalf("pre-shrink len = %d, want 4", w.history.len())
	}
	w.enableHistory(2 * snapshotDefaultSize)
	if w.history.len() != 2 {
		t.Fatalf("post-shrink len = %d, want 2", w.history.len())
	}
	if w.history.bytes() > 2*snapshotDefaultSize {
		t.Fatalf("bytes = %d over cap", w.history.bytes())
	}
}

// TestTimeTravelScrubIntegration drives a series of events
// through a window with history enabled, scrubs to an earlier
// snapshot, and asserts user state reverts. End-to-end check
// of the capture-post-restore flow without a live backend.
func TestTimeTravelScrubIntegration(t *testing.T) {
	s := &testState{}
	w := &Window{state: s, focused: true}
	w.enableHistory(0)

	// Drive 5 mouse-down events that each bump the counter.
	for i := 1; i <= 5; i++ {
		s.counter = i
		w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	}
	if n := w.history.len(); n != 5 {
		t.Fatalf("history len = %d, want 5", n)
	}
	if s.counter != 5 {
		t.Fatalf("live counter = %d, want 5", s.counter)
	}

	// Scrub back to snapshot 1 (counter was 2 when that entry
	// was captured AFTER the second event; snapshot 0 has
	// counter == 1). Jump to 0.
	c := &timeTravelController{App: w, Cursor: 4}
	c.jump(0)
	w.flushCommands()
	if s.counter != 1 {
		t.Fatalf("scrubbed counter = %d, want 1", s.counter)
	}
	if !w.isFrozen() {
		t.Fatal("scrubbing should auto-freeze")
	}

	// ResumeLive restores the newest entry and unfreezes.
	c.resumeLive()
	w.flushCommands()
	// Resume itself does not replay snapshots; it only unpins
	// time and unfreezes. Cursor snaps to newest entry.
	if c.Cursor != 4 {
		t.Fatalf("Cursor = %d, want 4 after ResumeLive", c.Cursor)
	}
	if w.isFrozen() {
		t.Fatal("still frozen after ResumeLive")
	}

	// Further events continue to append history.
	s.counter = 6
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if n := w.history.len(); n != 6 {
		t.Fatalf("post-resume len = %d, want 6", n)
	}
}
