package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Width:  900,
		Height: 540,
	})
	_ = view(w).GenerateLayout(w)
}
