//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// The experimental native Wayland backend (#919, docs/specs/wayland-backend.md).
//
// A Wayland window is an ordinary Backend whose platformState has a non-nil
// wl field. Everything that only reads EGL handles, the drawable size and the
// scale (makeCurrent, swap, drawableSize, dpiScale, renderFrame) works
// unchanged, so the renderer has no Wayland branch. The X11 fields stay zero;
// the X11 entry points the gui layer can reach (ShowWindow, opacity, cursor,
// IME) already return early on a nil connection.

// wlConfigureTimeout bounds the wait for the compositor's first configure.
// A compositor answers within a round trip, so hitting it means a broken
// compositor, and the caller falls back to X11.
const wlConfigureTimeout = 5 * time.Second

// wlFrameTimeout bounds the wait for a frame callback. A compositor sends
// none while the window is hidden or on no output. After the timeout the
// window renders anyway, at most once per timeout, so queued commands and
// animations still advance slowly instead of stalling forever.
const wlFrameTimeout = time.Second

// wlDisplay is the one compositor connection every Wayland window shares,
// with the globals and the EGL display made from it. Main thread only.
type wlDisplay struct {
	conn       *wl.Conn
	registry   wl.Registry
	compositor wl.Compositor
	wmBase     wl.XdgWmBase
	eglDpy     uintptr
	eglConfig  uintptr
	// refs counts the windows (and a running RunApp loop) using the
	// display. The last release disconnects.
	refs int
}

// wlShared is the open display, or nil. The wl package supports one
// connection at a time, which is also all a process needs.
var wlShared *wlDisplay

// acquireWaylandDisplay returns the shared display, connecting on first use.
func acquireWaylandDisplay() (*wlDisplay, error) {
	if wlShared != nil {
		wlShared.refs++
		return wlShared, nil
	}
	d, err := openWaylandDisplay()
	if err != nil {
		return nil, err
	}
	wlShared = d
	return d, nil
}

// openWaylandDisplay connects, binds the globals a window needs and
// initializes EGL on the connection. Every failure here happens before a
// window exists, so the caller can still fall back to X11.
func openWaylandDisplay() (d *wlDisplay, err error) {
	conn, err := wl.Connect("")
	if err != nil {
		return nil, err
	}
	d = &wlDisplay{conn: conn, refs: 1}
	defer func() {
		if err != nil {
			d.release()
		}
	}()

	var compName, compVer, wmName, wmVer uint32
	d.registry = conn.Display.GetRegistry()
	d.registry.SetHandlers(wl.RegistryHandlers{
		// iface is only valid during the call, so only numbers are kept.
		Global: func(name uint32, iface string, version uint32) {
			switch iface {
			case "wl_compositor":
				compName, compVer = name, version
			case "xdg_wm_base":
				wmName, wmVer = name, version
			}
		},
	})
	if err = conn.Roundtrip(); err != nil {
		return d, err
	}
	if compName == 0 || wmName == 0 {
		return d, errors.New("compositor offers no wl_compositor or xdg_wm_base")
	}
	d.compositor = wl.Compositor{Proxy: d.registry.Bind(compName,
		&wl.CompositorInterface, min(compVer, wl.CompositorInterface.Version()))}
	d.wmBase = wl.XdgWmBase{Proxy: d.registry.Bind(wmName,
		&wl.XdgWmBaseInterface, min(wmVer, wl.XdgWmBaseInterface.Version()))}
	// A client that misses pings is marked unresponsive by the compositor.
	d.wmBase.SetHandlers(wl.XdgWmBaseHandlers{Ping: d.wmBase.Pong})

	// libwayland-egl is checked here, not at the first window, so a
	// system without it falls back to X11 instead of failing New.
	if err = wl.LoadEGL(); err != nil {
		return d, err
	}
	if err = loadEGL(); err != nil {
		return d, err
	}
	var native uintptr
	if eglGetPlatformDisplay != nil {
		native = eglGetPlatformDisplay(eglPlatformWaylandKHR, conn.Display.Ptr(), nil)
	} else {
		// EGL 1.4: Mesa detects a wl_display* passed to eglGetDisplay.
		native = eglGetDisplay(conn.Display.Ptr())
	}
	dpy, cands, err := eglOpenDisplay(native, "eglGetPlatformDisplay(Wayland)")
	// eglOpenDisplay returns dpy non-zero after eglInitialize even on an
	// error, so release can terminate it.
	d.eglDpy = dpy
	if err != nil {
		return d, err
	}
	// Wayland has no visuals: any config of the right type works, so the
	// driver's first choice is taken. Its 8-bit alpha channel only shows
	// through where no opaque region is set (see setOpaque).
	d.eglConfig = cands[0].config
	return d, nil
}

