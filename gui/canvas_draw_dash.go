package gui

// canvas_draw_dash.go — the dashed stroke primitives.
//
// Both walk a segment in pattern periods and hand each dash to Line,
// so the dash geometry is the only thing here; the stroke expansion
// itself lives with Polyline.

import "math"

// DashedLine draws a dashed line segment. dashLen and gapLen
// control the pattern. Zero or negative values fall back to
// solid.
func (dc *DrawContext) DashedLine(
	x0, y0, x1, y1 float32,
	color Color, width, dashLen, gapLen float32,
) {
	if dashLen <= 0 || gapLen <= 0 {
		dc.Line(x0, y0, x1, y1, color, width)
		return
	}
	if width <= 0 || !f32IsFinite(width) || hasNaNInf(dashLen, gapLen) {
		return
	}
	if dc.recorder != nil {
		dc.rec().DashedLine(x0, y0, x1, y1, color, width, dashLen, gapLen)
		return
	}
	if hasNaNInf(x0, y0, x1, y1) {
		return
	}
	dx := x1 - x0
	dy := y1 - y0
	totalLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if totalLen < 1e-6 {
		return
	}
	ux := dx / totalLen
	uy := dy / totalLen
	patternLen := dashLen + gapLen
	drawn := float32(0)
	for n := dashCount(totalLen, patternLen); n > 0; n-- {
		end := drawn + dashLen
		if end > totalLen {
			end = totalLen
		}
		dc.Line(
			x0+ux*drawn, y0+uy*drawn,
			x0+ux*end, y0+uy*end,
			color, width,
		)
		drawn += patternLen
	}
}

// maxDashes bounds the dashes one segment may emit. A pattern short
// against the segment it walks — a 1e-6 gap over a 1e6 line, or a
// finite but astronomical endpoint — would otherwise run the walk for
// longer than the frame it sits in, and every dash past a pixel or two
// of pattern is invisible anyway.
const maxDashes = 1 << 16

// dashCount is how many pattern periods fit in a segment, capped.
// Computed in float64: totalLen/patternLen is past int64 for the
// extreme inputs this cap exists for, and an out-of-range float-to-int
// conversion is undefined in Go.
func dashCount(totalLen, patternLen float32) int {
	if !(patternLen > 0) || !(totalLen > 0) {
		return 0
	}
	n := math.Ceil(float64(totalLen) / float64(patternLen))
	return int(min(n, maxDashes))
}

// DashedPolyline draws a polyline with a dash pattern applied
// continuously across all segments.
func (dc *DrawContext) DashedPolyline(
	points []float32,
	color Color, width, dashLen, gapLen float32,
) {
	if len(points) < 4 {
		return
	}
	if dashLen <= 0 || gapLen <= 0 {
		dc.Polyline(points, color, width)
		return
	}
	if width <= 0 || !f32IsFinite(width) || hasNaNInf(dashLen, gapLen) {
		return
	}
	if dc.recorder != nil {
		dc.rec().DashedPolyline(points, color, width, dashLen, gapLen)
		return
	}
	patternLen := dashLen + gapLen
	offset := float32(0) // position within pattern
	for i := 0; i+3 < len(points); i += 2 {
		x0, y0 := points[i], points[i+1]
		x1, y1 := points[i+2], points[i+3]
		if hasNaNInf(x0, y0, x1, y1) {
			continue
		}
		dx := x1 - x0
		dy := y1 - y0
		segLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if segLen < 1e-6 {
			continue
		}
		ux := dx / segLen
		uy := dy / segLen
		pos := float32(0)
		// Two dash steps per period at worst — a dash then a gap — so
		// the period count bounds the walk with room to spare.
		steps := 2 * dashCount(segLen, patternLen)
		for ; pos < segLen && steps > 0; steps-- {
			inPattern := float32(math.Mod(float64(offset+pos),
				float64(patternLen)))
			if inPattern < dashLen {
				// In dash portion.
				remain := dashLen - inPattern
				end := pos + remain
				if end > segLen {
					end = segLen
				}
				dc.Line(
					x0+ux*pos, y0+uy*pos,
					x0+ux*end, y0+uy*end,
					color, width,
				)
				pos += remain
			} else {
				// In gap portion.
				pos += patternLen - inPattern
			}
		}
		offset += segLen
	}
}
