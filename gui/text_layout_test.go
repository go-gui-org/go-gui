package gui

import (
	"math"
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
// plainTextBoxHeight converts glyph's whole-line-box height into the
// ascent..descent box the single-line path uses: the trailing leading
// below the last baseline is dropped, inter-line spacing kept, and a
// face whose line box is tighter than ascent+descent keeps glyph's
// number untouched (growing the shape would be a regression).
func TestPlainTextBoxHeight(t *testing.T) {
	style := TextStyle{Size: 16}
	// fh controls fontHeight via the window's measurer.
	w := &Window{}
	w.textMeasurer = &stubTextMeasurer{fontHeight: 20}

	lines := func(n int) []glyph.Line { return make([]glyph.Line, n) }

	cases := []struct {
		name string
		l    glyph.Layout
		want float32
	}{
		{
			name: "one line drops trailing leading",
			l:    glyph.Layout{Height: 23, Lines: lines(1)},
			want: 20, // fh, not the 23px line box
		},
		{
			name: "two lines keep inter-line spacing",
			l:    glyph.Layout{Height: 46, Lines: lines(2)},
			want: 43, // 23px line box + 20px last line
		},
		{
			name: "empty lines keep layout height",
			l:    glyph.Layout{Height: 23},
			want: 23,
		},
		{
			name: "zero height passes through",
			l:    glyph.Layout{Height: 0, Lines: lines(1)},
			want: 0,
		},
		{
			name: "non-finite height passes through untouched",
			l:    glyph.Layout{Height: float32(math.NaN()), Lines: lines(1)},
			want: float32(math.NaN()),
		},
		{
			// fh 20 >= lineH 20: the formula would grow the shape
			// (1*20+20 = 40 > 40 is equal here; use fh > lineH below).
			name: "line box tighter than font height keeps layout height",
			l:    glyph.Layout{Height: 40, Lines: lines(2)},
			want: 40,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := plainTextBoxHeight(tc.l, style, w)
			if tc.want != tc.want { // NaN
				if got == got {
					t.Errorf("got %v, want NaN", got)
				}
				return
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}

	// fh (24) > lineH (20): growing the shape would be a regression,
	// so glyph's number stands even though the formula would say 44.
	wTight := &Window{}
	wTight.textMeasurer = &stubTextMeasurer{fontHeight: 24}
	if got := plainTextBoxHeight(glyph.Layout{
		Height: 40, Lines: lines(2),
	}, style, wTight); got != 40 {
		t.Errorf("tight line box = %v, want 40", got)
	}
}

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
