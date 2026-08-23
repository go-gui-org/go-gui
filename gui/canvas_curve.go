package gui

import "math"

// appendQuad appends a quad as two triangles. It writes into a plain
// slice rather than a batch so gradient fills, which tessellate into a
// scratch buffer before they know their vertex colors, share the same
// geometry as flat fills.
func appendQuad(dst *[]float32,
	x0, y0, x1, y1, x2, y2, x3, y3 float32) {
	*dst = append(*dst,
		x0, y0, x1, y1, x2, y2,
		x0, y0, x2, y2, x3, y3,
	)
}

// appendCornerFan appends a 90-degree filled arc fan.
func appendCornerFan(dst *[]float32,
	cx, cy, r, startAngle float32, segs int) {
	step := float32(math.Pi/2) / float32(segs)
	for i := range segs {
		a0 := float64(startAngle + step*float32(i))
		a1 := float64(startAngle + step*float32(i+1))
		*dst = append(*dst,
			cx, cy,
			cx+r*float32(math.Cos(a0)), cy+r*float32(math.Sin(a0)),
			cx+r*float32(math.Cos(a1)), cy+r*float32(math.Sin(a1)),
		)
	}
}

// appendArcPoints appends points for a 90-degree arc.
func appendArcPoints(pts []float32,
	cx, cy, r, startAngle float32, segs int) []float32 {
	step := float32(math.Pi/2) / float32(segs)
	for i := range segs + 1 {
		a := float64(startAngle + step*float32(i))
		pts = append(pts,
			cx+r*float32(math.Cos(a)),
			cy+r*float32(math.Sin(a)))
	}
	return pts
}

const (
	bezierTol      = float32(0.5)    // pixel tolerance
	bezierMaxDepth = 16              // max subdivision depth
	bezierDegenTol = float32(0.0001) // near-degenerate threshold
)

func flattenQuadBezier(
	buf *[]float32,
	x0, y0, cx, cy, x1, y1, tol float32, depth int,
) {
	mx := (x0 + x1) / 2
	my := (y0 + y1) / 2
	dx := cx - mx
	dy := cy - my
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if d != d || d <= tol || depth >= bezierMaxDepth {
		*buf = append(*buf, x1, y1)
		return
	}
	ax := (x0 + cx) / 2
	ay := (y0 + cy) / 2
	bx := (cx + x1) / 2
	by := (cy + y1) / 2
	abx := (ax + bx) / 2
	aby := (ay + by) / 2
	flattenQuadBezier(buf, x0, y0, ax, ay, abx, aby, tol, depth+1)
	flattenQuadBezier(buf, abx, aby, bx, by, x1, y1, tol, depth+1)
}

func flattenCubicBezier(
	buf *[]float32,
	x0, y0, c1x, c1y, c2x, c2y, x1, y1, tol float32, depth int,
) {
	dx := x1 - x0
	dy := y1 - y0
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if d != d || d < bezierDegenTol {
		*buf = append(*buf, x1, y1)
		return
	}

	d1 := f32Abs((c1x-x0)*dy-(c1y-y0)*dx) / d
	d2 := f32Abs((c2x-x0)*dy-(c2y-y0)*dx) / d

	if d1+d2 <= tol || depth >= bezierMaxDepth {
		*buf = append(*buf, x1, y1)
		return
	}
	ax := (x0 + c1x) / 2
	ay := (y0 + c1y) / 2
	bx := (c1x + c2x) / 2
	by := (c1y + c2y) / 2
	ex := (c2x + x1) / 2
	ey := (c2y + y1) / 2
	abx := (ax + bx) / 2
	aby := (ay + by) / 2
	bex := (bx + ex) / 2
	bey := (by + ey) / 2
	midx := (abx + bex) / 2
	midy := (aby + bey) / 2
	flattenCubicBezier(buf, x0, y0, ax, ay, abx, aby, midx, midy,
		tol, depth+1)
	flattenCubicBezier(buf, midx, midy, bex, bey, ex, ey, x1, y1,
		tol, depth+1)
}

// appendRoundedRectTris appends the fill geometry for a rounded rect:
// a center cross of three strips plus a fan per corner. radius must
// already be clamped to at most half the smaller dimension and be > 0.
//
// Shared by FilledRoundedRect and FilledRoundedRectGradient so the two
// tessellate identically — a gradient fill differs only in how its
// vertices are colored, never in where they are.
func appendRoundedRectTris(dst *[]float32, x, y, w, h, r float32) {
	// Center cross (vertical strip).
	appendQuad(dst, x+r, y, x+w-r, y, x+w-r, y+h, x+r, y+h)
	// Left strip.
	appendQuad(dst, x, y+r, x+r, y+r, x+r, y+h-r, x, y+h-r)
	// Right strip.
	appendQuad(dst, x+w-r, y+r, x+w, y+r, x+w, y+h-r, x+w-r, y+h-r)

	const segs = 8
	appendCornerFan(dst, x+r, y+r, r, math.Pi, segs)       // TL
	appendCornerFan(dst, x+w-r, y+r, r, 3*math.Pi/2, segs) // TR
	appendCornerFan(dst, x+w-r, y+h-r, r, 0, segs)         // BR
	appendCornerFan(dst, x+r, y+h-r, r, math.Pi/2, segs)   // BL
}

// appendArcFanTris appends a triangle fan from (cx, cy) to consecutive
// pairs in an arc polyline. Shared by FilledArc and FilledArcGradient.
func appendArcFanTris(dst *[]float32, cx, cy float32, pts []float32) {
	for i := 0; i+3 < len(pts); i += 2 {
		*dst = append(*dst, cx, cy, pts[i], pts[i+1], pts[i+2], pts[i+3])
	}
}

// appendPolygonFanTris appends a convex polygon as a fan from its
// first vertex. Shared by FilledPolygon and FilledPolygonGradient.
func appendPolygonFanTris(dst *[]float32, points []float32) {
	n := len(points) / 2
	x0, y0 := points[0], points[1]
	for i := 1; i < n-1; i++ {
		*dst = append(*dst, x0, y0,
			points[i*2], points[i*2+1],
			points[(i+1)*2], points[(i+1)*2+1])
	}
}
