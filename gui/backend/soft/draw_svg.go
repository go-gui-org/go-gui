package soft

import (
	"image"
	"math"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
)

// drawSvg rasterizes a tessellated SVG path: a flat triangle list in
// Triangles, optionally one color per vertex in VertexColors.
//
// The vertex transform order mirrors gui/backend/gl's drawSvg —
// animateTransform scale/translate, then rotation about (RotCX, RotCY),
// then the command's own Scale and origin — so an animated SVG lands in
// the same place headlessly as it does on a GPU.
//
// Triangles are drawn as one path in a single color rather than one
// triangle at a time. The rasterizer accumulates signed coverage across
// a whole path, so the shared interior edges cancel exactly; rasterizing
// each triangle separately would blend two half-covered edges at every
// seam and draw a lattice of pale lines across every filled shape.
func (r *renderer) drawSvg(cmd *gui.RenderCmd) {
	if cmd.IsClipMask {
		// Clip-mask geometry is a stencil source, not paint. No
		// backend consumes it yet; the render path still emits it.
		return
	}
	if len(cmd.Triangles) == 0 || len(cmd.Triangles)%6 != 0 {
		return
	}
	numVerts := len(cmd.Triangles) / 2
	hasVCols := len(cmd.VertexColors) == numVerts

	verts := r.svgVertices(cmd, numVerts)
	numTris := numVerts / 3

	if !hasVCols {
		r.fillTriangleRun(verts, cmd.Color)
		return
	}

	// Vertex-colored geometry is a mesh: its triangles tile a region
	// and share edges. Rasterized one at a time each shared edge is
	// antialiased twice — two ~50% coverages that never sum to one —
	// and the background shows through as a lattice of pale lines
	// over every gradient. So the mesh takes its coverage from a
	// single path, where the signed accumulation cancels the interior
	// edges exactly, and the shading is filled inside that mask.
	// The negated > rejects a NaN scale as well as a negative one:
	// max(0, min(NaN, 1)) is NaN, and uint8(NaN) is undefined.
	vAlpha := float32(1)
	if cmd.HasVertexAlpha {
		vAlpha = 0
		if cmd.VertexAlphaScale > 0 {
			vAlpha = min(cmd.VertexAlphaScale, 1)
		}
	}
	r.fillTriangleMesh(verts, cmd.VertexColors, numTris, vAlpha)
}

// fillTriangleMesh draws a vertex-colored triangle mesh with no interior
// seams: one antialiased coverage mask for the whole mesh, and a layer
// holding the shading, composited through it.
//
// The shading is written, not blended, so a pixel claimed by two
// triangles along a shared edge is painted once — their colors agree
// there, and blending twice would darken every seam of a translucent
// gradient.
func (r *renderer) fillTriangleMesh(verts []float32, cols []gui.Color,
	numTris int, vAlpha float32) {

	minX, minY, maxX, maxY := vertsBounds(verts)
	region := r.buf.region(minX, minY, maxX-minX, maxY-minY)
	if region.Empty() {
		return
	}
	mask := r.coveragePath(&r.maskPix, region,
		func(z *vector.Rasterizer, ox, oy float32) {
			for i := 0; i < numTris*6; i += 6 {
				emitTriangle(z, ox, oy, verts[i:i+6])
			}
		})
	if mask == nil {
		return
	}

	// score records, per pixel, how well the triangle that shaded it
	// actually contained it: the minimum of its barycentric weights,
	// which is >= 0 inside the triangle and negative out in its skirt.
	// A triangle may only overwrite a pixel it fits better than the one
	// already there.
	//
	// Without this the last triangle painted simply wins, skirt or not,
	// and a fan of narrow wedges — a glow struck from its own center,
	// where the wedges are sub-pixel wide near the hub — has its whole
	// inner region painted from extrapolated weights. The mesh's outer
	// edge still gets its skirt, because no interior sample competes
	// for those pixels.
	score := r.meshScore(region)
	layer := r.acquireLayer()
	for t := range numTris {
		shadeTriangle(layer, region, score, verts[t*6:t*6+6],
			scaleAlpha(cols[t*3], vAlpha),
			scaleAlpha(cols[t*3+1], vAlpha),
			scaleAlpha(cols[t*3+2], vAlpha))
	}
	layer.markDirty(region)
	composite(r.buf, layer, region, mask)
	r.releaseLayer(layer)
}

