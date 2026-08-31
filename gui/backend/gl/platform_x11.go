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
