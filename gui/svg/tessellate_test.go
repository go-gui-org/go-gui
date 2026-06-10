package svg

import (
	"math"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// --- Tier 1: Small pure functions ---

func TestTessellatePolygonAreaCCW(t *testing.T) {
	// Unit square CCW: (0,0) (1,0) (1,1) (0,1) → area > 0
	poly := []float32{0, 0, 1, 0, 1, 1, 0, 1}
	area := polygonArea(poly)
	if area >= 0 {
		t.Fatalf("CCW square should have negative area, got %f", area)
	}
	if f32Abs(area)-1.0 > 1e-6 {
		t.Fatalf("expected |area| = 1, got %f", f32Abs(area))
	}
}

func TestTessellatePolygonAreaCW(t *testing.T) {
	// Unit square CW: (0,0) (0,1) (1,1) (1,0) → area < 0
	poly := []float32{0, 0, 0, 1, 1, 1, 1, 0}
	area := polygonArea(poly)
	if area <= 0 {
		t.Fatalf("CW square should have positive area, got %f", area)
	}
	if f32Abs(area)-1.0 > 1e-6 {
		t.Fatalf("expected |area| = 1, got %f", f32Abs(area))
	}
}

func TestTessellatePolygonAreaTriangle(t *testing.T) {
	// Right triangle: (0,0) (4,0) (0,3) → area = 6
	poly := []float32{0, 0, 4, 0, 0, 3}
	area := polygonArea(poly)
	if f32Abs(f32Abs(area)-6.0) > 1e-5 {
		t.Fatalf("expected |area| = 6, got %f", f32Abs(area))
	}
}

func TestTessellatePointInTriangleInside(t *testing.T) {
	// Point (0.25, 0.25) inside triangle (0,0) (1,0) (0,1)
	if !pointInTriangle(0.25, 0.25, 0, 0, 1, 0, 0, 1) {
		t.Fatal("point should be inside triangle")
	}
}

func TestTessellatePointInTriangleOutside(t *testing.T) {
	if pointInTriangle(2, 2, 0, 0, 1, 0, 0, 1) {
		t.Fatal("point should be outside triangle")
	}
}

func TestTessellatePointInTriangleDegenerate(t *testing.T) {
	// Degenerate triangle (collinear points)
	if pointInTriangle(0.5, 0, 0, 0, 1, 0, 2, 0) {
		t.Fatal("degenerate triangle should return false")
	}
}

func TestTessellatePointInTriangleOnEdge(t *testing.T) {
	_ = t
	// Point on edge: (0.5, 0) on triangle (0,0) (1,0) (0,1)
	// barycentric: vv should be 0, so uu+vv < 1 but vv == 0
	// The function uses strict < 1, so edge points with uu+vv < 1 are inside
	result := pointInTriangle(0.5, 0, 0, 0, 1, 0, 0, 1)
	// On the AB edge, vv=0, uu depends on exact computation.
	// Just verify no panic.
	_ = result
}

func TestTessellateBboxFromTriangles(t *testing.T) {
	tris := []float32{1, 2, 5, 8, 3, 4, -1, 0, 10, 6, 7, 9}
	minX, minY, maxX, maxY := bboxFromTriangles(tris)
	if minX != -1 || minY != 0 || maxX != 10 || maxY != 9 {
		t.Fatalf("bbox = (%f,%f,%f,%f), want (-1,0,10,9)",
			minX, minY, maxX, maxY)
	}
}

func TestTessellateBboxFromTrianglesEmpty(t *testing.T) {
	minX, minY, maxX, maxY := bboxFromTriangles(nil)
	if minX != 0 || minY != 0 || maxX != 0 || maxY != 0 {
		t.Fatalf("empty bbox should be all zeros, got (%f,%f,%f,%f)",
			minX, minY, maxX, maxY)
	}
}

func TestTessellateBboxFromTrianglesSinglePoint(t *testing.T) {
	tris := []float32{3, 7}
	minX, minY, maxX, maxY := bboxFromTriangles(tris)
	if minX != 3 || minY != 7 || maxX != 3 || maxY != 7 {
		t.Fatalf("single point bbox = (%f,%f,%f,%f), want (3,7,3,7)",
			minX, minY, maxX, maxY)
	}
}

// --- Tier 2: Core algorithms ---

func TestTessellateEarClipTriangle(t *testing.T) {
	// Already a triangle — returned as-is
	poly := []float32{0, 0, 1, 0, 0, 1}
	tris := earClip(poly)
	if len(tris) != 6 {
		t.Fatalf("expected 6 floats, got %d", len(tris))
	}
}

func TestTessellateEarClipSquare(t *testing.T) {
	// CW square → 2 triangles = 12 floats
	poly := []float32{0, 0, 0, 1, 1, 1, 1, 0}
	tris := earClip(poly)
	if len(tris) != 12 {
		t.Fatalf("expected 12 floats (2 tris), got %d", len(tris))
	}
	verifyTriangleAreaSum(t, tris, 1.0)
}

func TestTessellateEarClipConcave(t *testing.T) {
	// L-shaped concave polygon (6 vertices)
	// (0,0) (2,0) (2,1) (1,1) (1,2) (0,2)
	poly := []float32{0, 0, 0, 2, 1, 2, 1, 1, 2, 1, 2, 0}
	tris := earClip(poly)
	// 6 verts → 4 triangles = 24 floats
	if len(tris) != 24 {
		t.Fatalf("expected 24 floats (4 tris), got %d", len(tris))
	}
	verifyTriangleAreaSum(t, tris, 3.0)
}

func TestTessellateEarClipTooFewVerts(t *testing.T) {
	if tris := earClip([]float32{0, 0, 1, 1}); tris != nil {
		t.Fatalf("< 3 verts should return nil, got %v", tris)
	}
	if tris := earClip(nil); tris != nil {
		t.Fatalf("nil input should return nil")
	}
}

func TestTessellateEarClipClosedDuplicate(t *testing.T) {
	// Triangle with closing duplicate vertex
	poly := []float32{0, 0, 1, 0, 0, 1, 0, 0}
	tris := earClip(poly)
	if len(tris) != 6 {
		t.Fatalf("expected 6 floats (1 tri), got %d", len(tris))
	}
}

func TestTessellateTessellatePolylinesEmpty(t *testing.T) {
	if tris := tessellatePolylines(nil, FillRuleNonzero); tris != nil {
		t.Fatalf("nil polylines should return nil")
	}
	if tris := tessellatePolylines([][]float32{}, FillRuleNonzero); tris != nil {
		t.Fatalf("empty polylines should return nil")
	}
}

func TestTessellateTessellatePolylinesSingle(t *testing.T) {
	poly := []float32{0, 0, 1, 0, 0, 1}
	tris := tessellatePolylines([][]float32{poly}, FillRuleNonzero)
	if len(tris) != 6 {
		t.Fatalf("expected 6 floats, got %d", len(tris))
	}
}

func TestTessellateTessellatePolylinesWithHole(t *testing.T) {
	// Outer square (CCW, positive area = 100).
	outer := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	// Inner square with OPPOSITE winding (CW, negative area = -16).
	// Under the default SVG nonzero fill-rule this is a real hole;
	// total filled area should be outer - hole = 84.
	hole := []float32{3, 3, 3, 7, 7, 7, 7, 3}
	tris := tessellatePolylines([][]float32{outer, hole}, FillRuleNonzero)
	if len(tris) == 0 {
		t.Fatal("expected triangles from polygon with hole")
	}
	totalArea := triangleAreaSum(tris)
	if f32Abs(totalArea-84.0) > 1.0 {
		t.Fatalf("expected area ~84, got %f", totalArea)
	}
}

func TestTessellateTessellatePolylinesSameWindingSeparateRegions(t *testing.T) {
	// Two non-overlapping squares with the same CCW winding. Under
	// nonzero they're independent filled regions, not outer + hole.
	a := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	b := []float32{20, 20, 30, 20, 30, 30, 20, 30}
	tris := tessellatePolylines([][]float32{a, b}, FillRuleNonzero)
	if len(tris) == 0 {
		t.Fatal("expected triangles from two same-winding contours")
	}
	totalArea := triangleAreaSum(tris)
	if f32Abs(totalArea-200.0) > 1.0 {
		t.Fatalf("expected combined area ~200, got %f", totalArea)
	}
}

// TestTessellatePathsStrokeWidthViewBoxUnits locks the semantics
// that tessellatePaths passes StrokeWidth through in viewBox units.
// Pre-scaling the width here (old behavior) combined with render-
// side vertex scaling produced width*scale² on screen; the fix
// relies on stroke triangles being identical across scale values.
func TestTessellatePathsStrokeWidthViewBoxUnits(t *testing.T) {
	mkPath := func() VectorPath {
		return VectorPath{
			Transform: identityTransform,
			Segments: []PathSegment{
				{Cmd: CmdMoveTo, Points: []float32{0, 0}},
				{Cmd: CmdLineTo, Points: []float32{10, 0}},
			},
			StrokeColor: gui.SvgColor{R: 0, G: 0, B: 0, A: 255},
			StrokeWidth: 4,
			StrokeCap:   gui.SvgButtCap,
			StrokeJoin:  gui.SvgMiterJoin,
			Opacity:     1,
		}
	}
	vg := &VectorGraphic{}
	a := mkPath()
	b := mkPath()
	lo := vg.tessellatePaths([]VectorPath{a}, 1)
	hi := vg.tessellatePaths([]VectorPath{b}, 100)
	if len(lo) != 1 || len(hi) != 1 {
		t.Fatalf("expected one stroke path each, got lo=%d hi=%d",
			len(lo), len(hi))
	}
	if len(lo[0].Triangles) != len(hi[0].Triangles) {
		t.Fatalf("triangle counts differ across scale: lo=%d hi=%d",
			len(lo[0].Triangles), len(hi[0].Triangles))
	}
	for i, v := range lo[0].Triangles {
		if f32Abs(v-hi[0].Triangles[i]) > 1e-4 {
			t.Fatalf("vertex %d differs: lo=%f hi=%f", i, v, hi[0].Triangles[i])
		}
	}
}

func TestTessellateTessellatePolylinesShortContour(t *testing.T) {
	// Contour with < 3 vertices should be skipped
	short := []float32{0, 0, 1, 1}
	tris := tessellatePolylines([][]float32{short}, FillRuleNonzero)
	if tris != nil {
		t.Fatalf("short contour should return nil, got %d floats", len(tris))
	}
}

// Unattached opposite-winding subpath (not bbox-contained by any
// region) is promoted to its own independent filled region rather
// than force-merged into regions[0] via mergeHole. This matches the
// nonzero fill-rule for peer subpaths with mixed windings (e.g.
// radial pinwheels) where bridging corrupts the mesh.
func TestTessellatePolylines_UnattachedHolePromoted(t *testing.T) {
	outer := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	// Opposite-winding square located far outside outer.
	stray := []float32{100, 100, 100, 104, 104, 104, 104, 100}
	tris := tessellatePolylines([][]float32{outer, stray}, FillRuleNonzero)
	if len(tris) == 0 {
		t.Fatal("expected triangulation")
	}
	got := triangleAreaSum(tris)
	// Outer=100, stray=16, total=116.
	if f32Abs(got-116.0) > 1.0 {
		t.Fatalf("expected area ~116 (stray promoted), got %f", got)
	}
}

// Two overlapping peer subpaths with opposite windings (e.g.
// rotated pinwheel blades on wind-toy) must carve at the overlap
// under the SVG nonzero fill-rule. Each 10×10 square has area 100
// and the 5×5 overlap cancels out (winding sum = 0), leaving
// 100 + 100 − 2×25 = 150.
func TestTessellatePolylines_PeerOverlapNonzeroCarved(t *testing.T) {
	ccw := []float32{0, 0, 10, 0, 10, 10, 0, 10}
	cw := []float32{5, 5, 5, 15, 15, 15, 15, 5}
	tris := tessellatePolylines([][]float32{ccw, cw}, FillRuleNonzero)
	if len(tris) == 0 {
		t.Fatal("expected triangulation")
	}
	got := triangleAreaSum(tris)
	if f32Abs(got-150.0) > 1.0 {
		t.Fatalf("expected nonzero-carved area ~150, got %f", got)
	}
}

func TestTessellateFlattenPathLineTo(t *testing.T) {
	path := &VectorPath{
		Transform: identityTransform,
		Segments: []PathSegment{
			{Cmd: CmdMoveTo, Points: []float32{0, 0}},
			{Cmd: CmdLineTo, Points: []float32{10, 0}},
			{Cmd: CmdLineTo, Points: []float32{10, 10}},
		},
	}
	polys := flattenPath(path, 0.5)
	if len(polys) != 1 {
		t.Fatalf("expected 1 polyline, got %d", len(polys))
	}
	// 3 points = 6 floats
	if len(polys[0]) != 6 {
		t.Fatalf("expected 6 floats, got %d", len(polys[0]))
	}
}

func TestTessellateFlattenPathQuadTo(t *testing.T) {
	path := &VectorPath{
		Transform: identityTransform,
		Segments: []PathSegment{
			{Cmd: CmdMoveTo, Points: []float32{0, 0}},
			{Cmd: CmdQuadTo, Points: []float32{5, 10, 10, 0}},
		},
	}
	polys := flattenPath(path, 0.5)
	if len(polys) != 1 {
		t.Fatalf("expected 1 polyline, got %d", len(polys))
	}
	// Quadratic curve should produce multiple segments
	if len(polys[0]) < 6 {
		t.Fatalf("expected multiple points from quad, got %d floats",
			len(polys[0]))
	}
	// Endpoint should be (10, 0)
	n := len(polys[0])
	endX, endY := polys[0][n-2], polys[0][n-1]
	if f32Abs(endX-10) > 0.01 || f32Abs(endY) > 0.01 {
		t.Fatalf("quad endpoint = (%f,%f), want (10,0)", endX, endY)
	}
}

func TestTessellateFlattenPathCubicTo(t *testing.T) {
	path := &VectorPath{
		Transform: identityTransform,
		Segments: []PathSegment{
			{Cmd: CmdMoveTo, Points: []float32{0, 0}},
			{Cmd: CmdCubicTo, Points: []float32{3, 10, 7, 10, 10, 0}},
		},
	}
	polys := flattenPath(path, 0.5)
	if len(polys) != 1 {
		t.Fatalf("expected 1 polyline, got %d", len(polys))
	}
	if len(polys[0]) < 8 {
		t.Fatalf("expected multiple points from cubic, got %d floats",
			len(polys[0]))
	}
	n := len(polys[0])
	endX, endY := polys[0][n-2], polys[0][n-1]
	if f32Abs(endX-10) > 0.01 || f32Abs(endY) > 0.01 {
		t.Fatalf("cubic endpoint = (%f,%f), want (10,0)", endX, endY)
	}
}

func TestTessellateFlattenPathClose(t *testing.T) {
	path := &VectorPath{
		Transform: identityTransform,
		Segments: []PathSegment{
			{Cmd: CmdMoveTo, Points: []float32{0, 0}},
			{Cmd: CmdLineTo, Points: []float32{10, 0}},
			{Cmd: CmdLineTo, Points: []float32{10, 10}},
			{Cmd: CmdClose},
		},
	}
	polys := flattenPath(path, 0.5)
	if len(polys) != 1 {
		t.Fatalf("expected 1 polyline, got %d", len(polys))
	}
	// Closed path: moveTo + 2 lineTo + close back to start = 4 points
	p := polys[0]
	n := len(p)
	// Last point should equal first (closed)
	if p[n-2] != p[0] || p[n-1] != p[1] {
		t.Fatalf("closed path: last (%f,%f) != first (%f,%f)",
			p[n-2], p[n-1], p[0], p[1])
	}
}

func TestTessellateFlattenPathTransform(t *testing.T) {
	// Scale 2x: [2,0,0,2,0,0]
	path := &VectorPath{
		Transform: [6]float32{2, 0, 0, 2, 0, 0},
		Segments: []PathSegment{
			{Cmd: CmdMoveTo, Points: []float32{5, 5}},
			{Cmd: CmdLineTo, Points: []float32{10, 10}},
		},
	}
	polys := flattenPath(path, 0.5)
	if len(polys) != 1 {
		t.Fatalf("expected 1 polyline, got %d", len(polys))
	}
	p := polys[0]
	if f32Abs(p[0]-10) > 0.01 || f32Abs(p[1]-10) > 0.01 {
		t.Fatalf("start = (%f,%f), want (10,10)", p[0], p[1])
	}
	if f32Abs(p[2]-20) > 0.01 || f32Abs(p[3]-20) > 0.01 {
		t.Fatalf("end = (%f,%f), want (20,20)", p[2], p[3])
	}
}

func TestTessellateFlattenPathMultipleSubpaths(t *testing.T) {
	path := &VectorPath{
		Transform: identityTransform,
		Segments: []PathSegment{
			{Cmd: CmdMoveTo, Points: []float32{0, 0}},
			{Cmd: CmdLineTo, Points: []float32{1, 0}},
			{Cmd: CmdLineTo, Points: []float32{1, 1}},
			{Cmd: CmdMoveTo, Points: []float32{5, 5}},
			{Cmd: CmdLineTo, Points: []float32{6, 5}},
			{Cmd: CmdLineTo, Points: []float32{6, 6}},
		},
	}
	polys := flattenPath(path, 0.5)
	if len(polys) != 2 {
		t.Fatalf("expected 2 polylines, got %d", len(polys))
	}
}

func TestTessellateApplyDasharrayBasic(t *testing.T) {
	// Horizontal line from (0,0) to (10,0), dash=[3,2]
	poly := []float32{0, 0, 10, 0}
	result := applyDasharray([][]float32{poly}, []float32{3, 2}, 0)
	// Expected dashes: 0-3 (draw), 3-5 (gap), 5-8 (draw), 8-10 (gap)
	if len(result) != 2 {
		t.Fatalf("expected 2 dash segments, got %d", len(result))
	}
	// First dash: (0,0)→(3,0)
	d0 := result[0]
	if f32Abs(d0[0]) > 0.01 || f32Abs(d0[2]-3) > 0.01 {
		t.Fatalf("first dash = (%f,%f)→(%f,%f), want (0,0)→(3,0)",
			d0[0], d0[1], d0[2], d0[3])
	}
	// Second dash: (5,0)→(8,0)
	d1 := result[1]
	if f32Abs(d1[0]-5) > 0.01 || f32Abs(d1[2]-8) > 0.01 {
		t.Fatalf("second dash = (%f,%f)→(%f,%f), want (5,0)→(8,0)",
			d1[0], d1[1], d1[2], d1[3])
	}
}

func TestTessellateApplyDasharrayEmpty(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	polys := [][]float32{poly}
	result := applyDasharray(polys, nil, 0)
	if len(result) != 1 {
		t.Fatalf("empty dasharray should return input, got %d", len(result))
	}
}

func TestTessellateApplyDasharrayShortSegment(t *testing.T) {
	// Polyline shorter than one dash
	poly := []float32{0, 0, 1, 0}
	result := applyDasharray([][]float32{poly}, []float32{5, 5}, 0)
	if len(result) != 1 {
		t.Fatalf("expected 1 dash segment, got %d", len(result))
	}
}

func TestTessellateApplyDasharrayTooShort(t *testing.T) {
	// Polyline with < 4 floats is skipped
	poly := []float32{0, 0}
	result := applyDasharray([][]float32{poly}, []float32{3, 2}, 0)
	if len(result) != 0 {
		t.Fatalf("expected 0 dash segments from short poly, got %d",
			len(result))
	}
}

// All-zero dasharray cycle must short-circuit to "solid" rather than
// loop forever. Guards against author error and DoS.
func TestApplyDasharray_AllZeroCycleReturnsSolid(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	result := applyDasharray([][]float32{poly}, []float32{0, 0}, 0)
	if len(result) != 1 || len(result[0]) != 4 {
		t.Fatalf("zero-cycle should return input unchanged; got %v", result)
	}
}

// Sub-epsilon cycle must short-circuit to "solid" — guards inner
// consume loop from running segLen / cycleLen iterations.
func TestApplyDasharray_NearZeroCycleReturnsSolid(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	tiny := []float32{1e-9, 1e-9}
	result := applyDasharray([][]float32{poly}, tiny, 0)
	if len(result) != 1 {
		t.Fatalf("expected solid passthrough; got %d segments", len(result))
	}
}

// NaN entries → cycleLen NaN → must short-circuit to solid.
func TestApplyDasharray_NaNDasharrayReturnsSolid(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	nan := float32(math.NaN())
	result := applyDasharray([][]float32{poly}, []float32{nan, 5}, 0)
	if len(result) != 1 {
		t.Fatalf("NaN dash should solid-passthrough; got %d", len(result))
	}
}

// Inf entries → cycleLen +Inf → must short-circuit to solid.
func TestApplyDasharray_InfDasharrayReturnsSolid(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	inf := float32(math.Inf(1))
	result := applyDasharray([][]float32{poly}, []float32{inf, 1}, 0)
	if len(result) != 1 {
		t.Fatalf("Inf dash should solid-passthrough; got %d", len(result))
	}
}

// Pathological micro-cycle (just above minDashCycleLen) over a long
// segment must terminate via maxDashIterPerPoly cap, not stall.
func TestApplyDasharray_PathologicalCycleCapsIters(t *testing.T) {
	poly := []float32{0, 0, 1e6, 0}
	dash := []float32{1.5e-3, 1.5e-3} // cycle ≈ 3e-3, > minDashCycleLen
	done := make(chan struct{})
	go func() {
		applyDasharray([][]float32{poly}, dash, 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("applyDasharray did not terminate within 1s")
	}
}

// dashPhase: positive offset advances forward through the cycle.
func TestDashPhase_PositiveOffsetSkipsIntoGap(t *testing.T) {
	idx, drawing, remaining := dashPhase([]float32{3, 2}, 4, 5)
	// offset 4: skip first 3 of [3,2]; remaining 1 of gap.
	if idx != 1 || drawing {
		t.Fatalf("idx=%d drawing=%v want (1,false)", idx, drawing)
	}
	if remaining < 0.99 || remaining > 1.01 {
		t.Fatalf("remaining=%v want 1", remaining)
	}
}

// dashPhase: negative offset wraps via cycle length modulo.
func TestDashPhase_NegativeOffsetWraps(t *testing.T) {
	idx, drawing, remaining := dashPhase([]float32{3, 2}, -4, 5)
	// -4 mod 5 = 1; skip 1 of first dash; still in dash[0].
	if idx != 0 || !drawing {
		t.Fatalf("idx=%d drawing=%v want (0,true)", idx, drawing)
	}
	if remaining < 1.99 || remaining > 2.01 {
		t.Fatalf("remaining=%v want 2", remaining)
	}
}

// dashPhase: leading zero entry must be skipped so [0,150] starts in
// the gap rather than emitting a degenerate zero-length dash.
func TestDashPhase_ZeroLeadingEntrySkipped(t *testing.T) {
	idx, drawing, remaining := dashPhase([]float32{0, 150}, 0, 150)
	if idx != 1 || drawing {
		t.Fatalf("idx=%d drawing=%v want (1,false)", idx, drawing)
	}
	if remaining != 150 {
		t.Fatalf("remaining=%v want 150", remaining)
	}
}

// dashPhase: NaN offset degrades to zero-offset.
func TestDashPhase_NaNOffsetDegrades(t *testing.T) {
	nan := float32(math.NaN())
	idx, drawing, remaining := dashPhase([]float32{3, 2}, nan, 5)
	if idx != 0 || !drawing || remaining != 3 {
		t.Fatalf("idx=%d drawing=%v rem=%v want (0,true,3)",
			idx, drawing, remaining)
	}
}

// dashPhase: Inf offset degrades to zero-offset.
func TestDashPhase_InfOffsetDegrades(t *testing.T) {
	inf := float32(math.Inf(1))
	idx, drawing, remaining := dashPhase([]float32{3, 2}, inf, 5)
	if idx != 0 || !drawing || remaining != 3 {
		t.Fatalf("idx=%d drawing=%v rem=%v want (0,true,3)",
			idx, drawing, remaining)
	}
}

// Shear-bake fallback: skewed transform fails decomposition, so
// vertices bake the matrix and HasBaseXform stays false.
func TestTessellate_ShearBakesIntoVerticesNoBase(t *testing.T) {
	// skewX(45): a=1,b=0,c=1,d=1,e=0,f=0. Not pure TRS.
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{
				Transform: [6]float32{1, 0, 1, 1, 0, 0},
				FillColor: gui.SvgColor{R: 255, A: 255},
				Opacity:   1,
				Segments: []PathSegment{
					{Cmd: CmdMoveTo, Points: []float32{0, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 10}},
					{Cmd: CmdClose},
				},
			},
		},
	}
	out := vg.tessellatePaths(vg.Paths, 1)
	if len(out) != 1 {
		t.Fatalf("paths=%d want 1", len(out))
	}
	if out[0].HasBaseXform {
		t.Fatal("shear must bake; HasBaseXform should be false")
	}
	// Sheared verts: x ranges include c*y skew. (10,10) → x=20.
	maxX := float32(-1)
	for i := 0; i+1 < len(out[0].Triangles); i += 2 {
		if x := out[0].Triangles[i]; x > maxX {
			maxX = x
		}
	}
	if maxX < 19 {
		t.Fatalf("expected baked skew (maxX≥19); got %v", maxX)
	}
}