// release drops one reference. The last one tears the connection down.
func (d *wlDisplay) release() {
	d.refs--
	if d.refs > 0 {
		return
	}
	if d.eglDpy != 0 {
		eglTerminate(d.eglDpy)
		d.eglDpy = 0
	}
	if d.wmBase.Valid() {
		d.wmBase.Destroy()
	}
	if d.compositor.Valid() {
		d.compositor.DestroyProxy()
	}
	if d.registry.Valid() {
		d.registry.DestroyProxy()
	}
	d.conn.Close()
	if wlShared == d {
		wlShared = nil
	}
}

// wlWindow is the Wayland state of one window: the surface, its xdg-shell
// roles, the EGL native window and the configure and frame bookkeeping.
type wlWindow struct {
	d          *wlDisplay
	b          *Backend
	surface    wl.Surface
	xdgSurface wl.XdgSurface
	toplevel   wl.XdgToplevel
	eglWin     wl.EGLWindow

	// pendW, pendH and pendActive hold what the last
	// xdg_toplevel.configure asked for. A size of 0 leaves the choice to
	// the client. They take effect on the xdg_surface.configure that
	// closes the sequence.
	pendW, pendH int32
	pendActive   bool

	// logW, logH are the window size in logical pixels; scale is the
	// integer buffer scale. The buffer is logW*scale × logH*scale.
	logW, logH int32
	scale      int32

	configured  bool // the first configure arrived
	active      bool // last focus state sent to gui
	ready       bool // EGL and GL are up; size changes resize and emit
	dirty       bool // the next loop pass renders even if FrameFn has nothing
	transparent bool // no opaque region: the alpha channel shows

	// throttle makes the loop wait for the compositor's frame callback
	// between frames. That is vsync on Wayland: eglSwapInterval stays 0
	// so eglSwapBuffers never blocks inside Mesa on a hidden window.
	// frameCb is the outstanding callback, frameAt when it was asked for.
	// frameDone is its event decoder, built once so a frame allocates no
	// closure for it.
	throttle  bool
	frameCb   wl.Callback
	frameAt   time.Time
	frameDone wl.CallbackDispatcher

	// title is a title set from another goroutine, applied by the loop.
	// The wl package is single-threaded, so SetTitle cannot send it.
	titleMu    sync.Mutex
	title      string
	titleDirty bool
}

// wlClipboard stands in for the system clipboard until wl_data_device
// lands (phase 6): copy and paste work inside the process only.
var wlClipboard struct {
	mu   sync.Mutex
	text string
}

// configureSize resolves a configure's suggested size against the current
// one. Zero (or a negative value from a broken compositor) means the
// client picks, and the client keeps what it has.
func configureSize(pendW, pendH, curW, curH int32) (int32, int32) {
	if pendW <= 0 {
		pendW = curW
	}
	if pendH <= 0 {
		pendH = curH
	}
	return max(pendW, 1), max(pendH, 1)
}

// statesHave reports whether an xdg_toplevel states array, a packed list
// of native-endian uint32 values, contains state.
func statesHave(states []byte, state uint32) bool {
	for i := 0; i+4 <= len(states); i += 4 {
		if binary.LittleEndian.Uint32(states[i:]) == state {
			return true
		}
	}
	return false
}

