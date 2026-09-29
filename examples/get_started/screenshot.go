package main

// This file is not part of the example. It lets the explorer capture tool
// (examples/explorer/cmd/capture) write screenshot.png without a window:
//
//	go run ./examples/get_started -screenshot <file>
//
// It lives apart from main.go so that main.go stays a short introduction.

import (
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// init runs before main. With "-screenshot <file>" it renders one frame to a
// PNG and exits, so main never opens a window. Any other arguments (for
// example the -test.* flags of go test) fall through to main unchanged.
func init() {
	if len(os.Args) != 3 || os.Args[1] != "-screenshot" {
		return
	}
	w := gui.SimpleWindow("Get Started", 300, 300, &App{}, func(w *gui.Window) {
		w.SetView(mainView)
	})
	if err := soft.RenderToPNG(w, 2, os.Args[2]); err != nil {
		log.Fatalf("screenshot: %v", err)
	}
	os.Exit(0)
}
