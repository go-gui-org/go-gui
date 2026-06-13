package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Last: "Click a shape."},
		Width:  720,
		Height: 480,
	})
	_ = view(w).GenerateLayout(w)
}
