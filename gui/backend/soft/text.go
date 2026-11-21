package soft

import (
	"log"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
)

// textMeasurer wraps glyph.TextSystem to implement gui.TextMeasurer.
//
// It doubles as this package's per-window handle on the text system: a
// window that has already been prepared carries it as its measurer, so a
// repeat render recovers the system (and its warm atlas) by type
// assertion instead of a package-level cache.
type textMeasurer struct {
	textSys *glyph.TextSystem
	back    *glyphBackend
	scale   float32
}

// setScale moves the text system to a new device scale in place, as the GL
// backend's applyDPIScale does: glyph.TextSystem.SetDPIScale purges the
// layout cache, the glyph cache and the atlas, and the glyph backend places
// quads at the new density. The measurer and its text system stay the
// window's, so text blocks the window persisted are never left holding
// layouts from a freed system. It reports false when go-glyph refuses the
// scale (above its density limit), leaving everything as it was.
func (tm *textMeasurer) setScale(s float32) bool {
	if s == tm.scale {
		return true
	}
	tm.textSys.SetDPIScale(s)
	if tm.textSys.DPIScale() != s {
		return false
	}
	tm.back.scale = s
	tm.scale = s
	return true
}

func (tm *textMeasurer) TextWidth(text string, style gui.TextStyle) float32 {
	w, err := tm.textSys.TextWidth(text, guiStyleToGlyphConfig(style))
	if err != nil {
		return 0
	}
	return w
}

func (tm *textMeasurer) TextHeight(text string, style gui.TextStyle) float32 {
	h, err := tm.textSys.TextHeight(text, guiStyleToGlyphConfig(style))
	if err != nil {
		return 0
	}
	return h
}

func (tm *textMeasurer) FontHeight(style gui.TextStyle) float32 {
	h, err := tm.textSys.FontHeight(guiStyleToGlyphConfig(style))
	if err != nil {
		return style.Size * 1.4
	}
	return h
}

func (tm *textMeasurer) FontAscent(style gui.TextStyle) float32 {
	m, err := tm.textSys.FontMetrics(guiStyleToGlyphConfig(style))
	if err != nil {
		return style.Size * 0.8
	}
	return m.Ascender
}

// TextInkBounds returns the painted box of text, relative to the
// top-left of its advance box — gui's optional ink-measuring capability.
func (tm *textMeasurer) TextInkBounds(
	text string, style gui.TextStyle) (gui.InkBounds, bool) {
	r, ok := tm.textSys.InkBounds(text, guiStyleToGlyphConfig(style))
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
	return tm.layoutText(text, style, wrapWidth, false)
}

func (tm *textMeasurer) LayoutTextNoHitTesting(
	text string, style gui.TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	return tm.layoutText(text, style, wrapWidth, true)
}

func (tm *textMeasurer) layoutText(
	text string, style gui.TextStyle, wrapWidth float32, noHitTesting bool,
) (glyph.Layout, error) {
	cfg := guiStyleToGlyphConfig(style)
	cfg.NoHitTesting = noHitTesting
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

// SetHinting forwards the window's text-hinting choice to the text system
// (gui.Window.SetTextHinting, #1334).
func (tm *textMeasurer) SetHinting(on bool) {
	tm.textSys.SetHinting(on)
}

func guiStyleToGlyphConfig(s gui.TextStyle) glyph.TextConfig {
	return glyphconv.GuiStyleToGlyphConfig(s)
}

// --- Text render kinds ---

func (r *renderer) drawText(cmd *gui.RenderCmd) {
	if r.textSys == nil || cmd.Text == "" {
		return
	}
	if gui.DrawTextTransformed(cmd, r.textSys,
		guiStyleToGlyphConfig,
		func(layout glyph.Layout, grad *glyph.GradientConfig) {
			if grad != nil {
				r.textSys.DrawLayoutTransformedWithGradient(
					layout, cmd.X, cmd.Y, *cmd.LayoutTransform, grad)
			} else {
				r.textSys.DrawLayoutTransformed(
					layout, cmd.X, cmd.Y, *cmd.LayoutTransform)
			}
		}) {
		return
	}
	cfg := glyphconv.GuiTextConfigFromRender(cmd)
	if err := r.textSys.DrawText(cmd.X, cmd.Y, cmd.Text, cfg); err != nil && !r.textErrLogged {
		// Warn once per renderer: a persistent failure (e.g. missing
		// font) would otherwise log every frame.
		r.textErrLogged = true
		log.Printf("soft: DrawText: %v", err)
	}
}

func (r *renderer) drawLayout(cmd *gui.RenderCmd) {
	if r.textSys == nil || cmd.LayoutPtr == nil {
		return
	}
	if cmd.TextGradient != nil {
		r.textSys.DrawLayoutWithGradient(
			*cmd.LayoutPtr, cmd.X, cmd.Y, cmd.TextGradient)
		return
	}
	r.textSys.DrawLayout(*cmd.LayoutPtr, cmd.X, cmd.Y)
}

func (r *renderer) drawLayoutTransformed(cmd *gui.RenderCmd) {
	if r.textSys == nil || cmd.LayoutPtr == nil ||
		cmd.LayoutTransform == nil {
		return
	}
	if cmd.TextGradient != nil {
		r.textSys.DrawLayoutTransformedWithGradient(
			*cmd.LayoutPtr, cmd.X, cmd.Y,
			*cmd.LayoutTransform, cmd.TextGradient)
		return
	}
	r.textSys.DrawLayoutTransformed(
		*cmd.LayoutPtr, cmd.X, cmd.Y, *cmd.LayoutTransform)
}

func (r *renderer) drawTextPath(cmd *gui.RenderCmd) {
	layout, placements, err := gui.ComputeTextPathPlacements(
		cmd, r.textSys, &r.placements, guiStyleToGlyphConfig)
	if err != nil || len(placements) == 0 {
		return
	}
	r.textSys.DrawLayoutPlaced(layout, placements)
}
