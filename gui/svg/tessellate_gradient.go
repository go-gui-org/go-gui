package svg

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// tessellate_gradient.go — gradient fills for SVG paths.
//
// The projection and subdivision here deliberately mirror
// gui/canvas_gradient.go, which does the same job for DrawContext
// gradients. They are not shared code and cannot be: gui/svg imports
// gui, so the dependency cannot run the other way, and the two sides
// disagree on the types anyway (SvgColor vs Color, SvgGradientStop's
// Offset vs GradientStop's Pos). Keep the two in sync by hand — a
// change to the falloff or subdivision math in one almost certainly
// wants the same change in the other.
//
// One divergence is deliberate. The canvas twin threads caller scratch
// buffers through the split passes to avoid allocating per fill; this
// side cannot, because tessellate.go retains the returned slice in
// TessellatedPath.Triangles for the lifetime of the cached asset, so a
// shared buffer would alias across paths. The canvas batch is consumed
// within the frame that builds it, which is why it can.

// --- Gradient support ---

// resolveGradient rewrites an objectBoundingBox gradient into user
// space against the geometry's bounds. Everything the def carries that
// is not geometry — the stops and the spread method — passes through
// untouched; dropping the spread here would silently pad every gradient
// that uses the default units, which is most of them.
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
			SpreadMethod:  g.SpreadMethod,
			GradientUnits: "userSpaceOnUse",
		}
	}
	return gui.SvgGradientDef{
		Stops:         g.Stops,
		X1:            minX + g.X1*w,
		Y1:            minY + g.Y1*h,
		X2:            minX + g.X2*w,
		Y2:            minY + g.Y2*h,
		SpreadMethod:  g.SpreadMethod,
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
	return svgRawT(vx, vy, &g)
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

// maxSvgStopIsolines caps how many parameter-space breakpoints a
// subdivision may split at. Reflect and Repeat generate one set of
// breakpoints per period the geometry spans, so a gradient whose
// endpoints are tiny relative to the fill would otherwise produce an
// unbounded split list.
const maxSvgStopIsolines = 256

// maxSvgIsolinePeriods bounds how many gradient periods a Repeat or
// Reflect fill may enumerate breakpoints for.
const maxSvgIsolinePeriods = 64

// maxSvgSplitFloats caps the geometry one gradient fill may expand to,
// and maxSvgRadialTris the triangle count the radial pass may quadruple
// its way to. Both are ceilings for hostile or careless input —
// hundreds of stops over a large mesh — not tuning knobs: an icon's
// gradient lands orders of magnitude below either.
const (
	maxSvgSplitFloats = 1 << 20 // ~175k triangles, 4 MB
	maxSvgRadialTris  = 1 << 16
)

// maxRadialDepth bounds the radial curvature recursion.
const maxRadialDepth = 6

// svgRadialTolerance is the largest error, in gradient-parameter units,
// that a triangle may carry before the radial pass splits it. 1/256 of
// the ramp is below one step of an 8-bit channel, so the split stops as
// soon as the remaining error cannot be seen.
const svgRadialTolerance = 1.0 / 256.0

func clampUnit(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// svgRawT is projectOntoGradientRaw over a pointer, so the split
// recursions below do not copy the gradient at every vertex.
func svgRawT(vx, vy float32, g *gui.SvgGradientDef) float32 {
	if g.IsRadial {
		r64 := float64(g.R)
		if g.R <= 0 || math.IsNaN(r64) || math.IsInf(r64, 0) {
			return 0
		}
		dx := vx - g.FX
		dy := vy - g.FY
		t := float32(math.Sqrt(float64(dx*dx+dy*dy))) / g.R
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
	t := ((vx-g.X1)*dx + (vy-g.Y1)*dy) / lenSq
	if t != t {
		return 0
	}
	return t
}

// svgStopIsolines lists the parameter values where the gradient's color
// ramp changes slope, over the raw-t range the geometry spans.
// Splitting a triangle at each of these is what makes per-vertex
// coloring exact for a linear gradient: t is affine in position, and
// between two breakpoints the color is linear in t, so Gouraud
// interpolation reproduces the ramp with no error.
//
// The list is built in raw-t space, the same space the coloring pass
// reads through projectAndSpread. Splitting on the clamped projection
// instead — as this file did before — puts the cuts nowhere near the
// ramp's breakpoints for reflect and repeat.
func svgStopIsolines(stops []gui.SvgGradientStop, spread gui.SvgGradientSpread,
	tMin, tMax float32, out []float32) []float32 {
	out = out[:0]
	if !finiteF32(tMin) || !finiteF32(tMax) || tMax < tMin {
		return out
	}
	// Pad is constant outside [0,1], so only the two clamp boundaries
	// and the interior stops break the ramp. Anything that is not
	// reflect or repeat pads, matching applySpread's default arm.
	if spread != gui.SvgSpreadReflect && spread != gui.SvgSpreadRepeat {
		if tMin < 0 && tMax > 0 {
			out = append(out, 0)
		}
		for _, s := range stops {
			// Same ceiling the tiling branch keeps. The split pass
			// scans this list at every node of its recursion, so an
			// absurd stop count costs there, not just here.
			if len(out) >= maxSvgStopIsolines {
				break
			}
			// A stop sitting on 0 or 1 is already covered by the clamp
			// boundary above; re-adding it would only make the split
			// pass rescan the same cut.
			if s.Offset <= 0.001 || s.Offset >= 0.999 {
				continue
			}
			if s.Offset > tMin+1e-4 && s.Offset < tMax-1e-4 {
				out = append(out, s.Offset)
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
		float64(maxSvgIsolinePeriods)))
	for i := 0; i <= periods; i++ {
		k := int64(lo) + int64(i)
		base := float32(k)
		if len(out) >= maxSvgStopIsolines {
			break
		}
		if base > tMin+1e-4 && base < tMax-1e-4 {
			out = append(out, base)
		}
		for _, s := range stops {
			if len(out) >= maxSvgStopIsolines {
				break
			}
			// A stop on a period boundary is the boundary breakpoint
			// already emitted above.
			if s.Offset <= 0.001 || s.Offset >= 0.999 {
				continue
			}
			// Reflect mirrors on odd periods, so a stop at Offset sits
			// at 1-Offset there.
			pos := s.Offset
			if spread == gui.SvgSpreadReflect && k&1 != 0 {
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

// svgCutFraction returns where along the segment (x0,y0)-(x1,y1) the
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
func svgCutFraction(x0, y0, x1, y1, t0, t1, tS float32,
	g *gui.SvgGradientDef) float32 {
	if !g.IsRadial {
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

// splitTriAtStops recursively cuts a triangle along the gradient
// isolines in stopTs, appending the resulting triangles to result.
func splitTriAtStops(ax, ay, bx, by, cx, cy float32,
	grad *gui.SvgGradientDef, stopTs []float32, depth int, result *[]float32) {
	// The depth cap alone bounds each triangle's own expansion, not the
	// batch's: a cut makes up to three pieces and every piece is
	// rescanned, so a large mesh crossed by many isolines composes the
	// two counts. Stop splitting once the batch has produced more
	// geometry than any fill can use. Past that point neighbours can
	// disagree about whether to cut a shared edge, which shows as a
	// hairline — a far better failure than exhausting memory, and one
	// no realistic fill reaches.
	if depth >= maxSplitTriDepth || len(*result) >= maxSvgSplitFloats {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	ta := svgRawT(ax, ay, grad)
	tb := svgRawT(bx, by, grad)
	tc := svgRawT(cx, cy, grad)
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

	// Sort the vertices by t so the cut is described in one
	// orientation: the isoline always crosses edge p0-p2.
	//
	// Sorting permutes the triangle, and an odd permutation reverses
	// its winding. That must not reach the output: the software
	// rasterizer takes a whole batch as one path and accumulates
	// *signed* coverage, so a mesh of mixed winding cancels itself
	// along every internal seam. flip tracks the parity so every
	// emitted triangle carries the winding the caller handed in.
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
			grad, stopTs, depth+1, result)
	}

	f02 := svgCutFraction(p0x, p0y, p2x, p2y, t0, t2, tS, grad)
	i1x := p0x + f02*(p2x-p0x)
	i1y := p0y + f02*(p2y-p0y)

	switch {
	case tS < t1-1e-4:
		// Cut also crosses p0-p1: one triangle below, two above.
		f01 := svgCutFraction(p0x, p0y, p1x, p1y, t0, t1, tS, grad)
		i2x := p0x + f01*(p1x-p0x)
		i2y := p0y + f01*(p1y-p0y)
		emit(p0x, p0y, i2x, i2y, i1x, i1y)
		emit(i2x, i2y, p1x, p1y, i1x, i1y)
		emit(p1x, p1y, p2x, p2y, i1x, i1y)
	case tS > t1+1e-4:
		// Cut also crosses p1-p2.
		f12 := svgCutFraction(p1x, p1y, p2x, p2y, t1, t2, tS, grad)
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

// svgEdgeDeviation measures how far the gradient parameter along one
// edge departs from the linear interpolation Gouraud shading applies:
// the true parameter at the edge's midpoint against the average of its
// endpoints. That is the quadratic term the linear interpolation drops.
func svgEdgeDeviation(x0, y0, x1, y1 float32, g *gui.SvgGradientDef) float32 {
	t0 := svgRawT(x0, y0, g)
	t1 := svgRawT(x1, y1, g)
	tm := svgRawT((x0+x1)*0.5, (y0+y1)*0.5, g)
	d := tm - (t0+t1)*0.5
	if d < 0 {
		d = -d
	}
	return d
}

// svgRadialDeviation is the worst of a triangle's three edge
// deviations, in parameter units.
//
// Choosing the split criterion this way, rather than by absolute edge
// length, is what keeps a radial fill cheap. A triangle fan struck from
// the gradient's own center is already radially aligned: distance is
// exactly linear along each spoke, so the deviation is ~0 and the fan
// is left alone. A rectangle filled radially is not aligned at all —
// its two triangles bulge badly — and it subdivides until they are. An
// edge-length rule cannot tell the two apart and splits the fan into
// thousands of triangles it did not need.
func svgRadialDeviation(ax, ay, bx, by, cx, cy float32,
	g *gui.SvgGradientDef) float32 {
	dev := svgEdgeDeviation(ax, ay, bx, by, g)
	dev = max(dev, svgEdgeDeviation(bx, by, cx, cy, g))
	return max(dev, svgEdgeDeviation(cx, cy, ax, ay, g))
}

// svgRadialSplitDepth picks how many times to halve every triangle in
// the batch so the worst of them carries no visible curvature error.
//
// Each level quarters the deviation, since it is a quadratic term, so
// the depth is the log4 of how far the worst triangle overshoots the
// tolerance.
func svgRadialSplitDepth(tris []float32, g *gui.SvgGradientDef) int {
	var worst float32
	for i := 0; i+5 < len(tris); i += 6 {
		worst = max(worst, svgRadialDeviation(tris[i], tris[i+1], tris[i+2],
			tris[i+3], tris[i+4], tris[i+5], g))
	}
	depth := 0
	for worst > svgRadialTolerance && depth < maxRadialDepth {
		worst *= 0.25
		depth++
	}
	// Each level quadruples the whole batch, so the per-triangle depth
	// has to answer for how many triangles there are. Back it off until
	// the batch's total fits the budget; a fan or a rect is nowhere
	// near it, so this only bites on geometry already dense enough that
	// another level would not be visible anyway.
	srcTris := len(tris) / 6
	for depth > 0 && srcTris > maxSvgRadialTris>>(2*depth) {
		depth--
	}
	return depth
}

// splitRadialTri splits a triangle four ways to a fixed depth,
// appending the leaves to result.
//
// The depth is fixed for the whole batch rather than decided per
// triangle, and that is the point: midpoint subdivision is watertight
// only while neighbours split the same number of times. Let one
// triangle stop early because it happens to be flat enough and its
// neighbour's extra vertex lands in the middle of their shared edge —
// a T-junction, which renders as a hairline crack along the seam.
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

// subdivideRadialTris refines tris until per-vertex coloring can
// represent the radial falloff. Sampling only at the vertices
// interpolates linearly across the triangle, which flattens the
// falloff for geometry that bulges across the isolines (a 100x100 rect
// filled by 2 triangles reads as a wash). Geometry already aligned to
// the isolines measures ~0 deviation and is returned unchanged.
func subdivideRadialTris(tris []float32, grad gui.SvgGradientDef) []float32 {
	// Guard against non-finite R: NaN survives every comparison below
	// and would defeat the depth heuristic.
	r64 := float64(grad.R)
	if math.IsNaN(r64) || math.IsInf(r64, 0) || grad.R <= 0 {
		return tris
	}
	if len(tris) < 6 {
		return nil
	}
	depth := svgRadialSplitDepth(tris, &grad)
	if depth <= 0 {
		return tris
	}
	result := make([]float32, 0, (len(tris)/6)*6<<(2*depth))
	for i := 0; i+5 < len(tris); i += 6 {
		splitRadialTri(tris[i], tris[i+1], tris[i+2], tris[i+3],
			tris[i+4], tris[i+5], depth, &result)
	}
	return result
}

// subdivideGradientTris splits tris so per-vertex coloring can
// represent the gradient. Returns tris unchanged when no split is
// needed.
func subdivideGradientTris(tris []float32, grad gui.SvgGradientDef) []float32 {
	if len(tris) < 6 {
		return nil
	}
	// Geometric refinement first, and only for a radial gradient: its
	// parameter is a distance, so it is not affine in position. Doing
	// it before the isoline pass keeps it cheap — it refines a handful
	// of source triangles rather than the hundreds the isoline pass
	// produces.
	in := tris
	if grad.IsRadial {
		r64 := float64(grad.R)
		if math.IsNaN(r64) || math.IsInf(r64, 0) || grad.R <= 0 {
			return tris
		}
		in = subdivideRadialTris(tris, grad)
	}

	// Then cut where the ramp changes slope. For a linear gradient this
	// is the whole job and it is exact: the parameter is affine in
	// position, so between two stops vertex interpolation reproduces
	// the ramp perfectly.
	tMin, tMax := svgTRange(in, &grad)
	isolines := svgStopIsolines(grad.Stops, grad.SpreadMethod, tMin, tMax, nil)
	if len(isolines) == 0 {
		return in
	}
	result := make([]float32, 0, len(in)*2)
	for i := 0; i+5 < len(in); i += 6 {
		splitTriAtStops(in[i], in[i+1], in[i+2], in[i+3], in[i+4], in[i+5],
			&grad, isolines, 0, &result)
	}
	return result
}

// svgTRange returns the raw parameter range the geometry spans.
func svgTRange(tris []float32, g *gui.SvgGradientDef) (float32, float32) {
	if len(tris) < 2 {
		return 0, 0
	}
	t := svgRawT(tris[0], tris[1], g)
	tMin, tMax := t, t
	for i := 2; i+1 < len(tris); i += 2 {
		t = svgRawT(tris[i], tris[i+1], g)
		tMin = min(tMin, t)
		tMax = max(tMax, t)
	}
	return tMin, tMax
}
