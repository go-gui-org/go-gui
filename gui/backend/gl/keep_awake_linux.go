//go:build linux && !js && !android

package gl

import "github.com/go-gui-org/go-gui/gui/backend/internal/screensaver"

// screensaverSet is screensaver.Set, a variable so tests can watch the
// fallback.
var screensaverSet = screensaver.Set

// SetKeepAwake begins or ends the process's request to keep the display
// on (issue #1172). On Wayland it uses the idle-inhibit protocol when the
// compositor offers it (issue #1287, wayland_keep_awake_linux.go). On X11,
// and on a compositor without the protocol (weston, Mutter before 45), it
// asks the freedesktop ScreenSaver D-Bus service; a desktop without that
// service refuses, and gui reports the error.
//
// Off also ends a D-Bus request: that does nothing when none is held, and
// so off undoes what on did whichever way on went.
func (n *nativePlatform) SetKeepAwake(on bool, app, reason string) error {
	if waylandKeepAwake(on) && on {
		return nil
	}
	return screensaverSet(on, app, reason)
}
