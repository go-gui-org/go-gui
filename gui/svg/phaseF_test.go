package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// Phase F: display:none / visibility:hidden cascade gating, plus
// @media (prefers-reduced-motion: reduce) gating.

func TestPhaseF_DisplayNone_PresAttrSkipsShape(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<rect x="0" y="0" width="50" height="50" fill="red"/>
		<rect x="0" y="0" width="50" height="50" fill="blue" display="none"/>
	</svg>`
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) != 1 {
		t.Fatalf("paths: %d (want 1; display:none should drop the second rect)",
			len(vg.Paths))
	}
}

func TestPhaseF_DisplayNone_CSSSkipsShape(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>.hide { display: none }</style>
		<rect x="0" y="0" width="50" height="50"/>
		<rect class="hide" x="0" y="0" width="50" height="50"/>
	</svg>`
	vg, _ := parseSvg(src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
}

func TestPhaseF_DisplayNone_OnGroupSkipsChildren(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<g display="none">
			<rect x="0" y="0" width="50" height="50"/>
			<circle cx="50" cy="50" r="10"/>
		</g>
		<rect x="60" y="60" width="20" height="20"/>
	</svg>`
	vg, _ := parseSvg(src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths: %d (group display:none should drop both children)",
			len(vg.Paths))
	}
}

func TestPhaseF_DisplayNone_DoesNotInheritToChildren(t *testing.T) {
	// Only the .hide ancestor is removed; siblings of the same class
	// outside the hidden subtree must still render.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>.hide { display: none }</style>
		<g>
			<rect x="0" y="0" width="50" height="50"/>
		</g>
		<rect class="hide" x="60" y="60" width="20" height="20"/>
	</svg>`
	vg, _ := parseSvg(src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
}

func TestPhaseF_VisibilityHidden_ZerosAlpha(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<rect x="0" y="0" width="50" height="50" fill="red" visibility="hidden"/>
	</svg>`
	vg, _ := parseSvg(src)
	if len(vg.Paths) != 1 {
		t.Fatalf("paths: %d (visibility:hidden keeps the box, "+
			"only suppresses paint)", len(vg.Paths))
	}
	p := vg.Paths[0]
	if p.FillColor.A != 0 || p.StrokeColor.A != 0 {
		t.Errorf("alpha: fill=%d stroke=%d (want 0/0)",
			p.FillColor.A, p.StrokeColor.A)
	}
}

func TestPhaseF_VisibilityHidden_InheritedThenChildVisible(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<g visibility="hidden">
			<rect x="0" y="0" width="50" height="50" fill="red"/>
			<rect x="0" y="0" width="50" height="50" fill="blue" visibility="visible"/>
		</g>
	</svg>`
	vg, _ := parseSvg(src)
	if len(vg.Paths) != 2 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
	if vg.Paths[0].FillColor.A != 0 {
		t.Errorf("inherited hidden alpha: %d", vg.Paths[0].FillColor.A)
	}
	if vg.Paths[1].FillColor.A == 0 {
		t.Errorf("child visibility:visible should override; got alpha=0")
	}
}

func TestPhaseF_Media_ReducedMotion_KeepsRules(t *testing.T) {
	src := `
		.x { fill: red }
		@media (prefers-reduced-motion: reduce) {
			.x { fill: blue }
		}
	`
	dropped := css.ParseFull(src, css.ParseOptions{})
	if n := len(dropped.Rules); n != 1 {
		t.Errorf("opts off → rules: %d (want 1)", n)
	}
	kept := css.ParseFull(src, css.ParseOptions{PrefersReducedMotion: true})
	if n := len(kept.Rules); n != 2 {
		t.Errorf("opts on → rules: %d (want 2)", n)
	}
}

func TestPhaseF_Media_NoPreference_Inverse(t *testing.T) {
	src := `
		@media (prefers-reduced-motion: no-preference) {
			.x { fill: red }
		}
	`
	off := css.ParseFull(src, css.ParseOptions{})
	if n := len(off.Rules); n != 1 {
		t.Errorf("no-preference @ opts off → rules: %d (want 1)", n)
	}
	on := css.ParseFull(src, css.ParseOptions{PrefersReducedMotion: true})
	if n := len(on.Rules); n != 0 {
		t.Errorf("no-preference @ opts on → rules: %d (want 0)", n)
	}
}

func TestPhaseF_Media_UnknownQueryDropsBlock(t *testing.T) {
	src := `
		.outside { fill: red }
		@media (max-width: 800px) {
			.x { fill: blue }
		}
	`
	for _, opt := range []css.ParseOptions{
		{}, {PrefersReducedMotion: true},
	} {
		sheet := css.ParseFull(src, opt)
		if n := len(sheet.Rules); n != 1 {
			t.Errorf("unknown media query at %+v → rules: %d (want 1)",
				opt, n)
		}
	}
}

func TestPhaseF_Media_KeyframesInsideMediaBlock(t *testing.T) {
	// @keyframes inside an @media block: should appear only when the
	// query matches; dropped otherwise.
	src := `
		@media (prefers-reduced-motion: reduce) {
			@keyframes still { from { opacity: 1 } to { opacity: 1 } }
		}
		@keyframes other { from { opacity: 1 } to { opacity: 0 } }
	`
	off := css.ParseFull(src, css.ParseOptions{})
	if n := len(off.Keyframes); n != 1 {
		t.Errorf("opts off → keyframes: %d (want 1)", n)
	}
	on := css.ParseFull(src, css.ParseOptions{PrefersReducedMotion: true})
	if n := len(on.Keyframes); n != 2 {
		t.Errorf("opts on → keyframes: %d (want 2)", n)
	}
}

func TestPhaseF_Media_EndToEnd_ReducedAnimationStripped(t *testing.T) {
	// Author intent: under reduced motion, drop the spin animation by
	// re-binding animation-name to a no-op keyframes block.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>
			@keyframes spin {
				from { transform: rotate(0deg) }
				to   { transform: rotate(360deg) }
			}
			.r { animation: spin 1s linear }
			@media (prefers-reduced-motion: reduce) {
				.r { animation: none }
			}
		</style>
		<rect class="r" x="10" y="20" width="40" height="60"/>
	</svg>`

	full, _ := parseSvgWith(src, ParseOptions{})
	if len(full.Animations) == 0 {
		t.Fatalf("no animation when reduced-motion off")
	}

	reduced, _ := parseSvgWith(src, ParseOptions{PrefersReducedMotion: true})
	if len(reduced.Animations) != 0 {
		t.Errorf("animation count under reduced-motion: %d (want 0)",
			len(reduced.Animations))
	}
}

func TestPhaseF_ParserHash_DiffersByReducedMotion(t *testing.T) {
	base := parserSourceHash("svg-src", true)
	a := mixOptsHash(base, gui.SvgParseOpts{})
	b := mixOptsHash(base, gui.SvgParseOpts{PrefersReducedMotion: true})
	if a == b {
		t.Errorf("hashes identical across reduced-motion flip: %x", a)
	}
}

func TestPhaseF_ParserCache_OptsVariantSeparate(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<rect width="10" height="10"/>
	</svg>`
	p := New()
	a, err := p.ParseSvgWithOpts(src, gui.SvgParseOpts{})
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}
	b, err := p.ParseSvgWithOpts(src,
		gui.SvgParseOpts{PrefersReducedMotion: true})
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}
	if a == b {
		t.Errorf("same *SvgParsed across options variants — cache "+
			"entries collided: %p", a)
	}
}
