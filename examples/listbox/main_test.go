package main

import (
	"fmt"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	items := make([]gui.ListBoxOption, 0, 10)
	for i := 1; i <= 10; i++ {
		id := fmt.Sprintf("%05d", i)
		items = append(items, gui.NewListBoxOption(id, id+" text", id))
	}
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Items: items},
		Width:  240,
		Height: 420,
	})
	_ = mainView(w).GenerateLayout(w)

}
