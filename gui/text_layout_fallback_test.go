package gui

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// A shaper refusal — past the byte budget, for example — must not
// degrade the caret, selection and grapheme delete in silence. These
// tests pin the DebugGlyphLayoutFallback finding: reported once per
// shape while the category is on, silent otherwise.

// failMeasurer refuses every layout the way an over-budget text is
// refused, without needing a 10KB string in a unit test.
type failMeasurer struct{ *stubTextMeasurer }

func (failMeasurer) LayoutText(
	string, TextStyle, float32,
) (glyph.Layout, error) {
	return glyph.Layout{}, errors.New("text exceeds max length")
}

func TestGlyphLayoutFallbackFinding(t *testing.T) {
	w := newTestWindow()
	w.textMeasurer = &failMeasurer{&stubTextMeasurer{charWidth: 10}}
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{
			Content: []View{
				// Wrap mode resolves the glyph layout during
				// arrange; single-line text only shapes on the
				// render paths that need boundaries.
				Text(TextCfg{ID: "long", Text: "hello",
					Mode: TextModeWrap}),
			},
		})
	}
	found := w.TestFindings(DebugGlyphLayoutFallback)
	joined := strings.Join(found, "\n")
	if !strings.Contains(joined, `"long"`) {
		t.Fatalf("want finding naming the shape, got %q", found)
	}
	if !strings.Contains(joined, "approximate metrics") {
		t.Fatalf("want fallback message, got %q", found)
	}
}

func TestGlyphLayoutFallbackWarnOnce(t *testing.T) {
	buf := captureDebugMask(t, DebugGlyphLayoutFallback)
	w := &Window{}
	w.textMeasurer = &failMeasurer{&stubTextMeasurer{}}
	shape := &Shape{ID: "once", TC: &shapeTextConfig{Text: "hello"}}
	style := DefaultTextStyle
	plainTextLayoutResolved("hello", shape, style, w)
	plainTextLayoutResolved("hello world", shape, style, w)
	if n := strings.Count(buf.String(), "approximate metrics"); n != 1 {
		t.Fatalf("warn-once: want 1 finding, got %d (%q)",
			n, buf.String())
	}
}

func TestGlyphLayoutFallbackCategoryGate(t *testing.T) {
	buf := captureDebug(t)
	DebugCategories(DebugMissingIDs)
	w := &Window{}
	w.textMeasurer = &failMeasurer{&stubTextMeasurer{}}
	shape := &Shape{ID: "gated", TC: &shapeTextConfig{Text: "hello"}}
	plainTextLayoutResolved("hello", shape, DefaultTextStyle, w)
	if got := buf.String(); got != "" {
		t.Fatalf("category off must be silent, got %q", got)
	}
}
