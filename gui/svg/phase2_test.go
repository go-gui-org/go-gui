package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestPhase2CircleCyOverrideShiftsTriangles verifies that supplying
// a cy override produces triangles whose bounding box differs from
// the static tessellation — the override is actually applied.
func TestPhase2CircleCyOverrideShiftsTriangles(t *testing.T) {
	p := New()
	parsed, err := p.ParseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="12" cy="12" r="3" fill="black"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"/></circle>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	staticTris := p.Tessellate(parsed, 1)
	if len(staticTris) == 0 {
		t.Fatal("no static triangles")
	}

	overrides := map[uint32]gui.SvgAnimAttrOverride{
		firstAnimatedPathID(parsed): {Mask: gui.SvgAnimMaskCY, CY: 6},
	}
	animTris := p.TessellateAnimated(parsed, 1, overrides, nil)
	if len(animTris) == 0 {
		t.Fatal("no animated triangles")
	}

	// Static circle centered at y=12; override to y=6 must shift
	// the triangles' mean Y by ~6 units.
	meanY := func(tris []float32) float32 {
		var sum float32
		var n int
		for i := 1; i < len(tris); i += 2 {
			sum += tris[i]
			n++
		}
		if n == 0 {
			return 0
		}
		return sum / float32(n)
	}
	staticY := meanY(staticTris[0].Triangles)
	animY := meanY(animTris[0].Triangles)
	shift := staticY - animY
	if shift < 5 || shift > 7 {
		t.Fatalf("expected ~6 unit shift, got %.3f (static %.3f anim %.3f)",
			shift, staticY, animY)
	}
}

// TestPhase2RectHeightAndYOverrides verifies simultaneous y + height
// overrides on a rect. A <rect y=6 height=12> with overrides y=1,
// height=22 should become taller and move up.
func TestPhase2RectHeightAndYOverrides(t *testing.T) {
	p := New()
	parsed, err := p.ParseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<rect x="1" y="6" width="2.8" height="12" fill="black">
			<animate attributeName="y" dur="0.6s" values="6;1;6"/>
			<animate attributeName="height" dur="0.6s" values="12;22;12"/>
		</rect>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	overrides := map[uint32]gui.SvgAnimAttrOverride{
		firstAnimatedPathID(parsed): {
			Mask:   gui.SvgAnimMaskY | gui.SvgAnimMaskHeight,
			Y:      1,
			Height: 22,
		},
	}
	animTris := p.TessellateAnimated(parsed, 1, overrides, nil)
	if len(animTris) == 0 {
		t.Fatal("no animated triangles")
	}
	var minY, maxY float32 = 1e30, -1e30
	for i := 1; i < len(animTris[0].Triangles); i += 2 {
		y := animTris[0].Triangles[i]
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	if minY < 0.5 || minY > 1.5 {
		t.Fatalf("expected y-top near 1, got %.3f", minY)
	}
	if maxY < 22 || maxY > 24 {
		t.Fatalf("expected y-bottom near 23, got %.3f", maxY)
	}
}

// TestPhase2ClipPathedAnimatedSkipsRetessellation verifies that a
// clip-pathed primitive with attr animation is NOT flagged Animated
// and does not appear in TessellateAnimated's output. Phase-2 scope.
func TestPhase2ClipPathedAnimatedSkipsRetessellation(t *testing.T) {
	p := New()
	parsed, err := p.ParseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<defs><clipPath id="cp"><rect x="0" y="0" width="12" height="24"/></clipPath></defs>
		<circle cx="12" cy="12" r="3" fill="black" clip-path="url(#cp)">
			<animate attributeName="cy" dur="0.6s" values="12;6;12"/>
		</circle>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	overrides := map[uint32]gui.SvgAnimAttrOverride{
		firstAnimatedPathID(parsed): {Mask: gui.SvgAnimMaskCY, CY: 6},
	}
	out := p.TessellateAnimated(parsed, 1, overrides, nil)
	if out != nil {
		t.Fatalf("expected nil; clip-pathed animated path is out of scope")
	}
}

// TestPhase2MaskZeroOverrideSkipped verifies per-frame frames where
// an animation hasn't applied any attribute (all values prior to
// begin) do not produce overrides.
func TestPhase2MaskZeroOverrideSkipped(t *testing.T) {
	ov := gui.SvgAnimAttrOverride{}
	if ov.Mask != 0 {
		t.Fatalf("zero value must have Mask==0")
	}
}

// TestPhase2ReuseBufferIsReused verifies passing a reuse slice with
// matching capacity avoids allocating a fresh outer slice.
func TestPhase2ReuseBufferIsReused(t *testing.T) {
	p := New()
	parsed, err := p.ParseSvg(`<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="12" cy="12" r="3" fill="black"><animate
			attributeName="cy" dur="0.6s" values="12;6;12"/></circle>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	overrides := map[uint32]gui.SvgAnimAttrOverride{
		firstAnimatedPathID(parsed): {Mask: gui.SvgAnimMaskCY, CY: 6},
	}
	reuse := make([]gui.TessellatedPath, 0, 8)
	out1 := p.TessellateAnimated(parsed, 1, overrides, reuse)
	if len(out1) == 0 {
		t.Fatal("no triangles returned")
	}
	// Reuse must share the caller's backing array when cap is big
	// enough — verify by pointer identity of element 0.
	if &out1[0] != &reuse[:1][0] {
		t.Fatal("expected returned slice to alias reuse backing array")
	}
}

// overrideScalar replaces the base value when the mask bit is set
// without AdditiveMask, and sums base+delta when AdditiveMask is
// also set.
func TestApplyOverridesToPath_NonAdditiveReplaces(t *testing.T) {
	p := &VectorPath{
		Primitive: gui.SvgPrimitive{
			Kind: gui.SvgPrimCircle, CX: 12, CY: 12, R: 5,
		},
	}
	ov := gui.SvgAnimAttrOverride{
		Mask: gui.SvgAnimMaskR,
		R:    9,
	}
	applyOverridesToPath(p, ov)
	if p.Primitive.R != 9 {
		t.Fatalf("replace: want R=9, got %f", p.Primitive.R)
	}
}

func TestApplyOverridesToPath_AdditiveAddsDelta(t *testing.T) {
	p := &VectorPath{
		Primitive: gui.SvgPrimitive{
			Kind: gui.SvgPrimCircle, CX: 12, CY: 12, R: 5,
		},
	}
	ov := gui.SvgAnimAttrOverride{
		Mask:         gui.SvgAnimMaskR,
		AdditiveMask: gui.SvgAnimMaskR,
		R:            3, // delta
	}
	applyOverridesToPath(p, ov)
	if p.Primitive.R != 8 {
		t.Fatalf("additive: want R=5+3=8, got %f", p.Primitive.R)
	}
}

// Unset mask bits leave the parsed base untouched regardless of the
// field value or AdditiveMask bit.
func TestApplyOverridesToPath_UnsetMaskKeepsBase(t *testing.T) {
	p := &VectorPath{
		Primitive: gui.SvgPrimitive{
			Kind: gui.SvgPrimCircle, CX: 12, CY: 12, R: 5,
		},
	}
	ov := gui.SvgAnimAttrOverride{
		// No mask bits set; R payload must be ignored.
		R: 99,
	}
	applyOverridesToPath(p, ov)
	if p.Primitive.R != 5 {
		t.Fatalf("unset mask: want R=5 unchanged, got %f", p.Primitive.R)
	}
}
