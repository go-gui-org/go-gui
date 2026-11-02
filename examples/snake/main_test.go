package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The start button sits in the landing Canvas. A Fit button there was
// never measured, so it was 0x0 and a click reached nothing (#1216).
func TestStartButtonStartsTheGame(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)

	app := &App{Game: NewGame(gridWidth, gridHeight, nil)}
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 560, Height: 740})
	w.TestRender(mainView)

	if err := w.TestClick(startButtonID); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if !app.Started {
		t.Error("game not started after clicking the start button")
	}
}
