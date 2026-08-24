// Package gradmesh tessellates a triangle mesh so per-vertex (Gouraud)
// coloring can reproduce a gradient ramp exactly.
//
// It is the one implementation shared by the two gradient fills in this
// module: gui's DrawContext gradients (gui/canvas_gradient.go) and
// gui/svg's path gradients (gui/svg/tessellate_gradient.go). Both used to
// carry their own copy of this math, kept in step by hand; the copies
// drifted and shipped a defect (see issue #415).
//
// Nothing here knows about color. The input is a flat x,y triangle list
// plus scalar gradient geometry and the stops' *offsets*, and the output
// is a flat triangle list. Each caller runs its own coloring pass over
// the result, sampling RawT + SpreadTri per triangle.
//
// Every entry point that produces geometry takes the destination slice
// from the caller, so buffer policy is a caller decision: gui reuses
// per-frame scratch, gui/svg allocates because it retains the result in a
// cached asset.
package gradmesh

import "math"

// Spread selects how a gradient handles parameter values outside [0,1].
// Pad clamps (the zero value), Reflect mirrors, Repeat wraps.
type Spread uint8

// Spread values.
const (
	SpreadPad Spread = iota
	SpreadReflect
	SpreadRepeat
)

// Params is the gradient geometry the tessellation reads: the neutral
// form both callers convert their own gradient type into.
//
// StopOffsets holds only the stops' positions along the ramp, in the
// caller's order. The tessellation reads nothing else from a stop, which
// is why no stop or color type crosses this boundary.
type Params struct {
	StopOffsets []float32
	X1, Y1      float32 // linear: start point
	X2, Y2      float32 // linear: end point
	CX, CY, R   float32 // radial: center and radius
	FX, FY      float32 // radial: focal point
	Radial      bool
	Spread      Spread
}

// maxStopIsolines caps how many parameter-space breakpoints a linear
// subdivision may split at. Reflect and Repeat generate one set of
// breakpoints per period the geometry spans, so a gradient whose
// endpoints are tiny relative to the fill would otherwise produce an
// unbounded split list.
const maxStopIsolines = 256

// maxIsolinePeriods bounds how many gradient periods a Repeat or Reflect
// fill may enumerate breakpoints for.
const maxIsolinePeriods = 64

// maxSplitDepth bounds the linear stop-isoline recursion, and
// maxRadialDepth the radial edge-length recursion.
const (
	maxSplitDepth  = 8
	maxRadialDepth = 6
)

// maxSplitFloats caps the geometry one gradient fill may expand to, and
// maxRadialTris the triangle count the radial pass may quadruple its way
// to. Both are ceilings for hostile or careless input — hundreds of stops
// over a large mesh — not tuning knobs: a glow or an icon's gradient
// lands orders of magnitude below either.
const (
	maxSplitFloats = 1 << 20 // ~175k triangles, 4 MB
	maxRadialTris  = 1 << 16
)

// radialTolerance is the largest error, in gradient-parameter units, that
// a triangle may carry before the radial pass splits it. 1/256 of the
// ramp is below one step of an 8-bit channel, so the split stops as soon
// as the remaining error cannot be seen.
const radialTolerance = 1.0 / 256.0

func clampUnit(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// finite reports whether v is a finite float32 (not NaN or Inf).
func finite(v float32) bool {
	v64 := float64(v)
	return !math.IsNaN(v64) && !math.IsInf(v64, 0)
}

// RawT projects a vertex onto the gradient, returning the raw (unclamped,
// unspread) parameter. Both the split passes and the callers' coloring
// passes read the ramp through this.
func RawT(vx, vy float32, p *Params) float32 {
	if p.Radial {
		r64 := float64(p.R)
		if !(p.R > 0) || math.IsInf(r64, 0) { // negated > also rejects NaN
			return 0
		}
		dx := vx - p.FX
		dy := vy - p.FY
		t := float32(math.Sqrt(float64(dx*dx+dy*dy))) / p.R
		if t != t { // NaN
			return 0
		}
		return t
	}
	dx := p.X2 - p.X1
	dy := p.Y2 - p.Y1
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return 0
	}
	t := ((vx-p.X1)*dx + (vy-p.Y1)*dy) / lenSq
	if t != t {
		return 0
	}
	return t
}

