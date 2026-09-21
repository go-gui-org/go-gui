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

// TestRichLinkForcesThemeColor pins the link role against the
// documented call, RichLink(text, url, t.TextStyleBody). Every theme style
// carries a color, so a constructor that honoured a caller color
// would draw that call in plain body color with only an underline
// to mark it as a link. A run that needs its own color is built as
// a RichTextRun literal instead.
func TestRichLinkForcesThemeColor(t *testing.T) {
	themed := guiTheme.TextStyleBody
	if !themed.Color.IsSet() {
		t.Fatal("Theme.TextStyleBody carries no color; the test proves nothing")
	}
	r := RichLink("click", "https://example.com", themed)
	if r.Style.Color != guiTheme.ColorSelect {
		t.Fatalf("theme style: color = %v, want ColorSelect %v",
			r.Style.Color, guiTheme.ColorSelect)
	}
	explicit := TextStyle{Size: 14, Color: RGB(10, 20, 30)}
	e := RichLink("click", "https://example.com", explicit)
	if e.Style.Color != guiTheme.ColorSelect {
		t.Fatalf("explicit style: color = %v, want ColorSelect %v",
			e.Style.Color, guiTheme.ColorSelect)
	}
	// The escape hatch the doc comment names.
	lit := RichTextRun{
		Text: "click", Link: "https://example.com",
		Style: explicit,
	}
	if lit.Style.Color != explicit.Color {
		t.Fatalf("literal run color = %v, want it kept",
			lit.Style.Color)
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
	cache := newBoundedDiagramCache(10)
	hash := diagramCacheHash("x^2")
	cache.Set(hash, DiagramCacheEntry{
		State:  diagramReady,
		Width:  120,
		Height: 40,
		dPI:    150,
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
	cache := newBoundedDiagramCache(10)
	hash := diagramCacheHash("y^2")
	cache.Set(hash, DiagramCacheEntry{
		State: diagramLoading,
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

// TestMathRunZeroSizeFallsBack pins the degenerate guard: a ready
// cache entry with an unset run size must not produce a zero-size
// inline object — the run falls back to its LaTeX source, in
// agreement with rtfFlatTextFromRuns.
func TestMathRunZeroSizeFallsBack(t *testing.T) {
	cache := newBoundedDiagramCache(10)
	cache.Set(diagramCacheHash("m"), DiagramCacheEntry{
		State:  diagramReady,
		Width:  120,
		Height: 40,
		dPI:    150,
	})
	rt := RichText{
		Runs: []RichTextRun{{
			MathID:    "m",
			MathLatex: "m",
			Style:     TextStyle{},
		}},
	}
	grt, mh := rt.toGlyphRichTextWithMath(cache)
	if grt.Runs[0].Text != "m" {
		t.Fatalf("expected fallback, got %q", grt.Runs[0].Text)
	}
	if grt.Runs[0].Style.Object != nil {
		t.Fatal("zero-size run must not produce an InlineObject")
	}
	if len(mh) != 0 {
		t.Fatalf("expected no mathHashes, got %v", mh)
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

// TestMathRunScaleOverflowFallsBack pins the product guard: every
// input can be finite and positive while the scaled object size
// still overflows float32 to +Inf. The run must fall back to its
// LaTeX source rather than shape an infinite object.
func TestMathRunScaleOverflowFallsBack(t *testing.T) {
	cache := newBoundedDiagramCache(4)
	cache.Set(diagramCacheHash("m"), DiagramCacheEntry{
		State: diagramReady, Width: 80, Height: 24,
		dPI: 1e-40, // denormal: 72/dPI overflows the scale
	})
	run := RichTextRun{
		MathID: "m", MathLatex: "a+b",
		Style: TextStyle{Size: 12},
	}
	if _, _, ok := rtfMathObject(&run, cache); ok {
		t.Fatal("an overflowing scale must not shape an object")
	}
	rt := RichText{Runs: []RichTextRun{run}}
	grt, mh := rt.toGlyphRichTextWithMath(cache)
	if grt.Runs[0].Text != "a+b" || grt.Runs[0].Style.Object != nil {
		t.Fatalf("expected LaTeX fallback, got %q object=%v",
			grt.Runs[0].Text, grt.Runs[0].Style.Object)
	}
	if len(mh) != 0 {
		t.Fatalf("expected no mathHashes, got %v", mh)
	}
	// Flat text must agree, or selection offsets drift.
	flat, _ := rtfFlatTextFromRuns(&rt, cache)
	if flat != "a+b" {
		t.Fatalf("flat text = %q, want the same fallback", flat)
	}
}

// TestMathRunScaleUnderflowFallsBack pins the other end of the
// product guard: a scale that underflows the object to zero is as
// unusable as an infinite one.
func TestMathRunScaleUnderflowFallsBack(t *testing.T) {
	cache := newBoundedDiagramCache(4)
	cache.Set(diagramCacheHash("m"), DiagramCacheEntry{
		State: diagramReady, Width: 1e-30, Height: 1e-30,
		dPI: 3e38,
	})
	run := RichTextRun{
		MathID: "m", MathLatex: "a+b",
		Style: TextStyle{Size: 12},
	}
	if _, _, ok := rtfMathObject(&run, cache); ok {
		t.Fatal("an underflowing scale must not shape an object")
	}
}
