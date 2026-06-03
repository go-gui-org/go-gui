package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Phase C end-to-end: spec-correct cascade (pres-attr < CSS rule <
// inline style; !important promotes), descendant/child combinators,
// :nth-child / :root, custom properties.

func TestPhaseC_InlineStyleBeatsCSSRule(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.cls { fill: red }</style>
		<rect class="cls" style="fill: blue" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("inline > CSS: %+v", vg.Paths[0].FillColor)
	}
}

func TestPhaseC_CSSImportantBeatsInlineNormal(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.cls { fill: red !important }</style>
		<rect class="cls" style="fill: blue" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("CSS!important > inline normal: %+v",
			vg.Paths[0].FillColor)
	}
}

func TestPhaseC_InlineImportantBeatsCSSImportant(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>.cls { fill: red !important }</style>
		<rect class="cls" style="fill: blue !important" width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("inline!important > CSS!important: %+v",
			vg.Paths[0].FillColor)
	}
}

func TestPhaseC_DescendantCombinator(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>g circle { fill: red }</style>
		<g><circle cx="5" cy="5" r="4"/></g>
		<circle cx="5" cy="5" r="4"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 2 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("descendant g circle should match: %+v",
			vg.Paths[0].FillColor)
	}
	if vg.Paths[1].FillColor != colorBlack {
		t.Errorf("non-descendant circle should not match: %+v",
			vg.Paths[1].FillColor)
	}
}

func TestPhaseC_ChildCombinator(t *testing.T) {
	// `g > circle` matches a circle whose direct parent is <g>. The
	// second circle's direct parent is <a>, not <g>, so the rule
	// must not match even though a <g> ancestor exists higher up.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>g > circle { fill: red }</style>
		<g><circle cx="5" cy="5" r="4"/></g>
		<g><a><circle cx="5" cy="5" r="4"/></a></g>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 2 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("direct child should match: %+v", vg.Paths[0].FillColor)
	}
	if vg.Paths[1].FillColor != colorBlack {
		t.Errorf("indirect descendant must not match g > circle: %+v",
			vg.Paths[1].FillColor)
	}
}

func TestPhaseC_NthChildOdd(t *testing.T) {
	// :nth-child counts ALL element siblings (including <style>),
	// so put rects under their own <g> for predictable indexing.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>rect:nth-child(odd) { fill: red }</style>
		<g>
			<rect width="2" height="2"/>
			<rect width="2" height="2"/>
			<rect width="2" height="2"/>
			<rect width="2" height="2"/>
		</g>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) != 4 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
	red := gui.SvgColor{R: 255, A: 255}
	if vg.Paths[0].FillColor != red || vg.Paths[2].FillColor != red {
		t.Errorf("odd indices should be red")
	}
	if vg.Paths[1].FillColor != colorBlack ||
		vg.Paths[3].FillColor != colorBlack {
		t.Errorf("even indices should be black")
	}
}

func TestPhaseC_RootSelector(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>:root { fill: red }</style>
		<rect width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	// :root matches the <svg>; fill inherits from svg to rect.
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf(":root cascade to child: %+v", vg.Paths[0].FillColor)
	}
}

func TestPhaseC_CustomProperty(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
			:root { --brand: rgb(255,0,0) }
			rect  { fill: var(--brand) }
		</style>
		<rect width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("custom prop substitution: %+v",
			vg.Paths[0].FillColor)
	}
}

func TestPhaseC_CustomPropertyInherits(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>rect { fill: var(--brand) }</style>
		<g style="--brand: rgb(0,0,255)">
			<rect width="10" height="10"/>
		</g>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("inherited var: %+v", vg.Paths[0].FillColor)
	}
}

func TestPhaseC_UndefinedVarDropsDecl(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>rect { fill: var(--missing) }</style>
		<rect width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	// Undefined var → decl dropped → fill defaults to black.
	if vg.Paths[0].FillColor != colorBlack {
		t.Errorf("undefined var should drop decl: %+v",
			vg.Paths[0].FillColor)
	}
}

func TestPhaseC_VarFallbackUsed(t *testing.T) {
	// v0.14.0: fallback after the comma is honored when the named
	// variable is undefined.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>rect { fill: var(--missing, blue) }</style>
		<rect width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("fallback should be used: %+v",
			vg.Paths[0].FillColor)
	}
}

func TestPhaseC_VarFallbackChain(t *testing.T) {
	// Nested fallback: var(--a, var(--b, red)) where --a undefined
	// and --b undefined → uses outer fallback's fallback.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>rect { fill: var(--a, var(--b, red)) }</style>
		<rect width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("nested fallback chain should resolve: %+v",
			vg.Paths[0].FillColor)
	}
}

func TestPhaseC_VarSelfReferenceWithFallback(t *testing.T) {
	// Cycle bound + fallback interaction: --a refers to itself, with
	// a fallback. The recursion cap kicks in before the fallback can
	// rescue, so the decl drops. Documents observable behavior.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
			:root { --a: var(--a, red) }
			rect  { fill: var(--a) }
		</style>
		<rect width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != colorBlack {
		t.Errorf("self-ref var w/ fallback: %+v",
			vg.Paths[0].FillColor)
	}
}

