//go:build linux && !js && !android

package gl

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// Selected X11 event mask for top-level windows.
const x11EventMask = xproto.EventMaskKeyPress | xproto.EventMaskKeyRelease |
	xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease |
	xproto.EventMaskPointerMotion | xproto.EventMaskExposure |
	xproto.EventMaskStructureNotify | xproto.EventMaskFocusChange

// X11 cursor-font glyph indices (from cursorfont.h). Each shape is two
// consecutive glyphs: the image and its mask.
const (
	xcLeftPtr           = 68
	xcXterm             = 152
	xcCrosshair         = 34
	xcHand2             = 60
	xcSbHDoubleArrow    = 108
	xcSbVDoubleArrow    = 116
	xcFleur             = 52
	xcBottomRightCorner = 14
	xcBottomLeftCorner  = 12
	xcXCursor           = 0
)

// platformState holds the X11 windowing + EGL state for the GL backend.
type platformState struct {
	conn     *xgb.Conn
	wakeConn *xgb.Conn
	window   xproto.Window

	eglDpy     uintptr
	eglConfig  uintptr
	eglSurface uintptr
	eglContext uintptr

	cursors   [11]xproto.Cursor
	curCursor xproto.Cursor

	wmDelete   xproto.Atom
	wakeAtom   xproto.Atom
	keymap     *xproto.GetKeyboardMappingReply
	minKeycode xproto.Keycode

	// Clipboard (X11 CLIPBOARD selection).
	atomClipboard xproto.Atom
	atomUTF8      xproto.Atom
	atomTargets   xproto.Atom
	atomClipProp  xproto.Atom
	clipboardText string
	ownsClipboard bool
	clipReadConn  *xgb.Conn     // dedicated connection for reads
	clipReadWin   xproto.Window // requestor window on clipReadConn

	// Per-monitor DPI (RandR). root anchors monitor queries; curCrtc is
	// the CRTC the window currently sits on; lastRootXY caches the last
	// root-relative position so ConfigureNotify only rescans on a move.
	root        xproto.Window
	haveRandr   bool
	curCrtc     randr.Crtc
	lastRootX   int16
	lastRootY   int16
	haveLastPos bool

	physW, physH int32
	scale        float32

	w   *gui.Window
	evt gui.Event // reused per event to avoid per-event allocation
}

func (p *platformState) makeCurrent() {
	eglMakeCurrent(p.eglDpy, p.eglSurface, p.eglSurface, p.eglContext)
}

func (p *platformState) swap() { eglSwapBuffers(p.eglDpy, p.eglSurface) }

func (p *platformState) drawableSize() (int32, int32) { return p.physW, p.physH }

func (p *platformState) dpiScale() float32 {
	if p.scale <= 0 {
		return 1
	}
	return p.scale
}

func (p *platformState) setCursor(mc gui.MouseCursor) {
	if int(mc) >= len(p.cursors) {
		return
	}
	c := p.cursors[mc]
	if c == 0 {
		c = p.cursors[gui.CursorDefault]
	}
	if c == p.curCursor {
		return
	}
	p.curCursor = c
	xproto.ChangeWindowAttributes(p.conn, p.window,
		xproto.CwCursor, []uint32{uint32(c)})
}

// wake sends a no-op ClientMessage to our own window from a second
// connection so the event-pump goroutine unblocks from WaitForEvent.
func (p *platformState) wake() {
	if p.wakeConn == nil {
		return
	}
	ev := xproto.ClientMessageEvent{
		Format: 32,
		Window: p.window,
		Type:   p.wakeAtom,
		Data:   xproto.ClientMessageDataUnionData32New([]uint32{0, 0, 0, 0, 0}),
	}
	cookie := xproto.SendEventChecked(p.wakeConn, false, p.window, 0, string(ev.Bytes()))
	_ = cookie.Check() // forces a flush; error is not actionable here
}

func (p *platformState) destroy() {
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
		eglTerminate(p.eglDpy)
		p.eglDpy = 0
	}
	if p.conn != nil && p.window != 0 {
		xproto.DestroyWindow(p.conn, p.window)
		p.window = 0
	}
	if p.wakeConn != nil {
		p.wakeConn.Close()
		p.wakeConn = nil
	}
	if p.clipReadConn != nil {
		p.clipReadConn.Close()
		p.clipReadConn = nil
	}
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}

