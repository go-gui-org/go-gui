//go:build !darwin && !js && !android && !gl && !windows && !linux

// Package backend provides platform-specific backend initialization.
package backend

import (
	"fmt"
	"runtime"

	"github.com/go-gui-org/go-gui/gui"
)

// Run is not available on this platform.
func Run(w *gui.Window) {
	panic(fmt.Sprintf("backend.Run: unsupported platform %s/%s — no native backend available", runtime.GOOS, runtime.GOARCH))
}

// RunApp is not available on this platform.
func RunApp(app *gui.App, windows ...*gui.Window) {
	panic(fmt.Sprintf("backend.RunApp: unsupported platform %s/%s — no native backend available", runtime.GOOS, runtime.GOARCH))
}
