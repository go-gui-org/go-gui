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

// A wrapped Text in a centred column shrinks to its longest line, and
// the block adopts that width so the render pass reuses it. The next
// frame asks for the column's width again: the block must take that as
// the same shaping, or it is rebuilt every frame.
func TestShrinkWrappedTextBlockKeepsItsChunks(t *testing.T) {
	// 20-letter words: one per line, 200 px of the 370 the column gives.
	text := strings.Repeat("abcdefghijklmnopqrst ", 450) // ~9 KB
	n := shapesPerRebuild(t, func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, HAlign: HAlignCenter,
			Content: []View{
				Text(TextCfg{ID: "t", Text: text, Mode: TextModeWrap}),
			}})
	})
	if n != 0 {
		t.Fatalf("five unchanged frames shaped %d layouts, want 0", n)
	}
}

// After a shrink, an edit re-chunks a paragraph, and it is shaped at the
// width the column gives, not the narrower one the box adopted. A line of
// 300 px fits the 370 px column: shaped at the box's old 200 px it would
// wrap, and the box would stay at 200 px.
func TestShrinkWrappedTextEditShapesAtColumnWidth(t *testing.T) {
	w := newBlockTestWindow(t)
	text := strings.Repeat("abcdefghijklmnopqrst ", 450)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, HAlign: HAlignCenter,
			Content: []View{
				Text(TextCfg{ID: "t", Text: text, Mode: TextModeWrap}),
			}})
	})
	// At the top, so the frame draws and shapes it.
	text = strings.Repeat("ab ", 10) + "\n" + text
	w.TestRender(nil)
	w.TestRender(nil)
	ly, ok := w.layout.FindByID("t")
	if !ok {
		t.Fatal("text not found")
	}
	if got := ly.Shape.Width; !f32AreClose(got, 300) {
		t.Fatalf("box width after the edit = %v, want 300", got)
	}
}
