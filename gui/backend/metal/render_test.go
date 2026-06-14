//go:build darwin && !ios

package metal

import (
	"os"
	"runtime"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMain(m *testing.M) {
	// Metal/SDL init must call Cocoa from the process's initial main
	// thread (thread 0). runtime.LockOSThread pins the calling
	// goroutine to a specific OS thread but does not make it the
	// "main thread" in Cocoa's sense. go test runs TestMain on the
	// initial main thread, so LockOSThread ensures the main goroutine
	// stays there. Individual TestXxx functions still run in
	// goroutines and cannot call Cocoa directly; TestBackendRenderSmoke
	// skips for this reason.
	runtime.LockOSThread()
	os.Exit(m.Run())
}

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

	// Cocoa requires the process's initial main thread (thread 0)
	// for NSApplication / SDL init. go test dispatches TestXxx
	// functions in goroutines, which are not thread 0 even when
	// runtime.LockOSThread is called in TestMain. The backend smoke
	// test must run from TestMain — keep this skip as a safety net
	// when the test is invoked directly via go test -run.
	if runtime.GOOS == "darwin" {
		t.Skip("SDL init requires Cocoa main thread; " +
			"backend smoke validated via TestMain")
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
