//go:build !js && !darwin && !android

package gl

import (
	"log"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
)

// glChannels holds a surface's planes as the textures a custom shader
// reads as iChannel0..3 (gui.Shader.Surface): uploaded from main memory,
// or, when frame is set, imported from the DMA-BUFs of a frame lent by
// surface (#1326), each texture backed by one of images.
type glChannels struct {
	tex     [4]glTexture
	fmt     [4]gui.PixelFormat
	n       int
	images  [4]uintptr
	frame   gui.GPUFrame
	surface *gui.PixelSurface
}

// glPlaneFormat maps a plane format to its GL internal format and the
// format of its pixel data.
func glPlaneFormat(f gui.PixelFormat) (internal int32, format uint32) {
	switch f {
	case gui.PixelFormatR8:
		return gogl.R8, gogl.RED
	case gui.PixelFormatRG8:
		return gogl.RG8, gogl.RG
	}
	return gogl.RGBA8, gogl.RGBA
}

// createPlaneTexture creates a texture for one plane. Like every texture
// here it samples linearly and clamps to the edge, the contract of
// gui.Shader.Surface.
func createPlaneTexture(f gui.PixelFormat, w, h int32, pix []byte) glTexture {
	internal, format := glPlaneFormat(f)
	var id uint32
	gogl.GenTextures(1, &id)
	gogl.BindTexture(gogl.TEXTURE_2D, id)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_MIN_FILTER, gogl.LINEAR)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_MAG_FILTER, gogl.LINEAR)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_WRAP_S, gogl.CLAMP_TO_EDGE)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_WRAP_T, gogl.CLAMP_TO_EDGE)
	// R8 and RG8 rows need not be 4-byte aligned; the default unpack
	// alignment would read past each row.
	gogl.PixelStorei(gogl.UNPACK_ALIGNMENT, 1)
	gogl.TexImage2D(gogl.TEXTURE_2D, 0, internal, w, h, 0, format,
		gogl.UNSIGNED_BYTE, unsafe.Pointer(&pix[0]))
	gogl.PixelStorei(gogl.UNPACK_ALIGNMENT, 4)
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
	return glTexture{id: id, w: w, h: h}
}

// uploadChannels puts a frame's planes into c, reusing each texture while
// its plane keeps its format and size.
func (b *Backend) uploadChannels(c glChannels, planes []gui.SurfacePlane) glChannels {
	if c.frame != nil {
		// An imported texture's storage is the producer's buffer: never
		// upload into it.
		c = b.retireImport(c, false)
	}
	for i, p := range planes {
		t := c.tex[i]
		w, h := int32(p.Width), int32(p.Height)
		if t.id == 0 || t.w != w || t.h != h || c.fmt[i] != p.Format {
			b.deleteSurfaceTexture(t)
			c.tex[i] = createPlaneTexture(p.Format, w, h, p.Pix)
			c.fmt[i] = p.Format
			continue
		}
		_, format := glPlaneFormat(p.Format)
		gogl.BindTexture(gogl.TEXTURE_2D, t.id)
		gogl.PixelStorei(gogl.UNPACK_ALIGNMENT, 1)
		gogl.TexSubImage2D(gogl.TEXTURE_2D, 0, 0, 0, w, h, format,
			gogl.UNSIGNED_BYTE, unsafe.Pointer(&p.Pix[0]))
		gogl.PixelStorei(gogl.UNPACK_ALIGNMENT, 4)
		gogl.BindTexture(gogl.TEXTURE_2D, 0)
	}
	for i := len(planes); i < c.n; i++ {
		b.deleteSurfaceTexture(c.tex[i])
		c.tex[i] = glTexture{}
	}
	c.n = len(planes)
	return c
}

// deleteChannels releases a surface's plane textures, and hands back an
// imported frame once the GPU is done with it.
func (b *Backend) deleteChannels(c glChannels) {
	if c.frame != nil {
		b.retireImport(c, false)
		return
	}
	for i := range c.n {
		b.deleteSurfaceTexture(c.tex[i])
	}
}

// importChannels shows a frame lent by surface: each plane's DMA-BUF
// becomes an EGLImage, and a texture takes its storage. Texture names are
// reused from frame to frame. The frame is already held for this window;
// the returned channels own that hold. When the driver refuses a plane the
// surface draws transparent until its next frame, and the frame goes back
// unread.
func (b *Backend) importChannels(c glChannels, surface *gui.PixelSurface, planes []gui.SurfacePlane,
	bufs []gui.DMABufPlane, frame gui.GPUFrame) glChannels {
	if c.frame != nil {
		c = b.retireImport(c, true)
	}
	for i := len(planes); i < c.n; i++ {
		b.deleteSurfaceTexture(c.tex[i])
		c.tex[i] = glTexture{}
	}
	c.surface, c.frame = surface, frame
	for i, p := range planes {
		step := "create"
		image := b.dmabuf.createImage(p.Format, p.Width, p.Height, bufs[i])
		t := c.tex[i]
		if image != 0 {
			step = "bind"
			if t.id == 0 {
				t.id = newImportTexture()
			}
			gogl.BindTexture(gogl.TEXTURE_2D, t.id)
			bound := b.dmabuf.bindImage(image)
			gogl.BindTexture(gogl.TEXTURE_2D, 0)
			if bound {
				t.w, t.h = int32(p.Width), int32(p.Height)
				c.tex[i], c.fmt[i], c.images[i] = t, p.Format, image
				continue
			}
			b.dmabuf.destroyImage(image)
			c.tex[i] = t
		}
		b.importOnce.Do(func() {
			log.Printf("gl: the driver refused to %s a DMA-BUF plane (%dx%d, format %d, modifier %#x, %d metadata planes); "+
				"the surface draws transparent, and the window reports that it does not import DMA-BUFs",
				step, p.Width, p.Height, p.Format, bufs[i].Modifier, len(bufs[i].Metadata))
		})
		return b.refuseImport(c, surface, frame, i, len(planes))
	}
	c.n = len(planes)
	return c
}

