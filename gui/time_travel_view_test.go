package gui

import (
	"testing"
	"time"
)

// newFixtureApp returns an app-style window with history filled
// to n entries and cursor at the newest entry.
func newFixtureApp(t *testing.T, n int) (*Window, *testState) {
	t.Helper()
	s := &testState{}
	w := &Window{state: s, focused: true}
	w.history = newSnapshotRing(1 << 20)
	for i := range n {
		s.counter = i + 1
		w.history.push(s.Snapshot(), time.Now(), "e")
	}
	return w, s
}

// TestControllerLenBytes reports ring state.
func TestControllerLenBytes(t *testing.T) {
	w, _ := newFixtureApp(t, 3)
	c := &timeTravelController{App: w}
	if got := c.Len(); got != 3 {
		t.Fatalf("Len = %d, want 3", got)
	}
	if c.Bytes() == 0 {
		t.Fatal("Bytes = 0, want > 0")
	}
}

// TestControllerJumpClamps clamps negative and overflow idx.
func TestControllerJumpClamps(t *testing.T) {
	w, s := newFixtureApp(t, 3)
	c := &timeTravelController{App: w}
	c.jump(-5)
	w.flushCommands()
	if c.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0 (clamped low)", c.Cursor)
	}
	if s.counter != 1 {
		t.Fatalf("state counter = %d, want 1", s.counter)
	}
	c.jump(999)
	w.flushCommands()
	if c.Cursor != 2 {
		t.Fatalf("Cursor = %d, want 2 (clamped high)", c.Cursor)
	}
	if s.counter != 3 {
		t.Fatalf("state counter = %d, want 3", s.counter)
	}
}

// TestControllerJumpFreezesApp confirms a jump auto-freezes.
func TestControllerJumpFreezesApp(t *testing.T) {
	w, _ := newFixtureApp(t, 2)
	c := &timeTravelController{App: w}
	c.jump(0)
	if !w.isFrozen() {
		t.Fatal("expected app window frozen after Jump")
	}
}

// TestControllerStepBackForward walks the cursor.
func TestControllerStepBackForward(t *testing.T) {
	w, s := newFixtureApp(t, 3)
	c := &timeTravelController{App: w, Cursor: 2}
	c.stepBack()
	w.flushCommands()
	if c.Cursor != 1 || s.counter != 2 {
		t.Fatalf("after StepBack: cursor=%d counter=%d", c.Cursor, s.counter)
	}
	c.stepForward()
	w.flushCommands()
	if c.Cursor != 2 || s.counter != 3 {
		t.Fatalf("after StepForward: cursor=%d counter=%d", c.Cursor, s.counter)
	}
}

// TestControllerFirstLast jumps to ends.
func TestControllerFirstLast(t *testing.T) {
	w, s := newFixtureApp(t, 4)
	c := &timeTravelController{App: w, Cursor: 2}
	c.First()
	w.flushCommands()
	if c.Cursor != 0 || s.counter != 1 {
		t.Fatalf("First: cursor=%d counter=%d", c.Cursor, s.counter)
	}
	c.Last()
	w.flushCommands()
	if c.Cursor != 3 || s.counter != 4 {
		t.Fatalf("Last: cursor=%d counter=%d", c.Cursor, s.counter)
	}
}

// TestControllerResumeLive unfreezes and snaps cursor to newest.
func TestControllerResumeLive(t *testing.T) {
	w, _ := newFixtureApp(t, 3)
	c := &timeTravelController{App: w}
	c.jump(0)
	c.resumeLive()
	if w.isFrozen() {
		t.Fatal("still frozen after ResumeLive")
	}
	if c.Cursor != 2 {
		t.Fatalf("Cursor = %d, want 2 (newest)", c.Cursor)
	}
	if w.virtualNow.Load() != nil {
		t.Fatal("virtualNow not cleared")
	}
}

// TestControllerToggleFreeze flips freeze on and off.
func TestControllerToggleFreeze(t *testing.T) {
	w, _ := newFixtureApp(t, 2)
	c := &timeTravelController{App: w, Cursor: 1}
	c.toggleFreeze()
	if !w.isFrozen() {
		t.Fatal("ToggleFreeze should freeze")
	}
	c.toggleFreeze()
	if w.isFrozen() {
		t.Fatal("second ToggleFreeze should unfreeze")
	}
}

