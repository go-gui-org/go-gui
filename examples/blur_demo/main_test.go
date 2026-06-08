package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDarkNoPadding)
	w := gui.NewWindow(gui.WindowCfg{
		Width:  800,
		Height: 800,
	})
	_ = mainView(w).GenerateLayout(w)

}
