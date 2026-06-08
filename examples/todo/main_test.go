package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeLight.WithPadding(false))
	w := gui.NewWindow(gui.WindowCfg{
		State:  newAppState(),
		Width:  540,
		Height: 640,
	})
	_ = mainView(w).GenerateLayout(w)

}
