//go:build windows && !js

package gl

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// Window state and placement (#1171, #1220, docs/specs/window-state.md).
//
// Each request is posted to the window and runs from the message pump.
// gui calls these from event handlers, and Windows answers ShowWindow
// and SetWindowPos with an inline WM_SIZE, whose handler runs EventFn
// and FrameFn. Run inline, that would overwrite the event in flight and
// rebuild the tree the handler is walking.

var (
	pIsZoomed           = user32.NewProc("IsZoomed")
	pIsIconic           = user32.NewProc("IsIconic")
	pGetWindowRect      = user32.NewProc("GetWindowRect")
	pGetWindowPlacement = user32.NewProc("GetWindowPlacement")
	pSetWindowPlacement = user32.NewProc("SetWindowPlacement")
)

const (
	// wmWindowOp carries one window-state request: the op in wParam, its
	// argument in lParam.
	wmWindowOp = wmApp + 1

	gwlStyle = ^uintptr(15) // -16 as an unsigned word

	wsMaximize = 0x01000000

	swMinimize = 6
	swMaximize = 3

	placeFlags = swpNoZOrder | swpNoActivate
)

// Window-state ops, the wParam of wmWindowOp.
const (
	windowOpMinimize = iota
	windowOpMaximize
	windowOpRestore
	windowOpFullscreen
	windowOpResize
	windowOpMove
)

// windowPlacement mirrors WINDOWPLACEMENT.
type windowPlacement struct {
	length     uint32
	flags      uint32
	showCmd    uint32
	minPos     pointL
	maxPos     pointL
	normalRect rectW
}

// packPair puts two values in an lParam, as MAKELPARAM does. loWordS
// and hiWordS read them back. Sizes fit: gui caps them at 32767. So do
// positions: SetWindowPos clamps a window's rect to 16 bits whatever it
// is passed (a request for x=40000 lands at 32767).
func packPair(lo, hi int) uintptr {
	return uintptr(uint16(clampInt16(lo))) | uintptr(uint16(clampInt16(hi)))<<16
}

func clampInt16(v int) int16 {
	return int16(min(max(v, -1<<15), 1<<15-1))
}

// postWindowOp queues op for the message pump.
func (n *nativePlatform) postWindowOp(op, arg uintptr) {
	if n.b == nil || n.b.plat.hwnd == 0 {
		return
	}
	pPostMessageW.Call(n.b.plat.hwnd, wmWindowOp, op, arg)
}

// MinimizeWindow asks for the taskbar.
func (n *nativePlatform) MinimizeWindow() { n.postWindowOp(windowOpMinimize, 0) }

// MaximizeWindow fills the work area.
func (n *nativePlatform) MaximizeWindow() { n.postWindowOp(windowOpMaximize, 0) }

// RestoreWindow undoes a minimize, then a maximize.
func (n *nativePlatform) RestoreWindow() { n.postWindowOp(windowOpRestore, 0) }

// SetWindowFullscreen enters or leaves borderless fullscreen.
func (n *nativePlatform) SetWindowFullscreen(fullscreen bool) {
	var arg uintptr
	if fullscreen {
		arg = 1
	}
	n.postWindowOp(windowOpFullscreen, arg)
}

// ResizeWindow sets the client area, in logical pixels.
func (n *nativePlatform) ResizeWindow(width, height int) {
	n.postWindowOp(windowOpResize, packPair(width, height))
}

// MoveWindow puts the client area's corner at x, y.
func (n *nativePlatform) MoveWindow(x, y int) {
	n.postWindowOp(windowOpMove, packPair(x, y))
}

// WindowPosition returns the client area's corner in physical desktop
// pixels. A minimized window sits at a parking position that means
// nothing to an app, so it reports no position.
func (n *nativePlatform) WindowPosition() (x, y int, ok bool) {
	if n.b == nil || n.b.plat.hwnd == 0 || isIconic(n.b.plat.hwnd) {
		return 0, 0, false
	}
	p := clientOrigin(n.b.plat.hwnd)
	return int(p.x), int(p.y), true
}

// windowOp runs one posted request.
func (b *Backend) windowOp(op, arg uintptr) {
	hwnd := b.plat.hwnd
	switch op {
	case windowOpMinimize:
		pShowWindow.Call(hwnd, swMinimize)
	case windowOpMaximize:
		if !b.plat.fullscreen {
			pShowWindow.Call(hwnd, swMaximize)
		}
	case windowOpRestore:
		// A fullscreen window restores only from the taskbar: the normal
		// rect it would take is the one SetWindowFullscreen(false) brings
		// back.
		if !b.plat.fullscreen || isIconic(hwnd) {
			pShowWindow.Call(hwnd, swRestore)
		}
	case windowOpFullscreen:
		b.setFullscreen(arg != 0)
	case windowOpResize:
		b.resizeClient(loWordS(arg), hiWordS(arg))
	case windowOpMove:
		b.moveClient(loWordS(arg), hiWordS(arg))
	}
}

