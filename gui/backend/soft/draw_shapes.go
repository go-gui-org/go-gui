package soft

import (
	"math"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
)

// maxDeviceCoord bounds every coordinate and size that reaches
// deviceRect. Rejecting only NaN and Inf is not enough: a huge finite
// value (1e30), or x+w overflowing float32, falls outside the int
// range, and Go leaves that float-to-int conversion to the
// architecture (amd64 gives MinInt64, arm64 saturates). 1<<24 pixels
// is far past any real framebuffer, and x+w stays at most 1<<25.
const maxDeviceCoord = 1 << 24

// inRange32 reports whether |v| <= maxDeviceCoord. NaN compares false,
// so it fails; so do both infinities.
func inRange32(v float32) bool {
	return v >= -maxDeviceCoord && v <= maxDeviceCoord
}

// validBox reports whether a device-space box has a bounded origin and
// a positive, bounded size. NaN compares false against 0, so it fails
// too.
func validBox(x, y, w, h float32) bool {
	return w > 0 && h > 0 && inRange32(w) && inRange32(h) &&
		inRange32(x) && inRange32(y)
}

// drawRect fills a (possibly rounded) rectangle.
func (r *renderer) drawRect(cmd *gui.RenderCmd) {
	if !cmd.Fill || cmd.Color.A == 0 {
		return
	}
	s := r.scale
	x, y, w, h := cmd.X*s, cmd.Y*s, cmd.W*s, cmd.H*s
	// Reject before region(): a negative box would canonicalize to a
	// non-empty rect, and NaN, Inf or a huge value relies on an
	// arch-specific int conversion.
	if !validBox(x, y, w, h) {
		return
	}
	rad := cmd.Radius * s
	region := r.buf.region(x, y, w, h)
	r.buf.fillPath(region, r.buf.solid(cmd.Color),
		func(z *vector.Rasterizer, ox, oy float32) {
			pathRoundRect(z, ox, oy, x, y, w, h, rad, false)
		})
}

// drawStrokeRect paints a rectangular ring: the outer rounded rect with
// the inner one wound backwards, so the rasterizer's signed coverage
// cancels in the middle. A zero thickness means a fill, matching the GL
// backend's SDF quad.
func (r *renderer) drawStrokeRect(cmd *gui.RenderCmd) {
	if cmd.Color.A == 0 {
		return
	}
	s := r.scale
	th := cmd.Thickness * s
	x, y, w, h := cmd.X*s, cmd.Y*s, cmd.W*s, cmd.H*s
	if !validBox(x, y, w, h) {
		return
	}
	rad := cmd.Radius * s
	// NaN-safe: a NaN thickness takes the fill branch instead of
	// poisoning the inset math below. +Inf is clamped by the min.
	if !(th > 0) {
		region := r.buf.region(x, y, w, h)
		r.buf.fillPath(region, r.buf.solid(cmd.Color),
			func(z *vector.Rasterizer, ox, oy float32) {
				pathRoundRect(z, ox, oy, x, y, w, h, rad, false)
			})
		return
	}
	th = min(th, min(w, h)/2)
	region := r.buf.region(x, y, w, h)
	r.buf.fillPath(region, r.buf.solid(cmd.Color),
		func(z *vector.Rasterizer, ox, oy float32) {
			pathRoundRect(z, ox, oy, x, y, w, h, rad, false)
			pathRoundRect(z, ox, oy, x+th, y+th,
				w-2*th, h-2*th, rad-th, true)
		})
}

// drawCircle fills a circle centred on (X, Y), as the GL backend does.
func (r *renderer) drawCircle(cmd *gui.RenderCmd) {
	// Written NaN-safe: Radius<=0 passes NaN through to deviceRect,
	// whose int(NaN) conversion is arch-specific. The range check on
	// the scaled radius below also rejects +Inf and huge values, which
	// the !(v > 0) idiom passes.
	if !cmd.Fill || !(cmd.Radius > 0) || cmd.Color.A == 0 {
		return
	}
	s := r.scale
	rad := cmd.Radius * s
	if !(rad > 0) || !inRange32(rad) {
		return
	}
	x, y := (cmd.X-cmd.Radius)*s, (cmd.Y-cmd.Radius)*s
	// d = 2*rad and x+d stay inside the int range once both are bounded.
	if !inRange32(x) || !inRange32(y) {
		return
	}
	d := 2 * rad
	region := r.buf.region(x, y, d, d)
	r.buf.fillPath(region, r.buf.solid(cmd.Color),
		func(z *vector.Rasterizer, ox, oy float32) {
			pathRoundRect(z, ox, oy, x, y, d, d, rad, false)
		})
}

// drawLine paints a line as a quad expanded either side of the segment,
// the same construction the GL backend uses.
func (r *renderer) drawLine(cmd *gui.RenderCmd) {
	if cmd.Color.A == 0 {
		return
	}
	s := r.scale
	x0, y0 := cmd.X*s, cmd.Y*s
	x1, y1 := cmd.OffsetX*s, cmd.OffsetY*s
	// Bound the endpoints so the quad's min/max below stay inside the
	// int range deviceRect converts to.
	if !inRange32(x0) || !inRange32(y0) || !inRange32(x1) || !inRange32(y1) {
		return
	}
	dx, dy := x1-x0, y1-y0
	length := float32(math.Hypot(float64(dx), float64(dy)))
	// NaN-safe: NaN<0.001 is false and would pass a NaN through to
	// the quad math and deviceRect below. Bounded endpoints keep the
	// length finite.
	if !(length >= 0.001) {
		return
	}
	thick := max(cmd.Thickness*s, 1)
	// max(NaN, 1) is NaN. A NaN, infinite or huge thickness falls back
	// to 1px, which keeps the quad inside the int range.
	if !(thick <= maxDeviceCoord) {
		thick = 1
	}
	nx := -dy / length * thick * 0.5
	ny := dx / length * thick * 0.5

	pts := [4][2]float32{
		{x0 + nx, y0 + ny},
		{x1 + nx, y1 + ny},
		{x1 - nx, y1 - ny},
		{x0 - nx, y0 - ny},
	}
	minX := min(min(pts[0][0], pts[1][0]), min(pts[2][0], pts[3][0]))
	minY := min(min(pts[0][1], pts[1][1]), min(pts[2][1], pts[3][1]))
	maxX := max(max(pts[0][0], pts[1][0]), max(pts[2][0], pts[3][0]))
	maxY := max(max(pts[0][1], pts[1][1]), max(pts[2][1], pts[3][1]))

	region := r.buf.region(minX, minY, maxX-minX, maxY-minY)
	r.buf.fillPath(region, r.buf.solid(cmd.Color),
		func(z *vector.Rasterizer, ox, oy float32) {
			pathQuad(z, ox, oy, pts)
		})
}
