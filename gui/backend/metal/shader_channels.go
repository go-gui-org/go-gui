//go:build darwin && !ios

package metal

/*
#include "metal_darwin.h"
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// metalChannels holds a surface's planes as the textures a custom shader
// reads as iChannel0..3 (gui.Shader.Surface). The textures are streamed:
// an upload first waits for a submitted frame still sampling them.
type metalChannels struct {
	tex [4]metalTexture
	fmt [4]gui.PixelFormat
	n   int
}

// uploadChannels puts a frame's planes into c, reusing each texture while
// its plane keeps its format and size.
func (b *windowState) uploadChannels(c metalChannels, planes []gui.SurfacePlane) metalChannels {
	for i, p := range planes {
		t := c.tex[i]
		w, h := int32(p.Width), int32(p.Height)
		bpp := C.int(p.Format.BytesPerPixel())
		ptr := unsafe.Pointer(&p.Pix[0])
		if t.id == 0 || t.w != w || t.h != h || c.fmt[i] != p.Format {
			b.deleteSurfaceTexture(t)
			id := C.metalCreatePlaneTexture(b.ctx, C.int(p.Format), C.int(w), C.int(h), bpp, ptr)
			c.tex[i] = metalTexture{id: int32(id), w: w, h: h}
			c.fmt[i] = p.Format
			continue
		}
		C.metalUpdateStreamTexture(b.ctx, C.int(t.id), C.int(w), C.int(h), bpp, ptr)
	}
	for i := len(planes); i < c.n; i++ {
		b.deleteSurfaceTexture(c.tex[i])
		c.tex[i] = metalTexture{}
	}
	c.n = len(planes)
	return c
}

// deleteChannels releases a surface's plane textures.
func (b *windowState) deleteChannels(c metalChannels) {
	for i := range c.n {
		b.deleteSurfaceTexture(c.tex[i])
	}
}

// bindChannels binds plane i at texture(i+1), and the transparent texture
// for every channel without a plane (or all of them when ok is false: a
// closed surface).
func (b *windowState) bindChannels(c metalChannels, ok bool) {
	for i := range 4 {
		var id int32
		if ok && i < c.n {
			id = c.tex[i].id
		}
		C.metalBindChannel(b.ctx, C.int(i), C.int(id))
	}
}
