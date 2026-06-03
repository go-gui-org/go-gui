package svg

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// testAnimMotionNode builds an xmlNode from a standalone
// <animateMotion ...> fragment. Returns a pointer into a local tree;
// safe to use for the duration of the test.
func testAnimMotionNode(frag string) *xmlNode {
	root, err := decodeSvgTree(`<svg xmlns="http://www.w3.org/2000/svg">` +
		frag + `</svg>`)
	if err != nil {
		panic(err)
	}
	for i := range root.Children {
		if root.Children[i].Name == "animateMotion" {
			return &root.Children[i]
		}
	}
	panic("no animateMotion in fragment")
}

// --- Time parsing ---

func TestAnimationParseTimeValueSeconds(t *testing.T) {
	v := parseTimeValue("1.5s")
	if f32Abs(v-1.5) > 1e-5 {
		t.Fatalf("expected 1.5, got %f", v)
	}
}

func TestAnimationParseTimeValueMilliseconds(t *testing.T) {
	v := parseTimeValue("200ms")
	if f32Abs(v-0.2) > 1e-5 {
		t.Fatalf("expected 0.2, got %f", v)
	}
}

func TestAnimationParseTimeValueBare(t *testing.T) {
	v := parseTimeValue("3")
	if f32Abs(v-3) > 1e-5 {
		t.Fatalf("expected 3, got %f", v)
	}
}

// --- Float lists ---

func TestAnimationParseSemicolonFloats(t *testing.T) {
	vals := parseSemicolonFloats("0;0.5;1")
	if len(vals) != 3 {
		t.Fatalf("expected 3 values, got %d", len(vals))
	}
	if vals[0] != 0 || f32Abs(vals[1]-0.5) > 1e-5 || vals[2] != 1 {
		t.Fatalf("expected [0,0.5,1], got %v", vals)
	}
}

func TestAnimationParseSemicolonFloatsEmpty(t *testing.T) {
	vals := parseSemicolonFloats("")
	if len(vals) != 0 {
		t.Fatalf("expected empty, got %v", vals)
	}
}

// SMIL from/to/by/values must reject malformed tokens rather than
// coerce to 0. parseF32 silently zeros for tolerant geometry paths;
// animations need ok=false so the malformed timeline is dropped
// instead of producing a real endpoint at zero.
func TestAnimationParseScalarValuesRejectsMalformed(t *testing.T) {
	cases := []string{
		`<animate attributeName="x" from="wat" to="10" dur="1s"/>`,
		`<animate attributeName="x" from="0" to="NaN" dur="1s"/>`,
		`<animate attributeName="x" by="Inf" dur="1s"/>`,
		`<animate attributeName="x" to="bogus" dur="1s"/>`,
		`<animate attributeName="x" values="0;wat;1" dur="1s"/>`,
	}
	for _, elem := range cases {
		if _, _, ok := parseScalarValues(elem); ok {
			t.Errorf("expected reject for %q", elem)
		}
	}
}

func TestAnimationParseScalarValuesAccepts(t *testing.T) {
	if vs, _, ok := parseScalarValues(
		`<animate attributeName="x" from="0" to="10" dur="1s"/>`,
	); !ok || len(vs) != 2 || vs[0] != 0 || vs[1] != 10 {
		t.Fatalf("from/to baseline broken: %v ok=%v", vs, ok)
	}
}

func TestAnimationParseSpaceFloats(t *testing.T) {
	vals := parseSpaceFloats("10 20 30")
	if len(vals) != 3 {
		t.Fatalf("expected 3 values, got %d", len(vals))
	}
	if vals[0] != 10 || vals[1] != 20 || vals[2] != 30 {
		t.Fatalf("expected [10,20,30], got %v", vals)
	}
}

func TestAnimationParseSpaceFloatsEmpty(t *testing.T) {
	vals := parseSpaceFloats("")
	if len(vals) != 0 {
		t.Fatalf("expected empty, got %v", vals)
	}
}

// --- parseAnimateElement ---

func TestAnimationParseAnimateElementValid(t *testing.T) {
	elem := `<animate attributeName="opacity" values="1;0;1" dur="2s" begin="0.5s">`
	gs := ComputedStyle{GroupID: "g1"}
	anim, ok := parseAnimateElement(elem, gs)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Kind != gui.SvgAnimOpacity {
		t.Fatalf("expected SvgAnimOpacity, got %d", anim.Kind)
	}
	if anim.GroupID != "g1" {
		t.Fatalf("expected GroupID 'g1', got %q", anim.GroupID)
	}
	if len(anim.Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(anim.Values))
	}
	if f32Abs(anim.DurSec-2) > 1e-5 {
		t.Fatalf("expected dur=2, got %f", anim.DurSec)
	}
	if f32Abs(anim.BeginSec-0.5) > 1e-5 {
		t.Fatalf("expected begin=0.5, got %f", anim.BeginSec)
	}
}

func TestAnimationParseAnimateElementNonOpacity(t *testing.T) {
	elem := `<animate attributeName="fill" values="red;blue" dur="1s">`
	_, ok := parseAnimateElement(elem, ComputedStyle{})
	if ok {
		t.Fatalf("expected ok=false for non-opacity")
	}
}

