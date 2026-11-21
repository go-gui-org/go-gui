//go:build js && wasm

package web

import (
	"fmt"
	"slices"
	"strings"
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// rectRecorder is a canvas that records fillRect and strokeRect calls
// with the lineWidth in force.
func rectRecorder(t *testing.T) (*Backend, *[]string) {
	t.Helper()
	obj := js.Global().Get("Object").New()
	var calls []string
	rec := func(name string) {
		f := js.FuncOf(func(_ js.Value, a []js.Value) any {
			parts := make([]string, len(a))
			for i, v := range a {
				parts[i] = fmt.Sprintf("%.6g", v.Float())
			}
			s := name + "(" + strings.Join(parts, ",") + ")"
			if name == "strokeRect" {
				s += fmt.Sprintf(" lw=%.6g", obj.Get("lineWidth").Float())
			}
			calls = append(calls, s)
			return nil
		})
		t.Cleanup(f.Release)
		obj.Set(name, f)
	}
	rec("fillRect")
	rec("strokeRect")
	return &Backend{ctx2d: obj}, &calls
}

// Rect edges snap to whole device pixels, and a stroke lies inside its
// box on whole device pixels, as the SDF backends draw it (#1334).
func TestDrawRectSnapsToDevicePixels(t *testing.T) {
	cases := []struct {
		scale float32
		cmd   gui.RenderCmd
		want  string
	}{
		{1, gui.RenderCmd{Kind: gui.RenderRect, Fill: true,
			X: 10.5, Y: 10.25, W: 40, H: 20, Color: gui.RGB(1, 2, 3)},
			"fillRect(11,10,40,20)"},
		{1, gui.RenderCmd{Kind: gui.RenderStrokeRect, Thickness: 1,
			X: 10, Y: 10.5, W: 40, H: 20, Color: gui.RGB(1, 2, 3)},
			"strokeRect(10.5,11.5,39,19) lw=1"},
		// Device: x 13..63, th 1.25 -> 1 px = 0.8 logical.
		{1.25, gui.RenderCmd{Kind: gui.RenderStrokeRect, Thickness: 1,
			X: 10, Y: 10, W: 40, H: 20, Color: gui.RGB(1, 2, 3)},
			"strokeRect(10.8,10.8,39.2,19.2) lw=0.8"},
		// A box thinner than the line caps the line at half the box, as
		// soft and the SDF quad do, so the stroke stays inside.
		{1, gui.RenderCmd{Kind: gui.RenderStrokeRect, Thickness: 4,
			X: 10, Y: 10, W: 40, H: 2, Color: gui.RGB(1, 2, 3)},
			"strokeRect(10.5,10.5,39,1) lw=1"},
		// An empty box strokes nothing.
		{1, gui.RenderCmd{Kind: gui.RenderStrokeRect, Thickness: 1,
			X: 10, Y: 10, W: 0, H: 20, Color: gui.RGB(1, 2, 3)}, ""},
	}
	for _, c := range cases {
		b, calls := rectRecorder(t)
		b.dpiScale = c.scale
		if c.cmd.Kind == gui.RenderRect {
			b.drawRect(&c.cmd)
		} else {
			b.drawStrokeRect(&c.cmd)
		}
		var want []string
		if c.want != "" {
			want = []string{c.want}
		}
		if !slices.Equal(*calls, want) {
			t.Errorf("scale %v %v: calls %q, want %q",
				c.scale, c.cmd.Kind, *calls, want)
		}
	}
}
