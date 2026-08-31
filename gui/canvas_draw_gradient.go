package gui

import (
	"math"

	"github.com/go-gui-org/go-gui/gui/internal/gradmesh"
)

// canvas_draw_gradient.go — the DrawContext gradient fills.
//
// Each method tessellates through the same helpers its flat twin uses,
// so a gradient fill and a flat fill of the same shape produce the same
// vertices; only the coloring differs. The geometry lands in a scratch
// buffer first, because the vertex colors cannot be assigned until the
// subdivision pass has decided how many vertices there are.

// FillTrianglesGradient fills caller-supplied geometry with a gradient.
// tris is a flat x,y triangle list — 6 floats per triangle, the same
// layout DrawCanvasTriBatch.Triangles uses.
//
// This is the escape hatch the shape-specific gradient fills are built
// on; reach for it when the geometry is not one of them.
//
// A nil gradient, one with no stops, or a malformed triangle list is a
// no-op, matching how the flat fills treat a degenerate rectangle.
func (dc *DrawContext) FillTrianglesGradient(tris []float32,
	g *CanvasGradient) {
	if g == nil || len(g.Stops) == 0 ||
		len(tris) == 0 || len(tris)%6 != 0 {
		return
	}
	// Bound hostile geometry; gradmesh caps output but tRange still
	// scans the input.
	const maxGradientTrisFloats = 1 << 20
	if len(tris) > maxGradientTrisFloats {
		return
	}
	if dc.recorder != nil {
		if gr, ok := dc.gradientRecorder(); ok {
			gr.FillTrianglesGradient(tris, g)
			return
		}
		// The recorder cannot express a gradient. Record the geometry
		// flat at the ramp's midpoint rather than dropping it: an SVG
		// or PDF export of a glow should be a disc, not a hole.
		dc.recordFlatTriangles(tris, g)
		return
	}

	// Normalize once: clamp and sort the stops, and resolve any
	// geometry the caller left degenerate against the fill's bounds.
	dc.gradStopBuf = dc.gradStopBuf[:0]
	stops := NormalizeGradientStops(g.Stops, &dc.gradStopBuf)
	if len(stops) == 0 {
		return
	}
	minX, minY, maxX, maxY := triBounds(tris)
	res := resolveCanvasGradient(*g, minX, minY, maxX, maxY)
	res.Stops = stops

	// The parameters are built once and read by both passes: the split
	// below and the per-vertex coloring after it project through the
	// same gradient.
	p := gradParams(&res, &dc.gradOffsetBuf)
	out := gradmesh.Subdivide(tris, &p,
		&dc.gradSplitBuf, &dc.gradRadialBuf, &dc.gradIsolineBuf)
	if len(out) == 0 || len(out)%6 != 0 {
		return
	}

	numVerts := len(out) / 2
	b := dc.gradientBatch(SampleGradientStopColor(stops, 0.5), numVerts)
	b.Triangles = append(b.Triangles, out...)
	// Color a triangle at a time, not a vertex at a time. Repeat's ramp
	// steps at every integer of the raw parameter and the split pass puts
	// cut vertices exactly there, so a vertex read on its own can take
	// the far side of the step; SpreadTri reads all three in the period
	// the triangle sits in. See issue #417.
	ramp := prepareGradRamp(stops, &dc.gradRampBuf)
	for i := 0; i+5 < len(out); i += 6 {
		ta, tb, tc := gradmesh.SpreadTri(
			gradmesh.RawT(out[i], out[i+1], &p),
			gradmesh.RawT(out[i+2], out[i+3], &p),
			gradmesh.RawT(out[i+4], out[i+5], &p), p.Spread)
		b.VertexColors = append(b.VertexColors,
			sampleGradRamp(stops, ramp, ta),
			sampleGradRamp(stops, ramp, tb),
			sampleGradRamp(stops, ramp, tc))
	}
}

// gradientBatch starts a fresh vertex-colored batch. A gradient fill
// never merges with the batch before it — not with a flat one, whose
// triangles carry no colors, and not with another gradient, whose
// colors are already assigned — so the batch's two lengths always
// agree and validSvgCmd can enforce the relation downstream.
func (dc *DrawContext) gradientBatch(mid Color,
	numVerts int) *DrawCanvasTriBatch {
	b := dc.takeBatch(mid, true, numVerts)
	dc.lastColor = mid
	dc.batchIsGradient = true
	return b
}

