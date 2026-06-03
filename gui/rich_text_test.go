package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestRichRunConstructors(t *testing.T) {
	s := TextStyle{Size: 14, Color: RGB(0, 0, 0)}

	r := RichRun("hello", s)
	if r.Text != "hello" || r.Style.Size != 14 {
		t.Fatalf("RichRun: got %q size=%v", r.Text, r.Style.Size)
	}

	br := RichBr()
	if br.Text != "\n" {
		t.Fatalf("RichBr: got %q", br.Text)
	}
}

func TestRichLinkSetsUnderline(t *testing.T) {
	s := TextStyle{Size: 14}
	r := RichLink("click", "https://example.com", s)
	if !r.Style.Underline {
		t.Fatal("RichLink should set Underline")
	}
	if r.Link != "https://example.com" {
		t.Fatalf("link: got %q", r.Link)
	}
}

func TestRichAbbrSetsBoldTypeface(t *testing.T) {
	s := TextStyle{Size: 14}
	r := RichAbbr("HTML", "HyperText Markup Language", s)
	if r.Style.Typeface != glyph.TypefaceBold {
		t.Fatal("RichAbbr should set TypefaceBold")
	}
	if r.Tooltip != "HyperText Markup Language" {
		t.Fatalf("tooltip: got %q", r.Tooltip)
	}
}

func TestRichFootnoteReducesSize(t *testing.T) {
	s := TextStyle{Size: 20}
	r := RichFootnote("1", "footnote text", s)
	if r.Style.Size >= 20 {
		t.Fatalf("size should be reduced: got %v", r.Style.Size)
	}
	if r.Tooltip != "footnote text" {
		t.Fatalf("tooltip: got %q", r.Tooltip)
	}
}

func TestRichTextToGlyphConversion(t *testing.T) {
	rt := RichText{
		Runs: []RichTextRun{
			{Text: "hello ", Style: TextStyle{Size: 14}},
			{Text: "world", Style: TextStyle{Size: 16}},
		},
	}
	grt := rt.toGlyphRichText()
	if len(grt.Runs) != 2 {
		t.Fatalf("expected 2 glyph runs, got %d", len(grt.Runs))
	}
	if grt.Runs[0].Text != "hello " {
		t.Fatalf("run 0 text: got %q", grt.Runs[0].Text)
	}
	if grt.Runs[1].Text != "world" {
		t.Fatalf("run 1 text: got %q", grt.Runs[1].Text)
	}
}

func TestMathRunEmitsInlineObject(t *testing.T) {
	cache := NewBoundedDiagramCache(10)
	hash := diagramCacheHash("x^2")
	cache.Set(hash, DiagramCacheEntry{
		State:  DiagramReady,
		Width:  120,
		Height: 40,
		DPI:    150,
	})
	rt := RichText{
		Runs: []RichTextRun{
			{Text: "before ", Style: TextStyle{Size: 12}},
			{
				MathID:    "x^2",
				MathLatex: "x^2",
				Style:     TextStyle{Size: 12},
			},
			{Text: " after", Style: TextStyle{Size: 12}},
		},
	}
	grt, mh := rt.toGlyphRichTextWithMath(cache)
	if len(grt.Runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(grt.Runs))
	}
	mid := grt.Runs[1]
	if mid.Text != "\uFFFC" {
		t.Fatalf("expected ORC placeholder, got %q", mid.Text)
	}
	if mid.Style.Object == nil {
		t.Fatal("expected InlineObject, got nil")
	}
	if mid.Style.Object.ID != "x^2" {
		t.Fatalf("object ID: got %q", mid.Style.Object.ID)
	}
	// 120 * (72/150) * (12/12) = 57.6
	if mid.Style.Object.Width < 57 || mid.Style.Object.Width > 58 {
		t.Fatalf("width: got %f", mid.Style.Object.Width)
	}
	// Offset = fontSize*0.4 - h/2 = 12*0.4 - 19.2/2 = -4.8
	if mid.Style.Object.Offset < -4.9 || mid.Style.Object.Offset > -4.7 {
		t.Fatalf("offset: got %f, want ~-4.8", mid.Style.Object.Offset)
	}
	if len(mh) != 1 || mh[0] != diagramCacheHash("x^2") {
		t.Fatalf("mathHashes: got %v", mh)
	}
}

func TestMathRunFallbackWhenLoading(t *testing.T) {
	cache := NewBoundedDiagramCache(10)
	hash := diagramCacheHash("y^2")
	cache.Set(hash, DiagramCacheEntry{
		State: DiagramLoading,
	})
	rt := RichText{
		Runs: []RichTextRun{{
			MathID:    "y^2",
			MathLatex: "y^2",
			Style:     TextStyle{Size: 12},
		}},
	}
	grt, mh := rt.toGlyphRichTextWithMath(cache)
	if grt.Runs[0].Text != "y^2" {
		t.Fatalf("expected fallback text, got %q", grt.Runs[0].Text)
	}
	if grt.Runs[0].Style.Object != nil {
		t.Fatal("should not create InlineObject for loading entry")
	}
	if len(mh) != 0 {
		t.Fatalf("expected no mathHashes for loading, got %v", mh)
	}
}

func TestMathRunFallbackNilCache(t *testing.T) {
	rt := RichText{
		Runs: []RichTextRun{{
			MathID:    "z",
			MathLatex: "z",
			Style:     TextStyle{Size: 14},
		}},
	}
	grt, _ := rt.toGlyphRichTextWithMath(nil)
	if grt.Runs[0].Text != "z" {
		t.Fatalf("expected fallback, got %q", grt.Runs[0].Text)
	}
}

func TestRichTextPlain(t *testing.T) {
	rt := RichText{
		Runs: []RichTextRun{
			{Text: "hello "},
			{Text: "world"},
		},
	}
	got := richTextPlain(rt)
	if got != "hello world" {
		t.Fatalf("plain text: got %q", got)
	}
}