// shadeTriangle writes one triangle's barycentric interpolation into dst,
// covering every pixel centre inside it plus a half-pixel skirt.
//
// The skirt is what keeps the mesh's outer edge from thinning: a pixel
// the coverage mask includes at 40% can have its centre just outside the
// triangle, and with no color written there the antialiased edge would
// fade to nothing. Interior pixels are unaffected — the triangles tile,
// so every centre inside the mesh is already claimed.
func shadeTriangle(dst *buffer, region image.Rectangle, score []float32,
	p []float32, c0, c1, c2 gui.Color) {

	src := newTriSrc(p, c0, c1, c2)
	if src == nil {
		return
	}
	minX, minY, maxX, maxY := vertsBounds(p)
	rect := deviceRect(minX-1, minY-1, maxX-minX+2, maxY-minY+2).
		Intersect(region)
	if rect.Empty() {
		return
	}
	t0, t1, t2 := src.skirt()

	stride := region.Dx()
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		i := dst.img.PixOffset(rect.Min.X, y)
		si := (y-region.Min.Y)*stride + (rect.Min.X - region.Min.X)
		for x := rect.Min.X; x < rect.Max.X; x++ {
			w0, w1, w2 := src.weights(x, y)
			if w0 < -t0 || w1 < -t1 || w2 < -t2 {
				i += 4
				si++
				continue
			}
			// An interior sample (min weight >= 0) always beats a skirt
			// sample, and among interior samples the containing
			// triangle wins. Equal scores keep the earlier write: that
			// happens on a shared edge, where both colors agree.
			if fit := min(w0, w1, w2); fit > score[si] {
				score[si] = fit
			} else {
				i += 4
				si++
				continue
			}
			cr, cg, cb, ca := premul(src.mix(w0, w1, w2))
			dst.img.Pix[i] = cr
			dst.img.Pix[i+1] = cg
			dst.img.Pix[i+2] = cb
			dst.img.Pix[i+3] = ca
			i += 4
			si++
		}
	}
}

// meshScore returns a per-pixel scratch buffer for region, reset to a
// value below any weight a skirt sample can produce so the first write
// to a pixel always takes.
func (r *renderer) meshScore(region image.Rectangle) []float32 {
	n := region.Dx() * region.Dy()
	if cap(r.meshScoreBuf) < n {
		r.meshScoreBuf = make([]float32, n)
	}
	s := r.meshScoreBuf[:n]
	for i := range s {
		s[i] = -math.MaxFloat32
	}
	return s
}

// svgVertices transforms every vertex into device space, reusing the
// scratch slice so an animated SVG does not allocate per frame.
func (r *renderer) svgVertices(cmd *gui.RenderCmd, numVerts int) []float32 {
	if cap(r.svgBatch) < numVerts*2 {
		r.svgBatch = make([]float32, numVerts*2)
	}
	verts := r.svgBatch[:numVerts*2]

	var sx, sy, tx, ty float32
	if cmd.HasXform {
		sx, sy, tx, ty = cmd.ScaleX, cmd.ScaleY, cmd.TransX, cmd.TransY
	}
	hasRot := cmd.RotAngle != 0
	var sinA, cosA float32
	if hasRot {
		rad := float64(cmd.RotAngle) * math.Pi / 180
		sinA = float32(math.Sin(rad))
		cosA = float32(math.Cos(rad))
	}

	s := r.scale
	for i := range numVerts {
		vx := cmd.Triangles[i*2]
		vy := cmd.Triangles[i*2+1]
		if cmd.HasXform {
			vx = vx*sx + tx
			vy = vy*sy + ty
		}
		if hasRot {
			dx := vx - cmd.RotCX
			dy := vy - cmd.RotCY
			vx = cmd.RotCX + dx*cosA - dy*sinA
			vy = cmd.RotCY + dx*sinA + dy*cosA
		}
		verts[i*2] = (cmd.X + vx*cmd.Scale) * s
		verts[i*2+1] = (cmd.Y + vy*cmd.Scale) * s
	}
	return verts
}

// fillTriangleRun fills every triangle as one path in color c.
func (r *renderer) fillTriangleRun(verts []float32, c gui.Color) {
	if len(verts) == 0 || c.A == 0 {
		return
	}
	minX, minY, maxX, maxY := vertsBounds(verts)
	region := r.buf.region(minX, minY, maxX-minX, maxY-minY)
	r.buf.fillPath(region, r.buf.solid(c),
		func(z *vector.Rasterizer, ox, oy float32) {
			for i := 0; i < len(verts); i += 6 {
				emitTriangle(z, ox, oy, verts[i:i+6])
			}
		})
}

// vertsBounds is the bounding box of an interleaved x/y vertex slice.
func vertsBounds(verts []float32) (minX, minY, maxX, maxY float32) {
	minX, minY = math.MaxFloat32, math.MaxFloat32
	maxX, maxY = -math.MaxFloat32, -math.MaxFloat32
	for i := 0; i < len(verts); i += 2 {
		minX, maxX = min(minX, verts[i]), max(maxX, verts[i])
		minY, maxY = min(minY, verts[i+1]), max(maxY, verts[i+1])
	}
	return
}

