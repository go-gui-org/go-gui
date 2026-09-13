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
