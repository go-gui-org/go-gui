package gui

import (
	"testing"
)

// benchViewFlat builds a production-shaped flat tree: one Column with n
// Text children. The benchmark measures the real factories (Column,
// Text), so its before/after numbers reflect production code rather
// than a harness stub's per-node Content() slice allocation.
func benchViewFlat(n int) View {
	views := make([]View, n)
	for i := range views {
		views[i] = Text(TextCfg{Text: "row"})
	}
	return Column(ContainerCfg{ID: "root", Content: views})
}

// benchViewNested builds a production-shaped 3-level tree:
// Column → childrenPerLevel Columns → Text leaves each.
func benchViewNested(childrenPerLevel int) View {
	columns := make([]View, childrenPerLevel)
	for i := range columns {
		rows := make([]View, childrenPerLevel)
		for j := range rows {
			rows[j] = Text(TextCfg{Text: "cell"})
		}
		columns[i] = Column(ContainerCfg{Content: rows})
	}
	return Column(ContainerCfg{ID: "root", Content: columns})
}

// benchViewDeep builds a production-shaped chain: depth nested Columns
// with a single Text leaf, exercising the recursion depth.
func benchViewDeep(depth int) View {
	if depth <= 0 {
		return Text(TextCfg{Text: "leaf"})
	}
	return Column(ContainerCfg{Content: []View{benchViewDeep(depth - 1)}})
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
		view := benchViewNested(10)
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
