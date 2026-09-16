package checkboxes

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
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 680, Height: 600})
	w.TestRender(func(w *gui.Window) gui.View { return View(w, app) })
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

// A click on the label toggles the box: the label is part of the row that
// carries the ID and the click handler.
func TestClickLabelToggles(t *testing.T) {
	app, w := newTestApp(t)
	const id = "page:material:wifi"
	label := mustFind(t, w, id).Children[1].Shape
	x, y := label.X+label.Width/2, label.Y+label.Height/2
	w.EventFn(&gui.Event{Type: gui.EventMouseDown, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	w.EventFn(&gui.Event{Type: gui.EventMouseUp, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	settle(w)
	if !app.on["Connect to Wi-Fi automatically"] {
		t.Fatal("click on the label did not check the box")
	}
	if s := mustFind(t, w, id).Shape; s.A11YState != gui.AccessStateChecked {
		t.Fatalf("a11y state %v, want checked", s.A11YState)
	}
}

// Space toggles a focused checkbox.
func TestSpaceToggles(t *testing.T) {
	app, w := newTestApp(t)
	if err := w.TestKey("page:xp:sounds", gui.KeySpace, gui.ModNone); err != nil {
		t.Fatal(err)
	}
	if !app.on["Play Windows startup sound"] {
		t.Fatal("Space did not check the box")
	}
}

// A disabled checkbox ignores clicks, and the "disable" switch takes the
// target from enabled to disabled.
func TestDisabledIgnoresClick(t *testing.T) {
	app, w := newTestApp(t)
	if err := w.TestClick("page:material-toggle:target"); err != nil {
		t.Fatal(err)
	}
	if app.on["Material target"] {
		t.Fatal("enabled target did not toggle")
	}
	if err := w.TestClick("page:material-toggle:disable"); err != nil {
		t.Fatal(err)
	}
	if !mustFind(t, w, "page:material-toggle:target").Shape.Disabled {
		t.Fatal("target not disabled after the switch")
	}
	// TestClick refuses a disabled target, so press it by position.
	s := mustFind(t, w, "page:material-toggle:target").Shape
	x, y := s.X+s.Width/2, s.Y+s.Height/2
	w.EventFn(&gui.Event{Type: gui.EventMouseDown, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	w.EventFn(&gui.Event{Type: gui.EventMouseUp, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	settle(w)
	if app.on["Material target"] {
		t.Fatal("disabled target toggled")
	}
}

// xpRim is the outer gold rim of an XP box: row → frame → face → rim.
func xpRim(t *testing.T, w *gui.Window, id string) gui.Color {
	t.Helper()
	return mustFind(t, w, id).Children[0].Children[0].Children[0].Shape.Color
}

// Hover lights the XP gold rim; a press dims the face and turns the rim off;
// moving away clears it. The box keeps its size throughout.
func TestXPHoverAndPress(t *testing.T) {
	_, w := newTestApp(t)
	const id = "page:xp:options"
	before := *mustFind(t, w, id).Children[0].Shape
	if c := xpRim(t, w, id); c != gui.ColorTransparent {
		t.Fatalf("idle rim %+v, want transparent", c)
	}
	s := mustFind(t, w, id).Shape
	x, y := s.X+s.Width/2, s.Y+s.Height/2
	moveTo(w, x, y)
	if c := xpRim(t, w, id); c != xpGoldLo {
		t.Fatalf("hovered rim %+v, want gold %+v", c, xpGoldLo)
	}
	w.EventFn(&gui.Event{Type: gui.EventMouseDown, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	settle(w)
	if c := xpRim(t, w, id); c != gui.ColorTransparent {
		t.Fatalf("pressed rim %+v, want transparent", c)
	}
	after := mustFind(t, w, id).Children[0].Shape
	if after.Width != before.Width || after.Height != before.Height {
		t.Fatalf("press resized the box: %vx%v → %vx%v",
			before.Width, before.Height, after.Width, after.Height)
	}
	w.EventFn(&gui.Event{Type: gui.EventMouseUp, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	moveTo(w, 1, 1)
	if c := xpRim(t, w, id); c != gui.ColorTransparent {
		t.Fatalf("rim still lit after the pointer left: %+v", c)
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
