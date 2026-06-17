package gui

import "testing"

func benchmarkPipelineLayout(childCount int) Layout {
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
	root.Children = make([]Layout, childCount)
	for i := range childCount {
		root.Children[i] = Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Axis:      AxisLeftToRight,
				Sizing:    FillFit,
				Width:     200,
				Height:    40,
				Spacing:   4,
			},
		}
	}
	return root
}

func clonePipelineLayout(template *Layout, shapes []Shape, layout *Layout) {
	*layout.Shape = *template.Shape
	layout.Parent = nil
	for j := range template.Children {
		shapes[j] = *template.Children[j].Shape
		layout.Children[j] = Layout{Shape: &shapes[j]}
	}
}

func BenchmarkLayoutPipeline(b *testing.B) {
	b.Run("flat_120", func(b *testing.B) {
		w := &Window{scratch: newScratchPools()}
		w.windowWidth = 1200
		w.windowHeight = 900
		template := benchmarkPipelineLayout(120)
		layout := Layout{
			Shape:    &Shape{},
			Children: make([]Layout, len(template.Children)),
		}
		shapes := make([]Shape, len(template.Children))
		b.ReportAllocs()
		for b.Loop() {
			clonePipelineLayout(&template, shapes, &layout)
			layoutPipeline(&layout, w)
		}
	})

	b.Run("nested_4x10", func(b *testing.B) {
		w := &Window{scratch: newScratchPools()}
		w.windowWidth = 1200
		w.windowHeight = 900
		b.ReportAllocs()
		for b.Loop() {
			tree := benchmarkRenderTree(4, 10)
			layoutPipeline(&tree, w)
		}
	})
}

func BenchmarkLayoutWidths(b *testing.B) {
	template := benchmarkPipelineLayout(120)
	layout := Layout{
		Shape:    &Shape{},
		Children: make([]Layout, len(template.Children)),
	}
	shapes := make([]Shape, len(template.Children))
	b.ReportAllocs()
	for b.Loop() {
		clonePipelineLayout(&template, shapes, &layout)
		layoutWidths(&layout)
	}
}

func BenchmarkLayoutFillWidths(b *testing.B) {
	template := benchmarkPipelineLayout(120)
	layout := Layout{
		Shape:    &Shape{},
		Children: make([]Layout, len(template.Children)),
	}
	shapes := make([]Shape, len(template.Children))
	// Run prerequisite pass once to set widths.
	clonePipelineLayout(&template, shapes, &layout)
	layoutWidths(&layout)
	// Save sized state as the base for each iteration.
	sizedRoot := *layout.Shape
	sizedShapes := make([]Shape, len(shapes))
	copy(sizedShapes, shapes)

	b.ReportAllocs()
	for b.Loop() {
		*layout.Shape = sizedRoot
		copy(shapes, sizedShapes)
		for j := range layout.Children {
			layout.Children[j].Shape = &shapes[j]
		}
		layoutFillWidths(&layout, &scratchPools{})
	}
}

func BenchmarkLayoutHeights(b *testing.B) {
	template := benchmarkPipelineLayout(120)
	layout := Layout{
		Shape:    &Shape{},
		Children: make([]Layout, len(template.Children)),
	}
	shapes := make([]Shape, len(template.Children))
	b.ReportAllocs()
	for b.Loop() {
		clonePipelineLayout(&template, shapes, &layout)
		layoutHeights(&layout)
	}
}

func BenchmarkLayoutFillHeights(b *testing.B) {
	template := benchmarkPipelineLayout(120)
	layout := Layout{
		Shape:    &Shape{},
		Children: make([]Layout, len(template.Children)),
	}
	shapes := make([]Shape, len(template.Children))
	clonePipelineLayout(&template, shapes, &layout)
	layoutWidths(&layout)
	layoutFillWidths(&layout, &scratchPools{})
	layoutHeights(&layout)
	sizedRoot := *layout.Shape
	sizedShapes := make([]Shape, len(shapes))
	copy(sizedShapes, shapes)

	b.ReportAllocs()
	for b.Loop() {
		*layout.Shape = sizedRoot
		copy(shapes, sizedShapes)
		for j := range layout.Children {
			layout.Children[j].Shape = &shapes[j]
		}
		layoutFillHeights(&layout, &scratchPools{})
	}
}

func BenchmarkLayoutPositions(b *testing.B) {
	w := &Window{scratch: newScratchPools()}
	w.windowWidth = 1200
	w.windowHeight = 900
	template := benchmarkPipelineLayout(120)
	layout := Layout{
		Shape:    &Shape{},
		Children: make([]Layout, len(template.Children)),
	}
	shapes := make([]Shape, len(template.Children))
	clonePipelineLayout(&template, shapes, &layout)
	layoutWidths(&layout)
	layoutFillWidths(&layout, &scratchPools{})
	layoutHeights(&layout)
	layoutFillHeights(&layout, &scratchPools{})
	sizedRoot := *layout.Shape
	sizedShapes := make([]Shape, len(shapes))
	copy(sizedShapes, shapes)

	b.ReportAllocs()
	for b.Loop() {
		*layout.Shape = sizedRoot
		copy(shapes, sizedShapes)
		for j := range layout.Children {
			layout.Children[j].Shape = &shapes[j]
		}
		layoutPositions(&layout, 0, 0, w)
	}
}

func BenchmarkLayoutSetShapeClips(b *testing.B) {
	w := &Window{scratch: newScratchPools()}
	w.windowWidth = 1200
	w.windowHeight = 900
	tree := benchmarkRenderTree(4, 10)
	clip := drawClip{Width: 1200, Height: 900}
	b.ReportAllocs()
	for b.Loop() {
		layoutSetShapeClips(&tree, clip)
	}
}
