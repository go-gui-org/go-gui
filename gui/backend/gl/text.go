//go:build !js

package gl

import (
	"unsafe"

	gogl "github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
)

// glyphBackend implements glyph.DrawBackend using OpenGL.
type glyphBackend struct {
	textures map[glyph.TextureID]glTexture
	nextID   glyph.TextureID
	dpiScale float32

	// Quad VAO/VBO for text rendering.
	vao, vbo uint32
}

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

	nc := normColor(c.R, c.G, c.B, c.A)

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
		{x0, y0, u0, v0, nc.r, nc.g, nc.b, nc.a},
		{x1, y0, u1, v0, nc.r, nc.g, nc.b, nc.a},
		{x1, y1, u1, v1, nc.r, nc.g, nc.b, nc.a},
		{x0, y1, u0, v1, nc.r, nc.g, nc.b, nc.a},
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
	nc := normColor(c.R, c.G, c.B, c.A)

	s := gb.dpiScale
	x0 := dst.X * s
	y0 := dst.Y * s
	x1 := (dst.X + dst.Width) * s
	y1 := (dst.Y + dst.Height) * s

	verts := [4][8]float32{
		{x0, y0, 0, 0, nc.r, nc.g, nc.b, nc.a},
		{x1, y0, 0, 0, nc.r, nc.g, nc.b, nc.a},
		{x1, y1, 0, 0, nc.r, nc.g, nc.b, nc.a},
		{x0, y1, 0, 0, nc.r, nc.g, nc.b, nc.a},
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

	nc := normColor(c.R, c.G, c.B, c.A)

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
			nc.r, nc.g, nc.b, nc.a,
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

func guiStyleToGlyphConfig(s gui.TextStyle) glyph.TextConfig {
	return glyphconv.GuiStyleToGlyphConfig(s)
}
