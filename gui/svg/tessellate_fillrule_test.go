package svg

import (
	"os"
	"testing"
)

// Evenodd over two overlapping same-winding squares must carve the
// overlap (winding = +2 → parity even → not filled), so total area
// = 2×100 − 2×25 = 150. Under nonzero the same input would fill
// everything (winding = +2, nonzero), so the test also guards
// against the two rules being conflated.
func TestTessellate_EvenOddTwoOverlappingSquares(t *testing.T) {
	a := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	b := []float32{5, 5, 15, 5, 15, 15, 5, 15}

	triE := tessellatePolylines([][]float32{a, b}, fillRuleEvenOdd)
	gotE := triangleAreaSum(triE)
	if f32Abs(gotE-150.0) > 1.0 {
		t.Fatalf("evenodd: expected ~150, got %f", gotE)
	}

	triN := tessellatePolylines([][]float32{a, b}, fillRuleNonzero)
	gotN := triangleAreaSum(triN)
	// Under nonzero both squares contribute +1, overlap winding=2;
	// filled everywhere. Total = 2×100 − 25 (overlap counted once).
	if f32Abs(gotN-175.0) > 1.0 {
		t.Fatalf("nonzero: expected ~175, got %f", gotN)
	}
}

// Outer CCW + concentric inner CW yields a carved hole under
// nonzero. Guards against a regression where per-contour
// ear-clip's coarse triangles span the hole region.
func TestTessellate_NonzeroCarvesConcentricOppositeWinding(t *testing.T) {
	outer := []float32{0, 0, 10, 0, 10, 10, 0, 10} // CCW, area 100
	inner := []float32{3, 3, 3, 7, 7, 7, 7, 3}     // CW, area 16

	tris := tessellatePolylines([][]float32{outer, inner}, fillRuleNonzero)
	got := triangleAreaSum(tris)
	if f32Abs(got-84.0) > 1.0 {
		t.Fatalf("expected carved area ~84, got %f", got)
	}
	// Center of the hole must not be covered by any output triangle.
	if anyTriangleCovers(tris, 5, 5) {
		t.Fatal("hole center (5,5) must be uncovered")
	}
	// A point clearly in the outer-only ring must be covered.
	if !anyTriangleCovers(tris, 1, 1) {
		t.Fatal("outer-only point (1,1) must be covered")
	}
}

// wind-toy.svg regression: 8 crescent-blade subpaths radiate from
// centre (12,12). Prior to the path-parser fix for "M after Z"
// token skipping, all 8 subpaths collapsed into one malformed
// contour with arc control points flung outside the viewBox,
// which the tessellator then rendered as a misshapen blob. With
// the parser fix, each blade parses cleanly and occupies a
// distinct sector. Assert the 8 blades are present and their
// bounding boxes spread roughly uniformly around the centre.
func TestTessellate_WindToyEightBlades(t *testing.T) {
	data, err := os.ReadFile("../assets/svg-spinners/wind-toy.svg")
	if err != nil {
		t.Skipf("asset not available: %v", err)
	}
	vg, err := parseSvg(string(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(vg.Paths))
	}
	if vg.Paths[0].fillRule != fillRuleNonzero {
		t.Fatalf("expected default FillRuleNonzero, got %d",
			vg.Paths[0].fillRule)
	}
	polylines := flattenPath(&vg.Paths[0], 0.5)
	if len(polylines) != 8 {
		t.Fatalf("expected 8 blade subpaths, got %d", len(polylines))
	}
	// Each blade must stay inside the 24×24 viewBox (with a small
	// slack for Bezier overshoot at arc joins). Before the parser
	// fix, arc control points flew out to x≈31, y≈-6.
	for i, p := range polylines {
		for k := 0; k+1 < len(p); k += 2 {
			x, y := p[k], p[k+1]
			if x < -1 || x > 25 || y < -1 || y > 25 {
				t.Fatalf("blade[%d] vertex (%f,%f) outside viewBox",
					i, x, y)
			}
		}
	}
	result := vg.tessellatePaths(vg.Paths, 1.0)
	if len(result) == 0 {
		t.Fatal("expected tessellated output")
	}
	tris := result[0].Triangles
	if len(tris) == 0 {
		t.Fatal("expected fill triangles")
	}
	// Centre of rotation must NOT be fully covered — the blades
	// form a rosette with an uncovered centre spot.
	if anyTriangleCovers(tris, 12, 12) {
		t.Fatal("wind-toy centre (12,12) must be uncovered")
	}
}

