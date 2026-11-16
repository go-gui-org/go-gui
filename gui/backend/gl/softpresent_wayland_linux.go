//go:build linux && !js && !android && (amd64 || arm64)

package gl

// The CPU renderer's present path on native Wayland (#1265, step 4).
//
// A soft Wayland window has no EGL window, surface or context and no GL
// function table. Each frame is drawn by soft.Renderer into its own reused
// premultiplied-RGBA buffer, swizzled straight into a wl_shm buffer the
// compositor maps, and committed. Everything else (xdg-shell, libdecor,
// input, IME, clipboard, scale, occlusion) is the ordinary Wayland code,
// shared with GL.
//
// What X11 soft needs and Wayland does not:
//
//   - Pacing. The loop already waits for the frame callback between
//     frames (wlWindow.throttle), which is vsync on Wayland for GL too.
//   - Expose. The compositor keeps the last committed buffer.
//   - A visual. wl_shm always offers ARGB8888 and XRGB8888, whose memory
//     order on a little-endian machine is the B, G, R, A that
//     swizzleBGRA writes. The wl package builds only for amd64 and arm64.
//   - Row bands. The swizzle writes the shared memory directly: one copy
//     per frame, and no allocation once the slots exist.
//
// The compositor reads a committed buffer until it sends wl_buffer.release,
// so a frame cannot be drawn into a buffer it still holds. wlSlotRing keeps
// up to wlSoftMaxSlots buffers, made on demand, as Mesa's own wl_shm swap
// chain does. Most compositors release a shm buffer soon after the commit
// (they copy it into a texture), so two slots are the usual case.

import (
	"errors"
	"fmt"
	"image"
	"log"
	"time"

	"golang.org/x/sys/unix"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// wlSoftMaxSlots bounds the buffers one window keeps. With all of them held,
// the frame is skipped and drawn after the next release.
const wlSoftMaxSlots = 3

// wlSoftSlot is one wl_shm buffer and the memory it maps. busy is set at
// commit and cleared by wl_buffer.release. Main thread.
type wlSoftSlot struct {
	w, h int32
	mem  []byte
	buf  wl.Buffer
	busy bool
}

// wlSlotRing hands out a free buffer of the frame's size. alloc and free are
// fields so the pure tests can drive the ring with no compositor.
type wlSlotRing struct {
	slots [wlSoftMaxSlots]*wlSoftSlot
	alloc func(w, h int32) (*wlSoftSlot, error)
	free  func(*wlSoftSlot)
}

// acquire returns a free slot of w x h, making one if there is room. It
// returns nil, nil when the compositor holds every slot. Free slots of
// another size (left by a resize) are freed first, so a resize does not
// keep the old frames' memory; a held one is freed by the first acquire
// after its release.
func (r *wlSlotRing) acquire(w, h int32) (*wlSoftSlot, error) {
	var found *wlSoftSlot
	for i, s := range r.slots {
		if s == nil || s.busy {
			continue
		}
		if s.w != w || s.h != h {
			r.free(s)
			r.slots[i] = nil
			continue
		}
		if found == nil {
			found = s
		}
	}
	if found != nil {
		return found, nil
	}
	for i, s := range r.slots {
		if s != nil {
			continue
		}
		ns, err := r.alloc(w, h)
		if err != nil {
			return nil, err
		}
		r.slots[i] = ns
		return ns, nil
	}
	return nil, nil
}

// destroyAll frees every slot, held ones too: it runs when the surface
// goes, so no release will come for them.
func (r *wlSlotRing) destroyAll() {
	for i, s := range r.slots {
		if s != nil {
			r.free(s)
			r.slots[i] = nil
		}
	}
}

// wlSoftPresent is one window's CPU render-and-commit state. Non-nil
// wlWindow.soft selects this path.
type wlSoftPresent struct {
	r    *soft.Renderer
	ring wlSlotRing
	// last is the slot most recently committed, nil before the first
	// frame. The live tests read the frame back from its memory.
	last *wlSoftSlot

	// errLogged keeps a failure that repeats every frame (a size past the
	// soft limits, a memfd failure) to one log line until a frame works.
	errLogged bool
}

// wlShmSize is the byte size of a w x h ARGB8888 buffer. wlMaxWindowSize
// bounds both sides, so the size is at most 1 GiB and fits the int32 that
// wl_shm.create_pool takes.
func wlShmSize(w, h int32) (int, error) {
	if w <= 0 || h <= 0 || w > wlMaxWindowSize || h > wlMaxWindowSize {
		return 0, fmt.Errorf("wayland: soft: bad buffer size %dx%d", w, h)
	}
	return int(w) * int(h) * 4, nil
}

// wlShmFormat is the buffer format for a window. An opaque window takes
// XRGB8888, so the compositor ignores the alpha byte, which soft does not
// force to 1 (GL needs finishFrameAlpha for that). A transparent window
// takes ARGB8888; Wayland expects premultiplied alpha, which soft writes.
func wlShmFormat(transparent bool) uint32 {
	if transparent {
		return wl.ShmFormatArgb8888
	}
	return wl.ShmFormatXrgb8888
}

// newShmSlot makes a w x h buffer in its own memfd. The pool is destroyed at
// once and the fd closed: the buffer keeps the pool alive on the compositor
// side, and the mapping keeps the memory on ours. One fd per slot means a
// resize never has to shrink a pool, which wl_shm_pool.resize cannot.
func (d *wlDisplay) newShmSlot(w, h int32, format uint32) (*wlSoftSlot, error) {
	n, err := wlShmSize(w, h)
	if err != nil {
		return nil, err
	}
	fd, err := unix.MemfdCreate("go-gui-soft", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, fmt.Errorf("wayland: soft: memfd_create: %w", err)
	}
	defer func() { _ = unix.Close(fd) }()
	if err := unix.Ftruncate(fd, int64(n)); err != nil {
		return nil, fmt.Errorf("wayland: soft: ftruncate: %w", err)
	}
	mem, err := unix.Mmap(fd, 0, n, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("wayland: soft: mmap: %w", err)
	}
	pool := d.shm.CreatePool(fd, int32(n))
	s := &wlSoftSlot{w: w, h: h, mem: mem}
	s.buf = pool.CreateBuffer(0, w, h, w*4, format)
	pool.Destroy()
	// One closure per slot, made here and never per frame.
	s.buf.SetHandlers(wl.BufferHandlers{Release: func() { s.busy = false }})
	return s, nil
}

// freeShmSlot destroys the buffer and unmaps its memory.
func freeShmSlot(s *wlSoftSlot) {
	if s.buf.Valid() {
		s.buf.Destroy()
	}
	if s.mem != nil {
		_ = unix.Munmap(s.mem)
		s.mem = nil
	}
}

// copyBGRA swizzles img into dst, rows packed at 4 bytes per pixel. One
// swizzleBGRA call covers a tight image; a strided one goes row by row.
func copyBGRA(dst []byte, img *image.RGBA) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	row := w * 4
	if img.Stride == row {
		swizzleBGRA(dst, img.Pix[:row*h])
		return
	}
	for y := range h {
		swizzleBGRA(dst[y*row:(y+1)*row], img.Pix[y*img.Stride:y*img.Stride+row])
	}
}

