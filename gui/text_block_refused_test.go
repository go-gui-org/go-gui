package gui

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// refusingMeasurer refuses a text with a NUL byte, as go-glyph does.
type refusingMeasurer struct{ blockStubMeasurer }

func (m refusingMeasurer) LayoutText(
	text string, style TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	if strings.IndexByte(text, 0) >= 0 {
		return glyph.Layout{}, errors.New("null byte")
	}
	return m.blockStubMeasurer.LayoutText(text, style, wrapWidth)
}

func newRefusingWindow(t *testing.T) *Window {
	t.Helper()
	w := newBlockTestWindow(t)
	w.textMeasurer = refusingMeasurer{}
	return w
}

// A refused chunk draws nothing but keeps a size from its estimate, so the
// lines around it stay put. A block within the eager budget shapes before
// it estimates, and a block past it estimates first: both end up the same.
func TestRefusedChunkKeepsEstimatedSize(t *testing.T) {
	const text = "one\nbad\x00\nthree"
	for _, tc := range []struct {
		name  string
		eager int
	}{
		{"shaped when created", textBlockEagerBytes},
		{"estimated when created", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withEagerBytes(t, tc.eager)
			w := newRefusingWindow(t)
			b := newTextBlock(text, 0, DefaultTextStyle, TextModeMultiline, w)
			blockEnsureAll(b, w)
			b.settle()
			c := b.chunks[0]
			if !c.failed || c.layout != nil {
				t.Fatalf("chunk failed = %v, layout = %v", c.failed, c.layout != nil)
			}
			if want := 3 * stubLH; b.height != want {
				t.Errorf("height = %v, want %v", b.height, want)
			}
			if c.width <= 0 || c.lastLineH != stubLH {
				t.Errorf("width = %v, last line = %v", c.width, c.lastLineH)
			}
			if b.hasEstimates() {
				t.Error("a refused chunk is not one a later pass can measure")
			}
		})
	}
}

// failErr drives the debug finding, so it must not outlive the refusal.
func TestRefusalClearsWithTheChunk(t *testing.T) {
	withChunkBytes(t, 8)
	const bad, good = "aaaa\nbb\x00\n", "aaaa\nbb\n"
	const tail, tailBad = "cccccc\ndddddd", "cc\x00ccc\ndddddd"
	shape := func(b *textBlock, w *Window) {
		blockEnsureAll(b, w)
		b.settle()
	}
	refused := func(t *testing.T, b *textBlock, want bool) {
		t.Helper()
		if got := b.failErr != nil; got != want {
			t.Fatalf("failErr set = %v, want %v (bytes %d)", got, want, b.failBytes)
		}
	}
	update := func(b *textBlock, text string, width float32, w *Window) {
		b.update(text, width, DefaultTextStyle, TextModeMultiline, w)
		shape(b, w)
	}

	t.Run("an edit removes the refused bytes", func(t *testing.T) {
		w := newRefusingWindow(t)
		b := newTextBlock(bad+tail, 0, DefaultTextStyle, TextModeMultiline, w)
		shape(b, w)
		refused(t, b, true)
		update(b, good+tail, 0, w)
		refused(t, b, false)
	})
	t.Run("another chunk is still refused", func(t *testing.T) {
		w := newRefusingWindow(t)
		b := newTextBlock(bad+tailBad, 0, DefaultTextStyle, TextModeMultiline, w)
		shape(b, w)
		refused(t, b, true)
		update(b, good+tailBad, 0, w)
		refused(t, b, true)
	})
	t.Run("a rebuild starts clean", func(t *testing.T) {
		w := newRefusingWindow(t)
		b := newTextBlock(bad+tail, 0, DefaultTextStyle, TextModeMultiline, w)
		shape(b, w)
		refused(t, b, true)
		update(b, good+tail, 500, w)
		refused(t, b, false)
	})
}
