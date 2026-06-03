package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeLightNoPadding)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{LightTheme: true},
		Width:  800,
		Height: 800,
	})
	layout := gui.GenerateViewLayout(mainView(w), w)
	if len(layout.Children) == 0 {
		t.Fatal("expected non-empty layout")
	}
}
