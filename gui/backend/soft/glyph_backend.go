package soft

import (
	"math"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
)

// softTexture is a glyph atlas page held in ordinary memory.
type softTexture struct {
	pix  []byte // straight RGBA, w*h*4
	w, h int
}

// glyphBackend implements glyph.DrawBackend over a CPU framebuffer.
//
// This is what makes headless text real rather than approximated: go-glyph
// is pure Go, so handing it a software draw backend gets shaping, metrics
// and glyph rasterization from the same code the GPU backends run. Every
// text render kind — plain text, pre-shaped layouts, transformed layouts,
// rich text, text on a path, text gradients — arrives through these three
// draw methods.
//
// buf is re-pointed per frame by the renderer; a nil buf makes every draw
// a no-op, which is what the atlas-warming pass wants.
type glyphBackend struct {
	buf      *buffer
	textures map[glyph.TextureID]*softTexture
	nextID   glyph.TextureID
	scale    float32
}

// maxSoftTexturePixels caps one atlas page. The dimensions arrive
// from glyph-controlled rasterization, so an unchecked width*height*4
// make is an OOM sink; 4096x4096 holds a full page with headroom.
const maxSoftTexturePixels = int64(4096 * 4096)

func newGlyphBackend(scale float32) *glyphBackend {
	return &glyphBackend{
		textures: make(map[glyph.TextureID]*softTexture),
		scale:    scale,
	}
}

func (gb *glyphBackend) NewTexture(width, height int) glyph.TextureID {
	if width <= 0 || height <= 0 {
		return 0
	}
	if int64(width)*int64(height) > maxSoftTexturePixels {
		return 0
	}
	gb.nextID++
	gb.textures[gb.nextID] = &softTexture{
		pix: make([]byte, width*height*4),
		w:   width,
		h:   height,
	}
	return gb.nextID
}

func (gb *glyphBackend) UpdateTexture(id glyph.TextureID, data []byte) {
	tex, ok := gb.textures[id]
	if !ok || len(data) == 0 {
		return
	}
	copy(tex.pix, data)
}

// UpdateTextureRect implements glyph.RectTextureUpdater: it copies the
// (x, y, w, h) region of a full-page source into the page.
//
// Being a RectTextureUpdater is what makes go-glyph upload new glyphs
// mid-frame, right after it resolves a layout and before it emits the quads
// that sample it (#1265). This backend samples the page at draw time, so
// without the mid-frame upload a glyph's first appearance draws blank until
// Commit — which is why RenderToImage runs an atlas warm pass, and why a live
// window can draw each frame in one pass.
//
// data is the whole page: source pixels start at y*srcStride + x*4 and rows
// advance by srcStride. Glyph promises an in-bounds, non-empty rect, but this
// is the last stop before a slice panic, so anything outside the texture or
// the source is ignored whole rather than clamped: a partial glyph is worse
// than none, and Commit's full-page UpdateTexture still covers the page.
func (gb *glyphBackend) UpdateTextureRect(id glyph.TextureID, data []byte,
	srcStride, x, y, w, h int) {

	tex, ok := gb.textures[id]
	// Each bound is written as a subtraction from a known-small value
	// (the texture size, the source length) so no sum of caller ints can
	// wrap negative and slip past: x+w > tex.w would, for x near MaxInt.
	if !ok || w <= 0 || h <= 0 || x < 0 || y < 0 ||
		w > tex.w-x || h > tex.h-y || srcStride < w*4 {
		return
	}
	// Last source byte read: the final row's end, (y+h-1)*srcStride +
	// (x+w)*4. x+w <= tex.w here, so rowEnd is small; the row count is
	// checked by division, so a huge stride cannot overflow the product.
	// Checked once up front so the loop below needs no per-row test.
	rowEnd := (x + w) * 4
	if rowEnd > len(data) || y+h-1 > (len(data)-rowEnd)/srcStride {
		return
	}
	rowBytes := w * 4
	dstStride := tex.w * 4
	for row := y; row < y+h; row++ {
		so := row*srcStride + x*4
		do := row*dstStride + x*4
		copy(tex.pix[do:do+rowBytes], data[so:so+rowBytes])
	}
}

func (gb *glyphBackend) DeleteTexture(id glyph.TextureID) {
	delete(gb.textures, id)
}

func (gb *glyphBackend) DPIScale() float32 { return gb.scale }

