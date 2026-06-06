//go:build !darwin && !js && !android && gl

// Package backend provides platform-specific backend initialization.
package backend

import (
	"github.com/go-gui-org/go-gui/gui"
	glbackend "github.com/go-gui-org/go-gui/gui/backend/gl"
)

// Run starts the application event loop using the OpenGL backend.
func Run(w *gui.Window) { glbackend.Run(w) }

// RunApp starts a multi-window event loop using the OpenGL backend.
func RunApp(app *gui.App, windows ...*gui.Window) {
	glbackend.RunApp(app, windows...)
}
