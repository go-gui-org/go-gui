package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// seedFromTransform defers TRS-decomposable non-identity transforms
// onto Base* seed fields; identity returns zero seed with bake=false;
// shear forces bake. Per-path animation routing means sibling
// collisions are impossible, so TRS-decomposable always defers.
func TestSeedFromTransform_IdentityDefersShearBakes(t *testing.T) {
	// Identity: no base, no bake.
	id := &VectorPath{Transform: identityTransform}
	seed, bake := seedFromTransform(id)
	if seed.HasBaseXform || bake {
		t.Fatalf("identity: want no base/no bake, got %+v bake=%v",
			seed, bake)
	}

	// Pure translate (10,20) — TRS-decomposable.
	tr := &VectorPath{Transform: [6]float32{1, 0, 0, 1, 10, 20}}
	seed, bake = seedFromTransform(tr)
	if bake || !seed.HasBaseXform {
		t.Fatalf("translate: want deferred base, got bake=%v seed=%+v",
			bake, seed)
	}
	if seed.BaseTransX != 10 || seed.BaseTransY != 20 {
		t.Fatalf("translate base wrong: %+v", seed)
	}

	// Shear matrix is not TRS-decomposable — forces bake.
	sh := &VectorPath{Transform: [6]float32{1, 0.5, 0, 1, 0, 0}}
	seed, bake = seedFromTransform(sh)
	if !bake || seed.HasBaseXform {
		t.Fatalf("shear: want bake, got bake=%v seed=%+v", bake, seed)
	}
}

// Sibling paths with divergent TRS transforms now each defer
// independently — per-PathID animation state ensures no collision.
func TestSeedFromTransform_SiblingsDeferIndependently(t *testing.T) {
	a := &VectorPath{
		PathID:    1,
		GroupID:   "g1",
		Transform: [6]float32{1, 0, 0, 1, 5, 0},
	}
	b := &VectorPath{
		PathID:    2,
		GroupID:   "g1",
		Transform: [6]float32{1, 0, 0, 1, 0, 7},
	}
	sa, bakeA := seedFromTransform(a)
	sb, bakeB := seedFromTransform(b)
	if bakeA || bakeB || !sa.HasBaseXform || !sb.HasBaseXform {
		t.Fatalf("siblings must both defer; a=%+v bakeA=%v b=%+v bakeB=%v",
			sa, bakeA, sb, bakeB)
	}
	if sa.BaseTransX != 5 || sb.BaseTransY != 7 {
		t.Fatalf("sibling bases overwritten: a=%+v b=%+v", sa, sb)
	}
}

// Parsing with non-zero viewBox origin keeps triangle coords in raw
// authored viewBox space; the shift is surfaced through
// parsed.ViewBoxX/Y for the render path to apply as an outer
// translate. A <rect x=10 y=20 w=5 h=5> in viewBox "10 20 32 32"
// therefore tessellates at (10,20)-(15,25), and subtracting
// (ViewBoxX, ViewBoxY) lands it at (0,0)-(5,5).
func TestTessellatePaths_ViewBoxOriginPreservesCoords(t *testing.T) {
	p := New()
	parsed, err := p.ParseSvg(`<svg viewBox="10 20 32 32"
		xmlns="http://www.w3.org/2000/svg">
		<rect x="10" y="20" width="5" height="5" fill="black"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.ViewBoxX != 10 || parsed.ViewBoxY != 20 {
		t.Fatalf("viewBox not propagated: (%v,%v)",
			parsed.ViewBoxX, parsed.ViewBoxY)
	}
	tris := p.Tessellate(parsed, 1)
	if len(tris) == 0 || len(tris[0].Triangles) == 0 {
		t.Fatal("no triangles")
	}
	tp := tris[0]
	if tp.HasBaseXform {
		t.Fatalf("identity author transform must not set base: %+v", tp)
	}
	var minX, minY, maxX, maxY float32 = 1e30, 1e30, -1e30, -1e30
	for i := 0; i+1 < len(tp.Triangles); i += 2 {
		x, y := tp.Triangles[i], tp.Triangles[i+1]
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}
	// Raw viewBox coords: (10,20)-(15,25).
	if minX < 9.9 || minX > 10.1 || minY < 19.9 || minY > 20.1 {
		t.Fatalf("min raw wrong: (%.3f,%.3f)", minX, minY)
	}
	if maxX < 14.9 || maxX > 15.1 || maxY < 24.9 || maxY > 25.1 {
		t.Fatalf("max raw wrong: (%.3f,%.3f)", maxX, maxY)
	}
	// After outer shift: (0,0)-(5,5).
	if minX-parsed.ViewBoxX > 0.1 || minY-parsed.ViewBoxY > 0.1 {
		t.Fatalf("post-shift min drift")
	}
}

// A hostile viewBox with NaN origin must skip the shift so vertices
// retain their authored coords instead of becoming NaN.
func TestTessellatePaths_NaNViewBoxLeavesVerticesFinite(t *testing.T) {
	vg := &VectorGraphic{
		Width: 32, Height: 32,
		ViewBoxX: float32(math.NaN()), ViewBoxY: 0,
		Paths: []VectorPath{{
			Transform: identityTransform,
			FillColor: gui.SvgColor{A: 255},
			Primitive: gui.SvgPrimitive{
				Kind: gui.SvgPrimRect, X: 0, Y: 0, W: 5, H: 5,
			},
			Segments: segmentsForRect(0, 0, 5, 5, 0, 0),
		}},
	}
	tris := vg.tessellatePaths(vg.Paths, 1)
	if len(tris) == 0 || len(tris[0].Triangles) == 0 {
		t.Fatal("no triangles")
	}
	for i, v := range tris[0].Triangles {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("non-finite vertex at %d: %v", i, v)
		}
	}
}
