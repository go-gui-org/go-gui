package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The reset button sits in the header Canvas. A Fit button there was
// never measured, so it was 0x0 and a click reached nothing (#1216).
func TestResetButtonResetsTheGame(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)

	rows, cols, mines := DiffBeginner.Config()
	app := &App{
		Game:        NewGame(rows, cols, mines, false, nil),
		Difficulty:  DiffBeginner,
		Screen:      ScreenPlaying,
		ElapsedSecs: 42,
	}
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 740, Height: 660})
	w.TestRender(mainView)

	if err := w.TestClick("mine_reset"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if app.ElapsedSecs != 0 {
		t.Errorf("ElapsedSecs = %d after reset, want 0", app.ElapsedSecs)
	}
}
