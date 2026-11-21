package main

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func newTestState(t *testing.T) (*App, *gui.PixelSurface) {
	t.Helper()
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Planes: nv12Planes})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(surface.Close)
	return &App{lent: new(atomic.Bool),
		shader: &gui.Shader{GLSL: nv12GLSL, Metal: nv12Metal, Surface: surface}}, surface
}

func TestFramesReachTheShader(t *testing.T) {
	state, surface := newTestState(t)
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 720, Height: 520, State: state})
	w.SetView(mainView)
	w.FrameFn()
	found, note := false, ""
	for _, cmd := range w.Renderers() {
		if cmd.Kind == gui.RenderCustomShader && cmd.DrawnSurface() == surface {
			found = true
		}
		if cmd.Kind == gui.RenderText && len(cmd.Text) > 20 {
			note = cmd.Text
		}
	}
	if !found {
		t.Error("the frame container did not draw through the shader with its surface")
	}
	// A test window has no GPU backend, so the view says so.
	if note != "This renderer runs no shaders, so the frame area stays empty" {
		t.Errorf("note = %q", note)
	}
}

// A window that cannot import DMA-BUFs gets every frame copied, even with
// GPU buffers at hand, and the producer stops when told.
func TestProducerCopiesWithoutImport(t *testing.T) {
	_, surface := newTestState(t)
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 720, Height: 520, State: new(int)})
	gpu, _ := newGPUFrames(3) // nil without a GPU; unused either way
	if gpu != nil {
		t.Cleanup(gpu.close)
	}
	var version uint64
	surface.ReadPlanes(0, func(_ []gui.SurfacePlane, v uint64) { version = v })
	stop, done := make(chan struct{}), make(chan struct{})
	lent := new(atomic.Bool)
	go produce(w, surface, gpu, newCPUFrame(), lent, stop, done)
	deadline := time.Now().Add(5 * time.Second)
	published := false
	for !published && time.Now().Before(deadline) {
		surface.ReadPlanes(version, func([]gui.SurfacePlane, uint64) { published = true })
		time.Sleep(5 * time.Millisecond)
	}
	close(stop)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("producer did not stop")
	}
	if !published || lent.Load() {
		t.Errorf("copied frame published %v, DMA-BUF lent %v", published, lent.Load())
	}
}

func TestFramesMove(t *testing.T) {
	f := newCPUFrame()
	f.paint(0)
	before := append([]byte(nil), f.luma...)
	f.paint(30)
	for i := range before {
		if before[i] != f.luma[i] {
			return
		}
	}
	t.Error("the marker did not move between frames")
}
