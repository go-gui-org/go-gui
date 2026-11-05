//go:build windows && !js

package gl

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

// -4 as an unsigned word
// Go's uintptr is unsigned, so the constant has to be written as a bit pattern.
const gwlpWndProc = ^uintptr(3)

// newPlacedWindow is newCaptionWindow with a gui.Window to report to.
// The window is unregistered, so its messages go to DefWindowProc and
// no GL state is touched.
func newPlacedWindow(t *testing.T) *Backend {
	t.Helper()
	b := newCaptionWindow(t).b
	b.plat.w = gui.NewTestWindow(t, gui.WindowCfg{})
	b.dpiScale = 1
	return b
}

func outerRect(hwnd uintptr) rectW {
	var r rectW
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

func windowStyle(hwnd uintptr) uintptr {
	s, _, _ := pGetWindowLongPtrW.Call(hwnd, gwlStyle)
	return s
}

func TestPackPairRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct{ lo, hi, wantLo, wantHi int }{
		{0, 0, 0, 0},
		{100, 200, 100, 200},
		{-1920, -1080, -1920, -1080},
		{1 << 20, -(1 << 20), 1<<15 - 1, -(1 << 15)},
	}
	for _, c := range cases {
		arg := packPair(c.lo, c.hi)
		if lo, hi := loWordS(arg), hiWordS(arg); int(lo) != c.wantLo || int(hi) != c.wantHi {
			t.Errorf("packPair(%d, %d) reads back %d, %d", c.lo, c.hi, lo, hi)
		}
	}
}

// The request must wait for the pump: Windows answers these calls with
// an inline WM_SIZE, which runs EventFn and FrameFn. Nothing may change
// before the posted message is delivered.
func TestWindowOpsArePosted(t *testing.T) {
	b := newPlacedWindow(t)
	n := &nativePlatform{b: b}
	before := outerRect(b.plat.hwnd)

	n.MaximizeWindow()
	n.SetWindowFullscreen(true)
	n.ResizeWindow(321, 123)
	n.MoveWindow(40, 50)

	if isZoomed(b.plat.hwnd) || b.plat.fullscreen {
		t.Error("window changed state before the pump ran")
	}
	if outerRect(b.plat.hwnd) != before {
		t.Error("window moved before the pump ran")
	}
	// Drop the requests: this test checks only that they waited.
	var msg msgW
	for {
		r, _, _ := pPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)),
			b.plat.hwnd, wmWindowOp, wmWindowOp, pmRemove)
		if r == 0 {
			return
		}
	}
}

// Nothing in a nil-handle backend may reach the OS.
func TestWindowOpsNoHandle(t *testing.T) {
	t.Parallel()
	n := &nativePlatform{b: &Backend{}}
	n.MinimizeWindow()
	n.ResizeWindow(10, 10)
	if _, _, ok := n.WindowPosition(); ok {
		t.Error("WindowPosition ok = true with no window")
	}
	var bare nativePlatform
	bare.MaximizeWindow()
	if _, _, ok := bare.WindowPosition(); ok {
		t.Error("WindowPosition ok = true with no backend")
	}
}

// Fullscreen drops the frame and covers the monitor; leaving puts back
// the style and the rect.
func TestFullscreenRoundTrip(t *testing.T) {
	b := newPlacedWindow(t)
	hwnd := b.plat.hwnd
	style, rect := windowStyle(hwnd), outerRect(hwnd)
	mon, ok := monitorRect(hwnd)
	if !ok {
		t.Skip("no monitor")
	}

	b.windowOp(windowOpFullscreen, 1)
	if !b.plat.w.IsFullscreen() {
		t.Error("IsFullscreen = false after entering")
	}
	if got := windowStyle(hwnd); got&wsOverlappedWindow != 0 {
		t.Errorf("style %#x keeps frame bits", got)
	}
	if got := outerRect(hwnd); got != mon {
		t.Errorf("rect = %+v, want the monitor %+v", got, mon)
	}

	// Entering twice must not overwrite the saved normal state.
	b.windowOp(windowOpFullscreen, 1)

	b.windowOp(windowOpFullscreen, 0)
	if b.plat.w.IsFullscreen() {
		t.Error("IsFullscreen = true after leaving")
	}
	// The test window starts hidden and SetWindowPlacement shows it; gui
	// refuses SetFullscreen on a hidden window, so an app never gets here
	// hidden.
	if got := windowStyle(hwnd); got&^wsVisible != style&^wsVisible {
		t.Errorf("style = %#x, want %#x", got, style)
	}
	if got := outerRect(hwnd); got != rect {
		t.Errorf("rect = %+v, want %+v", got, rect)
	}
}

