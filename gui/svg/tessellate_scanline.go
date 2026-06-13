package svg

import (
	"cmp"
	"math"
	"slices"

	"github.com/go-gui-org/go-gui/gui"
)

// --- Tessellation ---

// tessellatePolylines triangulates one or more closed polylines
// (flat [x0,y0,x1,y1,...] slices) honoring the given fill-rule.
// A single polyline takes the ear-clip fast path; multiple
// polylines go through a scanline trapezoidal decomposition that
// respects nonzero / evenodd winding across all contours. The
// decomposition correctly handles real outer+hole pairs, peer
// subpaths with mixed windings (e.g. radial pinwheels), and
// independent same-winding regions (e.g. circles.svg).
func tessellatePolylines(polylines [][]float32, rule FillRule) []float32 {
	if len(polylines) == 0 {
		return nil
	}
	contours := make([][]float32, 0, len(polylines))
	for _, poly := range polylines {
		if len(poly) >= 6 {
			contours = append(contours, poly)
		}
	}
	if len(contours) == 0 {
		return nil
	}
	// Single-contour nonzero: ear-clip fast path. Simple (non self-
	// intersecting) polygons fill identically under both rules, so
	// the fast path covers the common case. Evenodd on a self-
	// intersecting single contour (e.g. figure-8) needs the winding
	// decomposition, so route it to scanline.
	if len(contours) == 1 && rule == FillRuleNonzero {
		return earClip(contours[0])
	}
	return scanlineTessellate(contours, rule)
}

// scanEdge is a non-horizontal contour edge with y-normalized
// endpoints (y0 <= y1). sign is +1 when the original edge ran
// upward (y increasing) and -1 when downward; walking edges
// left-to-right at a given y and summing sign yields the winding
// number to the right of each edge.
type scanEdge struct {
	x0, y0, x1, y1 float32
	sign           int8
}

// maxEarClipVerts caps the vertex count for the single-contour
// ear-clip fast path. ear-clip is O(n³): at n≈30k the inner
// pointInTriangle scan stalls for many seconds per call. Hostile
// overrides (huge radii, scaled animation values) can inflate one
// contour to tens of thousands of flattened vertices, starving
// the race-detector test. Beyond this cap the polygon is almost
// certainly outside the viewport anyway, so returning nil is
// acceptable — scanline has its own maxScanEdges cap.
const maxEarClipVerts = 2048

// maxScanEdges caps the number of edges fed to the scanline
// tessellator. The intersection scan in collectScanYs is O(E²)
// and the strip loop is O(strips × E), so uncapped input (huge
// or hostile SVGs) could stall tessellation. ~8k edges keeps
// worst-case at ~64M ops, well under a frame budget.
const maxScanEdges = 8192

// maxScanYs caps the unique y-slice count. A radial pinwheel with
// E edges can have O(E²) intersections; left uncapped, the strip
// loop degrades to O(E³) and tris cap allocation explodes.
const maxScanYs = 16384

// buildScanEdges collects all non-horizontal edges from all
// contours with y-normalized endpoints. Edges touching non-finite
// coords are skipped (defense in depth against NaN/Inf that slip
// past parsing). Returns at most maxScanEdges entries.
func buildScanEdges(contours [][]float32) []scanEdge {
	n := 0
	for _, poly := range contours {
		n += len(poly) / 2
	}
	if n > maxScanEdges {
		n = maxScanEdges
	}
	edges := make([]scanEdge, 0, n)
	for _, poly := range contours {
		m := len(poly) / 2
		if m < 3 {
			continue
		}
		for k := range m {
			if len(edges) >= maxScanEdges {
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
				edges = append(edges, scanEdge{ax, ay, bx, by, +1})
			} else {
				edges = append(edges, scanEdge{bx, by, ax, ay, -1})
			}
		}
	}
	return edges
}