// DrawFilledRect paints an untextured rect — glyph uses it for
// decorations such as underlines and selection bands.
func (gb *glyphBackend) DrawFilledRect(dst glyph.Rect, c glyph.Color) {
	if gb.buf == nil || c.A == 0 {
		return
	}
	s := gb.scale
	x, y := dst.X*s, dst.Y*s
	w, h := dst.Width*s, dst.Height*s
	col := gui.RGBA(c.R, c.G, c.B, c.A)
	region := gb.buf.region(x, y, w, h)
	gb.buf.fillPath(region, gb.buf.solid(col),
		func(z *vector.Rasterizer, ox, oy float32) {
			pathRect(z, ox, oy, x, y, w, h)
		})
}

// DrawFilledRectTransformed implements glyph.TransformedFillBackend: the
// fill's corners go through t, so backgrounds and decorations rotate with
// their glyphs (#835). The quad is rasterized as a path, which antialiases
// its edges the way the GPU's MSAA does.
func (gb *glyphBackend) DrawFilledRectTransformed(dst glyph.Rect,
	c glyph.Color, t glyph.AffineTransform) {

	if gb.buf == nil || c.A == 0 {
		return
	}
	s := gb.scale
	// Device-space corners in winding order, and their bounding box to
	// size the rasterizer's region.
	var pts [4][2]float32
	corners := [4][2]float32{
		{dst.X, dst.Y},
		{dst.X + dst.Width, dst.Y},
		{dst.X + dst.Width, dst.Y + dst.Height},
		{dst.X, dst.Y + dst.Height},
	}
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := -float32(math.MaxFloat32), -float32(math.MaxFloat32)
	for i := range corners {
		cx, cy := t.Apply(corners[i][0], corners[i][1])
		pts[i] = [2]float32{cx * s, cy * s}
		minX, maxX = min(minX, pts[i][0]), max(maxX, pts[i][0])
		minY, maxY = min(minY, pts[i][1]), max(maxY, pts[i][1])
	}
	// The transform comes from the caller's LayoutTransform, so bound the
	// box like every other soft draw (validBox): NaN propagates through
	// min/max and Inf through the width, and either would reach
	// deviceRect's arch-specific float-to-int conversion.
	if !validBox(minX, minY, maxX-minX, maxY-minY) {
		return
	}
	region := gb.buf.region(minX, minY, maxX-minX, maxY-minY)
	col := gui.RGBA(c.R, c.G, c.B, c.A)
	gb.buf.fillPath(region, gb.buf.solid(col),
		func(z *vector.Rasterizer, ox, oy float32) {
			z.MoveTo(pts[0][0]-ox, pts[0][1]-oy)
			for i := 1; i < 4; i++ {
				z.LineTo(pts[i][0]-ox, pts[i][1]-oy)
			}
			z.ClosePath()
		})
}

// DrawTexturedQuad blits an axis-aligned atlas region, tinted by c.
//
// The atlas is rasterized at the device scale already, so the mapping is
// 1:1 at scale 1 and nearest sampling is exact; the GPU path blends with
// SRC_ALPHA/ONE_MINUS_SRC_ALPHA over a straight-alpha texture, which is
// what blendPixel reproduces.
func (gb *glyphBackend) DrawTexturedQuad(id glyph.TextureID,
	src, dst glyph.Rect, c glyph.Color) {

	tex, ok := gb.textures[id]
	if !ok || gb.buf == nil || c.A == 0 {
		return
	}
	s := gb.scale
	x0, y0 := dst.X*s, dst.Y*s
	x1, y1 := (dst.X+dst.Width)*s, (dst.Y+dst.Height)*s
	if x1 <= x0 || y1 <= y0 {
		return
	}
	region := gb.buf.region(x0, y0, x1-x0, y1-y0)
	if region.Empty() {
		return
	}
	gb.buf.markDirty(region)
	invW := 1 / (x1 - x0)
	invH := 1 / (y1 - y0)
	// The quad is axis-aligned, so a device row reads one atlas row. Both
	// row slices are taken once per row and blended in place: blendTexel
	// and blendPixel work the two offsets out again, and repeat the bounds
	// tests, for every pixel, which was most of a text-heavy frame (#1327).
	// The arithmetic is theirs, so the pixels are too.
	img := gb.buf.img
	rowLen := region.Dx() * 4
	tr, tg, tb, ta := uint32(c.R), uint32(c.G), uint32(c.B), uint32(c.A)
	for py := region.Min.Y; py < region.Max.Y; py++ {
		v := (float32(py) + 0.5 - y0) * invH
		sy := int(src.Y + v*src.Height)
		if sy < 0 || sy >= tex.h {
			continue
		}
		srcRow := tex.pix[sy*tex.w*4 : (sy+1)*tex.w*4]
		off := img.PixOffset(region.Min.X, py)
		dstRow := img.Pix[off : off+rowLen]
		for px := region.Min.X; px < region.Max.X; px++ {
			u := (float32(px) + 0.5 - x0) * invW
			sx := int(src.X + u*src.Width)
			if sx < 0 || sx >= tex.w {
				continue
			}
			p := srcRow[sx*4 : sx*4+4 : sx*4+4]
			sa := uint32(p[3]) * ta / 255
			if sa == 0 {
				continue
			}
			i := (px - region.Min.X) * 4
			d := dstRow[i : i+4 : i+4]
			inv := 255 - sa
			d[0] = uint8((uint32(p[0])*tr/255*sa + uint32(d[0])*inv + 127) / 255)
			d[1] = uint8((uint32(p[1])*tg/255*sa + uint32(d[1])*inv + 127) / 255)
			d[2] = uint8((uint32(p[2])*tb/255*sa + uint32(d[2])*inv + 127) / 255)
			d[3] = uint8((sa*255 + uint32(d[3])*inv + 127) / 255)
		}
	}
}