func TestAnimationParseAnimateElementNoValues(t *testing.T) {
	elem := `<animate attributeName="opacity" dur="1s">`
	_, ok := parseAnimateElement(elem, ComputedStyle{})
	if ok {
		t.Fatalf("expected ok=false for missing values")
	}
}

func TestAnimationParseAnimateElementZeroDur(t *testing.T) {
	elem := `<animate attributeName="opacity" values="1;0" dur="0s">`
	_, ok := parseAnimateElement(elem, ComputedStyle{})
	if ok {
		t.Fatalf("expected ok=false for zero duration")
	}
}

func TestAnimationParseCalcModeDiscrete(t *testing.T) {
	elem := `<animate attributeName="opacity" values="1;0;1" ` +
		`dur="1s" calcMode="discrete">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.CalcMode != gui.SvgAnimCalcDiscrete {
		t.Fatalf("expected discrete, got %d", anim.CalcMode)
	}
}

func TestAnimationParseCalcModeSpline(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" dur="1s" ` +
		`calcMode="spline" keySplines="0 0 1 1">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.CalcMode != gui.SvgAnimCalcSpline {
		t.Fatalf("expected spline, got %d", anim.CalcMode)
	}
}

func TestAnimationParseCalcModeLinearDefault(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" dur="1s">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.CalcMode != gui.SvgAnimCalcLinear {
		t.Fatalf("expected linear default, got %d", anim.CalcMode)
	}
}

func TestAnimationParseKeyTimesValid(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;0.5;1" ` +
		`dur="1s" keyTimes="0;.2;1">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(anim.KeyTimes) != 3 {
		t.Fatalf("expected 3 keyTimes, got %v", anim.KeyTimes)
	}
	if anim.KeyTimes[0] != 0 || f32Abs(anim.KeyTimes[1]-0.2) > 1e-5 ||
		anim.KeyTimes[2] != 1 {
		t.Fatalf("unexpected keyTimes: %v", anim.KeyTimes)
	}
}

func TestAnimationParseKeyTimesLengthMismatchDropped(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;0.5;1" ` +
		`dur="1s" keyTimes="0;1">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.KeyTimes != nil {
		t.Fatalf("expected nil keyTimes on length mismatch, got %v",
			anim.KeyTimes)
	}
}

func TestAnimationParseKeyTimesNonMonotonicDropped(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;0.5;1" ` +
		`dur="1s" keyTimes="0;.7;.3">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.KeyTimes != nil {
		t.Fatalf("expected nil keyTimes on non-monotonic, got %v",
			anim.KeyTimes)
	}
}

func TestAnimationParseSetOpacity(t *testing.T) {
	elem := `<set attributeName="opacity" to="0" begin="0.5s"/>`
	anim, ok := parseSetElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Kind != gui.SvgAnimOpacity {
		t.Fatalf("expected SvgAnimOpacity, got %d", anim.Kind)
	}
	if !anim.IsSet {
		t.Fatalf("expected IsSet=true")
	}
	if !anim.Freeze {
		t.Fatalf("expected Freeze=true by default")
	}
	if f32Abs(anim.BeginSec-0.5) > 1e-5 {
		t.Fatalf("expected begin=0.5, got %f", anim.BeginSec)
	}
	if len(anim.Values) != 2 || anim.Values[0] != 0 ||
		anim.Values[1] != 0 {
		t.Fatalf("expected Values=[0,0], got %v", anim.Values)
	}
}

func TestAnimationParseSetAttr(t *testing.T) {
	elem := `<set attributeName="r" to="12" begin="1s"/>`
	anim, ok := parseSetElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Kind != gui.SvgAnimAttr || anim.AttrName != gui.SvgAttrR {
		t.Fatalf("expected SvgAnimAttr/R, got %d/%d",
			anim.Kind, anim.AttrName)
	}
	if anim.Values[0] != 12 {
		t.Fatalf("expected to=12, got %v", anim.Values)
	}
}

func TestAnimationParseSetRejectsBadAttr(t *testing.T) {
	elem := `<set attributeName="fill" to="red"/>`
	_, ok := parseSetElement(elem, ComputedStyle{GroupID: "g"})
	if ok {
		t.Fatalf("expected ok=false for unsupported attr")
	}
}

func TestAnimationParseAnimateMotionInlinePath(t *testing.T) {
	n := testAnimMotionNode(`<animateMotion path="M0,0 L10,0" dur="1s"/>`)
	anim, ok := parseAnimateMotionElement(
		n, ComputedStyle{GroupID: "g"}, &parseState{})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Kind != gui.SvgAnimMotion {
		t.Fatalf("expected SvgAnimMotion, got %d", anim.Kind)
	}
	if len(anim.MotionPath) < 4 {
		t.Fatalf("expected flattened polyline, got %v", anim.MotionPath)
	}
	if anim.MotionLengths[len(anim.MotionLengths)-1] < 9.5 ||
		anim.MotionLengths[len(anim.MotionLengths)-1] > 10.5 {
		t.Fatalf("expected total length ~10, got %v",
			anim.MotionLengths)
	}
}

func TestAnimationParseAnimateMotionRotateAuto(t *testing.T) {
	n := testAnimMotionNode(
		`<animateMotion path="M0,0 L10,0" dur="1s" rotate="auto"/>`)
	anim, ok := parseAnimateMotionElement(
		n, ComputedStyle{GroupID: "g"}, &parseState{})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.MotionRotate != gui.SvgAnimMotionRotateAuto {
		t.Fatalf("expected auto, got %d", anim.MotionRotate)
	}
}

func TestAnimationParseAnimateMotionMpath(t *testing.T) {
	n := testAnimMotionNode(
		`<animateMotion dur="1s"><mpath xlink:href="#p1"/></animateMotion>`)
	state := &parseState{
		defsPaths: map[string]string{"p1": "M0,0 L20,0"},
	}
	anim, ok := parseAnimateMotionElement(
		n, ComputedStyle{GroupID: "g"}, state)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	total := anim.MotionLengths[len(anim.MotionLengths)-1]
	if total < 19.5 || total > 20.5 {
		t.Fatalf("expected total length ~20 from mpath, got %f", total)
	}
}

func TestAnimationParseAccumulateSum(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" ` +
		`dur="1s" accumulate="sum" repeatCount="3">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if !anim.Accumulate {
		t.Fatalf("expected Accumulate=true")
	}
}

func TestAnimationParseRestartNever(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" ` +
		`dur="1s" restart="never">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Restart != gui.SvgAnimRestartNever {
		t.Fatalf("expected never, got %d", anim.Restart)
	}
}

func TestAnimationParseRestartWhenNotActive(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" ` +
		`dur="1s" restart="whenNotActive">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Restart != gui.SvgAnimRestartWhenNotActive {
		t.Fatalf("expected whenNotActive, got %d", anim.Restart)
	}
}

func TestAnimationParseDurationMinClamp(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" ` +
		`dur="5s" min="6s">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if f32Abs(anim.DurSec-6) > 1e-5 {
		t.Fatalf("expected dur clamped to 6, got %f", anim.DurSec)
	}
}

func TestAnimationParseDurationMaxClamp(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" ` +
		`dur="5s" max="2s">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if f32Abs(anim.DurSec-2) > 1e-5 {
		t.Fatalf("expected dur clamped to 2, got %f", anim.DurSec)
	}
}

func TestAnimationParseDurationMinMaxInBand(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" ` +
		`dur="3s" min="1s" max="5s">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if f32Abs(anim.DurSec-3) > 1e-5 {
		t.Fatalf("expected dur unchanged at 3, got %f", anim.DurSec)
	}
}

func TestAnimationParseOpacityFromTo(t *testing.T) {
	elem := `<animate attributeName="opacity" from="1" to="0" dur="1s">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(anim.Values) != 2 ||
		anim.Values[0] != 1 || anim.Values[1] != 0 {
		t.Fatalf("expected Values=[1,0], got %v", anim.Values)
	}
	if anim.Additive {
		t.Fatalf("from/to must not imply additive")
	}
}

