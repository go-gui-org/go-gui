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
		Color:         gui.Color{R: 255, G: 128, B: 64, A: 255},
		BgColor:       gui.Color{R: 0, G: 0, B: 0, A: 128},
		Typeface:      glyph.TypefaceItalic,
		Underline:     true,
		Strikethrough: true,
		LetterSpacing: 2.5,
		StrokeWidth:   1.5,
		StrokeColor:   gui.Color{R: 255, G: 0, B: 0, A: 255},
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
