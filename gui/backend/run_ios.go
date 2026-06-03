//go:build ios

package backend

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ios"
)

func Run(w *gui.Window) { ios.Run(w) }

// RunApp starts a multi-window event loop. On iOS, only the
// first window is used; additional windows are ignored.
func RunApp(app *gui.App, windows ...*gui.Window) {
	if len(windows) > 0 {
		ios.Run(windows[0])
	}
}
