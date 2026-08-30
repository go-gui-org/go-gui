//go:build !js && !darwin && !android

package gl

import (
	"unsafe"

	"github.com/go-gui-org/go-glyph"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
)

// glyphBackend implements glyph.DrawBackend using OpenGL.
type glyphBackend struct {
	textures map[glyph.TextureID]glTexture
	nextID   glyph.TextureID
	dpiScale float32

	// Quad VAO/VBO for text rendering.
	vao, vbo uint32
}

// Asserted, not merely implemented: RectTextureUpdater is an optional
// interface, so a drifted signature would silently drop this backend back
// to end-of-frame uploads and reintroduce the one-frame-blank-text bug
// with nothing failing to compile.
var _ glyph.RectTextureUpdater = (*glyphBackend)(nil)

func newGlyphBackend(dpiScale float32) *glyphBackend {
	gb := &glyphBackend{
		textures: make(map[glyph.TextureID]glTexture),
		dpiScale: dpiScale,
	}
	gogl.GenVertexArrays(1, &gb.vao)
	gogl.GenBuffers(1, &gb.vbo)

	gogl.BindVertexArray(gb.vao)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, gb.vbo)
	// 4 verts * 8 floats (pos2 + uv2 + color4) * 4 bytes
	gogl.BufferData(gogl.ARRAY_BUFFER, 4*8*4,
		nil, gogl.DYNAMIC_DRAW)

	// Position (vec2) at location 0
	gogl.EnableVertexAttribArray(0)
	gogl.VertexAttribPointerWithOffset(0, 2, gogl.FLOAT, false,
		8*4, 0)
	// TexCoord (vec2) at location 1
	gogl.EnableVertexAttribArray(1)
	gogl.VertexAttribPointerWithOffset(1, 2, gogl.FLOAT, false,
		8*4, 2*4)
	// Color (vec4) at location 2
	gogl.EnableVertexAttribArray(2)
	gogl.VertexAttribPointerWithOffset(2, 4, gogl.FLOAT, false,
		8*4, 4*4)

	gogl.BindVertexArray(0)
	return gb
}

func (gb *glyphBackend) destroy() {
	for _, tex := range gb.textures {
		gogl.DeleteTextures(1, &tex.id)
	}
	gb.textures = nil
	if gb.vao != 0 {
		gogl.DeleteVertexArrays(1, &gb.vao)
	}
	if gb.vbo != 0 {
		gogl.DeleteBuffers(1, &gb.vbo)
	}
}

func (gb *glyphBackend) NewTexture(width, height int) glyph.TextureID {
	gb.nextID++
	id := gb.nextID
	tex := createTexture(int32(width), int32(height), nil)
	gb.textures[id] = tex
	return id
}

func (gb *glyphBackend) UpdateTexture(id glyph.TextureID, data []byte) {
	tex, ok := gb.textures[id]
	if !ok {
		return
	}
	if len(data) == 0 {
		return
	}
	gogl.BindTexture(gogl.TEXTURE_2D, tex.id)
	gogl.TexSubImage2D(gogl.TEXTURE_2D, 0, 0, 0,
		tex.w, tex.h, gogl.RGBA, gogl.UNSIGNED_BYTE,
		unsafe.Pointer(&data[0]))
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
}

// UpdateTextureRect implements glyph.RectTextureUpdater, uploading only
// the rows glyph just rasterized into.
//
// This is what lets go-glyph push new glyphs to the GPU mid-frame, before
// it emits the quads that sample them. GL draw calls read a texture at
// their position in the command stream, so without a mid-frame upload a
// glyph's first appearance renders blank until the following frame — and
// this backend only draws a frame when something asks it to, so "the
// following frame" can be whenever the user next moves the mouse.
//
// The upload is widened to whole rows and the x/w arguments ignored:
// full rows are contiguous in the page buffer, so no unpack row length
// has to be set (glbind exposes no glPixelStorei, and GLES2 lacks
// GL_UNPACK_ROW_LENGTH entirely). A glyph-tall band of a 1024-wide page
// is tens of KB against the 4 MiB the whole page would cost, so the
// widening does not undo the point of the exercise.
func (gb *glyphBackend) UpdateTextureRect(id glyph.TextureID, data []byte,
	srcStride, _, y, _, h int) {

	tex, ok := gb.textures[id]
	if !ok || h <= 0 || srcStride <= 0 {
		return
	}
	// Whole-row uploads hold only while a source row is exactly a texture
	// row: TexSubImage2D advances the read pointer by width*4 per row
	// (unpack row length is unsettable here — see above), so a padded
	// stride would shear the page diagonally instead of merely offsetting
	// it. Unreachable as glyph is written — a page sizes its own texture —
	// so this trades a corrupt atlas for a blank one if that ever changes.
	if srcStride != int(tex.w)*4 {
		return
	}
	// Clamp to the texture: glyph promises an in-bounds rect, but a
	// mismatch here is an out-of-range driver read, not a wrong pixel.
	if y < 0 {
		y = 0
	}
	if y+h > int(tex.h) {
		h = int(tex.h) - y
	}
	off := y * srcStride
	if h <= 0 || off+h*srcStride > len(data) {
		return
	}

	gogl.BindTexture(gogl.TEXTURE_2D, tex.id)
	gogl.TexSubImage2D(gogl.TEXTURE_2D, 0, 0, int32(y),
		tex.w, int32(h), gogl.RGBA, gogl.UNSIGNED_BYTE,
		unsafe.Pointer(&data[off]))
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
}

