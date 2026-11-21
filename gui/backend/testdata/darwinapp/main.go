// Command darwinapp is the smallest app that calls backend.Run. The
// tests in run_darwin_build_test.go build it for darwin.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

func main() {
	backend.Run(gui.NewWindow(gui.WindowCfg{}))
}
