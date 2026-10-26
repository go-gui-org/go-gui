package main

import (
	"math"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// yuvToRGB is the shader's conversion in Go, so the round trip below checks
// that the shader inverts rgbToYUV.
func yuvToRGB(y, cb, cr byte) (r, g, b float64) {
	yl := (float64(y)/255 - 16.0/255) * (255.0 / 219)
	u := (float64(cb)/255 - 128.0/255) * (255.0 / 224)
	v := (float64(cr)/255 - 128.0/255) * (255.0 / 224)
	clamp := func(x float64) float64 { return math.Min(1, math.Max(0, x)) * 255 }
	return clamp(yl + 1.5748*v), clamp(yl - 0.1873*u - 0.4681*v), clamp(yl + 1.8556*u)
}

func TestYUVRoundTrip(t *testing.T) {
	for _, c := range [][3]byte{{255, 255, 255}, {255, 255, 0}, {0, 255, 255}, {0, 255, 0},
		{255, 0, 255}, {255, 0, 0}, {0, 0, 255}, {0, 0, 0}, {128, 128, 128}} {
		r, g, b := yuvToRGB(rgbToYUV(c[0], c[1], c[2]))
		for i, got := range []float64{r, g, b} {
			if math.Abs(got-float64(c[i])) > 3 {
				t.Errorf("%v round trip = %.0f,%.0f,%.0f", c, r, g, b)
				break
			}
		}
	}
}

func TestFramesReachTheShader(t *testing.T) {
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Planes: nv12Planes})
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	state := &appState{surface: surface,
		shader: &gui.Shader{GLSL: nv12GLSL, Metal: nv12Metal, Surface: surface}}
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 720, Height: 500, State: state})
	w.SetView(mainView)
	frame := newFrame()
	frame.paint(0)
	if err = frame.publish(surface); err != nil {
		t.Fatal(err)
	}
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
	// The marker moves: two frames differ in luma.
	before := append([]byte(nil), frame.luma...)
	frame.paint(30)
	same := true
	for i := range before {
		if before[i] != frame.luma[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("the marker did not move between frames")
	}
}

func TestProducerStops(t *testing.T) {
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Planes: nv12Planes})
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	stop, done := make(chan struct{}), make(chan struct{})
	close(stop)
	go animate(surface, newFrame(), stop, done)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("producer did not stop")
	}
}
