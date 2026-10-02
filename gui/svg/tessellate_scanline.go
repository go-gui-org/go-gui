package svg

import (
	"github.com/go-gui-org/go-gui/gui/internal/pathfill"
)

// tessellate_scanline.go — the SVG bridge to the shared fill core.
//
// The geometry lives in gui/internal/pathfill, shared with
// DrawContext's path fills. What stays here are the svg-domain
// pieces: the fillRule conversion at the boundary and the
// unexported names the existing tests address (scanEdge,
// buildScanEdges, earClip and friends), which delegate element-wise.
// The algorithm itself is tested once, in pathfill.

// tessellatePolylines triangulates one or more closed polylines
// (flat [x0,y0,x1,y1,...] slices) honoring the given fill-rule.
func tessellatePolylines(polylines [][]float32, rule fillRule) []float32 {
	return pathfill.Append(nil, polylines, pathfillRule(rule), nil)
}

// pathfillRule maps an SVG fill rule onto the shared rule. Anything
// outside evenodd — including the zero value — fills nonzero, which
// is the SVG default.
func pathfillRule(rule fillRule) pathfill.Rule {
	if rule == fillRuleEvenOdd {
		return pathfill.RuleEvenOdd
	}
	return pathfill.RuleNonzero
}

// scanEdge is pathfill.Edge in svg spelling, kept so the existing
// edge-level tests read unchanged. Converted at the boundary.
type scanEdge struct {
	x0, y0, x1, y1 float32
	sign           int8
}

// maxScanEdges caps the number of edges fed to the scanline
// tessellator.
const maxScanEdges = pathfill.MaxScanEdges

// buildScanEdges collects all non-horizontal edges from all
// contours with y-normalized endpoints.
func buildScanEdges(contours [][]float32) []scanEdge {
	built := pathfill.BuildEdges(contours)
	edges := make([]scanEdge, len(built))
	for i, e := range built {
		edges[i] = scanEdge{x0: e.X0, y0: e.Y0, x1: e.X1, y1: e.Y1,
			sign: e.Sign}
	}
	return edges
}

// segmentIntersectionY returns the y-coordinate where two segments
// cross in their strict interiors.
func segmentIntersectionY(a, b scanEdge, denEps float32) (float32, bool) {
	return pathfill.SegmentIntersectionY(
		pathfill.Edge{X0: a.x0, Y0: a.y0, X1: a.x1, Y1: a.y1,
			Sign: a.sign},
		pathfill.Edge{X0: b.x0, Y0: b.y0, X1: b.x1, Y1: b.y1,
			Sign: b.sign}, denEps)
}

// edgesBoundsScale returns a representative linear scale for the
// bbox of edges. Returns 1 when edges are degenerate.
func edgesBoundsScale(edges []scanEdge) float32 {
	built := make([]pathfill.Edge, len(edges))
	for i, e := range edges {
		built[i] = pathfill.Edge{X0: e.x0, Y0: e.y0, X1: e.x1,
			Y1: e.y1, Sign: e.sign}
	}
	return pathfill.EdgesBoundScale(built)
}

// earClip triangulates one simple polygon.
func earClip(polygon []float32) []float32 {
	return pathfill.EarClip(polygon)
}

// polygonArea returns the signed doubled area of a polygon.
func polygonArea(polygon []float32) float32 {
	return pathfill.PolygonArea(polygon)
}

// pointInTriangle reports whether (px,py) lies inside the triangle
// (a,b,c).
func pointInTriangle(px, py, ax, ay, bx, by, cx, cy float32) bool {
	return pathfill.PointInTriangle(px, py, ax, ay, bx, by, cx, cy)
}
