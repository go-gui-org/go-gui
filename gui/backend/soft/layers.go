package soft

import (
	"image"
	"image/draw"

	"golang.org/x/image/vector"
)

// maxLayerDepth caps how deep begin/end brackets may nest before the
// renderer stops allocating offscreen layers. A stream that deep is
// pathological; drawing straight into the parent keeps the pairing
// intact and loses only the scoped effect.
const maxLayerDepth = 16

// bracketKind identifies which begin/end pair opened a layer.
type bracketKind uint8

const (
	bracketFilter bracketKind = iota
	bracketStencil
	bracketRotate
)

// bracket is one open begin/end pair.
//
// Every phase 2 kind that scopes later drawing works the same way: push
// an offscreen layer, let the bracketed commands draw into it, then
// composite it back with the scoped effect applied. Doing it with a
// layer rather than with per-draw state is what keeps text, gradients
// and images correct inside a rotation or a stencil clip without any
// phase 1 draw path knowing the bracket exists.
//
// A nil layer means the depth cap was hit: the bracketed commands drew
// straight into the parent and the matching end composites nothing.
type bracket struct {
	layer  *buffer
	matrix *[16]float32 // filter: color transform

	// Stencil geometry / rotation centre, device px.
	x, y, w, h, radius float32
	angle, cx, cy      float32

	blur   float32 // filter: blur std dev, device px
	layers int     // filter: composite count

	kind bracketKind
}

// pushLayer opens a bracket and re-points the render target at a fresh
// transparent layer. The layer inherits the parent's clip rect, and the
// clip the bracketed commands leave behind is carried back on pop —
// the GPU backends do not restore the scissor across an FBO bind
// either, and the render stream re-emits the clip it wants.
func (r *renderer) pushLayer(k bracketKind) *bracket {
	br := bracket{kind: k}
	if len(r.brackets) < maxLayerDepth {
		br.layer = r.acquireLayer()
		br.layer.clip = r.buf.clip
		r.setTarget(br.layer)
	}
	r.brackets = append(r.brackets, br)
	return &r.brackets[len(r.brackets)-1]
}

// popLayer closes the innermost bracket and restores the parent target.
// It reports false when the stream is unbalanced or the innermost
// bracket was opened by a different kind, in which case nothing is
// popped: a malformed stream must not unwind a bracket it did not open.
func (r *renderer) popLayer(k bracketKind) (bracket, bool) {
	n := len(r.brackets)
	if n == 0 || r.brackets[n-1].kind != k {
		return bracket{}, false
	}
	br := r.brackets[n-1]
	r.brackets = r.brackets[:n-1]

	// The new target is the nearest remaining bracket with a real
	// layer, or the root when none is left. A bracket pushed past
	// maxLayerDepth never switched the target, so the top of the stack
	// is not necessarily the current target.
	parent := r.rootBuf
	for i := n - 2; i >= 0; i-- {
		if l := r.brackets[i].layer; l != nil {
			parent = l
			break
		}
	}
	if br.layer != nil {
		parent.clip = br.layer.clip
	}
	r.setTarget(parent)
	return br, true
}

// setTarget re-points both the shape target and the glyph backend, so
// text lands in the same layer everything else does. The warming pass
// deliberately keeps a nil glyph target — its quads are discarded.
func (r *renderer) setTarget(b *buffer) {
	r.buf = b
	// glyphBack is nil in tests that drive shapes only.
	if !r.warm && r.glyphBack != nil {
		r.glyphBack.buf = b
	}
}

// acquireLayer returns a cleared full-size layer, reusing a pooled one
// when there is a spare. Layers are full size rather than bracket size
// so every coordinate stays in device space: no offset bookkeeping in
// any draw path, and clearing costs only the pooled layer's dirty box.
func (r *renderer) acquireLayer() *buffer {
	if n := len(r.layerPool); n > 0 {
		b := r.layerPool[n-1]
		r.layerPool = r.layerPool[:n-1]
		b.clearTransparent()
		return b
	}
	bnds := r.rootBuf.img.Bounds()
	return newBuffer(bnds.Dx(), bnds.Dy())
}