// appendDegeneratePlaceholders: stroke-only animated primitive emits
// a stroke placeholder, no fill placeholder.
func TestAppendDegenerate_StrokeOnly(t *testing.T) {
	p := &VectorPath{
		Animated:    true,
		Primitive:   gui.SvgPrimitive{Kind: gui.SvgPrimCircle},
		StrokeColor: gui.SvgColor{R: 255, A: 255},
		StrokeWidth: 2,
		GroupID:     "g",
	}
	out := appendDegeneratePlaceholders(nil, p,
		gui.TessellatedPath{PathID: 1})
	if len(out) != 1 {
		t.Fatalf("want 1 placeholder; got %d", len(out))
	}
	if !out[0].IsStroke {
		t.Fatal("expected stroke placeholder")
	}
	if len(out[0].Triangles) != 0 {
		t.Fatalf("expected zero triangles; got %d", len(out[0].Triangles))
	}
}

// appendDegeneratePlaceholders: no paint at all → still emits one
// fill placeholder so the animation system sees the path.
func TestAppendDegenerate_NoPaintForcesFillPlaceholder(t *testing.T) {
	p := &VectorPath{
		Animated:  true,
		Primitive: gui.SvgPrimitive{Kind: gui.SvgPrimCircle},
		GroupID:   "g",
	}
	out := appendDegeneratePlaceholders(nil, p,
		gui.TessellatedPath{PathID: 1})
	if len(out) != 1 {
		t.Fatalf("want 1 placeholder; got %d", len(out))
	}
	if out[0].IsStroke {
		t.Fatal("forced placeholder should be fill")
	}
}