// recordFlatTriangles forwards geometry to a recorder that cannot
// express gradients, one triangle at a time, shaded with the ramp's
// midpoint.
func (dc *DrawContext) recordFlatTriangles(tris []float32,
	g *CanvasGradient) {
	mid, ok := dc.midGradientColor(g)
	if !ok {
		return
	}
	dc.recordTriangles(tris, nil, mid)
}

// recordTriangles hands geometry to a recorder that can only take flat
// primitives, one triangle at a time.
//
// cols, when non-nil, holds one color per vertex and each triangle is
// recorded at the mean of its own three, which is the closest a flat
// primitive gets to interpolated shading. Otherwise every triangle
// takes flat.
func (dc *DrawContext) recordTriangles(tris []float32, cols []Color,
	flat Color) {
	var tri [6]float32
	for i := 0; i+5 < len(tris); i += 6 {
		copy(tri[:], tris[i:i+6])
		col := flat
		if cols != nil {
			v := i / 2
			col = meanColor(cols[v : v+3])
		}
		dc.rec().FilledPolygon(tri[:], col)
	}
}

// midGradientColor samples the gradient's midpoint, for the flat
// fallback the shape-specific fills hand to a plain recorder.
func (dc *DrawContext) midGradientColor(g *CanvasGradient) (Color, bool) {
	if g == nil || len(g.Stops) == 0 {
		return Color{}, false
	}
	dc.gradStopBuf = dc.gradStopBuf[:0]
	stops := NormalizeGradientStops(g.Stops, &dc.gradStopBuf)
	if len(stops) == 0 {
		return Color{}, false
	}
	return SampleGradientStopColor(stops, 0.5), true
}

// gradientRecorderFallback reports whether a shape-specific gradient
// fill should record itself as its flat twin instead of tessellating.
// Returns the midpoint color to record with.
func (dc *DrawContext) gradientRecorderFallback(
	g *CanvasGradient) (Color, bool) {
	if dc.recorder == nil {
		return Color{}, false
	}
	if _, ok := dc.gradientRecorder(); ok {
		// The recorder handles gradients; let the shape tessellate and
		// hand the geometry over in FillTrianglesGradient.
		return Color{}, false
	}
	return dc.midGradientColor(g)
}

// gradScratch resets and returns the shared geometry scratch buffer.
func (dc *DrawContext) gradScratch() *[]float32 {
	dc.gradTriBuf = dc.gradTriBuf[:0]
	return &dc.gradTriBuf
}

// FilledRectGradient fills a rectangle with a gradient. A gradient
// whose endpoints coincide runs top-to-bottom across the rect.
func (dc *DrawContext) FilledRectGradient(x, y, w, h float32,
	g *CanvasGradient) {
	if w <= 0 || h <= 0 {
		return
	}
	if mid, ok := dc.gradientRecorderFallback(g); ok {
		dc.rec().FilledRect(x, y, w, h, mid)
		return
	}
	dst := dc.gradScratch()
	*dst = append(*dst,
		x, y, x+w, y, x+w, y+h,
		x, y, x+w, y+h, x, y+h,
	)
	dc.FillTrianglesGradient(*dst, g)
}

// FilledCircleGradient fills a circle with a gradient. A radial
// gradient with R <= 0 centers on the circle and matches its radius,
// which makes the common glow a one-liner:
//
//	dc.FilledCircleGradient(cx, cy, r, &gui.CanvasGradient{
//		Radial: true,
//		Stops: []gui.GradientStop{
//			{Color: core, Pos: 0},
//			{Color: core.WithOpacity(0), Pos: 1},
//		},
//	})
func (dc *DrawContext) FilledCircleGradient(cx, cy, radius float32,
	g *CanvasGradient) {
	if dc.emitRadialGradient(cx, cy, radius, g) {
		return
	}
	if dc.fillConcentricRings(cx, cy, radius, g) {
		return
	}
	dc.FilledArcGradient(cx, cy, radius, radius, 0, 2*math.Pi, g)
}

