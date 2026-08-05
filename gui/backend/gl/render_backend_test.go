//go:build !js && !darwin && !cgo

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

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
					Color:  gui.Blue,
				}),
				gui.Text(gui.TextCfg{Text: "smoke gl"}),
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

	// Render one frame with the OpenGL backend — should not panic.
	b.renderFrame(w)
}
