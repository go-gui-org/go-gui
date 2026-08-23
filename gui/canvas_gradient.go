package gui

import "math"

// canvas_gradient.go — gradient fills for DrawContext.
//
// A canvas gradient is baked on the CPU into per-vertex colors on the
// batch's triangles, riding the existing RenderSvg command's
// VertexColors channel. Every backend already consumes that channel
// (metal, gl, web, soft, ios, android), and so does PDF export, so a
// canvas gradient needs no shader and no backend change.
//
// The projection and subdivision below deliberately mirror
// gui/svg/tessellate_gradient.go, which does the same job for SVG
// gradients. They are not shared code: gui/svg imports gui, so the
// dependency cannot run the other way, and routing SVG's SvgColor
// stops through Color would move existing SVG golden output for no
// gain here. Keep the two in sync by hand — a change to the falloff
// math in one almost certainly wants the same change in the other.

// GradientSpread selects how a canvas gradient handles parameter
// values outside [0,1]. Pad clamps (the zero value), Reflect mirrors,
// Repeat wraps.
// exportaudit:keep — the enum a CanvasGradient's Spread field takes
type GradientSpread uint8

// GradientSpread values.
const (
	// exportaudit:keep — the zero value, named for callers that set it
	// explicitly rather than relying on it
	SpreadPad GradientSpread = iota
	// exportaudit:keep — a member of an exported enum
	SpreadReflect
	SpreadRepeat
)

// CanvasGradient describes a gradient fill for DrawContext, in the
// canvas's own coordinate space (the same coordinates the drawing
// methods take).
//
// Geometry left degenerate is derived from the bounds of the geometry
// being filled, so the minimum useful value is a Stops list:
//
//   - a linear gradient whose endpoints coincide runs top-to-bottom
//     across the fill's bounding box
//   - a radial gradient with R <= 0 is centered on the fill's bounding
//     box with R set to half its larger extent
//
// FX/FY default to CX/CY when both are zero, matching SVG's default
// for fx/fy. A focal point genuinely at the origin, with a center
// elsewhere, must be nudged off it.
//
// Stops need not be sorted or in range; they are normalized on use.
// There is no stop-count limit — the GPU shader's five-stop cap
// applies to the uniform-packed shape gradient path, not to this one.
type CanvasGradient struct {
	Stops     []GradientStop
	X1, Y1    float32 // linear: start point
	X2, Y2    float32 // linear: end point
	CX, CY, R float32 // radial: center and radius
	FX, FY    float32 // radial: focal point
	Radial    bool
	Spread    GradientSpread
}

// maxCanvasStopIsolines caps how many parameter-space breakpoints a
// linear subdivision may split at. Reflect and Repeat generate one set
// of breakpoints per period the geometry spans, so a gradient whose
// endpoints are tiny relative to the fill would otherwise produce an
// unbounded split list.
const maxCanvasStopIsolines = 256

// maxCanvasIsolinePeriods bounds how many gradient periods a Repeat or
// Reflect fill may enumerate breakpoints for.
const maxCanvasIsolinePeriods = 64

// maxCanvasSplitDepth bounds the linear stop-isoline recursion, and
// maxCanvasRadialDepth the radial edge-length recursion. Both mirror
// gui/svg.
const (
	maxCanvasSplitDepth  = 8
	maxCanvasRadialDepth = 6
)

// maxCanvasSplitFloats caps the geometry one gradient fill may expand
// to, and maxCanvasRadialTris the triangle count the radial pass may
// quadruple its way to. Both are ceilings for hostile or careless
// input — hundreds of stops over a large mesh — not tuning knobs: the
// glow that motivated this file lands three orders of magnitude below
// either.
const (
	maxCanvasSplitFloats = 1 << 20 // ~175k triangles, 4 MB
	maxCanvasRadialTris  = 1 << 16
)