// segmentIntersectionY returns the y-coordinate where two segments
// cross in their strict interiors. Endpoint touches are excluded
// because those y values are already captured as edge endpoints.
// denEps scales with the bbox area (cross products are unit²) so
// parallel detection is meaningful across viewBox magnitudes.
func segmentIntersectionY(a, b scanEdge, denEps float32) (float32, bool) {
	d1x := a.x1 - a.x0
	d1y := a.y1 - a.y0
	d2x := b.x1 - b.x0
	d2y := b.y1 - b.y0
	den := d1x*d2y - d1y*d2x
	if f32Abs(den) < denEps {
		return 0, false
	}
	ex := b.x0 - a.x0
	ey := b.y0 - a.y0
	t := (ex*d2y - ey*d2x) / den
	s := (ex*d1y - ey*d1x) / den
	const endEps = 1e-7
	if t <= endEps || t >= 1-endEps || s <= endEps || s >= 1-endEps {
		return 0, false
	}
	return a.y0 + t*d1y, true
}

// collectScanYs gathers unique y values from edge endpoints and
// edge-edge intersections, sorted ascending. yEps and denEps are
// bbox-scaled so the dedup collapses truly coincident values
// regardless of viewBox magnitude. The intersection scan bails
// once the y pool exceeds maxScanYs to cap worst-case memory on
// dense self-intersecting contours (e.g. pinwheels).
func collectScanYs(edges []scanEdge, yEps, denEps float32) []float32 {
	ys := make([]float32, 0, min(2*len(edges), maxScanYs))
	for i := range edges {
		ys = append(ys, edges[i].y0, edges[i].y1)
	}
collect:
	for i := range edges {
		for j := i + 1; j < len(edges); j++ {
			if y, ok := segmentIntersectionY(edges[i], edges[j], denEps); ok {
				ys = append(ys, y)
				if len(ys) >= maxScanYs {
					break collect
				}
			}
		}
	}
	slices.Sort(ys)
	out := ys[:0]
	for _, y := range ys {
		if len(out) == 0 || y-out[len(out)-1] > yEps {
			out = append(out, y)
		}
	}
	return out
}

// edgesBoundsScale returns a representative linear scale for the
// bbox of edges (max extent). Used to scale epsilons that are
// absolute in viewBox units. Returns 1 when edges are degenerate
// so eps values remain finite.
func edgesBoundsScale(edges []scanEdge) float32 {
	if len(edges) == 0 {
		return 1
	}
	minX, minY := edges[0].x0, edges[0].y0
	maxX, maxY := minX, minY
	for i := range edges {
		e := edges[i]
		minX = min(minX, e.x0, e.x1)
		maxX = max(maxX, e.x0, e.x1)
		// scanEdge invariant: y0 < y1
		minY = min(minY, e.y0)
		maxY = max(maxY, e.y1)
	}
	s := max(maxX-minX, maxY-minY)
	if s <= 0 {
		return 1
	}
	return s
}

// xAtY linearly interpolates the edge's x at the given y.
// Precondition: e.y0 <= y <= e.y1.
func xAtY(e scanEdge, y float32) float32 {
	dy := e.y1 - e.y0
	if dy <= 0 {
		return e.x0
	}
	t := (y - e.y0) / dy
	return e.x0 + t*(e.x1-e.x0)
}

// scanlineTessellate decomposes the filled region under the fill
// rule into trapezoidal strips between consecutive unique y
// values. Within each strip no intersections occur (all are listed
// in the y set), so active edges keep a stable left-to-right
// order; winding is accumulated as edges are crossed and a
// trapezoid is emitted wherever the rule reports "filled".
func scanlineTessellate(contours [][]float32, rule FillRule) []float32 {
	edges := buildScanEdges(contours)
	if len(edges) == 0 {
		return nil
	}
	scale := edgesBoundsScale(edges)
	yEps := scale * 1e-6
	stripEps := scale * 1e-7
	activeEps := scale * 1e-6
	denEps := scale * scale * 1e-6
	areaEps := scale * scale * 1e-9
	ys := collectScanYs(edges, yEps, denEps)
	if len(ys) < 2 {
		return nil
	}
	active := make([]int, 0, len(edges))
	tris := make([]float32, 0, 6*len(ys))
	for i := 0; i+1 < len(ys); i++ {
		yTop := ys[i]
		yBot := ys[i+1]
		if yBot-yTop < stripEps {
			continue
		}
		yMid := (yTop + yBot) * 0.5

		active = active[:0]
		for j := range edges {
			if edges[j].y0 <= yTop+activeEps && edges[j].y1 >= yBot-activeEps {
				active = append(active, j)
			}
		}
		if len(active) < 2 {
			continue
		}
		slices.SortFunc(active, func(a, b int) int {
			return cmp.Compare(xAtY(edges[a], yMid), xAtY(edges[b], yMid))
		})

		winding := int32(0)
		leftIdx := -1
		for k := range active {
			eg := edges[active[k]]
			winding += int32(eg.sign)
			inside := false
			switch rule {
			case FillRuleEvenOdd:
				inside = winding&1 != 0
			default:
				inside = winding != 0
			}
			if inside && leftIdx < 0 {
				leftIdx = k
				continue
			}
			if !inside && leftIdx >= 0 {
				le := edges[active[leftIdx]]
				re := eg
				xLT := xAtY(le, yTop)
				xRT := xAtY(re, yTop)
				xLB := xAtY(le, yBot)
				xRB := xAtY(re, yBot)
				tris = appendTrapezoid(tris, xLT, xRT, xLB, xRB, yTop, yBot, areaEps)
				leftIdx = -1
			}
		}
	}
	return tris
}

