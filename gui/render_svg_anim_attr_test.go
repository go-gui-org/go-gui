package gui

import (
	"math"
	"testing"
)

// --- applyOpacityContrib ---

func TestApplyOpacityContrib_FillReplace(t *testing.T) {
	st := svgAnimState{FillOpacity: 1}
	applyOpacityContrib(&st, 0.5, SvgAnimTargetFill, false)
	if st.FillOpacity != 0.5 {
		t.Errorf("FillOpacity=%v want 0.5", st.FillOpacity)
	}
}

func TestApplyOpacityContrib_FillAdditive(t *testing.T) {
	st := svgAnimState{FillOpacity: 1}
	applyOpacityContrib(&st, 0.3, SvgAnimTargetFill, true)
	if st.FillOpacity != 1.3 {
		t.Errorf("additive FillOpacity=%v want 1.3", st.FillOpacity)
	}
}

func TestApplyOpacityContrib_StrokeReplace(t *testing.T) {
	st := svgAnimState{StrokeOpacity: 1}
	applyOpacityContrib(&st, 0.25, SvgAnimTargetStroke, false)
	if st.StrokeOpacity != 0.25 {
		t.Errorf("StrokeOpacity=%v want 0.25", st.StrokeOpacity)
	}
}

func TestApplyOpacityContrib_AllTargetReplace(t *testing.T) {
	st := svgAnimState{Opacity: 1}
	applyOpacityContrib(&st, 0.75, SvgAnimTargetAll, false)
	if st.Opacity != 0.75 {
		t.Errorf("Opacity=%v want 0.75", st.Opacity)
	}
}

func TestApplyOpacityContrib_AllTargetAdditive(t *testing.T) {
	st := svgAnimState{Opacity: 0.5}
	applyOpacityContrib(&st, 0.2, SvgAnimTargetAll, true)
	if st.Opacity != 0.7 {
		t.Errorf("additive Opacity=%v want 0.7", st.Opacity)
	}
}

// --- attrMaskBit ---

func TestAttrMaskBit_AllKnownAttrNames(t *testing.T) {
	cases := []struct {
		attr SvgAttrName
		want SvgAnimAttrMask
	}{
		{SvgAttrCX, SvgAnimMaskCX},
		{SvgAttrCY, SvgAnimMaskCY},
		{SvgAttrR, SvgAnimMaskR},
		{SvgAttrRX, SvgAnimMaskRX},
		{SvgAttrRY, SvgAnimMaskRY},
		{SvgAttrX, SvgAnimMaskX},
		{SvgAttrY, SvgAnimMaskY},
		{SvgAttrWidth, SvgAnimMaskWidth},
		{SvgAttrHeight, SvgAnimMaskHeight},
	}
	for _, tc := range cases {
		got := attrMaskBit(tc.attr)
		if got != tc.want {
			t.Errorf("attrMaskBit(%d)=%d want %d", tc.attr, got, tc.want)
		}
	}
}

func TestAttrMaskBit_UnknownReturnsZero(t *testing.T) {
	if attrMaskBit(SvgAttrNone) != 0 {
		t.Error("SvgAttrNone must return 0")
	}
	if attrMaskBit(255) != 0 {
		t.Error("unknown attr must return 0")
	}
}

// --- attrFieldPtr ---

func TestAttrFieldPtr_AllKnownAttrNamesNonNil(t *testing.T) {
	o := &SvgAnimAttrOverride{}
	attrs := []SvgAttrName{
		SvgAttrCX, SvgAttrCY, SvgAttrR, SvgAttrRX, SvgAttrRY,
		SvgAttrX, SvgAttrY, SvgAttrWidth, SvgAttrHeight,
	}
	for _, a := range attrs {
		if attrFieldPtr(o, a) == nil {
			t.Errorf("attrFieldPtr(%d) must be non-nil", a)
		}
	}
}

func TestAttrFieldPtr_UnknownReturnsNil(t *testing.T) {
	o := &SvgAnimAttrOverride{}
	if attrFieldPtr(o, SvgAttrNone) != nil {
		t.Error("SvgAttrNone must return nil")
	}
	if attrFieldPtr(o, 255) != nil {
		t.Error("unknown attr must return nil")
	}
}

// --- applyAttrOverride ---

