//go:build linux && !js && !android && !gl

package backend

import (
	"github.com/go-gui-org/go-gui/gui"
	glbackend "github.com/go-gui-org/go-gui/gui/backend/gl"
)

// Run starts the application event loop using the native X11 + EGL
// backend (SDL2-free).
func Run(w *gui.Window) { glbackend.Run(w) }

// RunApp starts a multi-window event loop using the native X11 + EGL
// backend (SDL2-free).
func RunApp(app *gui.App, windows ...*gui.Window) {
	glbackend.RunApp(app, windows...)
}
