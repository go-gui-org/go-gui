// Package glyphconv converts gui text styles to glyph text configs.
package glyphconv

import (
	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
)

// GuiStyleToGlyphConfig converts a gui.TextStyle to a
// glyph.TextConfig suitable for text measurement and rendering.
func GuiStyleToGlyphConfig(s gui.TextStyle) glyph.TextConfig {
	align := glyph.AlignLeft
	switch s.Align {
	case gui.TextAlignCenter:
		align = glyph.AlignCenter
	case gui.TextAlignRight:
		align = glyph.AlignRight
	}
	return glyph.TextConfig{
		Style: glyph.TextStyle{
			FontName:      s.Family,
			Size:          s.Size,
			Color:         glyph.Color{R: s.Color.R, G: s.Color.G, B: s.Color.B, A: s.Color.A},
			BgColor:       glyph.Color{R: s.BgColor.R, G: s.BgColor.G, B: s.BgColor.B, A: s.BgColor.A},
			Typeface:      s.Typeface,
			Underline:     s.Underline,
			Strikethrough: s.Strikethrough,
			LetterSpacing: s.LetterSpacing,
			EmojiBoxWidth: s.EmojiBoxWidth,
			StrokeWidth:   s.StrokeWidth,
			StrokeColor:   glyph.Color{R: s.StrokeColor.R, G: s.StrokeColor.G, B: s.StrokeColor.B, A: s.StrokeColor.A},
			Features:      s.Features,
		},
		Block: glyph.BlockStyle{
			Align:       align,
			Wrap:        glyph.WrapWord,
			Width:       -1,
			LineSpacing: s.LineSpacing,
		},
		Gradient: s.Gradient,
	}
}
