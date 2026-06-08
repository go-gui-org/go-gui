package gui

import "testing"

// benchmarkRenderTree builds a layout tree with depth levels of
// nesting, each container holding childrenPerLevel children.
func benchmarkRenderTree(depth, childrenPerLevel int) Layout {
	root := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			Sizing:    FillFill,
			Width:     1200,
			Height:    900,
		},
	}
	buildRenderChildren(&root, depth, childrenPerLevel)
	return root
}

func buildRenderChildren(parent *Layout, depth, n int) {
	if depth <= 0 {
		return
	}
	parent.Children = make([]Layout, n)
	h := parent.Shape.Height / float32(n)
	for i := range n {
		s := &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisLeftToRight,
			Width:     parent.Shape.Width,
			Height:    h,
			X:         parent.Shape.X,
			Y:         parent.Shape.Y + float32(i)*h,
			shapeClip: drawClip{
				X:      parent.Shape.X,
				Y:      parent.Shape.Y + float32(i)*h,
				Width:  parent.Shape.Width,
				Height: h,
			},
		}
		parent.Children[i] = Layout{Shape: s, Parent: parent}
		buildRenderChildren(&parent.Children[i], depth-1, n)
	}
}

func BenchmarkRenderLayout(b *testing.B) {
	b.Run("flat_120", func(b *testing.B) {
		w := &Window{scratch: newScratchPools()}
		tree := benchmarkRenderTree(1, 120)
		clip := drawClip{Width: 1200, Height: 900}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			w.renderers = w.renderers[:0]
			renderLayout(&tree, White, clip, w)
		}
	})

	b.Run("nested_4x10", func(b *testing.B) {
		w := &Window{scratch: newScratchPools()}
		tree := benchmarkRenderTree(4, 10)
		clip := drawClip{Width: 1200, Height: 900}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			w.renderers = w.renderers[:0]
			renderLayout(&tree, White, clip, w)
		}
	})
}
