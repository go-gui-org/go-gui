package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The game-over overlay sits in the board Canvas. Its Fit button was
// never measured, so it was 0x0 and a click reached nothing (#1216).
func TestTryAgainRestartsTheGame(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)

	g := NewGame()
	g.State = StateGameOver
	app := &App{Game: g, Screen: ScreenPlaying}
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 600, Height: 700})
	w.TestRender(mainView)

	if err := w.TestClick("g2048_try_again"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if g.State != StatePlaying {
		t.Errorf("game state = %v after Try again, want StatePlaying", g.State)
	}
}