// newWaylandBackend creates a window on the shared Wayland display. It
// mirrors New: the goroutine stays locked to its OS thread on success, and
// Destroy releases it.
//
//nolint:gocyclo // linear setup with one cleanup per step
func newWaylandBackend(w *gui.Window) (*Backend, error) {
	runtime.LockOSThread()
	handedOff := false
	defer func() {
		if !handedOff {
			runtime.UnlockOSThread()
		}
	}()

	d, err := acquireWaylandDisplay()
	if err != nil {
		return nil, err
	}
	cfg := w.Config
	b := &Backend{}
	ww := &wlWindow{d: d, b: b, scale: 1, throttle: !cfg.VSyncOff,
		transparent: cfg.Transparent}
	b.plat.wl = ww
	b.plat.w = w
	// At most one frame callback is outstanding (requestFrame), so the
	// one that fires is always ww.frameCb.
	ww.frameDone = wl.CallbackHandlers{Done: func(uint32) {
		ww.frameCb.DestroyProxy()
		ww.frameCb = wl.Callback{}
	}}.Dispatcher()
	fail := func(err error) (*Backend, error) {
		b.plat.destroy()
		return nil, err
	}

	ww.logW, ww.logH = int32(cfg.Width), int32(cfg.Height)
	if ww.logW <= 0 {
		ww.logW = 640
	}
	if ww.logH <= 0 {
		ww.logH = 480
	}
	b.plat.physW, b.plat.physH, b.plat.scale = ww.logW, ww.logH, 1

	ww.surface = d.compositor.CreateSurface()
	ww.surface.SetHandlers(wl.SurfaceHandlers{PreferredBufferScale: ww.setScale})
	ww.xdgSurface = d.wmBase.GetXdgSurface(ww.surface)
	ww.xdgSurface.SetHandlers(wl.XdgSurfaceHandlers{Configure: ww.applyConfigure})
	ww.toplevel = ww.xdgSurface.GetToplevel()
	ww.toplevel.SetHandlers(wl.XdgToplevelHandlers{
		Configure: func(width, height int32, states []byte) {
			ww.pendW, ww.pendH = width, height
			ww.pendActive = statesHave(states, wl.XdgToplevelStateActivated)
		},
		Close: func() { gui.DispatchCloseRequest(w) },
	})
	title := cfg.Title
	if title == "" {
		title = "go-gui"
	}
	ww.toplevel.SetTitle(title)
	if cfg.WMClass != "" {
		ww.toplevel.SetAppId(cfg.WMClass)
	}
	// Limits are in logical pixels, the unit xdg_toplevel uses. A zero
	// axis means "no limit" in both requests.
	limits := gui.WindowSizeLimits(cfg)
	if limits.MinW > 0 || limits.MinH > 0 {
		ww.toplevel.SetMinSize(int32(limits.MinW), int32(limits.MinH))
	}
	if limits.MaxW > 0 || limits.MaxH > 0 {
		ww.toplevel.SetMaxSize(int32(limits.MaxW), int32(limits.MaxH))
	}

	// The first commit carries no buffer; it asks for the initial
	// configure. No buffer may be attached before that configure is
	// acknowledged, so EGL is set up only after it.
	ww.surface.Commit()
	deadline := time.Now().Add(wlConfigureTimeout)
	for !ww.configured {
		if time.Now().After(deadline) {
			return fail(errors.New("wayland: no configure from the compositor"))
		}
		if err := d.conn.Dispatch(100 * time.Millisecond); err != nil {
			return fail(fmt.Errorf("wayland: wait for configure: %w", err))
		}
	}

	ww.eglWin, err = wl.NewEGLWindow(ww.surface, b.plat.physW, b.plat.physH)
	if err != nil {
		return fail(err)
	}
	b.plat.eglDpy = d.eglDpy
	b.plat.eglConfig = d.eglConfig
	surface, context, err := eglCreateSurfaceContextFunc(d.eglDpy, d.eglConfig, ww.eglWin.Ptr())
	// Kept before the error check, so destroy frees a half-made pair.
	b.plat.eglSurface, b.plat.eglContext = surface, context
	if err != nil {
		return fail(newGPUContextError("create EGL surface/context", err))
	}
	// Frame pacing comes from frame callbacks (see throttle).
	eglSwapInterval(d.eglDpy, 0)
	if err := gogl.InitWithProcAddrFunc(eglProc); err != nil {
		return fail(fmt.Errorf("gl: glbind init: %w", err))
	}

	b.dpiScale = b.plat.scale
	b.physW = b.plat.physW
	b.physH = b.plat.physH
	b.initCaches(cfg)
	if err := b.initGLResources(w); err != nil {
		b.destroyGLResources()
		return fail(fmt.Errorf("gl: initGLResources: %w", err))
	}
	ww.setOpaque()
	ww.ready = true
	ww.dirty = true

	w.SetTitleFn(ww.setTitle)
	w.SetClipboardFn(func(s string) {
		wlClipboard.mu.Lock()
		wlClipboard.text = s
		wlClipboard.mu.Unlock()
	})
	w.SetClipboardGetFn(func() string {
		wlClipboard.mu.Lock()
		defer wlClipboard.mu.Unlock()
		return wlClipboard.text
	})

	b.plat.lockedTid = syscall.Gettid()
	handedOff = true
	return b, nil
}

// id is the window's key in gui.App: the surface's protocol object id.
func (ww *wlWindow) id() uint32 { return ww.surface.ID() }

// applyConfigure handles xdg_surface.configure: acknowledge it, then apply
// the size and focus the toplevel configure before it carried. The
// compositor expects a new buffer in reply, so the window is marked dirty.
func (ww *wlWindow) applyConfigure(serial uint32) {
	ww.xdgSurface.AckConfigure(serial)
	ww.configured = true
	w, h := configureSize(ww.pendW, ww.pendH, ww.logW, ww.logH)
	ww.resize(w, h, ww.scale)
	ww.syncFocus()
	ww.dirty = true
}

// setScale handles wl_surface.preferred_buffer_scale (wl_surface v6).
func (ww *wlWindow) setScale(factor int32) {
	ww.resize(ww.logW, ww.logH, max(factor, 1))
	ww.dirty = true
}

