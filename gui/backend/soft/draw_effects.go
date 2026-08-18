package soft

import (
	"image"
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// blurExpand is how far past its own box a blurred shape can paint,
// as a multiple of the blur radius. It is the GPU path's quad
// expansion, kept identical so the two rasterizers agree on which
// pixels a shadow may touch.
const blurExpand = 1.5

// maxBlur caps a blur radius in device pixels. Blur radii arrive from
// widget config and from filter primitives in untrusted SVG, where a
// NaN or a six-digit stdDev would otherwise size a box-blur window
// from unvalidated input; nothing legible needs more spread than this.
const maxBlur = 512

// clampBlur reduces a command's blur radius to the supported range.
// The negated > rejects NaN as well as zero and negatives.
func clampBlur(v float32) float32 {
	if !(v > 0) {
		return 0
	}
	return min(v, maxBlur)
}

// sigmaPerBlur converts a command's blur radius to a Gaussian standard
// deviation. The GPU shaders ramp their SDF alpha linearly across a
// band of width BlurRadius; a Gaussian of half that sigma puts
// essentially all of its transition in the same band, so the two
// agree on the visible extent of the falloff.
const sigmaPerBlur = 0.5

// drawShadow paints a blurred rounded rect offset from its caster, and
// erases the part that would sit under the caster itself.
//
// The GPU path is one SDF quad evaluating two distance fields; the CPU
// path is softRoundRect with the caster cut-out enabled.
func (r *renderer) drawShadow(cmd *gui.RenderCmd) {
	r.softRoundRect(cmd, cmd.OffsetX, cmd.OffsetY, true)
}

// drawBlur paints a rounded rect with a soft edge — the GPU's blur
// pipeline, which is a shadow with no offset and no caster cut-out.
func (r *renderer) drawBlur(cmd *gui.RenderCmd) {
	r.softRoundRect(cmd, 0, 0, false)
}

// softRoundRect rasterizes a rounded rect into a coverage mask, blurs
// the mask, optionally multiplies in the inverse coverage of the
// un-offset caster, and composites the command's color through the
// result. Both soft-edged kinds are that one construction; the shaders
// differ only in whether the second distance field is evaluated.
//
// offX/offY are logical, and are what separates a shadow from its
// caster. The size guards are written as negated > so a NaN extent
// takes the reject branch instead of sizing a full-buffer mask that
// rasterizes to nothing.
func (r *renderer) softRoundRect(cmd *gui.RenderCmd, offX, offY float32,
	cutCaster bool) {

	if cmd.Color.A == 0 || !(cmd.W > 0) || !(cmd.H > 0) {
		return
	}
	s := r.scale
	w, h := cmd.W*s, cmd.H*s
	blur := clampBlur(cmd.BlurRadius * s)
	rad := cmd.Radius * s
	x := (cmd.X + offX) * s
	y := (cmd.Y + offY) * s

	expand := blur * blurExpand
	region := deviceRect(x-expand, y-expand, w+2*expand, h+2*expand).
		Intersect(r.buf.img.Bounds())
	if region.Empty() {
		return
	}

	mask := r.coverageRoundRect(&r.maskPix, region, x, y, w, h, rad)
	if mask == nil {
		return
	}
	if blur > 0 {
		boxBlurAlpha(mask, &r.blurTmp, blur*sigmaPerBlur)
	}
	if cutCaster {
		// Erase the shadow where the caster covers it: the caster is
		// the un-offset rect, and the shader's smoothstep(-1, 0, d_c)
		// is a coverage complement.
		caster := r.coverageRoundRect(&r.maskPix2, region,
			cmd.X*s, cmd.Y*s, w, h, rad)
		if caster != nil {
			subtractCoverage(mask, caster)
		}
	}
	r.compositeMask(mask, cmd.Color)
}

// compositeMask blends a straight color into the current target through
// a coverage mask, restricted to the clip rect.
func (r *renderer) compositeMask(mask *image.Alpha, c gui.Color) {
	rect := mask.Rect.Intersect(r.buf.clip)
	if rect.Empty() {
		return
	}
	pr, pg, pb, pa := premul(c)
	pix := r.buf.img.Pix
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		mi := mask.PixOffset(rect.Min.X, y)
		di := r.buf.img.PixOffset(rect.Min.X, y)
		for x := rect.Min.X; x < rect.Max.X; x++ {
			m := uint32(mask.Pix[mi])
			mi++
			if m == 0 {
				di += 4
				continue
			}
			sr := uint32(pr) * m / 255
			sg := uint32(pg) * m / 255
			sb := uint32(pb) * m / 255
			sa := uint32(pa) * m / 255
			inv := 255 - sa
			pix[di] = overPremul(sr, uint32(pix[di]), inv)
			pix[di+1] = overPremul(sg, uint32(pix[di+1]), inv)
			pix[di+2] = overPremul(sb, uint32(pix[di+2]), inv)
			pix[di+3] = overPremul(sa, uint32(pix[di+3]), inv)
			di += 4
		}
	}
	r.buf.markDirty(rect)
}

