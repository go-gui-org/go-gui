package gui

import "math"

const f32Tolerance = float32(0.01)

// f32Clamp returns x constrained between lo and hi. Callers must
// pass lo <= hi. NaN passes through unchanged: comparisons against
// NaN are all false, so no bound wins. Contain NaN at the trust
// boundary first (see clampSize, HSLA.Normalized).
func f32Clamp(x, lo, hi float32) float32 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// intClamp returns x constrained between lo and hi. Callers must
// pass lo <= hi.
func intClamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// f64Clamp returns v constrained between lo and hi. Callers must
// pass lo <= hi. NaN passes through unchanged, like f32Clamp.
func f64Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// f32AreClose tests if |a - b| <= f32Tolerance. The tolerance is
// absolute and pixel-oriented: layout compares coordinates in px,
// where a fixed subpixel epsilon is the right scale. It is not a
// general relative comparison, and NaN is never close to anything
// (NaN <= anything is false).
func f32AreClose(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= f32Tolerance
}

// f32Mod returns x mod y with the sign of the dividend, matching
// math.Mod. The old int-truncation form was identical for finite
// in-range values but undefined for large quotients (float-to-int
// overflow) and for y == 0 (int of Inf/NaN). The float64 trip also
// keeps full precision for large hues. NaN in, NaN out; y == 0 and
// infinite inputs return NaN, like math.Mod. Callers that need a
// non-negative wrap still add the modulus once when r < 0.
func f32Mod(x, y float32) float32 {
	return float32(math.Mod(float64(x), float64(y)))
}

// f32Abs returns absolute value of x. NaN passes through: NaN < 0
// is false, so NaN is returned unchanged.
func f32Abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// f32Min returns the smaller of a and b. NaN propagates from
// either position: a NaN size must poison later layout math so the
// invariant checker reports it, never silently resolve to the other
// operand depending on argument order.
func f32Min(a, b float32) float32 {
	if a != a || b != b {
		return float32(math.NaN())
	}
	if a < b {
		return a
	}
	return b
}

// f32Max returns the larger of a and b. NaN propagates from either
// position, for the same reason as f32Min.
func f32Max(a, b float32) float32 {
	if a != a || b != b {
		return float32(math.NaN())
	}
	if a > b {
		return a
	}
	return b
}

// f32Pi is math.Pi in float32 precision.
const f32Pi = float32(math.Pi)

// f32Sqrt returns the square root of x.
func f32Sqrt(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

// f32Sin returns the sine of x, in radians.
func f32Sin(x float32) float32 {
	return float32(math.Sin(float64(x)))
}

// f32Cos returns the cosine of x, in radians.
func f32Cos(x float32) float32 {
	return float32(math.Cos(float64(x)))
}

// f32Atan2 returns the arc tangent of y/x, using the signs of both to
// place the result in the correct quadrant.
func f32Atan2(y, x float32) float32 {
	return float32(math.Atan2(float64(y), float64(x)))
}

// f32IsFinite returns true if value is not NaN or Inf.
func f32IsFinite(f float32) bool {
	return math.Float32bits(f)&0x7F800000 != 0x7F800000
}

// ASCIILower returns the ASCII lowercase of c. Non-ASCII
// bytes pass through unchanged.
func ASCIILower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c | 0x20
	}
	return c
}
