package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Status: "Running."},
		Width:  500,
		Height: 300,
	})
	_ = mainView(w).GenerateLayout(w)
}

// Issue #1273: "Quit" in the tray menu must close every window. A quit
// request would only run the hide-on-close hook and leave the app
// running with no window showing. Removing the tray is not checked
// here: a test outside gui has no platform to create one.
func TestQuitFromTrayClosesEveryWindow(t *testing.T) {
	t.Parallel()
	app := gui.NewApp()
	app.ExitMode = gui.ExitOnTrayRemoved
	hideOnClose := func(w *gui.Window) { w.Hide() }
	first := gui.NewWindow(gui.WindowCfg{State: &App{}, OnCloseRequest: hideOnClose})
	second := gui.NewWindow(gui.WindowCfg{State: &App{}, OnCloseRequest: hideOnClose})
	app.Register(1, first)
	app.Register(2, second)

	quitFromTray(app, nil)

	if !first.CloseRequested() {
		t.Error("the first window was not asked to close")
	}
	if !second.CloseRequested() {
		t.Error("the second window was not asked to close")
	}
}

// With a tray, closing hides the window and keeps it alive for the
// tray menu's "Show Window".
func TestCloseHidesToTheTray(t *testing.T) {
	t.Parallel()
	w := gui.NewWindow(gui.WindowCfg{State: &App{HasTray: true}})

	hideToTrayOrClose(w)

	if w.IsVisible() {
		t.Error("the window was not hidden")
	}
	if w.CloseRequested() {
		t.Error("the window was closed, so the tray could not show it again")
	}
}

// Issue #1273: without a tray nothing could bring a hidden window back,
// so closing must close. The other case that closes, a platform that
// cannot hide a window, needs a real backend and is checked by hand.
func TestCloseWithoutTrayCloses(t *testing.T) {
	t.Parallel()
	w := gui.NewWindow(gui.WindowCfg{State: &App{}})

	hideToTrayOrClose(w)

	if !w.CloseRequested() {
		t.Error("the window was not closed")
	}
	if !w.IsVisible() {
		t.Error("the window was hidden with no tray to show it again")
	}
}