// concentricRadial reports whether this fill is the one shape the two
// closed-form paths below can take: a pad-spread radial ramp centered
// on the circle it fills, with no recorder attached to claim the raw
// geometry instead.
//
// Two ways to be concentric, tested against what the caller wrote
// rather than against a resolved copy. Resolving first and comparing
// the result would mean comparing cx to ((cx-r)+(cx+r))*0.5, which
// float32 does not always round back to cx — a fill would fall off the
// fast path for no reason a caller could see.
func (dc *DrawContext) concentricRadial(cx, cy, r float32,
	g *CanvasGradient) bool {
	if dc.recorder != nil || g == nil || len(g.Stops) == 0 {
		return false
	}
	// A non-uniform canvas transform turns the circle into an
	// ellipse, and the shader quad below cannot express one: its ramp
	// is always centered with radius max(W,H)/2. Decline, and the
	// caller falls back to the ring mesh, which lands in a normal
	// batch and so is transformed exactly, ellipse included.
	if xf, ok := dc.activeXform(); ok && !xf.uniform() {
		return false
	}
	if !g.Radial || g.Spread != SpreadPad || !(r > 0) {
		return false
	}
	if !(g.R > 0) {
		// Left to default. resolveCanvasGradient centers a degenerate
		// radial on the fill's bounds with R at half its larger
		// extent, which for a full circle's fan is this circle.
		return true
	}
	if g.R != r || g.CX != cx || g.CY != cy {
		return false
	}
	// FX/FY at the origin means "unset", and the resolve puts them on
	// the center; anywhere else is a real focal offset.
	return (g.FX == 0 && g.FY == 0) || (g.FX == cx && g.FY == cy)
}

// maxLoweredStopErr is how far a resampled ramp may paint from the
// full one, in premultiplied 0..255 channels, and still take the
// shader path.
//
// One 8-bit step. At that size the two are the same picture — the
// gradient shader's own dither moves a channel by half a step anyway —
// and the ring mesh it replaces costs hundreds of triangles per fill.
// Anything coarser is a visible band, and no amount of speed buys that
// back.
const maxLoweredStopErr = 1.0

// emitRadialGradient records a concentric radial fill as one shader
// quad instead of a triangle mesh, and reports whether it took it.
//
// The fragment shader already knows length(p - center) in closed form,
// so a ring mesh's hundreds of triangles buy nothing: they exist only
// to carry a distance the shader recomputes per pixel anyway, and
// every one of their vertices is re-validated and mean-colored on the
// way out of the frame. A large glow was a measurable share of the
// frame purely in that bookkeeping.
//
// The quad is the circle's bounding square, which is what makes the
// substitution exact. GradientDef carries no geometry of its own — no
// center, no radius, no focal point, no spread — because the shader's
// radial ramp is always centered on the quad with radius max(W,H)/2.
// Around a circle of radius r that is r, and the command's corner
// radius of W/2 rounds the quad down to the same circle. Every shape
// the ring mesh already declines is also a shape this cannot express,
// so concentricRadial gates both.
//
// The one thing the shader can lose is stop count: a ramp longer than
// its uniform slots is resampled. That is measured rather than
// assumed. maxPremulStopErr compares the resampled ramp to the full
// one exactly, and anything the eye could pick out goes back to the
// mesh — so a ten-stop glow, whose resampled form differs by under an
// 8-bit step, is lowered, while a ramp the slots genuinely cannot
// carry keeps its mesh.
func (dc *DrawContext) emitRadialGradient(cx, cy, r float32,
	g *CanvasGradient) bool {
	if !dc.concentricRadial(cx, cy, r, g) {
		return false
	}
	dc.gradStopBuf = dc.gradStopBuf[:0]
	stops := NormalizeGradientStopsInto(g.Stops,
		&dc.gradStopBuf, &dc.gradSampleBuf)
	if len(stops) == 0 {
		return false
	}
	if len(dc.gradSampleBuf) > 0 &&
		maxPremulStopErr(dc.gradStopBuf, stops) > maxLoweredStopErr {
		// The ramp was resampled to fit the uniforms and the result
		// would not paint the same. Hand it back to the mesh, which
		// reproduces every stop.
		return false
	}
	e := dc.takeGradient()
	// Copied rather than aliased: gradStopBuf is scratch the next fill
	// overwrites, while this list has to outlive the whole redraw.
	e.Def.Stops = append(e.Def.Stops[:0], stops...)
	e.Def.Type = GradientRadial
	// Baked: RenderGradient has no xform fields to ride on. The scale
	// is uniform here (concentricRadial declined otherwise), so the
	// circle stays a circle and |sx| is the whole story.
	// xfRect normalizes, so a negative (mirroring) scale still yields
	// the bounding square's top-left corner rather than its far one.
	e.X, e.Y, e.W, e.H = dc.xfRect(cx-r, cy-r, 2*r, 2*r)
	e.afterBatch = len(dc.batches)
	return true
}

