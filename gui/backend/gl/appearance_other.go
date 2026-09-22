//go:build !js && !darwin && !android && !windows && !linux

package gl

import (
	"github.com/go-gui-org/go-gui/gui"
)

// No OS appearance setting on this platform (issue #752): queries
// report no setting and callbacks never fire.
func (n *nativePlatform) SystemAppearance() (gui.Appearance, bool) {
	return gui.AppearanceLight, false
}

// SetSystemAppearanceCallback implements gui.NativePlatform as a no-op.
func (n *nativePlatform) SetSystemAppearanceCallback(_ func(gui.Appearance)) {}