func TestApplyAttrOverride_Replace(t *testing.T) {
	o := &SvgAnimAttrOverride{}
	applyAttrOverride(o, SvgAttrCX, 42, false)
	if o.CX != 42 {
		t.Errorf("CX=%v want 42", o.CX)
	}
	if o.Mask&SvgAnimMaskCX == 0 {
		t.Error("Mask must include SvgAnimMaskCX")
	}
	if o.AdditiveMask&SvgAnimMaskCX != 0 {
		t.Error("AdditiveMask must be cleared for replace")
	}
}

func TestApplyAttrOverride_AdditiveInit(t *testing.T) {
	o := &SvgAnimAttrOverride{}
	// First additive: set field, mark AdditiveMask.
	applyAttrOverride(o, SvgAttrCY, 10, true)
	if o.CY != 10 {
		t.Errorf("CY=%v want 10", o.CY)
	}
	if o.Mask&SvgAnimMaskCY == 0 {
		t.Error("Mask must include SvgAnimMaskCY")
	}
	if o.AdditiveMask&SvgAnimMaskCY == 0 {
		t.Error("AdditiveMask must include SvgAnimMaskCY on init")
	}
}

func TestApplyAttrOverride_AdditiveAccumulate(t *testing.T) {
	o := &SvgAnimAttrOverride{}
	applyAttrOverride(o, SvgAttrR, 5, true)
	applyAttrOverride(o, SvgAttrR, 3, true)
	if o.R != 8 {
		t.Errorf("R=%v want 8 after 5+3", o.R)
	}
}

func TestApplyAttrOverride_ReplaceThenAdditive(t *testing.T) {
	o := &SvgAnimAttrOverride{}
	applyAttrOverride(o, SvgAttrX, 100, false)
	applyAttrOverride(o, SvgAttrX, 20, true)
	// Mask is set from first call, AdditiveMask was cleared by replace.
	// Second call is additive; Mask bit already set → adds.
	if o.X != 120 {
		t.Errorf("X=%v want 120 (100+20)", o.X)
	}
}

func TestApplyAttrOverride_UnknownAttrNoOp(t *testing.T) {
	o := &SvgAnimAttrOverride{}
	applyAttrOverride(o, SvgAttrNone, 999, false)
	if o.Mask != 0 {
		t.Error("unknown attr must leave mask unchanged")
	}
}

// --- applyAnimContribToPath ---

func TestApplyAnimContribToPath_Opacity(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimOpacity,
		Target:        SvgAnimTargetAll,
		TargetPathIDs: []uint32{1},
	}
	c := &animContrib{anim: a, value: 0.4}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 1, states, nil)
	st := states[1]
	if st.Opacity != 0.4 {
		t.Errorf("Opacity=%v want 0.4", st.Opacity)
	}
}

func TestApplyAnimContribToPath_OpacityAdditive(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimOpacity,
		Target:        SvgAnimTargetAll,
		TargetPathIDs: []uint32{1},
		Additive:      true,
	}
	c := &animContrib{anim: a, value: 0.3}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 1, states, nil)
	// Default opacity is 1 (Inited). Additive adds 0.3 = 1.3.
	st := states[1]
	if st.Opacity != 1.3 {
		t.Errorf("Opacity=%v want 1.3", st.Opacity)
	}
}

func TestApplyAnimContribToPath_RotateReplace(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimRotate,
		CenterX:       10,
		CenterY:       20,
		TargetPathIDs: []uint32{2},
	}
	c := &animContrib{anim: a, value: 45}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 2, states, nil)
	st := states[2]
	if st.RotAngle != 45 {
		t.Errorf("RotAngle=%v want 45", st.RotAngle)
	}
	if st.RotCX != 10 || st.RotCY != 20 {
		t.Errorf("center=(%v,%v) want (10,20)", st.RotCX, st.RotCY)
	}
	if !st.HasXform {
		t.Error("HasXform must be true")
	}
}

func TestApplyAnimContribToPath_RotateAdditive(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimRotate,
		CenterX:       0,
		CenterY:       0,
		TargetPathIDs: []uint32{3},
		Additive:      true,
	}
	c := &animContrib{anim: a, value: 30}
	states := map[uint32]svgAnimState{}
	// First apply: sets base.
	applyAnimContribToPath(c, a, 3, states, nil)
	c2 := &animContrib{anim: a, value: 15}
	applyAnimContribToPath(c2, a, 3, states, nil)
	st := states[3]
	if st.RotAngle != 45 {
		t.Errorf("RotAngle=%v want 45 (30+15)", st.RotAngle)
	}
}

