//go:build linux && !js && !android

package gl

import (
	"fmt"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// argbDepth is the X visual depth that carries an alpha channel. A
// window created on a depth-24 visual has no per-pixel alpha to hand
// the compositor, however much alpha the GL surface itself keeps.
const argbDepth = 32

// visualDepths indexes an X screen's visual ids by depth, so the picker
// stays a pure function over data a test can build by hand.
func visualDepths(depths []xproto.DepthInfo) map[uint32]byte {
	m := make(map[uint32]byte)
	for _, d := range depths {
		for _, v := range d.Visuals {
			m[uint32(v.VisualId)] = d.Depth
		}
	}
	return m
}

// pickVisual chooses which EGL config the window is created with.
//
// The opaque path keeps the driver's first choice, which is what the
// backend did before transparency existed. A transparent window instead
// takes the first candidate whose X visual has depth 32; drivers list
// depth-24 configs first, so the first candidate is almost never the
// one wanted.
//
// When no candidate has an ARGB visual, it falls back to the driver's
// first choice and reports ok=false. The caller degrades to an opaque
// window rather than failing to open one at all.
//
// depths maps visual id to depth (see visualDepths); a visual missing
// from it is one this screen does not offer, and is skipped.
func pickVisual(cands []eglConfigVisual, depths map[uint32]byte, wantAlpha bool) (
	cfg eglConfigVisual, depth byte, ok bool,
) {
	if len(cands) == 0 {
		return eglConfigVisual{}, 0, false
	}
	first := cands[0]
	if wantAlpha {
		for _, c := range cands {
			if depths[c.visualID] == argbDepth {
				return c, argbDepth, true
			}
		}
	}
	// rootDepth is not known here, so report the visual's own depth and
	// let a zero mean "not in this screen's list" — CreateWindow needs
	// the depth that matches the visual it is given.
	return first, depths[first.visualID], !wantAlpha
}

// hasCompositor reports whether a compositing manager owns the
// _NET_WM_CM_S<screen> selection. Without one, an ARGB window is
// composited against nothing and renders black, so this is worth
// saying out loud rather than shipping a black window.
func hasCompositor(conn *xgb.Conn, screenNum int) bool {
	atom := internAtom(conn, fmt.Sprintf("_NET_WM_CM_S%d", screenNum))
	if atom == 0 {
		return false
	}
	reply, err := xproto.GetSelectionOwner(conn, atom).Reply()
	if err != nil || reply == nil {
		return false
	}
	return reply.Owner != 0
}

// warnTransparency reports the two ways a Transparent window quietly
// degrades on X11. Dev-mode only, matching the rest of gui.Debug.
func warnTransparency(w *gui.Window, gotARGB, composited bool) {
	if !gotARGB {
		w.DebugWindowTransparency("no depth-32 visual matched the GL " +
			"config; the window stays opaque")
	}
	if !composited {
		w.DebugWindowTransparency("no compositing manager is running; " +
			"the window will render black")
	}
}
