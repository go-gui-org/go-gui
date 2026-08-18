package glyphconv

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
)

func TestGuiStyleToGlyphConfigAlign(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		guiAlign gui.TextAlignment
		want     glyph.Alignment
	}{
		{"default zero", gui.TextAlignLeft, glyph.AlignLeft},
		{"left", gui.TextAlignLeft, glyph.AlignLeft},
		{"center", gui.TextAlignCenter, glyph.AlignCenter},
		{"right", gui.TextAlignRight, glyph.AlignRight},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := gui.TextStyle{Align: c.guiAlign}
			gc := GuiStyleToGlyphConfig(s)
			if gc.Block.Align != c.want {
				t.Errorf("align: got %v, want %v",
					gc.Block.Align, c.want)
			}
		})
	}
}

func TestGuiStyleToGlyphConfigFieldMapping(t *testing.T) {
	t.Parallel()
	s := gui.TextStyle{
		Family:        "Test Font",
		Size:          16,
		Color:         gui.RGBA(255, 128, 64, 255),
		BgColor:       gui.RGBA(0, 0, 0, 128),
		Typeface:      glyph.TypefaceItalic,
		Underline:     true,
		Strikethrough: true,
		LetterSpacing: 2.5,
		StrokeWidth:   1.5,
		StrokeColor:   gui.RGBA(255, 0, 0, 255),
		LineSpacing:   1.5,
	}
	gc := GuiStyleToGlyphConfig(s)

	if gc.Style.FontName != "Test Font" {
		t.Errorf("FontName: got %q, want %q", gc.Style.FontName, "Test Font")
	}
	if gc.Style.Size != 16 {
		t.Errorf("Size: got %v, want 16", gc.Style.Size)
	}
	if gc.Style.Color.R != 255 || gc.Style.Color.G != 128 ||
		gc.Style.Color.B != 64 || gc.Style.Color.A != 255 {
		t.Errorf("Color mismatch")
	}
	if gc.Style.BgColor.R != 0 || gc.Style.BgColor.A != 128 {
		t.Errorf("BgColor mismatch")
	}
	if gc.Style.Typeface != glyph.TypefaceItalic {
		t.Errorf("Typeface mismatch")
	}
	if !gc.Style.Underline {
		t.Error("Underline should be true")
	}
	if !gc.Style.Strikethrough {
		t.Error("Strikethrough should be true")
	}
	if gc.Style.LetterSpacing != 2.5 {
		t.Errorf("LetterSpacing: got %v, want 2.5",
			gc.Style.LetterSpacing)
	}
	if gc.Style.StrokeWidth != 1.5 {
		t.Errorf("StrokeWidth: got %v, want 1.5",
			gc.Style.StrokeWidth)
	}
	if gc.Style.StrokeColor.R != 255 {
		t.Errorf("StrokeColor mismatch")
	}
	if gc.Block.LineSpacing != 1.5 {
		t.Errorf("LineSpacing: got %v, want 1.5",
			gc.Block.LineSpacing)
	}
	// Block defaults.
	if gc.Block.Width != -1 {
		t.Errorf("Block.Width: got %v, want -1", gc.Block.Width)
	}
	if gc.Block.Wrap != glyph.WrapWord {
		t.Errorf("Block.Wrap: got %v, want WrapWord", gc.Block.Wrap)
	}
	// Features and Gradient pass through as nil when not set.
	if gc.Style.Features != nil {
		t.Errorf("Features: expected nil, got %v", gc.Style.Features)
	}
	if gc.Gradient != nil {
		t.Errorf("Gradient: expected nil, got %v", gc.Gradient)
	}
}

// The plain-text path is the one a terminal grid actually renders through, so
// the cell size and box-glyph opt-out must survive this conversion too.
func TestGuiStyleToGlyphConfigGridGeometry(t *testing.T) {
	t.Parallel()
	s := gui.TextStyle{
		Family:             "Mono",
		Size:               14,
		EmojiBoxWidth:      19,
		CellWidth:          9.5,
		CellHeight:         20,
		NoBuiltinBoxGlyphs: true,
	}
	gc := GuiStyleToGlyphConfig(s)
	if gc.Style.EmojiBoxWidth != 19 {
		t.Errorf("EmojiBoxWidth: got %v, want 19", gc.Style.EmojiBoxWidth)
	}
	if gc.Style.CellWidth != 9.5 {
		t.Errorf("CellWidth: got %v, want 9.5", gc.Style.CellWidth)
	}
	if gc.Style.CellHeight != 20 {
		t.Errorf("CellHeight: got %v, want 20", gc.Style.CellHeight)
	}
	if !gc.Style.NoBuiltinBoxGlyphs {
		t.Error("NoBuiltinBoxGlyphs should be true")
	}
}

func TestGuiStyleToGlyphConfigFeaturesGradient(t *testing.T) {
	t.Parallel()
	feat := &glyph.FontFeatures{
		OpenTypeFeatures: []glyph.FontFeature{
			{Tag: "kern", Value: 1},
			{Tag: "liga", Value: 1},
		},
	}
	grad := &glyph.GradientConfig{
		Direction: glyph.GradientHorizontal,
		Stops: []glyph.GradientStop{
			{Color: glyph.Color{R: 255, G: 0, B: 0, A: 255}, Position: 0},
			{Color: glyph.Color{R: 0, G: 0, B: 255, A: 255}, Position: 1},
		},
	}
	s := gui.TextStyle{
		Features: feat,
		Gradient: grad,
	}
	gc := GuiStyleToGlyphConfig(s)
	if gc.Style.Features != feat {
		t.Error("Features pointer not passed through")
	}
	if gc.Gradient != grad {
		t.Error("Gradient pointer not passed through")
	}
}

