//go:build windows && !js

package gl

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	pDwmEnableBlurBehindWindow = dwmapi.NewProc("DwmEnableBlurBehindWindow")
	// gdi32 is declared with the WGL bindings (wgl_windows.go).
	pCreateRectRgn = gdi32.NewProc("CreateRectRgn")
	pDeleteObject  = gdi32.NewProc("DeleteObject")
)

// DWM_BLURBEHIND flags (dwmapi.h).
const (
	dwmBBEnable      = 0x00000001
	dwmBBBlurRegion  = 0x00000002
	dwmBlurBehindWin = "DwmEnableBlurBehindWindow"
)

// dwmBlurBehind mirrors DWM_BLURBEHIND (dwmapi.h). fEnable is a Win32
// BOOL, so it is 4 bytes and the struct needs no manual padding.
type dwmBlurBehind struct {
	dwFlags                uint32
	fEnable                int32
	hRgnBlur               uintptr
	fTransitionOnMaximized int32
}

// enableWindowTransparency makes DWM honour the client area's alpha
// channel, so a GL frame cleared with alpha < 255 shows the desktop
// behind it.
//
// The mechanism is DwmEnableBlurBehindWindow with an *empty* blur
// region. An empty region means "blur nothing", but enabling
// blur-behind at all is what switches the window off the opaque
// composition path — so the result is plain per-pixel transparency
// with no blur. CreateRectRgn(0, 0, -1, -1) is the canonical empty
// region.
//
// WS_EX_LAYERED is deliberately not used: it composites through a
// redirection surface that fights the GL swap chain.
//
// Best-effort. A missing dwmapi export or a failed call leaves an
// opaque window and reports through gui.Debug rather than failing the
// window creation.
func enableWindowTransparency(w *gui.Window, hwnd uintptr) {
	if err := pDwmEnableBlurBehindWindow.Find(); err != nil {
		w.DebugWindowTransparency(dwmBlurBehindWin + " is unavailable; " +
			"the window stays opaque")
		return
	}
	// A null region still enables per-pixel alpha; it just also blurs.
	// Carry on without one rather than skipping transparency entirely.
	var rgn uintptr
	if err := pCreateRectRgn.Find(); err == nil {
		rgn, _, _ = pCreateRectRgn.Call(0, 0, ^uintptr(0), ^uintptr(0))
	}
	bb := dwmBlurBehind{
		dwFlags:  dwmBBEnable | dwmBBBlurRegion,
		fEnable:  1,
		hRgnBlur: rgn,
	}
	hr, _, _ := pDwmEnableBlurBehindWindow.Call(hwnd,
		uintptr(unsafe.Pointer(&bb)))
	// DWM owns the region once the call succeeds; on failure it is ours.
	if hr != 0 {
		if rgn != 0 {
			if err := pDeleteObject.Find(); err == nil {
				pDeleteObject.Call(rgn)
			}
		}
		w.DebugWindowTransparency(dwmBlurBehindWin +
			" failed; the window stays opaque")
	}
}
