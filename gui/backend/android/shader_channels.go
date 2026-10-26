//go:build android

package android

/*
#include "gles_android.h"
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// glesChannels holds a surface's planes as the textures a custom shader
// reads as iChannel0..3 (gui.Shader.Surface).
type glesChannels struct {
	tex [4]glesTexture
	fmt [4]gui.PixelFormat
	n   int
}

// uploadChannels puts a frame's planes into c, reusing each texture while
// its plane keeps its format and size.
func (b *Backend) uploadChannels(c glesChannels, planes []gui.SurfacePlane) glesChannels {
	for i, p := range planes {
		t := c.tex[i]
		w, h := int32(p.Width), int32(p.Height)
		ptr := unsafe.Pointer(&p.Pix[0])
		if t.id == 0 || t.w != w || t.h != h || c.fmt[i] != p.Format {
			b.deleteSurfaceTexture(t)
			id := C.glesCreatePlaneTexture(C.int(p.Format), C.int(w), C.int(h), ptr)
			c.tex[i] = glesTexture{id: int32(id), w: w, h: h}
			c.fmt[i] = p.Format
			continue
		}
		C.glesUpdatePlaneTexture(C.int(t.id), C.int(p.Format), C.int(w), C.int(h), ptr)
	}
	for i := len(planes); i < c.n; i++ {
		b.deleteSurfaceTexture(c.tex[i])
		c.tex[i] = glesTexture{}
	}
	c.n = len(planes)
	return c
}

// deleteChannels releases a surface's plane textures.
func (b *Backend) deleteChannels(c glesChannels) {
	for i := range c.n {
		b.deleteSurfaceTexture(c.tex[i])
	}
}

// bindChannels binds plane i to texture unit i+1, and the transparent
// texture to every unit without a plane (or all of them when ok is false:
// a closed surface).
func (b *Backend) bindChannels(c glesChannels, ok bool) {
	for i := range 4 {
		var id int32
		if ok && i < c.n {
			id = c.tex[i].id
		}
		C.glesBindChannel(C.int(i), C.int(id))
	}
}
