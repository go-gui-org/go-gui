//go:build android

package android

/*
#include "gles_android.h"
*/
import "C"

import "unsafe"

func (b *Backend) deleteSurfaceTexture(tex glesTexture) {
	if tex.id != 0 {
		C.glesDeleteTexture(C.int(tex.id))
	}
}

// uploadSurfaceTexture puts a surface frame into tex, reusing its storage at
// a fixed size and recreating it on a resize.
func (b *Backend) uploadSurfaceTexture(tex glesTexture, w, h int, pix []byte) glesTexture {
	if tex.id == 0 || tex.w != int32(w) || tex.h != int32(h) {
		b.deleteSurfaceTexture(tex)
		return createGLESTexture(int32(w), int32(h), pix)
	}
	C.glesUpdateTexture(C.int(tex.id), 0, 0,
		C.int(w), C.int(h), unsafe.Pointer(&pix[0]))
	return tex
}