// setFullscreen swaps the frame for a borderless window over the whole
// monitor, keeping what it needs to come back: the style, and the
// placement, which also carries whether the window was maximized.
//
// WS_MAXIMIZE goes too. Windows treats a window with that bit as already
// maximized, so leaving would keep the monitor rect and never ask
// WM_GETMINMAXINFO for the work area. Without the bit, the placement
// performs a real maximize, through the size limits and the frameless
// clamp. fullscreen changes first in both directions, because that
// message reads it.
func (b *Backend) setFullscreen(on bool) {
	p := &b.plat
	if on == p.fullscreen {
		return
	}
	hwnd := p.hwnd
	if !on {
		p.fullscreen = false
		pSetWindowLongPtrW.Call(hwnd, gwlStyle, p.savedStyle&^wsMaximize)
		pSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&p.savedPlacement)))
		pSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, titlebarRepaintFlags)
		b.syncWindowState()
		return
	}
	if isIconic(hwnd) {
		pShowWindow.Call(hwnd, swRestore)
	}
	rc, ok := monitorRect(hwnd)
	if !ok {
		return
	}
	p.savedPlacement = windowPlacement{}
	p.savedPlacement.length = uint32(unsafe.Sizeof(p.savedPlacement))
	r, _, _ := pGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&p.savedPlacement)))
	if r == 0 {
		return
	}
	p.savedStyle, _, _ = pGetWindowLongPtrW.Call(hwnd, gwlStyle)
	p.fullscreen = true
	pSetWindowLongPtrW.Call(hwnd, gwlStyle,
		p.savedStyle&^(wsOverlappedWindow|wsMaximize))
	pSetWindowPos.Call(hwnd, 0,
		uintptr(rc.left), uintptr(rc.top),
		uintptr(rc.right-rc.left), uintptr(rc.bottom-rc.top),
		placeFlags|swpFrameChanged)
	b.syncWindowState()
}

// monitorRect is the bounds of the monitor the window is mostly on.
func monitorRect(hwnd uintptr) (rectW, bool) {
	mon, _, _ := pMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if mon == 0 {
		return rectW{}, false
	}
	var mi monitorInfo
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	ok, _, _ := pGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	return mi.rcMonitor, ok != 0
}

// resizeClient sets the client area to a logical size. The frame is
// measured, not derived from the style, so it holds for every
// decoration, frameless included.
func (b *Backend) resizeClient(width, height int32) {
	hwnd := b.plat.hwnd
	if !b.freelyPlaced() {
		return
	}
	var outer rectW
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&outer)))
	cw, ch := clientSize(hwnd)
	frameW := outer.right - outer.left - cw
	frameH := outer.bottom - outer.top - ch
	s := b.dpiScale
	pSetWindowPos.Call(hwnd, 0, 0, 0,
		uintptr(clientExtent(int(width), s)+frameW),
		uintptr(clientExtent(int(height), s)+frameH),
		placeFlags|swpNoMove)
}

// moveClient puts the client area's corner at x, y in physical desktop
// pixels. SetWindowPos places the outer rect, so the frame is added.
func (b *Backend) moveClient(x, y int32) {
	hwnd := b.plat.hwnd
	if !b.freelyPlaced() {
		return
	}
	var outer rectW
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&outer)))
	origin := clientOrigin(hwnd)
	pSetWindowPos.Call(hwnd, 0,
		uintptr(x+outer.left-origin.x), uintptr(y+outer.top-origin.y), 0, 0,
		placeFlags|swpNoSize)
}

// freelyPlaced reports whether the window keeps a rect set on it.
// gui already refuses while maximized or fullscreen; this catches a
// state that changed between the post and the pump, and a minimized
// window, whose rect is the parking position.
func (b *Backend) freelyPlaced() bool {
	hwnd := b.plat.hwnd
	return !b.plat.fullscreen && !isZoomed(hwnd) && !isIconic(hwnd)
}

// syncWindowState tells gui what the window is now. A fullscreen window
// has no WS_MAXIMIZE, so the saved placement says what it returns to.
func (b *Backend) syncWindowState() {
	p := &b.plat
	maximized := isZoomed(p.hwnd) ||
		p.fullscreen && p.savedPlacement.showCmd == swMaximize
	gui.DispatchWindowState(p.w, maximized, p.fullscreen)
}

func isZoomed(hwnd uintptr) bool {
	r, _, _ := pIsZoomed.Call(hwnd)
	return r != 0
}

func isIconic(hwnd uintptr) bool {
	r, _, _ := pIsIconic.Call(hwnd)
	return r != 0
}

// clientOrigin is the client area's corner in desktop coordinates.
func clientOrigin(hwnd uintptr) pointL {
	var p pointL
	pClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&p)))
	return p
}
