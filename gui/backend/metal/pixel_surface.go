//go:build darwin && !ios

package metal

/*
#include "metal_darwin.h"
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/surfacecache"
)

func (b *windowState) deleteSurfaceTexture(tex metalTexture) {
	if tex.id != 0 {
		C.metalDeleteTexture(b.ctx, C.int(tex.id))
	}
}

// uploadSurfaceTexture puts a surface frame into tex, reusing its storage at
// a fixed size and recreating it on a resize. The texture is streamed: an
// earlier frame may still be sampling it on the GPU, so the update waits for
// that frame first (metalUpdateStreamTexture).
func (b *windowState) uploadSurfaceTexture(tex metalTexture, w, h int, pix []byte) metalTexture {
	if tex.id == 0 || tex.w != int32(w) || tex.h != int32(h) {
		b.deleteSurfaceTexture(tex)
		tex = createMetalTexture(b.ctx, int32(w), int32(h), pix)
		C.metalStreamTexture(b.ctx, C.int(tex.id))
		return tex
	}
	C.metalUpdateStreamTexture(b.ctx, C.int(tex.id),
		C.int(w), C.int(h), 4, unsafe.Pointer(&pix[0]))
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
func (s *metalSurfaces) setup(b *windowState) {
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
