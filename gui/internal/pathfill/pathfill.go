package pathfill

// pathfill — the shared polygon fill core for canvas paths and SVG.
//
// Both gui (CanvasPath fills) and gui/svg (path tessellation) fill
// concave and multi-contour shapes through this package: one
// tessellator, one set of caps, one behavior. It follows the
// gui/internal/gradmesh precedent: the projection-neutral geometry
// lives here, and each side keeps its own vocabulary (gui.FillRule,
// svg's fillRule) with a trivial converter at the boundary.
//
// The entry point is Append, which triangulates closed polylines
// (flat [x0,y0,x1,y1,...] slices) honoring the fill rule. A single
// polyline takes the ear-clip fast path; multiple polylines go
// through a scanline trapezoidal decomposition that respects nonzero
// / evenodd winding across all contours.

import (
	"cmp"
	"math"
	"slices"
)

// Rule selects how overlapping subpaths are filled.
type Rule uint8

// Fill rule values. The zero value is Nonzero, matching SVG's
// default.
const (
	RuleNonzero Rule = iota
	RuleEvenOdd
)

// Caps bounding hostile input. See the matching comments in the
// helpers below.
const (
	MaxEarClipVerts = 2048
	MaxScanEdges    = 8192
	MaxScanYs       = 16384
)

// MaxFlattenDepth bounds curve subdivision. Shared with the SVG
// path flattener it was lifted with.
const MaxFlattenDepth = 16

// curveDegenThreshold is the chord length below which a cubic is
// taken as-is.
const curveDegenThreshold = float32(0.0001)

// closedPathEpsilon is how close a contour's last point must land to
// its first to count as explicitly closed.
const closedPathEpsilon = float32(0.0001)

// Scratch is reusable workspace for Append. A nil scratch allocates;
// a caller on a hot path (a canvas redraw) keeps one and hands it in
// every frame so the edge, slice and active lists stop allocating.
// It must not be shared across goroutines.
type Scratch struct {
	Edges   []Edge
	Ys      []float32
	Active  []int
	Indices []int
}

// Edge is a non-horizontal contour edge with y-normalized endpoints
// (y0 <= y1). Sign is +1 when the original edge ran upward (y
// increasing) and -1 when downward; walking edges left-to-right at a
// given y and summing sign yields the winding number to the right of
// each edge.
type Edge struct {
	X0, Y0, X1, Y1 float32
	Sign           int8
}

// Append triangulates contours honoring rule and appends the flat
// x,y triangle list to dst. Contours shorter than 3 points are
// skipped. Scratch may be nil, in which case the workspace
// allocates; pass a kept Scratch on a hot path.
func Append(dst []float32, contours [][]float32, rule Rule,
	scratch *Scratch) []float32 {
	var edges []Edge
	var ys []float32
	var active []int
	var indices []int
	if scratch != nil {
		edges = scratch.Edges[:0]
		ys = scratch.Ys[:0]
		active = scratch.Active[:0]
		indices = scratch.Indices[:0]
	}
	kept := 0
	for _, poly := range contours {
		if len(poly) >= 6 {
			kept++
		}
	}
	if kept == 0 {
		releaseScratch(scratch, edges, ys, active, indices)
		return dst
	}
	// Single-contour nonzero: ear-clip fast path. Simple (non self-
	// intersecting) polygons fill identically under both rules, so
	// the fast path covers the common case. Evenodd on a self-
	// intersecting single contour (e.g. figure-8) needs the winding
	// decomposition, so route it to scanline.
	if kept == 1 && rule == RuleNonzero {
		for _, poly := range contours {
			if len(poly) >= 6 {
				dst = appendEarClip(dst, poly, &indices)
				break
			}
		}
		releaseScratch(scratch, edges, ys, active, indices)
		return dst
	}
	dst = appendScanline(dst, contours, rule, &edges, &ys, &active)
	releaseScratch(scratch, edges, ys, active, indices)
	return dst
}

// releaseScratch stores the workspace back when the caller kept one.
func releaseScratch(s *Scratch, edges []Edge, ys []float32,
	active []int, indices []int) {
	if s == nil {
		return
	}
	s.Edges, s.Ys, s.Active, s.Indices = edges, ys, active, indices
}

