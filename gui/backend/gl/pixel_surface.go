//go:build !js && !darwin && !android

package gl

import (
	"unsafe"

	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
)

func (b *Backend) deleteSurfaceTexture(tex glTexture) {
	if tex.id != 0 {
		gogl.DeleteTextures(1, &tex.id)
	}
}

// uploadSurfaceTexture puts a surface frame into tex, reusing its storage at
// a fixed size and recreating it on a resize.
func (b *Backend) uploadSurfaceTexture(tex glTexture, w, h int, pix []byte) glTexture {
	if tex.id == 0 || tex.w != int32(w) || tex.h != int32(h) {
		b.deleteSurfaceTexture(tex)
		return createTexture(int32(w), int32(h), pix)
	}
	gogl.BindTexture(gogl.TEXTURE_2D, tex.id)
	gogl.TexSubImage2D(gogl.TEXTURE_2D, 0, 0, 0, tex.w, tex.h,
		gogl.RGBA, gogl.UNSIGNED_BYTE, unsafe.Pointer(&pix[0]))
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
	return tex
}