// triBounds returns the bounding box of a flat x,y triangle list.
func triBounds(tris []float32) (minX, minY, maxX, maxY float32) {
	if len(tris) < 2 {
		return 0, 0, 0, 0
	}
	minX, minY = tris[0], tris[1]
	maxX, maxY = minX, minY
	for i := 2; i+1 < len(tris); i += 2 {
		x, y := tris[i], tris[i+1]
		minX = min(minX, x)
		maxX = max(maxX, x)
		minY = min(minY, y)
		maxY = max(maxY, y)
	}
	return minX, minY, maxX, maxY
}

// resolveCanvasGradient fills in geometry the caller left degenerate
// from the bounds of the geometry being filled. See CanvasGradient for
// the defaulting rules.
func resolveCanvasGradient(g CanvasGradient,
	minX, minY, maxX, maxY float32) CanvasGradient {
	if g.Radial {
		if !(g.R > 0) { // negated > also rejects NaN
			g.CX = (minX + maxX) * 0.5
			g.CY = (minY + maxY) * 0.5
			g.R = max(maxX-minX, maxY-minY) * 0.5
			g.FX, g.FY = 0, 0
		}
		if g.FX == 0 && g.FY == 0 {
			g.FX, g.FY = g.CX, g.CY
		}
		return g
	}
	if g.X1 == g.X2 && g.Y1 == g.Y2 {
		// Top-to-bottom, matching the CSS default direction.
		g.X1, g.Y1 = (minX+maxX)*0.5, minY
		g.X2, g.Y2 = (minX+maxX)*0.5, maxY
	}
	return g
}

