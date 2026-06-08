package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDarkNoPadding)
	w := gui.NewWindow(gui.WindowCfg{
		State:  newCalculatorState(),
		Width:  275,
		Height: 475,
	})
	_ = mainView(w).GenerateLayout(w)

}
