package main

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestApp renders the example once.
// Not t.Parallel in callers: SetTheme mutates process-global theme state.
func newTestApp(t *testing.T) (*App, *gui.Window) {
	t.Helper()
	gui.SetTheme(gui.ThemeLight)
	app := newApp()
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 720, Height: 620})
	w.TestRender(mainView)
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

// center returns the middle of the widget, in window coordinates.
func center(t *testing.T, w *gui.Window, id string) (float32, float32) {
	t.Helper()
	s := mustFind(t, w, id).Shape
	return s.X + s.Width/2, s.Y + s.Height/2
}

// settle runs two frames: the first records what the pointer is over, the
// second builds the look from it. The real frame loop does both in one frame.
func settle(w *gui.Window) {
	w.TestRender(nil)
	w.TestRender(nil)
}

// win98Face is the innermost container of a 98 button: frame → shadow →
// light → face.
func win98FaceOf(t *testing.T, w *gui.Window, id string) *gui.Shape {
	t.Helper()
	return mustFind(t, w, id).Children[0].Children[0].Children[0].Shape
}

// A press moves the 98 label down and right by 1px and keeps the outer size.
func TestWin98PressShiftsLabel(t *testing.T) {
	_, w := newTestApp(t)
	before := *mustFind(t, w, "page:win98:ok").Shape
	if p := win98FaceOf(t, w, "page:win98:ok").Padding; p.Top != 5 || p.Left != 14 {
		t.Fatalf("idle padding %+v, want top 5 left 14", p)
	}

	x, y := center(t, w, "page:win98:ok")
	w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: x, MouseY: y})
	w.EventFn(&gui.Event{Type: gui.EventMouseDown, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	settle(w)

	if p := win98FaceOf(t, w, "page:win98:ok").Padding; p.Top != 6 || p.Left != 15 {
		t.Fatalf("pressed padding %+v, want top 6 left 15", p)
	}
	after := mustFind(t, w, "page:win98:ok").Shape
	if after.Width != before.Width || after.Height != before.Height {
		t.Fatalf("press resized the button: %vx%v → %vx%v",
			before.Width, before.Height, after.Width, after.Height)
	}

	w.EventFn(&gui.Event{Type: gui.EventMouseUp, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	settle(w)
	if p := win98FaceOf(t, w, "page:win98:ok").Padding; p.Top != 5 {
		t.Fatalf("released padding top %v, want 5", p.Top)
	}
}

// Hover lights the XP gold rim; moving away clears it. The rim sits inside
// the blue frame and the gradient body: frame → body → rim.
func TestXPHoverLightsRim(t *testing.T) {
	_, w := newTestApp(t)
	rimColor := func() gui.Color { return mustFind(t, w, "page:xp:ok").Children[0].Children[0].Shape.Color }
	if rimColor() != gui.ColorTransparent {
		t.Fatalf("idle rim %+v, want transparent", rimColor())
	}
	x, y := center(t, w, "page:xp:ok")
	w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: x, MouseY: y})
	settle(w)
	if rimColor() != xpGoldOuter {
		t.Fatalf("hovered rim %+v, want gold %+v", rimColor(), xpGoldOuter)
	}
	w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: 1, MouseY: 1})
	settle(w)
	if rimColor() != gui.ColorTransparent {
		t.Fatalf("rim still lit after the pointer left: %+v", rimColor())
	}
}

// A disabled button never lights up.
func TestXPDisabledStaysDark(t *testing.T) {
	_, w := newTestApp(t)
	x, y := center(t, w, "page:xp:disabled")
	w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: x, MouseY: y})
	settle(w)
	if c := mustFind(t, w, "page:xp:disabled").Children[0].Children[0].Shape.Color; c != gui.ColorTransparent {
		t.Fatalf("disabled rim %+v, want transparent", c)
	}
}

func TestClickWritesLog(t *testing.T) {
	app, w := newTestApp(t)
	for id, want := range map[string]string{
		"page:win98:ok":     "98 OK",
		"page:xp:apply":     "XP Apply",
		"page:flat:outline": "flat Outline",
	} {
		if err := w.TestClick(id); err != nil {
			t.Fatalf("TestClick(%q) = %v", id, err)
		}
		if !strings.Contains(app.log, want) {
			t.Fatalf("after clicking %q log = %q, want %q", id, app.log, want)
		}
	}
}

func TestNoDuplicateIDs(t *testing.T) {
	_, w := newTestApp(t)
	if d := w.TestDuplicateIDs(); len(d) != 0 {
		t.Fatalf("duplicate IDs: %v", d)
	}
}
