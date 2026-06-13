package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{LastAction: "Ready."},
		Width:  600,
		Height: 400,
	})
	_ = mainView(w).GenerateLayout(w)
}
