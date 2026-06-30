package gui

import "testing"

type benchView struct {
	shape    Shape
	children []*benchView
}

func (v *benchView) Content() []View {
	if len(v.children) == 0 {
		return nil
	}
	views := make([]View, len(v.children))
	for i, c := range v.children {
		views[i] = c
	}
	return views
}

func (v *benchView) GenerateLayout(_ *Window) Layout {
	return Layout{Shape: &v.shape}
}

func benchViewFlat(n int) *benchView {
	root := &benchView{
		shape:    Shape{shapeType: shapeRectangle, Axis: AxisTopToBottom},
		children: make([]*benchView, n),
	}
	for i := range n {
		root.children[i] = &benchView{
			shape: Shape{
				shapeType: shapeRectangle,
				Width:     200,
				Height:    40,
				IDFocus:   uint32(i + 1),
			},
		}
	}
	return root
}

func benchViewNested(depth, childrenPerLevel int) *benchView {
	v := &benchView{
		shape: Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
		},
	}
	if depth <= 0 {
		return v
	}
	v.children = make([]*benchView, childrenPerLevel)
	for i := range childrenPerLevel {
		v.children[i] = benchViewNested(depth-1, childrenPerLevel)
	}
	return v
}

func benchViewDeep(depth int) *benchView {
	v := &benchView{
		shape: Shape{shapeType: shapeRectangle, Axis: AxisTopToBottom},
	}
	if depth <= 0 {
		return v
	}
	v.children = []*benchView{benchViewDeep(depth - 1)}
	return v
}

func BenchmarkGenerateViewLayout(b *testing.B) {
	// resetViewPools runs once per frame in the real pipeline
	// (window_update.go), recycling the frame-scoped layout-children
	// arena. Mirror that here so steady-state per-frame cost is
	// measured rather than unbounded cross-iteration arena growth.
	b.Run("flat_100", func(b *testing.B) {
		w := &Window{scratch: newScratchPools()}
		view := benchViewFlat(100)
		b.ReportAllocs()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(view, w)
		}
	})

	b.Run("nested_3x10", func(b *testing.B) {
		w := &Window{scratch: newScratchPools()}
		view := benchViewNested(3, 10)
		b.ReportAllocs()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(view, w)
		}
	})

	b.Run("deep_12x1", func(b *testing.B) {
		w := &Window{scratch: newScratchPools()}
		view := benchViewDeep(12)
		b.ReportAllocs()
		for b.Loop() {
			w.scratch.resetViewPools()
			_ = generateViewLayout(view, w)
		}
	})
}