// TestControllerEmptyRing leaves state unchanged.
func TestControllerEmptyRing(t *testing.T) {
	w := &Window{state: &testState{}}
	w.history = newSnapshotRing(1 << 20)
	c := &timeTravelController{App: w}
	c.jump(0)
	c.First()
	c.Last()
	if c.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0 (untouched)", c.Cursor)
	}
	if w.isFrozen() {
		t.Fatal("empty ring should not auto-freeze")
	}
}

// TestControllerNilSafe tolerates nil receivers and nil app.
func TestControllerNilSafe(t *testing.T) {
	var c *timeTravelController
	c.jump(0)
	c.stepBack()
	c.stepForward()
	c.First()
	c.Last()
	c.resumeLive()
	c.toggleFreeze()
	if got := c.Len(); got != 0 {
		t.Fatalf("nil Len = %d", got)
	}

	c2 := &timeTravelController{}
	c2.jump(0)
	if c2.Cursor != 0 {
		t.Fatalf("nil-App Cursor = %d", c2.Cursor)
	}
}

// TestControllerCauseLabel reports the entry's cause.
func TestControllerCauseLabel(t *testing.T) {
	w := &Window{state: &testState{}}
	w.history = newSnapshotRing(1 << 20)
	w.history.push((&testState{}).Snapshot(), time.Now(), "mouse-down")
	c := &timeTravelController{App: w, Cursor: 0}
	if got := c.cause(); got != "mouse-down" {
		t.Fatalf("Cause = %q, want mouse-down", got)
	}
}

// TestControllerHandleKey maps arrow/home/end/space/esc to
// the expected controller actions.
func TestControllerHandleKey(t *testing.T) {
	w, s := newFixtureApp(t, 4)
	c := &timeTravelController{App: w, Cursor: 2}

	fire := func(k KeyCode) *Event {
		e := &Event{Type: EventKeyDown, KeyCode: k}
		c.handleKey(EventCtx{nil, e, nil})
		w.flushCommands()
		return e
	}

	e := fire(KeyLeft)
	if c.Cursor != 1 || s.counter != 2 || !e.IsHandled {
		t.Fatalf("KeyLeft: cursor=%d counter=%d handled=%v",
			c.Cursor, s.counter, e.IsHandled)
	}
	fire(KeyRight)
	if c.Cursor != 2 {
		t.Fatalf("KeyRight: cursor=%d, want 2", c.Cursor)
	}
	fire(KeyHome)
	if c.Cursor != 0 {
		t.Fatalf("KeyHome: cursor=%d, want 0", c.Cursor)
	}
	fire(KeyEnd)
	if c.Cursor != 3 {
		t.Fatalf("KeyEnd: cursor=%d, want 3", c.Cursor)
	}
	// Prior jumps auto-froze; KeyEscape resumes.
	fire(KeyEscape)
	if w.isFrozen() {
		t.Fatal("KeyEscape should resume")
	}
	fire(KeySpace)
	if !w.isFrozen() {
		t.Fatal("KeySpace from live should freeze")
	}
	fire(KeySpace)
	if w.isFrozen() {
		t.Fatal("second KeySpace should resume")
	}

	// Unmapped keys pass through unhandled.
	e = &Event{Type: EventKeyDown, KeyCode: KeyF12}
	c.handleKey(EventCtx{nil, e, nil})
	if e.IsHandled {
		t.Fatal("unmapped key should not be marked handled")
	}
}

// TestEnableHistory initializes the ring with the given cap
// and is idempotent for subsequent calls.
func TestEnableHistory(t *testing.T) {
	w := &Window{state: &testState{}}
	w.enableHistory(0)
	if w.history == nil || w.history.maxBytes != defaultHistoryBytes {
		t.Fatalf("default cap not applied: %+v", w.history)
	}
	w.enableHistory(2048)
	if w.history.maxBytes != 2048 {
		t.Fatalf("cap update: %d, want 2048", w.history.maxBytes)
	}
}

// TestHistoryLen reports ring length safely when disabled.
func TestHistoryLen(t *testing.T) {
	var w *Window
	if got := w.HistoryLen(); got != 0 {
		t.Fatalf("nil HistoryLen = %d", got)
	}
	w = &Window{}
	if got := w.HistoryLen(); got != 0 {
		t.Fatalf("disabled HistoryLen = %d", got)
	}
	w.enableHistory(0)
	w.history.push((&testState{}).Snapshot(), time.Now(), "e")
	if got := w.HistoryLen(); got != 1 {
		t.Fatalf("HistoryLen = %d, want 1", got)
	}
}

