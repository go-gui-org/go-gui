//go:build windows && !js && !android && !gl

package backend

import (
	"github.com/go-gui-org/go-gui/gui"
	glbackend "github.com/go-gui-org/go-gui/gui/backend/gl"
)

// Run starts the application event loop using the native Win32 + WGL
// backend (SDL2-free).
func Run(w *gui.Window) { glbackend.Run(w) }

// RunApp starts a multi-window event loop. Multi-window is not yet
// supported on the native Windows backend.
func RunApp(app *gui.App, windows ...*gui.Window) {
	glbackend.RunApp(app, windows...)
}
