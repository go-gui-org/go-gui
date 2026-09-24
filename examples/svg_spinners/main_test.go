package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	// Not parallel: see TestIsolatedViewNoPanic — shared theme state.
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		Width:  520,
		Height: 640,
	})
	_ = mainView(w).GenerateLayout(w)

}