// refuseImport ends an import the driver refused at plane failed: the
// planes imported before it go, every texture goes, and the frame goes
// back unread. drawFrame then reports that the window does not import.
func (b *Backend) refuseImport(c glChannels, surface *gui.PixelSurface, frame gui.GPUFrame, failed, planes int) glChannels {
	b.importRefused = true
	b.retired.retire(surface, frame, false)
	for j := range failed {
		b.dmabuf.destroyImage(c.images[j])
	}
	for j := range planes {
		b.deleteSurfaceTexture(c.tex[j])
	}
	return glChannels{}
}

// retireImport ends an imported frame: its EGLImages go at once, and the
// frame goes back to its producer after a fence (gpu_frames.go). keepTex
// keeps the texture names for the next import; otherwise they are deleted.
func (b *Backend) retireImport(c glChannels, keepTex bool) glChannels {
	for i := range c.n {
		b.dmabuf.destroyImage(c.images[i])
		c.images[i] = 0
		if !keepTex {
			b.deleteSurfaceTexture(c.tex[i])
			c.tex[i] = glTexture{}
		}
	}
	b.retired.retire(c.surface, c.frame, true)
	c.frame, c.surface = nil, nil
	if !keepTex {
		c.n = 0
	}
	return c
}

// newImportTexture creates a texture name for an imported plane, sampled
// like every channel texture: linear, clamped to the edge.
func newImportTexture() uint32 {
	var id uint32
	gogl.GenTextures(1, &id)
	gogl.BindTexture(gogl.TEXTURE_2D, id)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_MIN_FILTER, gogl.LINEAR)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_MAG_FILTER, gogl.LINEAR)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_WRAP_S, gogl.CLAMP_TO_EDGE)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_WRAP_T, gogl.CLAMP_TO_EDGE)
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
	return id
}

// The fence calls frameRetirer makes, bound once in initGLResources.

func glFence() uintptr { return gogl.FenceSync(gogl.SYNC_GPU_COMMANDS_COMPLETE, 0) }

// glCheck polls a fence, flushing it to the GPU first so that it signals
// even when no swap follows.
func glCheck(fence uintptr) fenceState {
	switch gogl.ClientWaitSync(fence, gogl.SYNC_FLUSH_COMMANDS_BIT, 0) {
	case gogl.ALREADY_SIGNALED, gogl.CONDITION_SATISFIED:
		return fenceSignaled
	case gogl.TIMEOUT_EXPIRED:
		return fencePending
	}
	return fenceFailed
}

// glWait blocks until a fence signals, at teardown, and reports false
// when the wait failed or five one-second waits ran out; the caller then
// falls back to glFinish.
func glWait(fence uintptr) bool {
	for range 5 {
		switch gogl.ClientWaitSync(fence, gogl.SYNC_FLUSH_COMMANDS_BIT, 1_000_000_000) {
		case gogl.ALREADY_SIGNALED, gogl.CONDITION_SATISFIED:
			return true
		case gogl.TIMEOUT_EXPIRED:
			continue
		}
		return false
	}
	return false
}

// bindChannels binds plane i to texture unit i+1, and the transparent
// texture to every unit without a plane (or all of them when ok is false:
// a closed surface). Unit 0 is left active, as the other draws expect.
func (b *Backend) bindChannels(c glChannels, ok bool) {
	for i := range 4 {
		gogl.ActiveTexture(gogl.TEXTURE0 + uint32(i+1))
		id := b.transparentTex.id
		if ok && i < c.n {
			id = c.tex[i].id
		}
		gogl.BindTexture(gogl.TEXTURE_2D, id)
		b.stats.TextureBind()
	}
	gogl.ActiveTexture(gogl.TEXTURE0)
}

// setChannelUnits points a channel shader's iChannel samplers at units
// 1..4. A sampler's unit is program state, so this runs once per build.
func setChannelUnits(p pipeline) {
	gogl.UseProgram(p.program)
	for i, name := range [4]string{"iChannel0\x00", "iChannel1\x00", "iChannel2\x00", "iChannel3\x00"} {
		if loc := gogl.GetUniformLocation(p.program, glStr(name)); loc >= 0 {
			gogl.Uniform1i(loc, int32(i+1))
		}
	}
}
