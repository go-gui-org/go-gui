// This example demonstrates a go-gui app that runs in the browser via wasm (advanced: webassembly).
// Web_demo is a go-gui app that runs in the browser via wasm.
// Same source code pattern as get_started — proves cross-platform
// compilation with no wasm-specific code.
package main

import (
	"fmt"

	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type App struct {
	Clicks int
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	// Pinned for the demo's look: the page is designed around the
	// dark palette (get_started leaves the default unset).
	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "web_demo",
		Width:  640,
		Height: 480,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})

	if *screenshot != "" {
		out := filepath.Clean(*screenshot)
		if out == "" || out == "." {
			log.Fatalf("screenshot: invalid path %q", *screenshot)
		}
		if err := soft.RenderToPNG(w, 2, out); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Hello from WASM!",
				TextStyle: gui.CurrentTheme().TextStyleDisplay,
			}),
			gui.Button(gui.ButtonCfg{
				ID: "web_demo_button",
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%d Clicks", app.Clicks),
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Clicks++
				},
			}),
		},
	})
}
