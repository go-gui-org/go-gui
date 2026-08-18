package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func TestRendersToPNG(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Clicks: 3},
		Width:  320,
		Height: 200,
		OnInit: func(win *gui.Window) { win.UpdateView(mainView) },
	})
	t.Cleanup(func() { soft.Release(w) })

	path := filepath.Join(t.TempDir(), "shot.png")
	if err := soft.RenderToPNG(w, 2, path); err != nil {
		t.Fatalf("RenderToPNG: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("empty PNG")
	}
}
