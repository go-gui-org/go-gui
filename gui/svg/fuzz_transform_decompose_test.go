package svg

import (
	"math"
	"testing"
)

// FuzzDecomposeTRS feeds arbitrary 2×3 affine matrices through
// decomposeTRS and asserts:
//   - no panic
//   - ok=true → every returned scalar is finite AND recomposing
//     approximates the input within a slack epsilon
//   - ok=false is permitted (shear, non-decomposable matrix)
//
// Recomposition slack is 1e-3 relative; the decompose epsilon itself
// is 1e-4 absolute, so randomized inputs benefit from a looser
// compare.
func FuzzDecomposeTRS(f *testing.F) {
	f.Add(float32(1), float32(0), float32(0), float32(1),
		float32(0), float32(0)) // identity
	f.Add(float32(2), float32(0), float32(0), float32(3),
		float32(5), float32(7)) // pure scale+trans
	f.Add(float32(0), float32(1), float32(-1), float32(0),
		float32(0), float32(0)) // 90deg rotation
	f.Add(float32(1), float32(1), float32(0), float32(1),
		float32(0), float32(0)) // shear
	f.Add(float32(math.NaN()), float32(0), float32(0), float32(1),
		float32(0), float32(0))
	f.Add(float32(math.Inf(1)), float32(0), float32(0), float32(1),
		float32(0), float32(0))
	// Large anisotropic scale + tiny rotation: float32 angle quantization
	// perturbs the off-diagonal by ~1.5e-3, exact relative to the column
	// norm. Regression for the conditioning-aware compare below.
	f.Add(float32(-16002), float32(1), float32(0), float32(0.0125),
		float32(0), float32(0))

	f.Fuzz(func(t *testing.T, a, b, c, d, e, fv float32) {
		m := [6]float32{a, b, c, d, e, fv}
		tx, ty, sx, sy, rot, ok := decomposeTRS(m)
		if !ok {
			return
		}
		for i, v := range []float32{tx, ty, sx, sy, rot} {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("ok=true but output[%d]=%v non-finite "+
					"(input=%v)", i, v, m)
			}
		}
		// Recompose: m' = T · R · S.
		rad := float64(rot) * math.Pi / 180
		cosT, sinT := float32(math.Cos(rad)), float32(math.Sin(rad))
		ra := cosT * sx
		rb := sinT * sx
		rc := -sinT * sy
		rd := cosT * sy
		re := tx
		rf := ty
		const slack = 1e-3
		// Compare each column pair against the column's Euclidean
		// magnitude, not a per-component floor of 1. The rotation is
		// recomposed from a float32-quantized degree angle; for a large
		// anisotropic scale that quantization perturbs the off-diagonal
		// term by an amount that scales with the column norm (e.g.
		// scale(16002) with a ~0.004° rotation moves b by 1.5e-3 yet is
		// exact to 9e-8 relative to the column). Judging the off-diagonal
		// in isolation would flag a numerically faithful reconstruction.
		colClose := func(wx, wy, gx, gy float32) bool {
			for _, w := range []float32{wx, wy} {
				if math.IsNaN(float64(w)) || math.IsInf(float64(w), 0) {
					return false
				}
			}
			mag := math.Hypot(float64(wx), float64(wy))
			if mag < 1 {
				mag = 1
			}
			return math.Abs(float64(gx-wx)) <= slack*mag &&
				math.Abs(float64(gy-wy)) <= slack*mag
		}
		if !colClose(a, b, ra, rb) || !colClose(c, d, rc, rd) ||
			!colClose(e, fv, re, rf) {
			t.Fatalf("recompose mismatch input=%v got=[%v %v %v %v %v %v]",
				m, ra, rb, rc, rd, re, rf)
		}
	})
}