// BuildEdges collects all non-horizontal edges from all contours
// with y-normalized endpoints. Edges touching non-finite coords are
// skipped (defense in depth against NaN/Inf that slip past parsing).
// Returns at most MaxScanEdges entries.
func BuildEdges(contours [][]float32) []Edge {
	edges := make([]Edge, 0, edgeCapHint(contours))
	return appendEdges(edges, contours)
}

// edgeCapHint sizes the edge list without over-committing on hostile
// input.
func edgeCapHint(contours [][]float32) int {
	n := 0
	for _, poly := range contours {
		n += len(poly) / 2
	}
	if n > MaxScanEdges {
		n = MaxScanEdges
	}
	return n
}

// appendEdges grows edges with the contour edges, stopping at the
// cap.
func appendEdges(edges []Edge, contours [][]float32) []Edge {
	for _, poly := range contours {
		m := len(poly) / 2
		if m < 3 {
			continue
		}
		for k := range m {
			if len(edges) >= MaxScanEdges {
				return edges
			}
			ax := poly[2*k]
			ay := poly[2*k+1]
			j := (k + 1) % m
			bx := poly[2*j]
			by := poly[2*j+1]
			if ay == by {
				continue
			}
			if !finiteF32(ax) || !finiteF32(ay) ||
				!finiteF32(bx) || !finiteF32(by) {
				continue
			}
			if ay < by {
				edges = append(edges, Edge{X0: ax, Y0: ay, X1: bx,
					Y1: by, Sign: +1})
			} else {
				edges = append(edges, Edge{X0: bx, Y0: by, X1: ax,
					Y1: ay, Sign: -1})
			}
		}
	}
	return edges
}

// SegmentIntersectionY returns the y-coordinate where two segments
// cross in their strict interiors. Endpoint touches are excluded
// because those y values are already captured as edge endpoints.
// denEps scales with the bbox area (cross products are unit²) so
// parallel detection is meaningful across viewBox magnitudes.
func SegmentIntersectionY(a, b Edge, denEps float32) (float32, bool) {
	d1x := a.X1 - a.X0
	d1y := a.Y1 - a.Y0
	d2x := b.X1 - b.X0
	d2y := b.Y1 - b.Y0
	den := d1x*d2y - d1y*d2x
	if f32Abs(den) < denEps {
		return 0, false
	}
	ex := b.X0 - a.X0
	ey := b.Y0 - a.Y0
	t := (ex*d2y - ey*d2x) / den
	s := (ex*d1y - ey*d1x) / den
	const endEps = 1e-7
	if t <= endEps || t >= 1-endEps || s <= endEps || s >= 1-endEps {
		return 0, false
	}
	return a.Y0 + t*d1y, true
}

// collectScanYs gathers unique y values from edge endpoints and
// edge-edge intersections, sorted ascending. yEps and denEps are
// bbox-scaled so the dedup collapses truly coincident values
// regardless of viewBox magnitude. The intersection scan bails
// once the y pool exceeds MaxScanYs to cap worst-case memory on
// dense self-intersecting contours (e.g. pinwheels).
func collectScanYs(edges []Edge, ys []float32, yEps,
	denEps float32) []float32 {
	out := ys[:0]
	for i := range edges {
		out = append(out, edges[i].Y0, edges[i].Y1)
	}
collect:
	for i := range edges {
		for j := i + 1; j < len(edges); j++ {
			if y, ok := SegmentIntersectionY(edges[i], edges[j],
				denEps); ok {
				out = append(out, y)
				if len(out) >= MaxScanYs {
					break collect
				}
			}
		}
	}
	slices.Sort(out)
	dedup := out[:0]
	for _, y := range out {
		if len(dedup) == 0 || y-dedup[len(dedup)-1] > yEps {
			dedup = append(dedup, y)
		}
	}
	return dedup
}

// EdgesBoundScale returns a representative linear scale for the bbox
// of edges (max extent). Used to scale epsilons that are absolute in
// viewBox units. Returns 1 when edges are degenerate so eps values
// remain finite.
func EdgesBoundScale(edges []Edge) float32 {
	if len(edges) == 0 {
		return 1
	}
	minX, minY := edges[0].X0, edges[0].Y0
	maxX, maxY := minX, minY
	for i := range edges {
		e := edges[i]
		minX = min(minX, e.X0, e.X1)
		maxX = max(maxX, e.X0, e.X1)
		// Edge invariant: Y0 < Y1.
		minY = min(minY, e.Y0)
		maxY = max(maxY, e.Y1)
	}
	s := max(maxX-minX, maxY-minY)
	if s <= 0 {
		return 1
	}
	return s
}

