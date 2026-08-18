package soft

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
)

// kappa is the circle-to-cubic-Bezier constant: the control-point
// distance, as a fraction of the radius, that makes a cubic segment
// approximate a quarter circle to within ~0.02%.
const kappa = 0.5522847498

// buffer is the CPU render target: a premultiplied RGBA image plus the
// current clip rectangle, both in device pixels.
//
// Everything this package draws reduces to one operation — composite a
// source (a solid color, a gradient, an image) through a coverage mask,
// restricted to the clip rect. fillPath is that operation; the coverage
// mask comes from x/image/vector, which antialiases every edge.
type buffer struct {
	img  *image.RGBA
	clip image.Rectangle
	rast vector.Rasterizer

	// uni is the reusable solid-color source. image.NewUniform would
	// allocate once per draw command; the rasterizer only reads it
	// during the Draw call, so one instance can be re-pointed.
	uni image.Uniform
}

func newBuffer(w, h int) *buffer {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	return &buffer{img: img, clip: img.Bounds()}
}

// clear paints the whole buffer with c, ignoring the clip rect. c is
// straight (non-premultiplied) as every gui.Color is.
func (b *buffer) clear(c gui.Color) {
	r, g, bl, a := premul(c)
	pix := b.img.Pix
	for i := 0; i < len(pix); i += 4 {
		pix[i] = r
		pix[i+1] = g
		pix[i+2] = bl
		pix[i+3] = a
	}
}

// setClip replaces the clip rect, as RenderClip does — clips do not nest
// in the render stream. A degenerate rect restores the full buffer.
func (b *buffer) setClip(x, y, w, h float32) {
	if w <= 0 || h <= 0 {
		b.clip = b.img.Bounds()
		return
	}
	b.clip = deviceRect(x, y, w, h).Intersect(b.img.Bounds())
}

// region returns the pixel rectangle a shape of the given device-space
// bounds can touch: its bounding box, clipped.
func (b *buffer) region(x, y, w, h float32) image.Rectangle {
	return deviceRect(x, y, w, h).Intersect(b.clip)
}

// deviceRect converts a device-space box to the pixel rectangle that can
// be partially covered by it.
func deviceRect(x, y, w, h float32) image.Rectangle {
	return image.Rect(
		int(math.Floor(float64(x))),
		int(math.Floor(float64(y))),
		int(math.Ceil(float64(x+w))),
		int(math.Ceil(float64(y+h))),
	)
}

// fillPath rasterizes the path emitted by emit and composites src through
// the resulting coverage mask into region.
//
// emit receives the region origin because the rasterizer's mask is
// indexed from region.Min: every coordinate fed to it must be device
// space minus that origin. Geometry outside the mask is clipped by the
// rasterizer itself, which is why the clip rect costs nothing here.
//
// src is sampled at absolute device coordinates (sp = region.Min), so a
// gradient or image source does not need to know where it was clipped.
func (b *buffer) fillPath(region image.Rectangle, src image.Image,
	emit func(z *vector.Rasterizer, ox, oy float32)) {
	if region.Empty() {
		return
	}
	b.rast.Reset(region.Dx(), region.Dy())
	emit(&b.rast, float32(region.Min.X), float32(region.Min.Y))
	b.rast.Draw(b.img, region, src, region.Min)
}

// solid returns the reusable uniform source for c. Valid until the next
// call — pass it straight to fillPath.
func (b *buffer) solid(c gui.Color) image.Image {
	b.uni.C = color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	return &b.uni
}

// blendPixel composites one straight-alpha source color over the pixel at
// (x, y). Used by the paths that sample their own source — glyph atlas
// quads — where a coverage mask would be a detour.
//
// Both callers iterate a region already intersected with the clip rect
// (glyphBackend's region()), so no per-pixel clip test is needed.
func (b *buffer) blendPixel(x, y int, sr, sg, sb, sa uint32) {
	if sa == 0 {
		return
	}
	i := b.img.PixOffset(x, y)
	pix := b.img.Pix
	inv := 255 - sa
	pix[i] = uint8((sr*sa + uint32(pix[i])*inv + 127) / 255)
	pix[i+1] = uint8((sg*sa + uint32(pix[i+1])*inv + 127) / 255)
	pix[i+2] = uint8((sb*sa + uint32(pix[i+2])*inv + 127) / 255)
	pix[i+3] = uint8((sa*255 + uint32(pix[i+3])*inv + 127) / 255)
}

