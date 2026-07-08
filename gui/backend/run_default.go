//go:build !darwin && !js && !android && !gl && !windows

// Package backend provides platform-specific backend initialization.
package backend

import (
	"github.com/go-gui-org/go-gui/gui"
	sdl2backend "github.com/go-gui-org/go-gui/gui/backend/sdl2"
)

// Run starts the application event loop using the SDL2 renderer backend.
func Run(w *gui.Window) { sdl2backend.Run(w) }

// RunApp starts a multi-window event loop using the SDL2 renderer backend.
func RunApp(app *gui.App, windows ...*gui.Window) {
	sdl2backend.RunApp(app, windows...)
}
