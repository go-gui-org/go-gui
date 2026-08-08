package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Demonstrates the app-testing API on the thing this example exists to
// show: that OnKeyDown and OnKeyUp both fire, and are counted
// separately. A no-panic render could not tell the two apart.
func TestKeyPressCountsBothEdges(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	app := &App{}
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 400, Height: 300})
	w.TestRender(mainView)

	// TestKey delivers a press and a release, so one call should move
	// both counters by one.
	if err := w.TestKey("kud_input", gui.KeyA, 0); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if app.keyDownCount != 1 || app.keyUpCount != 1 {
		t.Fatalf("after one key: down=%d up=%d, want 1 and 1",
			app.keyDownCount, app.keyUpCount)
	}
	if app.lastKeyDown != gui.KeyA || app.lastKeyUp != gui.KeyA {
		t.Fatalf("last keys: down=%v up=%v, want KeyA on both",
			app.lastKeyDown, app.lastKeyUp)
	}
}
