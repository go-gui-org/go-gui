//go:build linux && !js && !android && !gl

package backend

import (
	"github.com/go-gui-org/go-gui/gui"
	glbackend "github.com/go-gui-org/go-gui/gui/backend/gl"
)

// Run starts the application event loop using the native X11 + EGL
// backend.
func Run(w *gui.Window) { glbackend.Run(w) }

// RunApp starts a multi-window event loop using the native X11 + EGL
// backend.
func RunApp(app *gui.App, windows ...*gui.Window) {
	glbackend.RunApp(app, windows...)
}
