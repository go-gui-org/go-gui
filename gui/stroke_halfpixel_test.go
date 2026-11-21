package gui

import "testing"

// A middle-aligned child whose height leaves an odd remainder lands on a
// half pixel, and its border stroke reaches the backend unsnapped. Pins
// the input the backends must snap (#1334); if snapping moves into
// layout, this test is the one to change.
func TestMiddleAlignedBorderStrokeKeepsFractionalY(t *testing.T) {
	for _, c := range []struct{ childH, wantY float32 }{
		{30, 35},
		{31, 34.5},
	} {
		root := &Layout{
			Shape: &Shape{
				X: 0, Y: 0, Width: 200, Height: 100,
				Axis:   axisTopToBottom,
				VAlign: VAlignMiddle,
			},
			Children: []Layout{{Shape: &Shape{
				shapeType: shapeRectangle, Width: 80, Height: c.childH,
				Color:       RGB(255, 255, 255),
				ColorBorder: RGB(200, 200, 200),
				SizeBorder:  1, Radius: 4, Opacity: 1,
			}}},
		}
		w := &Window{}
		layoutParents(root, nil)
		layoutPositions(root, 0, 0, w)
		renderShape(root.Children[0].Shape, ColorTransparent,
			makeClip(0, 0, 500, 500), w)
		i := cmdIndex(w.renderers, isKind(RenderStrokeRect))
		if i < 0 {
			t.Fatalf("childH=%v: no stroke emitted", c.childH)
		}
		if got := w.renderers[i].Y; !f32AreClose(got, c.wantY) {
			t.Errorf("childH=%v: stroke Y = %v, want %v", c.childH, got, c.wantY)
		}
	}
}
