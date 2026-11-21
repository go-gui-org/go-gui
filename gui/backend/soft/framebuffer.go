package soft

import (
	"image"
	"image/color"
	"image/draw"
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

	// dirty is the union of every pixel rectangle written since the
	// buffer was last cleared. A layer composite walks it instead of
	// the whole image, so a bracket around one small widget costs a
	// small pass even though layers are allocated full size.
	dirty image.Rectangle

	// uni is the reusable solid-color source. image.NewUniform would
	// allocate once per draw command; the rasterizer only reads it
	// during the Draw call, so one instance can be re-pointed.
	uni image.Uniform
	// col is the color uni.C points at. Storing a color.NRGBA value in
	// the color.Color interface boxes it, one allocation per solid draw
	// (#1265 step 5); a pointer fits the interface word, so solid writes
	// col and points uni.C at it with no allocation.
	col color.NRGBA

	// mask is the coverage scratch fillPath rasterizes into for every
	// source that is not a uniform (#1282); see compositeSampled.
	mask maskScratch
}

// maskScratch is a reusable coverage mask: the pixel slice and the
// image.Alpha header over it. Holding the header here, not building one
// per call, is what keeps a mask from allocating — a returned
// &image.Alpha{} escapes to the heap.
type maskScratch struct {
	pix []uint8
	img image.Alpha
}

// reset sizes the mask to region and returns it. The pixels are not
// cleared: every caller rasterizes with draw.Src, which overwrites them.
func (m *maskScratch) reset(region image.Rectangle) *image.Alpha {
	n := region.Dx() * region.Dy()
	if cap(m.pix) < n {
		m.pix = make([]uint8, n)
	}
	m.img = image.Alpha{Pix: m.pix[:n], Stride: region.Dx(), Rect: region}
	return &m.img
}

func newBuffer(w, h int) *buffer {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	return &buffer{img: img, clip: img.Bounds()}
}

// markDirty widens the written-pixels box. Callers that composite their
// own source pixel by pixel — the glyph atlas blitters — call it once
// with the region they are about to walk.
func (b *buffer) markDirty(r image.Rectangle) {
	if r.Empty() {
		return
	}
	if b.dirty.Empty() {
		b.dirty = r
		return
	}
	b.dirty = b.dirty.Union(r)
}

