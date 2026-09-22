//go:build linux && !js && !android

package gl

import (
	"os/exec"

	"github.com/go-gui-org/go-gui/gui"
)

// OS light/dark setting on Linux (issue #752). Source is gsettings,
// no D-Bus dependency: the initial value comes from `gsettings get`
// and changes from the shared monitor child (appearance_monitor.go).
// GNOME-family only — KDE and bare window managers report no setting
// (accepted gap, see the #752 spec). A missing binary or schema also
// reads as no setting, and the app keeps its own theme.

// querySystemAppearance runs `gsettings get` once.
func querySystemAppearance() (gui.Appearance, bool) {
	out, err := exec.Command("gsettings", "get", gsettingsSchema, gsettingsKey).Output()
	if err != nil {
		return gui.AppearanceLight, false
	}
	return parseGsettingsColorScheme(string(out))
}

// SystemAppearance implements gui.NativePlatform.
func (n *nativePlatform) SystemAppearance() (gui.Appearance, bool) {
	return querySystemAppearance()
}

// SetSystemAppearanceCallback implements gui.NativePlatform. The
// shared monitor runs while at least one window is subscribed; a nil
// cb unregisters.
func (n *nativePlatform) SetSystemAppearanceCallback(cb func(gui.Appearance)) {
	setAppearanceCallback(n, cb)
}
