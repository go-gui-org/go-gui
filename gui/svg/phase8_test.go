package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Phase 8: degenerate-static animated primitives must survive
// tessellation as zero-triangle placeholders so TessellateAnimated
// can substitute live geometry per frame. Without this an asset
// like 6-dots-scale.svg (12 circles all r="0" animating r) renders
// nothing because the animation system never sees the paths.

// TestPhase8ZeroRadiusCirclePreserved — a circle with r="0" plus an
// inline animate on r emits one TessellatedPath placeholder carrying
// the primitive metadata, with Triangles==nil and Animated==true.
func TestPhase8ZeroRadiusCirclePreserved(t *testing.T) {
	doc := `<svg viewBox="0 0 24 24" fill="black">` +
		`<circle cx="12" cy="12" r="0">` +
		`<animate attributeName="r" dur="1s" values="0;3;0"/>` +
		`</circle></svg>`
	p := New()
	parsed, err := p.ParseSvg(doc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Paths) != 1 {
		t.Fatalf("paths=%d want 1", len(parsed.Paths))
	}
	tp := parsed.Paths[0]
	if !tp.Animated {
		t.Fatal("placeholder not flagged Animated")
	}
	if len(tp.Triangles) != 0 {
		t.Fatalf("placeholder tris=%d want 0", len(tp.Triangles))
	}
	if tp.Primitive.Kind != gui.SvgPrimCircle {
		t.Fatalf("kind=%v want SvgPrimCircle", tp.Primitive.Kind)
	}
	if tp.Primitive.CX != 12 || tp.Primitive.CY != 12 {
		t.Fatalf("cx,cy=%v,%v want 12,12",
			tp.Primitive.CX, tp.Primitive.CY)
	}
	if tp.PathID == 0 {
		t.Fatal("missing PathID — TessellateAnimated cannot key it")
	}
}

// TestPhase8TessellateAnimatedRevives — feeding an R override to a
// degenerate-static circle produces non-empty triangles per frame.
func TestPhase8TessellateAnimatedRevives(t *testing.T) {
	doc := `<svg viewBox="0 0 24 24" fill="black">` +
		`<circle cx="12" cy="12" r="0">` +
		`<animate attributeName="r" dur="1s" values="0;3;0"/>` +
		`</circle></svg>`
	p := New()
	parsed, _ := p.ParseSvg(doc)
	tp := parsed.Paths[0]
	overrides := map[uint32]gui.SvgAnimAttrOverride{
		tp.PathID: {Mask: gui.SvgAnimMaskR, R: 3},
	}
	live := p.TessellateAnimated(parsed, 1.0, overrides, nil)
	if len(live) != 1 {
		t.Fatalf("live paths=%d want 1", len(live))
	}
	if len(live[0].Triangles) == 0 {
		t.Fatal("live triangles empty — primitive did not regenerate")
	}
}

// TestPhase8NonAnimatedZeroRadiusDropped — a degenerate circle with
// no animation must NOT get a placeholder; only animated paths need
// to survive. Otherwise authors can't use r="0" as a no-op shape.
func TestPhase8NonAnimatedZeroRadiusDropped(t *testing.T) {
	doc := `<svg viewBox="0 0 24 24" fill="black">` +
		`<circle cx="12" cy="12" r="0"/></svg>`
	p := New()
	parsed, _ := p.ParseSvg(doc)
	if len(parsed.Paths) != 0 {
		t.Fatalf("paths=%d want 0 (no animation, no placeholder)",
			len(parsed.Paths))
	}
}

// TestPhase8AssetCorpus6DotsScale — end-to-end: 6-dots-scale.svg has
// 12 circles all r="0" animating r. Static parse must yield 12
// placeholder paths, all Animated, all SvgPrimCircle.
func TestPhase8AssetCorpus6DotsScale(t *testing.T) {
	// Inline a trimmed equivalent (4 dots) to keep the test
	// hermetic — the corpus loader is not in this package's deps.
	doc := `<svg viewBox="0 0 24 24" fill="currentColor">` +
		`<circle cx="12" cy="3" r="0">` +
		`<animate attributeName="r" dur="0.6s" values="0;2;0"/>` +
		`</circle>` +
		`<circle cx="21" cy="12" r="0">` +
		`<animate attributeName="r" dur="0.6s" values="0;2;0"/>` +
		`</circle>` +
		`<circle cx="12" cy="21" r="0">` +
		`<animate attributeName="r" dur="0.6s" values="0;2;0"/>` +
		`</circle>` +
		`<circle cx="3" cy="12" r="0">` +
		`<animate attributeName="r" dur="0.6s" values="0;2;0"/>` +
		`</circle>` +
		`</svg>`
	p := New()
	parsed, err := p.ParseSvg(doc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Paths) != 4 {
		t.Fatalf("paths=%d want 4", len(parsed.Paths))
	}
	for i, tp := range parsed.Paths {
		if !tp.Animated {
			t.Errorf("[%d] not Animated", i)
		}
		if tp.Primitive.Kind != gui.SvgPrimCircle {
			t.Errorf("[%d] kind=%v want SvgPrimCircle",
				i, tp.Primitive.Kind)
		}
	}
}

// TestPhase8ZeroRectPreserved — the same logic must hold for rect
// primitives animating width/height from 0.
func TestPhase8ZeroRectPreserved(t *testing.T) {
	doc := `<svg viewBox="0 0 24 24" fill="black">` +
		`<rect x="0" y="0" width="0" height="10">` +
		`<animate attributeName="width" dur="1s" values="0;20;0"/>` +
		`</rect></svg>`
	p := New()
	parsed, _ := p.ParseSvg(doc)
	if len(parsed.Paths) != 1 {
		t.Fatalf("paths=%d want 1", len(parsed.Paths))
	}
	if !parsed.Paths[0].Animated {
		t.Fatal("rect placeholder not flagged Animated")
	}
	if parsed.Paths[0].Primitive.Kind != gui.SvgPrimRect {
		t.Fatalf("kind=%v want SvgPrimRect",
			parsed.Paths[0].Primitive.Kind)
	}
}