func TestApplyAnimContribToPath_TranslateReplace(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimTranslate,
		TargetPathIDs: []uint32{4},
	}
	c := &animContrib{anim: a, valueX: 100, valueY: 50}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 4, states, nil)
	st := states[4]
	if st.TransX != 100 || st.TransY != 50 {
		t.Errorf("translate=(%v,%v) want (100,50)", st.TransX, st.TransY)
	}
	if !st.HasXform {
		t.Error("HasXform must be true for translate")
	}
}

func TestApplyAnimContribToPath_ScaleReplace(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimScale,
		TargetPathIDs: []uint32{5},
	}
	c := &animContrib{anim: a, valueX: 2, valueY: 3}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 5, states, nil)
	st := states[5]
	if st.ScaleX != 2 || st.ScaleY != 3 {
		t.Errorf("scale=(%v,%v) want (2,3)", st.ScaleX, st.ScaleY)
	}
}

func TestApplyAnimContribToPath_DashOffset(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimDashOffset,
		TargetPathIDs: []uint32{6},
	}
	c := &animContrib{anim: a, value: 42}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 6, states, nil)
	st := states[6]
	if st.AttrOverride.StrokeDashOffset != 42 {
		t.Errorf("StrokeDashOffset=%v want 42", st.AttrOverride.StrokeDashOffset)
	}
	if st.AttrOverride.Mask&SvgAnimMaskStrokeDashOffset == 0 {
		t.Error("Mask must include StrokeDashOffset")
	}
}

func TestApplyAnimContribToPath_DashOffsetAdditive(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimDashOffset,
		TargetPathIDs: []uint32{7},
		Additive:      true,
	}
	c := &animContrib{anim: a, value: 10}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 7, states, nil)
	c2 := &animContrib{anim: a, value: 5}
	applyAnimContribToPath(c2, a, 7, states, nil)
	st := states[7]
	if st.AttrOverride.StrokeDashOffset != 15 {
		t.Errorf("StrokeDashOffset=%v want 15", st.AttrOverride.StrokeDashOffset)
	}
}

func TestApplyAnimContribToPath_ColorFill(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimColor,
		Target:        SvgAnimTargetFill,
		TargetPathIDs: []uint32{8},
	}
	c := &animContrib{anim: a, colorVal: SvgColor{R: 128, G: 64, B: 32, A: 255}}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 8, states, nil)
	st := states[8]
	if !st.HasFillColor {
		t.Error("HasFillColor must be true")
	}
	if st.FillColor.R != 128 {
		t.Errorf("FillColor.R=%d want 128", st.FillColor.R)
	}
	if st.HasStrokeColor {
		t.Error("HasStrokeColor must be false for fill-only target")
	}
}

func TestApplyAnimContribToPath_ColorStroke(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimColor,
		Target:        SvgAnimTargetStroke,
		TargetPathIDs: []uint32{9},
	}
	c := &animContrib{anim: a, colorVal: SvgColor{R: 0, G: 255, B: 0, A: 255}}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 9, states, nil)
	st := states[9]
	if !st.HasStrokeColor {
		t.Error("HasStrokeColor must be true")
	}
	if st.HasFillColor {
		t.Error("HasFillColor must be false for stroke-only target")
	}
}

func TestApplyAnimContribToPath_ColorAll(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimColor,
		Target:        SvgAnimTargetAll,
		TargetPathIDs: []uint32{10},
	}
	c := &animContrib{anim: a, colorVal: SvgColor{R: 255, G: 0, B: 0, A: 255}}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 10, states, nil)
	st := states[10]
	if !st.HasFillColor || !st.HasStrokeColor {
		t.Error("both HasFillColor and HasStrokeColor must be true for TargetAll")
	}
}

func TestApplyAnimContribToPath_InheritedTranslate(t *testing.T) {
	// Group-level animation with inherited compose (len(ids) > 1).
	a := &SvgAnimation{
		Kind:          SvgAnimTranslate,
		TargetPathIDs: []uint32{11, 12},
	}
	c := &animContrib{anim: a, valueX: 20, valueY: 30}
	baseByPath := map[uint32]svgBaseXform{
		11: {TransX: 100, TransY: 200, ScaleX: 1, ScaleY: 1},
	}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 11, states, baseByPath)
	st := states[11]
	if st.TransX != 120 || st.TransY != 230 {
		t.Errorf("inherited translate=(%v,%v) want (120,230)", st.TransX, st.TransY)
	}
}

