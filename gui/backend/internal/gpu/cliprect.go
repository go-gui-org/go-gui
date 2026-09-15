package gpu

import "math"

// clipCoordLimit bounds a device-pixel clip coordinate. A render
// command only has to be finite to pass validation, so a crafted or
// runaway coordinate can scale past the int32 range, where a Go
// float-to-int conversion is implementation-defined. Clamping well
// inside the range keeps x0+w and y0+h from wrapping too.
const clipCoordLimit = float64(1 << 24)

// ClipRect converts a logical clip box to the device-pixel rect that
// fully contains it. The near edge floors and the far edge ceils, so
// a fractional DPI scale never shaves a pixel off the right or bottom
// edge of clipped content — the rule gui/backend/soft's deviceRect
// already follows. Scaling w and h on their own instead (the old
// behaviour) puts the far edge at floor(x)+floor(w), up to a full
// device pixel short of floor(x+w).
//
// Returns the origin and the extent, never a negative extent: a GL
// scissor with one is GL_INVALID_VALUE. A non-finite or negative
// scale, which the backends reject before it reaches here, yields an
// empty rect rather than a reversed one.
func ClipRect(x, y, w, h, scale float32) (cx, cy, cw, ch int32) {
	s := float64(scale)
	x0 := clampClipCoord(math.Floor(float64(x) * s))
	y0 := clampClipCoord(math.Floor(float64(y) * s))
	// The far edges sum in float64: adding two float32s first can
	// round the sum down, which would put the edge short again.
	x1 := clampClipCoord(math.Ceil((float64(x) + float64(w)) * s))
	y1 := clampClipCoord(math.Ceil((float64(y) + float64(h)) * s))
	cw, ch = max(x1-x0, 0), max(y1-y0, 0)
	// A degenerate box stays degenerate. Flooring and ceiling a
	// zero-width box straddling a pixel boundary would widen it to
	// one pixel, and both backends read a zero extent as
	// clip-everything — the empty clip must survive the rounding.
	if w <= 0 {
		cw = 0
	}
	if h <= 0 {
		ch = 0
	}
	return x0, y0, cw, ch
}

// clampClipCoord pins a scaled coordinate to the safe int32 window.
// NaN clamps to zero, which reads as an empty clip rather than an
// arbitrary one.
func clampClipCoord(v float64) int32 {
	switch {
	case math.IsNaN(v):
		return 0
	case v > clipCoordLimit:
		return int32(clipCoordLimit)
	case v < -clipCoordLimit:
		return int32(-clipCoordLimit)
	}
	return int32(v)
}
