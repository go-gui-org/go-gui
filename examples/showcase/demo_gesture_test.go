package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The desktop backends send no touch events, so a trackpad or mouse
// wheel scroll is how the pad's square pans there. Deltas add up and
// each one bumps the canvas version so OnDraw runs again.
func TestGestureOnScrollPansSquare(t *testing.T) {
	app := &ShowcaseApp{}
	gestureOnScroll(app, &gui.Event{ScrollX: 3, ScrollY: -4})
	gestureOnScroll(app, &gui.Event{ScrollX: 2, ScrollY: 10})

	if app.GesturePadOffsetX != 5 || app.GesturePadOffsetY != 6 {
		t.Fatalf("offset = (%v, %v), want (5, 6)",
			app.GesturePadOffsetX, app.GesturePadOffsetY)
	}
	if app.GesturePadVersion != 2 {
		t.Fatalf("version = %d, want 2", app.GesturePadVersion)
	}
	if app.GesturePadLabel != "Scroll  offset (5, 6)" {
		t.Fatalf("label = %q", app.GesturePadLabel)
	}
}

// The pad takes its long press, so a finger that rests and then drags
// keeps the long press instead of letting it go to a pan (#1117). Each
// long press leaves one marker: Ended repeats the press point and adds
// none.
func TestGestureOnGestureTakesLongPress(t *testing.T) {
	app := &ShowcaseApp{}
	for _, phase := range []gui.GesturePhase{gui.GesturePhaseBegan, gui.GesturePhaseEnded} {
		e := &gui.Event{GestureType: gui.GestureLongPress, GesturePhase: phase,
			CentroidX: 10, CentroidY: 20}
		if !gestureOnGesture(app, e) {
			t.Fatalf("phase %v: long press not taken", phase)
		}
	}
	if len(app.GesturePadMarkers) != 1 || !app.GesturePadMarkers[0].LongPress {
		t.Fatalf("markers = %v, want one long-press marker", app.GesturePadMarkers)
	}
	if gestureOnGesture(app, &gui.Event{GestureType: gui.GestureTap}) {
		t.Fatal("tap taken: only the long press holds the finger")
	}
}
