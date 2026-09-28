//go:build linux && !js && !android

package gl

import (
	"runtime"
	"syscall"

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

	// lockedTid is the OS thread New locked its goroutine to, handed to
	// Destroy so it can release that lock (#827). Zero once released, or
	// when New failed and released it itself.
	lockedTid int

	cursors   [13]xproto.Cursor
	curCursor xproto.Cursor

	wmDelete xproto.Atom
	wakeAtom xproto.Atom
	// atomOpacity caches _NET_WM_WINDOW_OPACITY. An atom is fixed for
	// the life of a connection, and a fade animation would otherwise
	// pay a blocking InternAtom round trip every frame. Zero means
	// not interned yet.
	atomOpacity xproto.Atom
	keymap      *xproto.GetKeyboardMappingReply
	minKeycode  xproto.Keycode

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
	// imeRect caches the last caret rect reported to IBus in root
	// pixels, so the per-frame re-report from the render path costs
	// a comparison instead of a D-Bus call.
	ime         *ibus.Client
	imeBuf      []ibus.Event
	imeEvts     []gui.Event
	imeRect     [4]int32
	imeHaveRect bool

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
// (*gui.Window).InvalidateLayout), and the overwhelming majority already run
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

// releaseThread undoes New's runtime.LockOSThread, once.
//
// Leaving the lock held is harmless for an app, whose main goroutine is
// locked from init and never exits. It is not harmless for a goroutine that
// returns after Destroy, as every test does: the Go runtime then terminates
// that goroutine's OS thread. On linux/arm64 under CGO_ENABLED=0 thread exit
// runs through purego's fakecgo threadentry_trampoline. Before purego v0.11.0
// that trampoline corrupted the frame pointer it returned to glibc with, and
// the process segfaulted (#827). The bump to v0.11.1 fixes the crash. This
// unlock still matters: without it, every Destroyed backend costs its caller
// an OS thread.
//
// The unlock applies only on the thread New locked. A locked goroutine owns
// its thread, so a matching tid means Destroy runs on New's goroutine. A
// Destroy called from any other goroutine must not unlock, because that
// would release a lock that goroutine took for its own reasons. In that case
// the lock is left held, exactly as before this fix.
func (p *platformState) releaseThread() {
	if p.lockedTid == 0 || p.lockedTid != syscall.Gettid() {
		return
	}
	p.lockedTid = 0
	runtime.UnlockOSThread()
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

// xEventSource is the one method of *xgb.Conn the event pump needs;
// tests substitute a scripted source.
type xEventSource interface {
	WaitForEvent() (xgb.Event, xgb.Error)
}

// pumpXEvents reads X events on a dedicated goroutine and forwards them
// on ch. It closes ch when the connection ends so the main loop exits.
//
// src is captured once by the caller, on the main goroutine, and never
// re-read from platformState: destroy() sets p.conn to nil after closing
// it, and a pump that re-read the field each iteration raced that write
// and dereferenced nil on shutdown (issue #701). The captured *xgb.Conn
// stays valid after Close — WaitForEvent then reports (nil, nil), which
// ends the loop below.
func pumpXEvents(src xEventSource, ch chan<- xgb.Event) {
	for {
		ev, err := src.WaitForEvent()
		if ev == nil && err == nil {
			close(ch) // connection closed
			return
		}
		// A lone X error (ev == nil, err != nil) is dropped; the
		// connection is still live.
		if ev != nil {
			ch <- ev
		}
	}
}