// xAtY linearly interpolates the edge's x at the given y.
// Precondition: e.Y0 <= y <= e.Y1.
func xAtY(e Edge, y float32) float32 {
	dy := e.Y1 - e.Y0
	if dy <= 0 {
		return e.X0
	}
	t := (y - e.Y0) / dy
	return e.X0 + t*(e.X1-e.X0)
}

// appendScanline decomposes the filled region under the fill rule
// into trapezoidal strips between consecutive unique y values and
// appends the triangles to dst. Within each strip no intersections
// occur (all are listed in the y set), so active edges keep a stable
// left-to-right order; winding is accumulated as edges are crossed
// and a trapezoid is emitted wherever the rule reports "filled".
// edges, ys and active arrive as scratch pointers so a hot caller
// reuses them across frames.
func appendScanline(dst []float32, contours [][]float32, rule Rule,
	edges *[]Edge, ys *[]float32, active *[]int) []float32 {
	*edges = appendEdges((*edges)[:0], contours)
	if len(*edges) == 0 {
		return dst
	}
	scale := EdgesBoundScale(*edges)
	yEps := scale * 1e-6
	stripEps := scale * 1e-7
	activeEps := scale * 1e-6
	denEps := scale * scale * 1e-6
	areaEps := scale * scale * 1e-9
	slicesYs := collectScanYs(*edges, *ys, yEps, denEps)
	*ys = slicesYs
	if len(slicesYs) < 2 {
		return dst
	}
	*active = (*active)[:0]
	for i := 0; i+1 < len(slicesYs); i++ {
		yTop := slicesYs[i]
		yBot := slicesYs[i+1]
		if yBot-yTop < stripEps {
			continue
		}
		yMid := (yTop + yBot) * 0.5

		act := (*active)[:0]
		for j := range *edges {
			if (*edges)[j].Y0 <= yTop+activeEps &&
				(*edges)[j].Y1 >= yBot-activeEps {
				act = append(act, j)
			}
		}
		if len(act) < 2 {
			continue
		}
		edgeList := *edges
		slices.SortFunc(act, func(a, b int) int {
			return cmp.Compare(xAtY(edgeList[a], yMid),
				xAtY(edgeList[b], yMid))
		})

		winding := int32(0)
		leftIdx := -1
		for k := range act {
			eg := edgeList[act[k]]
			winding += int32(eg.Sign)
			inside := false
			switch rule {
			case RuleEvenOdd:
				inside = winding&1 != 0
			default:
				inside = winding != 0
			}
			if inside && leftIdx < 0 {
				leftIdx = k
				continue
			}
			if !inside && leftIdx >= 0 {
				le := edgeList[act[leftIdx]]
				re := eg
				xLT := xAtY(le, yTop)
				xRT := xAtY(re, yTop)
				xLB := xAtY(le, yBot)
				xRB := xAtY(re, yBot)
				dst = appendTrapezoid(dst, xLT, xRT, xLB, xRB,
					yTop, yBot, areaEps)
				leftIdx = -1
			}
		}
		*active = act[:0]
	}
	return dst
}

// appendTrapezoid emits the non-degenerate triangles of a trapezoid
// with left edge (xLT,yTop)-(xLB,yBot) and right edge
// (xRT,yTop)-(xRB,yBot). Each candidate triangle is skipped when its
// signed area is below areaEps; this avoids degenerate slivers at
// near-horizontal intersections that otherwise fool
// point-in-triangle tests via float32 precision loss.
func appendTrapezoid(dst []float32,
	xLT, xRT, xLB, xRB, yTop, yBot, areaEps float32,
) []float32 {
	dst = appendNonDegenTri(dst, xLT, yTop, xRT, yTop, xRB, yBot,
		areaEps)
	dst = appendNonDegenTri(dst, xLT, yTop, xRB, yBot, xLB, yBot,
		areaEps)
	return dst
}

