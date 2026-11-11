package gui

import (
	"strings"
	"testing"
)

// A text's block persists across frames under its identity, so a frame
// that changes nothing shapes nothing. These cover the shapes that lost
// their block every frame.

// shapesPerRebuild renders view once, then counts the layouts shaped
// over five more frames that change nothing.
func shapesPerRebuild(t *testing.T, view func(*Window) View) int {
	t.Helper()
	w := newBlockTestWindow(t)
	m := &probeCountingBlockMeasurer{}
	w.textMeasurer = m
	w.TestRender(view)
	before := m.full + m.probes
	for range 5 {
		w.TestRender(nil)
	}
	return m.full + m.probes - before
}

// One large string shown by two anonymous Texts of different widths:
// each keeps its own block. Keyed by the string alone, they shared one
// and reset it to each other's width every frame.
func TestAnonymousTextBlockPerWidth(t *testing.T) {
	text := strings.Repeat("lorem ipsum dolor sit amet ", 160)
	box := func(width float32) View {
		return Column(ContainerCfg{Sizing: FixedFit, Width: width,
			SizeBorder: NoBorder, Content: []View{
				Text(TextCfg{Text: text, Mode: TextModeWrap}),
			}})
	}
	n := shapesPerRebuild(t, func(*Window) View {
		return Row(ContainerCfg{Sizing: FillFill, Content: []View{
			box(300), box(200),
		}})
	})
	if n != 0 {
		t.Fatalf("five unchanged frames shaped %d layouts, want 0", n)
	}
}