// present draws w and commits it. It reports whether a buffer was
// committed. false (every slot held, or a frame that could not be drawn)
// leaves the window dirty, and the loop blocks until the next event: a
// buffer release is one, so a held buffer costs no busy wait.
func (sp *wlSoftPresent) present(ww *wlWindow, w *gui.Window, physW, physH int32,
	now time.Time) bool {

	slot, err := sp.ring.acquire(physW, physH)
	if err != nil {
		sp.logOnce(err)
		return false
	}
	if slot == nil {
		return false
	}
	// Draw reads the window's render commands, so it runs under the window
	// lock, as the GL drawFrame does.
	w.Lock()
	img, err := sp.r.Draw(w, int(physW), int(physH))
	w.Unlock()
	if err != nil {
		sp.logOnce(err)
		return false
	}
	sp.errLogged = false
	copyBGRA(slot.mem, img)
	// The frame callback is asked for before the commit it belongs to, as
	// the GL path does before its swap.
	ww.requestFrame(now)
	ww.surface.Attach(slot.buf, 0, 0)
	if ww.surface.Version() >= 4 {
		ww.surface.DamageBuffer(0, 0, physW, physH)
	} else {
		ww.surface.Damage(0, 0, ww.logW, ww.logH)
	}
	ww.surface.Commit()
	slot.busy = true
	sp.last = slot
	return true
}

// logOnce logs a failure that would otherwise repeat every frame.
func (sp *wlSoftPresent) logOnce(err error) {
	if !sp.errLogged {
		log.Printf("gl: soft: %v", err)
		sp.errLogged = true
	}
}

// destroy frees the buffers and the renderer. The surface is the caller's.
func (sp *wlSoftPresent) destroy() {
	if sp == nil {
		return
	}
	sp.last = nil
	sp.ring.destroyAll()
	sp.r.Release()
}

// waylandAutoSoft decides, from the probe of the Wayland EGL display,
// whether the display's windows draw with soft, and the log line that says
// why ("" for the plain GL default). It reuses autoPick's table and text.
// The display is open already, so there is no EGL failure case here: a
// failed display fails the Wayland setup, which falls back to X11, and X11
// probes for itself. Without wl_shm soft cannot present, so GL keeps
// drawing, slowly but correctly.
func waylandAutoSoft(res probeResult, haveShm bool) (bool, string) {
	c, why := autoPick(res)
	if c.soft && !haveShm {
		return false, fmt.Sprintf("gl: wayland: GL_RENDERER %q is a software rasterizer, "+
			"but the compositor offers no wl_shm for the CPU renderer; using GL",
			res.renderer)
	}
	return c.soft, why
}

