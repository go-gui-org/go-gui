//go:build darwin && !ios

package metal

/*
#include "metal_darwin.h"
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
)

// metalGlyphBackend implements glyph.DrawBackend using Metal.
type metalGlyphBackend struct {
	ctx      C.MetalCtx
	textures map[glyph.TextureID]metalTexInfo
	nextID   glyph.TextureID
	dpiScale float32
}

type metalTexInfo struct {
	cID  int32 // C-side texture ID
	w, h int32
}

func newMetalGlyphBackend(ctx C.MetalCtx,
	dpiScale float32) *metalGlyphBackend {
	return &metalGlyphBackend{
		ctx:      ctx,
		textures: make(map[glyph.TextureID]metalTexInfo),
		dpiScale: dpiScale,
	}
}

func (gb *metalGlyphBackend) destroy() {
	for _, t := range gb.textures {
		C.metalDeleteTexture(gb.ctx, C.int(t.cID))
	}
	gb.textures = nil
}

func (gb *metalGlyphBackend) NewTexture(
	width, height int) glyph.TextureID {
	gb.nextID++
	id := gb.nextID

	cID := C.metalCreateTexture(gb.ctx, C.int(width),
		C.int(height), nil, C.int(0))
	gb.textures[id] = metalTexInfo{
		cID: int32(cID), w: int32(width), h: int32(height),
	}
	return id
}

func (gb *metalGlyphBackend) UpdateTexture(
	id glyph.TextureID, data []byte) {
	t, ok := gb.textures[id]
	if !ok || len(data) == 0 {
		return
	}
	C.metalUpdateTexture(gb.ctx, C.int(t.cID), 0, 0,
		C.int(t.w), C.int(t.h),
		unsafe.Pointer(&data[0]))
}

func (gb *metalGlyphBackend) DeleteTexture(
	id glyph.TextureID) {
	t, ok := gb.textures[id]
	if !ok {
		return
	}
	C.metalDeleteTexture(gb.ctx, C.int(t.cID))
	delete(gb.textures, id)
}

func (gb *metalGlyphBackend) DrawTexturedQuad(
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

	C.metalSetPipeline(gb.ctx, C.int(pipeGlyphTex))
	C.metalBindTexture(gb.ctx, C.int(t.cID))
	C.metalDrawGlyphQuad(gb.ctx,
		(*C.float)(unsafe.Pointer(&verts[0])))
}

func (gb *metalGlyphBackend) DrawFilledRect(
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

	C.metalSetPipeline(gb.ctx, C.int(pipeGlyphColor))
	C.metalDrawGlyphQuad(gb.ctx,
		(*C.float)(unsafe.Pointer(&verts[0])))
}

func (gb *metalGlyphBackend) DrawTexturedQuadTransformed(
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

	C.metalSetPipeline(gb.ctx, C.int(pipeGlyphTex))
	C.metalBindTexture(gb.ctx, C.int(t.cID))
	C.metalDrawGlyphQuad(gb.ctx,
		(*C.float)(unsafe.Pointer(&verts[0])))
}

func (gb *metalGlyphBackend) DPIScale() float32 {
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