// ApplySpread maps a raw gradient parameter through the spread method.
// Pad clamps, Reflect is a triangle wave, Repeat a sawtooth. Non-finite
// input folds to 0.
//
// This is the right call for a lone point — hit testing, a sampled
// midpoint. A coloring pass over a tessellated mesh wants SpreadTri
// instead, which reads a triangle's three vertices in one period.
func ApplySpread(t float32, spread Spread) float32 {
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

// foldToPeriod maps one raw parameter into the period starting at k.
//
// The clamp is what handles the one case the split pass cannot: a
// triangle still spanning periods because the depth or float budget ran
// out. Flattening that overshoot beats wrapping it, which would run a
// whole extra ramp across a single triangle.
func foldToPeriod(t float32, k float64, mirror bool) float32 {
	s := clampUnit(float32(float64(t) - k))
	if mirror {
		return 1 - s
	}
	return s
}

// SpreadTri resolves the spread for one triangle's three vertices as a
// group. A coloring pass should call this rather than ApplySpread per
// vertex: ApplySpread is a pure function of one scalar, so it cannot tell
// which side of a break in the ramp the vertex's own triangle lies on.
//
// That matters for Repeat, whose sawtooth jumps from the ramp's end back
// to its start at every integer — and the split pass places cut vertices
// exactly on those folds. Such a vertex resolves to frac 0, the ramp's
// start, while the triangle below it needs the limit from inside, the
// ramp's end. Gouraud then carries that wrong endpoint across the whole
// triangle (issue #417).
//
// The split pass leaves no triangle spanning a fold, so the triangle
// names one period and all three of its vertices are read in it. Inside a
// period this is identical to ApplySpread, because the clamp is then a
// no-op; on a fold it takes the interior limit instead of the far side's.
// Reflect's triangle wave and Pad's clamp are both continuous at their
// breaks, so neither moves.
func SpreadTri(ta, tb, tc float32, spread Spread) (float32, float32, float32) {
	// Pad and any out-of-range value delegate: ApplySpread clamps those,
	// and treating an unknown value as repeat here would wrap where the
	// per-vertex read pads. Non-finite vertices delegate too: ApplySpread
	// folds them to 0.
	if (spread != SpreadReflect && spread != SpreadRepeat) ||
		!finite(ta) || !finite(tb) || !finite(tc) {
		return ApplySpread(ta, spread), ApplySpread(tb, spread),
			ApplySpread(tc, spread)
	}
	// The same clamp ApplySpread keeps, for the same reason: the parity
	// test below converts to int64, which is undefined out of range.
	const spreadLimit = float64(1 << 31)
	lo := max(-spreadLimit, min(float64(min(ta, tb, tc)), spreadLimit))
	hi := max(-spreadLimit, min(float64(max(ta, tb, tc)), spreadLimit))
	// Take the period from the range's midpoint rather than from any one
	// vertex, so a vertex sitting on either end of the period is read
	// from inside the triangle either way.
	k := math.Floor((lo + hi) * 0.5)
	mirror := spread == SpreadReflect && int64(k)&1 != 0
	return foldToPeriod(ta, k, mirror),
		foldToPeriod(tb, k, mirror),
		foldToPeriod(tc, k, mirror)
}

// stopIsolines lists the parameter values where the gradient's color ramp
// changes slope, over the raw-t range the geometry spans. Splitting a
// triangle at each of these is what makes per-vertex coloring exact for a
// linear gradient: t is affine in position, and between two breakpoints
// the color is linear in t, so Gouraud interpolation reproduces the ramp
// with no error.
//
// The list is built in raw-t space, the same space the callers' coloring
// passes read through ApplySpread. Splitting on the clamped projection
// instead puts the cuts nowhere near the ramp's breakpoints for reflect
// and repeat.
func stopIsolines(offsets []float32, spread Spread,
	tMin, tMax float32, out []float32) []float32 {
	out = out[:0]
	if !finite(tMin) || !finite(tMax) || tMax < tMin {
		return out
	}
	// Pad is constant outside [0,1], so only the two clamp boundaries
	// and the interior stops break the ramp.
	if spread == SpreadPad {
		if tMin < 0 && tMax > 0 {
			out = append(out, 0)
		}
		for _, pos := range offsets {
			// Same ceiling the tiling branch keeps. The split pass
			// scans this list at every node of its recursion, so an
			// absurd stop count costs there, not just here.
			if len(out) >= maxStopIsolines {
				break
			}
			// A stop sitting on 0 or 1 is already covered by the clamp
			// boundary above; re-adding it would only make the split
			// pass rescan the same cut.
			if pos <= 0.001 || pos >= 0.999 {
				continue
			}
			if pos > tMin+1e-4 && pos < tMax-1e-4 {
				out = append(out, pos)
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
		float64(maxIsolinePeriods)))
	for i := 0; i <= periods; i++ {
		k := int64(lo) + int64(i)
		base := float32(k)
		if len(out) >= maxStopIsolines {
			break
		}
		if base > tMin+1e-4 && base < tMax-1e-4 {
			out = append(out, base)
		}
		for _, off := range offsets {
			if len(out) >= maxStopIsolines {
				break
			}
			// A stop on a period boundary is the boundary breakpoint
			// already emitted above.
			if off <= 0.001 || off >= 0.999 {
				continue
			}
			// Reflect mirrors on odd periods, so a stop at off sits at
			// 1-off there.
			pos := off
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
func cutFraction(x0, y0, x1, y1, t0, t1, tS float32, p *Params) float32 {
	if !p.Radial {
		if t1-t0 > 1e-6 || t0-t1 > 1e-6 {
			return clampUnit((tS - t0) / (t1 - t0))
		}
		return 0.5
	}
	// |A + s*(B-A) - F| = tS*R, expanded to a quadratic in s.
	ux, uy := x0-p.FX, y0-p.FY
	vx, vy := x1-x0, y1-y0
	a := vx*vx + vy*vy
	if a <= 1e-12 {
		return 0.5
	}
	d := tS * p.R
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

// splitTriAtStops recursively cuts a triangle along the gradient isolines
// in stopTs, appending the resulting triangles to result.
func splitTriAtStops(ax, ay, bx, by, cx, cy float32,
	p *Params, stopTs []float32, depth int, result *[]float32) {
	// The depth cap alone bounds each triangle's own expansion, not the
	// batch's: a cut makes up to three pieces and every piece is
	// rescanned, so a large mesh crossed by many isolines composes the
	// two counts. Stop splitting once the batch has produced more
	// geometry than any fill can use. Past that point neighbours can
	// disagree about whether to cut a shared edge, which shows as a
	// hairline — a far better failure than exhausting memory, and one
	// no realistic fill reaches.
	if depth >= maxSplitDepth || len(*result) >= maxSplitFloats {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	ta := RawT(ax, ay, p)
	tb := RawT(bx, by, p)
	tc := RawT(cx, cy, p)
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
	if best < 0 {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	tS := stopTs[best]

	// Sort the vertices by t so the cut is described in one orientation:
	// the isoline always crosses edge p0-p2.
	//
	// Sorting permutes the triangle, and an odd permutation reverses its
	// winding. That must not reach the output: the software rasterizer
	// takes a whole batch as one path and accumulates *signed* coverage,
	// so a mesh of mixed winding cancels itself along every internal
	// seam and a glow renders as spokes radiating from its hub. flip
	// tracks the parity so every emitted triangle carries the winding
	// the caller handed in.
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
	// emit hands one sub-triangle to the recursion, undoing the sort's
	// winding flip when there was one.
	emit := func(x0, y0, x1, y1, x2, y2 float32) {
		if flip {
			x1, y1, x2, y2 = x2, y2, x1, y1
		}
		splitTriAtStops(x0, y0, x1, y1, x2, y2,
			p, stopTs, depth+1, result)
	}

	f02 := cutFraction(p0x, p0y, p2x, p2y, t0, t2, tS, p)
	i1x := p0x + f02*(p2x-p0x)
	i1y := p0y + f02*(p2y-p0y)

	switch {
	case tS < t1-1e-4:
		// Cut also crosses p0-p1: one triangle below, two above.
		f01 := cutFraction(p0x, p0y, p1x, p1y, t0, t1, tS, p)
		i2x := p0x + f01*(p1x-p0x)
		i2y := p0y + f01*(p1y-p0y)
		emit(p0x, p0y, i2x, i2y, i1x, i1y)
		emit(i2x, i2y, p1x, p1y, i1x, i1y)
		emit(p1x, p1y, p2x, p2y, i1x, i1y)
	case tS > t1+1e-4:
		// Cut also crosses p1-p2.
		f12 := cutFraction(p1x, p1y, p2x, p2y, t1, t2, tS, p)
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
}

// edgeDeviation measures how far the gradient parameter along one edge
// departs from the linear interpolation Gouraud shading applies: the true
// parameter at the edge's midpoint against the average of its endpoints.
// That is the quadratic term the linear interpolation drops.
func edgeDeviation(x0, y0, x1, y1 float32, p *Params) float32 {
	t0 := RawT(x0, y0, p)
	t1 := RawT(x1, y1, p)
	tm := RawT((x0+x1)*0.5, (y0+y1)*0.5, p)
	d := tm - (t0+t1)*0.5
	if d < 0 {
		d = -d
	}
	return d
}

// radialDeviation is the worst of a triangle's three edge deviations, in
// parameter units.
//
// Choosing the split criterion this way, rather than by absolute edge
// length, is what keeps a radial fill cheap. A triangle fan struck from
// the gradient's own center is already radially aligned: distance is
// exactly linear along each spoke, so the deviation is ~0 and the fan is
// left alone. A rectangle filled radially is not aligned at all — its two
// triangles bulge badly — and it subdivides until they are. An
// edge-length rule cannot tell the two apart and splits the fan into
// thousands of triangles it did not need.
func radialDeviation(ax, ay, bx, by, cx, cy float32, p *Params) float32 {
	dev := edgeDeviation(ax, ay, bx, by, p)
	dev = max(dev, edgeDeviation(bx, by, cx, cy, p))
	return max(dev, edgeDeviation(cx, cy, ax, ay, p))
}

// splitRadialTri splits a triangle four ways to a fixed depth, appending
// the leaves to result.
//
// The depth is fixed for the whole batch rather than decided per
// triangle, and that is the point: midpoint subdivision is watertight
// only while neighbours split the same number of times. Let one triangle
// stop early because it happens to be flat enough and its neighbour's
// extra vertex lands in the middle of their shared edge — a T-junction,
// which renders as a hairline crack along the seam. A glow is 74 wedges
// around one hub, so cracks would radiate from it like spokes.
func splitRadialTri(ax, ay, bx, by, cx, cy float32,
	depth int, result *[]float32) {
	if depth <= 0 {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	mabx, maby := (ax+bx)*0.5, (ay+by)*0.5
	mbcx, mbcy := (bx+cx)*0.5, (by+cy)*0.5
	mcax, mcay := (cx+ax)*0.5, (cy+ay)*0.5
	splitRadialTri(ax, ay, mabx, maby, mcax, mcay, depth-1, result)
	splitRadialTri(mabx, maby, bx, by, mbcx, mbcy, depth-1, result)
	splitRadialTri(mcax, mcay, mbcx, mbcy, cx, cy, depth-1, result)
	splitRadialTri(mabx, maby, mbcx, mbcy, mcax, mcay, depth-1, result)
}

// radialSplitDepth picks how many times to halve every triangle in the
// batch so the worst of them carries no visible curvature error.
//
// Each level quarters the deviation, since it is a quadratic term, so the
// depth is the log4 of how far the worst triangle overshoots the
// tolerance. A fan struck from the gradient's own center is already
// radially aligned — distance is exactly linear along every spoke — so it
// measures ~0 and pays nothing. A rectangle filled radially bulges badly
// and refines until it does not.
func radialSplitDepth(tris []float32, p *Params) int {
	var worst float32
	for i := 0; i+5 < len(tris); i += 6 {
		worst = max(worst, radialDeviation(tris[i], tris[i+1], tris[i+2],
			tris[i+3], tris[i+4], tris[i+5], p))
	}
	depth := 0
	for worst > radialTolerance && depth < maxRadialDepth {
		worst *= 0.25
		depth++
	}
	// Each level quadruples the whole batch, so the per-triangle depth
	// has to answer for how many triangles there are. Back it off until
	// the batch's total fits the budget; a fan or a rect is nowhere near
	// it, so this only bites on geometry already dense enough that
	// another level would not be visible anyway.
	srcTris := len(tris) / 6
	for depth > 0 && srcTris > maxRadialTris>>(2*depth) {
		depth--
	}
	return depth
}

// tRange returns the raw parameter range the geometry spans.
func tRange(tris []float32, p *Params) (float32, float32) {
	if len(tris) < 2 {
		return 0, 0
	}
	t := RawT(tris[0], tris[1], p)
	tMin, tMax := t, t
	for i := 2; i+1 < len(tris); i += 2 {
		t = RawT(tris[i], tris[i+1], p)
		tMin = min(tMin, t)
		tMax = max(tMax, t)
	}
	return tMin, tMax
}

// Subdivide splits tris so per-vertex coloring can represent the
// gradient. It returns tris unchanged when no split is needed, so a
// two-stop linear fill costs nothing.
//
// split, radial and isolines are caller-owned scratch: split receives the
// isoline pass's output, radial the geometric pass's, and isolines the
// breakpoint list. Each is reset before use, and the returned slice
// aliases either tris or one of them — so a caller that retains the
// result must not reuse the buffer it came from.
func Subdivide(tris []float32, p *Params,
	split, radial, isolines *[]float32) []float32 {
	// Geometric refinement first, and only for a radial gradient: its
	// parameter is a distance, so it is not affine in position, and a
	// triangle spanning a wide angle would have its falloff flattened
	// into a wash. Doing it before the isoline pass keeps it cheap — it
	// refines a handful of source triangles rather than the hundreds the
	// isoline pass produces.
	in := tris
	if p.Radial {
		r64 := float64(p.R)
		if !(p.R > 0) || math.IsInf(r64, 0) {
			return tris
		}
		if depth := radialSplitDepth(tris, p); depth > 0 {
			*radial = (*radial)[:0]
			for i := 0; i+5 < len(tris); i += 6 {
				splitRadialTri(tris[i], tris[i+1], tris[i+2],
					tris[i+3], tris[i+4], tris[i+5], depth, radial)
			}
			in = *radial
		}
	}

	// Then cut where the ramp changes slope. For a linear gradient this
	// is the whole job and it is exact: the parameter is affine in
	// position, so between two stops vertex interpolation reproduces the
	// ramp perfectly and a two-stop fill costs nothing at all.
	tMin, tMax := tRange(in, p)
	*isolines = stopIsolines(p.StopOffsets, p.Spread, tMin, tMax, *isolines)
	if len(*isolines) == 0 {
		return in
	}
	*split = (*split)[:0]
	for i := 0; i+5 < len(in); i += 6 {
		splitTriAtStops(in[i], in[i+1], in[i+2], in[i+3],
			in[i+4], in[i+5], p, *isolines, 0, split)
	}
	return *split
}
