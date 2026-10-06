//go:build darwin && cgo && !ios

package metal

import (
	"testing"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
)

// TestDrawPathsDoNotAllocate guards the per-draw vertex and uniform
// arrays. Each one is handed to C by pointer; built as a local, cgo
// makes the compiler move it to the heap, so every glyph and every
// rect cost one allocation. A terminal repainting a full screen at
// 60 fps spent about 13 MB/s of its garbage on these alone. The
// arrays live in per-backend scratch fields instead, so a draw must
// not allocate at all.
func TestDrawPathsDoNotAllocate(t *testing.T) {
	ws, gb, free := testNullBackends()
	defer free()

	rect := &gui.RenderCmd{
		X: 10, Y: 20, W: 100, H: 40, Radius: 4, Thickness: 1,
		Fill: true, Color: gui.Red,
		OffsetX: 50, OffsetY: 60, BlurRadius: 3, Spread: 1,
		StencilDepth: 1,
	}
	grad := &gui.RenderCmd{
		X: 0, Y: 0, W: 100, H: 40,
		Gradient: &gui.GradientDef{Stops: []gui.GradientStop{
			{Color: gui.Red, Pos: 0}, {Color: gui.Blue, Pos: 1},
		}},
	}
	src := glyph.Rect{X: 1, Y: 2, Width: 8, Height: 16}
	dst := glyph.Rect{X: 30, Y: 40, Width: 8, Height: 16}
	col := glyph.Color{R: 200, G: 200, B: 200, A: 255}
	tr := glyph.AffineTransform{XX: 1, YY: 1}

	cases := []struct {
		name string
		draw func()
	}{
		{"drawRect", func() { ws.drawRect(rect) }},
		{"drawStrokeRect", func() { ws.drawStrokeRect(rect) }},
		{"drawCircle", func() { ws.drawCircle(rect) }},
		{"drawLine", func() { ws.drawLine(rect) }},
		{"drawShadow", func() { ws.drawShadow(rect) }},
		{"drawBlur", func() { ws.drawBlur(rect) }},
		{"drawGradient", func() { ws.drawGradient(nil, grad) }},
		{"drawGradientBorder", func() { ws.drawGradientBorder(grad) }},
		{"beginStencilClip", func() { ws.beginStencilClip(rect) }},
		{"endStencilClip", func() { ws.endStencilClip(rect) }},
		{"DrawTexturedQuad", func() { gb.DrawTexturedQuad(1, src, dst, col) }},
		{"DrawFilledRect", func() { gb.DrawFilledRect(dst, col) }},
		{"DrawFilledRectTransformed", func() {
			gb.DrawFilledRectTransformed(dst, col, tr)
		}},
		{"DrawTexturedQuadTransformed", func() {
			gb.DrawTexturedQuadTransformed(1, src, dst, col, tr)
		}},
	}
	for _, tc := range cases {
		// One warm-up call grows any reusable scratch to size.
		tc.draw()
		if n := testing.AllocsPerRun(100, tc.draw); n != 0 {
			t.Errorf("%s: %v allocs per call, want 0", tc.name, n)
		}
	}
}