// A frameless window has no caption bits but a resize border, which
// must go too or the window keeps its limits and drop shadow.
func TestFullscreenFrameless(t *testing.T) {
	b := newPlacedWindow(t)
	hwnd := b.plat.hwnd
	frameless := uintptr(wsPopup | wsThickFrame | wsMinimizeBox | wsMaximizeBox)
	pSetWindowLongPtrW.Call(hwnd, gwlStyle, frameless)

	b.windowOp(windowOpFullscreen, 1)
	if got := windowStyle(hwnd); got&wsThickFrame != 0 || got&wsPopup == 0 {
		t.Errorf("style %#x, want popup without thick frame", got)
	}
	b.windowOp(windowOpFullscreen, 0)
	// Windows adds WS_CLIPSIBLINGS to every top-level window.
	if got := windowStyle(hwnd); got&^(wsVisible|wsClipSiblings) != frameless {
		t.Errorf("style = %#x, want %#x", got, frameless)
	}
}

// The limits and the frameless clamp must not bind a fullscreen window.
func TestFullscreenSkipsMinMaxInfo(t *testing.T) {
	t.Parallel()
	b := &Backend{}
	b.plat.w = gui.NewTestWindow(t, gui.WindowCfg{})
	b.plat.frameless = true
	b.plat.maxTrack = pointL{x: 100, y: 100}
	b.plat.fullscreen = true
	var mmi minMaxInfo
	lparam := uintptr(unsafe.Pointer(&mmi))
	if _, handled := b.handleMessage(wmGetMinMaxInfo, 0, lparam); handled {
		t.Error("WM_GETMINMAXINFO handled for a fullscreen window")
	}
	if mmi.ptMaxTrackSize != (pointL{}) {
		t.Errorf("ptMaxTrackSize = %+v, want untouched", mmi.ptMaxTrackSize)
	}
}

// Leaving must maximize for real, or a frameless window keeps the
// monitor rect over the taskbar and MaxWidth is ignored: the placement
// has to ask WM_GETMINMAXINFO, and find the window no longer fullscreen.
// The window's procedure is replaced to watch that message, since a
// registered window would reach GL.
func TestFullscreenLeaveAppliesLimits(t *testing.T) {
	b := newPlacedWindow(t)
	hwnd := b.plat.hwnd
	pShowWindow.Call(hwnd, swMaximize)
	if !isZoomed(hwnd) {
		t.Skip("window manager did not maximize")
	}
	b.windowOp(windowOpFullscreen, 1)
	if !b.plat.w.IsMaximized() {
		t.Error("IsMaximized = false in fullscreen from maximized")
	}

	seen, sawFullscreen := 0, false
	proc := windows.NewCallback(func(h, msg, wparam, lparam uintptr) uintptr {
		if msg == wmGetMinMaxInfo {
			seen++
			sawFullscreen = sawFullscreen || b.plat.fullscreen
		}
		r, _, _ := pDefWindowProcW.Call(h, msg, wparam, lparam)
		return r
	})
	pSetWindowLongPtrW.Call(hwnd, gwlpWndProc, proc)

	b.windowOp(windowOpFullscreen, 0)

	if seen == 0 {
		t.Error("no WM_GETMINMAXINFO: the window took no real maximize")
	}
	if sawFullscreen {
		t.Error("WM_GETMINMAXINFO still read the window as fullscreen")
	}
	if !isZoomed(hwnd) || !b.plat.w.IsMaximized() {
		t.Error("window did not return to maximized")
	}
}

// SetPosition(Position()) moves nothing, and a position lands on the
// client area, not the frame.
func TestMoveClient(t *testing.T) {
	b := newPlacedWindow(t)
	n := &nativePlatform{b: b}
	hwnd := b.plat.hwnd

	b.moveClient(120, 140)
	if x, y, ok := n.WindowPosition(); !ok || x != 120 || y != 140 {
		t.Errorf("position = %d, %d, %v, want 120, 140, true", x, y, ok)
	}
	before := outerRect(hwnd)
	x, y, _ := n.WindowPosition()
	b.moveClient(int32(x), int32(y))
	if got := outerRect(hwnd); got != before {
		t.Errorf("round trip moved the window: %+v to %+v", before, got)
	}
}

