//go:build linux

package gl

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/atspi"
)

var a11yBridge *atspi.Bridge

func (n *nativePlatform) A11yInit(cb func(int, int)) {
	a11yBridge = &atspi.Bridge{}
	a11yBridge.Init(cb)
}

func (n *nativePlatform) A11ySync(nodes []gui.A11yNode, count, focusedIdx int) {
	if a11yBridge != nil {
		a11yBridge.Sync(nodes, count, focusedIdx)
	}
}

func (n *nativePlatform) A11yDestroy() {
	if a11yBridge != nil {
		a11yBridge.Destroy()
	}
}

func (n *nativePlatform) A11yAnnounce(text string) {
	if a11yBridge != nil {
		a11yBridge.Announce(text)
	}
}
