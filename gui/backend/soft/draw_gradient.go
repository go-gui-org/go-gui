package soft

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
)

// gradientSrc samples a gui.GradientDef at absolute device coordinates.
// It reproduces the GPU gradient shader's parameterisation: the quad is
// mapped to uv in [-1,1], a linear gradient projects uv onto the
// direction vector, a radial one measures distance from the centre
// against the larger half-extent.
type gradientSrc struct {
	stops  []gui.GradientStop
	cx, cy float32 // centre, device px
	hw, hh float32 // half extents, device px
	dx, dy float32 // linear direction
	radius float32 // radial target radius
	radial bool
	// nr is the scratch color each At returns: vector.Draw consumes
	// the sample immediately, so one instance can be re-pointed.
	nr color.NRGBA
}

func (g *gradientSrc) ColorModel() color.Model { return color.NRGBAModel }

// Bounds covers the whole plane: the rasterizer clips to the draw region
// and never consults the source's bounds.
func (g *gradientSrc) Bounds() image.Rectangle {
	const big = 1 << 29
	return image.Rect(-big, -big, big, big)
}

func (g *gradientSrc) At(x, y int) color.Color {
	// Sample at the pixel centre.
	u := (float32(x) + 0.5 - g.cx) / g.hw
	v := (float32(y) + 0.5 - g.cy) / g.hh
	var t float32
	if g.radial {
		px, py := u*g.hw, v*g.hh
		t = float32(math.Hypot(float64(px), float64(py))) / g.radius
	} else {
		t = (u*g.dx+v*g.dy)*0.5 + 0.5
	}
	c := gui.SampleGradientStopColor(g.stops, min(max(t, 0), 1))
	g.nr = color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	return &g.nr
}

// drawGradient fills a (possibly rounded) rect with a gradient.
//
// Unlike the GPU path it does not resample to five stops: a CPU sampler
// has no uniform budget, so the full stop list is honoured — the same
// choice the web backend's canvas gradients make.
func (r *renderer) drawGradient(cmd *gui.RenderCmd) {
	if cmd.Gradient == nil || cmd.W <= 0 || cmd.H <= 0 {
		return
	}
	stops := gui.NormalizeGradientStops(cmd.Gradient.Stops, &r.normStops)
	if len(stops) == 0 {
		return
	}
	s := r.scale
	x, y, w, h := cmd.X*s, cmd.Y*s, cmd.W*s, cmd.H*s
	rad := cmd.Radius * s

	src := &gradientSrc{
		stops: stops,
		cx:    x + w/2,
		cy:    y + h/2,
		hw:    w / 2,
		hh:    h / 2,
	}
	if cmd.Gradient.Type == gui.GradientRadial {
		src.radial = true
		src.radius = max(src.hw, src.hh)
	} else {
		// GradientDir wants logical extents; direction is scale-free.
		src.dx, src.dy = gui.GradientDir(cmd.Gradient, cmd.W, cmd.H)
	}

	region := r.buf.region(x, y, w, h)
	r.buf.fillPath(region, src,
		func(z *vector.Rasterizer, ox, oy float32) {
			pathRoundRect(z, ox, oy, x, y, w, h, rad, false)
		})
}

// drawGradientBorder paints the four edge strips, each a flat sample of
// the gradient, exactly as the GPU backends do.
func (r *renderer) drawGradientBorder(cmd *gui.RenderCmd) {
	if cmd.Gradient == nil || len(cmd.Gradient.Stops) == 0 {
		return
	}
	s := r.scale
	rects := gui.GradientBorderRects(cmd)
	for i := range rects {
		rc := &rects[i]
		if rc.W <= 0 || rc.H <= 0 || rc.Color.A == 0 {
			continue
		}
		x, y := rc.X*s, rc.Y*s
		w, h := rc.W*s, rc.H*s
		region := r.buf.region(x, y, w, h)
		r.buf.fillPath(region, r.buf.solid(rc.Color),
			func(z *vector.Rasterizer, ox, oy float32) {
				pathRect(z, ox, oy, x, y, w, h)
			})
	}
}
