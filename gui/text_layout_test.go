package gui

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestPlainTextNeedsGlyphLayoutNils(t *testing.T) {
	if plainTextNeedsGlyphLayout(nil, &shapeTextConfig{}, TextStyle{}) {
		t.Error("nil shape should return false")
	}
	if plainTextNeedsGlyphLayout(&Shape{}, nil, TextStyle{}) {
		t.Error("nil tc should return false")
	}
}

func TestPlainTextNeedsGlyphLayoutDefault(t *testing.T) {
	s := &Shape{}
	tc := &shapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{Align: TextAlignLeft}
	if plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("default single-line left-aligned should return false")
	}
}

func TestPlainTextNeedsGlyphLayoutWrap(t *testing.T) {
	s := &Shape{}
	tc := &shapeTextConfig{TextMode: TextModeWrap}
	if !plainTextNeedsGlyphLayout(s, tc, TextStyle{}) {
		t.Error("TextModeWrap should return true")
	}
}

func TestPlainTextNeedsGlyphLayoutCenter(t *testing.T) {
	s := &Shape{}
	tc := &shapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{Align: TextAlignCenter}
	if !plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("TextAlignCenter should return true")
	}
}

func TestPlainTextNeedsGlyphLayoutSpacing(t *testing.T) {
	s := &Shape{}
	tc := &shapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{LineSpacing: 1.5}
	if !plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("non-zero LineSpacing should return true")
	}
}

func TestPlainTextNeedsGlyphLayoutBgColor(t *testing.T) {
	s := &Shape{}
	tc := &shapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{BgColor: RGBA(255, 0, 0, 128)}
	if !plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("BgColor with A>0 should return true")
	}
}

func TestPlainTextNeedsGlyphLayoutFeatures(t *testing.T) {
	s := &Shape{}
	tc := &shapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{Features: &glyph.FontFeatures{}}
	if !plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("non-nil Features should return true")
	}
}

func TestPlainTextLayoutWidthArgNils(t *testing.T) {
	if plainTextLayoutWidthArg(nil, &shapeTextConfig{}, TextStyle{}) != 0 {
		t.Error("nil shape should return 0")
	}
	if plainTextLayoutWidthArg(&Shape{}, nil, TextStyle{}) != 0 {
		t.Error("nil tc should return 0")
	}
	s := &Shape{}
	s.Width = 0
	if plainTextLayoutWidthArg(s, &shapeTextConfig{}, TextStyle{}) != 0 {
		t.Error("zero width should return 0")
	}
}

func TestPlainTextLayoutWidthArgWrap(t *testing.T) {
	s := &Shape{}
	s.Width = 200
	tc := &shapeTextConfig{TextMode: TextModeWrap}
	if got := plainTextLayoutWidthArg(s, tc, TextStyle{}); got != 200 {
		t.Errorf("got %f, want 200", got)
	}
}

func TestPlainTextLayoutWidthArgNonLeft(t *testing.T) {
	s := &Shape{}
	s.Width = 200
	tc := &shapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{Align: TextAlignCenter}
	if got := plainTextLayoutWidthArg(s, tc, style); got != -200 {
		t.Errorf("got %f, want -200", got)
	}
}

func TestPlainTextLayoutResolvedNilWindow(t *testing.T) {
	s := &Shape{TC: &shapeTextConfig{}}
	_, ok := plainTextLayoutResolved("test", s, TextStyle{}, nil)
	if ok {
		t.Error("nil window should return false")
	}
}

// plainTextHeightNoMeasurer is what keeps headless wrapped text from
// reporting one line tall. Asserted as a line count rather than a pixel
// height: the per-rune width is an approximation and may be retuned,
// but "more text in the same width is more lines" must hold.
func TestPlainTextHeightNoMeasurer(t *testing.T) {
	style := TextStyle{Size: 10}
	lineH := fallbackLineHeight(style)

	measure := func(text string, mode textMode, width float32) float32 {
		tc := &shapeTextConfig{Text: text, TextMode: mode}
		s := &Shape{Width: width, TC: tc}
		return plainTextHeightNoMeasurer(s, tc, style, nil)
	}

	// Single line, no wrapping: exactly one line however wide it is.
	if got := measure("hello", TextModeSingleLine, 10); got != lineH {
		t.Errorf("single line height = %v, want %v", got, lineH)
	}
	// Hard newlines count even without wrapping.
	if got := measure("a\nb\nc", TextModeMultiline, 500); got != 3*lineH {
		t.Errorf("three hard lines = %v, want %v", got, 3*lineH)
	}
	// Wrapping: 100 runes at ~6px each in a 60px box is ~10 lines.
	wrapped := measure(strings.Repeat("x", 100), TextModeWrap, 60)
	if wrapped <= lineH {
		t.Errorf("wrapped height = %v, want more than one line (%v)",
			wrapped, lineH)
	}
	// Twice the text in the same width is about twice as tall.
	twice := measure(strings.Repeat("x", 200), TextModeWrap, 60)
	if twice <= wrapped {
		t.Errorf("200 runes = %v, want more than 100 runes = %v",
			twice, wrapped)
	}
	// An unresolved width cannot wrap; fall back to the hard lines.
	if got := measure(strings.Repeat("x", 100), TextModeWrap, 0); got != lineH {
		t.Errorf("zero width = %v, want one line %v", got, lineH)
	}
	// LineSpacing widens every line.
	spaced := TextStyle{Size: 10, LineSpacing: 6}
	tc := &shapeTextConfig{Text: "a\nb", TextMode: TextModeMultiline}
	s := &Shape{Width: 500, TC: tc}
	want := 2 * (fallbackLineHeight(spaced) + 6)
	if got := plainTextHeightNoMeasurer(s, tc, spaced, nil); got != want {
		t.Errorf("line spacing height = %v, want %v", got, want)
	}
}
