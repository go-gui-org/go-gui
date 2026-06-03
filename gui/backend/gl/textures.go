//go:build !js

package gl

import (
	"log"
	"unsafe"

	gogl "github.com/go-gl/gl/v3.3-core/gl"

	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// glTexture holds an OpenGL texture handle and dimensions.
type glTexture struct {
	id   uint32
	w, h int32
}

func newGLTexCacheLRU(
	maxSize int,
) texcache.Cache[string, glTexture] {
	return texcache.New[string, glTexture](maxSize,
		func(tex glTexture) {
			if tex.id != 0 {
				gogl.DeleteTextures(1, &tex.id)
			}
		})
}

// createTexture creates an RGBA8 texture from pixel data.
func createTexture(w, h int32, pixels []byte) glTexture {
	var id uint32
	gogl.GenTextures(1, &id)
	gogl.BindTexture(gogl.TEXTURE_2D, id)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_MIN_FILTER,
		gogl.LINEAR)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_MAG_FILTER,
		gogl.LINEAR)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_WRAP_S,
		gogl.CLAMP_TO_EDGE)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_WRAP_T,
		gogl.CLAMP_TO_EDGE)

	var ptr unsafe.Pointer
	if len(pixels) > 0 {
		ptr = unsafe.Pointer(&pixels[0])
	}
	gogl.TexImage2D(gogl.TEXTURE_2D, 0, gogl.RGBA8,
		w, h, 0, gogl.RGBA, gogl.UNSIGNED_BYTE, ptr)
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
	return glTexture{id: id, w: w, h: h}
}

// createEmptyTexture creates a texture with no initial data
// (for FBO render targets).
func createEmptyTexture(w, h int32) glTexture {
	return createTexture(w, h, nil)
}

// --- FBO management ---

func (b *Backend) ensureFilterFBO(w, h int32) bool {
	if b.filterFBO != 0 && b.filterW == w && b.filterH == h {
		return true
	}
	b.destroyFilterFBO()

	gogl.GenFramebuffers(1, &b.filterFBO)

	texA := createEmptyTexture(w, h)
	texB := createEmptyTexture(w, h)
	b.filterTexA = texA.id
	b.filterTexB = texB.id
	b.filterW = w
	b.filterH = h

	// Attach a stencil renderbuffer so ClipContents works
	// inside filter containers.
	gogl.GenRenderbuffers(1, &b.filterStencil)
	gogl.BindRenderbuffer(gogl.RENDERBUFFER, b.filterStencil)
	gogl.RenderbufferStorage(gogl.RENDERBUFFER,
		gogl.STENCIL_INDEX8, w, h)
	gogl.BindRenderbuffer(gogl.RENDERBUFFER, 0)

	b.bindFBO(b.filterTexA)
	gogl.FramebufferRenderbuffer(gogl.FRAMEBUFFER,
		gogl.STENCIL_ATTACHMENT, gogl.RENDERBUFFER,
		b.filterStencil)
	status := gogl.CheckFramebufferStatus(gogl.FRAMEBUFFER)
	b.unbindFBO()
	if status != gogl.FRAMEBUFFER_COMPLETE {
		log.Printf("gl: incomplete filter framebuffer: 0x%x",
			status)
		b.destroyFilterFBO()
		return false
	}
	return true
}

func (b *Backend) destroyFilterFBO() {
	if b.filterStencil != 0 {
		gogl.DeleteRenderbuffers(1, &b.filterStencil)
		b.filterStencil = 0
	}
	if b.filterFBO != 0 {
		gogl.DeleteFramebuffers(1, &b.filterFBO)
		b.filterFBO = 0
	}
	if b.filterTexA != 0 {
		gogl.DeleteTextures(1, &b.filterTexA)
		b.filterTexA = 0
	}
	if b.filterTexB != 0 {
		gogl.DeleteTextures(1, &b.filterTexB)
		b.filterTexB = 0
	}
}

func (b *Backend) bindFBO(tex uint32) {
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, b.filterFBO)
	gogl.FramebufferTexture2D(gogl.FRAMEBUFFER,
		gogl.COLOR_ATTACHMENT0, gogl.TEXTURE_2D, tex, 0)
}

func (b *Backend) unbindFBO() {
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, 0)
}

// --- Image loading ---

// loadImageTexture opens, validates, decodes, and uploads an
// image to a GL texture.
func (b *Backend) loadImageTexture(
	path string,
) (glTexture, error) {
	f, err := imgload.OpenSafe(path, b.allowedImageRoots)
	if err != nil {
		return glTexture{}, err
	}
	defer func() { _ = f.Close() }()

	nrgba, err := imgload.DecodeNRGBA(
		path, f, b.maxImageBytes, b.maxImagePixels)
	if err != nil {
		return glTexture{}, err
	}
	bounds := nrgba.Bounds()
	w := int32(bounds.Dx())
	h := int32(bounds.Dy())
	return createTexture(w, h, nrgba.Pix), nil
}
