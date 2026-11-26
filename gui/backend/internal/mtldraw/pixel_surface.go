//go:build darwin && arm64

package mtldraw

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/surfacecache"
)

func (b *Renderer) deleteSurfaceTexture(tex metalTexture) {
	if tex.id != 0 {
		b.ctx.deleteTexture(int(tex.id))
	}
}

// uploadSurfaceTexture puts a surface frame into tex, reusing its storage at
// a fixed size and recreating it on a resize. The texture is streamed: an
// earlier frame may still be sampling it on the GPU, so the update waits for
// that frame first (metalUpdateStreamTexture).
func (b *Renderer) uploadSurfaceTexture(tex metalTexture, w, h int, pix []byte) metalTexture {
	if tex.id == 0 || tex.w != int32(w) || tex.h != int32(h) {
		b.deleteSurfaceTexture(tex)
		tex = createMetalTexture(b.ctx, int32(w), int32(h), pix)
		b.ctx.streamTexture(int(tex.id))
		return tex
	}
	b.ctx.updateStreamTexture(int(tex.id), w, h, 4, unsafe.Pointer(&pix[0]))
	return tex
}

// metalSurfaces holds a window's textures for gui.PixelSurface: one per
// surface an image draws, and the plane textures custom shaders read
// (gui.Shader.Surface). Each cache counts only its own kind of command.
type metalSurfaces struct {
	images   surfacecache.Cache[metalTexture]
	channels surfacecache.Cache[metalChannels]
}

// setup binds the functions that release the textures.
func (s *metalSurfaces) setup(b *Renderer) {
	s.images.SetRelease(b.deleteSurfaceTexture)
	s.channels.SetRelease(b.deleteChannels)
}

// prune releases the textures of surfaces this frame no longer draws.
func (s *metalSurfaces) prune(cmds []gui.RenderCmd) {
	s.images.Prune(cmds)
	s.channels.PruneShaders(cmds)
}

// clear releases every texture, at teardown.
func (s *metalSurfaces) clear() {
	s.images.Clear()
	s.channels.Clear()
}