// appendTrapezoid emits the non-degenerate triangles of a
// trapezoid with left edge (xLT,yTop)-(xLB,yBot) and right edge
// (xRT,yTop)-(xRB,yBot). Each candidate triangle is skipped when
// its signed area is below areaEps; this avoids degenerate slivers
// at near-horizontal intersections that otherwise fool
// point-in-triangle tests via float32 precision loss.
func appendTrapezoid(dst []float32,
	xLT, xRT, xLB, xRB, yTop, yBot, areaEps float32,
) []float32 {
	dst = appendNonDegenTri(dst, xLT, yTop, xRT, yTop, xRB, yBot, areaEps)
	dst = appendNonDegenTri(dst, xLT, yTop, xRB, yBot, xLB, yBot, areaEps)
	return dst
}

// appendNonDegenTri appends a triangle only when its 2× signed
// area exceeds areaEps. NaN/Inf coordinates are dropped — NaN
// comparisons always yield false, which would otherwise let
// non-finite vertices splat into the GPU vertex buffer.
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

func earClip(polygon []float32) []float32 {
	n := len(polygon) / 2
	if n < 3 {
		return nil
	}
	// Strip trailing duplicate
	if n > 3 {
		lx := polygon[(n-1)*2]
		ly := polygon[(n-1)*2+1]
		if f32Abs(lx-polygon[0]) < closedPathEpsilon && f32Abs(ly-polygon[1]) < closedPathEpsilon {
			n--
		}
	}
	if n < 3 {
		return nil
	}
	if n > maxEarClipVerts {
		return nil
	}
	poly := polygon[:n*2]
	if n == 3 {
		result := make([]float32, 6)
		copy(result, poly)
		return result
	}

	indices := make([]int, n)
	if polygonArea(poly) > 0 {
		for i := n - 1; i >= 0; i-- {
			indices[n-1-i] = i
		}
	} else {
		for i := range n {
			indices[i] = i
		}
	}

	triangles := make([]float32, 0, (n-2)*6)
	count := 2 * n
	v := n - 1

	for len(indices) > 2 {
		if count <= 0 {
			break
		}
		count--

		u := v
		if u >= len(indices) {
			u = 0
		}
		v = u + 1
		if v >= len(indices) {
			v = 0
		}
		w := v + 1
		if w >= len(indices) {
			w = 0
		}

		if isEar(poly, indices, u, v, w) {
			a := indices[u]
			b := indices[v]
			c := indices[w]
			triangles = append(triangles,
				poly[a*2], poly[a*2+1],
				poly[b*2], poly[b*2+1],
				poly[c*2], poly[c*2+1],
			)
			indices = append(indices[:v], indices[v+1:]...)
			count = 2 * len(indices)
		}
	}
	return triangles
}

func polygonArea(polygon []float32) float32 {
	n := len(polygon) / 2
	area := float32(0)
	j := n - 1
	for i := range n {
		area += (polygon[j*2] + polygon[i*2]) * (polygon[j*2+1] - polygon[i*2+1])
		j = i
	}
	return area / 2
}

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
		if (px == ax && py == ay) || (px == bx && py == by) || (px == cx && py == cy) {
			continue
		}
		if pointInTriangle(px, py, ax, ay, bx, by, cx, cy) {
			return false
		}
	}
	return true
}

