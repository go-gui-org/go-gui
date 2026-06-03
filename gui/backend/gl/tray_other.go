//go:build !linux && !js

package gl

import "github.com/go-gui-org/go-gui/gui"

// System tray — no-op on non-Linux GL.
func (n *nativePlatform) CreateSystemTray(
	_ gui.SystemTrayCfg, _ func(string),
) (int, error) {
	return 0, nil
}

func (n *nativePlatform) UpdateSystemTray(_ int, _ gui.SystemTrayCfg) {}
func (n *nativePlatform) RemoveSystemTray(_ int)                      {}
