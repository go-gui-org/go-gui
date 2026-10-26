package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func TestSurfaceExampleFramesAndShutdown(t *testing.T) {
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Width: frameWidth, Height: frameHeight})
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 720, Height: 500, State: &appState{surface: surface}})
	w.SetView(mainView)
	pix := make([]byte, frameWidth*frameHeight*4)
	paintFrame(pix, 0)
	if err = surface.Update(pix, frameWidth*4); err != nil {
		t.Fatal(err)
	}
	first, err := soft.RenderToImage(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	paintFrame(pix, 30)
	if err = surface.Update(pix, frameWidth*4); err != nil {
		t.Fatal(err)
	}
	second, err := soft.RenderToImage(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.Pix, second.Pix) {
		t.Error("animated marker did not move in rendered output")
	}
	stop, done := make(chan struct{}), make(chan struct{})
	close(stop)
	go animateSurface(surface, pix, stop, done)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("producer did not stop")
	}
}
