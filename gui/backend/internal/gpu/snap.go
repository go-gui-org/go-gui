package gpu

import "math"

// SnapRect scales a logical rect by s and rounds each device edge to a
// whole pixel (#1334). Edges round, not the size, so boxes that share
// a logical edge share a device edge. A positive size that rounds to
// zero keeps one pixel, so a hairline does not vanish. NaN and Inf
// pass through for the caller's own guards.
func SnapRect(x, y, w, h, s float32) (dx, dy, dw, dh float32) {
	dx, dw = snapSpan(x, w, s)
	dy, dh = snapSpan(y, h, s)
	return dx, dy, dw, dh
}

func snapSpan(p, size, s float32) (dp, dsize float32) {
	dp = round32(p * s)
	dsize = round32((p+size)*s) - dp
	if size > 0 && dsize == 0 {
		dsize = 1
	}
	return dp, dsize
}

// SnapThickness scales a stroke thickness by s and rounds it to whole
// device pixels, at least one. Zero (a fill) and NaN pass through.
func SnapThickness(th, s float32) float32 {
	if !(th > 0) {
		return th
	}
	return max(1, round32(th*s))
}

// Strip is a device-space rect.
type Strip struct{ X, Y, W, H float32 }

// SnapBorderStrips snaps the box and the thickness once, then cuts the
// four border strips (top, bottom, left, right, the order of
// gui.GradientBorderRects) from those device values. Snapping each
// strip on its own lets opposite edges round to different widths at a
// fractional scale. A strip never extends past the snapped box. A
// non-positive box or thickness gives zero strips.
func SnapBorderStrips(x, y, w, h, th, s float32) [4]Strip {
	dx, dy, dw, dh := SnapRect(x, y, w, h, s)
	t := SnapThickness(th, s)
	if !(dw > 0 && dh > 0 && t > 0) {
		return [4]Strip{}
	}
	th2, tv := min(t, dh), min(t, dw)
	return [4]Strip{
		{dx, dy, dw, th2},
		{dx, dy + dh - th2, dw, th2},
		{dx, dy, tv, dh},
		{dx + dw - tv, dy, tv, dh},
	}
}

// round32 rounds halves up, not away from zero, so a box keeps its
// device size as it scrolls across the origin.
func round32(v float32) float32 {
	return float32(math.Floor(float64(v) + 0.5))
}
