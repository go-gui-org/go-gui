package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{WeekdaysLen: "one"},
		Width:  1200,
		Height: 950,
	})
	_ = mainView(w).GenerateLayout(w)

}
