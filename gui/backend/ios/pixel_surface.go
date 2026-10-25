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
// a fixed size and recreating it on a resize.
func (b *Backend) uploadSurfaceTexture(tex metalTexture, w, h int, pix []byte) metalTexture {
	if tex.id == 0 || tex.w != int32(w) || tex.h != int32(h) {
		b.deleteSurfaceTexture(tex)
		return createMetalTexture(int32(w), int32(h), pix)
	}
	C.metalUpdateTexture(C.int(tex.id), 0, 0,
		C.int(w), C.int(h), unsafe.Pointer(&pix[0]))
	return tex
}
