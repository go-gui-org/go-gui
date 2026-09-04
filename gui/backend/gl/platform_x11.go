//go:build linux && !js && !android

package gl

import (
	"fmt"
	"runtime"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"

	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ibus"
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

// New creates an OpenGL 3.3 backend backed by a native X11 window and an
// EGL context. Pure Go via xgb + purego — no cgo.
// exportaudit:keep — lowercase new shadows the Go builtin
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
	// Hints are in physical pixels, matching the CreateWindow call.
	b.plat.limits = gui.WindowSizeLimits(cfg)
	setSizeHints(conn, win, b.plat.limits.Scaled(scale))
	setWindowIcon(conn, win, cfg)
	setWindowDecorations(conn, win, cfg.Decorations)
	if cfg.WMClass != "" {
		setWMClass(conn, win, cfg.WMClass)
	}
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

	if err := gogl.InitWithProcAddrFunc(eglProc); err != nil {
		b.plat.destroy()
		return nil, fmt.Errorf("gl: glbind init: %w", err)
	}

	wakeConn, err := xgb.NewConn()
	if err != nil {
		b.plat.destroy()
		return nil, fmt.Errorf("gl: x11 wake connect: %w", err)
	}
	b.plat.wakeConn = wakeConn

	// Needs wakeConn and wakeAtom: the client calls wake from a D-Bus
	// goroutine to bring the event loop round to drainIME.
	b.plat.ime = ibus.New("go-gui", b.plat.wake)

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
	w.SetPrimaryFn(func(s string) { setPrimary(&b.plat, s) })
	w.SetPrimaryGetFn(func() string { return getPrimary(&b.plat) })

	return b, nil
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

// XSizeHints flag bits (Xutil.h). Only the two size bounds are used;
// the rest of the structure is left zeroed.
const (
	sizeHintPMinSize = 1 << 4
	sizeHintPMaxSize = 1 << 5
)

// sizeHintWords is the length of XSizeHints in 32-bit words. The
// property is written whole even though only a few words are set,
// because window managers read it by fixed offset.
const sizeHintWords = 18

// maxSizeHintExtent is the largest extent an X drawable coordinate can
// carry, since they are 16-bit signed. Stands in for "no maximum".
const maxSizeHintExtent = 1<<15 - 1

// setSizeHints writes WM_NORMAL_HINTS so the window manager enforces
// the resize bounds. X11 has no resizable style bit — this property is
// the only way to constrain a drag, which is also why it is what makes
// WindowCfg.FixedSize work here (WindowSizeLimits reports a fixed
// window as min == max).
//
// Sizes must be physical pixels, since that is the unit CreateWindow
// was given; the caller scales before calling. A window with nothing
// constrained writes no property at all, leaving the WM's defaults.
func setSizeHints(conn *xgb.Conn, win xproto.Window, limits gui.SizeLimits) {
	if limits.None() {
		return
	}
	words := sizeHintWordsFor(limits)

	buf := make([]byte, 0, sizeHintWords*4)
	for _, wd := range words {
		buf = append(buf,
			byte(wd), byte(wd>>8), byte(wd>>16), byte(wd>>24))
	}
	xproto.ChangeProperty(conn, xproto.PropModeReplace, win,
		xproto.AtomWmNormalHints, xproto.AtomWmSizeHints, 32,
		sizeHintWords, buf)
}

// sizeHintWordsFor lays the limits out in the XSizeHints word order.
// Split from setSizeHints so the flag bits and the word offsets -- the
// part a window manager reads positionally, and the easiest thing here
// to get wrong -- are testable without an X server.
func sizeHintWordsFor(limits gui.SizeLimits) [sizeHintWords]uint32 {
	var words [sizeHintWords]uint32
	// A flag covers both axes at once, so a one-axis limit still has to
	// say something about the other. For a minimum, zero already means
	// "no floor", so an unset axis is written as it stands.
	if limits.MinW > 0 || limits.MinH > 0 {
		words[0] |= sizeHintPMinSize
		words[5] = uint32(limits.MinW)
		words[6] = uint32(limits.MinH)
	}
	// A maximum has no such luck: zero there would read as "0 pixels"
	// and pin the window shut, so an unset axis needs maxHintOr.
	if limits.MaxW > 0 || limits.MaxH > 0 {
		words[0] |= sizeHintPMaxSize
		words[7] = maxHintOr(limits.MaxW)
		words[8] = maxHintOr(limits.MaxH)
	}
	return words
}

// maxHintOr substitutes a practically unbounded extent for an unset
// axis. X11 has no "no maximum" sentinel, and a zero would pin the
// window shut, so an unconstrained axis gets the largest size the
// 16-bit X drawable coordinate space can express. A set value is
// already inside that range: gui.WindowSizeLimits caps every bound at
// the same ceiling.
func maxHintOr(v int) uint32 {
	if v <= 0 {
		return maxSizeHintExtent
	}
	return uint32(v)
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

	// The core cursor font ignores the Xcursor size and never scales
	// for HiDPI (issue #453), so every shape prefers the desktop's
	// Xcursor theme, uploaded via the RENDER extension. The font glyph
	// is the per-shape fallback when no themed cursor is available (no
	// theme, no RENDER, parse error) — never a hard error.
	loadGlyph := func(glyph uint16) xproto.Cursor {
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

	// Theme and size are resolved once: each step reads X root-window
	// properties. The resolved size is already in device pixels — the
	// compositor/settings daemon publishes the scaled value to X
	// clients (mutter multiplies cursor-size by the scale factor for
	// XSETTINGS and XCURSOR_SIZE), and libXcursor applies no scale of
	// its own. Multiplying by p.scale here doubled the cursor on a
	// 200% desktop (issue #453 follow-up).
	theme := xcursorThemeName(p.conn, p.root)
	size := xcursorThemeSize(p.conn, p.root)
	load := func(names []string, glyph uint16) xproto.Cursor {
		if c := xcursorThemeCursor(p, theme, size, names); c != 0 {
			return c
		}
		return loadGlyph(glyph)
	}
	p.cursors[gui.CursorDefault] = load(leftPtrCursorNames, xcLeftPtr)
	p.cursors[gui.CursorArrow] = load(leftPtrCursorNames, xcLeftPtr)
	p.cursors[gui.CursorIBeam] = load(xtermCursorNames, xcXterm)
	p.cursors[gui.CursorCrosshair] = load(crosshairCursorNames, xcCrosshair)
	p.cursors[gui.CursorPointingHand] = load(handCursorNames, xcHand2)
	p.cursors[gui.CursorResizeEW] = load(hResizeCursorNames, xcSbHDoubleArrow)
	p.cursors[gui.CursorResizeNS] = load(vResizeCursorNames, xcSbVDoubleArrow)
	p.cursors[gui.CursorResizeNWSE] = load(nwseCursorNames, xcBottomRightCorner)
	p.cursors[gui.CursorResizeNESW] = load(neswCursorNames, xcBottomLeftCorner)
	p.cursors[gui.CursorResizeAll] = load(moveCursorNames, xcFleur)
	p.cursors[gui.CursorNotAllowed] = load(notAllowedCursorNames, xcXCursor)
	// X protocol ordering runs the glyph-cursor creations queued above
	// before this closes the font.
	xproto.CloseFont(p.conn, font)
}
