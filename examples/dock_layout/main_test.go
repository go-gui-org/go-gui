package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Root: initialLayout()},
		Width:  1000,
		Height: 700,
	})
	_ = mainView(w).GenerateLayout(w)

}