// appendDegeneratePlaceholders: fill + stroke both set → two
// placeholders, fill first then stroke.
func TestAppendDegenerate_FillAndStroke(t *testing.T) {
	p := &VectorPath{
		Animated:    true,
		Primitive:   gui.SvgPrimitive{Kind: gui.SvgPrimCircle},
		FillColor:   gui.SvgColor{R: 255, A: 255},
		StrokeColor: gui.SvgColor{B: 255, A: 255},
		StrokeWidth: 1,
		GroupID:     "g",
	}
	out := appendDegeneratePlaceholders(nil, p,
		gui.TessellatedPath{PathID: 1, HasBaseXform: true})
	if len(out) != 2 || out[0].IsStroke || !out[1].IsStroke {
		t.Fatalf("want fill,stroke order; got %+v", out)
	}
	if !out[0].HasBaseXform || !out[1].HasBaseXform {
		t.Fatal("HasBaseXform should propagate to both placeholders")
	}
}

// --- Tier 3: Gradient support ---

func TestTessellateInterpolateGradientNoStops(t *testing.T) {
	c := interpolateGradient(nil, 0.5)
	if c.A != 255 {
		t.Fatalf("no stops: expected A=255, got A=%d", c.A)
	}
}

func TestTessellateInterpolateGradientSingleStop(t *testing.T) {
	stops := []gui.SvgGradientStop{
		{Offset: 0, Color: gui.SvgColor{R: 100, G: 200, B: 50, A: 255}},
	}
	c := interpolateGradient(stops, 0.5)
	if c.R != 100 || c.G != 200 || c.B != 50 {
		t.Fatalf("single stop: got (%d,%d,%d), want (100,200,50)",
			c.R, c.G, c.B)
	}
}