// canvasGradientT projects a vertex onto the gradient, returning the
// raw (unclamped, unspread) parameter.
func canvasGradientT(vx, vy float32, g *CanvasGradient) float32 {
	if g.Radial {
		r64 := float64(g.R)
		if !(g.R > 0) || math.IsInf(r64, 0) {
			return 0
		}
		dx := vx - g.FX
		dy := vy - g.FY
		t := float32(math.Sqrt(float64(dx*dx+dy*dy))) / g.R
		if t != t { // NaN
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
	t := ((vx-g.X1)*dx + (vy-g.Y1)*dy) / lenSq
	if t != t {
		return 0
	}
	return t
}

// applyCanvasSpread maps a raw gradient parameter through the spread
// method. Pad clamps, Reflect is a triangle wave, Repeat a sawtooth.
// Non-finite input folds to 0.
func applyCanvasSpread(t float32, spread GradientSpread) float32 {
	t64 := float64(t)
	if math.IsNaN(t64) || math.IsInf(t64, 0) {
		return 0
	}
	// Clamp to a range int64 conversion handles, so the reflect parity
	// test cannot hit implementation-defined overflow on hostile input.
	const spreadLimit = float64(1 << 31)
	t64 = max(-spreadLimit, min(t64, spreadLimit))
	switch spread {
	case SpreadReflect:
		n := math.Floor(t64)
		frac := float32(t64 - n)
		if int64(n)&1 != 0 {
			return 1 - frac
		}
		return frac
	case SpreadRepeat:
		n := math.Floor(t64)
		return float32(t64 - n)
	}
	return clampUnit(float32(t64))
}

// canvasStopIsolines lists the parameter values where the gradient's
// color ramp changes slope, over the raw-t range the geometry spans.
// Splitting a triangle at each of these is what makes per-vertex
// coloring exact for a linear gradient: t is affine in position, and
// between two breakpoints the color is linear in t, so Gouraud
// interpolation reproduces the ramp with no error.
func canvasStopIsolines(stops []GradientStop, spread GradientSpread,
	tMin, tMax float32, out []float32) []float32 {
	out = out[:0]
	if !isFiniteF(tMin) || !isFiniteF(tMax) || tMax < tMin {
		return out
	}
	// Pad is constant outside [0,1], so only the two clamp boundaries
	// and the interior stops break the ramp.
	if spread == SpreadPad {
		if tMin < 0 && tMax > 0 {
			out = append(out, 0)
		}
		for _, s := range stops {
			// Same ceiling the tiling branch keeps. The split pass
			// scans this list at every node of its recursion, so an
			// absurd stop count costs there, not just here.
			if len(out) >= maxCanvasStopIsolines {
				break
			}
			// A stop sitting on 0 or 1 is already covered by the clamp
			// boundary above; re-adding it would only make the split
			// pass rescan the same cut.
			if s.Pos <= 0.001 || s.Pos >= 0.999 {
				continue
			}
			if s.Pos > tMin+1e-4 && s.Pos < tMax-1e-4 {
				out = append(out, s.Pos)
			}
		}
		if tMin < 1 && tMax > 1 {
			out = append(out, 1)
		}
		return out
	}
	// Reflect and Repeat tile the ramp, so every period the geometry
	// spans contributes its own copy of the breakpoints. A period
	// boundary is itself a break (the sawtooth's step, the triangle
	// wave's fold).
	lo := math.Floor(float64(tMin))
	// Past 2^53 a float64 no longer holds consecutive integers, so a
	// float loop counter would stop advancing and never reach its
	// bound. A gradient period that small relative to the geometry has
	// no visible structure left, so there is nothing to split at.
	const maxIsolineBase = 1 << 53
	if lo < -maxIsolineBase || lo > maxIsolineBase {
		return out
	}
	// Count the periods as an int, clamped before the conversion: the
	// span can be arbitrarily large, and converting an out-of-range
	// float64 to int is undefined.
	periods := int(min(math.Ceil(float64(tMax))-lo,
		float64(maxCanvasIsolinePeriods)))
	for i := 0; i <= periods; i++ {
		k := int64(lo) + int64(i)
		base := float32(k)
		if len(out) >= maxCanvasStopIsolines {
			break
		}
		if base > tMin+1e-4 && base < tMax-1e-4 {
			out = append(out, base)
		}
		for _, s := range stops {
			if len(out) >= maxCanvasStopIsolines {
				break
			}
			// A stop on a period boundary is the boundary breakpoint
			// already emitted above.
			if s.Pos <= 0.001 || s.Pos >= 0.999 {
				continue
			}
			// Reflect mirrors on odd periods, so a stop at Pos sits at
			// 1-Pos there.
			pos := s.Pos
			if spread == SpreadReflect && k&1 != 0 {
				pos = 1 - pos
			}
			v := base + pos
			if v > tMin+1e-4 && v < tMax-1e-4 {
				out = append(out, v)
			}
		}
	}
	return out
}

// cutFraction returns where along the segment (x0,y0)-(x1,y1) the
// gradient reaches parameter tS, as a fraction in [0,1]. t0 and t1 are
// the segment's endpoint parameters.
//
// It must be a pure function of the segment — never of the triangle the
// segment belongs to — because that is what keeps the split watertight.
// Two triangles sharing an edge each decide independently whether to
// cut it and where; they agree only because both compute the same
// answer from the same two endpoints. Disagree and the mesh gets a
// T-junction, which renders as a hairline crack along the seam.
//
// For a linear gradient the parameter is affine along the segment, so
// the fraction is the obvious ratio. For a radial one it is a distance,
// and the ratio is merely close — close enough that the cut vertex does
// not land on the isoline, so the recursion cuts it again, and again,
// and neighbours diverge. Solving the quadratic puts the vertex exactly
// on the isoline instead.
func cutFraction(x0, y0, x1, y1, t0, t1, tS float32,
	g *CanvasGradient) float32 {
	if !g.Radial {
		if t1-t0 > 1e-6 || t0-t1 > 1e-6 {
			return clampUnit((tS - t0) / (t1 - t0))
		}
		return 0.5
	}
	// |A + s*(B-A) - F| = tS*R, expanded to a quadratic in s.
	ux, uy := x0-g.FX, y0-g.FY
	vx, vy := x1-x0, y1-y0
	a := vx*vx + vy*vy
	if a <= 1e-12 {
		return 0.5
	}
	d := tS * g.R
	b := 2 * (ux*vx + uy*vy)
	c := ux*ux + uy*uy - d*d
	disc := b*b - 4*a*c
	if disc < 0 {
		// The isoline misses the segment; the caller only asks when the
		// endpoints straddle it, so this is float noise, not a case.
		return clampUnit((tS - t0) / (t1 - t0 + 1e-9))
	}
	sq := float32(math.Sqrt(float64(disc)))
	s0 := (-b - sq) / (2 * a)
	s1 := (-b + sq) / (2 * a)
	// Prefer the root inside the segment. Both can be, when the segment
	// is a chord of the isoline circle; the endpoints straddle tS, so
	// exactly one root separates them, and it is the one nearer the end
	// whose parameter is on the far side.
	switch {
	case s0 >= 0 && s0 <= 1:
		return s0
	case s1 >= 0 && s1 <= 1:
		return s1
	}
	return clampUnit((tS - t0) / (t1 - t0 + 1e-9))
}

// splitCanvasTriAtStops recursively cuts a triangle along the gradient
// isolines in stopTs, appending the resulting triangles to result.
func splitCanvasTriAtStops(ax, ay, bx, by, cx, cy float32,
	g *CanvasGradient, stopTs []float32, depth int, result *[]float32) {
	// The depth cap alone bounds each triangle's own expansion, not the
	// batch's: a cut makes up to three pieces and every piece is
	// rescanned, so a large mesh crossed by many isolines composes the
	// two counts. Stop splitting once the batch has produced more
	// geometry than any fill can use. Past that point neighbours can
	// disagree about whether to cut a shared edge, which shows as a
	// hairline — a far better failure than exhausting memory, and one
	// no realistic fill reaches.
	if depth >= maxCanvasSplitDepth || len(*result) >= maxCanvasSplitFloats {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	ta := canvasGradientT(ax, ay, g)
	tb := canvasGradientT(bx, by, g)
	tc := canvasGradientT(cx, cy, g)
	tMin := min(ta, tb, tc)
	tMax := max(ta, tb, tc)

	// Cut at the isoline nearest the middle of the triangle's own range
	// rather than the first one that applies. Each cut makes up to three
	// pieces and every piece is rescanned, so an unbalanced choice —
	// shaving one thin band off the end, over and over — compounds into
	// several times the triangles a balanced one produces.
	mid := (tMin + tMax) * 0.5
	best := -1
	var bestDist float32
	for i, tS := range stopTs {
		if tS <= tMin+1e-4 || tS >= tMax-1e-4 {
			continue
		}
		d := tS - mid
		if d < 0 {
			d = -d
		}
		if best < 0 || d < bestDist {
			best, bestDist = i, d
		}
	}
	if best >= 0 {
		tS := stopTs[best]
		// Sort the vertices by t so the cut is described in one
		// orientation: the isoline always crosses edge p0-p2.
		//
		// Sorting permutes the triangle, and an odd permutation
		// reverses its winding. That must not reach the output: the
		// software rasterizer takes a whole batch as one path and
		// accumulates *signed* coverage, so a mesh of mixed winding
		// cancels itself and a glow renders as spokes radiating from
		// its hub. flip tracks the parity so every emitted triangle
		// carries the winding the caller handed in.
		flip := false
		p0x, p0y, t0 := ax, ay, ta
		p1x, p1y, t1 := bx, by, tb
		p2x, p2y, t2 := cx, cy, tc
		if t0 > t1 {
			p0x, p0y, t0, p1x, p1y, t1 = p1x, p1y, t1, p0x, p0y, t0
			flip = !flip
		}
		if t1 > t2 {
			p1x, p1y, t1, p2x, p2y, t2 = p2x, p2y, t2, p1x, p1y, t1
			flip = !flip
		}
		if t0 > t1 {
			p0x, p0y, t0, p1x, p1y, t1 = p1x, p1y, t1, p0x, p0y, t0
			flip = !flip
		}
		// emit hands one sub-triangle to the recursion, undoing the
		// sort's winding flip when there was one.
		emit := func(x0, y0, x1, y1, x2, y2 float32) {
			if flip {
				x1, y1, x2, y2 = x2, y2, x1, y1
			}
			splitCanvasTriAtStops(x0, y0, x1, y1, x2, y2,
				g, stopTs, depth+1, result)
		}

		f02 := cutFraction(p0x, p0y, p2x, p2y, t0, t2, tS, g)
		i1x := p0x + f02*(p2x-p0x)
		i1y := p0y + f02*(p2y-p0y)

		switch {
		case tS < t1-1e-4:
			// Cut also crosses p0-p1: one triangle below, two above.
			f01 := cutFraction(p0x, p0y, p1x, p1y, t0, t1, tS, g)
			i2x := p0x + f01*(p1x-p0x)
			i2y := p0y + f01*(p1y-p0y)
			emit(p0x, p0y, i2x, i2y, i1x, i1y)
			emit(i2x, i2y, p1x, p1y, i1x, i1y)
			emit(p1x, p1y, p2x, p2y, i1x, i1y)
		case tS > t1+1e-4:
			// Cut also crosses p1-p2.
			f12 := cutFraction(p1x, p1y, p2x, p2y, t1, t2, tS, g)
			i2x := p1x + f12*(p2x-p1x)
			i2y := p1y + f12*(p2y-p1y)
			emit(p0x, p0y, p1x, p1y, i1x, i1y)
			emit(p1x, p1y, i2x, i2y, i1x, i1y)
			emit(i1x, i1y, i2x, i2y, p2x, p2y)
		default:
			// Cut passes through p1: a clean two-way split.
			emit(p0x, p0y, p1x, p1y, i1x, i1y)
			emit(p1x, p1y, p2x, p2y, i1x, i1y)
		}
		return
	}
	*result = append(*result, ax, ay, bx, by, cx, cy)
}

// radialTolerance is the largest error, in gradient-parameter units,
// that a triangle may carry before the radial pass splits it. 1/256 of
// the ramp is below one step of an 8-bit channel, so the split stops as
// soon as the remaining error cannot be seen.
const radialTolerance = 1.0 / 256.0

// radialDeviation measures how far a triangle's distance field departs
// from the linear interpolation Gouraud shading will apply, in
// parameter units.
//
// Choosing the split criterion this way, rather than by absolute edge
// length, is what keeps a glow cheap. A triangle fan struck from the
// gradient's own center is already radially aligned: distance is exactly
// linear along each spoke, so the deviation is ~0 and the fan is left
// alone. A rectangle filled radially is not aligned at all — its two
// triangles bulge badly — and it subdivides until they are. An
// edge-length rule cannot tell the two apart and splits the fan into
// thousands of triangles it did not need.
//
// The measure is the error at each edge's midpoint: the true parameter
// there against the average of the edge's endpoints. That is the
// quadratic term the linear interpolation drops.
func radialDeviation(ax, ay, bx, by, cx, cy float32,
	g *CanvasGradient) float32 {
	dev := edgeDeviation(ax, ay, bx, by, g)
	dev = max(dev, edgeDeviation(bx, by, cx, cy, g))
	return max(dev, edgeDeviation(cx, cy, ax, ay, g))
}

func edgeDeviation(x0, y0, x1, y1 float32, g *CanvasGradient) float32 {
	t0 := canvasGradientT(x0, y0, g)
	t1 := canvasGradientT(x1, y1, g)
	tm := canvasGradientT((x0+x1)*0.5, (y0+y1)*0.5, g)
	d := tm - (t0+t1)*0.5
	if d < 0 {
		d = -d
	}
	return d
}

// splitCanvasRadialTri splits a triangle four ways to a fixed depth,
// appending the leaves to result.
//
// The depth is fixed for the whole batch rather than decided per
// triangle, and that is the point: midpoint subdivision is watertight
// only while neighbours split the same number of times. Let one
// triangle stop early because it happens to be flat enough and its
// neighbour's extra vertex lands in the middle of their shared edge —
// a T-junction, which renders as a hairline crack along the seam. A
// glow is 74 wedges around one hub, so cracks would radiate from it
// like spokes.
func splitCanvasRadialTri(ax, ay, bx, by, cx, cy float32,
	depth int, result *[]float32) {
	if depth <= 0 {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	mabx, maby := (ax+bx)*0.5, (ay+by)*0.5
	mbcx, mbcy := (bx+cx)*0.5, (by+cy)*0.5
	mcax, mcay := (cx+ax)*0.5, (cy+ay)*0.5
	splitCanvasRadialTri(ax, ay, mabx, maby, mcax, mcay, depth-1, result)
	splitCanvasRadialTri(mabx, maby, bx, by, mbcx, mbcy, depth-1, result)
	splitCanvasRadialTri(mcax, mcay, mbcx, mbcy, cx, cy, depth-1, result)
	splitCanvasRadialTri(mabx, maby, mbcx, mbcy, mcax, mcay, depth-1, result)
}

// radialSplitDepth picks how many times to halve every triangle in the
// batch so the worst of them carries no visible curvature error.
//
// Each level quarters the deviation, since it is a quadratic term, so
// the depth is the log4 of how far the worst triangle overshoots the
// tolerance. A fan struck from the gradient's own center is already
// radially aligned — distance is exactly linear along every spoke — so
// it measures ~0 and pays nothing. A rectangle filled radially bulges
// badly and refines until it does not.
func radialSplitDepth(tris []float32, g *CanvasGradient) int {
	var worst float32
	for i := 0; i+5 < len(tris); i += 6 {
		worst = max(worst, radialDeviation(tris[i], tris[i+1], tris[i+2],
			tris[i+3], tris[i+4], tris[i+5], g))
	}
	depth := 0
	for worst > radialTolerance && depth < maxCanvasRadialDepth {
		worst *= 0.25
		depth++
	}
	// Each level quadruples the whole batch, so the per-triangle depth
	// has to answer for how many triangles there are. Back it off until
	// the batch's total fits the budget; a fan or a rect is nowhere
	// near it, so this only bites on geometry already dense enough that
	// another level would not be visible anyway.
	srcTris := len(tris) / 6
	for depth > 0 && srcTris > maxCanvasRadialTris>>(2*depth) {
		depth--
	}
	return depth
}

// subdivideCanvasGradientTris splits tris so per-vertex coloring can
// represent the gradient. Returns tris unchanged when no split is
// needed, so a two-stop linear fill costs nothing.
func subdivideCanvasGradientTris(tris []float32, g *CanvasGradient,
	scratch, radialScratch, isolines *[]float32) []float32 {
	// Geometric refinement first, and only for a radial gradient: its
	// parameter is a distance, so it is not affine in position, and a
	// triangle spanning a wide angle would have its falloff flattened
	// into a wash. Doing it before the isoline pass keeps it cheap —
	// it refines a handful of source triangles rather than the hundreds
	// the isoline pass produces.
	in := tris
	if g.Radial {
		r64 := float64(g.R)
		if !(g.R > 0) || math.IsInf(r64, 0) {
			return tris
		}
		if depth := radialSplitDepth(tris, g); depth > 0 {
			*radialScratch = (*radialScratch)[:0]
			for i := 0; i+5 < len(tris); i += 6 {
				splitCanvasRadialTri(tris[i], tris[i+1], tris[i+2],
					tris[i+3], tris[i+4], tris[i+5], depth, radialScratch)
			}
			in = *radialScratch
		}
	}

	// Then cut where the ramp changes slope. For a linear gradient this
	// is the whole job and it is exact: the parameter is affine in
	// position, so between two stops vertex interpolation reproduces
	// the ramp perfectly and a two-stop fill costs nothing at all.
	tMin, tMax := canvasTRange(in, g)
	*isolines = canvasStopIsolines(g.Stops, g.Spread, tMin, tMax, *isolines)
	if len(*isolines) == 0 {
		return in
	}
	*scratch = (*scratch)[:0]
	for i := 0; i+5 < len(in); i += 6 {
		splitCanvasTriAtStops(in[i], in[i+1], in[i+2], in[i+3],
			in[i+4], in[i+5], g, *isolines, 0, scratch)
	}
	return *scratch
}

// canvasTRange returns the raw parameter range the geometry spans.
func canvasTRange(tris []float32, g *CanvasGradient) (float32, float32) {
	if len(tris) < 2 {
		return 0, 0
	}
	t := canvasGradientT(tris[0], tris[1], g)
	tMin, tMax := t, t
	for i := 2; i+1 < len(tris); i += 2 {
		t = canvasGradientT(tris[i], tris[i+1], g)
		tMin = min(tMin, t)
		tMax = max(tMax, t)
	}
	return tMin, tMax
}