func TestAnimationParseOpacityBy(t *testing.T) {
	elem := `<animate attributeName="opacity" by="-0.5" dur="1s">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(anim.Values) != 2 ||
		anim.Values[0] != 0 || anim.Values[1] != -0.5 {
		t.Fatalf("expected Values=[0,-0.5], got %v", anim.Values)
	}
	if !anim.Additive {
		t.Fatalf("by= must imply Additive=true")
	}
}

func TestAnimationParseAttrBy(t *testing.T) {
	elem := `<animate attributeName="r" by="5" dur="1s">`
	anim, ok := parseAnimateAttributeElement(
		elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(anim.Values) != 2 ||
		anim.Values[0] != 0 || anim.Values[1] != 5 {
		t.Fatalf("expected Values=[0,5], got %v", anim.Values)
	}
	if !anim.Additive {
		t.Fatalf("by= must imply Additive=true")
	}
}

func TestAnimationParseRotateBy(t *testing.T) {
	elem := `<animateTransform attributeName="transform" ` +
		`type="rotate" by="90 12 12" dur="1s">`
	anim, ok := parseAnimateTransformElement(
		elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(anim.Values) != 2 ||
		anim.Values[0] != 0 || anim.Values[1] != 90 {
		t.Fatalf("expected angles=[0,90], got %v", anim.Values)
	}
	if anim.CenterX != 12 || anim.CenterY != 12 {
		t.Fatalf("expected center=(12,12), got (%f,%f)",
			anim.CenterX, anim.CenterY)
	}
	if !anim.Additive {
		t.Fatalf("by= must imply Additive=true")
	}
}

func TestAnimationParseTranslateBy(t *testing.T) {
	elem := `<animateTransform attributeName="transform" ` +
		`type="translate" by="10 20" dur="1s">`
	anim, ok := parseAnimateTransformElement(
		elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := []float32{0, 0, 10, 20}
	if len(anim.Values) != 4 {
		t.Fatalf("expected 4 pair floats, got %v", anim.Values)
	}
	for i := range want {
		if anim.Values[i] != want[i] {
			t.Fatalf("want %v, got %v", want, anim.Values)
		}
	}
	if !anim.Additive {
		t.Fatalf("by= must imply Additive=true")
	}
}

func TestAnimationParseAdditiveSumExplicit(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" ` +
		`dur="1s" additive="sum">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if !anim.Additive {
		t.Fatalf("explicit additive=sum must set Additive")
	}
}

func TestAnimationParseSetFillRemove(t *testing.T) {
	elem := `<set attributeName="opacity" to="0.5" fill="remove"/>`
	anim, ok := parseSetElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Freeze {
		t.Fatalf("expected Freeze=false when fill=remove")
	}
}

func TestAnimationParseKeyTimesBadEndpointsDropped(t *testing.T) {
	elem := `<animate attributeName="opacity" values="0;1" ` +
		`dur="1s" keyTimes="0.1;1">`
	anim, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.KeyTimes != nil {
		t.Fatalf("expected nil keyTimes when first != 0, got %v",
			anim.KeyTimes)
	}
}

// --- parseAnimateTransformElement ---

func TestAnimationParseAnimateTransformValid(t *testing.T) {
	elem := `<animateTransform type="rotate" from="0 50 50" to="360 50 50" dur="3s">`
	gs := ComputedStyle{GroupID: "wheel"}
	anim, ok := parseAnimateTransformElement(elem, gs)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Kind != gui.SvgAnimRotate {
		t.Fatalf("expected SvgAnimRotate, got %d", anim.Kind)
	}
	if f32Abs(anim.Values[0]) > 1e-5 || f32Abs(anim.Values[1]-360) > 1e-5 {
		t.Fatalf("expected from=0 to=360, got %v", anim.Values)
	}
	if f32Abs(anim.CenterX-50) > 1e-5 || f32Abs(anim.CenterY-50) > 1e-5 {
		t.Fatalf("expected center (50,50), got (%f,%f)", anim.CenterX, anim.CenterY)
	}
}

func TestAnimationParseAnimateTransformUnknownType(t *testing.T) {
	elem := `<animateTransform type="skewX" from="0" to="30" dur="1s">`
	_, ok := parseAnimateTransformElement(elem, ComputedStyle{})
	if ok {
		t.Fatalf("expected ok=false for unsupported transform type")
	}
}

func TestAnimationParseAnimateTransformValuesForm(t *testing.T) {
	elem := `<animateTransform attributeName="transform" type="rotate" ` +
		`dur="0.75s" values="0 12 12;360 12 12" repeatCount="indefinite"/>`
	gs := ComputedStyle{GroupID: "ring"}
	anim, ok := parseAnimateTransformElement(elem, gs)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Kind != gui.SvgAnimRotate {
		t.Fatalf("expected SvgAnimRotate, got %d", anim.Kind)
	}
	if len(anim.Values) != 2 {
		t.Fatalf("expected 2 angles, got %d", len(anim.Values))
	}
	if f32Abs(anim.Values[0]) > 1e-5 || f32Abs(anim.Values[1]-360) > 1e-5 {
		t.Fatalf("expected [0,360], got %v", anim.Values)
	}
	if f32Abs(anim.CenterX-12) > 1e-5 || f32Abs(anim.CenterY-12) > 1e-5 {
		t.Fatalf("expected center (12,12), got (%f,%f)",
			anim.CenterX, anim.CenterY)
	}
	if f32Abs(anim.DurSec-0.75) > 1e-5 {
		t.Fatalf("expected dur=0.75, got %f", anim.DurSec)
	}
}

func TestAnimationParseAnimateTransformValuesMulti(t *testing.T) {
	elem := `<animateTransform type="rotate" dur="1s" ` +
		`values="0 5 5;90 5 5;180 5 5;360 5 5"/>`
	anim, ok := parseAnimateTransformElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(anim.Values) != 4 {
		t.Fatalf("expected 4 angles, got %d", len(anim.Values))
	}
	if anim.Values[0] != 0 || anim.Values[3] != 360 {
		t.Fatalf("expected [0,...,360], got %v", anim.Values)
	}
}

func TestAnimationParseAnimateTransformValuesSinglePoint(t *testing.T) {
	elem := `<animateTransform type="rotate" dur="1s" values="0 5 5"/>`
	_, ok := parseAnimateTransformElement(elem, ComputedStyle{})
	if ok {
		t.Fatalf("expected ok=false for single-keyframe values")
	}
}

// --- End-to-end parse of a rotate-via-values asset ---

const rotateValuesAsset = `<svg viewBox="0 0 24 24" ` +
	`xmlns="http://www.w3.org/2000/svg">` +
	`<path id="ring" d="M10,10 L14,10 L14,14 L10,14Z">` +
	`<animateTransform attributeName="transform" type="rotate" ` +
	`dur="0.75s" values="0 12 12;360 12 12" repeatCount="indefinite"/>` +
	`</path></svg>`

func TestAnimationRotateValuesEndToEnd(t *testing.T) {
	parsed, err := New().ParseSvg(rotateValuesAsset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Animations) == 0 {
		t.Fatal("expected at least one parsed animation")
	}
	anim := parsed.Animations[0]
	if anim.Kind != gui.SvgAnimRotate {
		t.Fatalf("expected SvgAnimRotate, got %d", anim.Kind)
	}
	if anim.GroupID != "ring" {
		t.Fatalf("expected GroupID 'ring', got %q", anim.GroupID)
	}
	if len(anim.Values) != 2 ||
		anim.Values[0] != 0 || anim.Values[1] != 360 {
		t.Fatalf("expected [0,360], got %v", anim.Values)
	}
	if anim.CenterX != 12 || anim.CenterY != 12 {
		t.Fatalf("expected center (12,12), got (%f,%f)",
			anim.CenterX, anim.CenterY)
	}
	if len(parsed.Paths) == 0 {
		t.Fatal("expected at least one path")
	}
	pid := parsed.Paths[0].PathID
	if pid == 0 || !slices.Contains(anim.TargetPathIDs, pid) {
		t.Fatalf("path PathID %d not in anim.TargetPathIDs %v — "+
			"animation would not bind to the shape",
			pid, anim.TargetPathIDs)
	}
}

// rotateNoIDAsset covers the synthetic-ID branch: a shape with
// inline animation but no id= attribute.
const rotateNoIDAsset = `<svg viewBox="0 0 24 24" ` +
	`xmlns="http://www.w3.org/2000/svg">` +
	`<path d="M10,10 L14,10 L14,14 L10,14Z">` +
	`<animateTransform attributeName="transform" type="rotate" ` +
	`dur="0.75s" values="0 12 12;360 12 12" repeatCount="indefinite"/>` +
	`</path></svg>`

// TestAnimationGroupSynthIDBindsSiblings — when a bare <g> wraps
// multiple shapes plus a shared <animateTransform>, the group
// must get a synthesized GroupID so every descendant shape and
// the animation all share it. Previously the animation and
// paths all had GroupID="" and the renderer's non-empty guard
// silently dropped the animation (e.g. 8-dots-rotate asset).
func TestAnimationGroupSynthIDBindsSiblings(t *testing.T) {
	asset := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">` +
		`<g>` +
		`<circle cx="3" cy="12" r="2"/>` +
		`<circle cx="21" cy="12" r="2"/>` +
		`<animateTransform attributeName="transform" type="rotate" ` +
		`dur="1.5s" values="0 12 12;360 12 12" repeatCount="indefinite"/>` +
		`</g></svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Paths) != 2 || len(parsed.Animations) != 1 {
		t.Fatalf("want 2 paths + 1 anim, got %d paths + %d anims",
			len(parsed.Paths), len(parsed.Animations))
	}
	targets := parsed.Animations[0].TargetPathIDs
	if len(targets) != len(parsed.Paths) {
		t.Fatalf("anim.TargetPathIDs len=%d, want %d",
			len(targets), len(parsed.Paths))
	}
	for i, p := range parsed.Paths {
		if p.PathID == 0 || !slices.Contains(targets, p.PathID) {
			t.Fatalf("path[%d] PathID %d not in TargetPathIDs %v",
				i, p.PathID, targets)
		}
	}
}

func TestAnimationSynthIDCouplesShapeAndAnim(t *testing.T) {
	parsed, err := New().ParseSvg(rotateNoIDAsset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Paths) == 0 || len(parsed.Animations) == 0 {
		t.Fatal("expected at least one path and one animation")
	}
	pid := parsed.Paths[0].PathID
	targets := parsed.Animations[0].TargetPathIDs
	if pid == 0 {
		t.Fatal("PathID should be non-zero on the path")
	}
	if !slices.Contains(targets, pid) {
		t.Fatalf("path PathID %d not in TargetPathIDs %v — "+
			"animation would miss",
			pid, targets)
	}
}

// --- maxKeyframes DoS caps ---

// TestParseSemicolonFloatsCapsAtMaxKeyframes — a pathological
// input with more than maxKeyframes entries is truncated so
// allocation stays bounded.
func TestParseSemicolonFloatsCapsAtMaxKeyframes(t *testing.T) {
	s := strings.Repeat("1;", maxKeyframes+100)
	got := parseSemicolonFloats(s)
	if len(got) > maxKeyframes {
		t.Fatalf("want len<=%d, got %d", maxKeyframes, len(got))
	}
}

// TestParseRotateValuesCapsAtMaxKeyframes — oversized rotate
// values list is truncated; remaining angles still valid.
func TestParseRotateValuesCapsAtMaxKeyframes(t *testing.T) {
	s := strings.Repeat("0 12 12;", maxKeyframes+50)
	angles, _, _, ok := parseRotateValues(s)
	if !ok {
		t.Fatal("parse failed on truncated-but-valid input")
	}
	if len(angles) > maxKeyframes {
		t.Fatalf("want len<=%d, got %d", maxKeyframes, len(angles))
	}
}

// --- parseFreeze ---

func TestParseFreeze_RecognizesOnlyFreeze(t *testing.T) {
	if !parseFreeze(`<animate fill="freeze">`) {
		t.Fatal("fill=freeze should be recognized")
	}
	if parseFreeze(`<animate fill="remove">`) {
		t.Fatal("fill=remove should be false")
	}
	if parseFreeze(`<animate>`) {
		t.Fatal("missing fill attr should be false")
	}
	if parseFreeze(`<animate fill="FREEZE">`) {
		t.Fatal("case-sensitive: FREEZE must not match")
	}
}

// --- parseRepeatCycle ---

func TestParseRepeatCycle_IndefiniteReturnsDur(t *testing.T) {
	// repeatCount="indefinite" yields dur.
	c := parseRepeatCycle(`<animate repeatCount="indefinite">`, 2.5)
	if f32Abs(c-2.5) > 1e-5 {
		t.Fatalf("expected cycle=2.5, got %f", c)
	}
	// repeatDur="indefinite" yields dur.
	c = parseRepeatCycle(`<animate repeatDur="indefinite">`, 1.0)
	if f32Abs(c-1.0) > 1e-5 {
		t.Fatalf("expected cycle=1.0, got %f", c)
	}
	// No repeat attrs → 0 (play once).
	if parseRepeatCycle(`<animate>`, 1.0) != 0 {
		t.Fatal("no repeat attr should yield cycle=0")
	}
}

func TestParseRepeatCycle_FiniteCountProducesDurTimesN(t *testing.T) {
	c := parseRepeatCycle(`<animate repeatCount="3">`, 2)
	if f32Abs(c-6) > 1e-5 {
		t.Fatalf("expected cycle=6, got %f", c)
	}
}

func TestParseRepeatCycle_RepeatDurProducesExplicitSeconds(t *testing.T) {
	c := parseRepeatCycle(`<animate repeatDur="4s">`, 2)
	if f32Abs(c-4) > 1e-5 {
		t.Fatalf("expected cycle=4, got %f", c)
	}
}

func TestParseRepeatCycle_ClampsHostileCount(t *testing.T) {
	// Hostile repeatCount is clamped so dur*n stays finite and
	// below maxCycleSec. Authoring mistakes or malicious SVGs
	// cannot poison downstream timing math with +Inf.
	c := parseRepeatCycle(`<animate repeatCount="1e30">`, 1)
	if c <= 0 || c > maxCycleSec {
		t.Fatalf("expected clamp into (0, %f], got %f", maxCycleSec, c)
	}
	c = parseRepeatCycle(`<animate repeatDur="1e30s">`, 1)
	if c <= 0 || c > maxCycleSec {
		t.Fatalf("expected clamp into (0, %f], got %f", maxCycleSec, c)
	}
}

// --- clampCycle ---

func TestClampCycle_BoundsAtMaxAndFloor(t *testing.T) {
	if clampCycle(0) != 0 {
		t.Fatal("0 should pass through as 0")
	}
	if clampCycle(-5) != 0 {
		t.Fatal("negative should clamp to 0")
	}
	if clampCycle(maxCycleSec+1) != maxCycleSec {
		t.Fatalf("above max should clamp to %f", maxCycleSec)
	}
	if clampCycle(10) != 10 {
		t.Fatal("in-range should pass through")
	}
}

// --- Opacity sub-target detection ---

func TestParseAnimateElement_FillOpacityTargetsFill(t *testing.T) {
	elem := `<animate attributeName="fill-opacity" values="1;0" dur="1s">`
	a, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("expected ok=true for fill-opacity")
	}
	if a.Target != gui.SvgAnimTargetFill {
		t.Fatalf("expected Target=Fill, got %d", a.Target)
	}
	if a.Kind != gui.SvgAnimOpacity {
		t.Fatalf("expected Kind=Opacity, got %d", a.Kind)
	}
}

func TestParseAnimateElement_StrokeOpacityTargetsStroke(t *testing.T) {
	elem := `<animate attributeName="stroke-opacity" values="1;0" dur="1s">`
	a, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("expected ok=true for stroke-opacity")
	}
	if a.Target != gui.SvgAnimTargetStroke {
		t.Fatalf("expected Target=Stroke, got %d", a.Target)
	}
}

func TestParseAnimateElement_OpacityTargetsAll(t *testing.T) {
	elem := `<animate attributeName="opacity" values="1;0" dur="1s">`
	a, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if a.Target != gui.SvgAnimTargetAll {
		t.Fatalf("expected Target=All, got %d", a.Target)
	}
}

// --- Rotate center transform baking ---

func TestParseAnimateTransform_RotateCenterUsesInheritedTransform(t *testing.T) {
	// Parent scale(2) translate(10,20) must fold into the rotate
	// pivot so the animated rotation pivots in absolute SVG space.
	inh := ComputedStyle{
		GroupID:   "g",
		Transform: [6]float32{2, 0, 0, 2, 10, 20},
	}
	elem := `<animateTransform type="rotate" dur="1s" values="0 5 5;360 5 5"/>`
	a, ok := parseAnimateTransformElement(elem, inh)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// applyTransformPt on (5,5) with the matrix yields (20,30).
	if f32Abs(a.CenterX-20) > 1e-5 || f32Abs(a.CenterY-30) > 1e-5 {
		t.Fatalf("expected center (20,30), got (%f,%f)",
			a.CenterX, a.CenterY)
	}
}

// Translate / scale pair values MUST remain in local space.
// Baking the ancestor transform here would apply it twice at
// render time.
func TestPairedTransform_TranslateValuesIgnoreInheritedTransform(t *testing.T) {
	inh := ComputedStyle{
		GroupID:   "g",
		Transform: [6]float32{2, 0, 0, 2, 10, 20},
	}
	elem := `<animateTransform type="translate" dur="1s" values="0 0;30 40"/>`
	a, ok := parsePairedAnimateTransform(elem, inh, gui.SvgAnimTranslate)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Values must equal the raw input, not (10,20;70,100).
	want := []float32{0, 0, 30, 40}
	if len(a.Values) != len(want) {
		t.Fatalf("len mismatch: %v", a.Values)
	}
	for i, v := range want {
		if f32Abs(a.Values[i]-v) > 1e-5 {
			t.Fatalf("values[%d]=%f, want %f (inherited transform "+
				"must not be applied)", i, a.Values[i], v)
		}
	}
}

// --- Cycle propagation through resolveBegins ---

// When multiple animations share a syncbase chain, resolveBegins
// must propagate the largest derived cycle to chain participants
// whose own Cycle is 0. Without this, a freeze-chained sequence
// stalls after the first loop.
func TestResolveBegins_PropagatesGlobalCycle(t *testing.T) {
	// Construct a chain of two animations in a group. The first
	// sets BeginSec=0 and repeatCount="indefinite" so it derives a
	// Cycle from parseRepeatCycle. The second has BeginSec=0 but
	// no cycle; it must inherit the global cycle since its begin
	// spec references the first.
	asset := `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg">` +
		`<rect id="box" x="0" y="0" width="10" height="10">` +
		`<animate id="a1" attributeName="opacity" values="1;0" ` +
		`dur="1s" begin="0s" repeatCount="indefinite"/>` +
		`<animate id="a2" attributeName="opacity" values="0;1" ` +
		`dur="1s" begin="a1.end"/>` +
		`</rect></svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Animations) != 2 {
		t.Fatalf("expected 2 animations, got %d", len(parsed.Animations))
	}
	// Both must carry a positive cycle; second inherits from first.
	for i, a := range parsed.Animations {
		if a.Cycle <= 0 {
			t.Fatalf("anim[%d] expected Cycle>0, got %f", i, a.Cycle)
		}
	}
}

// --- Hardening and edge-case coverage ---

// flattenMotionD truncates the polyline at maxMotionVertices so a
// pathological path cannot drive unbounded per-frame arc scans.
func TestFlattenMotionD_CapsAtMaxVertices(t *testing.T) {
	// Build a path with > maxMotionVertices L segments.
	var b strings.Builder
	b.WriteString("M0,0")
	for i := 1; i <= maxMotionVertices+200; i++ {
		b.WriteString(" L")
		// One unit apart on x-axis.
		b.WriteString(strconv.Itoa(i))
		b.WriteString(",0")
	}
	poly, lens := flattenMotionD(b.String())
	if len(poly)/2 > maxMotionVertices {
		t.Fatalf("polyline not capped: got %d vertices, cap %d",
			len(poly)/2, maxMotionVertices)
	}
	if len(lens) != len(poly)/2 {
		t.Fatalf("lens/poly length mismatch: %d vs %d",
			len(lens), len(poly)/2)
	}
}

// <set attributeName="fill-opacity"> must route to Target=Fill so
// only the fill channel receives the value at render time.
func TestAnimationParseSet_FillOpacityTargetsFill(t *testing.T) {
	elem := `<set attributeName="fill-opacity" to="0.25">`
	anim, ok := parseSetElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Kind != gui.SvgAnimOpacity {
		t.Fatalf("expected SvgAnimOpacity, got %d", anim.Kind)
	}
	if anim.Target != gui.SvgAnimTargetFill {
		t.Fatalf("expected TargetFill, got %d", anim.Target)
	}
}

// <set attributeName="stroke-opacity"> must route to Target=Stroke.
func TestAnimationParseSet_StrokeOpacityTargetsStroke(t *testing.T) {
	elem := `<set attributeName="stroke-opacity" to="0.75">`
	anim, ok := parseSetElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.Target != gui.SvgAnimTargetStroke {
		t.Fatalf("expected TargetStroke, got %d", anim.Target)
	}
}

// animateTransform with to= but no from= must imply additive=true
// so the animation sums onto the base transform at apply time.
func TestAnimationParsePairedTransform_ToOnlyImpliesAdditive(t *testing.T) {
	elem := `<animateTransform attributeName="transform" ` +
		`type="translate" to="5 7" dur="1s">`
	anim, ok := parseAnimateTransformElement(
		elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if !anim.Additive {
		t.Fatalf("to= without from= must imply Additive=true")
	}
	want := []float32{0, 0, 5, 7}
	if len(anim.Values) != 4 {
		t.Fatalf("expected 4 pair floats, got %v", anim.Values)
	}
	for i := range want {
		if anim.Values[i] != want[i] {
			t.Fatalf("want %v, got %v", want, anim.Values)
		}
	}
}

// rotate="auto-reverse" and unrecognized values map to AutoReverse
// and None respectively.
func TestAnimationParseMotionRotate_AutoReverseAndUnknown(t *testing.T) {
	revN := testAnimMotionNode(`<animateMotion path="M0,0 L10,0" dur="1s" ` +
		`rotate="auto-reverse"/>`)
	anim, ok := parseAnimateMotionElement(
		revN, ComputedStyle{GroupID: "g"}, &parseState{})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if anim.MotionRotate != gui.SvgAnimMotionRotateAutoReverse {
		t.Fatalf("expected AutoReverse, got %d", anim.MotionRotate)
	}

	unkN := testAnimMotionNode(
		`<animateMotion path="M0,0 L10,0" dur="1s" rotate="45"/>`)
	anim2, ok := parseAnimateMotionElement(
		unkN, ComputedStyle{GroupID: "g"}, &parseState{})
	if !ok {
		t.Fatalf("unknown-rotate: expected ok=true")
	}
	if anim2.MotionRotate != gui.SvgAnimMotionRotateNone {
		t.Fatalf("unknown rotate= must fall through to None, got %d",
			anim2.MotionRotate)
	}
}

// <mpath href="#id"> (non-xlink) must resolve against defsPaths.
func TestAnimationMotionPathD_BareHrefResolves(t *testing.T) {
	n := testAnimMotionNode(
		`<animateMotion dur="1s"><mpath href="#p2"/></animateMotion>`)
	state := &parseState{
		defsPaths: map[string]string{"p2": "M0,0 L15,0"},
	}
	anim, ok := parseAnimateMotionElement(
		n, ComputedStyle{GroupID: "g"}, state)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	total := anim.MotionLengths[len(anim.MotionLengths)-1]
	if total < 14.5 || total > 15.5 {
		t.Fatalf("expected ~15, got %f", total)
	}
}

// parseDashFrameFloats truncates fields exceeding cap+1 to bound
// allocation against hostile inputs with millions of fields.
func TestParseDashFrameFloats_TruncatesHostileWidth(t *testing.T) {
	// cap=8 → truncate at 9 fields. Build 100-field string.
	parts := make([]string, 100)
	for i := range parts {
		parts[i] = "1"
	}
	got := parseDashFrameFloats(strings.Join(parts, " "))
	if len(got) > gui.SvgAnimDashArrayCap+1 {
		t.Fatalf("len=%d should be ≤cap+1=%d",
			len(got), gui.SvgAnimDashArrayCap+1)
	}
}

// Empty input → nil. Whitespace-only treated identically.
func TestParseDashFrameFloats_EmptyReturnsNil(t *testing.T) {
	if got := parseDashFrameFloats(""); got != nil {
		t.Fatalf("empty: want nil, got %v", got)
	}
	if got := parseDashFrameFloats("   "); got != nil {
		t.Fatalf("ws: want nil, got %v", got)
	}
}

// Comma-separated values parse the same as whitespace-separated.
func TestParseDashFrameFloats_CommaSeparated(t *testing.T) {
	got := parseDashFrameFloats("3,5,7")
	if len(got) != 3 || got[0] != 3 || got[1] != 5 || got[2] != 7 {
		t.Fatalf("got %v want [3 5 7]", got)
	}
}

// repeatCount=indefinite must clamp the cycle to maxCycleSec when
// dur itself is huge. Guards against pathological assets producing
// runaway anim durations.
func TestParseRepeatCycle_IndefiniteClamped(t *testing.T) {
	elem := `<animate dur="200000s" repeatCount="indefinite"/>`
	dur := parseDuration(elem)
	cycle := parseRepeatCycle(elem, dur)
	if cycle != maxCycleSec {
		t.Fatalf("indefinite cycle=%v want clamp to %v",
			cycle, maxCycleSec)
	}
}

// repeatCount with a finite count whose product overflows the
// per-cycle clamp must clamp.
func TestParseRepeatCycle_LargeCountClamped(t *testing.T) {
	elem := `<animate dur="1s" repeatCount="999999999"/>`
	cycle := parseRepeatCycle(elem, 1)
	if cycle != maxCycleSec {
		t.Fatalf("cycle=%v want %v", cycle, maxCycleSec)
	}
}

// repeatDur=indefinite is equivalent to repeatCount=indefinite for
// clamping purposes.
func TestParseRepeatCycle_RepeatDurIndefiniteClamped(t *testing.T) {
	elem := `<animate dur="200000s" repeatDur="indefinite"/>`
	dur := parseDuration(elem)
	cycle := parseRepeatCycle(elem, dur)
	if cycle != maxCycleSec {
		t.Fatalf("cycle=%v want %v", cycle, maxCycleSec)
	}
}
