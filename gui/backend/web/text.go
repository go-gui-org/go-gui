//go:build js && wasm

package web

import (
	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
)

// textMeasurer wraps glyph.TextSystem to implement gui.TextMeasurer.
type textMeasurer struct {
	textSys *glyph.TextSystem
}

func (tm *textMeasurer) TextWidth(
	text string, style gui.TextStyle) float32 {
	cfg := glyphconv.GuiStyleToGlyphConfig(style)
	w, err := tm.textSys.TextWidth(text, cfg)
	if err != nil {
		return 0
	}
	return w
}

func (tm *textMeasurer) TextHeight(
	text string, style gui.TextStyle) float32 {
	cfg := glyphconv.GuiStyleToGlyphConfig(style)
	h, err := tm.textSys.TextHeight(text, cfg)
	if err != nil {
		return 0
	}
	return h
}

func (tm *textMeasurer) FontHeight(
	style gui.TextStyle) float32 {
	cfg := glyphconv.GuiStyleToGlyphConfig(style)
	h, err := tm.textSys.FontHeight(cfg)
	if err != nil {
		return style.Size * 1.4
	}
	return h
}

func (tm *textMeasurer) FontAscent(
	style gui.TextStyle) float32 {
	cfg := glyphconv.GuiStyleToGlyphConfig(style)
	m, err := tm.textSys.FontMetrics(cfg)
	if err != nil {
		return style.Size * 0.8
	}
	return m.Ascender
}

// TextInkBounds returns the painted box of text, relative to the
// top-left of its advance box. Backs gui's optional ink-measuring
// capability, which widgets use to centre a single glyph on its ink
// instead of on the font's advance box.
func (tm *textMeasurer) TextInkBounds(
	text string, style gui.TextStyle) (gui.InkBounds, bool) {
	cfg := glyphconv.GuiStyleToGlyphConfig(style)
	r, ok := tm.textSys.InkBounds(text, cfg)
	if !ok {
		return gui.InkBounds{}, false
	}
	return gui.InkBounds{
		X: r.X, Y: r.Y, Width: r.Width, Height: r.Height,
	}, true
}

func (tm *textMeasurer) LayoutText(
	text string, style gui.TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	cfg := glyphconv.GuiStyleToGlyphConfig(style)
	if wrapWidth > 0 {
		cfg.Block.Width = wrapWidth
		cfg.Block.Wrap = glyph.WrapWord
	} else if wrapWidth < 0 {
		cfg.Block.Width = -wrapWidth
		cfg.Block.Wrap = glyph.WrapNone
	}
	return tm.textSys.LayoutText(text, cfg)
}

func (tm *textMeasurer) LayoutRichText(
	rt glyph.RichText, cfg glyph.TextConfig,
) (glyph.Layout, error) {
	return tm.textSys.LayoutRichText(rt, cfg)
}
func (tm *textMeasurer) ListFontFamilies() []string {
	return tm.textSys.ListFontFamilies()
}
