package gui

import (
	"strconv"
	"testing"
)

func benchmarkArrangeLayout() Layout {
	root := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			Sizing:    FillFill,
			Width:     1200,
			Height:    900,
			Spacing:   8,
		},
	}
	root.Children = make([]Layout, 0, 120)
	for i := range 120 {
		ch := Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Axis:      AxisLeftToRight,
				Sizing:    FixedFit,
				Width:     200,
				Height:    40,
				Spacing:   4,
			},
		}
		if i%15 == 0 {
			ch.Shape.Float = true
		}
		root.Children = append(root.Children, ch)
	}
	return root
}

func BenchmarkLayoutArrange(b *testing.B) {
	w := &Window{scratch: newScratchPools()}
	w.windowWidth = 1200
	w.windowHeight = 900

	template := benchmarkArrangeLayout()
	layout := Layout{
		Shape:    &Shape{},
		Children: make([]Layout, len(template.Children)),
	}
	childShapes := make([]Shape, len(template.Children))

	b.ReportAllocs()
	for b.Loop() {
		*layout.Shape = *template.Shape
		layout.Parent = nil
		for j := range template.Children {
			childShapes[j] = *template.Children[j].Shape
			layout.Children[j] = Layout{
				Shape: &childShapes[j],
			}
		}
		layers := layoutArrange(&layout, w)
		w.scratch.layerLayouts.put(layers)
	}
}

func benchmarkWrapLayout() Layout {
	root := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisLeftToRight,
			Wrap:      true,
			Width:     600,
			Spacing:   6,
		},
		Children: make([]Layout, 0, 200),
	}
	for range 200 {
		root.Children = append(root.Children, Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Width:     70,
				Height:    20,
			},
		})
	}
	return root
}

func BenchmarkLayoutWrapContainers(b *testing.B) {
	w := &Window{scratch: newScratchPools()}
	b.ReportAllocs()
	for b.Loop() {
		w.scratch.resetViewPools()
		layout := benchmarkWrapLayout()
		layoutWrapContainers(&layout, w)
	}
}

func benchmarkFocusLayout() *Layout {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle},
	}
	root.Children = make([]Layout, 0, 200)
	for i := 1; i <= 200; i++ {
		root.Children = append(root.Children, Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Focusable: true,
				ID:        "f" + strconv.Itoa(i),
			},
		})
	}
	return root
}

func BenchmarkFocusTraversal(b *testing.B) {
	w := &Window{scratch: newScratchPools()}
	root := benchmarkFocusLayout()
	b.ReportAllocs()
	for b.Loop() {
		if s, ok := root.NextFocusable(w); ok {
			w.SetFocus(s.ID)
		}
	}
}
