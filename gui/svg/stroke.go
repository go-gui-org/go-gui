package svg

import (
	"github.com/go-gui-org/go-gui/gui"
)

// stroke.go — SVG stroke tessellation.
//
// The geometry lives in gui.AppendStrokeTris, shared with
// DrawContext's Styled strokes. What stays here are the svg-domain
// pieces: the SvgStrokeCap/SvgStrokeJoin types at the boundary, the
// per-polyline loop, and the capacity estimate.

// tessellateStroke converts polylines to stroke triangles.
func tessellateStroke(polylines [][]float32, width float32, lineCap gui.SvgStrokeCap, join gui.SvgStrokeJoin) []float32 {
	result := make([]float32, 0, estimateStrokeResultCap(polylines, lineCap, join))
	halfW := width / 2
	capStyle := strokeCapFromSvg(lineCap)
	joinStyle := strokeJoinFromSvg(join)

	var scratch []float32
	for _, poly := range polylines {
		// AppendStrokeTris skips short and odd-length polylines itself.
		scratch = gui.AppendStrokeTris(&result, poly, halfW,
			capStyle, joinStyle, scratch)
	}
	return result
}

// strokeCapFromSvg maps an SVG cap onto the shared style. Anything
// outside the three caps — including the inherit sentinel, which
// style resolution removes before tessellation — draws as butt.
func strokeCapFromSvg(lineCap gui.SvgStrokeCap) gui.StrokeCap {
	switch lineCap {
	case gui.SvgRoundCap:
		return gui.StrokeRoundCap
	case gui.SvgSquareCap:
		return gui.StrokeSquareCap
	default:
		return gui.StrokeButtCap
	}
}

// strokeJoinFromSvg maps an SVG join onto the shared style, with the
// same defensive default as the cap mapping.
func strokeJoinFromSvg(join gui.SvgStrokeJoin) gui.StrokeJoin {
	switch join {
	case gui.SvgRoundJoin:
		return gui.StrokeRoundJoin
	case gui.SvgBevelJoin:
		return gui.StrokeBevelJoin
	default:
		return gui.StrokeMiterJoin
	}
}

func estimateStrokeResultCap(polylines [][]float32, lineCap gui.SvgStrokeCap, join gui.SvgStrokeJoin) int {
	total := 0
	for _, poly := range polylines {
		if len(poly) < 4 || len(poly)%2 != 0 {
			continue
		}
		n := len(poly) / 2
		if n < 2 {
			continue
		}
		pointCount := n
		dxClose := poly[0] - poly[(n-1)*2]
		dyClose := poly[1] - poly[(n-1)*2+1]
		isClosed := n > 2 && f32Abs(dxClose) < closedPathEpsilon &&
			f32Abs(dyClose) < closedPathEpsilon
		if isClosed {
			pointCount--
		}
		if pointCount < 2 {
			continue
		}
		segCount := pointCount - 1
		joinCount := pointCount - 2
		if isClosed {
			segCount = pointCount
			joinCount = pointCount
		}
		total += segCount * 12
		switch join {
		case gui.SvgMiterJoin:
			// Two triangles per miter; a bevel fallback uses less.
			total += joinCount * 12
		case gui.SvgBevelJoin:
			total += joinCount * 6
		case gui.SvgRoundJoin:
			total += joinCount * strokeRoundCapSegs * 6
		}
		if !isClosed {
			switch lineCap {
			case gui.SvgSquareCap:
				total += 2 * 12
			case gui.SvgRoundCap:
				total += 2 * strokeRoundCapSegs * 6
			}
		}
	}
	return total
}
