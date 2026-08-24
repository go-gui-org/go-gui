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
	if dc.recorder != nil {
		if gr, ok := dc.recorder.(DrawGradientRecorder); ok {
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
	for i := range numVerts {
		t := gradmesh.ApplySpread(
			gradmesh.RawT(out[i*2], out[i*2+1], &p), p.Spread)
		b.VertexColors = append(b.VertexColors,
			SampleGradientStopColor(stops, t))
	}
}

// gradientBatch starts a fresh vertex-colored batch. A gradient fill
// never merges with the batch before it — not with a flat one, whose
// triangles carry no colors, and not with another gradient, whose
// colors are already assigned — so the batch's two lengths always
// agree and validSvgCmd can enforce the relation downstream.
func (dc *DrawContext) gradientBatch(mid Color,
	numVerts int) *DrawCanvasTriBatch {
	dc.batches = append(dc.batches, DrawCanvasTriBatch{
		Color:        mid,
		Triangles:    make([]float32, 0, numVerts*2),
		VertexColors: make([]Color, 0, numVerts),
	})
	dc.lastColor = mid
	dc.batchIsGradient = true
	dc.currentBatchIdx = len(dc.batches) - 1
	return &dc.batches[dc.currentBatchIdx]
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
		dc.recorder.FilledPolygon(tri[:], col)
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
	if _, ok := dc.recorder.(DrawGradientRecorder); ok {
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
		dc.recorder.FilledRect(x, y, w, h, mid)
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
	dc.FilledArcGradient(cx, cy, radius, radius, 0, 2*math.Pi, g)
}

// FilledArcGradient fills an elliptical arc (a pie slice) with a
// gradient.
func (dc *DrawContext) FilledArcGradient(cx, cy, rx, ry, start,
	sweep float32, g *CanvasGradient) {
	if mid, ok := dc.gradientRecorderFallback(g); ok {
		dc.recorder.FilledArc(cx, cy, rx, ry, start, sweep, mid)
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
		dc.recorder.FilledPolygon(points, mid)
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
		dc.recorder.FilledRoundedRect(x, y, w, h, radius, mid)
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
