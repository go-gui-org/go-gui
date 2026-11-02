package gui

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// blockDraws renders view and returns the frame's chunk draws with the
// text block they came from and its origin.
func blockDraws(
	t *testing.T, w *Window, view func(*Window) View, id string,
) (cmds []RenderCmd, blk *textBlock, baseX, baseY float32) {
	t.Helper()
	ly := w.TestRender(view)
	sh, ok := ly.FindByID(id)
	if !ok || sh.Shape.TC == nil || sh.Shape.TC.block == nil {
		t.Fatal("text shape or its block not found")
	}
	blk = sh.Shape.TC.block
	if len(blk.chunks) < 3 {
		t.Fatalf("want several chunks, got %d", len(blk.chunks))
	}
	for _, c := range w.Renderers() {
		if c.Kind == RenderLayout || c.Kind == RenderLayoutTransformed {
			cmds = append(cmds, c)
		}
	}
	return cmds, blk, sh.Shape.X + sh.Shape.PaddingLeft(),
		sh.Shape.Y + sh.Shape.PaddingTop()
}

// numberedLines is n short paragraphs, each distinct, so every chunk of
// a block over them is too and a draw names its chunk by its text.
func numberedLines(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "line %03d abcd\n", i)
	}
	return b.String()
}

// chunkAtText returns the chunk whose text a draw carries.
func chunkAtText(blk *textBlock, text string) (*textChunk, bool) {
	for i := range blk.chunks {
		c := &blk.chunks[i]
		if blk.text[c.start:c.end] == text {
			return c, true
		}
	}
	return nil, false
}

// A transformed text past one chunk turns as one block. glyph applies a
// draw's transform about the draw's origin, so a chunk drawn at its own
// origin would turn about its own corner; each chunk's corner must land
// where the block's transform puts it.
func TestRenderTextBlockTransformsAsOneBlock(t *testing.T) {
	withChunkBytes(t, 64)
	w := newBlockTestWindow(t)
	text := numberedLines(20)
	const angle = 0.5
	cmds, blk, baseX, baseY := blockDraws(t, w, func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Text(TextCfg{ID: "t", Text: text, Mode: TextModeWrap,
				TextStyle: TextStyle{Size: 16, Color: RGB(20, 20, 20),
					RotationRadians: angle}}),
		}})
	}, "t")
	rot := glyph.AffineRotation(angle)
	checked := 0
	for _, cmd := range cmds {
		c, ok := chunkAtText(blk, cmd.Text)
		if !ok || cmd.LayoutTransform == nil {
			continue
		}
		gx, gy := cmd.LayoutTransform.Apply(0, 0)
		gx, gy = gx+cmd.X, gy+cmd.Y
		wx, wy := rot.Apply(c.x, c.y)
		wx, wy = wx+baseX, wy+baseY
		if math.Abs(float64(gx-wx)) > 0.01 || math.Abs(float64(gy-wy)) > 0.01 {
			t.Errorf("chunk at %v,%v drawn with its corner at %v,%v, want %v,%v",
				c.x, c.y, gx, gy, wx, wy)
		}
		checked++
	}
	if checked < 2 {
		t.Fatalf("checked %d transformed chunk draws, want several", checked)
	}
}

// Culling tests a chunk's transformed box. Scaled by half, the chunks
// below the window's height come into view; tested untransformed they
// would be skipped and the text would end half way down its box.
func TestRenderTextBlockCullsTransformedChunks(t *testing.T) {
	withChunkBytes(t, 64)
	w := newBlockTestWindow(t) // 400 px tall
	text := numberedLines(60)
	half := glyph.AffineTransform{XX: 0.5, YY: 0.5}
	cmds, blk, _, _ := blockDraws(t, w, func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Text(TextCfg{ID: "t", Text: text, Mode: TextModeWrap,
				TextStyle: TextStyle{Size: 16, Color: RGB(20, 20, 20),
					AffineTransform: &half}}),
		}})
	}, "t")
	var deepest float32
	for _, cmd := range cmds {
		if c, ok := chunkAtText(blk, cmd.Text); ok {
			deepest = max(deepest, c.y)
		}
	}
	if deepest < 500 {
		t.Errorf("the deepest chunk drawn starts at %v, want past 500: "+
			"scaled by half, the chunks down to 800 are in view", deepest)
	}
}

// A fade of texts past one chunk draws both chunk by chunk, at their
// alphas. The plain path's single RenderText of a whole string hands the
// backend more than glyph.MaxTextLength bytes, which it draws as nothing:
// #1190 for the length of the fade.
func TestContentTransitionFadeLargeText(t *testing.T) {
	big := func(word string) string { return strings.Repeat(word+" text line\n", 1000) }
	cases := []struct{ name, from, to string }{
		{"both past one chunk", big("old"), big("new")},
		{"a long text cleared", big("old"), "new text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fadeLargeText(t, tc.from, tc.to)
		})
	}
}

func fadeLargeText(t *testing.T, from, to string) {
	w := NewTestWindow(t, WindowCfg{State: new(int), Width: 300, Height: 200})
	w.textMeasurer = blockStubMeasurer{}
	text := from
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Text(TextCfg{ID: "n", Text: text, Mode: TextModeWrap,
				Transition: ContentTransition{Kind: ContentTransitionFade}}),
		}})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	text = to
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.stopAnimationLoop()
	var oldA, newA uint8
	for _, c := range renderContentTransitionAt(t, w, 0.5) {
		// The shaper refuses a layout past its budget, so no draw of
		// either kind may carry that much; the stub would shape it.
		if len(c.Text) > glyph.MaxTextLength {
			t.Errorf("a %d-byte text drawn whole", len(c.Text))
		}
		switch c.Kind {
		case RenderLayout, RenderLayoutTransformed:
			if c.TextStylePtr == nil {
				continue
			}
			a := c.TextStylePtr.Color.A
			switch {
			case strings.HasPrefix(c.Text, "old"):
				oldA = max(oldA, a)
			case strings.HasPrefix(c.Text, "new"):
				newA = max(newA, a)
			}
		}
	}
	if oldA == 0 || newA == 0 {
		t.Fatalf("old text drawn at alpha %d, new at %d: want both", oldA, newA)
	}
	if oldA >= newA {
		t.Errorf("past halfway, old alpha %d should be below new %d", oldA, newA)
	}
}
