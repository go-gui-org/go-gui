//go:build ios

package ios

/*
#include "metal_darwin.h"
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// metalTexture holds a Metal texture ID and dimensions.
type metalTexture struct {
	id   int32
	w, h int32
}

func newMetalTexCacheLRU(
	maxSize int,
) texcache.Cache[string, metalTexture] {
	return texcache.New[string, metalTexture](maxSize,
		func(tex metalTexture) {
			if tex.id != 0 {
				C.metalDeleteTexture(C.int(tex.id))
			}
		})
}

func createMetalTexture(w, h int32,
	pixels []byte) metalTexture {
	hasData := C.int(0)
	var ptr unsafe.Pointer
	if len(pixels) > 0 {
		hasData = C.int(1)
		ptr = unsafe.Pointer(&pixels[0])
	}
	id := C.metalCreateTexture(
		C.int(w), C.int(h), ptr, hasData)
	return metalTexture{id: int32(id), w: w, h: h}
}

// loadImageTexture opens, validates, decodes, and uploads an
// image to a Metal texture.
func (b *Backend) loadImageTexture(
	path string) (metalTexture, error) {
	f, err := imgload.OpenSafe(path, b.allowedImageRoots)
	if err != nil {
		return metalTexture{}, err
	}
	defer func() { _ = f.Close() }()

	nrgba, err := imgload.DecodeNRGBA(
		path, f, b.maxImageBytes, b.maxImagePixels)
	if err != nil {
		return metalTexture{}, err
	}
	bounds := nrgba.Bounds()
	w := int32(bounds.Dx())
	h := int32(bounds.Dy())
	return createMetalTexture(w, h, nrgba.Pix), nil
}