// The client area gets the size, whatever the frame takes.
func TestResizeClient(t *testing.T) {
	b := newPlacedWindow(t)
	hwnd := b.plat.hwnd

	b.resizeClient(301, 203)
	if cw, ch := clientSize(hwnd); cw != 301 || ch != 203 {
		t.Errorf("client = %d×%d, want 301×203", cw, ch)
	}

	b.dpiScale = 2
	b.resizeClient(150, 100)
	if cw, ch := clientSize(hwnd); cw != 300 || ch != 200 {
		t.Errorf("client at 2x = %d×%d, want 300×200", cw, ch)
	}
}

// A request overtaken by fullscreen leaves the fullscreen rect alone.
func TestPlacementRefusedWhileFullscreen(t *testing.T) {
	b := newPlacedWindow(t)
	hwnd := b.plat.hwnd
	b.windowOp(windowOpFullscreen, 1)
	rect := outerRect(hwnd)

	b.resizeClient(100, 100)
	b.moveClient(1, 1)
	b.windowOp(windowOpMaximize, 0)
	b.windowOp(windowOpRestore, 0)

	if got := outerRect(hwnd); got != rect {
		t.Errorf("rect = %+v, want %+v", got, rect)
	}
	if isZoomed(hwnd) {
		t.Error("maximized while fullscreen")
	}
	b.windowOp(windowOpFullscreen, 0)
}

// A state change posted just before Hide reaches the pump after it, and
// must not show the window again. Resize and move show nothing.
func TestWindowOpsDroppedAfterHide(t *testing.T) {
	for _, op := range []uintptr{
		windowOpMinimize, windowOpMaximize, windowOpRestore, windowOpFullscreen,
	} {
		b := newPlacedWindow(t)
		hwnd := b.plat.hwnd
		b.plat.w.Hide()

		b.windowOp(op, 1)

		if windowStyle(hwnd)&wsVisible != 0 {
			t.Errorf("op %d showed a hidden window", op)
		}
		if b.plat.fullscreen {
			t.Errorf("op %d took a hidden window fullscreen", op)
		}
	}

	b := newPlacedWindow(t)
	b.plat.w.Hide()
	b.windowOp(windowOpResize, packPair(301, 203))
	if cw, ch := clientSize(b.plat.hwnd); cw != 301 || ch != 203 {
		t.Errorf("hidden window client = %d×%d, want 301×203", cw, ch)
	}
}

// Show must not undo a maximize: a tray "Show Window" on a maximized
// window only raises it.
func TestShowWindowKeepsMaximize(t *testing.T) {
	b := newPlacedWindow(t)
	n := &nativePlatform{b: b}
	hwnd := b.plat.hwnd
	pShowWindow.Call(hwnd, swMaximize)
	if !isZoomed(hwnd) {
		t.Skip("window manager did not maximize")
	}
	n.ShowWindow()
	if !isZoomed(hwnd) {
		t.Error("ShowWindow dropped the maximize")
	}
}

// Show must still bring a minimized window back.
func TestShowWindowRestoresMinimized(t *testing.T) {
	b := newPlacedWindow(t)
	n := &nativePlatform{b: b}
	hwnd := b.plat.hwnd
	pShowWindow.Call(hwnd, swMinimize)
	if !isIconic(hwnd) {
		t.Skip("window manager did not minimize")
	}
	if _, _, ok := n.WindowPosition(); ok {
		t.Error("minimized window reports a position")
	}
	n.ShowWindow()
	if isIconic(hwnd) {
		t.Error("ShowWindow left the window minimized")
	}
}

// A maximize made through the caption button reaches the getter, and a
// minimize does not clear it.
func TestWMSizeReportsMaximized(t *testing.T) {
	b := newPlacedWindow(t)
	hwnd := b.plat.hwnd
	pShowWindow.Call(hwnd, swMaximize)
	if !isZoomed(hwnd) {
		t.Skip("window manager did not maximize")
	}
	b.syncWindowState()
	if !b.plat.w.IsMaximized() {
		t.Error("IsMaximized = false for a maximized window")
	}
	if _, handled := b.handleMessage(wmSize, sizeMinimized, 0); !handled {
		t.Fatal("WM_SIZE/SIZE_MINIMIZED fell through")
	}
	if !b.plat.w.IsMaximized() {
		t.Error("minimize cleared IsMaximized")
	}
}
