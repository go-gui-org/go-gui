//go:build windows && !js

package gl

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// SetWindowLongPtrW and GetWindowLongPtrW are 64-bit-only exports; on
// 32-bit Windows the names are macros for the ...LongW pair and the
// procs are absent. Find() catches that and the fade degrades to a
// reported no-op, like any other missing export here.
var (
	pSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	pGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	pSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
)

// Win32 constants for the layered-window path (winuser.h).
const (
	wsExLayered = 0x00080000
	gwlExStyle  = ^uintptr(19) // -20 as an unsigned word
	lwaAlpha    = 0x00000002

	// setLayeredWindowAttributesFn names the export in the diagnostics,
	// so a reader can look up the call that was refused.
	setLayeredWindowAttributesFn = "SetLayeredWindowAttributes"
)

// layeredOpacityAllowed reports whether WS_EX_LAYERED may be set on
// this window.
//
// A Transparent window already composites through
// DwmEnableBlurBehindWindow (see transparency_win32.go). WS_EX_LAYERED
// adds a redirection surface that fights the GL swap chain, so the two
// are not combined: the fade is refused and reported rather than
// risking a window that presents nothing. See #516.
func layeredOpacityAllowed(transparent bool) bool { return !transparent }

// opacityAlphaByte converts an opacity in [0, 1] to the 0-255 alpha
// SetLayeredWindowAttributes takes. Pure, so it is tested without a
// window. A NaN names no fade, so it maps to opaque rather than to an
// implementation-defined byte conversion.
func opacityAlphaByte(opacity float32) byte {
	switch {
	case math.IsNaN(float64(opacity)):
		return 255
	case opacity <= 0:
		return 0
	case opacity >= 1:
		return 255
	}
	return byte(opacity*255 + 0.5)
}

// applyWindowOpacity fades the whole window with WS_EX_LAYERED plus
// SetLayeredWindowAttributes(LWA_ALPHA).
//
// The extended style bit is set on every call rather than tracked: the
// call is idempotent, and it costs one GetWindowLongPtrW. No
// SWP_FRAMECHANGED is needed — WS_EX_LAYERED changes how the window is
// composited, not the size of its frame.
//
// Best-effort, like every other window-level capability here: a missing
// export or a refused combination reports through gui.Debug and leaves
// the window as it was.
func applyWindowOpacity(w *gui.Window, hwnd uintptr, transparent bool, opacity float32) {
	if hwnd == 0 {
		return
	}
	warn := func(reason string) {
		if w != nil {
			w.DebugWindowOpacity(reason)
		}
	}
	if !layeredOpacityAllowed(transparent) {
		warn("the window is Transparent; WS_EX_LAYERED and per-pixel " +
			"alpha do not compose on Windows")
		return
	}
	if err := pSetLayeredWindowAttributes.Find(); err != nil {
		warn(setLayeredWindowAttributesFn + " is unavailable; the " +
			"window stays opaque")
		return
	}
	if err := pGetWindowLongPtrW.Find(); err != nil {
		warn("GetWindowLongPtrW is unavailable; the window stays opaque")
		return
	}
	if err := pSetWindowLongPtrW.Find(); err != nil {
		warn("SetWindowLongPtrW is unavailable; the window stays opaque")
		return
	}
	ex, _, _ := pGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	pSetWindowLongPtrW.Call(hwnd, gwlExStyle, ex|wsExLayered)
	ok, _, _ := pSetLayeredWindowAttributes.Call(hwnd, 0,
		uintptr(opacityAlphaByte(opacity)), lwaAlpha)
	if ok == 0 {
		// Put the style back. A layered window whose alpha was never
		// set is not painted at all, so leaving the bit on would turn
		// a refused fade into an invisible window — the opposite of
		// what the message below promises.
		pSetWindowLongPtrW.Call(hwnd, gwlExStyle, ex)
		warn(setLayeredWindowAttributesFn + " failed; the window " +
			"stays opaque")
	}
}