// premul converts a straight gui.Color to premultiplied bytes.
func premul(c gui.Color) (r, g, b, a uint8) {
	if c.A == 255 {
		return c.R, c.G, c.B, 255
	}
	a32 := uint32(c.A)
	return uint8(uint32(c.R) * a32 / 255),
		uint8(uint32(c.G) * a32 / 255),
		uint8(uint32(c.B) * a32 / 255),
		c.A
}

// --- Path builders ---
//
// All take device-space coordinates and the region origin to subtract.
// A reversed contour winds the other way, which the rasterizer's signed
// accumulation turns into a hole — that is how stroke rects get their
// inner cut-out without a second pass.

// pathRect emits an axis-aligned rectangle.
func pathRect(z *vector.Rasterizer, ox, oy, x, y, w, h float32) {
	x -= ox
	y -= oy
	z.MoveTo(x, y)
	z.LineTo(x+w, y)
	z.LineTo(x+w, y+h)
	z.LineTo(x, y+h)
	z.ClosePath()
}

// pathRoundRect emits a rectangle with radius-r corners. r is clamped to
// half the shorter side, so a radius of half a square's width is a circle.
// reverse winds the contour backwards.
func pathRoundRect(z *vector.Rasterizer, ox, oy, x, y, w, h, r float32,
	reverse bool) {
	if w <= 0 || h <= 0 {
		return
	}
	r = min(r, min(w, h)/2)
	if r <= 0 {
		if reverse {
			x -= ox
			y -= oy
			z.MoveTo(x, y)
			z.LineTo(x, y+h)
			z.LineTo(x+w, y+h)
			z.LineTo(x+w, y)
			z.ClosePath()
			return
		}
		pathRect(z, ox, oy, x, y, w, h)
		return
	}
	x -= ox
	y -= oy
	k := r * kappa
	x1, y1 := x+w, y+h
	if !reverse {
		z.MoveTo(x+r, y)
		z.LineTo(x1-r, y)
		z.CubeTo(x1-r+k, y, x1, y+r-k, x1, y+r)
		z.LineTo(x1, y1-r)
		z.CubeTo(x1, y1-r+k, x1-r+k, y1, x1-r, y1)
		z.LineTo(x+r, y1)
		z.CubeTo(x+r-k, y1, x, y1-r+k, x, y1-r)
		z.LineTo(x, y+r)
		z.CubeTo(x, y+r-k, x+r-k, y, x+r, y)
		z.ClosePath()
		return
	}
	z.MoveTo(x+r, y)
	z.CubeTo(x+r-k, y, x, y+r-k, x, y+r)
	z.LineTo(x, y1-r)
	z.CubeTo(x, y1-r+k, x+r-k, y1, x+r, y1)
	z.LineTo(x1-r, y1)
	z.CubeTo(x1-r+k, y1, x1, y1-r+k, x1, y1-r)
	z.LineTo(x1, y+r)
	z.CubeTo(x1, y+r-k, x1-r+k, y, x1-r, y)
	z.ClosePath()
}

// pathQuad emits an arbitrary four-point polygon, wound in the order
// given. Used for lines and transformed glyph quads.
func pathQuad(z *vector.Rasterizer, ox, oy float32, pts [4][2]float32) {
	z.MoveTo(pts[0][0]-ox, pts[0][1]-oy)
	z.LineTo(pts[1][0]-ox, pts[1][1]-oy)
	z.LineTo(pts[2][0]-ox, pts[2][1]-oy)
	z.LineTo(pts[3][0]-ox, pts[3][1]-oy)
	z.ClosePath()
}
