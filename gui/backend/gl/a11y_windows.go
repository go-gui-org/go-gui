//go:build windows && !js

package gl

import "github.com/go-gui-org/go-gui/gui"

// A11yInit attaches a UI Automation provider to the window; the window
// answers WM_GETOBJECT with it from then on.
func (n *nativePlatform) A11yInit(cb func(action, index int)) {
	if n.b == nil || n.b.plat.hwnd == 0 {
		return
	}
	// Initializing twice must not leave the first provider's elements
	// connected.
	n.A11yDestroy()
	n.b.plat.uia = newUIAProvider(n.b.plat.hwnd, cb)
}

func (n *nativePlatform) A11ySync(nodes []gui.A11yNode, count, focusedIdx int) {
	if n.b == nil || n.b.plat.uia == nil {
		return
	}
	count = max(0, min(count, len(nodes)))
	n.b.plat.uia.sync(nodes[:count], focusedIdx, n.b.dpiScale)
}

func (n *nativePlatform) A11yDestroy() {
	// Off Linux there is never an AT-SPI bridge, so this is already
	// nil. The assignment keeps the Linux-only field referenced.
	n.a11y = nil
	if n.b != nil && n.b.plat.uia != nil {
		n.b.plat.uia.destroy()
		n.b.plat.uia = nil
	}
}

func (n *nativePlatform) A11yAnnounce(text string) {
	if n.b != nil && n.b.plat.uia != nil {
		n.b.plat.uia.announce(text)
	}
}

// wmGetObject is WM_GETOBJECT, which UI Automation and MSAA send to
// ask the window for its accessible root.
const wmGetObject = 0x003D
