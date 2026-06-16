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
	placementsBuf []glyph.GlyphPlacement,
	styleToCfg func(TextStyle) glyph.TextConfig,
) (layout glyph.Layout, placements []glyph.GlyphPlacement, err error) {
	if textSys == nil || r.TextPath == nil || r.TextStylePtr == nil {
		return glyph.Layout{}, nil, nil
	}
	tp := r.TextPath
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
	if tp.Method == SvgTextPathMethodStretch && totalAdvance > 0 {
		remaining := tp.TotalLen - offset
		if remaining > 0 {
			advScale = remaining / totalAdvance
		}
	}

	n := min(len(layout.Glyphs), maxTextPathGlyphs)
	if cap(placementsBuf) < n {
		placementsBuf = make([]glyph.GlyphPlacement, n)
	}
	placements = placementsBuf[:n]
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
		px, py, angle := SamplePathAt(
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

// GradientBorderRect is one edge rect with its sampled color for a
// gradient border. Shared across all backends.
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