// concentricMinSegs is the angular resolution the ring mesh needs
// before its own interpolation error is invisible.
//
// A ring band's vertices sit on its two isolines, but Gouraud shading
// interpolates across the chord between them, so a point near the
// middle of a segment reads the color of a radius up to the sagitta
// r*(1-cos(pi/n)) away. Below 1/256 of the ramp — the same tolerance
// the general path's radial pass refines to — that is under one step
// of an 8-bit channel, and 1-cos(pi/36) is 0.0038.
//
// A full circle's segment count starts at 64 and climbs with radius,
// so this never rejects one in practice; it is the invariant stated
// rather than inherited from arcPoints' formula.
const concentricMinSegs = 36

// gradRing is one boundary of the concentric ring mesh: the radius the
// ramp reaches a given color at.
type gradRing struct {
	radius float32
	color  Color
}

// fillConcentricRings is the closed-form fill for the most common
// radial gradient there is: a ramp centered on the circle it fills.
// Reports whether it handled the fill.
//
// The general path has to *find* the ramp's isolines. It refines the
// fan until no triangle's radial error is visible, then cuts every
// triangle at every stop, then projects each resulting vertex back
// through a square root to recover the parameter it was cut at. For a
// concentric ramp all of that is answering a question with a known
// answer: the isolines are circles about the center, at radius
// stop.Pos * r.
//
// So the mesh is emitted as one quad strip per pair of adjacent stops,
// and the vertex colors are the stops themselves — no subdivision
// search, no per-vertex projection, and no ramp sampling. It is also
// exact where the general path approximates: interpolating a distance
// field linearly across a split triangle is only as good as the split
// was fine, while a ring boundary lies exactly on its isoline.
//
// Bails out to the general path for anything concentricRadial
// declines: an ellipse, an off-center or offset-focal ramp, a non-pad
// spread (whose isolines repeat past the rim), or an active recorder,
// whose gradient contract is the raw geometry plus the gradient.
//
// Still the path for a concentric fill whose ramp has more stops than
// the GPU shader's uniforms hold: emitRadialGradient takes the same
// shapes first but hands those back, because it would have to resample
// them and this mesh does not.
func (dc *DrawContext) fillConcentricRings(cx, cy, r float32,
	g *CanvasGradient) bool {
	if !dc.concentricRadial(cx, cy, r, g) {
		return false
	}

	dc.gradStopBuf = dc.gradStopBuf[:0]
	stops := NormalizeGradientStops(g.Stops, &dc.gradStopBuf)
	if len(stops) == 0 {
		return false
	}
	pts := dc.arcPoints(cx, cy, r, r, 0, 2*math.Pi)
	if len(pts) < 4 {
		return false
	}

	rings := dc.concentricRings(stops, r)
	if len(rings) < 2 {
		return false
	}

	segs := len(pts)/2 - 1
	if segs < concentricMinSegs {
		return false
	}

	// Two triangles per segment per band, and three for the innermost
	// band when it closes on the center. An over-estimate only sizes
	// the pooled buffers, so the center case is not special-cased here.
	b := dc.gradientBatch(SampleGradientStopColor(stops, 0.5),
		segs*6*(len(rings)-1))

	invR := 1 / r
	for j := 0; j+1 < len(rings); j++ {
		r0, c0 := rings[j].radius, rings[j].color
		r1, c1 := rings[j+1].radius, rings[j+1].color
		if !(r1 > r0) {
			// A hard stop — two stops at one position. The band has no
			// area, so it contributes no geometry; the color still
			// changes, because the next band opens on c1.
			continue
		}
		if r0 <= 0 {
			// Innermost band closes on the center: a fan, not a strip.
			// Its rim is still this band's outer isoline, not the
			// circle's — the two coincide only when the first stop is
			// also the last.
			s := r1 * invR
			for i := 0; i+3 < len(pts); i += 2 {
				b.Triangles = append(b.Triangles, cx, cy,
					cx+(pts[i]-cx)*s, cy+(pts[i+1]-cy)*s,
					cx+(pts[i+2]-cx)*s, cy+(pts[i+3]-cy)*s)
				b.VertexColors = append(b.VertexColors, c0, c1, c1)
			}
			continue
		}
		// The rim points are already on the circle of radius r, so an
		// inner ring is the same direction scaled — no trigonometry
		// beyond the one arcPoints pass.
		s0, s1 := r0*invR, r1*invR
		for i := 0; i+3 < len(pts); i += 2 {
			ux0, uy0 := pts[i]-cx, pts[i+1]-cy
			ux1, uy1 := pts[i+2]-cx, pts[i+3]-cy
			ax, ay := cx+ux0*s0, cy+uy0*s0
			bx, by := cx+ux1*s0, cy+uy1*s0
			ex, ey := cx+ux0*s1, cy+uy0*s1
			fx, fy := cx+ux1*s1, cy+uy1*s1
			// inner_i -> outer_i -> outer_i+1 -> inner_i+1 is the
			// quad's order in the same sense the fan winds; walking
			// the inner edge first reverses it.
			b.Triangles = append(b.Triangles,
				ax, ay, ex, ey, fx, fy,
				ax, ay, fx, fy, bx, by)
			b.VertexColors = append(b.VertexColors,
				c0, c1, c1, c0, c1, c0)
		}
	}
	return true
}

