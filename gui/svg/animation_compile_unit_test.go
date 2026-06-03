package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

func TestNormalizeKeyTimes_Valid(t *testing.T) {
	in := []float32{0, 0.3, 0.7, 1}
	got := normalizeKeyTimes(in)
	if len(got) != len(in) {
		t.Fatalf("len=%d want %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("[%d] got %v want %v", i, got[i], in[i])
		}
	}
}

func TestNormalizeKeyTimes_RejectMissingZero(t *testing.T) {
	if normalizeKeyTimes([]float32{0.1, 0.5, 1}) != nil {
		t.Error("expected nil when first != 0")
	}
}

func TestNormalizeKeyTimes_RejectMissingOne(t *testing.T) {
	if normalizeKeyTimes([]float32{0, 0.5, 0.9}) != nil {
		t.Error("expected nil when last != 1")
	}
}

func TestNormalizeKeyTimes_RejectNonMonotonic(t *testing.T) {
	if normalizeKeyTimes([]float32{0, 0.6, 0.5, 1}) != nil {
		t.Error("expected nil for descending segment")
	}
}

func TestNormalizeKeyTimes_RejectShort(t *testing.T) {
	if normalizeKeyTimes([]float32{1}) != nil {
		t.Error("len<2 should return nil")
	}
}

func TestNormalizeKeyTimes_AllowsRepeatedValues(t *testing.T) {
	// Equal adjacent values are monotonic non-decreasing — accepted.
	got := normalizeKeyTimes([]float32{0, 0.5, 0.5, 1})
	if got == nil {
		t.Error("equal adjacent values must be accepted")
	}
}

func TestNormalizeKeyTimes_CopiesInput(t *testing.T) {
	in := []float32{0, 0.5, 1}
	out := normalizeKeyTimes(in)
	if &in[0] == &out[0] {
		t.Error("expected a copy, not the same backing array")
	}
}

func TestPackRGBARoundtrip(t *testing.T) {
	c := gui.SvgColor{R: 0x12, G: 0x34, B: 0x56, A: 0x78}
	if packRGBA(c) != 0x12345678 {
		t.Errorf("packRGBA=%#x want 0x12345678", packRGBA(c))
	}
}

func TestParseTransformFunctions_RotateTranslateScale(t *testing.T) {
	got := parseTransformFunctions("rotate(45) translate(10, 20) scale(2)")
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].name != "rotate" || got[0].args[0] != 45 {
		t.Errorf("rotate: %+v", got[0])
	}
	if got[1].name != "translate" ||
		got[1].args[0] != 10 || got[1].args[1] != 20 {
		t.Errorf("translate: %+v", got[1])
	}
	if got[2].name != "scale" || got[2].args[0] != 2 {
		t.Errorf("scale: %+v", got[2])
	}
}

func TestParseTransformFunctions_LowercasesName(t *testing.T) {
	got := parseTransformFunctions("ROTATE(90)")
	if len(got) != 1 || got[0].name != "rotate" {
		t.Errorf("got %+v want rotate", got)
	}
}

func TestParseTransformFunctions_UnclosedParenStops(t *testing.T) {
	got := parseTransformFunctions("rotate(45 translate(10 20)")
	// Unbalanced: parser breaks rather than panic. Pin the contract.
	if len(got) > 1 {
		t.Errorf("expected at most one func from unclosed input; got %+v",
			got)
	}
}

func TestParseTransformFunctions_EmptyArgs(t *testing.T) {
	got := parseTransformFunctions("matrix()")
	if len(got) != 1 || got[0].name != "matrix" {
		t.Fatalf("got %+v want matrix()", got)
	}
	if len(got[0].args) != 0 {
		t.Errorf("args=%v want empty", got[0].args)
	}
}