func TestGuiStyleToGlyphConfigZeroValue(t *testing.T) {
	t.Parallel()
	gc := GuiStyleToGlyphConfig(gui.TextStyle{})
	// Zero-value gui.TextStyle should map to sensible defaults.
	if gc.Block.Align != glyph.AlignLeft {
		t.Errorf("zero-value align: got %v, want AlignLeft",
			gc.Block.Align)
	}
	if gc.Block.Wrap != glyph.WrapWord {
		t.Errorf("zero-value wrap: got %v, want WrapWord",
			gc.Block.Wrap)
	}
	if gc.Block.Width != -1 {
		t.Errorf("zero-value width: got %v, want -1",
			gc.Block.Width)
	}
}

// The flat fallback is what every backend renders when a RenderText
// command has no TextStylePtr; the fields must map exactly so the six
// backends cannot drift (issue #362).
func TestGuiTextConfigFromRenderFallback(t *testing.T) {
	t.Parallel()
	r := &gui.RenderCmd{
		FontName: "Fallback Font",
		FontSize: 13.5,
		Color:    gui.RGBA(10, 20, 30, 40),
	}
	gc := GuiTextConfigFromRender(r)

	if gc.Style.FontName != "Fallback Font" {
		t.Errorf("FontName: got %q, want %q",
			gc.Style.FontName, "Fallback Font")
	}
	if gc.Style.Size != 13.5 {
		t.Errorf("Size: got %v, want 13.5", gc.Style.Size)
	}
	if gc.Style.Color.R != 10 || gc.Style.Color.G != 20 ||
		gc.Style.Color.B != 30 || gc.Style.Color.A != 40 {
		t.Errorf("Color mismatch: got %+v", gc.Style.Color)
	}
	if gc.Style.BgColor != (glyph.Color{}) {
		t.Errorf("BgColor: expected zero, got %+v", gc.Style.BgColor)
	}
	if gc.Gradient != nil {
		t.Errorf("Gradient: expected nil, got %v", gc.Gradient)
	}
	// The fallback must not inherit the style converter's defaults:
	// the pre-issue blocks used DefaultBlockStyle() verbatim.
	def := glyph.DefaultBlockStyle()
	if gc.Block.Align != def.Align || gc.Block.Wrap != def.Wrap ||
		gc.Block.Width != def.Width || gc.Block.Indent != def.Indent ||
		gc.Block.LineSpacing != def.LineSpacing ||
		len(gc.Block.Tabs) != len(def.Tabs) {
		t.Errorf("Block: got %+v, want DefaultBlockStyle %+v",
			gc.Block, def)
	}
}

func TestGuiTextConfigFromRenderWrapWidth(t *testing.T) {
	t.Parallel()
	r := &gui.RenderCmd{W: 240}
	gc := GuiTextConfigFromRender(r)
	if gc.Block.Wrap != glyph.WrapWord {
		t.Errorf("Wrap: got %v, want WrapWord", gc.Block.Wrap)
	}
	if gc.Block.Width != 240 {
		t.Errorf("Width: got %v, want 240", gc.Block.Width)
	}

	// No width: keep the default block style untouched.
	gc = GuiTextConfigFromRender(&gui.RenderCmd{})
	def := glyph.DefaultBlockStyle()
	if gc.Block.Wrap != def.Wrap {
		t.Errorf("Wrap: got %v, want %v", gc.Block.Wrap, def.Wrap)
	}
	if gc.Block.Width != def.Width {
		t.Errorf("Width: got %v, want %v", gc.Block.Width, def.Width)
	}

	// Negative width must not enable wrapping.
	gc = GuiTextConfigFromRender(&gui.RenderCmd{W: -100})
	if gc.Block.Wrap != def.Wrap {
		t.Errorf("negative width: Wrap got %v, want %v",
			gc.Block.Wrap, def.Wrap)
	}
	if gc.Block.Width != def.Width {
		t.Errorf("negative width: Width got %v, want %v",
			gc.Block.Width, def.Width)
	}
}

func TestGuiTextConfigFromRenderStyled(t *testing.T) {
	t.Parallel()
	style := gui.TextStyle{
		Family: "Styled",
		Size:   18,
		Color:  gui.RGBA(1, 2, 3, 4),
	}
	grad := &glyph.GradientConfig{
		Direction: glyph.GradientVertical,
		Stops: []glyph.GradientStop{
			{Color: glyph.Color{R: 255, A: 255}, Position: 0},
			{Color: glyph.Color{R: 0, A: 255}, Position: 1},
		},
	}
	r := &gui.RenderCmd{
		TextStylePtr: &style,
		TextGradient: grad,
		W:            120,
	}
	gc := GuiTextConfigFromRender(r)

	// Styled path must agree with the style converter, plus gradient.
	want := GuiStyleToGlyphConfig(style)
	want.Gradient = grad
	want.Block.Wrap = glyph.WrapWord
	want.Block.Width = 120
	if gc.Style != want.Style {
		t.Errorf("Style: got %+v, want %+v", gc.Style, want.Style)
	}
	if gc.Gradient != grad {
		t.Errorf("Gradient pointer not passed through")
	}
	if gc.Block.Wrap != glyph.WrapWord || gc.Block.Width != 120 {
		t.Errorf("Block: got %+v, want wrap 120", gc.Block)
	}
}
