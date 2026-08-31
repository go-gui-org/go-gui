package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:       &App{Decoration: gui.DecorationNone},
		Width:       440,
		Height:      280,
		Decorations: gui.DecorationNone,
	})
	// Both window-gesture calls are no-ops with a nil native platform.
	w.StartWindowDrag()
	w.StartWindowResize(gui.EdgeBottomRight)
	_ = mainView(w).GenerateLayout(w)
}

func TestHiddenTitlebarLabel(t *testing.T) {
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:       &App{Decoration: gui.DecorationHiddenTitlebar},
		Width:       440,
		Height:      280,
		Decorations: gui.DecorationHiddenTitlebar,
	})
	_ = mainView(w).GenerateLayout(w)
}
