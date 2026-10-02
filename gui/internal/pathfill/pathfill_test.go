package pathfill

import (
	"math"
	"testing"
)

// areaSum returns the total unsigned area of a flat triangle list.
func areaSum(tris []float32) float32 {
	total := float32(0)
	for i := 0; i+5 < len(tris); i += 6 {
		ax, ay := tris[i], tris[i+1]
		bx, by := tris[i+2], tris[i+3]
		cx, cy := tris[i+4], tris[i+5]
		total += f32Abs((bx-ax)*(cy-ay)-(cx-ax)*(by-ay)) / 2
	}
	return total
}

// covers reports whether (px,py) lies inside any triangle of tris.
func covers(tris []float32, px, py float32) bool {
	for i := 0; i+5 < len(tris); i += 6 {
		if PointInTriangle(px, py,
			tris[i], tris[i+1],
			tris[i+2], tris[i+3],
			tris[i+4], tris[i+5]) {
			return true
		}
	}
	return false
}

// Two overlapping same-winding squares: evenodd carves the overlap
// (2×100 − 2×25 = 150), nonzero fills everything (2×100 − 25 = 175).
func TestAppend_EvenOddVsNonzero(t *testing.T) {
	a := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	b := []float32{5, 5, 15, 5, 15, 15, 5, 15}

	triE := Append(nil, [][]float32{a, b}, RuleEvenOdd, nil)
	if got := areaSum(triE); f32Abs(got-150) > 1 {
		t.Fatalf("evenodd area = %f, want ~150", got)
	}
	triN := Append(nil, [][]float32{a, b}, RuleNonzero, nil)
	if got := areaSum(triN); f32Abs(got-175) > 1 {
		t.Fatalf("nonzero area = %f, want ~175", got)
	}
}

// Outer CCW + concentric inner CW carves a hole under nonzero.
func TestAppend_NonzeroCarvesHole(t *testing.T) {
	outer := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	inner := []float32{3, 3, 3, 7, 7, 7, 7, 3}

	tris := Append(nil, [][]float32{outer, inner}, RuleNonzero, nil)
	if got := areaSum(tris); f32Abs(got-84) > 1 {
		t.Fatalf("area = %f, want ~84", got)
	}
	if covers(tris, 5, 5) {
		t.Fatal("hole center (5,5) must be uncovered")
	}
	if !covers(tris, 1, 1) {
		t.Fatal("outer-only point (1,1) must be covered")
	}
}

// A single nonzero concave contour takes the ear-clip fast path.
func TestAppend_ConcaveSingleContour(t *testing.T) {
	// L shape, area 3.
	l := []float32{0, 0, 2, 0, 2, 1, 1, 1, 1, 2, 0, 2}
	tris := Append(nil, [][]float32{l}, RuleNonzero, nil)
	if got := areaSum(tris); f32Abs(got-3) > 1e-3 {
		t.Fatalf("area = %f, want 3", got)
	}
	if !covers(tris, 0.5, 0.5) || !covers(tris, 0.5, 1.5) {
		t.Fatal("both L arms must be covered")
	}
	if covers(tris, 1.5, 1.5) {
		t.Fatal("missing corner (1.5,1.5) must be uncovered")
	}
}

// Evenodd on a single self-intersecting bowtie routes to scanline
// and carves the crossing: two lobes of 25.
func TestAppend_EvenOddBowtie(t *testing.T) {
	bowtie := []float32{0, 0, 10, 10, 10, 0, 0, 10}
	tris := Append(nil, [][]float32{bowtie}, RuleEvenOdd, nil)
	if got := areaSum(tris); f32Abs(got-50) > 1 {
		t.Fatalf("area = %f, want ~50", got)
	}
	if !covers(tris, 2, 5) || !covers(tris, 8, 5) {
		t.Fatal("both lobes must be covered")
	}
	if covers(tris, 5, 2) || covers(tris, 5, 8) {
		t.Fatal("outside-lobe points must be uncovered")
	}
}

