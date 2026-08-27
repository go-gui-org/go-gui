package gui

import (
	"math"

	"github.com/go-gui-org/go-glyph"
)

// offscreenSentinel marks unpositioned glyph placements.
const offscreenSentinel = -9999

// maxTextPathGlyphs caps glyph allocation to prevent DoS from
// pathological text layouts. Far beyond any realistic text path.
const maxTextPathGlyphs = 10000

// ComputeTextPathPlacements computes glyph placements along an SVG
// text path. This is the pure-computation core shared by all
// backends. The caller handles pipeline setup, DrawLayoutPlaced,
// and pipeline teardown.
func ComputeTextPathPlacements(
	r *RenderCmd,
	textSys *glyph.TextSystem,
	placementsBuf *[]glyph.GlyphPlacement,
	styleToCfg func(TextStyle) glyph.TextConfig,
) (layout glyph.Layout, placements []glyph.GlyphPlacement, err error) {
	if textSys == nil || r.textPath == nil || r.TextStylePtr == nil {
		return glyph.Layout{}, nil, nil
	}
	tp := r.textPath
	cfg := styleToCfg(*r.TextStylePtr)
	layout, err = textSys.LayoutTextCached(r.Text, cfg)
	if err != nil {
		return glyph.Layout{}, nil, err
	}
	positions := layout.GlyphPositions()
	if len(positions) == 0 {
		return layout, nil, nil
	}

	var totalAdvance float32
	for _, p := range positions {
		totalAdvance += p.Advance
	}

	offset := tp.Offset
	switch tp.Anchor {
	case SvgTextAnchorMiddle:
		offset -= totalAdvance / 2
	case SvgTextAnchorEnd:
		offset -= totalAdvance
	}

	advScale := float32(1)
	if tp.method == svgTextPathMethodStretch && totalAdvance > 0 {
		remaining := tp.totalLen - offset
		if remaining > 0 {
			advScale = remaining / totalAdvance
		}
	}

	n := min(len(layout.Glyphs), maxTextPathGlyphs)
	var buf []glyph.GlyphPlacement
	if placementsBuf != nil {
		buf = *placementsBuf
	}
	if cap(buf) < n {
		buf = make([]glyph.GlyphPlacement, n)
		if placementsBuf != nil {
			*placementsBuf = buf
		}
	}
	placements = buf[:n]
	for i := range placements {
		placements[i] = glyph.GlyphPlacement{
			X: offscreenSentinel, Y: offscreenSentinel,
		}
	}

	cumAdv := float32(0)
	for _, p := range positions {
		if p.Index >= n {
			continue
		}
		advance := p.Advance * advScale
		centerDist := offset + cumAdv + advance/2
		px, py, angle := samplePathAt(
			tp.Polyline, tp.Table, centerDist)

		halfAdv := advance / 2
		cosA := float32(math.Cos(float64(angle)))
		sinA := float32(math.Sin(float64(angle)))
		gx := px + r.X - halfAdv*cosA
		gy := py + r.Y - halfAdv*sinA

		placements[p.Index] = glyph.GlyphPlacement{
			X: gx, Y: gy, Angle: angle,
		}
		cumAdv += advance
	}

	return layout, placements, nil
}

// DrawTextTransformed draws a RenderText with an affine
// transform via the glyph layout cache. Returns true when it
// handled the command (including on error / empty text), so the
// caller should not also take the DrawText fast path. The fast
// path is left to the caller so each backend keeps its own
// glyph-pipeline setup.
func DrawTextTransformed(
	r *RenderCmd,
	textSys *glyph.TextSystem,
	styleToCfg func(TextStyle) glyph.TextConfig,
	drawFn func(glyph.Layout, *glyph.GradientConfig),
) bool {
	if r == nil || textSys == nil || r.LayoutTransform == nil {
		return false
	}
	// Reject NaN/Inf transforms before touching glyph
	// cache — render_validate also drops these, and the
	// glyph renderer is not required to handle them.
	t := *r.LayoutTransform
	if !f32IsFinite(t.XX) || !f32IsFinite(t.XY) ||
		!f32IsFinite(t.YX) || !f32IsFinite(t.YY) ||
		!f32IsFinite(t.X0) || !f32IsFinite(t.Y0) {
		return true
	}
	if len(r.Text) == 0 {
		return true
	}
	var cfg glyph.TextConfig
	if r.TextStylePtr != nil {
		cfg = styleToCfg(*r.TextStylePtr)
		cfg.Gradient = r.TextGradient
	} else {
		// Fallback for plain RenderText with no style ptr
		// (mirrors glyphconv.GuiTextConfigFromRender).
		cfg = glyph.TextConfig{
			Style: glyph.TextStyle{
				FontName: r.FontName,
				Size:     r.FontSize,
				Color: glyph.Color{
					R: r.Color.R, G: r.Color.G,
					B: r.Color.B, A: r.Color.A,
				},
			},
			Block: glyph.DefaultBlockStyle(),
		}
	}
	if r.W > 0 {
		cfg.Block.Wrap = glyph.WrapWord
		cfg.Block.Width = r.W
	}
	layout, err := textSys.LayoutTextCached(r.Text, cfg)
	if err != nil {
		return true
	}
	drawFn(layout, r.TextGradient)
	return true
}

// GradientBorderRect is one edge rect with its sampled color for a
// gradient border. Shared across all backends.
// exportaudit:keep — reachable from an exported signature
type GradientBorderRect struct {
	X, Y, W, H float32
	Color      Color
}

// GradientBorderRects computes the 4 edge rects with sampled colors.
// The caller applies DPI scaling to the returned rects.
func GradientBorderRects(r *RenderCmd) [4]GradientBorderRect {
	th := r.Thickness
	if len(r.Gradient.Stops) == 0 {
		return [4]GradientBorderRect{}
	}
	positions := [4]float32{0.0, 0.25, 0.5, 0.75}
	colors := [4]Color{
		SampleGradientStopColor(r.Gradient.Stops, positions[0]),
		SampleGradientStopColor(r.Gradient.Stops, positions[1]),
		SampleGradientStopColor(r.Gradient.Stops, positions[2]),
		SampleGradientStopColor(r.Gradient.Stops, positions[3]),
	}
	return [4]GradientBorderRect{
		{r.X, r.Y, r.W, th, colors[0]},
		{r.X, (r.Y + r.H) - th, r.W, th, colors[1]},
		{r.X, r.Y, th, r.H, colors[2]},
		{(r.X + r.W) - th, r.Y, th, r.H, colors[3]},
	}
}
