package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDarkBordered)
	app := &App{}
	presetFountain(app)
	app.EmitterX = canvasW / 2
	app.EmitterY = float32(windowH) * 0.7
	app.Particles = make([]Particle, 0, maxParticles)
	w := gui.NewWindow(gui.WindowCfg{
		State:  app,
		Width:  windowW,
		Height: windowH,
	})
	layout := gui.GenerateViewLayout(mainView(w), w)
	if len(layout.Children) == 0 {
		t.Fatal("expected non-empty layout")
	}
}
