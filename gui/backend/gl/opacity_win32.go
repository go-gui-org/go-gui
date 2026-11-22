//go:build windows && !js

package gl

import "math"

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
// On GL a Transparent window composites through
// DwmEnableBlurBehindWindow (see transparency_win32.go). WS_EX_LAYERED
// adds a redirection surface that fights the GL swap chain, so the two
// are not combined: the fade is refused and reported rather than
// risking a window that presents nothing. See #516. The CPU renderer
// refuses it on a Transparent window too (#1310); fadeRefusal says why
// for each kind of window.
func layeredOpacityAllowed(transparent bool) bool { return !transparent }

// fadeRefusal is the reason the fade is refused on a window, or "" when
// layeredOpacityAllowed lets it through. soft is the CPU renderer, and
// layered its Transparent frameless window.
//
//   - GL: the window is transparent through DWM's blur-behind, which the
//     fade's WS_EX_LAYERED would fight (layeredOpacityAllowed).
//   - Layered: the window already has WS_EX_LAYERED and shows its per-pixel
//     alpha through UpdateLayeredWindow. The two calls exclude each other:
//     once SetLayeredWindowAttributes has been called on a layered window,
//     UpdateLayeredWindow fails on it (documented), so the fade would
//     stop every later frame.
//   - A system frame: the CPU renderer draws the window opaque, and the
//     fade would work on it as on any opaque soft window. It is refused
//     anyway, so whether a Transparent window takes a fade does not depend
//     on the renderer: the same window refuses it on GL, and a fallback
//     from GL does not change the answer.
func fadeRefusal(transparent, soft, layered bool) string {
	switch {
	case layeredOpacityAllowed(transparent):
		return ""
	case layered:
		return "the window is Transparent and frameless: the CPU renderer " +
			"shows it with UpdateLayeredWindow, which fails once " +
			setLayeredWindowAttributesFn + " has been called"
	case soft:
		return "the window is Transparent; the CPU renderer draws it opaque " +
			"because it has a system frame, and refuses the fade on a " +
			"Transparent window as OpenGL does"
	}
	return "the window is Transparent; on OpenGL it composites through " +
		"DWM blur-behind, and the fade's WS_EX_LAYERED would fight the swap chain"
}

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
// export or a refused combination is reported through warn, the window's
// DebugWindowOpacity or New's collection for the window being opened, and
// leaves the window as it was. refusal is fadeRefusal's answer for the
// window: when it is not empty, the fade is refused with it.
func applyWindowOpacity(warn func(string), hwnd uintptr, refusal string, opacity float32) {
	if hwnd == 0 {
		return
	}
	if refusal != "" {
		warn(refusal)
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
