package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newTestWindow() *gui.Window {
	gui.SetTheme(gui.ThemeDark)
	return gui.NewWindow(gui.WindowCfg{State: &App{}, Width: 520, Height: 300})
}

func TestMainViewNoPanic(t *testing.T) {
	w := newTestWindow()
	_ = mainView(w).GenerateLayout(w)
}

// The labels follow the getters, so they stay right when the user
// changes the state through the OS.
func TestViewFollowsState(t *testing.T) {
	w := newTestWindow()
	gui.DispatchWindowState(w, true, true)
	if !w.IsMaximized() || !w.IsFullscreen() {
		t.Fatal("dispatch did not reach the getters")
	}
	_ = mainView(w).GenerateLayout(w)
}

// Every request is a no-op with a nil native platform.
func TestRequestsWithoutPlatform(t *testing.T) {
	w := newTestWindow()
	w.Minimize()
	w.Maximize()
	w.Restore()
	w.SetFullscreen(true)
	w.SetWindowSize(640, 420)
	moveWindow(w)
}
