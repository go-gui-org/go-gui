package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State: &App{
			Color:      gui.HSLA{H: 210, S: 0.7, L: 0.55, A: 0.85},
			ShowPacked: true,
		},
		Width:  920,
		Height: 760,
	})
	_ = mainView(w).GenerateLayout(w)
}