// resize adopts a new logical size and buffer scale. Before the window is
// ready it only records them; New sizes the EGL window from the result.
func (ww *wlWindow) resize(w, h, scale int32) {
	if w == ww.logW && h == ww.logH && scale == ww.scale {
		return
	}
	if scale != ww.scale && ww.surface.Version() >= 3 {
		// Applies with the next commit, which is the eglSwapBuffers of a
		// buffer already at the new size.
		ww.surface.SetBufferScale(scale)
	}
	ww.logW, ww.logH, ww.scale = w, h, scale
	b := ww.b
	b.plat.physW, b.plat.physH, b.plat.scale = w*scale, h*scale, float32(scale)
	if !ww.ready {
		return
	}
	ww.eglWin.Resize(b.plat.physW, b.plat.physH)
	ww.setOpaque()
	// Several windows share the thread, so the GL calls in handleResize
	// need this window's context current.
	b.plat.makeCurrent()
	b.handleResize()
	lw, lh := b.logicalXY(b.physW, b.physH)
	b.emit(gui.Event{
		Type:         gui.EventResized,
		WindowWidth:  int(lw),
		WindowHeight: int(lh),
	})
}

// syncFocus sends gui a focus change when the compositor's activated
// state differs from what gui was last told. Before the window is ready
// it waits: the run loop calls it again once OnInit has run.
func (ww *wlWindow) syncFocus() {
	if !ww.ready || ww.pendActive == ww.active {
		return
	}
	ww.active = ww.pendActive
	if ww.active {
		ww.b.emit(gui.Event{Type: gui.EventFocused})
	} else {
		ww.b.emit(gui.Event{Type: gui.EventUnfocused})
	}
}

// setOpaque marks the whole surface opaque, so the compositor ignores the
// alpha channel and skips blending what is behind the window. A
// transparent window sets no region. Applies with the next commit.
func (ww *wlWindow) setOpaque() {
	if ww.transparent {
		return
	}
	r := ww.d.compositor.CreateRegion()
	r.Add(0, 0, ww.logW, ww.logH)
	ww.surface.SetOpaqueRegion(r)
	r.Destroy()
}

// setTitle is the window's title hook. It may run on any goroutine.
func (ww *wlWindow) setTitle(t string) {
	ww.titleMu.Lock()
	ww.title, ww.titleDirty = t, true
	ww.titleMu.Unlock()
	ww.d.conn.Wake()
}

// flushTitle sends a title set by setTitle. Main thread.
func (ww *wlWindow) flushTitle() {
	ww.titleMu.Lock()
	t, dirty := ww.title, ww.titleDirty
	ww.titleDirty = false
	ww.titleMu.Unlock()
	if dirty {
		ww.toplevel.SetTitle(t)
	}
}

// framePending reports whether the window waits for a frame callback, and
// for how much longer at most.
func (ww *wlWindow) framePending(now time.Time) (bool, time.Duration) {
	if !ww.frameCb.Valid() {
		return false, 0
	}
	left := wlFrameTimeout - now.Sub(ww.frameAt)
	return left > 0, left
}

// requestFrame asks for a frame callback before the frame is committed.
// One callback is outstanding at most. When one timed out (a hidden
// window), it stays outstanding and frameAt restarts the timeout, which
// limits a hidden window to one frame per wlFrameTimeout.
func (ww *wlWindow) requestFrame(now time.Time) {
	if !ww.throttle {
		return
	}
	ww.frameAt = now
	if ww.frameCb.Valid() {
		return
	}
	ww.frameCb = ww.surface.Frame()
	ww.frameCb.SetDispatcher(ww.frameDone)
}

// destroyWayland frees the window's EGL and Wayland objects, children
// before parents, and drops its display reference. The shared EGL
// display is terminated only by the last window.
func (p *platformState) destroyWayland() {
	ww := p.wl
	p.wl = nil
	if p.eglDpy != 0 {
		eglMakeCurrent(p.eglDpy, 0, 0, 0)
		if p.eglContext != 0 {
			eglDestroyContext(p.eglDpy, p.eglContext)
			p.eglContext = 0
		}
		if p.eglSurface != 0 {
			eglDestroySurface(p.eglDpy, p.eglSurface)
			p.eglSurface = 0
		}
		p.eglDpy = 0
	}
	ww.eglWin.Destroy()
	if ww.frameCb.Valid() {
		ww.frameCb.DestroyProxy()
	}
	if ww.toplevel.Valid() {
		ww.toplevel.Destroy()
	}
	if ww.xdgSurface.Valid() {
		ww.xdgSurface.Destroy()
	}
	if ww.surface.Valid() {
		ww.surface.Destroy()
	}
	ww.d.release()
}
