package gui

import "math"

// appendQuad appends two triangles forming a quad.
func appendQuad(b *DrawCanvasTriBatch,
	x0, y0, x1, y1, x2, y2, x3, y3 float32) {
	b.Triangles = append(b.Triangles,
		x0, y0, x1, y1, x2, y2,
		x0, y0, x2, y2, x3, y3,
	)
}

// appendCornerFan appends a 90-degree filled arc fan.
func appendCornerFan(b *DrawCanvasTriBatch,
	cx, cy, r, startAngle float32, segs int) {
	step := float32(math.Pi/2) / float32(segs)
	for i := range segs {
		a0 := float64(startAngle + step*float32(i))
		a1 := float64(startAngle + step*float32(i+1))
		b.Triangles = append(b.Triangles,
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
