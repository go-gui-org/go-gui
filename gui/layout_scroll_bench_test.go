package gui

import "testing"

func benchScrollLayout(scrollRegions, childrenPer int) Layout {
	root := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			Sizing:    FillFill,
			Width:     1200,
			Height:    900,
		},
		Children: make([]Layout, scrollRegions),
	}
	for i := range scrollRegions {
		container := Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Axis:      AxisTopToBottom,
				IDScroll:  uint32(i + 1),
				Width:     1200,
				Height:    200,
			},
			Children: make([]Layout, childrenPer),
		}
		for j := range childrenPer {
			container.Children[j] = Layout{
				Shape: &Shape{
					shapeType: shapeRectangle,
					Width:     1200,
					Height:    40,
					IDFocus:   uint32(i*childrenPer + j + 1),
				},
			}
		}
		root.Children[i] = container
	}
	return root
}

func BenchmarkLayoutAdjustScrollOffsets(b *testing.B) {
	w := &Window{scratch: newScratchPools()}
	w.windowWidth = 1200
	w.windowHeight = 900
	layout := benchScrollLayout(10, 20)
	b.ReportAllocs()
	for b.Loop() {
		layoutAdjustScrollOffsets(&layout, w)
	}
}

func benchOverflowLayout(totalChildren int) Layout {
	root := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisLeftToRight,
			Overflow:  true,
			Width:     600,
			Height:    40,
			Spacing:   4,
		},
		Children: make([]Layout, totalChildren),
	}
	for i := range totalChildren - 1 {
		root.Children[i] = Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Width:     80,
				Height:    40,
			},
		}
	}
	// Last child is the overflow trigger button.
	root.Children[totalChildren-1] = Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     32,
			Height:    40,
		},
	}
	return root
}

func BenchmarkLayoutOverflow(b *testing.B) {
	w := &Window{scratch: newScratchPools()}
	layout := benchOverflowLayout(20)
	b.ReportAllocs()
	for b.Loop() {
		// Reset visibility each iteration.
		for i := range layout.Children {
			layout.Children[i].Shape.shapeType = shapeRectangle
		}
		layoutOverflow(&layout, w)
	}
}
