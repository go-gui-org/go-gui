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

	// VAO/VBO/IBO for text rendering. The VBO holds up to
	// maxGlyphQuads quads; the IBO is a static two-triangles-per-quad
	// index list over all of them.
	vao, vbo, ibo uint32

	// Quad batch (#816). Glyph emits one DrawTexturedQuad per glyph;
	// drawing each at once cost 7 GL calls a glyph, 4 of them re-binding
	// state already bound. Quads now queue here and draw with one
	// DrawElements per run from the same atlas page.
	//
	// A batch never outlives the text command that filled it:
	// Backend.restoreAfterGlyph flushes it before the next RenderCmd, so
	// scissor, stencil, blend, framebuffer and MVP are constant for every
	// quad in it, and GL draws a single call's primitives in order. That
	// is what keeps overlap and clipping identical to the unbatched path.
	//
	// Other flush triggers, each needed for correctness:
	//   - a quad from a different texture (one draw samples one page);
	//   - the batch filling up (flushed at once, so batchCap 1 is exactly
	//     the old draw-per-quad sequence, which the tests compare against);
	//   - an upload to, or deletion of, the queued page. go-glyph evicts
	//     and re-uploads a page mid-command (atlas.go resetPage); queued
	//     quads must draw from the texels they were laid out against.
	//   - DrawFilledRect, which draws immediately and must stay behind
	//     the glyphs queued before it.
	//
	// The array lives inside the backend, allocated once with it, so
	// queueing is a copy with no per-frame heap allocation.
	batch    [maxGlyphQuads][4][8]float32
	batchN   int    // queued quads
	batchTex uint32 // GL texture the queued quads sample
	batchCap int    // flush threshold, maxGlyphQuads; tests lower it to 1
	flushes  int    // draws issued by flush; read by tests as a call count
}

// maxGlyphQuads bounds one batch. 1024 quads (a dense screen of text is a
// few thousand glyphs) keep the VBO at 128 KiB and the quad indices
// (4*maxGlyphQuads-1) inside uint16 range.
const maxGlyphQuads = 1024

// glyphQuadBytes is one quad: 4 verts * 8 floats (pos2 + uv2 + color4)
// * 4 bytes.
const glyphQuadBytes = 4 * 8 * 4

// Asserted, not merely implemented: RectTextureUpdater is an optional
// interface, so a drifted signature would silently drop this backend back
// to end-of-frame uploads and reintroduce the one-frame-blank-text bug
// with nothing failing to compile.
var _ glyph.RectTextureUpdater = (*glyphBackend)(nil)

func newGlyphBackend(dpiScale float32) *glyphBackend {
	gb := &glyphBackend{
		textures: make(map[glyph.TextureID]glTexture),
		dpiScale: dpiScale,
		batchCap: maxGlyphQuads,
	}
	gogl.GenVertexArrays(1, &gb.vao)
	gogl.GenBuffers(1, &gb.vbo)
	gogl.GenBuffers(1, &gb.ibo)

	gogl.BindVertexArray(gb.vao)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, gb.vbo)
	gogl.BufferData(gogl.ARRAY_BUFFER, maxGlyphQuads*glyphQuadBytes,
		nil, gogl.DYNAMIC_DRAW)

	// Quad i's corners are verts 4i..4i+3 in fan order (TL, TR, BR,
	// BL), so the triangles are (0,1,2) and (0,2,3) offset by 4i — the
	// same split TRIANGLE_FAN made of a single quad. The element buffer
	// binding is VAO state, so it is bound while the VAO is.
	indices := make([]uint16, maxGlyphQuads*6)
	for i := range maxGlyphQuads {
		v := uint16(i * 4)
		copy(indices[i*6:], []uint16{v, v + 1, v + 2, v, v + 2, v + 3})
	}
	gogl.BindBuffer(gogl.ELEMENT_ARRAY_BUFFER, gb.ibo)
	gogl.BufferData(gogl.ELEMENT_ARRAY_BUFFER, len(indices)*2,
		unsafe.Pointer(&indices[0]), gogl.STATIC_DRAW)

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
	if gb.ibo != 0 {
		gogl.DeleteBuffers(1, &gb.ibo)
	}
}

// queue adds one quad sampling GL texture tex, flushing first when the
// batch holds another texture's quads, and after when it is full.
func (gb *glyphBackend) queue(tex uint32, verts *[4][8]float32) {
	if gb.batchN > 0 && gb.batchTex != tex {
		gb.flush()
	}
	gb.batchTex = tex
	gb.batch[gb.batchN] = *verts
	gb.batchN++
	if gb.batchN >= gb.batchCap {
		gb.flush()
	}
}

// flush draws the queued quads with one DrawElements and empties the
// batch. No-op when empty, so callers flush unconditionally.
func (gb *glyphBackend) flush() {
	n := gb.batchN
	if n == 0 {
		return
	}
	gb.batchN = 0

	gogl.ActiveTexture(gogl.TEXTURE0)
	gogl.BindTexture(gogl.TEXTURE_2D, gb.batchTex)

	gogl.BindVertexArray(gb.vao)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, gb.vbo)
	gogl.BufferSubData(gogl.ARRAY_BUFFER, 0,
		n*glyphQuadBytes, unsafe.Pointer(&gb.batch[0]))
	gogl.DrawElements(gogl.TRIANGLES, int32(n*6), gogl.UNSIGNED_SHORT, nil)
	gogl.BindVertexArray(0)
	gb.flushes++
}

// flushIfSampling flushes when the queued quads sample GL texture tex,
// ahead of a call that changes or frees it. Other textures leave the batch
// queued: changing them cannot change what the queued quads sample.
func (gb *glyphBackend) flushIfSampling(tex uint32) {
	if gb.batchN > 0 && gb.batchTex == tex {
		gb.flush()
	}
}

// discard drops queued quads without drawing them. For a frame cut short,
// where the state they were queued under is already gone.
func (gb *glyphBackend) discard() {
	gb.batchN = 0
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
	gb.flushIfSampling(tex.id)
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

	gb.flushIfSampling(tex.id)
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
	gb.flushIfSampling(tex.id)
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

	gb.queue(tex.id, &verts)
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

	// Drawn at once, not queued: it binds no texture of its own (#835),
	// so it cannot join a batch. Queued glyphs go first to keep order.
	gb.flush()
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

	gb.queue(tex.id, &verts)
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