func TestTessellateInterpolateGradientMidpoint(t *testing.T) {
	stops := []gui.SvgGradientStop{
		{Offset: 0, Color: gui.SvgColor{R: 0, G: 0, B: 0, A: 255}},
		{Offset: 1, Color: gui.SvgColor{R: 200, G: 100, B: 50, A: 255}},
	}
	c := interpolateGradient(stops, 0.5)
	if c.R != 100 || c.G != 50 || c.B != 25 {
		t.Fatalf("midpoint: got (%d,%d,%d), want (100,50,25)",
			c.R, c.G, c.B)
	}
}

func TestTessellateInterpolateGradientBeyondRange(t *testing.T) {
	stops := []gui.SvgGradientStop{
		{Offset: 0.2, Color: gui.SvgColor{R: 10, A: 255}},
		{Offset: 0.8, Color: gui.SvgColor{R: 200, A: 255}},
	}
	// Before first stop
	c := interpolateGradient(stops, 0.0)
	if c.R != 10 {
		t.Fatalf("before first: got R=%d, want 10", c.R)
	}
	// After last stop
	c = interpolateGradient(stops, 1.0)
	if c.R != 200 {
		t.Fatalf("after last: got R=%d, want 200", c.R)
	}
}

func TestTessellateInterpolateGradientThreeStops(t *testing.T) {
	stops := []gui.SvgGradientStop{
		{Offset: 0, Color: gui.SvgColor{R: 0, A: 255}},
		{Offset: 0.5, Color: gui.SvgColor{R: 100, A: 255}},
		{Offset: 1, Color: gui.SvgColor{R: 200, A: 255}},
	}
	// t=0.25 → between stop 0 and 1, midpoint → R=50
	c := interpolateGradient(stops, 0.25)
	if c.R != 50 {
		t.Fatalf("t=0.25: got R=%d, want 50", c.R)
	}
	// t=0.75 → between stop 1 and 2, midpoint → R=150
	c = interpolateGradient(stops, 0.75)
	if c.R != 150 {
		t.Fatalf("t=0.75: got R=%d, want 150", c.R)
	}
}

