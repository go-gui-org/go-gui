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

// buildButtonFrameView creates a tree of buttons with hover/focus colors
// set, exercising the buttonColors pool and buttonOnHover/buttonAmendLayout
// wiring in the folded containerView.GenerateLayout.
func buildButtonFrameView(buttons int) View {
	content := make([]View, buttons)
	for i := range buttons {
		content[i] = Button(ButtonCfg{
			ID:      "btn-" + strconv.Itoa(i),
			Sizing:  FitFill,
			OnClick: func(_ *Layout, _ *Event, _ *Window) {},
			Content: []View{
				Text(TextCfg{Text: "Button " + strconv.Itoa(i)}),
			},
		})
	}
	return Column(ContainerCfg{
		ID:      "btn-root",
		Sizing:  FillFill,
		Content: content,
	})
}

// buildEffectsFrameView creates containers with shadow/gradient effects
// set, exercising the viewEffects pool.
func buildEffectsFrameView(containers int) View {
	shadow := &BoxShadow{Color: Black, OffsetX: 2, OffsetY: 2, BlurRadius: 4}
	content := make([]View, containers)
	for i := range containers {
		content[i] = Row(ContainerCfg{
			ID:     "fx-" + strconv.Itoa(i),
			Sizing: FitFit,
			Shadow: shadow,
			Color:  LightGray,
			Content: []View{
				Text(TextCfg{Text: "Effect " + strconv.Itoa(i)}),
			},
		})
	}
	return Column(ContainerCfg{
		ID:      "fx-root",
		Sizing:  FillFill,
		Content: content,
	})
}

// BenchmarkButtonFrame exercises the Button factory path (folded
// buttonView) through the full frame pipeline.
func BenchmarkButtonFrame(b *testing.B) {
	for _, count := range []int{50, 200} {
		b.Run("btns_"+strconv.Itoa(count), func(b *testing.B) {
			w := &Window{scratch: newScratchPools()}
			w.windowWidth = 1200
			w.windowHeight = 900
			b.ReportAllocs()
			for b.Loop() {
				w.scratch.resetViewPools()
				view := buildButtonFrameView(count)
				_ = generateViewLayout(view, w)
			}
		})
	}
}

// BenchmarkEffectsFrame exercises containers with shadow/gradient effects
// through the full frame pipeline, verifying the viewEffects pool.
func BenchmarkEffectsFrame(b *testing.B) {
	for _, count := range []int{50, 200} {
		b.Run("fx_"+strconv.Itoa(count), func(b *testing.B) {
			w := &Window{scratch: newScratchPools()}
			w.windowWidth = 1200
			w.windowHeight = 900
			b.ReportAllocs()
			for b.Loop() {
				w.scratch.resetViewPools()
				view := buildEffectsFrameView(count)
				_ = generateViewLayout(view, w)
			}
		})
	}
}
