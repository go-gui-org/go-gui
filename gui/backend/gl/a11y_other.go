//go:build !linux && !js && !darwin

package gl

import "github.com/go-gui-org/go-gui/gui"

func (n *nativePlatform) A11yInit(_ func(action, index int))  {}
func (n *nativePlatform) A11ySync(_ []gui.A11yNode, _, _ int) {}
func (n *nativePlatform) A11yDestroy() {
	// Off Linux there is never a bridge, so this is already nil. The
	// assignment keeps the Linux-only field referenced on platforms
	// whose stubs never touch it (windows cross-build trips unused
	// otherwise).
	n.a11y = nil
}
func (n *nativePlatform) A11yAnnounce(_ string) {}
