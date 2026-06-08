package gui

import "testing"

func buildFuzzLayoutTree(depth, childCount int, width, height float32) Layout {
	root := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			Sizing:    FillFill,
			Width:     width,
			Height:    height,
		},
	}
	if depth > 0 && childCount > 0 {
		root.Children = make([]Layout, childCount)
		childH := height / f32Max(float32(childCount), 1)
		for i := range childCount {
			axis := AxisTopToBottom
			if i%2 == 0 {
				axis = AxisLeftToRight
			}
			root.Children[i] = Layout{
				Shape: &Shape{
					shapeType: shapeRectangle,
					Axis:      axis,
					Sizing:    FillFit,
					Width:     width / 2,
					Height:    childH,
				},
			}
			if depth > 1 {
				sub := buildFuzzLayoutTree(
					depth-1, childCount/2, width/2, childH)
				root.Children[i].Children = sub.Children
			}
		}
	}
	return root
}

func walkLayoutAssertNonNegative(t *testing.T, layout *Layout) {
	t.Helper()
	if layout.Shape.Width < 0 {
		t.Errorf("negative width: %f", layout.Shape.Width)
	}
	if layout.Shape.Height < 0 {
		t.Errorf("negative height: %f", layout.Shape.Height)
	}
	for i := range layout.Children {
		walkLayoutAssertNonNegative(t, &layout.Children[i])
	}
}

func FuzzLayoutPipelineDimensions(f *testing.F) {
	f.Add(uint8(3), uint8(5), float32(800), float32(600))
	f.Add(uint8(1), uint8(50), float32(100), float32(100))
	f.Add(uint8(0), uint8(0), float32(0), float32(0))
	f.Add(uint8(5), uint8(1), float32(1920), float32(1080))
	f.Add(uint8(2), uint8(10), float32(1), float32(1))
	f.Fuzz(func(t *testing.T, depth, childCount uint8, width, height float32) {
		d := int(depth % 6)
		n := int(childCount % 20)
		// Clamp to reasonable positive values.
		if width < 0 {
			width = -width
		}
		if height < 0 {
			height = -height
		}
		if width > 4096 {
			width = 4096
		}
		if height > 4096 {
			height = 4096
		}
		w := &Window{scratch: newScratchPools()}
		w.windowWidth = int(width)
		w.windowHeight = int(height)
		layout := buildFuzzLayoutTree(d, n, width, height)
		layoutPipeline(&layout, w)
		walkLayoutAssertNonNegative(t, &layout)
	})
}
