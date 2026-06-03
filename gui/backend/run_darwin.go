//go:build darwin && !ios

// Package backend provides platform-specific backend initialization.
package backend

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/metal"
)

// Run starts the GUI event loop.
func Run(w *gui.Window) { metal.Run(w) }

// RunApp starts a multi-window event loop.
func RunApp(app *gui.App, windows ...*gui.Window) {
	metal.RunApp(app, windows...)
}
