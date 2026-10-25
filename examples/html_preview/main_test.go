package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Source: sampleHTML},
		Width:  900,
		Height: 500,
	})
	_ = mainView(w).GenerateLayout(w)
}
