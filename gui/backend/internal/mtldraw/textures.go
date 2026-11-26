//go:build darwin && arm64

package mtldraw

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/backend/internal/memtex"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// metalTexture holds a Metal texture ID and dimensions.
type metalTexture struct {
	id   int32
	w, h int32
	// serial is the gui registry serial a mem: texture was uploaded
	// from; 0 for every other texture.
	serial uint64
}

// keepTexture is the texture cache's sweep predicate. A plain function,
// not a closure, so the sweep allocates nothing.
func keepTexture(key string, tex metalTexture) bool {
	return memtex.Current(key, tex.serial)
}

// metalTextureBytes is the texture cache's size function: RGBA8, 4 bytes a
// pixel. A failed load caches a zero texture, which costs nothing.
func metalTextureBytes(tex metalTexture) int64 {
	return int64(tex.w) * int64(tex.h) * 4
}

// newMetalTexCacheLRU bounds the textures by count and by bytes (#1239).
// A non-positive maxBytes (WindowCfg.MaxTextureBytes unset) takes the desktop
// default.
func newMetalTexCacheLRU(ctx *metalCtx,
	maxSize int, maxBytes int64,
) texcache.Cache[string, metalTexture] {
	return texcache.NewSized[string, metalTexture](maxSize,
		texcache.Budget(maxBytes, texcache.DesktopTextureBudget), metalTextureBytes,
		func(tex metalTexture) {
			if tex.id != 0 {
				ctx.deleteTexture(int(tex.id))
			}
		})
}

func createMetalTexture(ctx *metalCtx, w, h int32,
	pixels []byte) metalTexture {
	var ptr unsafe.Pointer
	if len(pixels) > 0 {
		ptr = unsafe.Pointer(&pixels[0])
	}
	id := ctx.createTexture(int(w), int(h), ptr)
	return metalTexture{id: int32(id), w: w, h: h}
}

// --- Image loading ---

// loadImageTexture opens, validates, decodes, and uploads an
// image to a Metal texture.
func (b *Renderer) loadImageTexture(
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
	return createMetalTexture(b.ctx, w, h, nrgba.Pix), nil
}
