package gui

import (
	"strconv"
	"testing"
)

// benchFrameIDs is hoisted so the bench measures view construction, not
// strconv.
var benchFrameIDs = func() []string {
	ids := make([]string, 256)
	for i := range ids {
		ids[i] = "row-" + strconv.Itoa(i)
	}
	return ids
}()

// buildFrameView constructs a widget tree through the public factories,
// exactly as a per-frame view function would. Unlike benchViewFlat, the
// Shapes are allocated inside the loop, which is what makes this bench
// sensitive to sizeof(Shape).
func buildFrameView(rows int) View {
	content := make([]View, rows)
	for i := range rows {
		content[i] = Row(ContainerCfg{
			ID:      benchFrameIDs[i],
			Sizing:  FillFit,
			Padding: NoPadding,
			Content: []View{
				Column(ContainerCfg{Sizing: FillFit, Color: ColorTransparent}),
				Column(ContainerCfg{Sizing: FillFit, Color: ColorTransparent}),
			},
		})
	}
	return Column(ContainerCfg{
		ID:      "frame-root",
		Sizing:  FillFill,
		Content: content,
	})
}

// BenchmarkViewFrame builds the view tree AND generates the layout each
// iteration, which is what happens on every real frame.
func BenchmarkViewFrame(b *testing.B) {
	for _, rows := range []int{50, 200} {
		b.Run("rows_"+strconv.Itoa(rows), func(b *testing.B) {
			w := &Window{scratch: newScratchPools()}
			w.windowWidth = 1200
			w.windowHeight = 900
			b.ReportAllocs()
			for b.Loop() {
				// Mirrors window_update.go: the frame-scoped arena is
				// recycled once per frame.
				w.scratch.resetViewPools()
				view := buildFrameView(rows)
				_ = generateViewLayout(view, w)
			}
		})
	}
}