// pumpEvents reads X events on a dedicated goroutine and forwards them
// on ch. It closes ch when the connection ends so the main loop exits.
func (p *platformState) pumpEvents(ch chan<- xgb.Event) {
	for {
		ev, err := p.conn.WaitForEvent()
		if ev == nil && err == nil {
			close(ch) // connection closed
			return
		}
		if ev != nil {
			ch <- ev
		}
	}
}

// New creates an OpenGL 3.3 backend backed by a native X11 window and an
// EGL context. Pure Go via xgb + purego — no SDL2, no cgo.
func New(w *gui.Window) (*Backend, error) {
	runtime.LockOSThread()

	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("gl: x11 connect: %w", err)
	}
	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)

	dpy, config, visualID, err := eglInitDisplay()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("gl: %w", err)
	}

	cfg := w.Config
	title := cfg.Title
	if title == "" {
		title = "go-gui"
	}
	width := int32(cfg.Width)
	if width <= 0 {
		width = 640
	}
	height := int32(cfg.Height)
	if height <= 0 {
		height = 480
	}

	haveRandr := randr.Init(conn) == nil
	// The window is created at 0,0, so its initial monitor is whichever
	// CRTC covers the origin.
	scale, crtc := dpiScaleForWindow(conn, screen.Root, haveRandr, 0, 0)
	physW := int32(float32(width) * scale)
	physH := int32(float32(height) * scale)

	depth := screen.RootDepth
	for _, d := range screen.AllowedDepths {
		for _, v := range d.Visuals {
			if uint32(v.VisualId) == visualID {
				depth = d.Depth
			}
		}
	}

	cmID, err := conn.NewId()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("gl: x11 new id: %w", err)
	}
	xproto.CreateColormap(conn, xproto.ColormapAllocNone,
		xproto.Colormap(cmID), screen.Root, xproto.Visualid(visualID))

	wid, err := conn.NewId()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("gl: x11 new id: %w", err)
	}
	win := xproto.Window(wid)
	xproto.CreateWindow(conn, depth, win, screen.Root,
		0, 0, uint16(physW), uint16(physH), 0,
		xproto.WindowClassInputOutput, xproto.Visualid(visualID),
		xproto.CwBorderPixel|xproto.CwEventMask|xproto.CwColormap,
		[]uint32{0, x11EventMask, cmID})

	b := &Backend{}
	b.plat.conn = conn
	b.plat.window = win
	b.plat.eglDpy = dpy
	b.plat.eglConfig = config
	b.plat.scale = scale
	b.plat.physW = physW
	b.plat.physH = physH
	b.plat.root = screen.Root
	b.plat.haveRandr = haveRandr
	b.plat.curCrtc = crtc

	setWindowTitle(conn, win, title)
	b.plat.wmDelete = setupCloseProtocol(conn, win)
	b.plat.wakeAtom = internAtom(conn, "_GOGUI_WAKE")
	b.plat.atomClipboard = internAtom(conn, "CLIPBOARD")
	b.plat.atomUTF8 = internAtom(conn, "UTF8_STRING")
	b.plat.atomTargets = internAtom(conn, "TARGETS")
	b.plat.atomClipProp = internAtom(conn, "_GOGUI_CLIPBOARD")
	b.plat.minKeycode = setup.MinKeycode
	b.plat.keymap = loadKeymap(conn, setup)

	loadCursors(&b.plat)
	xproto.MapWindow(conn, win)

	// Flush all pending X requests and wait for the server to realize
	// the window before EGL (on its own X connection) wraps it.
	conn.Sync()

	surface, context, err := eglCreateSurfaceContext(dpy, config, uint32(wid))
	if err != nil {
		b.plat.destroy()
		return nil, fmt.Errorf("gl: %w", err)
	}
	b.plat.eglSurface = surface
	b.plat.eglContext = context

	if err := gl.InitWithProcAddrFunc(eglProc); err != nil {
		b.plat.destroy()
		return nil, fmt.Errorf("gl: gl.Init: %w", err)
	}

	wakeConn, err := xgb.NewConn()
	if err != nil {
		b.plat.destroy()
		return nil, fmt.Errorf("gl: x11 wake connect: %w", err)
	}
	b.plat.wakeConn = wakeConn

	b.dpiScale = scale
	b.physW = physW
	b.physH = physH
	b.initCaches(cfg)

	if err := b.initGLResources(w); err != nil {
		b.Destroy()
		return nil, fmt.Errorf("gl: initGLResources: %w", err)
	}

	w.SetTitleFn(func(t string) { setWindowTitle(conn, win, t) })
	w.SetClipboardFn(func(s string) { setClipboard(&b.plat, s) })
	w.SetClipboardGetFn(func() string { return getClipboard(&b.plat) })

	return b, nil
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	b.destroyGLResources()
	b.plat.destroy()
}

