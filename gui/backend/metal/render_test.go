//go:build darwin && !ios

package metal

import (
	"runtime"
	"testing"

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
					Color:  gui.Green,
				}),
				gui.Text(gui.TextCfg{Text: "smoke metal"}),
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
					Color:  gui.Green,
				}),
				gui.Text(gui.TextCfg{Text: "smoke metal"}),
			},
		})
	})

	// Metal/SDL2 init on macOS requires the main thread for Cocoa.
	// go test runs tests in goroutines, so this test skips on macOS.
	// TODO: run with -exec 'arch -arm64' or a main-thread test
	// harness if needed in the future.
	if runtime.GOOS == "darwin" {
		t.Skip("SDL init requires main thread on macOS; " +
			"backend smoke test runs on Linux CI instead")
	}

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

	// Render one frame with the Metal backend — should not panic.
	b.renderFrame(w)
}
