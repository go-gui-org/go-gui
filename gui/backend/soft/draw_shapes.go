package soft

import (
	"math"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
)

// drawRect fills a (possibly rounded) rectangle.
func (r *renderer) drawRect(cmd *gui.RenderCmd) {
	if !cmd.Fill || cmd.Color.A == 0 {
		return
	}
	s := r.scale
	x, y, w, h := cmd.X*s, cmd.Y*s, cmd.W*s, cmd.H*s
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
	rad := cmd.Radius * s
	if th <= 0 {
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
	if !cmd.Fill || cmd.Radius <= 0 || cmd.Color.A == 0 {
		return
	}
	s := r.scale
	rad := cmd.Radius * s
	x, y := (cmd.X-cmd.Radius)*s, (cmd.Y-cmd.Radius)*s
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
	dx, dy := x1-x0, y1-y0
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length < 0.001 {
		return
	}
	thick := max(cmd.Thickness*s, 1)
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
