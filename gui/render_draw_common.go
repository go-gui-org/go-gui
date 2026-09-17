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

// gradientBorderMaxStops bounds the stop list the sampler scans.
// Four samples over an arbitrary stop count is a per-frame scan an
// untrusted document should not get to name, so a longer list is
// truncated to this many stops.
//
// gradientBorderInlineStops sizes the on-stack normalize buffer.
// Every backend calls GradientBorderRects once per
// gradient-border command per frame, and the function takes no
// scratch buffer it could reuse, so normalizing must not reach the
// heap. A border gradient past this many stops cannot resolve into
// four sampled edge colors anyway; it is truncated with the rest.
const (
	gradientBorderMaxStops    = 8192
	gradientBorderInlineStops = 16
)

// gradientStopsNormalized reports whether stops already have the
// shape NormalizeGradientStops would produce — ascending
// positions, all inside [0,1] — so the sampler can read the
// caller's slice with no copy. A NaN position fails both
// comparisons and takes the normalizing path, where clampUnit
// folds it to 0.
func gradientStopsNormalized(stops []GradientStop) bool {
	if len(stops) > gradientBorderMaxStops {
		return false
	}
	prev := float32(0)
	for _, s := range stops {
		if !(s.Pos >= prev) || s.Pos > 1 {
			return false
		}
		prev = s.Pos
	}
	return true
}

// normalizeStopsInline clamps positions to [0,1] and sorts into
// buf, returning the sorted prefix. Stops past len(buf) are
// dropped: this is the misordered-input path, and the alternative
// is a heap allocation on every frame of every gradient border.
//
// The sort is a hand-written insertion sort, not slices.SortFunc:
// buf must not be passed to another function by pointer, or escape
// analysis moves it to the heap and the allocation is back.
// Insertion sort is also the faster choice at this length.
func normalizeStopsInline(
	stops []GradientStop, buf *[gradientBorderInlineStops]GradientStop,
) []GradientStop {
	n := 0
	for _, s := range stops {
		if n == len(buf) {
			break
		}
		buf[n] = GradientStop{Color: s.Color, Pos: clampUnit(s.Pos)}
		n++
	}
	for i := 1; i < n; i++ {
		cur := buf[i]
		j := i - 1
		for j >= 0 && buf[j].Pos > cur.Pos {
			buf[j+1] = buf[j]
			j--
		}
		buf[j+1] = cur
	}
	return buf[:n]
}

// GradientBorderRects computes the 4 edge rects with sampled colors.
// The caller applies DPI scaling to the returned rects. A nil command
// or gradient yields zeros. Stops are clamped to [0,1] and sorted
// before sampling, like every gradient fill path: a caller-built
// GradientDef carries no ordering guarantee.
func GradientBorderRects(r *RenderCmd) [4]GradientBorderRect {
	if r == nil || r.Gradient == nil {
		return [4]GradientBorderRect{}
	}
	th := r.Thickness
	stops := r.Gradient.Stops
	if len(stops) == 0 {
		return [4]GradientBorderRect{}
	}
	var inline [gradientBorderInlineStops]GradientStop
	if !gradientStopsNormalized(stops) {
		stops = normalizeStopsInline(stops, &inline)
		if len(stops) == 0 {
			return [4]GradientBorderRect{}
		}
	}
	positions := [4]float32{0.0, 0.25, 0.5, 0.75}
	colors := [4]Color{
		SampleGradientStopColor(stops, positions[0]),
		SampleGradientStopColor(stops, positions[1]),
		SampleGradientStopColor(stops, positions[2]),
		SampleGradientStopColor(stops, positions[3]),
	}
	return [4]GradientBorderRect{
		{r.X, r.Y, r.W, th, colors[0]},
		{r.X, (r.Y + r.H) - th, r.W, th, colors[1]},
		{r.X, r.Y, th, r.H, colors[2]},
		{(r.X + r.W) - th, r.Y, th, r.H, colors[3]},
	}
}