func (gb *glyphBackend) DeleteTexture(id glyph.TextureID) {
	tex, ok := gb.textures[id]
	if !ok {
		return
	}
	gogl.DeleteTextures(1, &tex.id)
	delete(gb.textures, id)
}

func (gb *glyphBackend) DrawTexturedQuad(id glyph.TextureID,
	src, dst glyph.Rect, c glyph.Color) {

	tex, ok := gb.textures[id]
	if !ok {
		return
	}

	cr, cg, cb, ca := gpu.NormColor(c.R, c.G, c.B, c.A)

	// UV from source rect (pixel coords → 0..1).
	tw := float32(tex.w)
	th := float32(tex.h)
	u0 := src.X / tw
	v0 := src.Y / th
	u1 := (src.X + src.Width) / tw
	v1 := (src.Y + src.Height) / th

	// Glyph passes logical coordinates; scale to physical pixels.
	s := gb.dpiScale
	x0 := dst.X * s
	y0 := dst.Y * s
	x1 := (dst.X + dst.Width) * s
	y1 := (dst.Y + dst.Height) * s

	verts := [4][8]float32{
		{x0, y0, u0, v0, cr, cg, cb, ca},
		{x1, y0, u1, v0, cr, cg, cb, ca},
		{x1, y1, u1, v1, cr, cg, cb, ca},
		{x0, y1, u0, v1, cr, cg, cb, ca},
	}

	gogl.ActiveTexture(gogl.TEXTURE0)
	gogl.BindTexture(gogl.TEXTURE_2D, tex.id)

	gogl.BindVertexArray(gb.vao)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, gb.vbo)
	gogl.BufferSubData(gogl.ARRAY_BUFFER, 0,
		int(unsafe.Sizeof(verts)), unsafe.Pointer(&verts[0]))
	gogl.DrawArrays(gogl.TRIANGLE_FAN, 0, 4)
	gogl.BindVertexArray(0)
}

func (gb *glyphBackend) DrawFilledRect(dst glyph.Rect, c glyph.Color) {
	cr, cg, cb, ca := gpu.NormColor(c.R, c.G, c.B, c.A)

	s := gb.dpiScale
	x0 := dst.X * s
	y0 := dst.Y * s
	x1 := (dst.X + dst.Width) * s
	y1 := (dst.Y + dst.Height) * s

	verts := [4][8]float32{
		{x0, y0, 0, 0, cr, cg, cb, ca},
		{x1, y0, 0, 0, cr, cg, cb, ca},
		{x1, y1, 0, 0, cr, cg, cb, ca},
		{x0, y1, 0, 0, cr, cg, cb, ca},
	}

	gogl.BindVertexArray(gb.vao)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, gb.vbo)
	gogl.BufferSubData(gogl.ARRAY_BUFFER, 0,
		int(unsafe.Sizeof(verts)), unsafe.Pointer(&verts[0]))
	gogl.DrawArrays(gogl.TRIANGLE_FAN, 0, 4)
	gogl.BindVertexArray(0)
}

