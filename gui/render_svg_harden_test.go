package gui

import (
	"math"
	"testing"
)

// emitSvgPathRenderer must route the rotation pivot to the translate
// offset when a path carries a TRS-decomposed author base transform.
// Backend order is scale → translate → rotate-about-(rcx,rcy); a TRS
// decomposition only matches that order when the pivot equals the
// translate offset. Rotating about origin would mis-position the path.
func TestEmitSvgPathRenderer_BaseTRSRoutesRotPivotToTranslate(t *testing.T) {
	w := &Window{}
	path := CachedSvgPath{
		Triangles:    []float32{0, 0, 1, 0, 0, 1},
		Color:        Color{0, 0, 0, 255, true},
		BaseTransX:   10,
		BaseTransY:   20,
		BaseScaleX:   2,
		BaseScaleY:   2,
		BaseRotAngle: 45,
		HasBaseXform: true,
	}
	emitSvgPathRenderer(path, Color{}, 0, 0, 1, 0, 0, false, nil, w)
	if len(w.renderers) != 1 {
		t.Fatalf("expected 1 renderer, got %d", len(w.renderers))
	}
	rc := w.renderers[0]
	if !rc.HasXform {
		t.Fatal("HasXform must be true when author base xform present")
	}
	if rc.RotCX != 10 || rc.RotCY != 20 {
		t.Fatalf("rotation pivot must equal translate: got (%v,%v) want (10,20)",
			rc.RotCX, rc.RotCY)
	}
	if rc.TransX != 10 || rc.TransY != 20 {
		t.Fatalf("translate lost: got (%v,%v)", rc.TransX, rc.TransY)
	}
	if rc.RotAngle != 45 {
		t.Fatalf("rotation angle lost: got %v", rc.RotAngle)
	}
}

// An animation state present for the GroupID overrides the author
// base; rotCX/rotCY come from the animation, not from transX/transY.
// Guards against a regression that re-derives pivot from animated
// translate when the anim state already specified one.
func TestEmitSvgPathRenderer_AnimStateRotCenterOverridesBase(t *testing.T) {
	w := &Window{}
	path := CachedSvgPath{
		Triangles:    []float32{0, 0, 1, 0, 0, 1},
		Color:        Color{0, 0, 0, 255, true},
		PathID:       1,
		BaseTransX:   10,
		BaseTransY:   20,
		HasBaseXform: true,
	}
	animState := map[uint32]svgAnimState{
		1: {
			Inited:        true,
			Opacity:       1,
			FillOpacity:   1,
			StrokeOpacity: 1,
			HasXform:      true,
			ScaleX:        1, ScaleY: 1,
			TransX: 3, TransY: 4,
			RotAngle: 90,
			RotCX:    7, RotCY: 8,
		},
	}
	emitSvgPathRenderer(path, Color{}, 0, 0, 1, 0, 0, false, animState, w)
	rc := w.renderers[0]
	if rc.RotCX != 7 || rc.RotCY != 8 {
		t.Fatalf("anim rotCenter lost: got (%v,%v) want (7,8)",
			rc.RotCX, rc.RotCY)
	}
	if rc.TransX != 3 || rc.TransY != 4 {
		t.Fatalf("anim translate lost: got (%v,%v)", rc.TransX, rc.TransY)
	}
}

// applyAnimContrib init-from-base seeds RotCX/RotCY from the base
// translate so the first frame's rotation pivot matches the author's
// TRS decomposition. A later SvgAnimRotate contrib then overwrites
// with its own cx/cy if present.
func TestApplyAnimContrib_BaseSeedRotCenterFollowsTranslate(t *testing.T) {
	states := map[uint32]svgAnimState{}
	base := map[uint32]svgBaseXform{
		1: {TransX: 12, TransY: 16, ScaleX: 1, ScaleY: 1, RotAngle: 30},
	}
	a := &SvgAnimation{
		Kind: SvgAnimOpacity, GroupID: "g", TargetPathIDs: []uint32{1},
		Target: SvgAnimTargetAll,
		Values: []float32{0, 1}, DurSec: 1, Cycle: 1,
	}
	applyAnimContrib(&animContrib{anim: a, value: 1}, states, base)
	st := states[1]
	if !st.HasXform {
		t.Fatal("HasXform must be set after base seed")
	}
	if st.RotCX != 12 || st.RotCY != 16 {
		t.Fatalf("base rot center: got (%v,%v) want (12,16)",
			st.RotCX, st.RotCY)
	}

	// A subsequent SvgAnimRotate contrib with explicit center
	// overwrites the seed.
	aRot := &SvgAnimation{
		Kind: SvgAnimRotate, GroupID: "g", TargetPathIDs: []uint32{1},
		CenterX: 50, CenterY: 60,
		Values: []float32{0, 90}, DurSec: 1, Cycle: 1,
	}
	applyAnimContrib(&animContrib{anim: aRot, value: 45}, states, base)
	st = states[1]
	if st.RotCX != 50 || st.RotCY != 60 {
		t.Fatalf("rotate anim must overwrite center: got (%v,%v) want (50,60)",
			st.RotCX, st.RotCY)
	}
	if st.RotAngle != 45 {
		t.Fatalf("rotate angle: got %v want 45", st.RotAngle)
	}
}

