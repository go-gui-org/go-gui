package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Material: gui.VibrancyUnderWindow},
		Width:  360,
		Height: 260,
	})
	// SetWindowVibrancy is a no-op with a nil native platform (tests).
	w.SetWindowVibrancy(gui.VibrancyUnderWindow)
	_ = mainView(w).GenerateLayout(w)
}
