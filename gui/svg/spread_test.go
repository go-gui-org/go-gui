package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func mkLinear() gui.SvgGradientDef {
	return gui.SvgGradientDef{
		X1: 0, Y1: 0, X2: 1, Y2: 0,
	}
}

func almostEq(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func TestApplySpreadPadClamps(t *testing.T) {
	g := mkLinear()
	g.SpreadMethod = gui.SvgSpreadPad
	if got := projectAndSpread(-0.5, 0, g); got != 0 {
		t.Fatalf("pad t=-0.5 expected 0, got %v", got)
	}
	if got := projectAndSpread(2, 0, g); got != 1 {
		t.Fatalf("pad t=2 expected 1, got %v", got)
	}
	if got := projectAndSpread(0.25, 0, g); !almostEq(got, 0.25, 1e-6) {
		t.Fatalf("pad t=0.25 expected 0.25, got %v", got)
	}
}

func TestApplySpreadReflectTriangleWave(t *testing.T) {
	g := mkLinear()
	g.SpreadMethod = gui.SvgSpreadReflect
	cases := []struct {
		in, want float32
	}{
		{0.25, 0.25},
		{1.25, 0.75},
		{2.25, 0.25},
		{-0.25, 0.25},
		{-1.25, 0.75},
	}
	for _, c := range cases {
		got := projectAndSpread(c.in, 0, g)
		if !almostEq(got, c.want, 1e-5) {
			t.Errorf("reflect t=%v expected %v got %v", c.in, c.want, got)
		}
	}
}

func TestApplySpreadRepeatSawtooth(t *testing.T) {
	g := mkLinear()
	g.SpreadMethod = gui.SvgSpreadRepeat
	cases := []struct {
		in, want float32
	}{
		{0.25, 0.25},
		{1.25, 0.25},
		{2.75, 0.75},
		{-0.25, 0.75},
	}
	for _, c := range cases {
		got := projectAndSpread(c.in, 0, g)
		if !almostEq(got, c.want, 1e-5) {
			t.Errorf("repeat t=%v expected %v got %v", c.in, c.want, got)
		}
	}
}

func TestApplySpreadNaNInfReturnsZero(t *testing.T) {
	g := mkLinear()
	g.SpreadMethod = gui.SvgSpreadReflect
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	if got := applySpread(nan, gui.SvgSpreadReflect); got != 0 {
		t.Fatalf("NaN expected 0, got %v", got)
	}
	if got := applySpread(inf, gui.SvgSpreadRepeat); got != 0 {
		t.Fatalf("+Inf expected 0, got %v", got)
	}
	if got := applySpread(-inf, gui.SvgSpreadPad); got != 0 {
		t.Fatalf("-Inf expected 0, got %v", got)
	}
}

func TestApplySpreadHugeTNoOverflow(t *testing.T) {
	// Past the int64 floor cast danger zone. Must not panic and must
	// return a finite value in [0, 1].
	huge := float32(1e30)
	got := applySpread(huge, gui.SvgSpreadReflect)
	if got < 0 || got > 1 {
		t.Fatalf("huge t reflect out of [0,1]: %v", got)
	}
	got = applySpread(huge, gui.SvgSpreadRepeat)
	if got < 0 || got > 1 {
		t.Fatalf("huge t repeat out of [0,1]: %v", got)
	}
}

func TestParseSpreadMethodKeywords(t *testing.T) {
	cases := []struct {
		in   string
		want gui.SvgGradientSpread
	}{
		{"pad", gui.SvgSpreadPad},
		{"", gui.SvgSpreadPad},
		{"reflect", gui.SvgSpreadReflect},
		{"  reflect ", gui.SvgSpreadReflect},
		{"repeat", gui.SvgSpreadRepeat},
		{"unknown", gui.SvgSpreadPad},
	}
	for _, c := range cases {
		got := parseSpreadMethod(c.in)
		if got != c.want {
			t.Errorf("parseSpreadMethod(%q) = %v want %v", c.in, got, c.want)
		}
	}
}

func TestParseDefsGradientsSpreadPropagates(t *testing.T) {
	svg := `<svg>
		<defs>
			<linearGradient id="lg" spreadMethod="reflect"
				x1="0" y1="0" x2="1" y2="0">
				<stop offset="0" stop-color="red"/>
				<stop offset="1" stop-color="blue"/>
			</linearGradient>
			<radialGradient id="rg" spreadMethod="repeat"
				cx="0.5" cy="0.5" r="0.5">
				<stop offset="0" stop-color="red"/>
				<stop offset="1" stop-color="blue"/>
			</radialGradient>
		</defs>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if vg.Gradients["lg"].SpreadMethod != gui.SvgSpreadReflect {
		t.Errorf("linear spread expected reflect, got %v",
			vg.Gradients["lg"].SpreadMethod)
	}
	if vg.Gradients["rg"].SpreadMethod != gui.SvgSpreadRepeat {
		t.Errorf("radial spread expected repeat, got %v",
			vg.Gradients["rg"].SpreadMethod)
	}
}
