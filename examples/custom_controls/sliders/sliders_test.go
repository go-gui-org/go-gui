package sliders

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestApp renders the example once.
// Not t.Parallel in callers: SetTheme mutates process-global theme state.
func newTestApp(t *testing.T) (*App, *gui.Window) {
	t.Helper()
	gui.SetTheme(gui.ThemeLight)
	app := New()
	// Taller than the real window: with no text measurer the test layout
	// runs longer, and the pointer must stay inside the window to hover.
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 720, Height: 1000})
	w.TestRender(func(w *gui.Window) gui.View { return View(w, app) })
	return app, w
}

func mustFind(t *testing.T, w *gui.Window, id string) *gui.Shape {
	t.Helper()
	ly, ok := w.TestRender(nil).FindByID(id)
	if !ok {
		t.Fatalf("no widget %q", id)
	}
	return ly.Shape
}

// settle runs two frames: the first records what the pointer is over, the
// second builds the look from it. The real frame loop does both in one frame.
func settle(w *gui.Window) {
	w.TestRender(nil)
	w.TestRender(nil)
}

// send delivers one mouse event. Only a press or release names a button, as
// the backends do.
func send(w *gui.Window, typ gui.EventType, x, y float32) {
	e := gui.Event{Type: typ, MouseX: x, MouseY: y}
	if typ == gui.EventMouseDown || typ == gui.EventMouseUp {
		e.MouseButton = gui.MouseLeft
	}
	w.EventFn(&e)
	settle(w)
}

func near(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.001
}

// xAt is the window X at which the handle's center sits for value v. inset is
// half the handle width: the center stays that far inside each end.
func xAt(s *gui.Shape, inset, v float32) float32 {
	return s.X + inset + v/valueMax*(s.Width-2*inset)
}

// A press sets the value under the pointer. The drag keeps setting it while
// the pointer is outside the slider, and clamps it at both ends.
func TestPressAndDrag(t *testing.T) {
	app, w := newTestApp(t)
	tr := tracks[2] // Call volume
	s := mustFind(t, w, "page:material:"+tr.id)
	y := s.Y + s.Height/2

	send(w, gui.EventMouseDown, xAt(s, materialBladeW/2, 25), y)
	if v := app.value[tr.name]; !near(v, 25) {
		t.Fatalf("after press value %v, want 25", v)
	}
	// The press focuses the slider, and the focused blade is 6 wide, so the
	// drag maps over a span 1 px shorter at each end.
	const dragInset = 3

	// Far below and to the right of the slider: the lock still delivers
	// the move, and the value stops at 1.
	send(w, gui.EventMouseMove, s.X+s.Width+200, y+300)
	if v := app.value[tr.name]; v != valueMax {
		t.Fatalf("drag past the end: value %v, want %v", v, valueMax)
	}
	send(w, gui.EventMouseMove, xAt(s, dragInset, 60), y)
	if v := app.value[tr.name]; !near(v, 60) {
		t.Fatalf("drag back: value %v, want 60", v)
	}
	send(w, gui.EventMouseMove, s.X-50, y)
	if v := app.value[tr.name]; v != 0 {
		t.Fatalf("drag past the start: value %v, want 0", v)
	}

	// After the release, a move no longer changes the value.
	send(w, gui.EventMouseUp, s.X-50, y)
	send(w, gui.EventMouseMove, xAt(s, dragInset, 50), y)
	if v := app.value[tr.name]; v != 0 {
		t.Fatalf("move after release: value %v, want 0", v)
	}
}

// Arrow keys step the value; Home and End jump to the ends. As on gui.Slider,
// Right and Down increase it.
func TestKeys(t *testing.T) {
	app, w := newTestApp(t)
	const id = "page:xp"
	start := app.value["XP"]
	steps := []struct {
		key  gui.KeyCode
		want float32
	}{
		{gui.KeyRight, start + keyStep},
		{gui.KeyDown, start + 2*keyStep},
		{gui.KeyLeft, start + keyStep},
		{gui.KeyEnd, valueMax},
		{gui.KeyRight, valueMax},
		{gui.KeyHome, 0},
		{gui.KeyUp, 0},
	}
	for _, st := range steps {
		if err := w.TestKey(id, st.key, gui.ModNone); err != nil {
			t.Fatal(err)
		}
		if v := app.value["XP"]; !near(v, st.want) {
			t.Fatalf("after key %v value %v, want %v", st.key, v, st.want)
		}
	}
}

// The knob and the fill follow the value, and the slider keeps its size.
func TestPartsFollowValue(t *testing.T) {
	app, w := newTestApp(t)
	tr := tracks[0] // Display
	const id = "page:apple:display"
	for _, v := range []float32{0, 50, valueMax} {
		app.value[tr.name] = v
		s := mustFind(t, w, id)
		knob := mustFind(t, w, id+":knob")
		fill := mustFind(t, w, id+":fill")
		if s.Width != appleW || s.Height != appleH {
			t.Fatalf("value %v: slider %vx%v, want %vx%v", v, s.Width, s.Height, appleW, appleH)
		}
		travel := v / valueMax * (appleW - appleH)
		if got := knob.X - s.X; !near(got, travel) {
			t.Fatalf("value %v: knob at %v, want %v", v, got, travel)
		}
		// The fill ends at the knob's center.
		if got, want := fill.Width, travel+appleH/2; !near(got, want) {
			t.Fatalf("value %v: fill width %v, want %v", v, got, want)
		}
	}
}

// The press state stays on while the drag runs, and hover lights the XP
// handle also when the pointer is over a floating part (#661).
func TestPressAndHoverLooks(t *testing.T) {
	app, w := newTestApp(t)
	tr := tracks[4] // XP
	s := mustFind(t, w, "page:xp")
	h := mustFind(t, w, "page:xp:handle")
	y := s.Y + s.Height/2

	send(w, gui.EventMouseMove, h.X+h.Width/2, y)
	if !w.IsHovered("page:xp") {
		t.Fatal("pointer over the floating handle: slider not hovered")
	}

	send(w, gui.EventMouseDown, xAt(s, xpHandleW/2, app.value[tr.name]), y)
	send(w, gui.EventMouseMove, s.X+s.Width+100, y)
	if !w.IsPressed("page:xp") {
		t.Fatal("drag outside the slider: slider not pressed")
	}
	send(w, gui.EventMouseUp, s.X+s.Width+100, y)
	if w.IsPressed("page:xp") {
		t.Fatal("after release: slider still pressed")
	}
}

// The wheel moves the value one unit per line, clamped to the range.
func TestWheel(t *testing.T) {
	app, w := newTestApp(t)
	start := app.value["XP"]
	if err := w.TestScroll("page:xp", 0, 3); err != nil {
		t.Fatal(err)
	}
	if v := app.value["XP"]; v == start {
		t.Fatalf("wheel did not move the value from %v", start)
	}
}

// Every slider is focusable, has the slider role, and the window has no
// duplicate IDs or debug findings.
func TestInteractiveLooksClean(t *testing.T) {
	_, w := newTestApp(t)
	for _, id := range []string{"page:apple:display", "page:apple:sound",
		"page:material:call", "page:material:media", "page:xp"} {
		s := mustFind(t, w, id)
		if !s.Focusable || s.A11YRole != gui.AccessRoleSlider {
			t.Fatalf("%s: focusable=%v role=%v", id, s.Focusable, s.A11YRole)
		}
	}
	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Fatalf("duplicate IDs: %v", dups)
	}
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("debug findings: %v", found)
	}
}