// Run starts the event loop. Blocks until the window is closed.
func (b *Backend) Run(w *gui.Window) {
	defer w.WindowCleanup()
	b.plat.w = w
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	w.SetWakeMainFn(b.plat.wake)

	events := make(chan xgb.Event, 64)
	go b.plat.pumpEvents(events)

	// drain processes every currently-queued event without blocking.
	// Returns false when the connection has closed.
	drain := func() bool {
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					return false
				}
				b.handleXEvent(ev)
			default:
				return true
			}
		}
	}

	var rendered bool
	running := true
	for running {
		if !drain() || w.CloseRequested() {
			break
		}
		rendered = w.FrameFn()
		if rendered {
			b.renderFrame(w)
		}
		b.plat.setCursor(w.MouseCursorState())
		if !rendered {
			select {
			case ev, ok := <-events:
				if !ok {
					running = false
				} else {
					b.handleXEvent(ev)
				}
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

// Run initializes the backend, runs the event loop, and cleans up on
// exit. Panics on error; call RunE for the error-returning variant.
func Run(w *gui.Window) {
	if err := RunE(w); err != nil {
		panic(fmt.Sprintf("gl: %v", err))
	}
}

// RunE initializes the backend, runs the event loop, and cleans up on
// exit. Returns an error instead of panicking.
func RunE(w *gui.Window) error {
	b, err := New(w)
	if err != nil {
		return fmt.Errorf("gl: %w", err)
	}
	defer b.Destroy()
	b.Run(w)
	return nil
}

// RunApp starts a multi-window event loop. Panics on error; call
// RunAppE for the error-returning variant.
func RunApp(app *gui.App, initialWindows ...*gui.Window) {
	if err := RunAppE(app, initialWindows...); err != nil {
		panic(fmt.Sprintf("gl: %v", err))
	}
}

// taggedEvent carries an X event alongside the backend it belongs to so
// a single channel can multiplex several windows' event pumps.
type taggedEvent struct {
	b      *Backend
	ev     xgb.Event
	closed bool
}

// RunAppE starts a multi-window event loop. Each window is created and
// registered with app, keyed by its X window id. Blocks until the last
// window closes.
//
//nolint:gocyclo // backend event loop
func RunAppE(app *gui.App, initialWindows ...*gui.Window) error {
	runtime.LockOSThread()

	backends := make(map[uint32]*Backend) // window XID → backend
	events := make(chan taggedEvent, 128)

	open := func(w *gui.Window) error {
		b, err := New(w)
		if err != nil {
			return err
		}
		b.plat.w = w
		id := uint32(b.plat.window)
		backends[id] = b
		app.Register(id, w)
		w.SetWakeMainFn(b.plat.wake)
		if w.Config.OnInit != nil {
			w.Config.OnInit(w)
		}
		go func(bk *Backend) {
			ch := make(chan xgb.Event, 64)
			go bk.plat.pumpEvents(ch)
			for ev := range ch {
				events <- taggedEvent{b: bk, ev: ev}
			}
			events <- taggedEvent{b: bk, closed: true}
		}(b)
		return nil
	}

	for _, w := range initialWindows {
		if err := open(w); err != nil {
			for _, b := range backends {
				b.Destroy()
			}
			return fmt.Errorf("gl: create window: %w", err)
		}
	}

	closeWindow := func(b *Backend) bool {
		id := uint32(b.plat.window)
		if w := app.Window(id); w != nil {
			w.WindowCleanup()
		}
		b.Destroy()
		delete(backends, id)
		return app.Unregister(id)
	}

	var rendered bool
	for len(backends) > 0 {
		// Drain queued events + window opens.
		drained := false
		for !drained {
			select {
			case te := <-events:
				if te.closed {
					continue
				}
				te.b.handleXEvent(te.ev)
			case cfg := <-app.PendingOpen():
				if err := open(gui.NewWindow(cfg)); err != nil {
					log.Printf("gl: open window: %v", err)
				}
			default:
				drained = true
			}
		}

		// Handle close requests.
		for _, b := range backends {
			w := app.Window(uint32(b.plat.window))
			if w == nil || !w.CloseRequested() {
				continue
			}
			if closeWindow(b) {
				return nil // last window closed
			}
		}
		if len(backends) == 0 {
			return nil
		}

		// Frame + render each window.
		rendered = false
		for _, b := range backends {
			w := app.Window(uint32(b.plat.window))
			if w == nil {
				continue
			}
			if w.FrameFn() {
				b.renderFrame(w)
				rendered = true
			}
			b.plat.setCursor(w.MouseCursorState())
		}

		if !rendered {
			select {
			case te := <-events:
				if !te.closed {
					te.b.handleXEvent(te.ev)
				}
			case cfg := <-app.PendingOpen():
				if err := open(gui.NewWindow(cfg)); err != nil {
					log.Printf("gl: open window: %v", err)
				}
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	return nil
}

// --- helpers ---

func internAtom(conn *xgb.Conn, name string) xproto.Atom {
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil || reply == nil {
		return 0
	}
	return reply.Atom
}

func setWindowTitle(conn *xgb.Conn, win xproto.Window, title string) {
	xproto.ChangeProperty(conn, xproto.PropModeReplace, win,
		xproto.AtomWmName, xproto.AtomString, 8,
		uint32(len(title)), []byte(title))
}

// setupCloseProtocol registers WM_DELETE_WINDOW so the window manager's
// close button delivers a ClientMessage instead of killing the client.
func setupCloseProtocol(conn *xgb.Conn, win xproto.Window) xproto.Atom {
	protocols := internAtom(conn, "WM_PROTOCOLS")
	del := internAtom(conn, "WM_DELETE_WINDOW")
	if protocols == 0 || del == 0 {
		return del
	}
	buf := []byte{
		byte(del), byte(del >> 8), byte(del >> 16), byte(del >> 24),
	}
	xproto.ChangeProperty(conn, xproto.PropModeReplace, win,
		protocols, xproto.AtomAtom, 32, 1, buf)
	return del
}

func loadKeymap(conn *xgb.Conn, setup *xproto.SetupInfo) *xproto.GetKeyboardMappingReply {
	count := byte(setup.MaxKeycode - setup.MinKeycode + 1)
	km, err := xproto.GetKeyboardMapping(conn, setup.MinKeycode, count).Reply()
	if err != nil {
		return nil
	}
	return km
}

func loadCursors(p *platformState) {
	fid, err := p.conn.NewId()
	if err != nil {
		return
	}
	font := xproto.Font(fid)
	xproto.OpenFont(p.conn, font, uint16(len("cursor")), "cursor")

	load := func(glyph uint16) xproto.Cursor {
		cid, cerr := p.conn.NewId()
		if cerr != nil {
			return 0
		}
		c := xproto.Cursor(cid)
		xproto.CreateGlyphCursor(p.conn, c, font, font,
			glyph, glyph+1,
			0, 0, 0, 0xffff, 0xffff, 0xffff)
		return c
	}
	p.cursors[gui.CursorDefault] = load(xcLeftPtr)
	p.cursors[gui.CursorArrow] = load(xcLeftPtr)
	p.cursors[gui.CursorIBeam] = load(xcXterm)
	p.cursors[gui.CursorCrosshair] = load(xcCrosshair)
	p.cursors[gui.CursorPointingHand] = load(xcHand2)
	p.cursors[gui.CursorResizeEW] = load(xcSbHDoubleArrow)
	p.cursors[gui.CursorResizeNS] = load(xcSbVDoubleArrow)
	p.cursors[gui.CursorResizeNWSE] = load(xcBottomRightCorner)
	p.cursors[gui.CursorResizeNESW] = load(xcBottomLeftCorner)
	p.cursors[gui.CursorResizeAll] = load(xcFleur)
	p.cursors[gui.CursorNotAllowed] = load(xcXCursor)
}

// readDPIScale derives a UI scale from the Xft.dpi entry in the root
// RESOURCE_MANAGER property, falling back to 1.0.
func readDPIScale(conn *xgb.Conn, root xproto.Window) float32 {
	atom := internAtom(conn, "RESOURCE_MANAGER")
	if atom == 0 {
		return 1
	}
	reply, err := xproto.GetProperty(conn, false, root, atom,
		xproto.AtomString, 0, 1<<16).Reply()
	if err != nil || reply == nil || len(reply.Value) == 0 {
		return 1
	}
	for line := range strings.SplitSeq(string(reply.Value), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "Xft.dpi" {
			continue
		}
		dpi, perr := strconv.Atoi(strings.TrimSpace(val))
		if perr == nil && dpi > 0 {
			return float32(dpi) / 96.0
		}
	}
	return 1
}

// Plausible bounds for a physical display DPI. Values outside this range
// usually mean a bogus EDID physical size, so the RandR path is rejected
// in favor of the Xft.dpi fallback.
const (
	minPlausibleDPI = 50
	maxPlausibleDPI = 400
)

// dpiScaleForWindow computes the UI scale for the monitor containing the
// root-relative point (x,y), using that monitor's RandR-reported physical
// size. It falls back to the global Xft.dpi scale when RandR is
// unavailable or reports no usable physical size. Returns the scale and
// the CRTC the point lands on (0 when none was resolved).
func dpiScaleForWindow(conn *xgb.Conn, root xproto.Window, haveRandr bool, x, y int32) (float32, randr.Crtc) {
	if haveRandr {
		if s, crtc, ok := randrDPIScale(conn, root, x, y); ok {
			return s, crtc
		}
	}
	return readDPIScale(conn, root), 0
}

// randrDPIScale finds the CRTC covering (x,y) and derives a UI scale from
// its output's physical size. ok is false when RandR data is missing or
// implausible.
func randrDPIScale(conn *xgb.Conn, root xproto.Window, x, y int32) (float32, randr.Crtc, bool) {
	res, err := randr.GetScreenResourcesCurrent(conn, root).Reply()
	if err != nil || res == nil {
		return 0, 0, false
	}
	for _, crtc := range res.Crtcs {
		info, ierr := randr.GetCrtcInfo(conn, crtc, res.ConfigTimestamp).Reply()
		if ierr != nil || info == nil || info.Width == 0 || info.Height == 0 {
			continue // disabled/disconnected CRTC
		}
		if x < int32(info.X) || x >= int32(info.X)+int32(info.Width) ||
			y < int32(info.Y) || y >= int32(info.Y)+int32(info.Height) {
			continue
		}
		if len(info.Outputs) == 0 {
			return 0, 0, false
		}
		out, oerr := randr.GetOutputInfo(conn, info.Outputs[0], res.ConfigTimestamp).Reply()
		if oerr != nil || out == nil {
			return 0, 0, false
		}
		if dpi, ok := crtcDPI(info, out); ok {
			return float32(dpi / 96.0), crtc, true
		}
		return 0, 0, false
	}
	return 0, 0, false
}

// crtcDPI averages the horizontal and vertical DPI from the CRTC pixel
// size and the output's physical millimetre size. ok is false when no
// physical dimension is reported or the result is implausible.
func crtcDPI(info *randr.GetCrtcInfoReply, out *randr.GetOutputInfoReply) (float64, bool) {
	const mmPerInch = 25.4
	// A 90/270° rotation swaps the pixel axes relative to physical size.
	pw, ph := float64(info.Width), float64(info.Height)
	if info.Rotation&(randr.RotationRotate90|randr.RotationRotate270) != 0 {
		pw, ph = ph, pw
	}
	var sum float64
	var n int
	if out.MmWidth > 0 {
		sum += pw / (float64(out.MmWidth) / mmPerInch)
		n++
	}
	if out.MmHeight > 0 {
		sum += ph / (float64(out.MmHeight) / mmPerInch)
		n++
	}
	if n == 0 {
		return 0, false
	}
	dpi := sum / float64(n)
	if dpi < minPlausibleDPI || dpi > maxPlausibleDPI {
		return 0, false
	}
	return dpi, true
}

// maybeRescaleDPI re-evaluates the per-monitor scale when the window has
// moved to a CRTC with a different DPI, updating plat.scale and the glyph
// backend. It reports whether the scale changed so the caller can trigger
// a relayout. ConfigureNotify coordinates are frame-relative under a
// reparenting WM, so the true root position is queried explicitly and a
// RandR rescan runs only when that position changed.
func (b *Backend) maybeRescaleDPI() bool {
	if !b.plat.haveRandr {
		return false
	}
	t, err := xproto.TranslateCoordinates(b.plat.conn, b.plat.window,
		b.plat.root, 0, 0).Reply()
	if err != nil || t == nil {
		return false
	}
	if b.plat.haveLastPos && t.DstX == b.plat.lastRootX &&
		t.DstY == b.plat.lastRootY {
		return false // no move → still on the same monitor
	}
	b.plat.lastRootX, b.plat.lastRootY = t.DstX, t.DstY
	b.plat.haveLastPos = true

	scale, crtc := dpiScaleForWindow(b.plat.conn, b.plat.root, true,
		int32(t.DstX), int32(t.DstY))
	b.plat.curCrtc = crtc
	if scale == b.plat.scale {
		return false
	}
	b.plat.scale = scale
	if b.glyphBack != nil {
		b.glyphBack.dpiScale = scale
	}
	return true
}
