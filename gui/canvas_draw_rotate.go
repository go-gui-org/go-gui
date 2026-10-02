package gui

import "math"

// canvas_draw_rotate.go — the rotation-specific record paths for
// DrawContext (issue #904).
//
// Split out of canvas_draw_transform.go, which owns the CTM itself:
// both files sit under the same 800-line gate as canvas_draw.go, and
// the rotation fallbacks would push either one past it. What lives
// here are the pure-geometry helpers the rotated recorder branches
// call — corner mapping, loop closing, outline building — not the
// branches themselves, which stay next to the unrotated fast paths
// they guard.

// angle is the turn content is drawn at, in radians. Only meaningful
// when rotated() holds; axis-aligned callers keep angle 0 so a
// mirroring scale never turns text upside down.
//
// Without a mirror it is the rotation of the first basis column. A
// mirror (det < 0) has two readings that differ by π — the turn of
// the first column, or of the second column's preimage (0,1) — since
// R(θ)·diag(1,-1) == R(θ+π)·diag(-1,1). Content is never mirrored, so
// take the reading that stays upright: the second column's when the
// first one points backwards (angleFromCol1). Otherwise a tiny turn
// under ScaleBy(-1, 1) would read as ≈π and flip text and images
// relative to the same mirror with no turn.
func (x canvasXform) angle() float32 {
	if x.angleFromCol1() {
		return float32(math.Atan2(float64(-x.xy), float64(x.yy)))
	}
	return float32(math.Atan2(float64(x.yx), float64(x.xx)))
}

// angleFromCol1 reports whether angle reads the second column: a
// mirror whose first column points backwards. bakeRotatedImage keys
// its anchor corner on the same test, so the two cannot disagree.
func (x canvasXform) angleFromCol1() bool { return x.det() < 0 && x.xx < 0 }

// xfCorners maps the four corners of an axis rect through the full
// affine, in outline order. A rotated rect is a parallelogram, which
// the recorder's axis-aligned rect methods cannot express — the
// rotated callers hand this to the polygon methods instead.
func (x canvasXform) xfCorners(rx, ry, w, h float32) [8]float32 {
	x0, y0 := x.apply(rx, ry)
	x1, y1 := x.apply(rx+w, ry)
	x2, y2 := x.apply(rx+w, ry+h)
	x3, y3 := x.apply(rx, ry+h)
	return [8]float32{x0, y0, x1, y1, x2, y2, x3, y3}
}

// xfBBox is the axis-aligned bounding box of the mapped rect. Used
// where the sink is axis-aligned by construction — an image (whose
// content the rotation bracket turns, but whose export entry has no
// angle) and a scissor clip — and documented as approximate there.
func (x canvasXform) xfBBox(rx, ry, w, h float32) (float32, float32, float32, float32) {
	c := x.xfCorners(rx, ry, w, h)
	minX := min(c[0], c[2], c[4], c[6])
	maxX := max(c[0], c[2], c[4], c[6])
	minY := min(c[1], c[3], c[5], c[7])
	maxY := max(c[1], c[3], c[5], c[7])
	return minX, minY, maxX - minX, maxY - minY
}

// closedLoop maps points and closes the loop for a stroked outline.
// The mapped copy lives in the shared scratch buffer, so the next
// primitive overwrites it, which is exactly the retention rule
// DrawRecorder states. It maps into the buffer itself rather than
// through xfPoints so the grown slice, close point included, is
// stored back: appending the close onto xfPoints' result would
// reallocate the buffer on every call.
func (r *xformRecorder) closedLoop(points []float32) []float32 {
	r.dc.xfPtBuf = append(r.dc.xfPtBuf[:0], points...)
	buf := r.dc.xfPtBuf
	for i := 0; i+1 < len(buf); i += 2 {
		buf[i], buf[i+1] = r.xf.apply(buf[i], buf[i+1])
	}
	if len(buf) >= 2 {
		r.dc.xfPtBuf = append(r.dc.xfPtBuf, buf[0], buf[1])
	}
	return r.dc.xfPtBuf
}

// roundedOutline builds the rounded-rect outline in local space,
// mirroring the main RoundedRect path: four corner arcs at eight
// segments each. A zero radius is a plain rect outline. The loop is
// left OPEN: a fill needs no close point, and the stroke closes it
// through closedLoop, which would otherwise add a second, zero-length
// closing segment.
//
// Both shapes build into the pooled roundRectBuf, so neither
// allocates per call.
func (r *xformRecorder) roundedOutline(x, y, w, h, radius float32) []float32 {
	rad := min(radius, w/2, h/2)
	pts := r.dc.roundRectBuf[:0]
	if rad <= 0 {
		pts = append(pts, x, y, x+w, y, x+w, y+h, x, y+h)
	} else {
		const segs = 8
		pts = appendArcPoints(pts, x+rad, y+rad, rad, math.Pi, segs)
		pts = appendArcPoints(pts, x+w-rad, y+rad, rad, 3*math.Pi/2, segs)
		pts = appendArcPoints(pts, x+w-rad, y+h-rad, rad, 0, segs)
		pts = appendArcPoints(pts, x+rad, y+h-rad, rad, math.Pi/2, segs)
	}
	r.dc.roundRectBuf = pts
	return pts
}