func TestTessellateProjectOntoGradientBasic(t *testing.T) {
	g := gui.SvgGradientDef{X1: 0, Y1: 0, X2: 10, Y2: 0}
	// Midpoint
	val := projectOntoGradient(5, 0, g)
	if f32Abs(val-0.5) > 1e-5 {
		t.Fatalf("midpoint: got %f, want 0.5", val)
	}
	// Before start → clamped to 0
	val = projectOntoGradient(-5, 0, g)
	if val != 0 {
		t.Fatalf("before start: got %f, want 0", val)
	}
	// After end → clamped to 1
	val = projectOntoGradient(15, 0, g)
	if val != 1 {
		t.Fatalf("after end: got %f, want 1", val)
	}
}

func TestTessellateProjectOntoGradientZeroLength(t *testing.T) {
	g := gui.SvgGradientDef{X1: 5, Y1: 5, X2: 5, Y2: 5}
	val := projectOntoGradient(5, 5, g)
	if val != 0 {
		t.Fatalf("zero-length gradient: got %f, want 0", val)
	}
}

func TestTessellateProjectOntoGradientDiagonal(t *testing.T) {
	g := gui.SvgGradientDef{X1: 0, Y1: 0, X2: 10, Y2: 10}
	// Point at (5,5) is at t=0.5
	val := projectOntoGradient(5, 5, g)
	if f32Abs(val-0.5) > 1e-5 {
		t.Fatalf("diagonal midpoint: got %f, want 0.5", val)
	}
	// Point perpendicular to gradient line at midpoint
	// (10, 0) projects to t=0.5 on the (0,0)→(10,10) line
	val = projectOntoGradient(10, 0, g)
	if f32Abs(val-0.5) > 1e-5 {
		t.Fatalf("perpendicular: got %f, want 0.5", val)
	}
}

