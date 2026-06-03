package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestApplyCSSProp_OpacityPercent(t *testing.T) {
	out := ComputedStyle{}
	applyCSSProp("opacity", "50%", &out)
	if out.Opacity != 0.5 {
		t.Errorf("opacity 50%%: got %v want 0.5", out.Opacity)
	}
}

func TestApplyCSSProp_FillOpacityPercent(t *testing.T) {
	out := ComputedStyle{}
	applyCSSProp("fill-opacity", "25%", &out)
	if out.FillOpacity != 0.25 {
		t.Errorf("fill-opacity 25%%: got %v", out.FillOpacity)
	}
}

func TestApplyCSSProp_StrokeOpacityPercent(t *testing.T) {
	out := ComputedStyle{}
	applyCSSProp("stroke-opacity", "100%", &out)
	if out.StrokeOpacity != 1 {
		t.Errorf("stroke-opacity 100%%: got %v", out.StrokeOpacity)
	}
}

func TestApplyCSSProp_OpacityUnitlessUnchanged(t *testing.T) {
	out := ComputedStyle{}
	applyCSSProp("opacity", "0.4", &out)
	if out.Opacity != 0.4 {
		t.Errorf("opacity 0.4: got %v", out.Opacity)
	}
}

func TestApplyCSSProp_OpacityPercentClamped(t *testing.T) {
	out := ComputedStyle{}
	applyCSSProp("opacity", "150%", &out)
	if out.Opacity != 1 {
		t.Errorf("150%% should clamp to 1: got %v", out.Opacity)
	}
	applyCSSProp("opacity", "-10%", &out)
	if out.Opacity != 0 {
		t.Errorf("-10%% should clamp to 0: got %v", out.Opacity)
	}
}

// Malformed % → parseFloatTrimmed returns NaN → clampOpacity01 maps
// to 0. Without the clamp, the uint8 alpha cast at bake time is
// implementation-defined for NaN.
func TestApplyCSSProp_OpacityMalformedPercent(t *testing.T) {
	out := ComputedStyle{Opacity: 0.7}
	applyCSSProp("opacity", "abc%", &out)
	if out.Opacity != 0 {
		t.Errorf("abc%%: got %v want 0 (clamped NaN)", out.Opacity)
	}
}

// Unrecognized paint syntax must NOT clobber an inherited fill that
// the cascade already resolved. Previously parseSvgColor returned
// zero (transparent black) for unsupported values like hsl(),
// uppercase keywords, or rgb-with-percent; applyCSSProp then set
// FillSet=true with that zero, hiding shapes. Now invalid
// declarations are dropped per CSS "invalid → ignore".
func TestApplyCSSProp_FillUnknownSyntaxPreservesInherited(t *testing.T) {
	prior := gui.SvgColor{R: 10, G: 20, B: 30, A: 255}
	out := ComputedStyle{Fill: prior, FillSet: true}
	applyCSSProp("fill", "hsl(0, 100%, 50%)", &out)
	if !out.FillSet || out.Fill != prior {
		t.Errorf("hsl() should be ignored, fill=%+v set=%v", out.Fill, out.FillSet)
	}
	applyCSSProp("fill", "rgb(100% 0 0 / 50%)", &out)
	if !out.FillSet || out.Fill != prior {
		t.Errorf("modern rgb() should be ignored, fill=%+v set=%v",
			out.Fill, out.FillSet)
	}
	applyCSSProp("fill", "notacolor", &out)
	if !out.FillSet || out.Fill != prior {
		t.Errorf("garbage should be ignored, fill=%+v set=%v",
			out.Fill, out.FillSet)
	}
}

func TestApplyCSSProp_StrokeUnknownSyntaxPreservesInherited(t *testing.T) {
	prior := gui.SvgColor{R: 200, G: 100, B: 50, A: 255}
	out := ComputedStyle{Stroke: prior, StrokeSet: true}
	applyCSSProp("stroke", "hsl(120 50% 50%)", &out)
	if !out.StrokeSet || out.Stroke != prior {
		t.Errorf("hsl() stroke should be ignored, got %+v set=%v",
			out.Stroke, out.StrokeSet)
	}
}

