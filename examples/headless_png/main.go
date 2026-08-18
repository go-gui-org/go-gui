// This example renders a window to a PNG with no GPU and no window on
// screen, using the software rasterizer in gui/backend/soft.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type App struct {
	Clicks int
}

func main() {
	out := "headless.png"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	// A window built exactly as it would be for backend.Run — the
	// software renderer runs OnInit itself, as a backend does.
	w := gui.SimpleWindow("Headless", 320, 200, &App{Clicks: 3},
		func(w *gui.Window) {
			w.UpdateView(mainView)
		})

	// Scale 2 captures at Retina density; 1 is one device pixel per
	// logical pixel.
	if err := soft.RenderToPNG(w, 2, out); err != nil {
		log.Fatalf("render: %v", err)
	}
	fmt.Printf("wrote %s\n", out)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SomeF(12),
		Content: []gui.View{
			gui.Label("Rendered without a GPU", gui.CurrentTheme().B1),
			gui.TextButton("hp_counter",
				fmt.Sprintf("%d Clicks", app.Clicks), nil),
		},
	})
}