func (gb *glyphBackend) DrawTexturedQuadTransformed(
	id glyph.TextureID, src, dst glyph.Rect,
	c glyph.Color, t glyph.AffineTransform) {

	tex, ok := gb.textures[id]
	if !ok {
		return
	}

	cr, cg, cb, ca := gpu.NormColor(c.R, c.G, c.B, c.A)

	tw := float32(tex.w)
	th := float32(tex.h)
	u0 := src.X / tw
	v0 := src.Y / th
	u1 := (src.X + src.Width) / tw
	v1 := (src.Y + src.Height) / th

	// Apply affine transform to corner positions.
	corners := [4][2]float32{
		{dst.X, dst.Y},
		{dst.X + dst.Width, dst.Y},
		{dst.X + dst.Width, dst.Y + dst.Height},
		{dst.X, dst.Y + dst.Height},
	}
	uvs := [4][2]float32{
		{u0, v0}, {u1, v0}, {u1, v1}, {u0, v1},
	}

	// Apply affine transform then scale to physical pixels.
	s := gb.dpiScale
	var verts [4][8]float32
	for i := range 4 {
		px := corners[i][0]
		py := corners[i][1]
		tx := (t.XX*px + t.XY*py + t.X0) * s
		ty := (t.YX*px + t.YY*py + t.Y0) * s
		verts[i] = [8]float32{
			tx, ty, uvs[i][0], uvs[i][1],
			cr, cg, cb, ca,
		}
	}

	gogl.ActiveTexture(gogl.TEXTURE0)
	gogl.BindTexture(gogl.TEXTURE_2D, tex.id)

	gogl.BindVertexArray(gb.vao)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, gb.vbo)
	gogl.BufferSubData(gogl.ARRAY_BUFFER, 0,
		int(unsafe.Sizeof(verts)), unsafe.Pointer(&verts[0]))
	gogl.DrawArrays(gogl.TRIANGLE_FAN, 0, 4)
	gogl.BindVertexArray(0)
}

func (gb *glyphBackend) DPIScale() float32 {
	return gb.dpiScale
}

// --- TextMeasurer ---

// textMeasurer wraps glyph.TextSystem to implement gui.TextMeasurer.
type textMeasurer struct {
	textSys *glyph.TextSystem
}

func (tm *textMeasurer) TextWidth(text string, style gui.TextStyle) float32 {
	cfg := guiStyleToGlyphConfig(style)
	w, err := tm.textSys.TextWidth(text, cfg)
	if err != nil {
		return 0
	}
	return w
}

func (tm *textMeasurer) TextHeight(text string, style gui.TextStyle) float32 {
	cfg := guiStyleToGlyphConfig(style)
	h, err := tm.textSys.TextHeight(text, cfg)
	if err != nil {
		return 0
	}
	return h
}

func (tm *textMeasurer) FontHeight(style gui.TextStyle) float32 {
	cfg := guiStyleToGlyphConfig(style)
	h, err := tm.textSys.FontHeight(cfg)
	if err != nil {
		return style.Size * 1.4
	}
	return h
}

func (tm *textMeasurer) FontAscent(style gui.TextStyle) float32 {
	cfg := guiStyleToGlyphConfig(style)
	m, err := tm.textSys.FontMetrics(cfg)
	if err != nil {
		return style.Size * 0.8
	}
	return m.Ascender
}

// TextInkBounds returns the painted box of text, relative to the
// top-left of its advance box. Backs gui's optional ink-measuring
// capability, which widgets use to centre a single glyph on its ink
// instead of on the font's advance box.
func (tm *textMeasurer) TextInkBounds(
	text string, style gui.TextStyle) (gui.InkBounds, bool) {
	cfg := guiStyleToGlyphConfig(style)
	r, ok := tm.textSys.InkBounds(text, cfg)
	if !ok {
		return gui.InkBounds{}, false
	}
	return gui.InkBounds{
		X: r.X, Y: r.Y, Width: r.Width, Height: r.Height,
	}, true
}

func (tm *textMeasurer) LayoutText(
	text string, style gui.TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	cfg := guiStyleToGlyphConfig(style)
	if wrapWidth > 0 {
		cfg.Block.Width = wrapWidth
		cfg.Block.Wrap = glyph.WrapWord
	} else if wrapWidth < 0 {
		cfg.Block.Width = -wrapWidth
		cfg.Block.Wrap = glyph.WrapNone
	}
	return tm.textSys.LayoutText(text, cfg)
}

func (tm *textMeasurer) LayoutRichText(
	rt glyph.RichText, cfg glyph.TextConfig,
) (glyph.Layout, error) {
	return tm.textSys.LayoutRichText(rt, cfg)
}

func (tm *textMeasurer) ListFontFamilies() []string {
	return tm.textSys.ListFontFamilies()
}

func guiStyleToGlyphConfig(s gui.TextStyle) glyph.TextConfig {
	return glyphconv.GuiStyleToGlyphConfig(s)
}