func TestTessellateResolveGradient(t *testing.T) {
	g := gui.SvgGradientDef{
		X1: 0, Y1: 0, X2: 1, Y2: 1,
		GradientUnits: "objectBoundingBox",
		Stops: []gui.SvgGradientStop{
			{Offset: 0, Color: gui.SvgColor{R: 255, A: 255}},
		},
	}
	resolved := resolveGradient(g, 10, 20, 110, 220)
	if resolved.X1 != 10 || resolved.Y1 != 20 {
		t.Fatalf("resolved start = (%f,%f), want (10,20)",
			resolved.X1, resolved.Y1)
	}
	if resolved.X2 != 110 || resolved.Y2 != 220 {
		t.Fatalf("resolved end = (%f,%f), want (110,220)",
			resolved.X2, resolved.Y2)
	}
	if resolved.GradientUnits != "userSpaceOnUse" {
		t.Fatalf("units = %q, want userSpaceOnUse", resolved.GradientUnits)
	}
	if len(resolved.Stops) != 1 {
		t.Fatalf("stops not preserved: got %d", len(resolved.Stops))
	}
}

func TestTessellateResolveGradientPartial(t *testing.T) {
	// Gradient from (0.25, 0.25) to (0.75, 0.75) in a 100x200 bbox
	g := gui.SvgGradientDef{X1: 0.25, Y1: 0.25, X2: 0.75, Y2: 0.75}
	resolved := resolveGradient(g, 0, 0, 100, 200)
	if f32Abs(resolved.X1-25) > 0.01 || f32Abs(resolved.Y1-50) > 0.01 {
		t.Fatalf("start = (%f,%f), want (25,50)", resolved.X1, resolved.Y1)
	}
	if f32Abs(resolved.X2-75) > 0.01 || f32Abs(resolved.Y2-150) > 0.01 {
		t.Fatalf("end = (%f,%f), want (75,150)", resolved.X2, resolved.Y2)
	}
}

