package gui

import (
	"math"
	"testing"
)

// --- lerpKeyframes ---

func TestLerpKeyframes_EmptyReturnsOne(t *testing.T) {
	got := lerpKeyframes(nil, nil, nil, SvgAnimCalcLinear, 0.5)
	if got != 1 {
		t.Errorf("empty keyframes: got %v want 1", got)
	}
}

func TestLerpKeyframes_SingleValue(t *testing.T) {
	got := lerpKeyframes([]float32{42}, nil, nil, SvgAnimCalcLinear, 0.5)
	if got != 42 {
		t.Errorf("single value: got %v want 42", got)
	}
}

func TestLerpKeyframes_TwoValuesLinear(t *testing.T) {
	vals := []float32{0, 100}
	got := lerpKeyframes(vals, nil, nil, SvgAnimCalcLinear, 0.25)
	if got != 25 {
		t.Errorf("linear t=0.25: got %v want 25", got)
	}
	got2 := lerpKeyframes(vals, nil, nil, SvgAnimCalcLinear, 0.75)
	if got2 != 75 {
		t.Errorf("linear t=0.75: got %v want 75", got2)
	}
}

func TestLerpKeyframes_AtEndReturnsLast(t *testing.T) {
	vals := []float32{10, 20, 30}
	got := lerpKeyframes(vals, nil, nil, SvgAnimCalcLinear, 1.0)
	if got != 30 {
		t.Errorf("at end: got %v want 30", got)
	}
}

func TestLerpKeyframes_FracAboveOneReturnsLast(t *testing.T) {
	vals := []float32{10, 20}
	got := lerpKeyframes(vals, nil, nil, SvgAnimCalcLinear, 5.0)
	if got != 20 {
		t.Errorf("frac>1: got %v want 20", got)
	}
}

func TestLerpKeyframes_Discrete(t *testing.T) {
	vals := []float32{10, 30, 50}
	// n=3, discrete with default keyTimes: idx = int(frac*n), capped.
	got := lerpKeyframes(vals, nil, nil, SvgAnimCalcDiscrete, 0.1)
	if got != 10 {
		t.Errorf("discrete frac=0.1: got %v want 10", got)
	}
	got2 := lerpKeyframes(vals, nil, nil, SvgAnimCalcDiscrete, 0.5)
	if got2 != 30 {
		t.Errorf("discrete frac=0.5: got %v want 30", got2)
	}
	got3 := lerpKeyframes(vals, nil, nil, SvgAnimCalcDiscrete, 0.7)
	if got3 != 50 {
		t.Errorf("discrete frac=0.7: got %v want 50", got3)
	}
}

func TestLerpKeyframes_NaNInput(t *testing.T) {
	vals := []float32{0, 100}
	got := lerpKeyframes(vals, nil, nil, SvgAnimCalcLinear, float32(math.NaN()))
	// locateSeg treats NaN as <=0 and returns idx=0, t=NaN, atEnd=false.
	// lerp: vals[0] + (vals[1]-vals[0])*NaN = NaN.
	// This test verifies no panic; NaN result is acceptable (caller
	// should sanitize).
	if math.IsNaN(float64(got)) {
		t.Log("NaN input produces NaN (expected without sanitization)")
	}
}

func TestLerpKeyframes_InfInput(t *testing.T) {
	vals := []float32{0, 100}
	got := lerpKeyframes(vals, nil, nil, SvgAnimCalcLinear, float32(math.Inf(1)))
	// Positive infinity treated as >=1 → atEnd, returns last.
	if got != 100 {
		t.Errorf("+Inf: got %v want 100", got)
	}
	got2 := lerpKeyframes(vals, nil, nil, SvgAnimCalcLinear, float32(math.Inf(-1)))
	// Negative infinity treated as <0 → idx=0, t depends on locateSeg.
	// Verify no panic.
	_ = got2
}

func TestLerpKeyframes_NegativeFrac(t *testing.T) {
	vals := []float32{5, 95}
	got := lerpKeyframes(vals, nil, nil, SvgAnimCalcLinear, -0.5)
	// locateSeg clamps: frac<0 → idx=0, t=0 (clamped).
	if got != 5 {
		t.Errorf("negative frac: got %v want 5", got)
	}
}

// --- lerpKeyframes2D ---

func TestLerpKeyframes2D_EmptyReturnsZero(t *testing.T) {
	x, y := lerpKeyframes2D(nil, nil, nil, SvgAnimCalcLinear, 0.5)
	if x != 0 || y != 0 {
		t.Errorf("empty: got (%v,%v) want (0,0)", x, y)
	}
}