// releaseLayer returns a layer to the pool for the next bracket.
func (r *renderer) releaseLayer(b *buffer) {
	if b == nil {
		return
	}
	r.layerPool = append(r.layerPool, b)
}

// clearTransparent zeroes the pixels written since the last clear and
// resets the dirty box, readying a pooled layer for reuse.
func (b *buffer) clearTransparent() {
	r := b.dirty.Intersect(b.img.Bounds())
	if !r.Empty() {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			row := b.img.Pix[b.img.PixOffset(r.Min.X, y):]
			row = row[:4*r.Dx()]
			clear(row)
		}
	}
	b.dirty = image.Rectangle{}
	b.clip = b.img.Bounds()
}

// composite draws src over dst inside rect. Both sides are
// premultiplied, so the blend is the plain source-over of two
// premultiplied colors; mask, when non-nil, attenuates the source by a
// per-pixel coverage value (the stencil's rounded-rect mask).
func composite(dst, src *buffer, rect image.Rectangle, mask *image.Alpha) {
	rect = rect.Intersect(dst.img.Bounds()).Intersect(src.img.Bounds())
	if mask != nil {
		rect = rect.Intersect(mask.Rect)
	}
	if rect.Empty() {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		si := src.img.PixOffset(rect.Min.X, y)
		di := dst.img.PixOffset(rect.Min.X, y)
		mi := 0
		if mask != nil {
			mi = mask.PixOffset(rect.Min.X, y)
		}
		for x := rect.Min.X; x < rect.Max.X; x++ {
			sr := uint32(src.img.Pix[si])
			sg := uint32(src.img.Pix[si+1])
			sb := uint32(src.img.Pix[si+2])
			sa := uint32(src.img.Pix[si+3])
			if mask != nil {
				m := uint32(mask.Pix[mi])
				mi++
				sr = sr * m / 255
				sg = sg * m / 255
				sb = sb * m / 255
				sa = sa * m / 255
			}
			si += 4
			if sa == 0 && sr == 0 && sg == 0 && sb == 0 {
				di += 4
				continue
			}
			inv := 255 - sa
			p := dst.img.Pix
			p[di] = overPremul(sr, uint32(p[di]), inv)
			p[di+1] = overPremul(sg, uint32(p[di+1]), inv)
			p[di+2] = overPremul(sb, uint32(p[di+2]), inv)
			p[di+3] = overPremul(sa, uint32(p[di+3]), inv)
			di += 4
		}
	}
	dst.markDirty(rect)
}

// coverageRoundRect rasterizes a rounded rect into an alpha mask
// covering region, backed by the caller's scratch slice so repeated
// brackets do not allocate. The mask is antialiased, where the GPU
// stencil pipeline thresholds at half coverage — a CPU mask
// multiplies, so it can keep the smooth edge.
func (r *renderer) coverageRoundRect(scratch *[]uint8,
	region image.Rectangle, x, y, w, h, rad float32) *image.Alpha {

	return r.coveragePath(scratch, region,
		func(z *vector.Rasterizer, ox, oy float32) {
			pathRoundRect(z, ox, oy, x, y, w, h, rad, false)
		})
}

// coveragePath rasterizes an arbitrary path into an alpha mask covering
// region. draw.Src overwrites the scratch slice rather than blending
// into whatever the last mask left there.
func (r *renderer) coveragePath(scratch *[]uint8, region image.Rectangle,
	emit func(z *vector.Rasterizer, ox, oy float32)) *image.Alpha {

	if region.Empty() {
		return nil
	}
	n := region.Dx() * region.Dy()
	if cap(*scratch) < n {
		*scratch = make([]uint8, n)
	}
	mask := &image.Alpha{
		Pix:    (*scratch)[:n],
		Stride: region.Dx(),
		Rect:   region,
	}
	r.maskRast.Reset(region.Dx(), region.Dy())
	r.maskRast.DrawOp = draw.Src
	emit(&r.maskRast, float32(region.Min.X), float32(region.Min.Y))
	r.maskRast.Draw(mask, region, image.Opaque, image.Point{})
	return mask
}

