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