// Case-insensitive keyword matching: "RED" and "Red" should resolve
// to red, not be treated as unknown.
func TestApplyCSSProp_FillKeywordCaseInsensitive(t *testing.T) {
	out := ComputedStyle{}
	applyCSSProp("fill", "RED", &out)
	want := gui.SvgColor{R: 255, A: 255}
	if !out.FillSet || out.Fill != want {
		t.Errorf("'RED' should map to red, got %+v set=%v",
			out.Fill, out.FillSet)
	}
}

// `fill: currentColor` must produce the colorCurrent sentinel so
// render-time tinting can substitute. Earlier review noted this
// path was uncovered: a refactor that swallowed the sentinel would
// silently break themed icons.
func TestApplyCSSProp_FillCurrentColorSetsCurrentSentinel(t *testing.T) {
	out := ComputedStyle{}
	applyCSSProp("fill", "currentColor", &out)
	if !out.FillSet {
		t.Fatal("FillSet must be true after currentColor")
	}
	if out.Fill != colorCurrent {
		t.Errorf("Fill = %+v, want colorCurrent sentinel", out.Fill)
	}
	// Case-insensitive variant.
	out = ComputedStyle{}
	applyCSSProp("fill", "CURRENTCOLOR", &out)
	if out.Fill != colorCurrent {
		t.Errorf("Fill (uppercase) = %+v, want colorCurrent sentinel",
			out.Fill)
	}
}

// stroke-width must run through sanitizeStrokeWidth in the cascade
// so negative / NaN / Inf values clamp to 0 before reaching
// tessellation.
func TestApplyCSSProp_StrokeWidthSanitized(t *testing.T) {
	cases := []struct {
		in   string
		want float32
	}{
		{"3", 3},
		{"-5", 0},
		{"NaN", 0},
		{"+Inf", 0},
	}
	for _, tc := range cases {
		out := ComputedStyle{}
		applyCSSProp("stroke-width", tc.in, &out)
		if out.StrokeWidth != tc.want {
			t.Errorf("stroke-width=%q: got %f want %f",
				tc.in, out.StrokeWidth, tc.want)
		}
	}
}

// isCSSInheritKeyword underpins fill/stroke override-skip and the
// text raw-stroke fallback. Verify all CSS-wide control keywords,
// case-insensitive matching, surrounding whitespace, and rejection
// of non-keyword values.
func TestIsCSSInheritKeyword_AllVariants(t *testing.T) {
	accept := []string{
		"inherit", "INHERIT", "Inherit",
		"unset", "UNSET",
		"revert", "Revert",
		"revert-layer", "REVERT-LAYER",
		"  inherit  ", "\tunset\n",
	}
	for _, in := range accept {
		if !isCSSInheritKeyword(in) {
			t.Errorf("%q must be recognized as CSS-wide keyword", in)
		}
	}
	reject := []string{"", "  ", "red", "currentColor", "inherits",
		"un set", "revert-layers"}
	for _, in := range reject {
		if isCSSInheritKeyword(in) {
			t.Errorf("%q must NOT be recognized", in)
		}
	}
}

func TestSplitVarArgs_EmptyAndNoComma(t *testing.T) {
	cases := []struct {
		in               string
		wantName, wantFB string
		wantHas          bool
	}{
		{"", "", "", false},
		{"--brand", "--brand", "", false},
		{"--brand,red", "--brand", "red", true},
		{"  --brand  ,  red  ", "  --brand  ", "  red  ", true},
	}
	for _, tc := range cases {
		name, fb, has := splitVarArgs(tc.in)
		if name != tc.wantName || fb != tc.wantFB || has != tc.wantHas {
			t.Errorf("splitVarArgs(%q) = (%q,%q,%v), want (%q,%q,%v)",
				tc.in, name, fb, has, tc.wantName, tc.wantFB, tc.wantHas)
		}
	}
}

func TestSplitVarArgs_NestedParens(t *testing.T) {
	// Top-level comma split must not fire inside nested parens.
	in := "--a, var(--b, red)"
	name, fb, has := splitVarArgs(in)
	if !has || name != "--a" || fb != " var(--b, red)" {
		t.Errorf("nested parens split wrong: name=%q fb=%q has=%v",
			name, fb, has)
	}
	// calc() inside fallback.
	in = "--w, calc(10px + 4px)"
	name, fb, has = splitVarArgs(in)
	if !has || name != "--w" || fb != " calc(10px + 4px)" {
		t.Errorf("calc fallback split: name=%q fb=%q has=%v",
			name, fb, has)
	}
}
