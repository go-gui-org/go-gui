package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Phase B end-to-end: <style> blocks + tag/id/class selectors feed
// the cascade and resolve into VectorPath paint properties.

func parseSvgT(t *testing.T, src string) *VectorGraphic {
	t.Helper()
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parseSvg: %v", err)
	}
	return vg
}

func TestPhaseB_ClassSelectorFill(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.dot { fill: rgb(255,0,0) }</style>
		<rect class="dot" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
	got := vg.Paths[0].FillColor
	want := gui.SvgColor{R: 255, A: 255}
	if got != want {
		t.Errorf("fill: %+v want %+v", got, want)
	}
}

func TestPhaseB_IDBeatsClass(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
			.cls { fill: red }
			#a   { fill: blue }
		</style>
		<rect id="a" class="cls" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("fill: %+v", vg.Paths[0].FillColor)
	}
}

func TestPhaseB_PresentationAttrBeatsCSS(t *testing.T) {
	// Phase C: a CSS class rule (specificity 0,1,0) beats a
	// presentation attribute (specificity 0). Pres-attr is the
	// lowest-precedence layer above inheritance.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.cls { fill: red }</style>
		<rect class="cls" fill="blue" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("CSS class should beat pres attr in Phase C: %+v",
			vg.Paths[0].FillColor)
	}
}

func TestPhaseB_ImportantBeatsLowerSpec(t *testing.T) {
	// !important rule from a class beats a normal rule from an id.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
			#a   { fill: blue }
			.cls { fill: red !important }
		</style>
		<rect id="a" class="cls" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("!important: %+v", vg.Paths[0].FillColor)
	}
}

func TestPhaseB_StrokeAndWidth(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
			circle { stroke: #00ff00; stroke-width: 3 }
		</style>
		<circle cx="5" cy="5" r="4"/>
	</svg>`
	vg := parseSvgT(t, src)
	p := vg.Paths[0]
	if p.StrokeColor != (gui.SvgColor{G: 255, A: 255}) {
		t.Errorf("stroke: %+v", p.StrokeColor)
	}
	if p.StrokeWidth != 3 {
		t.Errorf("stroke-width: %v", p.StrokeWidth)
	}
}

func TestPhaseB_OpacityCascade(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.dim { opacity: 0.5 }</style>
		<rect class="dim" width="10" height="10" fill="red"/>
	</svg>`
	vg := parseSvgT(t, src)
	a := vg.Paths[0].FillColor.A
	if a < 120 || a > 135 {
		t.Errorf("opacity-baked alpha: %d, want ~127", a)
	}
}

func TestPhaseB_GroupRuleInheritsToShape(t *testing.T) {
	// Author CSS on the group should still cascade to children
	// even though the child has no rule of its own.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>g { fill: rgb(0,128,0) }</style>
		<g><rect width="10" height="10"/></g>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{G: 128, A: 255}) {
		t.Errorf("inherited fill: %+v", vg.Paths[0].FillColor)
	}
}

func TestPhaseB_NoStyleBlock_NoRegression(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<rect width="10" height="10" fill="red"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("baseline fill broke: %+v", vg.Paths[0].FillColor)
	}
}

func TestPhaseB_MultiClass(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.a.b { fill: red }</style>
		<rect class="a b c" width="10" height="10"/>
		<rect class="a" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 2 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("multi-class match: %+v", vg.Paths[0].FillColor)
	}
	// Second rect missing class "b" — falls through to default (black).
	if vg.Paths[1].FillColor != colorBlack {
		t.Errorf("partial class shouldn't match: %+v", vg.Paths[1].FillColor)
	}
}
