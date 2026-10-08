//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/decor"
	"github.com/go-gui-org/go-gui/gui/backend/internal/devscale"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
	"github.com/go-gui-org/go-gui/gui/backend/internal/xkb"
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
// none while the window is hidden, covered or on no output. Past it the
// window counts as occluded (checkStall) until the callback's done.
const wlFrameTimeout = time.Second

// wlDisplay is the one compositor connection every Wayland window shares,
// with the globals and the EGL display made from it. Main thread only.
type wlDisplay struct {
	conn       *wl.Conn
	registry   wl.Registry
	compositor wl.Compositor
	wmBase     wl.XdgWmBase
	// Optional globals (phase 6). Each is the zero value when the
	// compositor does not offer it, and the feature degrades: see the
	// file that uses it.
	shm          wl.Shm                                // fallback cursor images
	cursorShape  wl.WpCursorShapeManagerV1             // cursors by name
	dataMgr      wl.DataDeviceManager                  // clipboard
	primaryMgr   wl.ZwpPrimarySelectionDeviceManagerV1 // primary selection
	decoMgr      wl.ZxdgDecorationManagerV1            // server-side frames
	viewporter   wl.WpViewporter                       // fractional scale
	fracMgr      wl.WpFractionalScaleManagerV1         // fractional scale
	textInputMgr wl.ZwpTextInputManagerV3              // input methods
	// decor is the libdecor instance, made by the first window that
	// needs a frame drawn (wayland_shell_linux.go); decorTried records
	// that it was tried, so a failure is not retried per window.
	decor      *decor.Context
	decorTried bool
	eglDpy     uintptr
	eglConfig  uintptr
	// seat is the input seat, nil when the compositor offers none.
	// seatName is its registry global name, to notice its removal.
	seat     *wlSeat
	seatName uint32
	// outputs are the bound wl_outputs by registry global name, for the
	// scale on a compositor that sends none (wayland_output_linux.go).
	outputs map[uint32]*wlOutput
	// wins maps a wl_surface pointer to its window, so input events,
	// which name the surface, reach the right Backend.
	wins map[uintptr]*Backend
	// localSel is the clipboard and primary text when the compositor
	// offers no selection device (or no seat): copy and paste then work
	// inside the process only. Main thread.
	localSel [2]string
	// calls holds work other goroutines handed to the loop (post).
	callsMu sync.Mutex
	calls   []func()
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
	d = &wlDisplay{conn: conn, refs: 1, wins: map[uintptr]*Backend{},
		outputs: map[uint32]*wlOutput{}}
	defer func() {
		if err != nil {
			d.release()
		}
	}()

	// globals holds what the first round trip announced, by interface.
	// Only the seat and the outputs are followed after that: the other
	// globals are bound once, and one appearing later is not used.
	globals := map[string]wlGlobal{}
	d.registry = conn.Display.GetRegistry()
	d.registry.SetHandlers(wl.RegistryHandlers{
		// iface is only valid during the call; a kept one is cloned.
		Global: func(name uint32, iface string, version uint32) {
			switch iface {
			case "wl_seat":
				d.addSeat(name, version)
			case "wl_output":
				d.addOutput(name, version)
			case "wl_compositor", "xdg_wm_base", "wl_shm",
				"wp_cursor_shape_manager_v1", "wl_data_device_manager",
				"zwp_primary_selection_device_manager_v1",
				"zxdg_decoration_manager_v1", "wp_viewporter",
				"wp_fractional_scale_manager_v1", "zwp_text_input_manager_v3":
				if _, seen := globals[iface]; !seen {
					globals[strings.Clone(iface)] = wlGlobal{name, version}
				}
			}
		},
		GlobalRemove: d.removeGlobal,
	})
	if err = conn.Roundtrip(); err != nil {
		return d, err
	}
	bind := func(iface *wl.Interface, key string) wl.Proxy {
		g, ok := globals[key]
		if !ok {
			return wl.Proxy{}
		}
		return d.registry.Bind(g.name, iface, min(g.version, iface.Version()))
	}
	d.compositor = wl.Compositor{Proxy: bind(&wl.CompositorInterface, "wl_compositor")}
	d.wmBase = wl.XdgWmBase{Proxy: bind(&wl.XdgWmBaseInterface, "xdg_wm_base")}
	if !d.compositor.Valid() || !d.wmBase.Valid() {
		return d, errors.New("compositor offers no wl_compositor or xdg_wm_base")
	}
	d.shm = wl.Shm{Proxy: bind(&wl.ShmInterface, "wl_shm")}
	d.cursorShape = wl.WpCursorShapeManagerV1{Proxy: bind(
		&wl.WpCursorShapeManagerV1Interface, "wp_cursor_shape_manager_v1")}
	d.dataMgr = wl.DataDeviceManager{Proxy: bind(
		&wl.DataDeviceManagerInterface, "wl_data_device_manager")}
	d.primaryMgr = wl.ZwpPrimarySelectionDeviceManagerV1{Proxy: bind(
		&wl.ZwpPrimarySelectionDeviceManagerV1Interface,
		"zwp_primary_selection_device_manager_v1")}
	d.decoMgr = wl.ZxdgDecorationManagerV1{Proxy: bind(
		&wl.ZxdgDecorationManagerV1Interface, "zxdg_decoration_manager_v1")}
	d.viewporter = wl.WpViewporter{Proxy: bind(&wl.WpViewporterInterface, "wp_viewporter")}
	d.fracMgr = wl.WpFractionalScaleManagerV1{Proxy: bind(
		&wl.WpFractionalScaleManagerV1Interface, "wp_fractional_scale_manager_v1")}
	d.textInputMgr = wl.ZwpTextInputManagerV3{Proxy: bind(
		&wl.ZwpTextInputManagerV3Interface, "zwp_text_input_manager_v3")}
	if d.seat != nil {
		// The seat was bound during the round trip, before the managers
		// its selection devices come from.
		d.seat.attachSelection()
		d.seat.attachTextInput()
	}
	// A client that misses pings is marked unresponsive by the compositor.
	d.wmBase.SetHandlers(wl.XdgWmBaseHandlers{Ping: d.wmBase.Pong})

	// libwayland-egl and libxkbcommon are checked here, not at the first
	// window, so a system without them falls back to X11 instead of
	// failing New, or running with a dead keyboard.
	if err = xkb.Load(); err != nil {
		return d, err
	}
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

// dispatch is Conn.Dispatch for the backend: it also raises a panic a
// libdecor callback recovered during the dispatch.
func (d *wlDisplay) dispatch(timeout time.Duration) error {
	err := d.conn.Dispatch(timeout)
	decor.Rethrow()
	return err
}

// post runs fn on the loop's thread, on its next pass. Any goroutine.
func (d *wlDisplay) post(fn func()) {
	d.callsMu.Lock()
	d.calls = append(d.calls, fn)
	d.callsMu.Unlock()
	d.conn.Wake()
}

// runPosted runs the work post queued. Main thread.
func (d *wlDisplay) runPosted() {
	d.callsMu.Lock()
	calls := d.calls
	d.calls = nil
	d.callsMu.Unlock()
	for _, fn := range calls {
		fn()
	}
}

// wlGlobal is a registry global's name and version.
type wlGlobal struct{ name, version uint32 }

// addSeat binds a wl_seat global unless a seat is bound already. The
// registry handlers stay installed, so a seat that appears later (the
// first one, or one replacing a removed seat) is bound then.
func (d *wlDisplay) addSeat(name, version uint32) {
	if d.seat != nil {
		return
	}
	d.seat = newWlSeat(d, name, version)
	d.seatName = name
	d.seat.attachSelection()
	d.seat.attachTextInput()
}

// removeGlobal handles wl_registry.global_remove. Only the seat and the
// outputs matter: the seat's devices are released and a later wl_seat
// global can take its place; an output leaves every window on it.
func (d *wlDisplay) removeGlobal(name uint32) {
	if o, ok := d.outputs[name]; ok {
		d.removeOutput(o)
		return
	}
	if d.seat == nil || name != d.seatName {
		return
	}
	d.seat.destroy()
	d.seat, d.seatName = nil, 0
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
	if d.seat != nil {
		d.seat.destroy()
		d.seat = nil
	}
	for _, o := range d.outputs {
		o.destroy()
	}
	// After the windows, whose frames it made.
	d.decor.Unref()
	d.destroyOptionalGlobals()
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

// destroyOptionalGlobals frees the phase 6 globals, with the destructor
// request each has, so the compositor frees its side too.
func (d *wlDisplay) destroyOptionalGlobals() {
	if d.shm.Valid() {
		if d.shm.Version() >= 2 {
			d.shm.Release()
		} else {
			d.shm.DestroyProxy()
		}
	}
	if d.cursorShape.Valid() {
		d.cursorShape.Destroy()
	}
	if d.dataMgr.Valid() {
		d.dataMgr.DestroyProxy() // the interface has no destructor
	}
	if d.primaryMgr.Valid() {
		d.primaryMgr.Destroy()
	}
	if d.decoMgr.Valid() {
		d.decoMgr.Destroy()
	}
	if d.viewporter.Valid() {
		d.viewporter.Destroy()
	}
	if d.fracMgr.Valid() {
		d.fracMgr.Destroy()
	}
	if d.textInputMgr.Valid() {
		d.textInputMgr.Destroy()
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
	// deco is the xdg-decoration object, when the compositor draws the
	// frame. frame is the libdecor frame, when libdecor draws it; it
	// then owns the xdg_surface and xdg_toplevel, and the two fields
	// above stay zero. See wayland_shell_linux.go.
	deco  wl.ZxdgToplevelDecorationV1
	frame *decor.Frame

	// pendW, pendH, pendActive and pendSuspended hold what the last
	// xdg_toplevel.configure asked for. A size of 0 leaves the choice to
	// the client. They take effect on the xdg_surface.configure that
	// closes the sequence.
	pendW, pendH  int32
	pendActive    bool
	pendSuspended bool

	// logW, logH are the window size in logical pixels; scale120 is the
	// scale in 120ths (fractional-scale-v1's unit, so 180 is 1.5). The
	// buffer is logW×logH scaled by it, rounded (wlScaled).
	logW, logH int32
	scale120   int32

	// fracScale and viewport are set when the compositor offers
	// fractional scaling: the buffer then keeps scale 1 and the viewport
	// shows it at the logical size. Without them, the scale is the
	// integer wl_surface buffer scale.
	fracScale wl.WpFractionalScaleV1
	viewport  wl.WpViewport
	// outputs are the outputs the surface is on (wl_surface.enter and
	// .leave). outputScale is set when the compositor sends no scale of
	// its own (no fractional scale, wl_surface before v6): the scale then
	// comes from these outputs (wayland_output_linux.go).
	outputs     []*wlOutput
	outputScale bool

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
	// stalled: frameCb outlived wlFrameTimeout (wayland_occlusion_linux.go).
	throttle  bool
	stalled   bool
	frameCb   wl.Callback
	frameAt   time.Time
	frameDone wl.CallbackDispatcher

	// imeOn is gui's IMEStart (an editable text has focus); imeRect is
	// the caret it last reported, in logical pixels.
	imeOn       bool
	imeRect     [4]int32
	imeHaveRect bool

	// title is a title set from another goroutine, applied by the loop.
	// The wl package is single-threaded, so SetTitle cannot send it.
	titleMu    sync.Mutex
	title      string
	titleDirty bool
}

// wlMaxWindowSize bounds a window side, in logical pixels here and in
// buffer pixels in resize, which also scales the logical bound down so
// logical × scale stays under it. A broken compositor (or a huge
// WindowCfg size) must not make the buffer overflow or ask EGL for
// gigabytes: 16384² at 4 bytes is 1 GiB, the most it can ask for.
const wlMaxWindowSize = 16384

// wlInitialSize is the logical size a window starts at: cfg's, or
// 640×480 for an unset or negative side, bounded by wlMaxWindowSize.
// Clamped as ints, before the conversion: a size outside int32 would
// wrap, and a wrapped negative one could turn positive.
func wlInitialSize(w, h int) (int32, int32) {
	side := func(v, def int) int32 {
		if v <= 0 {
			return int32(def)
		}
		return int32(min(v, wlMaxWindowSize))
	}
	return side(w, 640), side(h, 480)
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
	return min(max(pendW, 1), wlMaxWindowSize), min(max(pendH, 1), wlMaxWindowSize)
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
	ww := &wlWindow{d: d, b: b, scale120: 120, throttle: !cfg.VSyncOff,
		transparent: cfg.Transparent}
	b.plat.wl = ww
	b.plat.w = w
	// At most one frame callback is outstanding (requestFrame), so the
	// one that fires is always ww.frameCb.
	ww.frameDone = wl.CallbackHandlers{Done: func(uint32) {
		ww.frameCb.DestroyProxy()
		ww.frameCb = wl.Callback{}
		ww.setStalled(false) // the compositor paints the window again
	}}.Dispatcher()
	fail := func(err error) (*Backend, error) {
		b.plat.destroy()
		return nil, err
	}

	ww.logW, ww.logH = wlInitialSize(cfg.Width, cfg.Height)
	b.plat.physW, b.plat.physH, b.plat.scale = ww.logW, ww.logH, 1

	ww.surface = d.compositor.CreateSurface()
	// Enter and leave matter only where the compositor sends no scale of
	// its own (outputScale, set below); they are tracked regardless.
	ww.surface.SetHandlers(wl.SurfaceHandlers{
		PreferredBufferScale: ww.setScale,
		Enter: func(px wl.Output) {
			if o := d.outputFor(px); o != nil {
				ww.enterOutput(o)
			}
		},
		Leave: func(px wl.Output) {
			if o := d.outputFor(px); o != nil {
				ww.leaveOutput(o)
			}
		},
	})
	ww.outputScale = ww.surface.Version() < 6
	if d.fracMgr.Valid() && d.viewporter.Valid() {
		ww.outputScale = false
		ww.viewport = d.viewporter.GetViewport(ww.surface)
		ww.viewport.SetDestination(ww.logW, ww.logH)
		ww.fracScale = d.fracMgr.GetFractionalScale(ww.surface)
		ww.fracScale.SetHandlers(wl.WpFractionalScaleV1Handlers{PreferredScale: ww.setFracScale})
	}
	// The window's role: an xdg_toplevel, made by libdecor when it draws
	// the frame. Its first commit carries no buffer; it asks for the
	// initial configure. No buffer may be attached before that configure
	// is acknowledged, so EGL is set up only after it.
	if err := ww.makeToplevel(cfg); err != nil {
		return fail(err)
	}
	deadline := time.Now().Add(wlConfigureTimeout)
	for !ww.configured {
		if time.Now().After(deadline) {
			return fail(errors.New("wayland: no configure from the compositor"))
		}
		if err := d.dispatch(100 * time.Millisecond); err != nil {
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
	w.SetClipboardFn(func(s string) { d.setSelection(b, wlSelClipboard, s) })
	w.SetClipboardGetFn(func() string { return d.getSelection(b, wlSelClipboard) })
	w.SetPrimaryFn(func(s string) { d.setSelection(b, wlSelPrimary, s) })
	w.SetPrimaryGetFn(func() string { return d.getSelection(b, wlSelPrimary) })

	d.wins[ww.surface.Ptr()] = b
	b.plat.lockedTid = syscall.Gettid()
	handedOff = true
	return b, nil
}

// id is the window's key in gui.App: the surface's protocol object id.
func (ww *wlWindow) id() uint32 { return ww.surface.ID() }

// applyConfigure handles xdg_surface.configure: acknowledge it, then apply
// the size, focus and visibility the toplevel configure before it carried.
// The compositor expects a new buffer in reply, so the window is marked
// dirty. A suspending configure still draws that one frame; after it,
// FrameFn reports nothing to draw while suspended, so the loop no longer
// renders the one frame per wlFrameTimeout a hidden window used to get.
func (ww *wlWindow) applyConfigure(serial uint32) {
	ww.xdgSurface.AckConfigure(serial)
	ww.applyPending(configureSize(ww.pendW, ww.pendH, ww.logW, ww.logH))
}

// applyPending is the part of a configure both paths share, after their
// own acknowledgement (ack_configure, or libdecor's frame commit): adopt
// the size, then send gui the pending focus and visibility. One function,
// so a state added to one path cannot be forgotten in the other, and so
// tests can drive it without a live xdg_surface.
func (ww *wlWindow) applyPending(w, h int32) {
	ww.configured = true
	ww.resize(w, h, ww.scale120)
	ww.syncFocus()
	ww.syncOccluded()
	ww.dirty = true
}

// wlMaxScale120 bounds the scale a compositor can ask for (8×). A broken
// one must not make the buffer, sized logical × scale, huge.
const wlMaxScale120 = 8 * 120

// setScale handles wl_surface.preferred_buffer_scale (wl_surface v6), the
// integer scale. A window with fractional scaling ignores it: the
// fractional event says the same more precisely.
func (ww *wlWindow) setScale(factor int32) {
	if ww.fracScale.Valid() {
		return
	}
	ww.resize(ww.logW, ww.logH, min(max(factor, 1), wlMaxScale120/120)*120)
	ww.dirty = true
}

// setFracScale handles wp_fractional_scale_v1.preferred_scale, in 120ths.
func (ww *wlWindow) setFracScale(scale uint32) {
	ww.resize(ww.logW, ww.logH, int32(min(max(scale, 120), wlMaxScale120)))
	ww.dirty = true
}

// wlScaled is a logical length in buffer pixels at scale120, rounded half
// up as fractional-scale-v1 asks.
func wlScaled(v, scale120 int32) int32 {
	return int32((int64(v)*int64(scale120) + 60) / 120)
}

// wlBoundSize caps a logical size so its buffer, logical × scale, stays
// within wlMaxWindowSize on each side. scale120 is at least 120.
func wlBoundSize(w, h, scale120 int32) (int32, int32) {
	limit := wlMaxWindowSize * 120 / max(scale120, 120)
	return min(w, limit), min(h, limit)
}

// resize adopts a new logical size and scale. Before the window is ready
// it only records them; New sizes the EGL window from the result.
func (ww *wlWindow) resize(w, h, scale120 int32) {
	// Every scale passes here, so GOGUI_DEVICE_SCALE replaces it here (#971).
	scale120 = min(devscale.Apply120(scale120, !ww.viewport.Valid()), wlMaxScale120)
	w, h = wlBoundSize(w, h, scale120)
	if w == ww.logW && h == ww.logH && scale120 == ww.scale120 {
		return
	}
	// Both apply with the next commit, which is the eglSwapBuffers of a
	// buffer already at the new size.
	switch {
	case ww.viewport.Valid():
		// The buffer stays at scale 1; the viewport maps it onto the
		// logical size.
		if w != ww.logW || h != ww.logH {
			ww.viewport.SetDestination(w, h)
		}
	case scale120 != ww.scale120 && ww.surface.Valid() && ww.surface.Version() >= 3:
		ww.surface.SetBufferScale(scale120 / 120)
	}
	ww.logW, ww.logH, ww.scale120 = w, h, scale120
	b := ww.b
	b.plat.physW, b.plat.physH = wlScaled(w, scale120), wlScaled(h, scale120)
	b.plat.scale = float32(scale120) / 120
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
		ww.setRoleTitle(t)
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
// window), it stays outstanding, and its done is what ends the stall.
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
	delete(ww.d.wins, ww.surface.Ptr())
	if ww.d.seat != nil {
		ww.d.seat.forget(ww.b)
	}
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
	if ww.fracScale.Valid() {
		ww.fracScale.Destroy()
	}
	if ww.viewport.Valid() {
		ww.viewport.Destroy()
	}
	ww.destroyRole()
	if ww.surface.Valid() {
		ww.surface.Destroy()
	}
	ww.d.release()
}
