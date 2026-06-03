package svg

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestStaticValueFor_StrokeDashOffset(t *testing.T) {
	got := staticValueFor("stroke-dashoffset",
		ComputedStyle{StrokeDashOffset: 12.5})
	if got != "12.5" {
		t.Errorf("got %q want 12.5", got)
	}
}

func TestStaticValueFor_OpacityArms(t *testing.T) {
	c := ComputedStyle{Opacity: 0.5, FillOpacity: 0.25, StrokeOpacity: 1}
	cases := map[string]string{
		"opacity":        "0.5",
		"fill-opacity":   "0.25",
		"stroke-opacity": "1",
	}
	for prop, want := range cases {
		if got := staticValueFor(prop, c); got != want {
			t.Errorf("%s: got %q want %q", prop, got, want)
		}
	}
}

func TestStaticValueFor_FillUnsetReturnsEmpty(t *testing.T) {
	if got := staticValueFor("fill", ComputedStyle{}); got != "" {
		t.Errorf("fill unset: got %q", got)
	}
	if got := staticValueFor("stroke", ComputedStyle{}); got != "" {
		t.Errorf("stroke unset: got %q", got)
	}
}

func TestStaticValueFor_FillSetReturnsRGBA(t *testing.T) {
	c := ComputedStyle{
		FillSet: true,
		Fill:    gui.SvgColor{R: 10, G: 20, B: 30, A: 255},
	}
	got := staticValueFor("fill", c)
	if got != "rgba(10,20,30,1)" {
		t.Errorf("fill: got %q", got)
	}
}

func TestStaticValueFor_StrokeSetReturnsRGBA(t *testing.T) {
	c := ComputedStyle{
		StrokeSet: true,
		Stroke:    gui.SvgColor{R: 0, G: 128, B: 255, A: 128},
	}
	got := staticValueFor("stroke", c)
	if !strings.HasPrefix(got, "rgba(0,128,255,") {
		t.Errorf("stroke prefix: got %q", got)
	}
}

func TestStaticValueFor_UnknownProp(t *testing.T) {
	if got := staticValueFor("font-size", ComputedStyle{}); got != "" {
		t.Errorf("unknown: got %q", got)
	}
}

func TestSvgColorToString_FullAlpha(t *testing.T) {
	got := svgColorToString(gui.SvgColor{R: 255, G: 0, B: 0, A: 255})
	if got != "rgba(255,0,0,1)" {
		t.Errorf("got %q", got)
	}
}

func TestSvgColorToString_HalfAlpha(t *testing.T) {
	got := svgColorToString(gui.SvgColor{R: 1, G: 2, B: 3, A: 128})
	// 128/255 ≈ 0.5019607...; FormatFloat -1 32 gives shortest repr.
	if !strings.HasPrefix(got, "rgba(1,2,3,0.50") {
		t.Errorf("got %q", got)
	}
}

func TestTransformIdentityFor_KnownFunctions(t *testing.T) {
	fns := []cssTxFunc{
		{name: "rotate"},
		{name: "translate"},
		{name: "scale"},
	}
	got := transformIdentityFor(fns)
	want := "rotate(0) translate(0,0) scale(1,1)"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTransformIdentityFor_UnknownFunctionFallback(t *testing.T) {
	fns := []cssTxFunc{{name: "skew"}}
	got := transformIdentityFor(fns)
	if got != "skew()" {
		t.Errorf("got %q", got)
	}
}

func TestPhaseD_TransformImplicitEndpointIdentity(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes spin { to { transform: rotate(360deg) } }
		.x { animation: spin 1s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Animations) == 0 {
		t.Fatalf("expected at least one animation")
	}
	var rot *gui.SvgAnimation
	for i := range vg.Animations {
		if vg.Animations[i].Kind == gui.SvgAnimRotate {
			rot = &vg.Animations[i]
			break
		}
	}
	if rot == nil {
		t.Fatalf("no rotate animation")
	}
	if len(rot.Values) < 2 {
		t.Fatalf("values: %v", rot.Values)
	}
	if rot.Values[0] != 0 {
		t.Errorf("first value: %v want 0 (identity)", rot.Values[0])
	}
	if rot.Values[len(rot.Values)-1] != 360 {
		t.Errorf("last value: %v want 360", rot.Values[len(rot.Values)-1])
	}
}

// Single mid-stop with no static would compile to nothing, but
// stroke-dashoffset defaults to 0 — verify the default-valued static
// pads both endpoints so the timeline still emits.
func TestPhaseD_DashOffset_SingleStopNoStaticDrops(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
		@keyframes dash { 50% { stroke-dashoffset: 10 } }
		.x { stroke: black; stroke-dasharray: 5; animation: dash 1s }
		</style>
		<rect class="x" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	for _, a := range vg.Animations {
		if a.Kind == gui.SvgAnimDashOffset {
			// StrokeDashOffset defaults to 0 → static = "0" → padding
			// runs and timeline compiles. Expect 3 values: 0, 10, 0.
			if len(a.Values) != 3 {
				t.Fatalf("values: %v", a.Values)
			}
			if a.Values[0] != 0 || a.Values[1] != 10 || a.Values[2] != 0 {
				t.Errorf("values: %v want [0 10 0]", a.Values)
			}
			return
		}
	}
	t.Fatalf("no dash-offset animation emitted")
}
