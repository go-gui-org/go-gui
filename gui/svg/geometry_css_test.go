package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// SVG2 geometry properties (cx, cy, r, rx, ry, x, y, width, height,
// x1, y1, x2, y2) double as CSS properties: author rules and inline
// style override the presentation attributes after var()/calc()
// resolution. The showcase var()+calc() sample exercises this on
// circle r; without it the sample renders zero paths.

func TestGeometryCSS_VarCalcRadius(t *testing.T) {
	src := `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <style>
    svg { --accent: #06b6d4; --r: 18; }
    .v { fill: var(--accent, #888); r: calc(var(--r) - 4); }
  </style>
  <circle class="v" cx="25" cy="50"/>
  <circle class="v" cx="55" cy="50"/>
  <circle class="v" cx="85" cy="50"/>
</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 3 {
		t.Fatalf("paths=%d want 3", len(vg.Paths))
	}
	wantFill := gui.SvgColor{R: 6, G: 182, B: 212, A: 255}
	for i, p := range vg.Paths {
		if p.Primitive.Kind != gui.SvgPrimCircle {
			t.Errorf("path %d kind=%v want circle", i, p.Primitive.Kind)
		}
		if p.Primitive.R != 14 {
			t.Errorf("path %d R=%v want 14", i, p.Primitive.R)
		}
		if p.FillColor != wantFill {
			t.Errorf("path %d fill=%+v want %+v", i, p.FillColor, wantFill)
		}
	}
}

func TestGeometryCSS_RuleOverridesAttr(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.big { r: 9 }</style>
		<circle class="big" cx="5" cy="5" r="5"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths=%d want 1", len(vg.Paths))
	}
	if vg.Paths[0].Primitive.R != 9 {
		t.Errorf("R=%v want 9 (rule beats pres-attr)", vg.Paths[0].Primitive.R)
	}
}

func TestGeometryCSS_InlineOverridesRule(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.big { r: 9 }</style>
		<circle class="big" cx="5" cy="5" r="5" style="r: 3"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths=%d want 1", len(vg.Paths))
	}
	if vg.Paths[0].Primitive.R != 3 {
		t.Errorf("R=%v want 3 (inline beats rule)", vg.Paths[0].Primitive.R)
	}
}

func TestGeometryCSS_InvalidRuleKeepsAttr(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.bad { r: bogus }</style>
		<circle class="bad" cx="5" cy="5" r="7"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths=%d want 1", len(vg.Paths))
	}
	if vg.Paths[0].Primitive.R != 7 {
		t.Errorf("R=%v want 7 (invalid rule ignored)", vg.Paths[0].Primitive.R)
	}
}

func TestGeometryCSS_MissingRadiusDropsCircle(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<circle cx="5" cy="5"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 0 {
		t.Errorf("paths=%d want 0 (no r anywhere)", len(vg.Paths))
	}
}

func TestGeometryCSS_NegativeRuleRadiusDropsCircle(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.neg { r: -3 }</style>
		<circle class="neg" cx="5" cy="5" r="5"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 0 {
		t.Errorf("paths=%d want 0 (negative r disables)", len(vg.Paths))
	}
}

func TestGeometryCSS_NotInherited(t *testing.T) {
	// Geometry is per-element: a declaration on <g> must not leak
	// into a child circle that declares no r of its own.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<g style="r: 4"><circle cx="5" cy="5"/></g>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 0 {
		t.Errorf("paths=%d want 0 (r not inherited)", len(vg.Paths))
	}
}

func TestGeometryCSS_RectLineEllipse(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
		<style>
			.wide { width: calc(10px + 4px); height: 6 }
			.tall { x2: 12 }
			.flat { rx: 8 }
		</style>
		<rect class="wide" x="1" y="1" width="2" height="2"/>
		<line class="tall" x1="1" y1="1" x2="2" y2="2"/>
		<ellipse class="flat" cx="10" cy="10" ry="3"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 3 {
		t.Fatalf("paths=%d want 3", len(vg.Paths))
	}
	rect := vg.Paths[0].Primitive
	if rect.W != 14 || rect.H != 6 {
		t.Errorf("rect W,H=%v,%v want 14,6", rect.W, rect.H)
	}
	line := vg.Paths[1].Primitive
	if line.X2 != 12 {
		t.Errorf("line X2=%v want 12", line.X2)
	}
	ell := vg.Paths[2].Primitive
	// rx:8 via CSS, ry:3 via attribute.
	if ell.RX != 8 || ell.RY != 3 {
		t.Errorf("ellipse RX,RY=%v,%v want 8,3", ell.RX, ell.RY)
	}
}

func TestGeometryCSS_CalcPxSuffix(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 30">
		<style>.mv { cx: calc(10px + 5px); cy: 20px }</style>
		<circle class="mv" r="4"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths=%d want 1", len(vg.Paths))
	}
	p := vg.Paths[0].Primitive
	if p.CX != 15 || p.CY != 20 {
		t.Errorf("CX,CY=%v,%v want 15,20", p.CX, p.CY)
	}
}

func TestGeometryCSS_EllipseAutoMirrorFromCSS(t *testing.T) {
	// A lone CSS rx with no ry anywhere mirrors, matching the
	// presentation-attribute auto rule; a negative CSS rx reads as
	// auto too, so it mirrors instead of disabling.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
		<style>.solo { rx: 6 } .neg { rx: -2; ry: 5 }</style>
		<ellipse class="solo" cx="5" cy="5"/>
		<ellipse class="neg" cx="15" cy="5"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 2 {
		t.Fatalf("paths=%d want 2", len(vg.Paths))
	}
	if got := vg.Paths[0].Primitive; got.RX != 6 || got.RY != 6 {
		t.Errorf("solo RX,RY=%v,%v want 6,6", got.RX, got.RY)
	}
	if got := vg.Paths[1].Primitive; got.RX != 5 || got.RY != 5 {
		t.Errorf("neg RX,RY=%v,%v want 5,5", got.RX, got.RY)
	}
}

func TestGeometryCSS_NegativeRuleSizeDisablesRect(t *testing.T) {
	// A negative CSS width disables like a negative attribute does;
	// it does not fall back to the attribute.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.neg { width: -4 }</style>
		<rect class="neg" x="1" y="1" width="8" height="8"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 0 {
		t.Errorf("paths=%d want 0 (negative width disables)", len(vg.Paths))
	}
}

func TestParseGeometryLength(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want float32
		ok   bool
	}{
		{"14", 14, true},
		{"14px", 14, true},
		{"  7.5px  ", 7.5, true},
		{"-3", -3, true},
		{"", 0, false},
		{"bogus", 0, false},
		{"px", 0, false},
		{"NaN", 0, false},
		{"Inf", 0, false},
		{"1e300", 0, false},
		{"10%", 0, false},
	} {
		got, ok := parseGeometryLength(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseGeometryLength(%q)=%v,%v want %v,%v",
				tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