func TestPhaseC_AttrSelectorEndToEnd(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 10">
		<style>
			rect { fill: blue }
			rect[data-state=active] { fill: red }
		</style>
		<rect x="0"  y="0" width="10" height="10"/>
		<rect x="10" y="0" width="10" height="10" data-state="active"/>
		<rect x="20" y="0" width="10" height="10" data-state="other"/>
	</svg>`
	vg := parseSvgT(t, src)
	if got := vg.Paths[0].FillColor; got != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("plain rect: %+v", got)
	}
	if got := vg.Paths[1].FillColor; got != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("active rect: %+v", got)
	}
	if got := vg.Paths[2].FillColor; got != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("other rect: %+v", got)
	}
}

func TestPhaseC_AdjacentSiblingEndToEnd(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 50 10">
		<style>
			circle { fill: blue }
			rect + circle { fill: red }
		</style>
		<circle cx="5"  cy="5" r="4"/>
		<rect   x="10" y="0" width="10" height="10"/>
		<circle cx="25" cy="5" r="4"/>
		<circle cx="35" cy="5" r="4"/>
	</svg>`
	vg := parseSvgT(t, src)
	// Path order: circle, rect, circle, circle.
	// Path[0] circle: no preceding sibling → blue.
	// Path[1] rect.
	// Path[2] circle: prev sibling rect → red.
	// Path[3] circle: prev sibling circle → blue.
	if got := vg.Paths[0].FillColor; got != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("first circle (no rect prev): %+v", got)
	}
	if got := vg.Paths[2].FillColor; got != (gui.SvgColor{R: 255, A: 255}) {
		t.Errorf("circle after rect: %+v", got)
	}
	if got := vg.Paths[3].FillColor; got != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("circle after circle: %+v", got)
	}
}

func TestPhaseC_VarDefinedBeatsFallback(t *testing.T) {
	// When --primary IS defined, fallback is unused.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
			:root { --primary: blue }
			rect  { fill: var(--primary, red) }
		</style>
		<rect width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{B: 255, A: 255}) {
		t.Errorf("defined var should beat fallback: %+v",
			vg.Paths[0].FillColor)
	}
}

// Hostile asset: a → b → a custom-property cycle. Without the
// recursion cap, resolveVarRefs blows the stack. Cap drops the
// declaration, fill falls back to default (black).
func TestPhaseC_VarCycleBoundedDepth(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>
			:root { --a: var(--b); --b: var(--a) }
			rect  { fill: var(--a) }
		</style>
		<rect width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != colorBlack {
		t.Errorf("cyclic var should drop decl: %+v",
			vg.Paths[0].FillColor)
	}
}

// stroke-dasharray must reject NaN (and any non-finite) tokens, else
// the bogus float poisons stroke geometry downstream.
func TestPhaseC_DashArrayRejectsNaN(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>line { stroke: black; stroke-dasharray: NaN 2 }</style>
		<line x1="0" y1="0" x2="10" y2="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths) == 0 {
		t.Fatal("expected stroked line")
	}
	for i := range vg.Paths {
		for _, d := range vg.Paths[i].StrokeDasharray {
			if d != d || d < 0 { // d != d ⇒ NaN
				t.Errorf("dash slot leaked non-finite: %v", d)
			}
		}
	}
}

func TestPhaseC_TransformNoDoubleApply(t *testing.T) {
	// A shape's transform should compose parent × own exactly once,
	// even when CSS rules also match the shape (forcing the cascade
	// to re-run for the shape).
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
		<style>circle { stroke: red }</style>
		<circle cx="0" cy="0" r="5" transform="translate(10,10)"/>
	</svg>`
	vg := parseSvgT(t, src)
	tx := vg.Paths[0].Transform[4]
	ty := vg.Paths[0].Transform[5]
	if tx != 10 || ty != 10 {
		t.Errorf("transform composed twice: tx=%v ty=%v", tx, ty)
	}
}

func TestPhaseC_InlineStyleVarDef(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<rect style="--c: rgb(0,200,0); fill: var(--c)"
			width="10" height="10"/>
	</svg>`
	vg := parseSvgT(t, src)
	if vg.Paths[0].FillColor != (gui.SvgColor{G: 200, A: 255}) {
		t.Errorf("inline style var: %+v", vg.Paths[0].FillColor)
	}
}

func TestPhaseC_StrokeDasharrayCSS(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>line { stroke: black; stroke-dasharray: 3 2 }</style>
		<line x1="0" y1="5" x2="10" y2="5"/>
	</svg>`
	vg := parseSvgT(t, src)
	if len(vg.Paths[0].StrokeDasharray) != 2 ||
		vg.Paths[0].StrokeDasharray[0] != 3 ||
		vg.Paths[0].StrokeDasharray[1] != 2 {
		t.Errorf("stroke-dasharray via CSS: %+v",
			vg.Paths[0].StrokeDasharray)
	}
}

func TestPhaseC_GroupSelector(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<style>rect, circle { fill: red }</style>
		<rect width="2" height="2"/>
		<circle cx="5" cy="5" r="2"/>
	</svg>`
	vg := parseSvgT(t, src)
	red := gui.SvgColor{R: 255, A: 255}
	if vg.Paths[0].FillColor != red || vg.Paths[1].FillColor != red {
		t.Errorf("group selector: %+v %+v",
			vg.Paths[0].FillColor, vg.Paths[1].FillColor)
	}
}