// TestOpenDebugWindowNoApp is a no-op without an App.
func TestOpenDebugWindowNoApp(t *testing.T) {
	w := &Window{state: &testState{}}
	w.enableHistory(0)
	w.openDebugWindow() // must not panic
}

// TestOpenDebugWindowQueuesCfg verifies a WindowCfg is queued
// on the App with the controller as its State.
func TestOpenDebugWindowQueuesCfg(t *testing.T) {
	app := NewApp()
	s := &testState{counter: 1}
	w := &Window{state: s}
	w.app = app
	w.platformID = 1
	w.Config.Title = "MyApp"
	w.enableHistory(0)
	w.history.push(s.Snapshot(), time.Now(), "init")

	w.openDebugWindow()

	select {
	case cfg := <-app.PendingOpen():
		if cfg.Title != "Time Travel — MyApp" {
			t.Fatalf("title = %q", cfg.Title)
		}
		ctrl, ok := cfg.State.(*timeTravelController)
		if !ok {
			t.Fatalf("State type = %T", cfg.State)
		}
		if ctrl.App != w {
			t.Fatal("controller.App not wired to parent")
		}
		if ctrl.Cursor != 0 {
			t.Fatalf("Cursor = %d, want 0 (len-1 of 1 entry)", ctrl.Cursor)
		}
		if cfg.OnInit == nil {
			t.Fatal("OnInit unset")
		}
	default:
		t.Fatal("no WindowCfg queued")
	}
}

// TestDebugTimeTravelCfgEnablesHistory auto-enables history
// when NewWindow sees DebugTimeTravel in the cfg.
func TestDebugTimeTravelCfgEnablesHistory(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:           &testState{},
		DebugTimeTravel: true,
		historyBytes:    4096,
	})
	if w.history == nil {
		t.Fatal("history not enabled")
	}
	if w.history.maxBytes != 4096 {
		t.Fatalf("maxBytes = %d, want 4096", w.history.maxBytes)
	}
}

// TestDebugTimeTravelCfgDefaultCap applies the default cap
// when HistoryBytes is zero.
func TestDebugTimeTravelCfgDefaultCap(t *testing.T) {
	w := NewWindow(WindowCfg{
		State:           &testState{},
		DebugTimeTravel: true,
	})
	if w.history == nil || w.history.maxBytes != defaultHistoryBytes {
		t.Fatalf("default cap not applied: %+v", w.history)
	}
}

// TestDebugTimeTravelCfgDisabled leaves history nil when the
// flag is false.
func TestDebugTimeTravelCfgDisabled(t *testing.T) {
	w := NewWindow(WindowCfg{State: &testState{}})
	if w.history != nil {
		t.Fatal("history should stay nil without DebugTimeTravel")
	}
}

// TestAppRegisterSpawnsDebugWindow confirms Register enqueues a
// scrubber cfg for windows that opted into DebugTimeTravel.
func TestAppRegisterSpawnsDebugWindow(t *testing.T) {
	app := NewApp()
	w := NewWindow(WindowCfg{
		State:           &testState{},
		DebugTimeTravel: true,
	})
	app.Register(1, w)

	select {
	case cfg := <-app.PendingOpen():
		if _, ok := cfg.State.(*timeTravelController); !ok {
			t.Fatalf("queued cfg State = %T, want *TimeTravelController",
				cfg.State)
		}
	default:
		t.Fatal("no scrubber cfg queued after Register")
	}
}

// TestAppRegisterNoSpawnWhenDisabled confirms Register does not
// queue anything when DebugTimeTravel is false.
func TestAppRegisterNoSpawnWhenDisabled(t *testing.T) {
	app := NewApp()
	w := NewWindow(WindowCfg{State: &testState{}})
	app.Register(1, w)

	select {
	case cfg := <-app.PendingOpen():
		t.Fatalf("unexpected cfg queued: %+v", cfg.Title)
	default:
	}
}

// TestControllerView returns a non-nil view for empty and
// populated rings.
func TestControllerView(t *testing.T) {
	c := &timeTravelController{}
	host := &Window{}
	if c.View(host) == nil {
		t.Fatal("empty View nil")
	}
	w, _ := newFixtureApp(t, 2)
	c.App = w
	if c.View(host) == nil {
		t.Fatal("populated View nil")
	}
}
