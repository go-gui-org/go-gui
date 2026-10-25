// Command nocgodarwin is the smallest app that calls backend.Run. The
// test in run_nocgo_test.go links it for darwin with cgo off.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

func main() {
	backend.Run(gui.NewWindow(gui.WindowCfg{}))
}
