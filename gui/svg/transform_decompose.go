package svg

import "math"

// Reconstruction-error bound; matrices with shear exceed it.
const decomposeTRSEpsilon = 1e-4

// decomposeTRS splits a 2x3 affine [a b c d e f] into TRS.
// Convention: (x',y') = (a*x + c*y + e, b*x + d*y + f).
// ok=false when the matrix has shear; caller must bake instead.
func decomposeTRS(m [6]float32) (tx, ty, sx, sy, rotDeg float32, ok bool) {
	tx, ty = m[4], m[5]
	if math.IsNaN(float64(tx)) || math.IsInf(float64(tx), 0) ||
		math.IsNaN(float64(ty)) || math.IsInf(float64(ty), 0) {
		return 0, 0, 0, 0, 0, false
	}
	a, b, c, d := m[0], m[1], m[2], m[3]

	sx = float32(math.Sqrt(float64(a*a + b*b)))
	if sx == 0 {
		// x-axis collapsed: only pure scale(0) on x with diagonal y
		// decomposes cleanly.
		sy = float32(math.Sqrt(float64(c*c + d*d)))
		ok = f32Abs(c) <= decomposeTRSEpsilon &&
			f32Abs(d-sy) <= decomposeTRSEpsilon
		return tx, ty, 0, sy, 0, ok
	}
	cosT := a / sx
	sinT := b / sx
	sy = cosT*d - sinT*c
	rotDeg = float32(math.Atan2(float64(sinT), float64(cosT)) * 180 / math.Pi)

	recC := -sy * sinT
	recD := sy * cosT
	ok = f32Abs(c-recC) <= decomposeTRSEpsilon &&
		f32Abs(d-recD) <= decomposeTRSEpsilon
	return tx, ty, sx, sy, rotDeg, ok
}
