package gui

// view_markdown_blocks_test.go covers the block renderers that the
// interaction suite never reaches: HR, image and definition-list
// blocks (view_markdown_blocks.go). These are pure view factories —
// the interactive suite is the right place for click/keyboard
// behavior, but a divider or a definition term has no behavior to
// drive, so shape structure is asserted directly, in the style of
// TestRenderMdMathErrorsWrap.

import (
	"os"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestMdRenderHR(t *testing.T) {
	style := DefaultMarkdownStyle()
	layout := generateViewLayout(mdRenderHR(MarkdownCfg{
		Style: style,
	}), &Window{})

	shape := layout.Shape
	if shape == nil {
		t.Fatal("expected shape")
	}
	if shape.Height != 1 {
		t.Errorf("HR height = %g, want 1", shape.Height)
	}
	if shape.Color != style.hRColor {
		t.Errorf("HR color = %v, want %v", shape.Color, style.hRColor)
	}
	if shape.Sizing != FillFixed {
		t.Errorf("HR sizing = %v, want FillFixed", shape.Sizing)
	}
}

// TestMdRenderImage asserts an image block becomes a shapeImage with
// the configured path and dimensions. The source path must exist —
// Image.GenerateLayout stats it — so the test points at a real temp
// file.
func TestMdRenderImage(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "md_render_image_*.png")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	_ = f.Close()

	layout := generateViewLayout(mdRenderImage(markdownBlock{
		IsImage:     true,
		ImageSrc:    path,
		ImageWidth:  123,
		ImageHeight: 45,
	}), &Window{})

	shape := layout.Shape
	if shape == nil {
		t.Fatal("expected shape")
	}
	if shape.shapeType != shapeImage {
		t.Errorf("shape type = %v, want shapeImage", shape.shapeType)
	}
	if shape.Resource != path {
		t.Errorf("Resource = %q, want %q", shape.Resource, path)
	}
	if shape.Width != 123 || shape.Height != 45 {
		t.Errorf("size = %gx%g, want 123x45",
			shape.Width, shape.Height)
	}
}

// mdDefListBlocks parses a definition list and returns its term and
// value blocks, sharing the real parser so the renderers see exactly
// what markdownBuildContent would hand them.
func mdDefListBlocks(t *testing.T) (term, value markdownBlock) {
	t.Helper()
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks("Term\n:   Definition text", style)
	for _, b := range blocks {
		switch {
		case b.IsDefTerm:
			term = b
		case b.IsDefValue:
			value = b
		}
	}
	if !term.IsDefTerm || !value.IsDefValue {
		t.Fatalf("definition list not parsed: term=%v value=%v",
			term.IsDefTerm, value.IsDefValue)
	}
	return term, value
}

// TestMdRenderDefTerm asserts the term renders as an RTF block with
// the term text, styled bold (styleMdBlock selects style.Bold for
// terms), and stamps the markdown block context when one is supplied.
func TestMdRenderDefTerm(t *testing.T) {
	term, _ := mdDefListBlocks(t)
	style := DefaultMarkdownStyle()

	// No context: the block must not claim a markdown ID.
	layout := generateViewLayout(
		mdRenderDefTerm(term, TextModeSingleLine, nil), &Window{})
	tc := layout.Shape.TC
	if tc == nil {
		t.Fatal("expected text config on term shape")
	}
	if tc.rTFFlatText != "Term" {
		t.Errorf("term text = %q, want %q", tc.rTFFlatText, "Term")
	}
	if tc.markdownID != "" {
		t.Error("term without mdBlockCtx claimed a markdown ID")
	}
	if term.baseStyle.Typeface != glyphBoldTypeface(style) {
		t.Errorf("term base style typeface = %v, want bold",
			term.baseStyle.Typeface)
	}

	// With context: the block joins the markdown's virtual flat text.
	layout = generateViewLayout(mdRenderDefTerm(term,
		TextModeSingleLine, &mdBlockCtx{ID: "md", Start: 42}), &Window{})
	tc = layout.Shape.TC
	if tc.markdownID != "md" {
		t.Errorf("markdownID = %q, want %q", tc.markdownID, "md")
	}
	if tc.markdownBlockStart != 42 {
		t.Errorf("markdownBlockStart = %d, want 42",
			tc.markdownBlockStart)
	}
	if tc.markdownRuneLen != uint32(utf8RuneCount("Term")) {
		t.Errorf("markdownRuneLen = %d, want %d",
			tc.markdownRuneLen, utf8RuneCount("Term"))
	}
}

