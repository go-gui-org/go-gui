//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// Occlusion on Wayland: when the compositor shows the window nowhere, gui
// is told (DispatchWindowOccluded), and running animations stop waking the
// main loop every 16 ms. Two signals feed one flag:
//
//   - suspended (xdg_toplevel v6, issue #953): minimized, on another
//     workspace, behind a locked screen. Arrives with configure.
//   - a stalled frame callback (issue #959): compositors stop answering
//     frame callbacks for a surface they do not paint, which also covers a
//     window that others hide completely, and compositors before v6. The
//     outstanding callback's done, sent when the compositor paints the
//     surface again, ends the stall; no new commit is needed for it.
//
// The window is occluded while either holds. A wrong stall guess (a
// compositor slower than wlFrameTimeout, a window on no output) costs one
// full layout refresh when done arrives, never a stuck window.

// syncOccluded tells gui whether the compositor shows the window at all.
// The show marks a full refresh and wakes the loop once.
// DispatchWindowOccluded ignores a repeat of the current state, so this
// needs no last-sent copy, and unlike focus it needs no ready gate: it
// only flips a flag on the gui window.
func (ww *wlWindow) syncOccluded() {
	gui.DispatchWindowOccluded(ww.b.plat.w, ww.pendSuspended || ww.stalled)
}

// setStalled records whether the frame callback stalled and passes any
// change on to gui.
func (ww *wlWindow) setStalled(stalled bool) {
	if ww.stalled == stalled {
		return
	}
	ww.stalled = stalled
	ww.syncOccluded()
}

// checkStall marks the window stalled once its frame callback has been
// outstanding for wlFrameTimeout. The loop calls it for a window no
// longer waiting on its callback. A window with none outstanding
// (VSyncOff, or the callback answered) cannot stall.
func (ww *wlWindow) checkStall(now time.Time) {
	if ww.frameCb.Valid() && now.Sub(ww.frameAt) >= wlFrameTimeout {
		ww.setStalled(true)
	}
}