// concentricRings turns normalized stops into the ring boundaries of
// the mesh, in increasing radius.
//
// Pad is what the two extra rings are for: the ramp is flat inside the
// first stop and outside the last, so each of those regions gets a
// boundary at the end color rather than a ramp running off the stops.
func (dc *DrawContext) concentricRings(stops []GradientStop,
	r float32) []gradRing {
	dc.gradRingBuf = dc.gradRingBuf[:0]
	first, last := stops[0], stops[len(stops)-1]
	if first.Pos > 0 {
		dc.gradRingBuf = append(dc.gradRingBuf,
			gradRing{radius: 0, color: first.Color})
	}
	for i := range stops {
		dc.gradRingBuf = append(dc.gradRingBuf,
			gradRing{radius: stops[i].Pos * r, color: stops[i].Color})
	}
	if last.Pos < 1 {
		dc.gradRingBuf = append(dc.gradRingBuf,
			gradRing{radius: r, color: last.Color})
	}
	return dc.gradRingBuf
}

// FilledArcGradient fills an elliptical arc (a pie slice) with a
// gradient.
func (dc *DrawContext) FilledArcGradient(cx, cy, rx, ry, start,
	sweep float32, g *CanvasGradient) {
	if mid, ok := dc.gradientRecorderFallback(g); ok {
		dc.rec().FilledArc(cx, cy, rx, ry, start, sweep, mid)
		return
	}
	pts := dc.arcPoints(cx, cy, rx, ry, start, sweep)
	if len(pts) < 4 {
		return
	}
	dst := dc.gradScratch()
	appendArcFanTris(dst, cx, cy, pts)
	dc.FillTrianglesGradient(*dst, g)
}

// FilledPolygonGradient fills a convex polygon with a gradient.
// points is a flat x,y list.
func (dc *DrawContext) FilledPolygonGradient(points []float32,
	g *CanvasGradient) {
	if len(points) < 6 {
		return
	}
	if mid, ok := dc.gradientRecorderFallback(g); ok {
		dc.rec().FilledPolygon(points, mid)
		return
	}
	dst := dc.gradScratch()
	appendPolygonFanTris(dst, points)
	dc.FillTrianglesGradient(*dst, g)
}

// FilledRoundedRectGradient fills a rounded rectangle with a gradient.
// Radius is clamped to half the smaller dimension.
func (dc *DrawContext) FilledRoundedRectGradient(x, y, w, h,
	radius float32, g *CanvasGradient) {
	if w <= 0 || h <= 0 {
		return
	}
	if mid, ok := dc.gradientRecorderFallback(g); ok {
		dc.rec().FilledRoundedRect(x, y, w, h, radius, mid)
		return
	}
	radius = min(radius, w/2, h/2)
	if radius <= 0 {
		dc.FilledRectGradient(x, y, w, h, g)
		return
	}
	dst := dc.gradScratch()
	appendRoundedRectTris(dst, x, y, w, h, radius)
	dc.FillTrianglesGradient(*dst, g)
}
