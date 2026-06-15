package gui

import (
	"math"

	glyph "github.com/go-gui-org/go-glyph"
)

// cachedSvgPaths converts TessellatedPath slices to CachedSvgPath.
func cachedSvgPaths(paths []TessellatedPath) []CachedSvgPath {
	out := make([]CachedSvgPath, len(paths))
	for i := range paths {
		p := &paths[i]
		var vcols []Color
		if len(p.VertexColors) != 0 {
			vcols = make([]Color, len(p.VertexColors))
			for j := range p.VertexColors {
				vcols[j] = svgToColor(p.VertexColors[j])
			}
		}
		out[i] = CachedSvgPath{
			Triangles:    p.Triangles,
			Color:        svgToColor(p.Color),
			VertexColors: vcols,
			IsClipMask:   p.IsClipMask,
			ClipGroup:    p.ClipGroup,
			PathID:       p.PathID,
			Animated:     p.Animated,
			IsStroke:     p.IsStroke,
			Primitive:    p.Primitive,
			BaseTransX:   p.BaseTransX,
			BaseTransY:   p.BaseTransY,
			BaseScaleX:   p.BaseScaleX,
			BaseScaleY:   p.BaseScaleY,
			BaseRotAngle: p.BaseRotAngle,
			BaseRotCX:    p.BaseRotCX,
			BaseRotCY:    p.BaseRotCY,
			HasBaseXform: p.HasBaseXform,
		}
	}
	return out
}

// buildSvgTextStyle builds a TextStyle from SVG text properties.
func buildSvgTextStyle(
	fontFamily string, fontWeight int, isBold, isItalic bool,
	fontSize, letterSpacing, strokeWidth float32,
	strokeColor, color SvgColor, opacity, scale float32,
) TextStyle {
	fontName := fontFamily
	if wn := pangoWeightName(fontWeight); wn != "" {
		fontName += " " + wn
	} else if isBold {
		fontName += " Bold"
	}
	typeface := glyph.TypefaceRegular
	if isItalic {
		typeface = glyph.TypefaceItalic
	}
	ts := TextStyle{
		Family:        fontName,
		Size:          fontSize * scale,
		LetterSpacing: letterSpacing * scale,
		Typeface:      typeface,
		StrokeWidth:   strokeWidth * scale,
		StrokeColor:   svgToColor(strokeColor),
	}
	if opacity < 1.0 {
		ts.Color = Color{color.R, color.G, color.B,
			uint8(float32(color.A)*opacity + 0.5), true}
	} else {
		ts.Color = svgToColor(color)
	}
	return ts
}

// cachedSvgTextDraws converts SvgText elements to CachedSvgTextDraw.
func cachedSvgTextDraws(texts []SvgText, scale float32,
	gradients map[string]SvgGradientDef, w *Window) []CachedSvgTextDraw {
	draws := make([]CachedSvgTextDraw, 0, len(texts))
	for _, t := range texts {
		if len(t.Text) == 0 {
			continue
		}
		ts := buildSvgTextStyle(t.FontFamily, t.FontWeight,
			t.IsBold, t.IsItalic, t.FontSize, t.LetterSpacing,
			t.StrokeWidth, t.StrokeColor, t.Color, t.Opacity, scale)
		ts.Underline = t.Underline
		ts.Strikethrough = t.Strikethrough

		// Stroke-only text: fill=none + stroke set → transparent fill.
		if ts.StrokeWidth > 0 && ts.Color.A == 0 {
			ts.Color = Color{0, 0, 0, 0, true}
		}

		// Build gradient config from SVG gradient def.
		var grad *glyph.GradientConfig
		if t.FillGradientID != "" && gradients != nil {
			if gdef, ok := gradients[t.FillGradientID]; ok {
				grad = svgGradientToGlyph(gdef)
			}
		}

		// Measure text width for anchor adjustment.
		var tw float32
		var fh float32
		if w.textMeasurer != nil {
			tw = w.textMeasurer.TextWidth(t.Text, ts)
			fh = w.textMeasurer.FontHeight(ts)
		} else {
			fh = t.FontSize * scale
		}
		ascent := fh * 0.8
		x := t.X * scale
		y := t.Y*scale - ascent
		switch t.Anchor {
		case SvgTextAnchorMiddle:
			x -= tw / 2
		case SvgTextAnchorEnd:
			x -= tw
		}
		draws = append(draws, CachedSvgTextDraw{
			Text:      t.Text,
			TextStyle: ts,
			X:         x,
			Y:         y,
			TextWidth: tw,
			Gradient:  grad,
		})
	}
	return draws
}

