//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// Window state on Wayland (#1171, #1220, docs/specs/window-state.md).
//
// A client asks; the compositor answers with a configure, which is where
// the state comes back from, whoever changed it. Each request is posted
// to the loop: gui calls these from event handlers, and a resize emits
// EventResized from inside wlWindow.resize.
//
// A client has no way to read or set its position, and no request to
// bring a minimized window back.

// wlPlacement is the state a configure carries, pending until the
// xdg_surface.configure that closes the sequence, and the size the
// window goes back to.
type wlPlacement struct {
	pendMaximized, pendFullscreen bool
	maximized, fullscreen         bool
	// floatW, floatH are the logical size outside maximized and
	// fullscreen. A compositor may answer un-maximize with 0×0, which
	// leaves the size to the client, and the client's current size is
	// then the maximized one.
	floatW, floatH int32
	// maxW, maxH are the configured ceiling, kept to restore it, and
	// lifted says it is off for fullscreen: the protocol lets a
	// compositor refuse a state the max size rules out.
	maxW, maxH int32
	lifted     bool
}

// onLoop runs a request on the loop's next pass, if the window is still
// there.
func (ww *wlWindow) onLoop(fn func()) {
	key := ww.surface.Ptr()
	d, b := ww.d, ww.b
	d.post(func() {
		if d.wins[key] == b {
			fn()
		}
	})
}

func (ww *wlWindow) minimize() {
	ww.onLoop(func() {
		if ww.frame != nil {
			ww.frame.SetMinimized()
			return
		}
		ww.toplevel.SetMinimized()
	})
}

func (ww *wlWindow) maximize() {
	ww.onLoop(func() {
		if ww.frame != nil {
			ww.frame.SetMaximized()
			return
		}
		ww.toplevel.SetMaximized()
	})
}

// restore undoes a maximize. No Wayland request undoes a minimize, and
// Restore leaves fullscreen alone.
func (ww *wlWindow) restore() {
	ww.onLoop(func() {
		if ww.place.fullscreen {
			return
		}
		if ww.frame != nil {
			ww.frame.UnsetMaximized()
			return
		}
		ww.toplevel.UnsetMaximized()
	})
}

func (ww *wlWindow) setFullscreen(fullscreen bool) {
	ww.onLoop(func() {
		if fullscreen {
			ww.commitLiftedCeiling()
		}
		switch {
		case ww.frame != nil && fullscreen:
			ww.frame.SetFullscreen()
		case ww.frame != nil:
			ww.frame.UnsetFullscreen()
		case fullscreen:
			// A null output leaves the choice to the compositor.
			ww.toplevel.SetFullscreen(wl.Output{})
		default:
			ww.toplevel.UnsetFullscreen()
		}
	})
}

// liftCeiling takes the max size off or puts it back. The change waits
// for the next surface commit, double-buffered like every xdg state.
func (ww *wlWindow) liftCeiling(lift bool) {
	p := &ww.place
	if lift == p.lifted || (p.maxW == 0 && p.maxH == 0) {
		return
	}
	p.lifted = lift
	w, h := p.maxW, p.maxH
	if lift {
		w, h = 0, 0
	}
	if ww.frame != nil {
		ww.frame.SetMaxContentSize(w, h)
		return
	}
	ww.toplevel.SetMaxSize(w, h)
}

// commitLiftedCeiling lifts the ceiling ahead of a fullscreen request,
// so the compositor has it before it decides. Before the first
// configure there is nothing to commit yet: that configure's own commit
// carries it. A dirty window commits with its next frame instead, since
// a resize staged on the surface must not apply to the old buffer.
func (ww *wlWindow) commitLiftedCeiling() {
	if ww.place.lifted {
		return
	}
	ww.liftCeiling(true)
	if !ww.place.lifted || !ww.configured || ww.dirty {
		return
	}
	if ww.frame != nil {
		ww.frame.Commit(ww.logW, ww.logH, nil)
	}
	ww.surface.Commit()
}

// resizeTo sets a floating window's size, which a client picks itself.
// While the compositor dictates the size, a late request only changes
// the size the window returns to.
func (ww *wlWindow) resizeTo(w, h int32) {
	ww.onLoop(func() {
		ww.place.floatW, ww.place.floatH = w, h
		if ww.place.maximized || ww.place.fullscreen {
			return
		}
		ww.resize(w, h, ww.scale120)
		ww.dirty = true
		if ww.frame != nil {
			ww.commitFrame(w, h, nil)
		}
	})
}

// configuredSize resolves a configure's size. Leaving maximized or
// fullscreen with no size named returns to the floating size.
func (ww *wlWindow) configuredSize(pendW, pendH int32) (int32, int32) {
	curW, curH := ww.logW, ww.logH
	p := &ww.place
	leaving := (p.maximized || p.fullscreen) &&
		!p.pendMaximized && !p.pendFullscreen
	if leaving && p.floatW > 0 && p.floatH > 0 {
		curW, curH = p.floatW, p.floatH
	}
	return configureSize(pendW, pendH, curW, curH)
}

// syncPlacement adopts the pending state and tells gui, before the
// resize in applyPending, so EventResized sees the new state. A floating
// window's size becomes the one to come back to. A compositor may drop
// maximized from the states while fullscreen; the window still returns
// to it, so the answer from outside fullscreen stands.
func (ww *wlWindow) syncPlacement(w, h int32) {
	p := &ww.place
	if !p.pendFullscreen || p.pendMaximized {
		p.maximized = p.pendMaximized
	}
	p.fullscreen = p.pendFullscreen
	if !p.maximized && !p.fullscreen {
		p.floatW, p.floatH = w, h
	}
	if ww.b.plat.w != nil {
		gui.DispatchWindowState(ww.b.plat.w, p.maximized, p.fullscreen)
	}
}
