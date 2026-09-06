//go:build linux && !js && !android

package gl

import (
	"math"
	"unsafe"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// netWMOpacityProp is the EWMH-adjacent property every compositing
// manager reads for a whole-window fade. It is not part of the window
// system: the X server stores it, and the compositor applies it.
const netWMOpacityProp = "_NET_WM_WINDOW_OPACITY"

// opacityCardinal encodes an opacity in [0, 1] the way the property
// wants it: a single 32-bit CARDINAL, 0 invisible and 0xFFFFFFFF fully
// opaque. Pure, so it is tested with no X server. A NaN names no fade,
// so it maps to opaque rather than to an implementation-defined
// float-to-integer conversion.
func opacityCardinal(opacity float32) uint32 {
	if math.IsNaN(float64(opacity)) {
		return 0xFFFFFFFF
	}
	if opacity <= 0 {
		return 0
	}
	if opacity >= 1 {
		return 0xFFFFFFFF
	}
	// Round rather than truncate, so 1/255 steps land where a caller
	// stepping through a slider expects them to.
	return uint32(float64(opacity)*0xFFFFFFFF + 0.5)
}

// setNetWMWindowOpacity writes (or clears) the fade property.
//
// Full opacity deletes the property instead of writing 0xFFFFFFFF: an
// absent property puts the window back on the compositor's untouched
// path, which is not always the same code as "faded by 100%".
//
// The property applies whether or not the window is Transparent — it is
// a window-manager hint, not a visual, so nothing about the GL surface
// is involved.
func setNetWMWindowOpacity(conn *xgb.Conn, win xproto.Window,
	atom xproto.Atom, opacity float32) {
	if opacity >= 1 {
		xproto.DeleteProperty(conn, win, atom)
		return
	}
	val := opacityCardinal(opacity)
	// Values are in the client's byte order, the same local-connection
	// assumption setWindowDecorations makes for _MOTIF_WM_HINTS.
	xproto.ChangeProperty(conn, xproto.PropModeReplace, win,
		atom, xproto.AtomCardinal, 32, 1,
		unsafe.Slice((*byte)(unsafe.Pointer(&val)), 4))
}

// applyWindowOpacity sets the fade and reports the ways it degrades.
// Without a compositing manager nothing reads the property, which is
// the same silent failure a Transparent window has.
//
// w may be nil: platformState.w is only filled once Run starts, and the
// setter is reachable before that. The property still gets set; only
// the diagnostic is skipped.
func applyWindowOpacity(w *gui.Window, p *platformState, opacity float32) {
	if p.conn == nil || p.window == 0 {
		return
	}
	warn := func(reason string) {
		if w != nil {
			w.DebugWindowOpacity(reason)
		}
	}
	if p.atomOpacity == 0 {
		p.atomOpacity = internAtom(p.conn, netWMOpacityProp)
	}
	if p.atomOpacity == 0 {
		warn("the X server would not intern " + netWMOpacityProp +
			"; the window stays opaque")
		return
	}
	setNetWMWindowOpacity(p.conn, p.window, p.atomOpacity, opacity)
	// Flush, or the property sits in the request buffer until some
	// unrelated request goes out and the fade lands a frame late.
	p.conn.Sync()
	// The compositor check costs two more blocking round trips, and it
	// only feeds a dev-mode message, so a fade animation must not pay
	// for it every frame.
	if opacity < 1 && gui.DebugEnabled() &&
		!hasCompositor(p.conn, p.conn.DefaultScreen) {
		warn("no compositing manager is running; nothing reads " +
			netWMOpacityProp)
	}
}