// blitTransformed composites src over dst with a rotation about
// (cx, cy) in device pixels, inverse-mapping each destination pixel and
// sampling the source bilinearly.
//
// Rotation is applied to a finished layer rather than to each shape's
// geometry, so text, gradients and images rotate as faithfully as
// rectangles do and no phase 1 draw path changes.
func blitTransformed(dst, src *buffer, rect image.Rectangle,
	sin, cos, cx, cy float32) {

	rect = rect.Intersect(dst.img.Bounds())
	if rect.Empty() {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		dy := float32(y) + 0.5 - cy
		di := dst.img.PixOffset(rect.Min.X, y)
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := float32(x) + 0.5 - cx
			// Inverse rotation: the source point that lands here.
			sx := cx + dx*cos + dy*sin
			sy := cy - dx*sin + dy*cos
			sr, sg, sb, sa := sampleBilinear(src, sx-0.5, sy-0.5)
			if sa == 0 && sr == 0 && sg == 0 && sb == 0 {
				di += 4
				continue
			}
			inv := 255 - sa
			p := dst.img.Pix
			p[di] = overPremul(sr, uint32(p[di]), inv)
			p[di+1] = overPremul(sg, uint32(p[di+1]), inv)
			p[di+2] = overPremul(sb, uint32(p[di+2]), inv)
			p[di+3] = overPremul(sa, uint32(p[di+3]), inv)
			di += 4
		}
	}
	dst.markDirty(rect)
}

// sampleBilinear reads a premultiplied sample at a continuous texel
// coordinate, treating everything outside the image as transparent.
func sampleBilinear(src *buffer, fx, fy float32) (r, g, b, a uint32) {
	x0 := int(floor32(fx))
	y0 := int(floor32(fy))
	tx := fx - float32(x0)
	ty := fy - float32(y0)
	var acc [4]float32
	texelInto(&acc, src, x0, y0, (1-tx)*(1-ty))
	texelInto(&acc, src, x0+1, y0, tx*(1-ty))
	texelInto(&acc, src, x0, y0+1, (1-tx)*ty)
	texelInto(&acc, src, x0+1, y0+1, tx*ty)
	return uint32(acc[0] + 0.5), uint32(acc[1] + 0.5),
		uint32(acc[2] + 0.5), uint32(acc[3] + 0.5)
}

// texelInto accumulates one weighted source texel; out-of-bounds reads
// contribute nothing, which fades a rotated layer's edge rather than
// smearing it.
func texelInto(acc *[4]float32, src *buffer, x, y int, w float32) {
	bnds := src.img.Bounds()
	if x < bnds.Min.X || y < bnds.Min.Y || x >= bnds.Max.X || y >= bnds.Max.Y {
		return
	}
	i := src.img.PixOffset(x, y)
	acc[0] += float32(src.img.Pix[i]) * w
	acc[1] += float32(src.img.Pix[i+1]) * w
	acc[2] += float32(src.img.Pix[i+2]) * w
	acc[3] += float32(src.img.Pix[i+3]) * w
}

// finishBrackets closes any bracket the stream left open, so a
// truncated or malformed command list still lands its content on the
// window instead of dropping it with the layer.
func (r *renderer) finishBrackets() {
	for len(r.brackets) > 0 {
		switch r.brackets[len(r.brackets)-1].kind {
		case bracketFilter:
			r.endFilter()
		case bracketStencil:
			r.endStencil()
		case bracketRotate:
			r.endRotation()
		}
	}
}
