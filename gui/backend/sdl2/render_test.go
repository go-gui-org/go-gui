//go:build !js && !darwin

package sdl2

import (
	"testing"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"

	"github.com/go-gui-org/go-gui/gui"
)

func TestSmokeRenderPipeline(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	w.UpdateView(func(_ *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Content: []gui.View{
				gui.Rectangle(gui.RectangleCfg{
					Width:  100,
					Height: 50,
					Color:  gui.Red,
				}),
				gui.Text(gui.TextCfg{Text: "smoke"}),
			},
		})
	})

	if !w.FrameFn() {
		t.Fatal("FrameFn returned false — expected renderer rebuild")
	}
	cmds := w.Renderers()
	if len(cmds) == 0 {
		t.Fatal("expected non-empty renderers after FrameFn")
	}

	var hasRect, hasText bool
	for _, r := range cmds {
		switch r.Kind {
		case gui.RenderRect:
			hasRect = true
		case gui.RenderText:
			hasText = true
		}
	}
	if !hasRect {
		t.Error("expected at least one RenderRect command")
	}
	if !hasText {
		t.Error("expected at least one RenderText command")
	}
}

func TestBackendRenderSmoke(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	w.UpdateView(func(_ *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Content: []gui.View{
				gui.Rectangle(gui.RectangleCfg{
					Width:  100,
					Height: 50,
					Color:  gui.Red,
				}),
				gui.Text(gui.TextCfg{Text: "smoke"}),
			},
		})
	})

	b, err := New(w)
	if err != nil {
		t.Skipf("backend init failed (no display?): %v", err)
	}
	defer b.Destroy()

	if !w.FrameFn() {
		t.Fatal("FrameFn returned false — expected renderer rebuild")
	}
	cmds := w.Renderers()
	if len(cmds) == 0 {
		t.Fatal("expected non-empty renderers before renderFrame")
	}

	// Render one frame — should not panic.
	b.renderFrame(w)

	// Read back center pixel to confirm framebuffer is non-empty.
	// SDL2 renderer always has a target; after renderFrame the
	// backbuffer is the target.
	pitch := 200 * 4 // width * RGBA
	pixels := make([]byte, pitch*200)
	rect := &sdl.Rect{X: 50, Y: 25, W: 1, H: 1}
	if err := b.renderer.ReadPixels(rect,
		sdl.PIXELFORMAT_ABGR8888,
		unsafe.Pointer(&pixels[0]), pitch); err != nil {
		t.Logf("ReadPixels failed: %v (non-fatal)", err)
		return
	}
	// Check that the pixel isn't pure black (the clear color).
	// ABGR8888: [0]=R, [1]=G, [2]=B, [3]=A
	r, g, b2, a := pixels[0], pixels[1], pixels[2], pixels[3]
	if r == 0 && g == 0 && b2 == 0 && a == 0 {
		t.Error("framebuffer center pixel is transparent black — " +
			"expected rendered content")
	}
}