func cachedSvgTextPathDraws(textPaths []SvgTextPath,
	defsPathData map[string]cachedDefsPathData,
	scale float32,
) []CachedSvgTextPathDraw {
	if len(textPaths) == 0 {
		return nil
	}
	out := make([]CachedSvgTextPathDraw, 0, len(textPaths))
	for i := range textPaths {
		tp := textPaths[i]
		if tp.Text == "" {
			continue
		}
		cached, ok := defsPathData[tp.PathID]
		if !ok || len(cached.polyline) < 4 || cached.totalLen <= 0 {
			continue
		}
		ts := buildSvgTextStyle(tp.FontFamily, tp.FontWeight,
			tp.IsBold, tp.IsItalic, tp.FontSize, tp.LetterSpacing,
			tp.StrokeWidth, tp.StrokeColor, tp.Color, tp.Opacity, scale)

		offset := tp.StartOffset * scale
		if tp.IsPercent {
			offset = (tp.StartOffset / 100) * cached.totalLen
		}
		out = append(out, CachedSvgTextPathDraw{
			Text:      tp.Text,
			TextStyle: ts,
			Path: TextPathData{
				Polyline: cached.polyline,
				Table:    cached.table,
				TotalLen: cached.totalLen,
				Offset:   offset,
				Anchor:   tp.Anchor,
				Method:   tp.Method,
			},
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pangoWeightName maps CSS font-weight (100-900) to a Pango
// weight descriptor. Returns "" for regular (400) or unset (0).
func pangoWeightName(w int) string {
	switch w {
	case 100:
		return "Thin"
	case 200:
		return "Ultra-Light"
	case 300:
		return "Light"
	case 500:
		return "Medium"
	case 600:
		return "Semi-Bold"
	case 700:
		return "Bold"
	case 800:
		return "Ultra-Bold"
	case 900:
		return "Heavy"
	default:
		return ""
	}
}

// svgGradientToGlyph converts an SvgGradientDef to a glyph
// GradientConfig.
func svgGradientToGlyph(g SvgGradientDef) *glyph.GradientConfig {
	if len(g.Stops) == 0 {
		return nil
	}
	stops := make([]glyph.GradientStop, len(g.Stops))
	for i, s := range g.Stops {
		stops[i] = glyph.GradientStop{
			Color:    glyph.Color{R: s.Color.R, G: s.Color.G, B: s.Color.B, A: s.Color.A},
			Position: s.Offset,
		}
	}
	dx := g.X2 - g.X1
	dy := g.Y2 - g.Y1
	// Guard NaN/Inf — all subsequent comparisons silently go the wrong way.
	dx64 := float64(dx)
	dy64 := float64(dy)
	if math.IsNaN(dx64) || math.IsInf(dx64, 0) {
		dx = 0
	}
	if math.IsNaN(dy64) || math.IsInf(dy64, 0) {
		dy = 0
	}
	dx2 := dx * dx
	dy2 := dy * dy
	// Classify gradient direction: horizontal, vertical, or diagonal.
	// Diagonal when both axis components are significant (ratio ≥ 0.2,
	// i.e. squared ratio ≥ 0.04).
	var dir glyph.GradientDirection
	switch {
	case dx2 == 0 && dy2 == 0:
		dir = glyph.GradientHorizontal
	case dx2 > dy2:
		if dy2 > 0 && float64(dy2)/float64(dx2) >= 0.04 {
			dir = glyph.GradientDiagonal
		} else {
			dir = glyph.GradientHorizontal
		}
	default:
		if dx2 > 0 && float64(dx2)/float64(dy2) >= 0.04 {
			dir = glyph.GradientDiagonal
		} else {
			dir = glyph.GradientVertical
		}
	}
	return &glyph.GradientConfig{
		Stops:     stops,
		Direction: dir,
	}
}

func buildDefsPathDataCache(
	textPaths []SvgTextPath,
	filtered []SvgParsedFilteredGroup,
	defsPaths map[string]string,
	scale float32,
) map[string]cachedDefsPathData {
	if (len(textPaths) == 0 && len(filtered) == 0) || len(defsPaths) == 0 {
		return nil
	}
	pathIDs := make(map[string]struct{}, len(textPaths))
	for i := range textPaths {
		id := textPaths[i].PathID
		if id != "" {
			pathIDs[id] = struct{}{}
		}
	}
	for i := range filtered {
		for j := range filtered[i].TextPaths {
			id := filtered[i].TextPaths[j].PathID
			if id != "" {
				pathIDs[id] = struct{}{}
			}
		}
	}
	if len(pathIDs) == 0 {
		return nil
	}
	cached := make(map[string]cachedDefsPathData, len(pathIDs))
	for pathID := range pathIDs {
		d, ok := defsPaths[pathID]
		if !ok {
			continue
		}
		polyline := flattenDefsPath(d, scale)
		if len(polyline) < 4 {
			continue
		}
		table, totalLen := buildArcLengthTable(polyline)
		if totalLen <= 0 {
			continue
		}
		cached[pathID] = cachedDefsPathData{
			polyline: polyline,
			table:    table,
			totalLen: totalLen,
		}
	}
	if len(cached) == 0 {
		return nil
	}
	return cached
}

func buildSvgCacheLookupKey(
	srcHash uint64, width, height float32, opts SvgParseOpts,
) svgCacheKey {
	return svgCacheKey{
		srcHash:       srcHash,
		w10:           int32(width * 10),
		h10:           int32(height * 10),
		reducedMotion: opts.PrefersReducedMotion,
		flatness10000: quantizeFlatness(opts.FlatnessTolerance),
		hoveredID:     clampSvgCacheID(opts.HoveredElementID),
		focusedID:     clampSvgCacheID(opts.FocusedElementID),
	}
}

// quantizeFlatness maps FlatnessTolerance into the int32 cache key
// slot. NaN/Inf and out-of-range values collapse to 0, which matches
// the "no override" key used by callers that don't set the field.
// Without this guard, `int32(NaN*10000)` is implementation-defined
// and `int32(huge*10000)` overflows.
func quantizeFlatness(t float32) int32 {
	t64 := float64(t)
	if math.IsNaN(t64) || math.IsInf(t64, 0) || t <= 0 {
		return 0
	}
	scaled := t64 * 10000
	if scaled > float64(math.MaxInt32) {
		return math.MaxInt32
	}
	return int32(scaled)
}

func clampSvgCacheID(s string) string {
	if len(s) > maxSvgCacheElementIDLen {
		return s[:maxSvgCacheElementIDLen]
	}
	return s
}

// buildBaseByPath collects the per-path decomposed base transforms,
// by PathID, for paths that any animation targets. Seeding the
// per-frame svgAnimState with these lets SMIL additive / replace
// compose over the author's base (see CachedSvg.BaseByPath).
func buildBaseByPath(
	paths []CachedSvgPath,
	filteredGroups []CachedFilteredGroup,
	anims []SvgAnimation,
) map[uint32]svgBaseXform {
	if len(anims) == 0 {
		return nil
	}
	targeted := make(map[uint32]struct{}, len(anims))
	for i := range anims {
		for _, pid := range anims[i].TargetPathIDs {
			targeted[pid] = struct{}{}
		}
	}
	if len(targeted) == 0 {
		return nil
	}
	out := make(map[uint32]svgBaseXform)
	collect := func(ps []CachedSvgPath) {
		for i := range ps {
			p := &ps[i]
			if !p.HasBaseXform || p.PathID == 0 {
				continue
			}
			if _, ok := targeted[p.PathID]; !ok {
				continue
			}
			if _, already := out[p.PathID]; already {
				continue
			}
			out[p.PathID] = svgBaseXform{
				TransX:   p.BaseTransX,
				TransY:   p.BaseTransY,
				ScaleX:   p.BaseScaleX,
				ScaleY:   p.BaseScaleY,
				RotAngle: p.BaseRotAngle,
				RotCX:    p.BaseRotCX,
				RotCY:    p.BaseRotCY,
			}
		}
	}
	collect(paths)
	for i := range filteredGroups {
		collect(filteredGroups[i].RenderPaths)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// computeTriangleBBox computes bounding box from tessellated paths.
// Returns [x, y, width, height].
func computeTriangleBBox(tpaths []TessellatedPath) [4]float32 {
	minX := float32(1e30)
	minY := float32(1e30)
	maxX := float32(-1e30)
	maxY := float32(-1e30)
	hasData := false

	for _, tp := range tpaths {
		for i := 0; i+1 < len(tp.Triangles); i += 2 {
			x := tp.Triangles[i]
			y := tp.Triangles[i+1]
			minX = min(minX, x)
			maxX = max(maxX, x)
			minY = min(minY, y)
			maxY = max(maxY, y)
			hasData = true
		}
	}

	if !hasData {
		return [4]float32{0, 0, 0, 0}
	}
	return [4]float32{minX, minY, maxX - minX, maxY - minY}
}