func pointInTriangle(px, py, ax, ay, bx, by, cx, cy float32) bool {
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

// --- Gradient support ---

func resolveGradient(g gui.SvgGradientDef, minX, minY, maxX, maxY float32) gui.SvgGradientDef {
	w := maxX - minX
	h := maxY - minY
	if g.IsRadial {
		// OBB → user space mapping. Spec maps the OBB to a 1×1
		// square then transforms back, which can yield elliptical
		// gradients. Approximation: scale R uniformly by the average
		// of width and height. For square viewBoxes (most icon use)
		// this is exact; for wide/tall bboxes the gradient stays
		// circular rather than stretching to an ellipse.
		avg := (w + h) * 0.5
		return gui.SvgGradientDef{
			Stops:         g.Stops,
			CX:            minX + g.CX*w,
			CY:            minY + g.CY*h,
			R:             g.R * avg,
			FX:            minX + g.FX*w,
			FY:            minY + g.FY*h,
			IsRadial:      true,
			GradientUnits: "userSpaceOnUse",
		}
	}
	return gui.SvgGradientDef{
		Stops:         g.Stops,
		X1:            minX + g.X1*w,
		Y1:            minY + g.Y1*h,
		X2:            minX + g.X2*w,
		Y2:            minY + g.Y2*h,
		GradientUnits: "userSpaceOnUse",
	}
}

func bboxFromTriangles(tris []float32) (float32, float32, float32, float32) {
	if len(tris) < 2 {
		return 0, 0, 0, 0
	}
	minX, minY := tris[0], tris[1]
	maxX, maxY := minX, minY
	for i := 2; i < len(tris); i += 2 {
		x, y := tris[i], tris[i+1]
		minX = min(minX, x)
		maxX = max(maxX, x)
		minY = min(minY, y)
		maxY = max(maxY, y)
	}
	return minX, minY, maxX, maxY
}

func projectOntoGradient(vx, vy float32, g gui.SvgGradientDef) float32 {
	if g.IsRadial {
		return projectOntoRadial(vx, vy, g)
	}
	dx := g.X2 - g.X1
	dy := g.Y2 - g.Y1
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return 0
	}
	t := ((vx-g.X1)*dx + (vy-g.Y1)*dy) / lenSq
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

// projectAndSpread projects (vx, vy) onto g without clamping to [0,1]
// then applies g.SpreadMethod. With pad (default) the clamp matches
// projectOntoGradient's historic behavior; reflect mirrors and
// repeat wraps for t outside [0,1].
func projectAndSpread(vx, vy float32, g gui.SvgGradientDef) float32 {
	t := projectOntoGradientRaw(vx, vy, g)
	return applySpread(t, g.SpreadMethod)
}

func projectOntoGradientRaw(vx, vy float32, g gui.SvgGradientDef) float32 {
	if g.IsRadial {
		r64 := float64(g.R)
		if g.R <= 0 || math.IsNaN(r64) || math.IsInf(r64, 0) {
			return 0
		}
		dx := vx - g.FX
		dy := vy - g.FY
		d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		t := d / g.R
		if t != t {
			return 0
		}
		return t
	}
	dx := g.X2 - g.X1
	dy := g.Y2 - g.Y1
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return 0
	}
	return ((vx-g.X1)*dx + (vy-g.Y1)*dy) / lenSq
}

