package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:       &App{},
		Width:       380,
		Height:      240,
		Transparent: true,
	})
	// Transparent is honoured at window creation by the backends; with a
	// nil native platform it only has to reach the clear color.
	if got := w.FrameBackground(); got != gui.ColorTransparent {
		t.Errorf("FrameBackground() = %+v, want fully clear", got)
	}
	_ = mainView(w).GenerateLayout(w)
}
