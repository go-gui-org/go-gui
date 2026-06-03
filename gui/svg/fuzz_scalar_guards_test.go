package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// FuzzNonNegF32 asserts nonNegF32 never returns NaN or a negative
// finite value. +Inf passes through by design (not currently clamped).
func FuzzNonNegF32(f *testing.F) {
	f.Add(float32(0))
	f.Add(float32(-1))
	f.Add(float32(1))
	f.Add(float32(math.NaN()))
	f.Add(float32(math.Inf(1)))
	f.Add(float32(math.Inf(-1)))
	f.Add(float32(math.MaxFloat32))
	f.Add(float32(-math.MaxFloat32))

	f.Fuzz(func(t *testing.T, v float32) {
		got := nonNegF32(v)
		if math.IsNaN(float64(got)) {
			t.Fatalf("nonNegF32(%v)=NaN", v)
		}
		if !math.IsInf(float64(got), 0) && got < 0 {
			t.Fatalf("nonNegF32(%v)=%v (<0)", v, got)
		}
	})
}

// FuzzOverrideScalar asserts:
//   - mask bit unset → returns base unchanged
//   - NaN v → returns base (non-finite fallback)
//   - finite base + finite v → finite result
func FuzzOverrideScalar(f *testing.F) {
	// seed tuple: base, v, maskSet (bool), additive (bool)
	f.Add(float32(0), float32(0), true, false)
	f.Add(float32(3), float32(5), true, true)
	f.Add(float32(3), float32(math.NaN()), true, false)
	f.Add(float32(3), float32(math.Inf(1)), true, false)
	f.Add(float32(3), float32(5), false, false)
	f.Add(float32(math.NaN()), float32(5), true, true)

	f.Fuzz(func(t *testing.T, base, v float32, maskSet, additive bool) {
		ov := gui.SvgAnimAttrOverride{}
		bit := gui.SvgAnimMaskCX
		if maskSet {
			ov.Mask = bit
		}
		if additive {
			ov.AdditiveMask = bit
		}
		got := overrideScalar(base, v, &ov, bit)
		if !maskSet {
			if !bitEqualF32(got, base) {
				t.Fatalf("mask unset: got %v want base %v", got, base)
			}
			return
		}
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			if !bitEqualF32(got, base) {
				t.Fatalf("non-finite v=%v: got %v want base %v",
					v, got, base)
			}
			return
		}
		// Finite v with mask set: finite base → finite got.
		if !math.IsNaN(float64(base)) && !math.IsInf(float64(base), 0) {
			if math.IsNaN(float64(got)) {
				t.Fatalf("finite base/v produced NaN: base=%v v=%v",
					base, v)
			}
		}
	})
}

// bitEqualF32 compares two float32 bit-for-bit so NaN==NaN passes
// when both are NaN. Needed because overrideScalar returns base
// verbatim — including NaN base when the mask bit is unset.
func bitEqualF32(a, b float32) bool {
	return math.Float32bits(a) == math.Float32bits(b)
}