// emitTriangle adds one closed triangle contour to the path.
func emitTriangle(z *vector.Rasterizer, ox, oy float32, p []float32) {
	z.MoveTo(p[0]-ox, p[1]-oy)
	z.LineTo(p[2]-ox, p[3]-oy)
	z.LineTo(p[4]-ox, p[5]-oy)
	z.ClosePath()
}

// scaleAlpha applies the command's vertex alpha multiplier, which
// exists so animating opacity does not copy the vertex colors.
func scaleAlpha(c gui.Color, mul float32) gui.Color {
	if mul >= 1 {
		return c
	}
	c.A = uint8(float32(c.A) * mul)
	return c
}

// triSrc interpolates a triangle's vertex colors barycentrically, as
// the GPU's varying interpolation does.
type triSrc struct {
	ax, ay     float32 // vertex A, the barycentric origin
	abx, aby   float32 // A→B
	acx, acy   float32 // A→C
	c0, c1, c2 gui.Color
	invDen     float32
}

// newTriSrc returns nil for a degenerate (zero-area) triangle, which
// covers no pixels anyway.
func newTriSrc(p []float32, c0, c1, c2 gui.Color) *triSrc {
	abx, aby := p[2]-p[0], p[3]-p[1]
	acx, acy := p[4]-p[0], p[5]-p[1]
	den := abx*acy - acx*aby
	if den == 0 {
		return nil
	}
	return &triSrc{
		ax: p[0], ay: p[1],
		abx: abx, aby: aby,
		acx: acx, acy: acy,
		c0: c0, c1: c1, c2: c2,
		invDen: 1 / den,
	}
}

// weights returns the barycentric coordinates of the centre of pixel
// (x, y): w1 belongs to B, w2 to C, the remainder to A. They are
// negative outside the triangle, which is what the skirt test reads.
func (t *triSrc) weights(x, y int) (w0, w1, w2 float32) {
	px := float32(x) + 0.5 - t.ax
	py := float32(y) + 0.5 - t.ay
	w1 = (px*t.acy - t.acx*py) * t.invDen
	w2 = (t.abx*py - px*t.aby) * t.invDen
	return 1 - w1 - w2, w1, w2
}

// skirt returns how negative each weight may go and still be within half
// a pixel of the triangle.
//
// A weight falls from 1 at its own vertex to 0 on the opposite edge, so
// it changes by 1/h per pixel of distance, where h is the height to that
// edge — and h is 2*area/len, with 2*area being |den|. One weight unit
// is therefore |den|/len pixels, and half a pixel is len/(2*|den|).
func (t *triSrc) skirt() (s0, s1, s2 float32) {
	den := t.invDen
	if den < 0 {
		den = -den
	}
	// bc = AC - AB, the edge opposite A.
	bcx, bcy := t.acx-t.abx, t.acy-t.aby
	half := 0.5 * den
	return hypot32(bcx, bcy) * half,
		hypot32(t.acx, t.acy) * half,
		hypot32(t.abx, t.aby) * half
}

// mix blends the three vertex colors by the given weights, clamped into
// the simplex first: a pixel centre inside the skirt sits just outside
// the triangle, and extrapolating past a vertex color there would
// overshoot the gradient.
func (t *triSrc) mix(w0, w1, w2 float32) gui.Color {
	w0, w1, w2 = clampWeights(w0, w1, w2)
	blend := func(a, b, c uint8) uint8 {
		return uint8(float32(a)*w0 + float32(b)*w1 +
			float32(c)*w2 + 0.5)
	}
	return gui.RGBA(
		blend(t.c0.R, t.c1.R, t.c2.R),
		blend(t.c0.G, t.c1.G, t.c2.G),
		blend(t.c0.B, t.c1.B, t.c2.B),
		blend(t.c0.A, t.c1.A, t.c2.A))
}

// hypot32 is math.Hypot without the float64 round trip; the inputs here
// are pixel distances, far from overflow.
func hypot32(x, y float32) float32 {
	return float32(math.Sqrt(float64(x*x + y*y)))
}

// clampWeights clamps barycentric weights into the simplex and
// renormalizes so they still sum to one.
func clampWeights(w0, w1, w2 float32) (float32, float32, float32) {
	w0, w1, w2 = max(w0, 0), max(w1, 0), max(w2, 0)
	sum := w0 + w1 + w2
	if sum == 0 {
		return 1, 0, 0
	}
	inv := 1 / sum
	return w0 * inv, w1 * inv, w2 * inv
}