// appendNonDegenTri appends a triangle only when its 2× signed area
// exceeds areaEps. NaN/Inf coordinates are dropped — NaN comparisons
// always yield false, which would otherwise let non-finite vertices
// splat into the GPU vertex buffer.
func appendNonDegenTri(dst []float32,
	ax, ay, bx, by, cx, cy, areaEps float32,
) []float32 {
	if !finiteF32(ax) || !finiteF32(ay) ||
		!finiteF32(bx) || !finiteF32(by) ||
		!finiteF32(cx) || !finiteF32(cy) {
		return dst
	}
	area := f32Abs((bx-ax)*(cy-ay) - (cx-ax)*(by-ay))
	if !(area >= areaEps) {
		return dst
	}
	return append(dst, ax, ay, bx, by, cx, cy)
}

// EarClip triangulates one simple polygon and returns the flat x,y
// triangle list. Returns nil for degenerate input. It is O(n³), so
// input past MaxEarClipVerts is refused — past it the polygon is
// almost certainly outside the viewport anyway.
func EarClip(polygon []float32) []float32 {
	return appendEarClip(nil, polygon, nil)
}

// appendEarClip is EarClip appending to dst. indices is reusable
// workspace for the vertex list, or nil to allocate.
func appendEarClip(dst, polygon []float32, indices *[]int) []float32 {
	n := len(polygon) / 2
	if n < 3 {
		return dst
	}
	// Drop polygons with bad vertices. A NaN vertex passes the area
	// tests below and ends in the GPU vertex buffer.
	for _, v := range polygon {
		if !finiteF32(v) {
			return dst
		}
	}
	// Strip trailing duplicate.
	if n > 3 {
		lx := polygon[(n-1)*2]
		ly := polygon[(n-1)*2+1]
		if f32Abs(lx-polygon[0]) < closedPathEpsilon &&
			f32Abs(ly-polygon[1]) < closedPathEpsilon {
			n--
		}
	}
	if n < 3 {
		return dst
	}
	if n > MaxEarClipVerts {
		return dst
	}
	poly := polygon[:n*2]
	if n == 3 {
		return append(dst, poly...)
	}

	var live []int
	if indices != nil {
		live = (*indices)[:0]
	}
	if cap(live) < n {
		live = make([]int, n)
	} else {
		live = live[:n]
	}
	if indices != nil {
		*indices = live
	}
	verts := live
	if PolygonArea(poly) > 0 {
		for i := n - 1; i >= 0; i-- {
			verts[n-1-i] = i
		}
	} else {
		for i := range n {
			verts[i] = i
		}
	}

	count := 2 * n
	v := n - 1

	for len(verts) > 2 {
		if count <= 0 {
			break
		}
		count--

		u := v
		if u >= len(verts) {
			u = 0
		}
		v = u + 1
		if v >= len(verts) {
			v = 0
		}
		w := v + 1
		if w >= len(verts) {
			w = 0
		}

		if isEar(poly, verts, u, v, w) {
			a := verts[u]
			b := verts[v]
			c := verts[w]
			dst = append(dst,
				poly[a*2], poly[a*2+1],
				poly[b*2], poly[b*2+1],
				poly[c*2], poly[c*2+1],
			)
			verts = append(verts[:v], verts[v+1:]...)
			count = 2 * len(verts)
		}
	}
	if indices != nil {
		*indices = verts[:0]
	}
	return dst
}

// PolygonArea returns the signed doubled area of a polygon.
func PolygonArea(polygon []float32) float32 {
	n := len(polygon) / 2
	area := float32(0)
	j := n - 1
	for i := range n {
		area += (polygon[j*2] + polygon[i*2]) *
			(polygon[j*2+1] - polygon[i*2+1])
		j = i
	}
	return area / 2
}

// isEar reports whether the vertex triple u,v,w forms an ear: a
// convex corner containing no other vertex.
func isEar(polygon []float32, indices []int, u, v, w int) bool {
	ax := polygon[indices[u]*2]
	ay := polygon[indices[u]*2+1]
	bx := polygon[indices[v]*2]
	by := polygon[indices[v]*2+1]
	cx := polygon[indices[w]*2]
	cy := polygon[indices[w]*2+1]

	cross := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
	if cross <= 0 {
		return false
	}

	for i := range len(indices) {
		if i == u || i == v || i == w {
			continue
		}
		px := polygon[indices[i]*2]
		py := polygon[indices[i]*2+1]
		if (px == ax && py == ay) || (px == bx && py == by) ||
			(px == cx && py == cy) {
			continue
		}
		if PointInTriangle(px, py, ax, ay, bx, by, cx, cy) {
			return false
		}
	}
	return true
}

