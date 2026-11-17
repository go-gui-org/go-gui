package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newTestWindow(t *testing.T) *gui.Window {
	t.Helper()
	gui.SetTheme(gui.ThemeDark)
	return gui.NewTestWindow(t, gui.WindowCfg{
		State: &App{}, Width: 520, Height: 360,
	})
}

func TestMainViewNoPanic(t *testing.T) {
	w := newTestWindow(t)
	_ = mainView(w).GenerateLayout(w)
}

// Each player holds its own request: starting a player twice takes one,
// and stopping one player leaves the other's.
func TestPlayersHoldTheirOwnRequests(t *testing.T) {
	w := newTestWindow(t)
	app := gui.State[App](w)

	setPlaying(w, 0, true)
	setPlaying(w, 0, true)
	setPlaying(w, 1, true)
	if n := app.held(); n != 2 {
		t.Fatalf("held = %d after starting both players, want 2", n)
	}
	setPlaying(w, 0, false)
	if app.release[0] != nil || app.release[1] == nil {
		t.Fatal("stopping player 1 changed player 2")
	}
	setPlaying(w, 1, false)
	setPlaying(w, 1, false)
	if n := app.held(); n != 0 {
		t.Errorf("held = %d after stopping both, want 0", n)
	}
}

// A click on a switch starts its player, a second click stops it.
func TestSwitchClickTogglesPlayer(t *testing.T) {
	w := newTestWindow(t)
	app := gui.State[App](w)
	w.TestRender(mainView)

	if err := w.TestClick(playerID(1)); err != nil {
		t.Fatal(err)
	}
	if app.release[1] == nil || app.release[0] != nil {
		t.Fatal("a click on player 2's switch did not start player 2 alone")
	}
	w.TestRender(nil)
	if err := w.TestClick(playerID(1)); err != nil {
		t.Fatal(err)
	}
	if app.held() != 0 {
		t.Error("a second click did not stop player 2")
	}
}
