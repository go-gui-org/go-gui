package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// applyOverridesToPath: dash-array override clones the override
// backing into the path so subsequent mutation of the override does
// not corrupt the path's StrokeDasharray. Cache isolation invariant.
func TestApplyOverrides_DashArrayClonesBacking(t *testing.T) {
	p := &VectorPath{Transform: identityTransform}
	ov := gui.SvgAnimAttrOverride{
		Mask:               gui.SvgAnimMaskStrokeDashArray,
		StrokeDashArrayLen: 2,
	}
	ov.StrokeDashArray[0] = 4
	ov.StrokeDashArray[1] = 6
	applyOverridesToPath(p, ov)
	// Mutate override after apply.
	ov.StrokeDashArray[0] = 99
	if p.StrokeDasharray[0] != 4 || p.StrokeDasharray[1] != 6 {
		t.Fatalf("path corrupted by override mutation: %v",
			p.StrokeDasharray)
	}
}

// Length field clamps to SvgAnimDashArrayCap to keep slice access in
// bounds even when caller hands an over-cap len.
func TestApplyOverrides_DashArrayLenClampsToCap(t *testing.T) {
	p := &VectorPath{Transform: identityTransform}
	ov := gui.SvgAnimAttrOverride{
		Mask:               gui.SvgAnimMaskStrokeDashArray,
		StrokeDashArrayLen: 250,
	}
	for i := range ov.StrokeDashArray {
		ov.StrokeDashArray[i] = float32(i + 1)
	}
	applyOverridesToPath(p, ov)
	if len(p.StrokeDasharray) != gui.SvgAnimDashArrayCap {
		t.Fatalf("got len=%d want %d",
			len(p.StrokeDasharray), gui.SvgAnimDashArrayCap)
	}
}

// Replace semantics for dash offset: existing parsed value is
// overwritten when AdditiveMask bit is unset.
func TestApplyOverrides_DashOffsetReplaces(t *testing.T) {
	p := &VectorPath{
		Transform:        identityTransform,
		StrokeDashOffset: 100,
	}
	ov := gui.SvgAnimAttrOverride{
		Mask:             gui.SvgAnimMaskStrokeDashOffset,
		StrokeDashOffset: -25,
	}
	applyOverridesToPath(p, ov)
	if p.StrokeDashOffset != -25 {
		t.Fatalf("offset=%v want -25 (replace)", p.StrokeDashOffset)
	}
}

// Additive semantics: override value is added to the parsed base.
func TestApplyOverrides_DashOffsetAdditiveSums(t *testing.T) {
	p := &VectorPath{
		Transform:        identityTransform,
		StrokeDashOffset: 10,
	}
	ov := gui.SvgAnimAttrOverride{
		Mask:             gui.SvgAnimMaskStrokeDashOffset,
		AdditiveMask:     gui.SvgAnimMaskStrokeDashOffset,
		StrokeDashOffset: 7,
	}
	applyOverridesToPath(p, ov)
	if p.StrokeDashOffset != 17 {
		t.Fatalf("offset=%v want 17 (10+7)", p.StrokeDashOffset)
	}
}

// Dash overrides apply even on non-primitive paths (Kind=None);
// stroke-dasharray/offset are stroke attrs, not primitive geometry.
func TestApplyOverrides_DashAppliesToNonPrimitive(t *testing.T) {
	p := &VectorPath{
		Transform: identityTransform,
		Primitive: gui.SvgPrimitive{Kind: gui.SvgPrimNone},
	}
	ov := gui.SvgAnimAttrOverride{
		Mask:               gui.SvgAnimMaskStrokeDashArray,
		StrokeDashArrayLen: 1,
	}
	ov.StrokeDashArray[0] = 3
	applyOverridesToPath(p, ov)
	if len(p.StrokeDasharray) != 1 || p.StrokeDasharray[0] != 3 {
		t.Fatalf("non-primitive path missed dash override: %v",
			p.StrokeDasharray)
	}
}
