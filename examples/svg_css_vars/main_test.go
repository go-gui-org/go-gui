package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &appState{ThemeIdx: 0, StrokeBase: 2},
		Width:  640,
		Height: 480,
	})
	_ = view(w).GenerateLayout(w)
}
