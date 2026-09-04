//go:build windows && !js

package gl

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	pMonitorFromWindow = user32.NewProc("MonitorFromWindow")
	pGetMonitorInfoW   = user32.NewProc("GetMonitorInfoW")
)

const (
	// Window styles for a frameless window. WS_POPUP drops the caption;
	// WS_THICKFRAME is kept anyway because it is what gives the window
	// OS resize, Aero snap and the drop shadow. The caption strip it
	// would otherwise reserve is removed in WM_NCCALCSIZE.
	wsPopup       = 0x80000000
	wsThickFrame  = 0x00040000
	wsMinimizeBox = 0x00020000
	wsMaximizeBox = 0x00010000

	wmNcCalcSize    = 0x0083
	wmGetMinMaxInfo = 0x0024
	wmSysCommand    = 0x0112

	scSize = 0xF000
	scMove = 0xF010

	htCaption = 2

	// WMSZ_* resize directions for SC_SIZE.
	wmszLeft        = 1
	wmszRight       = 2
	wmszTop         = 3
	wmszTopLeft     = 4
	wmszTopRight    = 5
	wmszBottom      = 6
	wmszBottomLeft  = 7
	wmszBottomRight = 8

	monitorDefaultToNearest = 0x0002
)

// monitorInfo mirrors MONITORINFO.
type monitorInfo struct {
	cbSize    uint32
	rcMonitor rectW
	rcWork    rectW
	dwFlags   uint32
}

// pointL mirrors POINT.
type pointL struct {
	x, y int32
}

// minMaxInfo mirrors MINMAXINFO.
type minMaxInfo struct {
	ptReserved     pointL
	ptMaxSize      pointL
	ptMaxPosition  pointL
	ptMinTrackSize pointL
	ptMaxTrackSize pointL
}

// windowStyleFor picks the CreateWindowEx style for a config. Split out
// of New so the mapping is unit testable without a window.
func windowStyleFor(cfg gui.WindowCfg) uintptr {
	style := uintptr(wsClipSiblings | wsClipChildren)
	switch {
	case cfg.Decorations == gui.DecorationNone && cfg.FixedSize:
		// No resize border either: only the popup shell remains.
		style |= wsPopup
	case cfg.Decorations == gui.DecorationNone:
		style |= wsPopup | wsThickFrame | wsMinimizeBox | wsMaximizeBox
	case cfg.FixedSize:
		style |= wsFixed
	default:
		// DecorationHiddenTitlebar has no Win32 equivalent, so it
		// lands here with the standard frame.
		style |= wsOverlappedWindow
	}
	return style
}

// wmszFor maps a WindowEdge to the WMSZ_* direction SC_SIZE expects.
func wmszFor(edge gui.WindowEdge) uintptr {
	switch edge {
	case gui.EdgeTopLeft:
		return wmszTopLeft
	case gui.EdgeTop:
		return wmszTop
	case gui.EdgeTopRight:
		return wmszTopRight
	case gui.EdgeRight:
		return wmszRight
	case gui.EdgeBottomRight:
		return wmszBottomRight
	case gui.EdgeBottom:
		return wmszBottom
	case gui.EdgeBottomLeft:
		return wmszBottomLeft
	case gui.EdgeLeft:
		return wmszLeft
	}
	return 0
}

// trackSizeFor converts logical content-size limits into the outer
// frame track sizes WM_GETMINMAXINFO reports. Windows sizes a window by
// its whole frame, but WindowCfg speaks in client area, so the same
// AdjustWindowRectExForDpi expansion New applies to Width/Height is
// applied here — otherwise a MinWidth would mean a smaller client area
// on Windows than on the other platforms.
//
// Returns zeroed points for the axes with no limit; the caller leaves
// those fields of MINMAXINFO alone so Windows keeps its defaults. Split
// out of the message handler so the conversion is unit testable without
// a window, and so the handler itself stays allocation-free.
func trackSizeFor(limits gui.SizeLimits, style uintptr, dpi uint32) (minTrack, maxTrack pointL) {
	if limits.None() {
		// Nothing to measure, and no reason to pay for the syscall on
		// the ordinary window that sets no bounds at all.
		return minTrack, maxTrack
	}
	// The frame overhead is the same for both bounds, so it is measured
	// once against an empty client rect rather than per bound.
	var rc rectW
	pAdjustRectDpi.Call(uintptr(unsafe.Pointer(&rc)), style, 0, 0, uintptr(dpi))
	frameW := rc.right - rc.left
	frameH := rc.bottom - rc.top

	// SizeLimits.Scaled owns the logical -> physical conversion for
	// every backend, so a MinWidth resolves to the same client size
	// here as it does on X11.
	phys := limits.Scaled(float32(dpi) / 96.0)
	if phys.MinW > 0 {
		minTrack.x = int32(phys.MinW) + frameW
	}
	if phys.MinH > 0 {
		minTrack.y = int32(phys.MinH) + frameH
	}
	if phys.MaxW > 0 {
		maxTrack.x = int32(phys.MaxW) + frameW
	}
	if phys.MaxH > 0 {
		maxTrack.y = int32(phys.MaxH) + frameH
	}
	return minTrack, maxTrack
}