// locateSeg clamps spline-evaluated t into [0,1]. A bezier with y
// control points >1 overshoots; unchecked, the out-of-range t would
// drive downstream lerps past either keyframe and, for transform
// animations, produce visibly unstable frames.
func TestLocateSeg_SplineOvershootClampedToUnit(t *testing.T) {
	// Two-keyframe spline animation; splines with y2=2 produce bezier
	// output >1 at mid-t. Expect clamp to 1.
	splines := []float32{0, 0, 1, 2}
	_, tt, _ := locateSeg(2, 0.5, splines, nil, SvgAnimCalcSpline)
	if tt < 0 || tt > 1 {
		t.Fatalf("spline overshoot not clamped: t=%v", tt)
	}
	if tt != 1 {
		// Not a strict requirement — bezier may stay inside — but
		// log if under. Fail only on out-of-range.
		t.Logf("spline t=%v (in range but <1)", tt)
	}
}

// NaN-valued spline control points produce NaN bezier output.
// clampUnit folds NaN to 0 so downstream lerps receive a valid
// progress value instead of propagating NaN through vertex coords.
func TestLocateSeg_NaNSplineFoldsToZero(t *testing.T) {
	nan := float32(math.NaN())
	splines := []float32{0, 0, nan, nan}
	_, tt, _ := locateSeg(2, 0.5, splines, nil, SvgAnimCalcSpline)
	if math.IsNaN(float64(tt)) {
		t.Fatal("NaN spline leaked through: t is NaN")
	}
	if tt != 0 {
		t.Fatalf("NaN spline must fold to 0, got %v", tt)
	}
}

// locateSegKeyTimes applies the same spline clamp when keyTimes
// drives segment selection.
func TestLocateSegKeyTimes_SplineOvershootClampedToUnit(t *testing.T) {
	splines := []float32{0, 0, 1, 2}
	keyTimes := []float32{0, 1}
	_, tt, _ := locateSegKeyTimes(2, 0.5, splines, keyTimes,
		SvgAnimCalcSpline)
	if tt < 0 || tt > 1 {
		t.Fatalf("keytimes spline overshoot not clamped: t=%v", tt)
	}
}

func TestEmitSvgPathRenderer_NonUniformCompoundsWithAnimXform(t *testing.T) {
	// Non-uniform stretch composes multiplicatively with animation
	// transform scales. Base xform skipped when animation active (animApplied takes priority).
	w := &Window{}
	animState := map[uint32]svgAnimState{
		1: {
			HasXform: true,
			ScaleX:   0.25,
			ScaleY:   4,
			Opacity:  1,
			Inited:   true,
		},
	}
	path := CachedSvgPath{
		PathID:       1,
		Triangles:    []float32{0, 0, 10, 0, 5, 10, 5, 10, 10, 0, 10, 10},
		Color:        Color{255, 255, 255, 255, true},
		HasBaseXform: true,
		BaseScaleX:   0.5,
		BaseScaleY:   2,
	}
	// nsScaleX=2, nsScaleY=3, uniform=4 → nonUniform.
	// Anim only: 2*0.25=0.5, 3*4=12. Base xform skipped (animApplied).

	emitSvgPathRenderer(path, Color{}, 0, 0, 4, 2, 3, true, animState, w)

	if len(w.renderers) != 1 {
		t.Fatalf("expected 1 renderer, got %d", len(w.renderers))
	}
	r := w.renderers[0]
	if r.Scale != 1 {
		t.Errorf("Scale = %v, want 1", r.Scale)
	}
	if !r.HasXform {
		t.Fatal("HasXform should be true")
	}
	if r.ScaleX != 0.5 {
		t.Errorf("ScaleX = %v, want 0.5 (2*0.25)", r.ScaleX)
	}
	if r.ScaleY != 12 {
		t.Errorf("ScaleY = %v, want 12 (3*4)", r.ScaleY)
	}
}