// glyphBoldTypeface returns the bold typeface the theme uses, so the
// assertion above stays tied to the theme rather than a magic value.
func glyphBoldTypeface(style MarkdownStyle) glyph.Typeface {
	return style.Bold.Typeface
}

// TestMdRenderDefValue asserts the value renders as an RTF block
// indented by the style's nestIndent and carrying the value text with
// the plain text style.
func TestMdRenderDefValue(t *testing.T) {
	_, value := mdDefListBlocks(t)
	style := DefaultMarkdownStyle()

	layout := generateViewLayout(mdRenderDefValue(value,
		MarkdownCfg{Style: style}, TextModeSingleLine,
		&mdBlockCtx{ID: "md", Start: 4}), &Window{})

	shape := layout.Shape
	if shape == nil {
		t.Fatal("expected shape")
	}
	if shape.Padding.Left != style.nestIndent {
		t.Errorf("value padding left = %g, want %g",
			shape.Padding.Left, style.nestIndent)
	}
	if len(layout.Children) != 1 {
		t.Fatalf("value children = %d, want 1 (the RTF)",
			len(layout.Children))
	}
	tc := layout.Children[0].Shape.TC
	if tc == nil || tc.rTFFlatText != "Definition text" {
		t.Errorf("value text = %q, want %q",
			tc.rTFFlatText, "Definition text")
	}
	if tc.markdownID != "md" || tc.markdownBlockStart != 4 {
		t.Errorf("value block ctx = %q@%d, want md@4",
			tc.markdownID, tc.markdownBlockStart)
	}
	if value.baseStyle.Typeface != glyphBoldTypeface(style) &&
		value.baseStyle.Typeface != style.Text.Typeface {
		t.Errorf("value base style typeface = %v, want plain %v",
			value.baseStyle.Typeface, style.Text.Typeface)
	}
}

// TestMdHeaderStyle pins the header-level → style mapping, including
// the default fallback (h6) for out-of-range levels.
func TestMdHeaderStyle(t *testing.T) {
	style := DefaultMarkdownStyle()
	tests := []struct {
		level int
		want  TextStyle
	}{
		{1, style.h1},
		{2, style.H2},
		{3, style.h3},
		{4, style.h4},
		{5, style.h5},
		{6, style.h6},
		{0, style.h6}, // not a heading
		{7, style.h6}, // out of range
	}
	for _, tt := range tests {
		if got := mdHeaderStyle(tt.level, style); got != tt.want {
			t.Errorf("mdHeaderStyle(%d) = %v, want %v",
				tt.level, got, tt.want)
		}
	}
}

// TestMarkdownBuildContentRendersHRInPipeline asserts an HR source
// produces a divider shape through markdownBuildContent, the path a
// rendered document actually takes (the direct mdRenderHR test above
// pins the factory itself).
func TestMarkdownBuildContentRendersHRInPipeline(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks("before\n\n---\n\nafter", style)
	content := markdownBuildContent(blocks, MarkdownCfg{
		Style: style,
	}, TextModeWrap, &Window{})

	// "before", HR, "after" — three children; the HR is the middle one.
	if len(content) != 3 {
		t.Fatalf("content blocks = %d, want 3", len(content))
	}
	layout := generateViewLayout(content[1], &Window{})
	if layout.Shape == nil || layout.Shape.Height != 1 {
		t.Errorf("pipeline HR height = %v, want 1", layout.Shape)
	}
	if layout.Shape.Color != style.hRColor {
		t.Errorf("pipeline HR color = %v, want %v",
			layout.Shape.Color, style.hRColor)
	}
}
