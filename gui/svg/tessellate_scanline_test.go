package svg

import (
	"math"
	"testing"
)

// Non-finite endpoints must be filtered so NaN/Inf never reach the
// active-edge sort or the GPU vertex buffer.
func TestBuildScanEdges_DropsNonFiniteEndpoints(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	poly := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	bad := []float32{nan, 0, 5, 5, 5, 15}
	worse := []float32{0, 0, inf, 5, 5, 15}
	edges := buildScanEdges([][]float32{poly, bad, worse})
	for _, e := range edges {
		if !finiteF32(e.x0) || !finiteF32(e.y0) ||
			!finiteF32(e.x1) || !finiteF32(e.y1) {
			t.Fatalf("edge with non-finite endpoint slipped through: %+v", e)
		}
	}
	// Clean poly contributes 2 non-horizontal edges (the y=0 and
	// y=10 sides are horizontal → dropped). Bad/worse each keep
	// one survivor edge whose endpoints are both finite. Total 4.
	if len(edges) != 4 {
		t.Fatalf("expected 4 surviving edges, got %d", len(edges))
	}
}

// buildScanEdges must cap at maxScanEdges regardless of input size.
func TestBuildScanEdges_RespectsCap(t *testing.T) {
	// Build enough triangles to overflow the cap. Each triangle
	// contributes 3 non-horizontal edges.
	const tris = 3500 // 10500 > maxScanEdges (8192)
	polys := make([][]float32, 0, tris)
	for i := range tris {
		fy := float32(i)
		polys = append(polys, []float32{
			0, fy, 1, fy + 2, 0, fy + 3,
		})
	}
	edges := buildScanEdges(polys)
	if len(edges) > maxScanEdges {
		t.Fatalf("exceeded cap: got %d edges, cap %d", len(edges), maxScanEdges)
	}
	if len(edges) != maxScanEdges {
		t.Fatalf("expected exactly cap %d, got %d", maxScanEdges, len(edges))
	}
}

// Endpoint-touching crossings must be excluded; only strict
// interior intersections return true.
func TestSegmentIntersectionY_SkipsEndpointTouches(t *testing.T) {
	// Two edges sharing endpoint (0,0). No interior crossing.
	a := scanEdge{x0: 0, y0: 0, x1: 10, y1: 10, sign: +1}
	b := scanEdge{x0: 0, y0: 0, x1: -10, y1: 10, sign: +1}
	if _, ok := segmentIntersectionY(a, b, 0); ok {
		t.Fatal("shared endpoint must not count as intersection")
	}
	// T-junction: b's endpoint lies on a's interior. Still excluded
	// because the crossing is at b's endpoint (s in {0,1}).
	c := scanEdge{x0: 5, y0: 5, x1: 5, y1: 10, sign: +1}
	if _, ok := segmentIntersectionY(a, c, 0); ok {
		t.Fatal("T-junction endpoint must not count")
	}
	// Proper X crossing: interior on both segments.
	d := scanEdge{x0: 0, y0: 10, x1: 10, y1: 0, sign: +1}
	y, ok := segmentIntersectionY(a, d, 0)
	if !ok {
		t.Fatal("proper X crossing must return ok")
	}
	if f32Abs(y-5) > 1e-4 {
		t.Fatalf("expected y≈5 at X crossing, got %f", y)
	}
}

// Parallel (and collinear) segments must be reported as non-
// intersecting via the denEps guard.
func TestSegmentIntersectionY_ParallelRejected(t *testing.T) {
	a := scanEdge{x0: 0, y0: 0, x1: 10, y1: 10, sign: +1}
	b := scanEdge{x0: 1, y0: 0, x1: 11, y1: 10, sign: +1}
	if _, ok := segmentIntersectionY(a, b, 1e-6); ok {
		t.Fatal("parallel edges must not intersect")
	}
	// Collinear.
	c := scanEdge{x0: 2, y0: 2, x1: 8, y1: 8, sign: +1}
	if _, ok := segmentIntersectionY(a, c, 1e-6); ok {
		t.Fatal("collinear edges must not intersect (denEps guard)")
	}
}

// edgesBoundsScale returns max(dx, dy) and falls back to 1 on a
// zero-extent input so downstream epsilons stay finite.
func TestEdgesBoundsScale_DegenerateReturnsOne(t *testing.T) {
	// Single zero-extent edge (all coords equal). Real input would
	// have been filtered upstream, but the helper must still cope.
	e := []scanEdge{{x0: 5, y0: 5, x1: 5, y1: 5, sign: +1}}
	if s := edgesBoundsScale(e); s != 1 {
		t.Fatalf("expected fallback 1 on zero bbox, got %f", s)
	}
	// Non-degenerate: max extent dominates.
	e2 := []scanEdge{
		{x0: 0, y0: 0, x1: 100, y1: 2, sign: +1},
		{x0: 3, y0: 4, x1: 7, y1: 5, sign: +1},
	}
	if s := edgesBoundsScale(e2); f32Abs(s-100) > 1e-4 {
		t.Fatalf("expected scale 100, got %f", s)
	}
}

// Non-finite input must not propagate into tessellated triangles.
// This covers the full buildScanEdges → appendNonDegenTri chain:
// any NaN/Inf in vertex data must be silently dropped rather than
// landed in the GPU vertex buffer.
func TestTessellate_NaNInputProducesNoNonFiniteTriangles(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	good := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	bad := []float32{20, 20, nan, 25, 25, 25, 25, 20}
	worse := []float32{30, 30, 40, inf, 40, 40, 30, 40}
	tris := tessellatePolylines(
		[][]float32{good, bad, worse}, fillRuleNonzero)
	for i, v := range tris {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("non-finite value at index %d: %v", i, v)
		}
	}
	// Clean square alone area 100; surviving carve should stay
	// bounded — we don't pin the exact value, only finiteness.
	if len(tris)%6 != 0 {
		t.Fatalf("triangle slice not multiple of 6: %d", len(tris))
	}
}

// Evenodd on a single self-intersecting (figure-8 / bowtie)
// contour must route through scanlineTessellate and carve the
// crossing, not the ear-clip fast path. Left and right lobes
// each have area 25; total = 50.
func TestTessellate_EvenOddBowtieCarved(t *testing.T) {
	bowtie := []float32{0, 0, 10, 10, 10, 0, 0, 10}
	tris := tessellatePolylines([][]float32{bowtie}, fillRuleEvenOdd)
	got := triangleAreaSum(tris)
	if f32Abs(got-50.0) > 1.0 {
		t.Fatalf("expected evenodd bowtie area ~50, got %f", got)
	}
	// Centre of the X is the crossing point — under evenodd it is
	// a 0-measure boundary. Slightly off-centre in each lobe must
	// be covered; the opposite quadrants must not.
	if !anyTriangleCovers(tris, 2, 5) {
		t.Fatal("left lobe point (2,5) must be covered")
	}
	if !anyTriangleCovers(tris, 8, 5) {
		t.Fatal("right lobe point (8,5) must be covered")
	}
	if anyTriangleCovers(tris, 5, 2) {
		t.Fatal("top/bottom outside-lobe point (5,2) must be uncovered")
	}
	if anyTriangleCovers(tris, 5, 8) {
		t.Fatal("top/bottom outside-lobe point (5,8) must be uncovered")
	}
}
