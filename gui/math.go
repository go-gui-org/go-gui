package gui

import "math"

const f32Tolerance = float32(0.01)

// f32Clamp returns x constrained between lo and hi.
func f32Clamp(x, lo, hi float32) float32 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// intClamp returns x constrained between lo and hi.
func intClamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// f64Clamp returns v constrained between lo and hi.
func f64Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// f32AreClose tests if |a - b| <= f32Tolerance.
func f32AreClose(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= f32Tolerance
}

// f32Mod returns x mod y without float64 round-trip.
func f32Mod(x, y float32) float32 {
	return x - y*float32(int(x/y))
}

// f32Abs returns absolute value of x.
func f32Abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// f32Min returns the smaller of a and b.
func f32Min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// f32Max returns the larger of a and b.
func f32Max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
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