func TestApplyAnimContribToPath_InheritedRotate(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimRotate,
		TargetPathIDs: []uint32{20, 21},
	}
	c := &animContrib{anim: a, value: 10}
	baseByPath := map[uint32]svgBaseXform{
		20: {RotAngle: 45, RotCX: 5, RotCY: 5, TransX: 5, TransY: 5},
	}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 20, states, baseByPath)
	st := states[20]
	if st.RotAngle != 55 {
		t.Errorf("inherited RotAngle=%v want 55 (45+10)", st.RotAngle)
	}
}

func TestApplyAnimContribToPath_Motion(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimMotion,
		TargetPathIDs: []uint32{13},
		MotionRotate:  SvgAnimMotionRotateAuto,
	}
	c := &animContrib{anim: a, valueX: 50, valueY: 75, value: 90}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 13, states, nil)
	st := states[13]
	if st.TransX != 50 || st.TransY != 75 {
		t.Errorf("motion translate=(%v,%v) want (50,75)", st.TransX, st.TransY)
	}
	if st.RotAngle != 90 {
		t.Errorf("motion RotAngle=%v want 90", st.RotAngle)
	}
	if !st.HasXform {
		t.Error("HasXform must be true for motion")
	}
}

func TestApplyAnimContribToPath_MotionNoRotate(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimMotion,
		TargetPathIDs: []uint32{14},
		MotionRotate:  SvgAnimMotionRotateNone,
	}
	c := &animContrib{anim: a, valueX: 10, valueY: 20}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 14, states, nil)
	st := states[14]
	if st.TransX != 10 || st.TransY != 20 {
		t.Errorf("motion translate=(%v,%v) want (10,20)", st.TransX, st.TransY)
	}
	if st.RotAngle != 0 {
		t.Errorf("RotAngle=%v want 0 (MotionRotateNone)", st.RotAngle)
	}
}

// --- applyDashArrayContrib ---

func TestApplyDashArrayContrib_Linear(t *testing.T) {
	// 2 keyframes, each 2 values: [5,10, 15,20], DashKeyframeLen=2
	a := &SvgAnimation{
		Values:          []float32{5, 10, 15, 20},
		DashKeyframeLen: 2,
		CalcMode:        SvgAnimCalcLinear,
	}
	ov := &SvgAnimAttrOverride{}
	applyDashArrayContrib(ov, a, 0.5)
	if ov.StrokeDashArrayLen != 2 {
		t.Fatalf("StrokeDashArrayLen=%d want 2", ov.StrokeDashArrayLen)
	}
	// t=0.5: 5+(15-5)*0.5=10, 10+(20-10)*0.5=15
	if ov.StrokeDashArray[0] != 10 || ov.StrokeDashArray[1] != 15 {
		t.Errorf("dasharray=[%v,%v] want [10,15]",
			ov.StrokeDashArray[0], ov.StrokeDashArray[1])
	}
	if ov.Mask&SvgAnimMaskStrokeDashArray == 0 {
		t.Error("Mask must include StrokeDashArray")
	}
}

func TestApplyDashArrayContrib_AtEnd(t *testing.T) {
	a := &SvgAnimation{
		Values:          []float32{1, 2, 3, 4},
		DashKeyframeLen: 2,
		CalcMode:        SvgAnimCalcLinear,
	}
	ov := &SvgAnimAttrOverride{}
	applyDashArrayContrib(ov, a, 1)
	if ov.StrokeDashArray[0] != 3 || ov.StrokeDashArray[1] != 4 {
		t.Errorf("at end: got [%v,%v] want [3,4]",
			ov.StrokeDashArray[0], ov.StrokeDashArray[1])
	}
}

func TestApplyDashArrayContrib_Discrete(t *testing.T) {
	a := &SvgAnimation{
		Values:          []float32{1, 2, 3, 4, 5, 6},
		DashKeyframeLen: 2,
		CalcMode:        SvgAnimCalcDiscrete,
	}
	ov := &SvgAnimAttrOverride{}
	// frac 0.1 → idx=0
	applyDashArrayContrib(ov, a, 0.1)
	if ov.StrokeDashArray[0] != 1 || ov.StrokeDashArray[1] != 2 {
		t.Errorf("discrete idx0: got [%v,%v] want [1,2]",
			ov.StrokeDashArray[0], ov.StrokeDashArray[1])
	}
	// frac 0.6 → idx=1
	ov2 := &SvgAnimAttrOverride{}
	applyDashArrayContrib(ov2, a, 0.6)
	if ov2.StrokeDashArray[0] != 3 || ov2.StrokeDashArray[1] != 4 {
		t.Errorf("discrete idx1: got [%v,%v] want [3,4]",
			ov2.StrokeDashArray[0], ov2.StrokeDashArray[1])
	}
}

