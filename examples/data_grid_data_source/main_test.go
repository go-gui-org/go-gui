package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	app := &App{SimulateLatency: true}
	app.AllRows = makeRows(100)
	app.Columns = makeColumns()
	rebuildSource(app)
	w := gui.NewWindow(gui.WindowCfg{
		State:  app,
		Width:  1240,
		Height: 760,
	})
	_ = mainView(w).GenerateLayout(w)

}
