//go:build !js && !android

package gl

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// parseGsettingsColorScheme maps gsettings output to an appearance
// (issue #752). Accepts both `get` output ('prefer-dark') and
// `monitor` lines (org.gnome.desktop.interface color-scheme:
// 'prefer-dark'). 'default' maps to light. Anything else is no
// setting. Shared (not Linux-only) so its test runs everywhere.
func parseGsettingsColorScheme(s string) (gui.Appearance, bool) {
	switch {
	case strings.Contains(s, "prefer-dark"):
		return gui.AppearanceDark, true
	case strings.Contains(s, "prefer-light"), strings.Contains(s, "default"):
		return gui.AppearanceLight, true
	default:
		return gui.AppearanceLight, false
	}
}

// parseGsettingsEnableAnimations maps `gsettings get
// org.gnome.desktop.interface enable-animations` output to a
// reduced-motion report (issue #757). The key reports animations
// enabled, so false inverts to reduced motion. Anything but true
// or false is no setting. Shared (not Linux-only) so its test runs
// everywhere.
func parseGsettingsEnableAnimations(s string) (reduced bool, ok bool) {
	switch {
	case strings.Contains(s, "false"):
		return true, true
	case strings.Contains(s, "true"):
		return false, true
	default:
		return false, false
	}
}