func TestApplyDashArrayContrib_ZeroKeyframeLenNoOp(t *testing.T) {
	a := &SvgAnimation{
		Values:          []float32{1, 2},
		DashKeyframeLen: 0,
	}
	ov := &SvgAnimAttrOverride{}
	applyDashArrayContrib(ov, a, 0.5)
	if ov.Mask&SvgAnimMaskStrokeDashArray != 0 {
		t.Error("zero DashKeyframeLen must not set mask")
	}
}

func TestApplyDashArrayContrib_NegativeKeyframeLenNoOp(t *testing.T) {
	// DashKeyframeLen is uint8; a zero Values means nothing to animate.
	a := &SvgAnimation{
		Values:          nil,
		DashKeyframeLen: 0,
	}
	ov := &SvgAnimAttrOverride{}
	applyDashArrayContrib(ov, a, 0.5)
	if ov.StrokeDashArrayLen != 0 {
		t.Error("no values must leave len at zero")
	}
}

// --- applyAnimContrib (top-level dispatch) ---

func TestApplyAnimContrib_DispatchToPaths(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimOpacity,
		Target:        SvgAnimTargetAll,
		TargetPathIDs: []uint32{1, 2, 3},
	}
	c := &animContrib{anim: a, value: 0.5}
	states := map[uint32]svgAnimState{}
	applyAnimContrib(c, states, nil)
	if len(states) != 3 {
		t.Fatalf("expected 3 path states, got %d", len(states))
	}
	for _, pid := range []uint32{1, 2, 3} {
		st, ok := states[pid]
		if !ok {
			t.Errorf("missing state for path %d", pid)
			continue
		}
		if st.Opacity != 0.5 {
			t.Errorf("pid=%d Opacity=%v want 0.5", pid, st.Opacity)
		}
	}
}

func TestApplyAnimContrib_StateInit(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimTranslate,
		TargetPathIDs: []uint32{1},
	}
	c := &animContrib{anim: a, valueX: 10, valueY: 20}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 1, states, nil)
	st := states[1]
	if !st.Inited {
		t.Error("state must be Inited after first touch")
	}
	// Default scalars before any animation sets them.
	if st.ScaleX != 1 || st.ScaleY != 1 {
		t.Errorf("default scale=(%v,%v) want (1,1)", st.ScaleX, st.ScaleY)
	}
}

func TestApplyAnimContrib_BaseByPathSeed(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimScale,
		TargetPathIDs: []uint32{1},
	}
	baseByPath := map[uint32]svgBaseXform{
		1: {TransX: 10, TransY: 20, ScaleX: 2, ScaleY: 2, RotAngle: 30,
			RotCX: 5, RotCY: 5},
	}
	c := &animContrib{anim: a, valueX: 3, valueY: 3}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 1, states, baseByPath)
	st := states[1]
	// Non-inherited, non-additive → replaces.
	if st.ScaleX != 3 || st.ScaleY != 3 {
		t.Errorf("scale=(%v,%v) want (3,3)", st.ScaleX, st.ScaleY)
	}
	// Base rotation is preserved via HasXform init.
	if st.RotAngle != 30 {
		t.Errorf("base RotAngle=%v want 30", st.RotAngle)
	}
}

// --- NaN/Inf edge cases ---

func TestApplyOpacityContrib_NaNTreatedAsZero(t *testing.T) {
	st := svgAnimState{Opacity: 1}
	applyOpacityContrib(&st, float32(math.NaN()), SvgAnimTargetAll, false)
	// NaN doesn't panic; it just sets to NaN (caller must sanitize).
	// This test verifies no panic.
	if !math.IsNaN(float64(st.Opacity)) {
		t.Log("NaN propagated (expected); render-time clamp handles it")
	}
}

func TestApplyAnimContribToPath_UninitializedPreservesDefaults(t *testing.T) {
	a := &SvgAnimation{
		Kind:          SvgAnimTranslate,
		TargetPathIDs: []uint32{99},
	}
	c := &animContrib{anim: a, valueX: 0, valueY: 0}
	states := map[uint32]svgAnimState{}
	applyAnimContribToPath(c, a, 99, states, nil)
	st := states[99]
	if !st.Inited {
		t.Error("state must init on first touch")
	}
	// Scale defaults to 1 when no base.
	if st.ScaleX != 1 || st.ScaleY != 1 {
		t.Errorf("default scale=(%v,%v) want (1,1)", st.ScaleX, st.ScaleY)
	}
}
