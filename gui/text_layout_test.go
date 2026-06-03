package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestPlainTextNeedsGlyphLayoutNils(t *testing.T) {
	if plainTextNeedsGlyphLayout(nil, &ShapeTextConfig{}, TextStyle{}) {
		t.Error("nil shape should return false")
	}
	if plainTextNeedsGlyphLayout(&Shape{}, nil, TextStyle{}) {
		t.Error("nil tc should return false")
	}
}

func TestPlainTextNeedsGlyphLayoutDefault(t *testing.T) {
	s := &Shape{}
	tc := &ShapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{Align: TextAlignLeft}
	if plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("default single-line left-aligned should return false")
	}
}

func TestPlainTextNeedsGlyphLayoutWrap(t *testing.T) {
	s := &Shape{}
	tc := &ShapeTextConfig{TextMode: TextModeWrap}
	if !plainTextNeedsGlyphLayout(s, tc, TextStyle{}) {
		t.Error("TextModeWrap should return true")
	}
}

func TestPlainTextNeedsGlyphLayoutCenter(t *testing.T) {
	s := &Shape{}
	tc := &ShapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{Align: TextAlignCenter}
	if !plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("TextAlignCenter should return true")
	}
}

func TestPlainTextNeedsGlyphLayoutSpacing(t *testing.T) {
	s := &Shape{}
	tc := &ShapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{LineSpacing: 1.5}
	if !plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("non-zero LineSpacing should return true")
	}
}

func TestPlainTextNeedsGlyphLayoutBgColor(t *testing.T) {
	s := &Shape{}
	tc := &ShapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{BgColor: Color{R: 255, G: 0, B: 0, A: 128}}
	if !plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("BgColor with A>0 should return true")
	}
}

func TestPlainTextNeedsGlyphLayoutFeatures(t *testing.T) {
	s := &Shape{}
	tc := &ShapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{Features: &glyph.FontFeatures{}}
	if !plainTextNeedsGlyphLayout(s, tc, style) {
		t.Error("non-nil Features should return true")
	}
}

func TestPlainTextLayoutWidthArgNils(t *testing.T) {
	if plainTextLayoutWidthArg(nil, &ShapeTextConfig{}, TextStyle{}) != 0 {
		t.Error("nil shape should return 0")
	}
	if plainTextLayoutWidthArg(&Shape{}, nil, TextStyle{}) != 0 {
		t.Error("nil tc should return 0")
	}
	s := &Shape{}
	s.Width = 0
	if plainTextLayoutWidthArg(s, &ShapeTextConfig{}, TextStyle{}) != 0 {
		t.Error("zero width should return 0")
	}
}

func TestPlainTextLayoutWidthArgWrap(t *testing.T) {
	s := &Shape{}
	s.Width = 200
	tc := &ShapeTextConfig{TextMode: TextModeWrap}
	if got := plainTextLayoutWidthArg(s, tc, TextStyle{}); got != 200 {
		t.Errorf("got %f, want 200", got)
	}
}

func TestPlainTextLayoutWidthArgNonLeft(t *testing.T) {
	s := &Shape{}
	s.Width = 200
	tc := &ShapeTextConfig{TextMode: TextModeSingleLine}
	style := TextStyle{Align: TextAlignCenter}
	if got := plainTextLayoutWidthArg(s, tc, style); got != -200 {
		t.Errorf("got %f, want -200", got)
	}
}

func TestPlainTextLayoutResolvedNilWindow(t *testing.T) {
	s := &Shape{TC: &ShapeTextConfig{}}
	_, ok := plainTextLayoutResolved("test", s, TextStyle{}, nil)
	if ok {
		t.Error("nil window should return false")
	}
}
