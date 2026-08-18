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

func newGlyphBackend(scale float32) *glyphBackend {
	return &glyphBackend{
		textures: make(map[glyph.TextureID]*softTexture),
		scale:    scale,
	}
}

func (gb *glyphBackend) NewTexture(width, height int) glyph.TextureID {
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
	for py := region.Min.Y; py < region.Max.Y; py++ {
		v := (float32(py) + 0.5 - y0) * invH
		sy := int(src.Y + v*src.Height)
		for px := region.Min.X; px < region.Max.X; px++ {
			u := (float32(px) + 0.5 - x0) * invW
			sx := int(src.X + u*src.Width)
			gb.blendTexel(tex, sx, sy, px, py, c)
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
