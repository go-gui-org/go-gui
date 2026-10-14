// Package homography holds the 3x3 matrix math the CPU-side backends
// (soft and web) need for a gui.RenderTransformBegin. The GPU backends
// put the matrix into their MVP (gpu.ApplyHomography) and need none of
// this.
//
// A matrix is row-major and maps (x, y, 1) to (x', y', w). The mapped
// point is (x'/w, y'/w). A point with w <= 0 is at or behind the viewer
// and has no place on screen.
package homography

// Mat is a row-major 3x3 homography.
type Mat = [9]float32

// Identity is the matrix that maps every point to itself.
var Identity = Mat{1, 0, 0, 0, 1, 0, 0, 0, 1}

// Scaled returns h for physical pixels: S·h·S⁻¹ with S = diag(s, s, 1).
// The move terms grow by s and the perspective terms shrink by s, so a
// point in physical pixels maps to the same place h maps it in logical
// pixels.
func Scaled(h *Mat, s float32) Mat {
	if s <= 0 {
		s = 1
	}
	return Mat{
		h[0], h[1], h[2] * s,
		h[3], h[4], h[5] * s,
		h[6] / s, h[7] / s, h[8],
	}
}

// Mul returns a·b: the map that applies b first, then a.
func Mul(a, b *Mat) Mat {
	var out Mat
	for r := range 3 {
		for c := range 3 {
			out[r*3+c] = a[r*3]*b[c] + a[r*3+1]*b[3+c] + a[r*3+2]*b[6+c]
		}
	}
	return out
}

// Apply maps (x, y) through h. ok is false when the point lands at or
// behind the viewer (w <= 0).
func Apply(h *Mat, x, y float32) (px, py float32, ok bool) {
	w := h[6]*x + h[7]*y + h[8]
	if !(w > 0) {
		return 0, 0, false
	}
	return (h[0]*x + h[1]*y + h[2]) / w, (h[3]*x + h[4]*y + h[5]) / w, true
}

// Inverse returns the exact inverse of h, so for a point p = h(q) with
// w > 0, the w of Inverse(h)(p) is 1/w and also positive. ok is false
// when h has no inverse.
// gui.xfMat has its own inverse for the pointer mapping: that one works
// in float64 with an epsilon gate, and package gui cannot import a
// backend-internal package, so the two stay separate on purpose.
func Inverse(h *Mat) (Mat, bool) {
	a, b, c := h[0], h[1], h[2]
	d, e, f := h[3], h[4], h[5]
	g, k, i := h[6], h[7], h[8]
	c00 := e*i - f*k
	c01 := f*g - d*i
	c02 := d*k - e*g
	det := a*c00 + b*c01 + c*c02
	if det == 0 || det != det {
		return Mat{}, false
	}
	inv := 1 / det
	return Mat{
		c00 * inv, (c*k - b*i) * inv, (b*f - c*e) * inv,
		c01 * inv, (a*i - c*g) * inv, (c*d - a*f) * inv,
		c02 * inv, (b*g - a*k) * inv, (a*e - b*d) * inv,
	}, true
}

// Bounds returns the bounding box of the rect (x, y, w, h) mapped
// through m. ok is false when a corner lands at or behind the viewer:
// the mapped shape then has no finite box. It is the same four-corner
// shape as gui.mapRectBox, kept separate because package gui cannot
// import a backend-internal package.
func Bounds(m *Mat, x, y, w, h float32) (bx, by, bw, bh float32, ok bool) {
	var minX, minY, maxX, maxY float32
	for i := range 4 {
		cx, cy := x, y
		if i&1 != 0 {
			cx += w
		}
		if i&2 != 0 {
			cy += h
		}
		px, py, okc := Apply(m, cx, cy)
		if !okc {
			return 0, 0, 0, 0, false
		}
		if i == 0 {
			minX, maxX, minY, maxY = px, px, py, py
			continue
		}
		minX, maxX = min(minX, px), max(maxX, px)
		minY, maxY = min(minY, py), max(maxY, py)
	}
	return minX, minY, maxX - minX, maxY - minY, true
}

// AffineAt returns the flat (affine) map that best matches h near
// (x, y): it maps (x, y) where h does, and it stretches and turns the
// area around that point the way h does there. A backend with no
// perspective draws with it in place of h. The result is in Canvas2D
// order: x' = a·x + c·y + e, y' = b·x + d·y + f. ok is false when (x, y)
// lands at or behind the viewer. It repeats gui.xfMat.affineAt, kept
// separate because package gui cannot import a backend-internal package.
func AffineAt(h *Mat, x, y float32) (a, b, c, d, e, f float32, ok bool) {
	w := h[6]*x + h[7]*y + h[8]
	if !(w > 0) {
		return 0, 0, 0, 0, 0, 0, false
	}
	px := (h[0]*x + h[1]*y + h[2]) / w
	py := (h[3]*x + h[4]*y + h[5]) / w
	// The derivative of (u/w, v/w): (du - p·dw) / w.
	a = (h[0] - px*h[6]) / w // d px / d x
	c = (h[1] - px*h[7]) / w // d px / d y
	b = (h[3] - py*h[6]) / w // d py / d x
	d = (h[4] - py*h[7]) / w // d py / d y
	e = px - a*x - c*y
	f = py - b*x - d*y
	return a, b, c, d, e, f, true
}
