//go:build !js && !darwin && !android

package gl

import (
	"log"

	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
)

// Antialiasing for the GL backend (#823).
//
// SVG paths and other triangle meshes are drawn with the solid pipeline, whose
// fragment shader gives full alpha anywhere inside a triangle. Without
// multisampling every edge pixel is either fully in or fully out, so a small
// icon shows hard stairs where a browser shows a smooth edge. The SDF shapes
// (rects, circles) are not affected: their shaders already fade the edge.
//
// The fix mirrors the Metal backend. The frame is drawn into an offscreen
// framebuffer with 4 samples per pixel, then resolved (averaged) into the
// window with one glBlitFramebuffer just before the swap. Filter containers
// get their own multisampled target, resolved into filterTexA before the blur
// and color passes, which read it as a plain texture.
//
// When the driver cannot give a complete multisampled framebuffer the backend
// falls back to drawing straight into the window, exactly as before.

// wantSamples is the sample count asked for. 4x is what every desktop GPU and
// Mesa's llvmpipe support, and what Metal uses, so edges match across backends.
const wantSamples = 4

// msaaTarget is one multisampled framebuffer: a color and a stencil
// renderbuffer with the same sample count. The stencil must be multisampled
// too, because GL requires every attachment of a framebuffer to agree on it.
type msaaTarget struct {
	fbo     uint32
	color   uint32
	stencil uint32
	w, h    int32

	// failed is set when the driver refused the framebuffer. The target
	// then stays off, so the refusal is logged once and not every frame.
	failed bool

	// resolveChecked is set once the first resolve out of this target has
	// been checked for a GL error (see endMainPass). destroy clears it, so a
	// target reallocated at a new size is checked again.
	resolveChecked bool
}

// initMSAA picks the sample count. It runs once, with the context current.
// A driver that reports fewer than 2 samples gets 1, which turns the feature
// off.
func (b *Backend) initMSAA() {
	var maxSamples int32
	gogl.GetIntegerv(gogl.MAX_SAMPLES, &maxSamples)
	b.msaaSamples = min(wantSamples, maxSamples)
	if b.msaaSamples < 2 {
		b.msaaSamples = 1
	}
}

// ensure (re)allocates t at w x h with the given sample count. It returns
// false, and leaves t empty, when the framebuffer is not complete.
func (t *msaaTarget) ensure(w, h, samples int32) bool {
	if t.failed {
		return false
	}
	if t.fbo != 0 && t.w == w && t.h == h {
		return true
	}
	t.destroy()
	if w <= 0 || h <= 0 {
		return false
	}

	gogl.GenRenderbuffers(1, &t.color)
	gogl.BindRenderbuffer(gogl.RENDERBUFFER, t.color)
	gogl.RenderbufferStorageMultisample(gogl.RENDERBUFFER, samples,
		gogl.RGBA8, w, h)
	gogl.GenRenderbuffers(1, &t.stencil)
	gogl.BindRenderbuffer(gogl.RENDERBUFFER, t.stencil)
	gogl.RenderbufferStorageMultisample(gogl.RENDERBUFFER, samples,
		gogl.STENCIL_INDEX8, w, h)
	gogl.BindRenderbuffer(gogl.RENDERBUFFER, 0)

	gogl.GenFramebuffers(1, &t.fbo)
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, t.fbo)
	gogl.FramebufferRenderbuffer(gogl.FRAMEBUFFER,
		gogl.COLOR_ATTACHMENT0, gogl.RENDERBUFFER, t.color)
	gogl.FramebufferRenderbuffer(gogl.FRAMEBUFFER,
		gogl.STENCIL_ATTACHMENT, gogl.RENDERBUFFER, t.stencil)
	status := gogl.CheckFramebufferStatus(gogl.FRAMEBUFFER)
	if status != gogl.FRAMEBUFFER_COMPLETE {
		log.Printf("gl: incomplete %dx multisample framebuffer: 0x%x; "+
			"edges will not be antialiased", samples, status)
		t.destroy()
		t.failed = true
		return false
	}
	t.w, t.h = w, h
	return true
}

func (t *msaaTarget) destroy() {
	if t.fbo != 0 {
		gogl.DeleteFramebuffers(1, &t.fbo)
	}
	if t.color != 0 {
		gogl.DeleteRenderbuffers(1, &t.color)
	}
	if t.stencil != 0 {
		gogl.DeleteRenderbuffers(1, &t.stencil)
	}
	*t = msaaTarget{}
}

// resolve averages t's samples into dst, a single-sample framebuffer (0 is the
// window). The blit ignores the stencil test and blending but not the scissor
// test, so the scissor is lifted for the copy and put back after it: a filter
// container resolves mid-frame, while a clip may still be active.
func (b *Backend) resolve(t *msaaTarget, dst uint32) {
	if b.scissorOn {
		gogl.Disable(gogl.SCISSOR_TEST)
	}
	gogl.BindFramebuffer(gogl.READ_FRAMEBUFFER, t.fbo)
	gogl.BindFramebuffer(gogl.DRAW_FRAMEBUFFER, dst)
	gogl.BlitFramebuffer(0, 0, t.w, t.h, 0, 0, t.w, t.h,
		gogl.COLOR_BUFFER_BIT, gogl.NEAREST)
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, dst)
	if b.scissorOn {
		gogl.Enable(gogl.SCISSOR_TEST)
	}
}

// beginMainPass binds the frame's draw target: the multisampled one when it
// can be had at the current size, else the window. A failure to allocate
// leaves msaaMain empty for the rest of the backend's life.
func (b *Backend) beginMainPass() {
	if b.msaaSamples > 1 {
		b.msaaMain.ensure(b.physW, b.physH, b.msaaSamples)
	}
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, b.msaaMain.fbo)
}

// endMainPass resolves the frame into dst. Without MSAA the frame was drawn
// into the window directly and there is nothing to do.
//
// The first resolve of each target is checked with glGetError. GL 3.3 refuses
// (INVALID_OPERATION) a multisample resolve between color formats that are
// not identical, and the window's format is the driver's pick: the EGL config
// and WGL pixel format ask only for at least 8 bits per channel, so a driver
// may hand back a format other than RGBA8. The offscreen framebuffer is
// complete either way, so ensure cannot see this. On an error the target is
// dropped and later frames draw straight into the window, without
// antialiasing, instead of showing nothing. One frame is lost to the check.
func (b *Backend) endMainPass(dst uint32) {
	t := &b.msaaMain
	if t.fbo == 0 {
		return
	}
	if t.resolveChecked {
		b.resolve(t, dst)
		return
	}
	// Clear errors left by earlier calls, so the check below sees only the
	// blit's. Bounded: a lost context may keep reporting an error.
	for range 8 {
		if gogl.GetError() == gogl.NO_ERROR {
			break
		}
	}
	b.resolve(t, dst)
	t.resolveChecked = true
	if e := gogl.GetError(); e != gogl.NO_ERROR {
		log.Printf("gl: %dx multisample resolve refused: 0x%x; "+
			"edges will not be antialiased", b.msaaSamples, e)
		t.destroy()
		t.failed = true
		gogl.BindFramebuffer(gogl.FRAMEBUFFER, dst)
	}
}

func (b *Backend) destroyMSAA() {
	b.msaaMain.destroy()
	b.msaaFilter.destroy()
}
