//go:build ios

package ios

/*
#include "metal_darwin.h"
*/
import "C"

import "unsafe"

func (b *Backend) deleteSurfaceTexture(tex metalTexture) {
	if tex.id != 0 {
		C.metalDeleteTexture(C.int(tex.id))
	}
}

// uploadSurfaceTexture puts a surface frame into tex, reusing its storage at
// a fixed size and recreating it on a resize. The texture is streamed: an
// earlier frame may still be sampling it on the GPU, so the update waits for
// that frame first (metalUpdateStreamTexture).
func (b *Backend) uploadSurfaceTexture(tex metalTexture, w, h int, pix []byte) metalTexture {
	if tex.id == 0 || tex.w != int32(w) || tex.h != int32(h) {
		b.deleteSurfaceTexture(tex)
		tex = createMetalTexture(int32(w), int32(h), pix)
		C.metalStreamTexture(C.int(tex.id))
		return tex
	}
	C.metalUpdateStreamTexture(C.int(tex.id),
		C.int(w), C.int(h), 4, unsafe.Pointer(&pix[0]))
	return tex
}
