package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Color: gui.RGBA(0x3D, 0x81, 0x7C, 255), ShowHSV: true},
		Width:  300,
		Height: 490,
	})
	_ = mainView(w).GenerateLayout(w)

}