// clear paints the whole buffer with c, ignoring the clip rect. c is
// straight (non-premultiplied) as every gui.Color is.
//
// One pixel is stored and then doubled with copy, which is a memmove. A
// loop that stored four bytes per pixel was the largest single cost of a
// simple frame: 36 % of Draw at 1920×1080 (#1327).
func (b *buffer) clear(c gui.Color) {
	r, g, bl, a := premul(c)
	b.dirty = b.img.Bounds()
	pix := b.img.Pix
	if len(pix) < 4 {
		return
	}
	if r|g|bl|a == 0 {
		clear(pix)
		return
	}
	pix[0], pix[1], pix[2], pix[3] = r, g, bl, a
	for n := 4; n < len(pix); {
		n += copy(pix[n:], pix[:n])
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

// floor32 is math.Floor without the float64 round trip at the call site.
func floor32(v float32) float32 {
	return float32(math.Floor(float64(v)))
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
	// A uniform source onto *image.RGBA is the rasterizer's typed fast
	// path. Any other source would take its generic loop, which allocates
	// per pixel (dst.At boxes a color.RGBA, dst.Set converts through
	// RGBAModel) and once per call (its RGBA64 scratch escapes), so those
	// go through a coverage mask and compositeSampled instead (#1282).
	if _, uniform := src.(*image.Uniform); uniform {
		b.rast.Draw(b.img, region, src, region.Min)
	} else {
		mask := b.mask.reset(region)
		// draw.Src onto *image.Alpha with an opaque source is the
		// rasterizer's other fast path: it writes coverage straight
		// into the mask. DrawOp is restored for the uniform path, which
		// must blend (draw.Over is the Rasterizer's zero value).
		b.rast.DrawOp = draw.Src
		b.rast.Draw(mask, region, image.Opaque, image.Point{})
		b.rast.DrawOp = draw.Over
		b.compositeSampled(region, src, mask)
	}
	b.markDirty(region)
}

// compositeSampled blends src over the buffer through mask, inside
// region. src is sampled at absolute device coordinates, as fillPath
// documents.
//
// The arithmetic is vector.Rasterizer's generic draw.Over loop
// (rasterizeOpOver) transcribed onto typed pixels: 16-bit straight
// channels from src.At, 16-bit destination channels, and the result's
// high byte stored — what image.RGBA.Set keeps. Keeping that formula,
// rather than the 8-bit overPremul composite the layers use, is what
// makes fully covered pixels byte-identical to the generic loop. The
// one difference is coverage: the generic loop reads 16-bit coverage,
// and the mask holds only its high byte, widened back here as m*0x101.
// That moves antialiased edge pixels by at most one level; fully
// covered (0xff -> 0xffff) and uncovered pixels are exact.
//
// The sources reaching here (gradientSrc, imageSrc) return a pointer to
// a scratch color from At, so a sample does not allocate either.
func (b *buffer) compositeSampled(region image.Rectangle, src image.Image,
	mask *image.Alpha) {

	// The rasterizer clips its draw rect to the destination's bounds;
	// this loop indexes Pix directly, so it must too. Callers pass a
	// region already inside the clip, which lies inside the buffer, but
	// an out-of-range region here would wrap rows or panic rather than
	// clip. The mask still covers all of region, so it indexes by
	// absolute coordinates either way.
	rect := region.Intersect(b.img.Bounds())
	pix := b.img.Pix
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		mi := mask.PixOffset(rect.Min.X, y)
		di := b.img.PixOffset(rect.Min.X, y)
		for x := rect.Min.X; x < rect.Max.X; x, mi, di = x+1, mi+1, di+4 {
			m := uint32(mask.Pix[mi])
			// Zero coverage leaves the pixel as it was: the generic
			// formula reduces to d*0xffff/0xffff. Skipping it also skips
			// the sample, which is most of the cost.
			if m == 0 {
				continue
			}
			ma := m * 0x101
			sr, sg, sb, sa := src.At(x, y).RGBA()
			a := 0xffff - (sa * ma / 0xffff)
			p := pix[di : di+4 : di+4]
			// uint32(p)*0x101 is what color.RGBA.RGBA() widens a byte to.
			// No uint32 overflow: RGBA() is premultiplied (s <= sa), so
			// d*a + s*ma <= 0xffff*0xffff + 0xfffe, as in vector's loop.
			p[0] = uint8((uint32(p[0])*0x101*a + sr*ma) / 0xffff >> 8)
			p[1] = uint8((uint32(p[1])*0x101*a + sg*ma) / 0xffff >> 8)
			p[2] = uint8((uint32(p[2])*0x101*a + sb*ma) / 0xffff >> 8)
			p[3] = uint8((uint32(p[3])*0x101*a + sa*ma) / 0xffff >> 8)
		}
	}
}

// solid returns the reusable uniform source for c. Valid until the next
// call — pass it straight to fillPath.
func (b *buffer) solid(c gui.Color) image.Image {
	b.col = color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	// Re-pointed every call, not once in newBuffer: cheap, and right even
	// for a buffer that was copied or built without newBuffer.
	b.uni.C = &b.col
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

// overPremul composites one premultiplied source channel over one
// destination channel: s + dst*inv/255, saturating.
//
// The clamp is not decoration. A filter's color matrix clamps each
// channel independently, so it can leave a pixel whose color exceeds
// its own alpha; the unclamped sum then passes 255 and wraps a bright
// pixel to near-black. Saturating is what the GPU blend does.
func overPremul(s, d, inv uint32) uint8 {
	v := s + (d*inv+127)/255
	if v > 255 {
		return 255
	}
	return uint8(v)
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
	// Written as negated > rather than <= so a NaN — which a hostile
	// SVG radius or a NaN-producing animation can reach here through
	// the shadow, blur and stencil masks — takes the reject branch
	// instead of emitting NaN control points into the rasterizer.
	if !(w > 0) || !(h > 0) {
		return
	}
	r = min(r, min(w, h)/2)
	if !(r > 0) {
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