// subtractCoverage multiplies dst by the complement of sub, in place.
// Both masks must cover the same rectangle.
func subtractCoverage(dst, sub *image.Alpha) {
	n := min(len(dst.Pix), len(sub.Pix))
	for i := range n {
		dst.Pix[i] = uint8(uint32(dst.Pix[i]) *
			uint32(255-sub.Pix[i]) / 255)
	}
}

// --- Box blur ---
//
// Three box passes approximate a Gaussian to within about 3% — the
// standard result — at a cost independent of the radius, which is what
// makes a large shadow blur affordable on the CPU. Everything outside
// the blurred rectangle counts as zero: a mask's region is expanded by
// blurExpand before rasterizing, so the falloff is already inside it,
// and a filter layer really is transparent out there.

// boxBlurAlpha blurs a coverage mask in place.
func boxBlurAlpha(m *image.Alpha, tmp *[]uint8, sigma float32) {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	blurPlanes(m.Pix, w, h, m.Stride, 1, 1, tmp, sigma)
}

// boxBlurRGBA blurs every channel of rect in img in place. The pixels
// are premultiplied, so the four channels blur independently without
// the halo a straight-alpha blur would produce.
func boxBlurRGBA(img *image.RGBA, rect image.Rectangle, tmp *[]uint8,
	sigma float32) {

	if rect.Empty() {
		return
	}
	pix := img.Pix[img.PixOffset(rect.Min.X, rect.Min.Y):]
	blurPlanes(pix, rect.Dx(), rect.Dy(), img.Stride, 4, 4, tmp, sigma)
}

// blurPlanes runs the three-pass box blur over every interleaved
// channel of a strided plane.
func blurPlanes(pix []byte, w, h, stride, pixStride, channels int,
	tmp *[]uint8, sigma float32) {

	// Negated > rejects a NaN sigma, which would otherwise reach
	// boxRadii and size the window loops from int(NaN).
	if w <= 0 || h <= 0 || !(sigma > 0) {
		return
	}
	// A box wider than the plane already averages all of it, so a
	// larger sigma buys nothing and only lengthens the window loops.
	sigma = min(sigma, float32(max(w, h)))
	radii := boxRadii(sigma)
	need := w * h
	if cap(*tmp) < need {
		*tmp = make([]uint8, need)
	}
	scratch := (*tmp)[:need]
	for ch := range channels {
		for _, rad := range radii {
			if rad <= 0 {
				continue
			}
			boxPassH(pix, scratch, w, h, stride, pixStride, ch, rad)
			boxPassV(scratch, pix, w, h, stride, pixStride, ch, rad)
		}
	}
}

// boxPassH averages a horizontal window into the packed scratch plane.
func boxPassH(src []byte, dst []uint8, w, h, stride, pixStride, ch,
	rad int) {

	win := uint32(2*rad + 1)
	for y := range h {
		row := src[y*stride+ch:]
		out := dst[y*w:]
		var sum uint32
		for i := -rad; i <= rad; i++ {
			if i >= 0 && i < w {
				sum += uint32(row[i*pixStride])
			}
		}
		for x := range w {
			out[x] = uint8(sum / win)
			// Slide: drop the leftmost sample, add the next one.
			if lo := x - rad; lo >= 0 && lo < w {
				sum -= uint32(row[lo*pixStride])
			}
			if hi := x + rad + 1; hi >= 0 && hi < w {
				sum += uint32(row[hi*pixStride])
			}
		}
	}
}

// boxPassV averages a vertical window from the packed scratch plane
// back into the interleaved destination.
func boxPassV(src []uint8, dst []byte, w, h, stride, pixStride, ch,
	rad int) {

	win := uint32(2*rad + 1)
	for x := range w {
		var sum uint32
		for i := -rad; i <= rad; i++ {
			if i >= 0 && i < h {
				sum += uint32(src[i*w+x])
			}
		}
		for y := range h {
			dst[y*stride+x*pixStride+ch] = uint8(sum / win)
			if lo := y - rad; lo >= 0 && lo < h {
				sum -= uint32(src[lo*w+x])
			}
			if hi := y + rad + 1; hi >= 0 && hi < h {
				sum += uint32(src[hi*w+x])
			}
		}
	}
}

// boxRadii picks three box radii whose successive application has the
// same variance as a Gaussian of the given sigma (Kovesi's
// approximation), so the passes land on the intended blur width rather
// than an arbitrary one.
func boxRadii(sigma float32) [3]int {
	const n = 3
	wIdeal := math.Sqrt(float64(12*sigma*sigma)/n + 1)
	wl := int(math.Floor(wIdeal))
	if wl%2 == 0 {
		wl--
	}
	wu := wl + 2
	mIdeal := (12*float64(sigma*sigma) - float64(n*wl*wl) -
		float64(4*n*wl) - float64(3*n)) /
		float64(-4*wl-4)
	m := int(math.Round(mIdeal))

	var radii [3]int
	for i := range n {
		size := wu
		if i < m {
			size = wl
		}
		radii[i] = max((size-1)/2, 0)
	}
	return radii
}