// errNoShm: GOGUI_RENDERER=soft on a compositor with no wl_shm. The Wayland
// setup fails with it, and New falls back to X11, where soft presents with
// PutImage.
var errNoShm = errors.New("wayland: GOGUI_RENDERER=soft needs wl_shm, " +
	"which the compositor does not offer")

// --- Backend glue: the sites where the Wayland path branches on soft ---

// useNamedSoft is openWaylandDisplay's GOGUI_RENDERER=soft case: no
// libwayland-egl and no EGL are loaded. With no wl_shm soft cannot present;
// the Wayland setup fails and New falls back to X11, where soft presents
// with PutImage.
func (d *wlDisplay) useNamedSoft() error {
	if !d.shm.Valid() {
		return errNoShm
	}
	d.soft = true
	return nil
}

// autoPickSoft is openWaylandDisplay's unset-GOGUI_RENDERER case, once dpy is
// open: probe this display, not X11's, since on a Cinnamon Wayland session
// they differ (#1265). The probe's context is current on this thread only
// while it runs; New and runAppE have locked the thread.
func (d *wlDisplay) autoPickSoft(dpy uintptr) {
	soft, why := waylandAutoSoft(probeContext(dpy, d.eglConfig), d.shm.Valid())
	if why != "" {
		log.Print(why)
	}
	if !soft {
		return
	}
	// Frees the display's objects. Mesa keeps the driver itself mapped;
	// only GOGUI_RENDERER=soft avoids loading it.
	eglTerminate(dpy)
	d.eglDpy, d.eglConfig = 0, 0
	d.soft = true
}

// initGLWayland gives a configured window its EGL window, surface and
// context, and the GL resources. A failure leaves what was made in b.plat,
// where destroy frees it.
func (b *Backend) initGLWayland(w *gui.Window) error {
	ww, d := b.plat.wl, b.plat.wl.d
	eglWin, err := wl.NewEGLWindow(ww.surface, b.plat.physW, b.plat.physH)
	if err != nil {
		return err
	}
	ww.eglWin = eglWin
	b.plat.eglDpy = d.eglDpy
	b.plat.eglConfig = d.eglConfig
	surface, context, err := eglCreateSurfaceContextFunc(d.eglDpy, d.eglConfig, ww.eglWin.Ptr())
	// Kept before the error check, so destroy frees a half-made pair.
	b.plat.eglSurface, b.plat.eglContext = surface, context
	if err != nil {
		return newGPUContextError("create EGL surface/context", err)
	}
	// Frame pacing comes from frame callbacks (see throttle).
	eglSwapInterval(d.eglDpy, 0)
	if err := gogl.InitWithProcAddrFunc(eglProc); err != nil {
		return fmt.Errorf("gl: glbind init: %w", err)
	}
	if err := b.initGLResources(w); err != nil {
		b.destroyGLResources()
		return fmt.Errorf("gl: initGLResources: %w", err)
	}
	return nil
}

// initSoftWayland stands in for initGLWayland on a soft window: the renderer
// installs the text measurer and SVG parser, and the window is told it
// cannot run custom shaders, as on X11 soft.
func (b *Backend) initSoftWayland(w *gui.Window) error {
	ww := b.plat.wl
	r, err := soft.NewRenderer(w, b.dpiScale)
	if err != nil {
		return fmt.Errorf("gl: soft: %w", err)
	}
	format := wlShmFormat(ww.transparent)
	d := ww.d
	ww.soft = &wlSoftPresent{r: r, ring: wlSlotRing{
		alloc: func(sw, sh int32) (*wlSoftSlot, error) { return d.newShmSlot(sw, sh, format) },
		free:  freeShmSlot,
	}}
	w.SetNativePlatform(&nativePlatform{b: b})
	gui.SetWindowRendersShaders(w, false)
	return nil
}

// presentWayland is the Wayland loop's frame call. It reports whether a
// buffer was committed; GL always commits (its swap does).
func (b *Backend) presentWayland(w *gui.Window, now time.Time) bool {
	ww := b.plat.wl
	if sp := ww.soft; sp != nil {
		return sp.present(ww, w, b.physW, b.physH, now)
	}
	ww.requestFrame(now)
	b.renderFrame(w)
	return true
}

// resizeWayland applies a new buffer size and scale that wlWindow.resize
// already recorded. GL resizes the EGL window and its viewport; soft only
// needs the size and the renderer's scale; its buffers follow the size on
// the next acquire.
func (b *Backend) resizeWayland() {
	ww := b.plat.wl
	if sp := ww.soft; sp != nil {
		b.physW, b.physH = b.plat.drawableSize()
		b.applyDPIScale(b.plat.dpiScale())
		if err := sp.r.SetScale(b.dpiScale); err != nil {
			log.Printf("gl: soft: rescale: %v", err)
		}
		return
	}
	ww.eglWin.Resize(b.plat.physW, b.plat.physH)
	// Several windows share the thread, so the GL calls in handleResize
	// need this window's context current.
	b.plat.makeCurrent()
	b.handleResize()
}