func TestLerpKeyframes2D_OddLengthTruncates(t *testing.T) {
	// 3 values → n=1 (single point via n/2 truncation).
	x, y := lerpKeyframes2D([]float32{10, 20, 30}, nil, nil, SvgAnimCalcLinear, 0.5)
	if x != 10 || y != 20 {
		t.Errorf("odd length: got (%v,%v) want (10,20)", x, y)
	}
}

func TestLerpKeyframes2D_SinglePoint(t *testing.T) {
	x, y := lerpKeyframes2D([]float32{5, 10}, nil, nil, SvgAnimCalcLinear, 0.5)
	if x != 5 || y != 10 {
		t.Errorf("single point: got (%v,%v) want (5,10)", x, y)
	}
}

func TestLerpKeyframes2D_TwoPointsLinear(t *testing.T) {
	vals := []float32{0, 0, 100, 200}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, 0.5)
	if x != 50 || y != 100 {
		t.Errorf("linear t=0.5: got (%v,%v) want (50,100)", x, y)
	}
}

func TestLerpKeyframes2D_AtEnd(t *testing.T) {
	vals := []float32{1, 2, 3, 4, 5, 6}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, 1.0)
	if x != 5 || y != 6 {
		t.Errorf("at end: got (%v,%v) want (5,6)", x, y)
	}
}

func TestLerpKeyframes2D_Discrete(t *testing.T) {
	vals := []float32{10, 20, 30, 40, 50, 60}
	// n=3: frac=0.3 → idx=0; frac=0.5 → idx=1; frac=0.8 → idx=2
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcDiscrete, 0.3)
	if x != 10 || y != 20 {
		t.Errorf("discrete frac=0.3: got (%v,%v) want (10,20)", x, y)
	}
	x2, y2 := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcDiscrete, 0.5)
	if x2 != 30 || y2 != 40 {
		t.Errorf("discrete frac=0.5: got (%v,%v) want (30,40)", x2, y2)
	}
	x3, y3 := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcDiscrete, 0.8)
	if x3 != 50 || y3 != 60 {
		t.Errorf("discrete frac=0.8: got (%v,%v) want (50,60)", x3, y3)
	}
}

func TestLerpKeyframes2D_NaNInput(t *testing.T) {
	vals := []float32{0, 0, 100, 200}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, float32(math.NaN()))
	// No panic is the contract. NaN result is acceptable.
	_ = x
	_ = y
}

func TestLerpKeyframes2D_InfInput(t *testing.T) {
	vals := []float32{0, 0, 100, 200}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, float32(math.Inf(1)))
	// +Inf → atEnd, returns last point.
	if x != 100 || y != 200 {
		t.Errorf("+Inf: got (%v,%v) want (100,200)", x, y)
	}
	x2, y2 := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, float32(math.Inf(-1)))
	// -Inf → clamped to idx=0, t=0.
	if x2 != 0 || y2 != 0 {
		t.Errorf("-Inf: got (%v,%v) want (0,0)", x2, y2)
	}
}

// --- lerpColorKeyframes edge cases ---

func TestLerpColorKeyframes_EmptyKeyframesNoPanic(t *testing.T) {
	got := lerpColorKeyframes(nil, nil, nil, SvgAnimCalcLinear, 0.5)
	zero := SvgColor{}
	if got != zero {
		t.Errorf("empty keyframes: got %+v want zero", got)
	}
}

func TestLerpColorKeyframes_EmptySlice(t *testing.T) {
	got := lerpColorKeyframes([]uint32{}, nil, nil, SvgAnimCalcLinear, 0.5)
	zero := SvgColor{}
	if got != zero {
		t.Errorf("empty slice: got %+v want zero", got)
	}
}

func TestLerpColorKeyframes_SingleStop(t *testing.T) {
	v := uint32(0x11223344)
	got := lerpColorKeyframes([]uint32{v}, nil, nil, SvgAnimCalcLinear, 0.5)
	want := unpackRGBA(v)
	if got != want {
		t.Errorf("single stop frac=0.5: got %+v want %+v", got, want)
	}
	got2 := lerpColorKeyframes([]uint32{v}, nil, nil, SvgAnimCalcLinear, 1.0)
	if got2 != want {
		t.Errorf("single stop frac=1.0: got %+v want %+v", got2, want)
	}
}
