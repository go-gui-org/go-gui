//go:build js && wasm

package backend

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/web"
)

// Run starts the GUI event loop using the Canvas2D web backend.
func Run(w *gui.Window) { web.Run(w) }

// RunApp starts a multi-window event loop. On web, only the
// first window is used; additional windows are ignored.
func RunApp(app *gui.App, windows ...*gui.Window) {
	if len(windows) > 0 {
		web.Run(windows[0])
	}
}
