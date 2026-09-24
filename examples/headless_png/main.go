// This example renders a window to a PNG with no GPU and no window on
// screen, using the software rasterizer in gui/backend/soft.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type App struct {
	Clicks int
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit (overrides positional arg)")
	flag.Parse()
	out := "headless.png"
	if *screenshot != "" {
		out = *screenshot
	} else if flag.NArg() > 0 {
		out = flag.Arg(0)
	}
	// Clean the CLI-supplied path and ensure its directory exists, so
	// "out/dir/headless.png" writes instead of failing on Mkdir-less
	// output paths.
	out = filepath.Clean(out)
	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			log.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	// A window built exactly as it would be for backend.Run — the
	// software renderer runs OnInit itself, as a backend does.
	w := gui.SimpleWindow("Headless", 320, 200, &App{Clicks: 3},
		func(w *gui.Window) {
			w.SetView(mainView)
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
			gui.Label("Rendered without a GPU", gui.CurrentTheme().TextStyleDisplay),
			gui.Label(fmt.Sprintf("%d Clicks", app.Clicks), gui.CurrentTheme().TextStyleBody),
		},
	})
}
