package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestPhase1CirclePrimitiveRecorded verifies <circle> parsing stores
// raw cx/cy/r on VectorPath.Primitive for re-tessellation.
func TestPhase1CirclePrimitiveRecorded(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3" fill="black"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) != 1 {
		t.Fatalf("want 1 path, got %d", len(vg.Paths))
	}
	p := vg.Paths[0].Primitive
	if p.Kind != gui.SvgPrimCircle {
		t.Fatalf("want circle kind, got %d", p.Kind)
	}
	if p.CX != 4 || p.CY != 12 || p.R != 3 {
		t.Fatalf("want cx=4 cy=12 r=3, got %+v", p)
	}
	if vg.Paths[0].Animated {
		t.Fatal("static circle should not be Animated")
	}
}

// TestPhase1RectPrimitiveRecorded verifies <rect> parsing stores x/y/w/h.
func TestPhase1RectPrimitiveRecorded(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<rect x="1" y="6" width="2.8" height="12" fill="black"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := vg.Paths[0].Primitive
	if p.Kind != gui.SvgPrimRect {
		t.Fatalf("want rect kind, got %d", p.Kind)
	}
	if p.X != 1 || p.Y != 6 || p.W != 2.8 || p.H != 12 {
		t.Fatalf("want x=1 y=6 w=2.8 h=12, got %+v", p)
	}
}

// TestPhase1AttrAnimationMarksShapeAnimated verifies an inline
// <animate attributeName="cy"> flags the parent path as animated
// and records an SvgAnimAttr animation with matching attr name.
func TestPhase1AttrAnimationMarksShapeAnimated(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3"><animate attributeName="cy"
			dur="0.6s" values="12;6;12"/></circle>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) != 1 || !vg.Paths[0].Animated {
		t.Fatalf("circle with cy animation must be Animated: %+v",
			vg.Paths)
	}
	if len(vg.Animations) != 1 {
		t.Fatalf("want 1 animation, got %d", len(vg.Animations))
	}
	a := vg.Animations[0]
	if a.Kind != gui.SvgAnimAttr || a.AttrName != gui.SvgAttrCY {
		t.Fatalf("want SvgAnimAttr/cy, got kind=%d attr=%d",
			a.Kind, a.AttrName)
	}
	if a.GroupID == "" || a.GroupID != vg.Paths[0].GroupID {
		t.Fatalf("GroupID mismatch: path=%q anim=%q",
			vg.Paths[0].GroupID, a.GroupID)
	}
}

// TestPhase1OpacityAnimationNotAttrKind guards against regressing
// the pre-phase-1 opacity path to the new SvgAnimAttr branch.
func TestPhase1OpacityAnimationNotAttrKind(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3"><animate attributeName="opacity"
			dur="1s" values="0;1;0"/></circle>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if vg.Paths[0].Animated {
		t.Fatal("opacity animation must not flag Animated")
	}
	if vg.Animations[0].Kind != gui.SvgAnimOpacity {
		t.Fatalf("want SvgAnimOpacity, got %d",
			vg.Animations[0].Kind)
	}
}

// TestPhase1TessellatePropagatesPrimitiveAndAnimated verifies the
// tessellator copies VectorPath.Animated and VectorPath.Primitive
// through to gui.TessellatedPath.
func TestPhase1TessellatePropagatesPrimitiveAndAnimated(t *testing.T) {
	vg, err := parseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3" fill="black"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"/></circle>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tpaths := vg.getTriangles(1)
	if len(tpaths) == 0 {
		t.Fatal("no tessellated paths")
	}
	var found bool
	for _, tp := range tpaths {
		if tp.Animated && tp.Primitive.Kind == gui.SvgPrimCircle {
			if tp.Primitive.CX == 4 && tp.Primitive.CY == 12 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("Animated circle primitive not propagated to TessellatedPath")
	}
}

// TestPhase1ParserImplementsAnimatedSvgParser guards the optional
// interface. Sibling backends can assume *svg.Parser satisfies it.
func TestPhase1ParserImplementsAnimatedSvgParser(t *testing.T) {
	var _ gui.AnimatedSvgParser = (*Parser)(nil)
}

// TestPhase1TessellateAnimatedReturnsOnlyFlaggedPaths verifies the
// new API limits output to Animated paths.
func TestPhase1TessellateAnimatedReturnsOnlyFlaggedPaths(t *testing.T) {
	p := New()
	parsed, err := p.ParseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3" fill="black"/>
		<circle cx="12" cy="12" r="3" fill="black"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"/></circle>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	overrides := map[uint32]gui.SvgAnimAttrOverride{
		firstAnimatedPathID(parsed): {Mask: gui.SvgAnimMaskCY, CY: 6},
	}
	out := p.TessellateAnimated(parsed, 1, overrides, nil)
	if len(out) == 0 {
		t.Fatal("expected at least one animated triangle batch")
	}
	for _, tp := range out {
		if !tp.Animated {
			t.Fatal("TessellateAnimated returned non-animated path")
		}
	}
}

// TestPhase1NoOverridesYieldsNil verifies TessellateAnimated returns
// nil when the overrides map is empty — caller falls back to cached.
func TestPhase1NoOverridesYieldsNil(t *testing.T) {
	p := New()
	parsed, err := p.ParseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="4" cy="12" r="3" fill="black"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"/></circle>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := p.TessellateAnimated(parsed, 1, nil, nil)
	if out != nil {
		t.Fatalf("expected nil on empty overrides, got %d paths", len(out))
	}
}
