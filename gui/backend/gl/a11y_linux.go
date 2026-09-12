//go:build linux && !android

package gl

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/atspi"
)

func (n *nativePlatform) A11yInit(cb func(int, int)) {
	// Fresh bridge per init, as before: initializing twice must not
	// stack a second live bus connection onto the first.
	n.A11yDestroy()
	n.a11y = &atspi.Bridge{}
	n.a11y.Init(cb)
}

func (n *nativePlatform) A11ySync(nodes []gui.A11yNode, count, focusedIdx int) {
	if n.a11y != nil {
		if count < 0 {
			count = 0
		}
		if count > len(nodes) {
			count = len(nodes)
		}
		n.a11y.Sync(nodes, count, focusedIdx)
	}
}

func (n *nativePlatform) A11yDestroy() {
	if n.a11y != nil {
		n.a11y.Destroy()
		n.a11y = nil
	}
}

func (n *nativePlatform) A11yAnnounce(text string) {
	if n.a11y != nil {
		n.a11y.Announce(text)
	}
}
