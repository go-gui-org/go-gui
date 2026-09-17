package toggles

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestApp renders the example once.
// Not t.Parallel in callers: SetTheme mutates process-global theme state.
func newTestApp(t *testing.T) (*App, *gui.Window) {
	t.Helper()
	gui.SetTheme(gui.ThemeLight)
	app := New()
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 640, Height: 700})
	w.TestRender(func(*gui.Window) gui.View { return View(app) })
	return app, w
}

func mustFind(t *testing.T, w *gui.Window, id string) *gui.Layout {
	t.Helper()
	ly, ok := w.TestRender(nil).FindByID(id)
	if !ok {
		t.Fatalf("no widget %q", id)
	}
	return ly
}

// settle runs two frames: the first records what the pointer is over, the
// second builds the look from it. The real frame loop does both in one frame.
func settle(w *gui.Window) {
	w.TestRender(nil)
	w.TestRender(nil)
}

func moveTo(w *gui.Window, x, y float32) {
	w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: x, MouseY: y})
	settle(w)
}

// A click flips the value, writes the log and updates the checked state
// that a screen reader reads.
func TestClickFlips(t *testing.T) {
	app, w := newTestApp(t)
	const id = "page:green:b"
	if s := mustFind(t, w, id).Shape; s.A11YState != gui.AccessStateNone {
		t.Fatalf("idle state %v, want none", s.A11YState)
	}
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	if !app.on["Green B"] || app.log != "Green B → on" {
		t.Fatalf("after click on=%v log=%q", app.on["Green B"], app.log)
	}
	if s := mustFind(t, w, id).Shape; s.A11YState != gui.AccessStateChecked {
		t.Fatalf("clicked state %v, want checked", s.A11YState)
	}
}

// Space and Enter flip a focused switch, as they press a button.
func TestKeyboardFlips(t *testing.T) {
	app, w := newTestApp(t)
	const id = "page:check:b"
	if err := w.TestKey(id, gui.KeySpace, gui.ModNone); err != nil {
		t.Fatal(err)
	}
	if !app.on["Check B"] {
		t.Fatal("Space did not flip the switch on")
	}
	if err := w.TestKey(id, gui.KeyEnter, gui.ModNone); err != nil {
		t.Fatal(err)
	}
	if app.on["Check B"] {
		t.Fatal("Enter did not flip the switch off")
	}
}

// Turning a switch on moves its knob to the right end of the track, and the
// switch keeps its outer size.
func TestKnobMovesInsideFixedBounds(t *testing.T) {
	_, w := newTestApp(t)
	const id = "page:material:bluetooth"
	before := *mustFind(t, w, id).Shape
	knobX := func() float32 { return mustFind(t, w, id).Children[0].Shape.X }
	offX := knobX()
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	after := mustFind(t, w, id).Shape
	if after.Width != before.Width || after.Height != before.Height {
		t.Fatalf("flip resized the switch: %vx%v → %vx%v",
			before.Width, before.Height, after.Width, after.Height)
	}
	if onX := knobX(); onX <= offX {
		t.Fatalf("knob x %v after turning on, want right of %v", onX, offX)
	}
}

// Hover shades the track; moving away restores it.
func TestHoverShadesTrack(t *testing.T) {
	_, w := newTestApp(t)
	const id = "page:green:a"
	s := mustFind(t, w, id).Shape
	if s.Color != greenOn {
		t.Fatalf("idle track %+v, want %+v", s.Color, greenOn)
	}
	moveTo(w, s.X+s.Width/2, s.Y+s.Height/2)
	if c := mustFind(t, w, id).Shape.Color; c == greenOn {
		t.Fatal("hover did not shade the track")
	}
	moveTo(w, 1, 1)
	if c := mustFind(t, w, id).Shape.Color; c != greenOn {
		t.Fatalf("track %+v after the pointer left, want %+v", c, greenOn)
	}
}

// The labelled switch floats its knob over the track. The pointer over the
// knob is still over the switch, so the track stays shaded. This needs the
// knob's ID: see labelToggle. A float leaves
// its parent's children after arrange, so the knob center is computed from
// the switch: "Label A" is on, so the 28px knob sits 1px from the right end.
func TestHoverOverFloatingKnob(t *testing.T) {
	_, w := newTestApp(t)
	const id = "page:label:a"
	sw := mustFind(t, w, id)
	idle := sw.Children[0].Shape.Color
	moveTo(w, sw.Shape.X+sw.Shape.Width-1-14, sw.Shape.Y+sw.Shape.Height/2)
	if c := mustFind(t, w, id).Children[0].Shape.Color; c == idle {
		t.Fatal("pointer over the floating knob does not count as hover")
	}
}

func TestInteractiveLooksClean(t *testing.T) {
	_, w := newTestApp(t)
	if d := w.TestDuplicateIDs(); len(d) != 0 {
		t.Fatalf("duplicate IDs: %v", d)
	}
	if found := w.TestFindings(gui.DebugMissingIDs | gui.DebugDuplicates); len(found) != 0 {
		t.Fatalf("findings: %v", found)
	}
}
