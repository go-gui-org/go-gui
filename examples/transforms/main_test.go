package main

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newWindow(t *testing.T) *gui.Window {
	t.Helper()
	w := gui.NewTestWindow(t, gui.WindowCfg{
		State:  &app{},
		Width:  640,
		Height: 300,
	})
	w.TestRender(mainView)
	return w
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.5 }

func TestMainViewNoFindings(t *testing.T) {
	w := newWindow(t)
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// The card under the pointer lifts through an animation, and the row
// that takes the hover stays put, so the pointer stays on it.
func TestLiftOnHover(t *testing.T) {
	w := newWindow(t)
	cardID := gui.ScopeID("lift", "card")
	_, y0, ok := w.LocalToWindow(cardID, 0, 0)
	if !ok {
		t.Fatal("card not on screen")
	}
	mx, my, _ := w.LocalToWindow("lift", cardW/2, cardH-2)
	w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: mx, MouseY: my})
	w.TestRender(nil)
	if !w.IsHovered("lift") {
		t.Error("lift row not hovered at its bottom edge")
	}
	if !w.HasAnimation("lift") {
		t.Fatal("hover did not start the lift animation")
	}
	// Animation time does not advance in a test: jump to the end.
	gui.State[app](w).lift = 1
	w.TestRender(nil)
	_, y1, ok := w.LocalToWindow(cardID, 0, 0)
	if !ok {
		t.Fatal("card not on screen after hover")
	}
	if y1 >= y0-4 {
		t.Errorf("card top = %v after hover, want above %v", y1, y0-4)
	}
}

// LocalToWindow and WindowToLocal undo each other through the badge's
// turn, and the turn tilts the badge's left edge.
func TestBadgeCornerRoundTrip(t *testing.T) {
	w := newWindow(t)
	x, y, ok := w.LocalToWindow("badge", 0, 0)
	if !ok {
		t.Fatal("badge corner not on screen")
	}
	lx, ly, ok := w.WindowToLocal("badge", x, y)
	if !ok || !near(lx, 0) || !near(ly, 0) {
		t.Errorf("round trip = (%v, %v, %v), want (0, 0, true)", lx, ly, ok)
	}
	// Unturned, the left edge would be upright: both ends at one x.
	x2, _, _ := w.LocalToWindow("badge", 0, 20)
	if near(x, x2) {
		t.Errorf("badge left edge is upright at x %v: not turned", x)
	}
}

// A click on the turned badge reaches it and reports the window point.
func TestBadgeClick(t *testing.T) {
	w := newWindow(t)
	if err := w.TestClick("badge"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if gui.State[app](w).badgeHit == "" {
		t.Fatal("badge click not reported")
	}
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// A click starts the flip. Once the card has turned 180 degrees, the
// front face is hidden, and the back face is shown where the front was.
func TestFlipShowsBack(t *testing.T) {
	w := newWindow(t)
	frontID := gui.ScopeID("flip", "front")
	backID := gui.ScopeID("flip", "back")
	fx, fy, ok := w.LocalToWindow(frontID, 0, 0)
	if !ok {
		t.Fatal("front hidden before the flip")
	}
	if _, _, ok = w.LocalToWindow(backID, 0, 0); ok {
		t.Fatal("back shown before the flip")
	}
	if err := w.TestClick("flip"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if !gui.State[app](w).flipped || !w.HasAnimation("flip") {
		t.Fatal("click did not start the flip")
	}
	// Animation time does not advance in a test: jump to the end.
	gui.State[app](w).angle = 180
	w.TestRender(nil)
	if _, _, ok = w.LocalToWindow(frontID, 0, 0); ok {
		t.Error("front shown after the flip")
	}
	bx, by, ok := w.LocalToWindow(backID, 0, 0)
	if !ok {
		t.Fatal("back hidden after the flip")
	}
	if !near(bx, fx) || !near(by, fy) {
		t.Errorf("back corner = (%v, %v), want the front's (%v, %v)",
			bx, by, fx, fy)
	}
	// The card still takes the click that flips it back.
	if err := w.TestClick("flip"); err != nil {
		t.Fatalf("TestClick back: %v", err)
	}
	if gui.State[app](w).flipped {
		t.Error("second click did not flip back")
	}
}
