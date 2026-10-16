package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newWindow(t *testing.T) *gui.Window {
	t.Helper()
	w := gui.NewTestWindow(t, gui.WindowCfg{
		State:  newApp(),
		Width:  480,
		Height: 360,
	})
	w.TestRender(mainView)
	return w
}

// shape returns the arranged shape with the given effective ID.
func shape(t *testing.T, w *gui.Window, id string) *gui.Shape {
	t.Helper()
	ly, ok := w.TestRender(nil).FindByID(id)
	if !ok {
		t.Fatalf("%q not found", id)
	}
	return ly.Shape
}

func TestMainViewNoFindings(t *testing.T) {
	w := newWindow(t)
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// Every wrapper starts at the Stack's content origin, so only the
// wrapper's padding moves a card: 80 right and 50 down per card, measured
// from Red.
func TestCardsStepFromStackOrigin(t *testing.T) {
	w := newWindow(t)
	base := shape(t, w, gui.ScopeID("stage", "red"))
	for i, id := range []string{"red", "green", "blue"} {
		c := shape(t, w, gui.ScopeID("stage", id))
		dx, dy := c.X-base.X, c.Y-base.Y
		if dx != float32(i)*80 || dy != float32(i)*50 {
			t.Errorf("%s at (%.0f,%.0f) from red, want (%d,%d)",
				id, dx, dy, i*80, i*50)
		}
	}
}

// A click on the top card keeps the order.
func TestClickTopCard(t *testing.T) {
	w := newWindow(t)
	if err := w.TestClick(gui.ScopeID("stage", "blue")); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if got, want := gui.State[app](w).order, [3]int{0, 1, 2}; got != want {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

// Green covers the center of Red. A click there goes to Green, the upper
// layer, so Green comes to the front, not Red.
func TestClickOverlapGoesToUpperCard(t *testing.T) {
	w := newWindow(t)
	if err := w.TestClick(gui.ScopeID("stage", "red")); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if got, want := gui.State[app](w).order, [3]int{0, 2, 1}; got != want {
		t.Fatalf("order = %v, want %v (green on top)", got, want)
	}
}

// Raising Red puts it on top without moving it.
func TestRaiseKeepsPlace(t *testing.T) {
	w := newWindow(t)
	before := *shape(t, w, gui.ScopeID("stage", "red"))
	raise(gui.State[app](w), 0)
	after := shape(t, w, gui.ScopeID("stage", "red"))
	if after.X != before.X || after.Y != before.Y {
		t.Fatalf("red moved from (%.0f,%.0f) to (%.0f,%.0f)",
			before.X, before.Y, after.X, after.Y)
	}
}

func TestRaise(t *testing.T) {
	s := newApp()
	raise(s, 0)
	if want := [3]int{1, 2, 0}; s.order != want {
		t.Fatalf("order = %v, want %v", s.order, want)
	}
}

// A card index not in the order leaves the order unchanged.
func TestRaiseUnknownCard(t *testing.T) {
	s := newApp()
	raise(s, 7)
	if want := [3]int{0, 1, 2}; s.order != want {
		t.Fatalf("order = %v, want %v", s.order, want)
	}
}