// applySizeLimits answers WM_GETMINMAXINFO with the configured track
// sizes. Each axis is written only when it is constrained, so an unset
// axis keeps the default Windows already put in the struct.
func (b *Backend) applySizeLimits(lparam uintptr) bool {
	if lparam == 0 {
		return false
	}
	minT, maxT := b.plat.minTrack, b.plat.maxTrack
	if minT == (pointL{}) && maxT == (pointL{}) {
		return false
	}
	mmi := (*minMaxInfo)(ptrFromLParam(lparam))
	if minT.x > 0 {
		mmi.ptMinTrackSize.x = minT.x
	}
	if minT.y > 0 {
		mmi.ptMinTrackSize.y = minT.y
	}
	// ptMaxTrackSize caps the maximize button as well as the drag,
	// which matches what setContentMaxSize: does on macOS.
	if maxT.x > 0 {
		mmi.ptMaxTrackSize.x = maxT.x
	}
	if maxT.y > 0 {
		mmi.ptMaxTrackSize.y = maxT.y
	}
	return true
}

// clampMaximizeToWorkArea answers WM_GETMINMAXINFO for a frameless
// window. Zeroing the non-client area in WM_NCCALCSIZE also removes the
// inset Windows normally applies when maximizing, so without this the
// window overhangs every screen edge by the resize border and covers
// the taskbar.
func (b *Backend) clampMaximizeToWorkArea(lparam uintptr) (uintptr, bool) {
	if lparam == 0 || b.plat.hwnd == 0 {
		return 0, false
	}
	mon, _, _ := pMonitorFromWindow.Call(b.plat.hwnd, monitorDefaultToNearest)
	if mon == 0 {
		return 0, false
	}
	var mi monitorInfo
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	ok, _, _ := pGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	if ok == 0 {
		return 0, false
	}
	mmi := (*minMaxInfo)(ptrFromLParam(lparam))
	// Both rects are in virtual-screen coordinates, but ptMaxPosition
	// is relative to the monitor.
	mmi.ptMaxPosition = pointL{
		x: mi.rcWork.left - mi.rcMonitor.left,
		y: mi.rcWork.top - mi.rcMonitor.top,
	}
	mmi.ptMaxSize = pointL{
		x: mi.rcWork.right - mi.rcWork.left,
		y: mi.rcWork.bottom - mi.rcWork.top,
	}
	return 0, true
}

// ptrFromLParam reinterprets an lparam as the pointer Windows put
// there. A direct unsafe.Pointer(lparam) conversion is what go vet
// warns about, since a uintptr is not a reference the GC tracks — but
// this address belongs to the OS message, not to the Go heap, so
// nothing here can move underneath it.
func ptrFromLParam(l uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&l))
}

// StartWindowDrag hands the pointer to the system move loop, which
// keeps Aero snap working. ReleaseCapture first: the mouse-down that
// got us here left the window holding capture, and SC_MOVE needs it.
func (n *nativePlatform) StartWindowDrag() {
	if n.b == nil || n.b.plat.hwnd == 0 {
		return
	}
	pReleaseCapture.Call()
	pSendMessageW.Call(n.b.plat.hwnd, wmSysCommand, scMove|htCaption, 0)
}

// StartWindowResize hands the pointer to the system resize loop for one
// edge or corner.
func (n *nativePlatform) StartWindowResize(edge gui.WindowEdge) {
	if n.b == nil || n.b.plat.hwnd == 0 {
		return
	}
	dir := wmszFor(edge)
	if dir == 0 {
		return
	}
	pReleaseCapture.Call()
	pSendMessageW.Call(n.b.plat.hwnd, wmSysCommand, scSize|dir, 0)
}