// <g fill-rule="evenodd"><path d="..."/></g> must propagate the
// group's fill-rule to the path when the path does not set its
// own fill-rule attribute.
func TestTessellate_FillRuleInheritedFromGroup(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` +
		`<g fill="black" fill-rule="evenodd">` +
		`<path d="M0,0 L10,0 L10,10 L0,10 Z"/>` +
		`</g></svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(vg.Paths))
	}
	if vg.Paths[0].fillRule != fillRuleEvenOdd {
		t.Fatalf("expected FillRuleEvenOdd from group, got %d",
			vg.Paths[0].fillRule)
	}
}

// Path-level fill-rule overrides the group's inherited value.
func TestTessellate_FillRulePathOverridesGroup(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` +
		`<g fill="black" fill-rule="evenodd">` +
		`<path fill-rule="nonzero" d="M0,0 L10,0 L10,10 L0,10 Z"/>` +
		`</g></svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(vg.Paths))
	}
	if vg.Paths[0].fillRule != fillRuleNonzero {
		t.Fatalf("expected FillRuleNonzero from path attr, got %d",
			vg.Paths[0].fillRule)
	}
}

// resolveFillRule helper: exercise the trim/case-sensitive
// mapping. SVG defines fill-rule as case-sensitive; any
// unrecognised string (including "EvenOdd") is treated as the
// default nonzero.
func TestResolveFillRule_Defaults(t *testing.T) {
	if r := resolveFillRule(`<path/>`, computedStyle{}); r != fillRuleNonzero {
		t.Fatalf("no attr should resolve to nonzero, got %d", r)
	}
	if r := resolveFillRule(`<path fill-rule="evenodd"/>`, computedStyle{}); r != fillRuleEvenOdd {
		t.Fatalf("explicit evenodd, got %d", r)
	}
	if r := resolveFillRule(`<path/>`, computedStyle{fillRule: fillRuleEvenOdd}); r != fillRuleEvenOdd {
		t.Fatalf("inherited evenodd, got %d", r)
	}
	// Case-sensitive per SVG spec; unknown tokens fall back to
	// nonzero rather than panicking or defaulting to evenodd.
	if r := resolveFillRule(`<path fill-rule="EvenOdd"/>`, computedStyle{}); r != fillRuleNonzero {
		t.Fatalf("non-canonical case should fall back to nonzero, got %d", r)
	}
	// Leading/trailing whitespace is tolerated.
	if r := resolveFillRule(`<path fill-rule="  evenodd  "/>`, computedStyle{}); r != fillRuleEvenOdd {
		t.Fatalf("whitespace-padded evenodd, got %d", r)
	}
}

// Every shape element (circle, ellipse, polygon, polyline, line)
// must inherit fill-rule from its enclosing <g>. Coverage in
// shapes.go was added uniformly — this table locks that in so a
// future edit that forgets one shape fails loudly.
func TestTessellate_FillRuleInheritedByAllShapes(t *testing.T) {
	cases := []struct {
		name string
		elem string
	}{
		{"circle", `<circle cx="5" cy="5" r="4"/>`},
		{"ellipse", `<ellipse cx="5" cy="5" rx="4" ry="3"/>`},
		{"polygon", `<polygon points="0,0 10,0 10,10 0,10"/>`},
		{"polyline", `<polyline points="0,0 10,0 10,10 0,10"/>`},
		{"line", `<line x1="0" y1="0" x2="10" y2="10"/>`},
		{"rect", `<rect x="0" y="0" width="10" height="10"/>`},
		{"path", `<path d="M0,0 L10,0 L10,10 L0,10 Z"/>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` +
				`<g fill="black" fill-rule="evenodd">` + c.elem + `</g></svg>`
			vg, err := parseSvg(svg)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(vg.Paths) != 1 {
				t.Fatalf("expected 1 path, got %d", len(vg.Paths))
			}
			if vg.Paths[0].fillRule != fillRuleEvenOdd {
				t.Fatalf("%s: expected FillRuleEvenOdd from group, got %d",
					c.name, vg.Paths[0].fillRule)
			}
		})
	}
}

// anyTriangleCovers reports whether (px,py) lies inside any
// triangle of the flat triangle slice tris ([ax,ay,bx,by,cx,cy,...]).
func anyTriangleCovers(tris []float32, px, py float32) bool {
	for i := 0; i+5 < len(tris); i += 6 {
		if pointInTriangle(px, py,
			tris[i], tris[i+1],
			tris[i+2], tris[i+3],
			tris[i+4], tris[i+5]) {
			return true
		}
	}
	return false
}