// Empty, short and degenerate contours produce nothing.
func TestAppend_Degenerate(t *testing.T) {
	if out := Append(nil, nil, RuleNonzero, nil); out != nil {
		t.Fatal("nil contours must yield nil")
	}
	if out := Append(nil, [][]float32{}, RuleNonzero, nil); out != nil {
		t.Fatal("empty contours must yield nil")
	}
	short := []float32{0, 0, 1, 1}
	if out := Append(nil, [][]float32{short}, RuleNonzero,
		nil); out != nil {
		t.Fatal("2-point contour must yield nil")
	}
	if out := EarClip(nil); out != nil {
		t.Fatal("nil ear-clip must yield nil")
	}
	nan := float32(math.NaN())
	if out := EarClip([]float32{0, 0, 1, 0, nan, 1}); out != nil {
		t.Fatal("NaN ear-clip must yield nil")
	}
}

// Non-finite vertices never reach the output.
func TestAppend_NaNProducesNoNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	good := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	bad := []float32{20, 20, nan, 25, 25, 25, 25, 20}
	worse := []float32{30, 30, 40, inf, 40, 40, 30, 40}
	tris := Append(nil, [][]float32{good, bad, worse},
		RuleNonzero, nil)
	for i, v := range tris {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("non-finite value at index %d: %v", i, v)
		}
	}
	if len(tris)%6 != 0 {
		t.Fatalf("triangle slice not multiple of 6: %d", len(tris))
	}
}

// A kept scratch stops the workspace allocating across calls.
func TestAppend_ScratchReuse(t *testing.T) {
	a := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	b := []float32{5, 5, 15, 5, 15, 15, 5, 15}
	var s Scratch
	first := Append(nil, [][]float32{a, b}, RuleEvenOdd, &s)
	if len(s.Edges) == 0 || len(s.Ys) == 0 {
		t.Fatal("scratch must retain workspace after a call")
	}
	second := Append(nil, [][]float32{a, b}, RuleEvenOdd, &s)
	if areaSum(first) != areaSum(second) {
		t.Fatal("scratch reuse must not change output")
	}
}

// Flattened curves stay within tolerance and connect endpoints.
func TestFlatten_Curves(t *testing.T) {
	var pts []float32
	FlattenQuad(0, 0, 5, 10, 10, 0, 0.5, &pts)
	if len(pts) < 4 || pts[len(pts)-2] != 10 || pts[len(pts)-1] != 0 {
		t.Fatalf("quad must end at (10,0), got %v", pts)
	}
	// The curve bulges upward; every point stays above the chord.
	for i := 0; i+1 < len(pts); i += 2 {
		if pts[i+1] < -0.6 {
			t.Fatalf("quad point below tolerance: %v", pts[i])
		}
	}

	pts = nil
	FlattenCubic(0, 0, 0, 10, 10, 10, 10, 0, 0.5, &pts)
	if len(pts) < 4 || pts[len(pts)-2] != 10 || pts[len(pts)-1] != 0 {
		t.Fatalf("cubic must end at (10,0), got %v", pts)
	}
}

// Edge helpers: parallel edges never intersect, endpoint touches
// are excluded, and degenerate bounds fall back to 1.
func TestEdges_Helpers(t *testing.T) {
	a := Edge{0, 0, 10, 10, +1}
	b := Edge{1, 0, 11, 10, +1}
	if _, ok := SegmentIntersectionY(a, b, 1e-6); ok {
		t.Fatal("parallel edges must not intersect")
	}
	c := Edge{0, 0, -10, 10, +1}
	if _, ok := SegmentIntersectionY(a, c, 0); ok {
		t.Fatal("shared endpoint must not count as intersection")
	}
	d := Edge{0, 10, 10, 0, +1}
	y, ok := SegmentIntersectionY(a, d, 0)
	if !ok || f32Abs(y-5) > 1e-4 {
		t.Fatalf("X crossing at y≈5, got %v, %v", y, ok)
	}
	if s := EdgesBoundScale(
		[]Edge{{5, 5, 5, 5, +1}}); s != 1 {
		t.Fatalf("degenerate scale = %v, want 1", s)
	}
	edges := BuildEdges([][]float32{{0, 0, 10, 0, 10, 10, 0, 10}})
	// The y=0 and y=10 sides are horizontal and dropped.
	if len(edges) != 2 {
		t.Fatalf("edges = %d, want 2", len(edges))
	}
}