func TestReverseTimeline_ScalarValues(t *testing.T) {
	a := &gui.SvgAnimation{
		Kind:   gui.SvgAnimOpacity,
		Values: []float32{0, 0.5, 1},
	}
	reverseTimeline(a)
	want := []float32{1, 0.5, 0}
	for i, v := range a.Values {
		if v != want[i] {
			t.Errorf("Values[%d]=%v want %v", i, v, want[i])
		}
	}
}

func TestReverseTimeline_PairedTranslate(t *testing.T) {
	a := &gui.SvgAnimation{
		Kind:   gui.SvgAnimTranslate,
		Values: []float32{0, 0, 10, 20, 30, 40},
	}
	reverseTimeline(a)
	want := []float32{30, 40, 10, 20, 0, 0}
	for i, v := range a.Values {
		if v != want[i] {
			t.Errorf("Values[%d]=%v want %v", i, v, want[i])
		}
	}
}

func TestReverseTimeline_ColorKeyframes(t *testing.T) {
	a := &gui.SvgAnimation{
		Kind:        gui.SvgAnimColor,
		ColorValues: []uint32{0xAA, 0xBB, 0xCC},
	}
	reverseTimeline(a)
	want := []uint32{0xCC, 0xBB, 0xAA}
	for i, v := range a.ColorValues {
		if v != want[i] {
			t.Errorf("[%d]=%#x want %#x", i, v, want[i])
		}
	}
}

func TestTransformIdentityFor_Translate3dZeroes(t *testing.T) {
	got := transformIdentityFor([]cssTxFunc{{name: "translate3d"}})
	want := "translate3d(0,0,0)"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTransformIdentityFor_MixedListIncludesTranslate3d(t *testing.T) {
	got := transformIdentityFor([]cssTxFunc{
		{name: "rotate"}, {name: "translate3d"}, {name: "scale"},
	})
	want := "rotate(0) translate3d(0,0,0) scale(1,1)"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestCompileTransformTimeline_Translate3dEmitsXY(t *testing.T) {
	def := &css.KeyframesDef{
		Name: "slide",
		Stops: []css.KeyframeStop{
			{Offset: 0, Decls: []css.Decl{
				{Name: "transform", Value: "translate3d(0,0,0)"},
			}},
			{Offset: 1, Decls: []css.Decl{
				{Name: "transform", Value: "translate3d(10,20,5)"},
			}},
		},
	}
	state := &parseState{}
	spec := cssAnimSpec{
		Name: "slide", DurationSec: 1,
		IterCount: 1, IterCountSet: true,
	}
	added := compileTransformTimeline(def, spec, 42, 0, 0, state)
	if added != 1 {
		t.Fatalf("added=%d want 1", added)
	}
	a := state.animations[0]
	if a.Kind != gui.SvgAnimTranslate {
		t.Errorf("Kind=%v want SvgAnimTranslate", a.Kind)
	}
	want := []float32{0, 0, 10, 20}
	if len(a.Values) != len(want) {
		t.Fatalf("Values=%v want %v", a.Values, want)
	}
	for i := range want {
		if a.Values[i] != want[i] {
			t.Errorf("Values[%d]=%v want %v", i, a.Values[i], want[i])
		}
	}
	if len(a.TargetPathIDs) != 1 || a.TargetPathIDs[0] != 42 {
		t.Errorf("TargetPathIDs=%v want [42]", a.TargetPathIDs)
	}
}

func TestReverseTimeline_KeyTimesComplemented(t *testing.T) {
	a := &gui.SvgAnimation{
		Kind:     gui.SvgAnimOpacity,
		Values:   []float32{0, 1},
		KeyTimes: []float32{0, 0.25, 1},
	}
	reverseTimeline(a)
	want := []float32{0, 0.75, 1}
	for i, v := range a.KeyTimes {
		// float32 round-trip: 1 - 0.25 == 0.75 exactly here.
		if v != want[i] {
			t.Errorf("KeyTimes[%d]=%v want %v", i, v, want[i])
		}
	}
}
