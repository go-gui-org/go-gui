package gui

// canvas_draw_colors.go — the DrawContext per-vertex color fill.
//
// A gradient shades geometry by evaluating a ramp at every vertex, and
// its level sets are conic curves nested by construction. Some shading
// models do not produce that family: a Lambert-shaded sphere's isophotes
// are ellipses whose radius grows as sqrt(1-k^2), non-monotonic in the
// ramp parameter and clipped at the silhouette, so no gradient of any
// kind emits them. FillTrianglesColors is the primitive underneath the
// gradient fills, for a caller that has already evaluated its own
// shading model per vertex.

// FillTrianglesColors fills caller-supplied geometry with caller-supplied
// per-vertex colors. tris is a flat x,y triangle list — 6 floats per
// triangle, the same layout DrawCanvasTriBatch.Triangles uses — and
// colors holds exactly one entry per vertex, so len(colors)*2 ==
// len(tris).
//
// Nothing is evaluated here: no projection, no stop isolines, no
// subdivision. The caller's colors are the shading, interpolated across
// each triangle by the backend.
//
// A malformed triangle list, or a color count that does not match the
// vertex count, is a no-op — matching how FillTrianglesGradient treats a
// degenerate input rather than painting something the caller did not
// ask for.
func (dc *DrawContext) FillTrianglesColors(tris []float32,
	colors []Color) {
	if len(tris) == 0 || len(tris)%6 != 0 ||
		len(colors)*2 != len(tris) {
		return
	}
	if dc.recorder != nil {
		if vr, ok := dc.recorder.(DrawVertexColorRecorder); ok {
			vr.FillTrianglesColors(tris, colors)
			return
		}
		// The recorder cannot express per-vertex color. Record each
		// triangle flat at its own mean rather than dropping the
		// geometry: an SVG or PDF export of a shaded sphere should be
		// a disc, not a hole.
		dc.recordTriangles(tris, colors, Color{})
		return
	}

	// The batch's flat Color is the mesh's mean, playing the part the
	// gradient's midpoint plays: it is what a flat-only consumer sees,
	// and it is never read when VertexColors is honored.
	b := dc.gradientBatch(meanColor(colors), len(colors))
	b.Triangles = append(b.Triangles, tris...)
	b.VertexColors = append(b.VertexColors, colors...)
}

// meanColor averages a color slice channel by channel, alpha included.
// The average is unweighted — a translucent vertex pulls the mean's hue
// as hard as an opaque one. That is only ever seen by a consumer that
// cannot read per-vertex colors at all, where a cheap stand-in beats a
// correct one nobody looks at.
//
// Accumulation is uint64 so a fill large enough to overflow a uint32
// channel sum (a mesh of roughly sixteen million vertices) still gets
// a mean rather than a wrap.
func meanColor(colors []Color) Color {
	if len(colors) == 0 {
		return Color{}
	}
	var r, g, b, a uint64
	for i := range colors {
		r += uint64(colors[i].R)
		g += uint64(colors[i].G)
		b += uint64(colors[i].B)
		a += uint64(colors[i].A)
	}
	n := uint64(len(colors))
	return RGBA(uint8(r/n), uint8(g/n), uint8(b/n), uint8(a/n))
}
