//go:build android

package android

/*
#include "gles_android.h"
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// glesTexture holds a GLES texture ID and dimensions.
type glesTexture struct {
	id   int32
	w, h int32
}

func newGLESTexCacheLRU(
	maxSize int,
) texcache.Cache[string, glesTexture] {
	return texcache.New[string, glesTexture](maxSize,
		func(tex glesTexture) {
			if tex.id != 0 {
				C.glesDeleteTexture(C.int(tex.id))
			}
		})
}

func createGLESTexture(w, h int32,
	pixels []byte) glesTexture {
	hasData := C.int(0)
	var ptr unsafe.Pointer
	if len(pixels) > 0 {
		hasData = C.int(1)
		ptr = unsafe.Pointer(&pixels[0])
	}
	id := C.glesCreateTexture(
		C.int(w), C.int(h), ptr, hasData)
	return glesTexture{id: int32(id), w: w, h: h}
}

// loadImageTexture opens, validates, decodes, and uploads an
// image to a GLES texture.
func (b *Backend) loadImageTexture(
	path string) (glesTexture, error) {
	f, err := imgload.OpenSafe(path, b.allowedImageRoots)
	if err != nil {
		return glesTexture{}, err
	}
	defer func() { _ = f.Close() }()

	nrgba, err := imgload.DecodeNRGBA(
		path, f, b.maxImageBytes, b.maxImagePixels)
	if err != nil {
		return glesTexture{}, err
	}
	bounds := nrgba.Bounds()
	w := int32(bounds.Dx())
	h := int32(bounds.Dy())
	return createGLESTexture(w, h, nrgba.Pix), nil
}