// applySpread maps raw gradient parameter t through SpreadMethod.
// Pad clamps to [0,1]; reflect produces a triangle wave; repeat
// produces a sawtooth. NaN/Inf coerced to 0.
func applySpread(t float32, spread gui.SvgGradientSpread) float32 {
	t64 := float64(t)
	if math.IsNaN(t64) || math.IsInf(t64, 0) {
		return 0
	}
	// Clamp to a safe int64-convertible range so math.Floor's int64
	// cast for reflect parity cannot hit implementation-defined
	// overflow on hostile inputs. ±2^31 covers any plausible
	// gradient projection by many orders of magnitude.
	const spreadLimit = float64(1 << 31)
	if t64 > spreadLimit {
		t64 = spreadLimit
	} else if t64 < -spreadLimit {
		t64 = -spreadLimit
	}
	switch spread {
	case gui.SvgSpreadReflect:
		n := math.Floor(t64)
		frac := float32(t64 - n)
		if int64(n)&1 != 0 {
			return 1 - frac
		}
		return frac
	case gui.SvgSpreadRepeat:
		n := math.Floor(t64)
		return float32(t64 - n)
	}
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

// projectOntoRadial computes gradient parameter t for a radial
// gradient at vertex (vx, vy). Simplified implementation: distance
// from focal point divided by R, clamped to [0,1]. Full spec maps
// the focal-to-edge vector through a cone, which produces subtly
// different falloff when fx,fy != cx,cy. Tracked as future polish.
func projectOntoRadial(vx, vy float32, g gui.SvgGradientDef) float32 {
	r64 := float64(g.R)
	if g.R <= 0 || math.IsNaN(r64) || math.IsInf(r64, 0) {
		return 0
	}
	dx := vx - g.FX
	dy := vy - g.FY
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	t := d / g.R
	if t != t { // NaN
		return 0
	}
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

func interpolateGradient(stops []gui.SvgGradientStop, t float32) gui.SvgColor {
	if len(stops) == 0 {
		return gui.SvgColor{A: 255}
	}
	if t <= stops[0].Offset || len(stops) == 1 {
		return stops[0].Color
	}
	last := stops[len(stops)-1]
	if t >= last.Offset {
		return last.Color
	}
	for i := 0; i < len(stops)-1; i++ {
		s0 := stops[i]
		s1 := stops[i+1]
		if t >= s0.Offset && t <= s1.Offset {
			r := s1.Offset - s0.Offset
			if r <= 0 {
				return s0.Color
			}
			f := (t - s0.Offset) / r
			return gui.SvgColor{
				R: uint8(float32(s0.Color.R) + (float32(s1.Color.R)-float32(s0.Color.R))*f),
				G: uint8(float32(s0.Color.G) + (float32(s1.Color.G)-float32(s0.Color.G))*f),
				B: uint8(float32(s0.Color.B) + (float32(s1.Color.B)-float32(s0.Color.B))*f),
				A: uint8(float32(s0.Color.A) + (float32(s1.Color.A)-float32(s0.Color.A))*f),
			}
		}
	}
	return last.Color
}

func subdivideGradientTris(tris []float32, grad gui.SvgGradientDef) []float32 {
	if grad.IsRadial {
		return subdivideRadialTris(tris, grad)
	}
	if len(grad.Stops) <= 2 {
		return tris
	}
	stopTs := make([]float32, 0, len(grad.Stops))
	for _, s := range grad.Stops {
		if s.Offset > 0.001 && s.Offset < 0.999 {
			stopTs = append(stopTs, s.Offset)
		}
	}
	if len(stopTs) == 0 {
		return tris
	}
	result := make([]float32, 0, len(tris)*2)
	for i := 0; i < len(tris)-5; i += 6 {
		splitTriAtStops(tris[i], tris[i+1], tris[i+2], tris[i+3],
			tris[i+4], tris[i+5], grad, stopTs, 0, &result)
	}
	return result
}

// subdivideRadialTris recursively splits triangles whose edges span
// more than ~1/24 of the gradient radius. Per-vertex sampling on
// long edges interpolates linearly across the triangle, which
// linearizes the radial falloff and produces a flat appearance for
// large primitives (e.g. a 100×100 rect filled by 2 triangles).
// Smaller triangles approximate the circular iso-t lines closely
// enough that vertex coloring reads as a smooth radial gradient.
func subdivideRadialTris(tris []float32, grad gui.SvgGradientDef) []float32 {
	// Guard against non-finite R (NaN survives the < comparison and
	// would defeat the depth-cap heuristic, forcing every source
	// triangle to recurse to maxRadialDepth = 4096 splits).
	r64 := float64(grad.R)
	if math.IsNaN(r64) || math.IsInf(r64, 0) || grad.R <= 0 {
		return tris
	}
	target := grad.R / 24
	if target < 1e-3 {
		target = 1e-3
	}
	result := make([]float32, 0, len(tris)*4)
	for i := 0; i+5 < len(tris); i += 6 {
		splitRadialTri(tris[i], tris[i+1], tris[i+2], tris[i+3],
			tris[i+4], tris[i+5], target, 0, &result)
	}
	return result
}

func splitRadialTri(ax, ay, bx, by, cx, cy float32,
	target float32, depth int, result *[]float32) {
	const maxRadialDepth = 6
	abx := bx - ax
	aby := by - ay
	bcx := cx - bx
	bcy := cy - by
	cax := ax - cx
	cay := ay - cy
	maxLenSq := max(abx*abx+aby*aby,
		max(bcx*bcx+bcy*bcy, cax*cax+cay*cay))
	if depth >= maxRadialDepth || maxLenSq <= target*target {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	mabx, maby := (ax+bx)*0.5, (ay+by)*0.5
	mbcx, mbcy := (bx+cx)*0.5, (by+cy)*0.5
	mcax, mcay := (cx+ax)*0.5, (cy+ay)*0.5
	splitRadialTri(ax, ay, mabx, maby, mcax, mcay,
		target, depth+1, result)
	splitRadialTri(mabx, maby, bx, by, mbcx, mbcy,
		target, depth+1, result)
	splitRadialTri(mcax, mcay, mbcx, mbcy, cx, cy,
		target, depth+1, result)
	splitRadialTri(mabx, maby, mbcx, mbcy, mcax, mcay,
		target, depth+1, result)
}

func splitTriAtStops(ax, ay, bx, by, cx, cy float32, grad gui.SvgGradientDef, stopTs []float32, depth int, result *[]float32) {
	if depth >= maxSplitTriDepth {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	ta := projectOntoGradient(ax, ay, grad)
	tb := projectOntoGradient(bx, by, grad)
	tc := projectOntoGradient(cx, cy, grad)

	tMin := ta
	tMin = min(tMin, tb, tc)
	tMax := ta
	tMax = max(tMax, tb, tc)

	for _, tS := range stopTs {
		if tS > tMin+1e-4 && tS < tMax-1e-4 {
			// Sort vertices by t
			p0x, p0y, t0 := ax, ay, ta
			p1x, p1y, t1 := bx, by, tb
			p2x, p2y, t2 := cx, cy, tc
			if t0 > t1 {
				p0x, p0y, t0, p1x, p1y, t1 = p1x, p1y, t1, p0x, p0y, t0
			}
			if t1 > t2 {
				p1x, p1y, t1, p2x, p2y, t2 = p2x, p2y, t2, p1x, p1y, t1
			}
			if t0 > t1 {
				p0x, p0y, t0, p1x, p1y, t1 = p1x, p1y, t1, p0x, p0y, t0
			}

			f02 := float32(0.5)
			if t2-t0 > 1e-6 {
				f02 = (tS - t0) / (t2 - t0)
			}
			i1x := p0x + f02*(p2x-p0x)
			i1y := p0y + f02*(p2y-p0y)

			if tS < t1-1e-4 {
				f01 := float32(0.5)
				if t1-t0 > 1e-6 {
					f01 = (tS - t0) / (t1 - t0)
				}
				i2x := p0x + f01*(p1x-p0x)
				i2y := p0y + f01*(p1y-p0y)
				splitTriAtStops(p0x, p0y, i2x, i2y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(i2x, i2y, p1x, p1y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(p1x, p1y, p2x, p2y, i1x, i1y, grad, stopTs, depth+1, result)
			} else if tS > t1+1e-4 {
				f12 := float32(0.5)
				if t2-t1 > 1e-6 {
					f12 = (tS - t1) / (t2 - t1)
				}
				i2x := p1x + f12*(p2x-p1x)
				i2y := p1y + f12*(p2y-p1y)
				splitTriAtStops(p0x, p0y, p1x, p1y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(p1x, p1y, i2x, i2y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(i1x, i1y, i2x, i2y, p2x, p2y, grad, stopTs, depth+1, result)
			} else {
				splitTriAtStops(p0x, p0y, p1x, p1y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(p1x, p1y, p2x, p2y, i1x, i1y, grad, stopTs, depth+1, result)
			}
			return
		}
	}
	*result = append(*result, ax, ay, bx, by, cx, cy)
}
