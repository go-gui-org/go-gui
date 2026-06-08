package main

import (
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Delay: 200 * time.Millisecond},
		Width:  800,
		Height: 600,
	})
	_ = mainView(w).GenerateLayout(w)

}
