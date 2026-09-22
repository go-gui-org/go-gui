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

// PrefersReducedMotion reports the OS reduce-motion setting
// (issue #757). Source is the GNOME enable-animations key,
// inverted like the Windows SPI flag: animations off means the
// user asked for reduced motion. A missing binary or schema reads
// as no setting, and the app keeps animating, like the color-scheme
// query above. GNOME-family only; other desktops report no setting
// (accepted gap, same as #752). Re-read per call: the setting
// changes while the app runs, and a theme change or SVG load is far
// too rare for a process spawn to matter.
func (n *nativePlatform) PrefersReducedMotion() bool {
	reduced, ok := reducedMotionQuery()
	return ok && reduced
}

// enableAnimationsSchema and enableAnimationsKey name the gsettings
// key reporting whether desktop animations are enabled.
const (
	enableAnimationsSchema = "org.gnome.desktop.interface"
	enableAnimationsKey    = "enable-animations"
)

// queryPrefersReducedMotion runs `gsettings get` once for the
// enable-animations key.
func queryPrefersReducedMotion() (bool, bool) {
	out, err := exec.Command("gsettings", "get", enableAnimationsSchema, enableAnimationsKey).Output()
	if err != nil {
		return false, false
	}
	return parseGsettingsEnableAnimations(string(out))
}

// reducedMotionQuery reads the OS setting. A var so tests can feed
// a reading without spawning gsettings.
var reducedMotionQuery = queryPrefersReducedMotion

// SetSystemAppearanceCallback implements gui.NativePlatform. The
// shared monitor runs while at least one window is subscribed; a nil
// cb unregisters.
func (n *nativePlatform) SetSystemAppearanceCallback(cb func(gui.Appearance)) {
	setAppearanceCallback(n, cb)
}