// --- Tier 4: Integration ---

func TestTessellateTessellatePathsFilled(t *testing.T) {
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{
				Transform: identityTransform,
				FillColor: gui.SvgColor{R: 255, A: 255},
				Opacity:   1,
				Segments: []PathSegment{
					{Cmd: CmdMoveTo, Points: []float32{0, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 10}},
					{Cmd: CmdClose},
				},
			},
		},
	}
	result := vg.tessellatePaths(vg.Paths, 1.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 tessellated path, got %d", len(result))
	}
	if len(result[0].Triangles) < 6 {
		t.Fatalf("expected at least 1 triangle, got %d floats",
			len(result[0].Triangles))
	}
	if result[0].Color.R != 255 || result[0].Color.A != 255 {
		t.Fatalf("color mismatch: got %+v", result[0].Color)
	}
}

func TestTessellateTessellatePathsStroked(t *testing.T) {
	vg := &VectorGraphic{
		Paths: []VectorPath{
			{
				Transform:   identityTransform,
				StrokeColor: gui.SvgColor{G: 255, A: 255},
				StrokeWidth: 2,
				Opacity:     1,
				Segments: []PathSegment{
					{Cmd: CmdMoveTo, Points: []float32{0, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 0}},
				},
			},
		},
	}
	result := vg.tessellatePaths(vg.Paths, 1.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 tessellated path, got %d", len(result))
	}
	if len(result[0].Triangles) == 0 {
		t.Fatal("expected stroke triangles")
	}
	if result[0].Color.G != 255 {
		t.Fatalf("stroke color: got %+v", result[0].Color)
	}
}

