package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	w := gui.NewWindow(gui.WindowCfg{
		State:  &app{},
		Width:  600,
		Height: 500,
	})
	_ = mainView(w).GenerateLayout(w)

}