// PointInTriangle reports whether (px,py) lies inside the triangle
// (a,b,c).
func PointInTriangle(px, py, ax, ay, bx, by, cx, cy float32) bool {
	v0x := cx - ax
	v0y := cy - ay
	v1x := bx - ax
	v1y := by - ay
	v2x := px - ax
	v2y := py - ay

	dot00 := v0x*v0x + v0y*v0y
	dot01 := v0x*v1x + v0y*v1y
	dot02 := v0x*v2x + v0y*v2y
	dot11 := v1x*v1x + v1y*v1y
	dot12 := v1x*v2x + v1y*v2y

	denom := dot00*dot11 - dot01*dot01
	if f32Abs(denom) < 1e-10 {
		return false
	}
	invDenom := 1.0 / denom
	uu := (dot11*dot02 - dot01*dot12) * invDenom
	vv := (dot00*dot12 - dot01*dot02) * invDenom

	return uu >= 0 && vv >= 0 && (uu+vv) < 1
}

// --- Curve flattening ---

// FlattenQuad appends the flattened quadratic from (x0,y0) to
// (x1,y1), excluding the start point the caller already holds.
func FlattenQuad(x0, y0, cx, cy, x1, y1, tolerance float32,
	points *[]float32) {
	flattenQuadRec(x0, y0, cx, cy, x1, y1, tolerance, 0, points)
}

func flattenQuadRec(x0, y0, cx, cy, x1, y1, tolerance float32,
	depth int, points *[]float32) {
	mx := (x0 + x1) / 2
	my := (y0 + y1) / 2
	dx := cx - mx
	dy := cy - my
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if d <= tolerance || depth >= MaxFlattenDepth {
		*points = append(*points, x1, y1)
	} else {
		ax := (x0 + cx) / 2
		ay := (y0 + cy) / 2
		bx := (cx + x1) / 2
		by := (cy + y1) / 2
		abx := (ax + bx) / 2
		aby := (ay + by) / 2
		flattenQuadRec(x0, y0, ax, ay, abx, aby, tolerance,
			depth+1, points)
		flattenQuadRec(abx, aby, bx, by, x1, y1, tolerance,
			depth+1, points)
	}
}

// FlattenCubic appends the flattened cubic from (x0,y0) to (x1,y1),
// excluding the start point the caller already holds.
func FlattenCubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1,
	tolerance float32, points *[]float32) {
	flattenCubicRec(x0, y0, c1x, c1y, c2x, c2y, x1, y1, tolerance,
		0, points)
}

func flattenCubicRec(x0, y0, c1x, c1y, c2x, c2y, x1, y1,
	tolerance float32, depth int, points *[]float32) {
	dx := x1 - x0
	dy := y1 - y0
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if d < curveDegenThreshold {
		*points = append(*points, x1, y1)
		return
	}

	d1 := f32Abs((c1x-x0)*dy-(c1y-y0)*dx) / d
	d2 := f32Abs((c2x-x0)*dy-(c2y-y0)*dx) / d

	if d1+d2 <= tolerance || depth >= MaxFlattenDepth {
		*points = append(*points, x1, y1)
	} else {
		ax := (x0 + c1x) / 2
		ay := (y0 + c1y) / 2
		bx := (c1x + c2x) / 2
		by := (c1y + c2y) / 2
		cx := (c2x + x1) / 2
		cy := (c2y + y1) / 2
		abx := (ax + bx) / 2
		aby := (ay + by) / 2
		bcx := (bx + cx) / 2
		bcy := (by + cy) / 2
		mx := (abx + bcx) / 2
		my := (aby + bcy) / 2
		flattenCubicRec(x0, y0, ax, ay, abx, aby, mx, my,
			tolerance, depth+1, points)
		flattenCubicRec(mx, my, bcx, bcy, cx, cy, x1, y1,
			tolerance, depth+1, points)
	}
}

// f32Abs returns the absolute value. NaN passes through.
func f32Abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// finiteF32 reports whether f is neither NaN nor Inf.
func finiteF32(f float32) bool {
	return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0)
}
