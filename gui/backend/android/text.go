//go:build android

package android

/*
#include "gles_android.h"
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
)

// glesGlyphBackend implements glyph.DrawBackend using GLES3.
type glesGlyphBackend struct {
	textures map[glyph.TextureID]glesTexInfo
	nextID   glyph.TextureID
	dpiScale float32
}

type glesTexInfo struct {
	cID  int32 // C-side texture ID
	w, h int32
}

func newGLESGlyphBackend(dpiScale float32) *glesGlyphBackend {
	return &glesGlyphBackend{
		textures: make(map[glyph.TextureID]glesTexInfo),
		dpiScale: dpiScale,
	}
}

func (gb *glesGlyphBackend) destroy() {
	for _, t := range gb.textures {
		C.glesDeleteTexture(C.int(t.cID))
	}
	gb.textures = nil
}

func (gb *glesGlyphBackend) NewTexture(
	width, height int) glyph.TextureID {
	gb.nextID++
	id := gb.nextID

	cID := C.glesCreateTexture(C.int(width), C.int(height),
		nil, C.int(0))
	gb.textures[id] = glesTexInfo{
		cID: int32(cID), w: int32(width), h: int32(height),
	}
	return id
}

func (gb *glesGlyphBackend) UpdateTexture(
	id glyph.TextureID, data []byte) {
	t, ok := gb.textures[id]
	if !ok || len(data) == 0 {
		return
	}
	C.glesUpdateTexture(C.int(t.cID), 0, 0,
		C.int(t.w), C.int(t.h),
		unsafe.Pointer(&data[0]))
}

func (gb *glesGlyphBackend) DeleteTexture(
	id glyph.TextureID) {
	t, ok := gb.textures[id]
	if !ok {
		return
	}
	C.glesDeleteTexture(C.int(t.cID))
	delete(gb.textures, id)
}

func (gb *glesGlyphBackend) DrawTexturedQuad(
	id glyph.TextureID,
	src, dst glyph.Rect, c glyph.Color) {

	t, ok := gb.textures[id]
	if !ok {
		return
	}
	nc := normColor(c.R, c.G, c.B, c.A)

	tw := float32(t.w)
	th := float32(t.h)
	u0 := src.X / tw
	v0 := src.Y / th
	u1 := (src.X + src.Width) / tw
	v1 := (src.Y + src.Height) / th

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

	C.glesSetPipeline(C.int(pipeGlyphTex))
	C.glesBindTexture(C.int(t.cID))
	C.glesDrawGlyphQuad(
		(*C.float)(unsafe.Pointer(&verts[0])))
}

func (gb *glesGlyphBackend) DrawFilledRect(
	dst glyph.Rect, c glyph.Color) {
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

	C.glesSetPipeline(C.int(pipeGlyphColor))
	C.glesDrawGlyphQuad(
		(*C.float)(unsafe.Pointer(&verts[0])))
}

func (gb *glesGlyphBackend) DrawTexturedQuadTransformed(
	id glyph.TextureID, src, dst glyph.Rect,
	c glyph.Color, tr glyph.AffineTransform) {

	t, ok := gb.textures[id]
	if !ok {
		return
	}
	nc := normColor(c.R, c.G, c.B, c.A)

	tw := float32(t.w)
	th := float32(t.h)
	u0 := src.X / tw
	v0 := src.Y / th
	u1 := (src.X + src.Width) / tw
	v1 := (src.Y + src.Height) / th

	corners := [4][2]float32{
		{dst.X, dst.Y},
		{dst.X + dst.Width, dst.Y},
		{dst.X + dst.Width, dst.Y + dst.Height},
		{dst.X, dst.Y + dst.Height},
	}
	uvs := [4][2]float32{
		{u0, v0}, {u1, v0}, {u1, v1}, {u0, v1},
	}

	s := gb.dpiScale
	var verts [4][8]float32
	for i := range 4 {
		px := corners[i][0]
		py := corners[i][1]
		tx := (tr.XX*px + tr.XY*py + tr.X0) * s
		ty := (tr.YX*px + tr.YY*py + tr.Y0) * s
		verts[i] = [8]float32{
			tx, ty, uvs[i][0], uvs[i][1],
			nc.r, nc.g, nc.b, nc.a,
		}
	}

	C.glesSetPipeline(C.int(pipeGlyphTex))
	C.glesBindTexture(C.int(t.cID))
	C.glesDrawGlyphQuad(
		(*C.float)(unsafe.Pointer(&verts[0])))
}

func (gb *glesGlyphBackend) DPIScale() float32 {
	return gb.dpiScale
}

// --- TextMeasurer ---

type textMeasurer struct {
	textSys *glyph.TextSystem
}

func (tm *textMeasurer) TextWidth(
	text string, style gui.TextStyle) float32 {
	cfg := guiStyleToGlyphConfig(style)
	w, err := tm.textSys.TextWidth(text, cfg)
	if err != nil {
		return 0
	}
	return w
}

func (tm *textMeasurer) TextHeight(
	text string, style gui.TextStyle) float32 {
	cfg := guiStyleToGlyphConfig(style)
	h, err := tm.textSys.TextHeight(text, cfg)
	if err != nil {
		return 0
	}
	return h
}

func (tm *textMeasurer) FontHeight(
	style gui.TextStyle) float32 {
	cfg := guiStyleToGlyphConfig(style)
	h, err := tm.textSys.FontHeight(cfg)
	if err != nil {
		return style.Size * 1.4
	}
	return h
}

func (tm *textMeasurer) FontAscent(
	style gui.TextStyle) float32 {
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
