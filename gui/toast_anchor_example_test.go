package gui_test

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// This file proves an app outside the gui package can set where toasts
// stack (#1103). It also pins the dx-cheat-sheet section that quotes
// it (examples/showcase/sound_snippet_test.go).

// doc:snippet-begin toast-anchor
//
// useTopLeftToasts makes toasts stack in the top-left corner. The
// anchor is part of ThemeCfg, so a theme built again from its Cfg
// (WithPadding, WithBorders, AdjustFontSize) keeps it.

func useTopLeftToasts() {
	cfg := gui.ThemeDark.Cfg
	cfg.ToastAnchor = gui.Some(gui.ToastTopLeft)
	gui.SetTheme(gui.ThemeMaker(cfg))
}

// doc:snippet-end toast-anchor

func TestToastAnchorExample(t *testing.T) {
	// SetTheme changes the app default theme, so put back ThemeDark,
	// the default that init sets, for the tests that run after this
	// one. Do not save gui.CurrentTheme() here: it is the theme that
	// was installed last, which can be the theme of another test's
	// window, not the app default.
	t.Cleanup(func() { gui.SetTheme(gui.ThemeDark) })

	useTopLeftToasts()
	got, set := gui.CurrentTheme().Cfg.ToastAnchor.Value()
	if !set || got != gui.ToastTopLeft {
		t.Errorf("ToastAnchor = %d (set %v), want ToastTopLeft", got, set)
	}
}
