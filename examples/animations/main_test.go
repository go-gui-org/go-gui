package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		State: &State{
			SidebarWidth: 200,
			BoxX:         50,
			SpringValue:  100,
		},
		Width:  800,
		Height: 600,
	})
	_ = mainView(w).GenerateLayout(w)

}
