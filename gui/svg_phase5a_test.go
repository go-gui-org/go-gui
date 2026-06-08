package gui

import (
	"math"
	"testing"
)

// TestPhase5aLerp2DLinear — pair [0,10, 10,0] at frac=0.5 should
// yield (5, 5).
func TestPhase5aLerp2DLinear(t *testing.T) {
	vals := []float32{0, 10, 10, 0}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, 0.5)
	if f32AbsP5(x-5) > 1e-5 || f32AbsP5(y-5) > 1e-5 {
		t.Fatalf("want (5,5), got (%g,%g)", x, y)
	}
}

// TestPhase5aLerp2DEdgeClamp — frac=0 gives first pair, frac>=1
// gives last pair.
func TestPhase5aLerp2DEdgeClamp(t *testing.T) {
	vals := []float32{12, 12, 0, 0}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, 0)
	if x != 12 || y != 12 {
		t.Fatalf("frac=0: want (12,12), got (%g,%g)", x, y)
	}
	x, y = lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, 1)
	if x != 0 || y != 0 {
		t.Fatalf("frac=1: want (0,0), got (%g,%g)", x, y)
	}
}

// TestPhase5aComputeTranslateScalePopulatesState verifies that
// computeSvgAnimations threads translate + scale into svgAnimState.
func TestPhase5aComputeTranslateScalePopulatesState(t *testing.T) {
	anims := []SvgAnimation{
		{
			Kind:          SvgAnimTranslate,
			GroupID:       "g",
			TargetPathIDs: []uint32{1},
			Values:        []float32{12, 12, 0, 0},
			DurSec:        1,
		},
		{
			Kind:          SvgAnimScale,
			GroupID:       "g",
			TargetPathIDs: []uint32{1},
			Values:        []float32{0, 0, 1, 1},
			DurSec:        1,
		},
	}
	states := computeSvgAnimations(anims, 0.5, nil)
	st, ok := states[1]
	if !ok {
		t.Fatal("state for 'g' missing")
	}
	if !st.HasXform {
		t.Fatal("HasXform must be true")
	}
	if f32AbsP5(st.TransX-6) > 1e-5 || f32AbsP5(st.TransY-6) > 1e-5 {
		t.Fatalf("translate mid want (6,6), got (%g,%g)",
			st.TransX, st.TransY)
	}
	if f32AbsP5(st.ScaleX-0.5) > 1e-5 ||
		f32AbsP5(st.ScaleY-0.5) > 1e-5 {
		t.Fatalf("scale mid want (0.5,0.5), got (%g,%g)",
			st.ScaleX, st.ScaleY)
	}
}

// TestPhase5aComputeDefaultsWhenNoXform — opacity-only animation
// leaves HasXform false and identity scale on the returned state.
func TestPhase5aComputeDefaultsWhenNoXform(t *testing.T) {
	anims := []SvgAnimation{
		{
			Kind:          SvgAnimOpacity,
			GroupID:       "g",
			TargetPathIDs: []uint32{1},
			Values:        []float32{1, 0},
			DurSec:        1,
		},
	}
	states := computeSvgAnimations(anims, 0.5, nil)
	st := states[1]
	if st.HasXform {
		t.Fatal("HasXform must stay false for opacity-only anim")
	}
	// Identity scale still initialized so any future transform
	// layered onto the same group inherits (1,1), not (0,0).
	if st.ScaleX != 1 || st.ScaleY != 1 {
		t.Fatalf("identity scale want (1,1), got (%g,%g)",
			st.ScaleX, st.ScaleY)
	}
}

// TestLerpKeyframes2DNegativeFracReturnsFirst — negative frac
// must clamp to 0 and return the first (x,y) pair without
// panicking on a negative slice index.
func TestLerpKeyframes2DNegativeFracReturnsFirst(t *testing.T) {
	vals := []float32{10, 20, 0, 0}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, -5)
	if x != 10 || y != 20 {
		t.Fatalf("negative frac should clamp to first, got (%g,%g)", x, y)
	}
}

// TestLerpKeyframes2DNaNFracReturnsFirst — NaN frac must clamp
// to 0 (first pair).
func TestLerpKeyframes2DNaNFracReturnsFirst(t *testing.T) {
	vals := []float32{10, 20, 0, 0}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, float32(math.NaN()))
	if x != 10 || y != 20 {
		t.Fatalf("NaN frac should clamp to first, got (%g,%g)", x, y)
	}
}

// TestLerpKeyframes2DSplineBendsBothAxes — with an ease-out
// spline, the 2D Lerp at frac=0.5 must bend both x and y past
// the linear midpoint the same way the 1D form does.
func TestLerpKeyframes2DSplineBendsBothAxes(t *testing.T) {
	vals := []float32{0, 0, 100, 100}
	splines := []float32{.33, .66, .66, 1}
	linX, linY := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcLinear, 0.5)
	easedX, easedY := lerpKeyframes2D(vals, splines, nil, SvgAnimCalcSpline, 0.5)
	if easedX <= linX {
		t.Fatalf("expected x ease-out > linear: lin=%g eased=%g",
			linX, easedX)
	}
	if easedY <= linY {
		t.Fatalf("expected y ease-out > linear: lin=%g eased=%g",
			linY, easedY)
	}
	if f32AbsP5(easedX-easedY) > 1e-5 {
		t.Fatalf("symmetric 0→100 axes should match; got (%g,%g)",
			easedX, easedY)
	}
}

// TestLerpKeyframes2DDiscreteHoldsPair — discrete mode on a paired
// keyframe stream returns the covering pair unchanged.
func TestLerpKeyframes2DDiscreteHoldsPair(t *testing.T) {
	vals := []float32{0, 0, 10, 20, 30, 40, 50, 60}
	x, y := lerpKeyframes2D(vals, nil, nil, SvgAnimCalcDiscrete, 0.3)
	// n=4 pairs, idx = floor(0.3*4) = 1 → (10, 20).
	if x != 10 || y != 20 {
		t.Fatalf("discrete mid want (10,20), got (%g,%g)", x, y)
	}
	x, y = lerpKeyframes2D(vals, nil, nil, SvgAnimCalcDiscrete, 1.0)
	if x != 50 || y != 60 {
		t.Fatalf("discrete end want (50,60), got (%g,%g)", x, y)
	}
}

func f32AbsP5(f float32) float32 {
	return float32(math.Abs(float64(f)))
}