func TestTessellateTessellatePathsClipped(t *testing.T) {
	vg := &VectorGraphic{
		ClipPaths: map[string][]VectorPath{
			"clip1": {
				{
					Transform: identityTransform,
					Segments: []PathSegment{
						{Cmd: CmdMoveTo, Points: []float32{0, 0}},
						{Cmd: CmdLineTo, Points: []float32{20, 0}},
						{Cmd: CmdLineTo, Points: []float32{20, 20}},
						{Cmd: CmdClose},
					},
				},
			},
		},
		Paths: []VectorPath{
			{
				Transform:  identityTransform,
				FillColor:  gui.SvgColor{R: 128, A: 255},
				ClipPathID: "clip1",
				Opacity:    1,
				Segments: []PathSegment{
					{Cmd: CmdMoveTo, Points: []float32{0, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 10}},
					{Cmd: CmdClose},
				},
			},
		},
	}
	result := vg.tessellatePaths(vg.Paths, 1.0)
	// Should have clip mask + filled path = 2
	if len(result) != 2 {
		t.Fatalf("expected 2 tessellated paths (clip + fill), got %d",
			len(result))
	}
	if !result[0].IsClipMask {
		t.Fatal("first path should be clip mask")
	}
	if result[0].ClipGroup == 0 {
		t.Fatal("clip group should be non-zero")
	}
	if result[1].ClipGroup != result[0].ClipGroup {
		t.Fatal("clipped path should share clip group with mask")
	}
}

func TestTessellateTessellatePathsGradientFill(t *testing.T) {
	vg := &VectorGraphic{
		Gradients: map[string]gui.SvgGradientDef{
			"grad1": {
				X1: 0, Y1: 0, X2: 1, Y2: 0,
				GradientUnits: "objectBoundingBox",
				Stops: []gui.SvgGradientStop{
					{Offset: 0, Color: gui.SvgColor{R: 255, A: 255}},
					{Offset: 1, Color: gui.SvgColor{B: 255, A: 255}},
				},
			},
		},
		Paths: []VectorPath{
			{
				Transform:      identityTransform,
				FillColor:      gui.SvgColor{A: 255},
				FillGradientID: "grad1",
				Opacity:        1,
				FillOpacity:    1,
				Segments: []PathSegment{
					{Cmd: CmdMoveTo, Points: []float32{0, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 0}},
					{Cmd: CmdLineTo, Points: []float32{10, 10}},
					{Cmd: CmdClose},
				},
			},
		},
	}
	result := vg.tessellatePaths(vg.Paths, 1.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 tessellated path, got %d", len(result))
	}
	if len(result[0].VertexColors) == 0 {
		t.Fatal("gradient fill should produce vertex colors")
	}
}

func TestTessellateTessellatePathsEmpty(t *testing.T) {
	vg := &VectorGraphic{}
	result := vg.tessellatePaths(nil, 1.0)
	if len(result) != 0 {
		t.Fatalf("empty paths: expected 0, got %d", len(result))
	}
}

// --- Helpers ---

func triangleAreaSum(tris []float32) float32 {
	total := float32(0)
	for i := 0; i < len(tris)-5; i += 6 {
		ax, ay := tris[i], tris[i+1]
		bx, by := tris[i+2], tris[i+3]
		cx, cy := tris[i+4], tris[i+5]
		area := float32(math.Abs(float64(
			(bx-ax)*(cy-ay)-(cx-ax)*(by-ay)))) / 2
		total += area
	}
	return total
}

func verifyTriangleAreaSum(t *testing.T, tris []float32, expected float32) {
	t.Helper()
	total := triangleAreaSum(tris)
	if f32Abs(total-expected) > 0.5 {
		t.Fatalf("triangle area sum = %f, want ~%f", total, expected)
	}
}

func TestClampStrokeWidthForScale(t *testing.T) {
	cases := []struct {
		name  string
		w     float32
		scale float32
		want  float32
	}{
		{"scale_zero_unchanged", 0.5, 0, 0.5},
		{"scale_negative_unchanged", 0.5, -1, 0.5},
		{"subpixel_widened", 0.1, 1, 1},
		{"already_above_minVB", 5, 1, 5},
		{"high_dpi_scale", 0.01, 2, 0.5},
		{"borderline_equal_to_minVB", 1, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := clampStrokeWidthForScale(c.w, c.scale)
			if got != c.want {
				t.Fatalf("got %g, want %g", got, c.want)
			}
			if math.IsNaN(float64(got)) || math.IsInf(float64(got), 0) {
				t.Fatalf("non-finite result: %v", got)
			}
		})
	}
}

// Subnormal/extremely small positive scale must not blow up the
// derived 1/scale to +Inf — the function floors scale at 1e-6.
func TestClampStrokeWidthForScale_SubnormalScaleStaysFinite(t *testing.T) {
	got := clampStrokeWidthForScale(0.001, 1e-39)
	if math.IsInf(float64(got), 0) || math.IsNaN(float64(got)) {
		t.Fatalf("expected finite, got %v", got)
	}
	const wantMax = float32(1e6) + 1
	if got > wantMax {
		t.Fatalf("strokeWidth %g exceeds floor-derived cap %g", got, wantMax)
	}
}
