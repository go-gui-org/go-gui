//go:build linux && !js && !android

package gl

import "github.com/go-gui-org/go-gui/gui/backend/internal/screensaver"

// SetKeepAwake begins or ends the process's request to keep the display
// on (issue #1172), through the freedesktop ScreenSaver D-Bus service,
// on X11 and Wayland alike. A desktop without that service (sway,
// Hyprland) refuses it, and gui reports the error.
func (n *nativePlatform) SetKeepAwake(on bool, app, reason string) error {
	return screensaver.Set(on, app, reason)
}