// DrawTexturedQuadTransformed blits an atlas region through an affine
// transform, inverse-mapping each covered device pixel back to a texel.
func (gb *glyphBackend) DrawTexturedQuadTransformed(id glyph.TextureID,
	src, dst glyph.Rect, c glyph.Color, t glyph.AffineTransform) {

	tex, ok := gb.textures[id]
	if !ok || gb.buf == nil || c.A == 0 ||
		dst.Width <= 0 || dst.Height <= 0 {
		return
	}
	det := t.XX*t.YY - t.XY*t.YX
	if det == 0 {
		return
	}
	s := gb.scale
	corners := [4][2]float32{
		{dst.X, dst.Y},
		{dst.X + dst.Width, dst.Y},
		{dst.X + dst.Width, dst.Y + dst.Height},
		{dst.X, dst.Y + dst.Height},
	}
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := -float32(math.MaxFloat32), -float32(math.MaxFloat32)
	for i := range corners {
		cx, cy := t.Apply(corners[i][0], corners[i][1])
		cx *= s
		cy *= s
		minX, maxX = min(minX, cx), max(maxX, cx)
		minY, maxY = min(minY, cy), max(maxY, cy)
	}
	region := gb.buf.region(minX, minY, maxX-minX, maxY-minY)
	if region.Empty() {
		return
	}
	gb.buf.markDirty(region)

	// Inverse of the affine, applied to logical coordinates.
	invDet := 1 / det
	ixx, ixy := t.YY*invDet, -t.XY*invDet
	iyx, iyy := -t.YX*invDet, t.XX*invDet
	invS := 1 / s

	for py := region.Min.Y; py < region.Max.Y; py++ {
		dy := (float32(py) + 0.5) * invS
		for px := region.Min.X; px < region.Max.X; px++ {
			dx := (float32(px) + 0.5) * invS
			ox, oy := dx-t.X0, dy-t.Y0
			lx := ixx*ox + ixy*oy
			ly := iyx*ox + iyy*oy
			u := (lx - dst.X) / dst.Width
			v := (ly - dst.Y) / dst.Height
			if u < 0 || u >= 1 || v < 0 || v >= 1 {
				continue
			}
			sx := int(src.X + u*src.Width)
			sy := int(src.Y + v*src.Height)
			gb.blendTexel(tex, sx, sy, px, py, c)
		}
	}
}

// blendTexel composites one tinted atlas texel onto a device pixel.
func (gb *glyphBackend) blendTexel(tex *softTexture, sx, sy, px, py int,
	c glyph.Color) {

	if sx < 0 || sy < 0 || sx >= tex.w || sy >= tex.h {
		return
	}
	i := (sy*tex.w + sx) * 4
	ta := uint32(tex.pix[i+3])
	if ta == 0 {
		return
	}
	// frag = texel * tint, both straight (the GPU path's
	// FsFilterTextureGLSL multiplies, then blends with SRC_ALPHA).
	sr := uint32(tex.pix[i]) * uint32(c.R) / 255
	sg := uint32(tex.pix[i+1]) * uint32(c.G) / 255
	sb := uint32(tex.pix[i+2]) * uint32(c.B) / 255
	sa := ta * uint32(c.A) / 255
	gb.buf.blendPixel(px, py, sr, sg, sb, sa)
}
