//go:build linux && !js && !android

package gl

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ibus"
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

	// Selections. X11 has two independent text buffers: CLIPBOARD (explicit
	// copy/paste) and PRIMARY (filled by selecting text, pasted with the
	// middle button). Both are served by the same ownership machinery, so
	// each keeps its own cached text and owner flag while sharing the atoms
	// and the read connection below.
	atomClipboard xproto.Atom
	atomUTF8      xproto.Atom
	atomTargets   xproto.Atom
	atomClipProp  xproto.Atom
	clipboardText string
	ownsClipboard bool
	primaryText   string
	ownsPrimary   bool
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

	// Last mouse-press position in root coordinates, kept for
	// _NET_WM_MOVERESIZE: the window manager needs the press that
	// started the gesture, which the Go-side event no longer carries.
	pressRootX  int16
	pressRootY  int16
	pressButton byte
	havePress   bool

	physW, physH int32
	scale        float32

	// limits holds the configured resize bounds in logical pixels, kept
	// so a DPI change can rewrite WM_NORMAL_HINTS at the new scale
	// without re-reading the window config.
	limits gui.SizeLimits

	// Input method. ime is nil when none is reachable, in which case
	// key presses keep going straight through the keysym path. imeBuf
	// is reused by drainIME to keep the handoff allocation-free.
	ime     *ibus.Client
	imeBuf  []ibus.Event
	imeEvts []gui.Event

	// Dead-key / Multi_key composition for the raw keysym path (see
	// compose_x11.go). Only consulted after the input method declines
	// a key press.
	compose compose

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
//
// Deliberately unchecked. Every redraw request calls this (see
// (*gui.Window).UpdateWindow), and the overwhelming majority already run
// on the frame thread where the loop is not parked and the wake is
// redundant — so it must not cost a server round trip. It does not:
// xgb's NewRequest hands the buffer to the sendRequests goroutine and
// blocks until writeBuffer has put it on the wire, so the message is
// flushed by the time this returns. Check() would only add a wait for an
// error reply, and the error was never actionable here anyway.
//
// Nothing drains errors from wakeConn, so its cookie buffer still forces
// an occasional round trip once it fills (xgb amortizes this at one per
// cookieBuffer requests, ~1000) — bounded, and not per call.
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
	xproto.SendEvent(p.wakeConn, false, p.window, 0, string(ev.Bytes()))
}

func (p *platformState) destroy() {
	if p.ime != nil {
		p.ime.Close()
		p.ime = nil
	}
	if p.conn != nil {
		for _, c := range p.cursors {
			if c != 0 {
				xproto.FreeCursor(p.conn, c)
			}
		}
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
