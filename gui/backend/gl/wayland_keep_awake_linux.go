//go:build linux && !js && !android && (amd64 || arm64)

package gl

import "github.com/go-gui-org/go-gui/gui/backend/internal/wl"

// Keeping the display awake on Wayland (issue #1287), through the
// idle-inhibit-unstable-v1 protocol. An inhibitor belongs to one surface,
// and sway and KWin honour it only while that surface is visible. gui keeps
// one request for the whole app and calls SetKeepAwake through whichever
// window ran its check, only when the app-wide state changes. So while the
// request is on, every window carries an inhibitor: when the request moves
// from a hidden holder to a visible one, no call comes, and an inhibitor on
// the window that got the first call would sit on a hidden surface.
//
// Without the protocol (weston, Mutter before 45) the D-Bus ScreenSaver
// service is used, as on X11 (keep_awake_linux.go).

// wlKeepAwake is the display's keep-awake state. Main thread.
type wlKeepAwake struct {
	// mgr is the bound zwp_idle_inhibit_manager_v1, zero when the
	// compositor does not offer it.
	mgr wl.ZwpIdleInhibitManagerV1
	// on says the app-wide request is on and went through mgr.
	on bool
	// inhibitors holds each window's inhibitor while on is true.
	inhibitors map[*wlWindow]wl.ZwpIdleInhibitorV1
}

// waylandKeepAwake applies the app-wide request through the protocol. It
// reports false when the process has no Wayland display, or the
// compositor does not offer the protocol: the caller then uses D-Bus.
//
// The choice follows the process, not the window gui called through: it
// assumes every open window uses one window system, as RunApp does. Two
// windows from separate New calls can differ (a Wayland window, then one
// that fell back to X11), but running both at once is not supported: the
// Wayland connection belongs to one thread, and an X11 window's call
// would reach it from another. An X11 window holding the request beside
// an open Wayland display would get no D-Bus request.
func waylandKeepAwake(on bool) bool {
	d := wlShared
	if d == nil || !d.keepAwake.mgr.Valid() {
		return false
	}
	d.keepAwake.on = on
	for _, b := range d.wins {
		d.keepAwake.sync(b.plat.wl)
	}
	return true
}

// sync gives ww an inhibitor while the request is on, and takes it away
// while it is off. It runs for every window on a change, and for a window
// that opens while the request is on.
func (k *wlKeepAwake) sync(ww *wlWindow) {
	inh, has := k.inhibitors[ww]
	switch {
	case k.on && !has:
		if k.inhibitors == nil {
			k.inhibitors = map[*wlWindow]wl.ZwpIdleInhibitorV1{}
		}
		k.inhibitors[ww] = k.mgr.CreateInhibitor(ww.surface)
	case !k.on && has:
		inh.Destroy()
		delete(k.inhibitors, ww)
	}
}

// drop destroys ww's inhibitor, if it has one. A closing window calls it
// before it destroys its surface.
func (k *wlKeepAwake) drop(ww *wlWindow) {
	if inh, ok := k.inhibitors[ww]; ok {
		inh.Destroy()
		delete(k.inhibitors, ww)
	}
}

// destroy frees the manager when the display closes. Every window, with
// its inhibitor, is gone by then.
func (k *wlKeepAwake) destroy() {
	if k.mgr.Valid() {
		k.mgr.Destroy()
	}
	k.mgr = wl.ZwpIdleInhibitManagerV1{}
	k.on = false
}
