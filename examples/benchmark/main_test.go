package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{WidgetCount: 100, WidgetType: typeButton, Running: true},
		Width:  1024,
		Height: 768,
	})
	_ = benchView(w).GenerateLayout(w)
}
