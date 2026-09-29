// This example demonstrates the app manifest: one appinfo.toml that sets the app's name, ID and version for both buildapp and the running app.
package main

import (
	_ "embed"
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/appinfo"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// doc:snippet-begin appinfo-embed
// The quoted block below is pinned by
// examples/showcase/sound_snippet_test.go against docs/deployment.md.

//go:embed appinfo.toml
var manifest []byte

// info is parsed once at start-up. MustParse panics on a bad file: the
// file is compiled into the program, so a bad one is a build mistake.
var info = appinfo.MustParse(manifest)

func newWindow() *gui.Window {
	// AppInfo gives the window its title, and the app its file-access
	// ID, X11 WM_CLASS and menubar name. No other place spells them.
	return gui.NewWindow(gui.WindowCfg{
		AppInfo: info,
		State:   &App{},
		Width:   480,
		Height:  320,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})
}

// doc:snippet-end appinfo-embed

type App struct{}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	app := gui.NewApp()
	w := newWindow()

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.RunApp(app, w)
}

// mainView lists what the window read from appinfo.toml.
func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	ai := w.AppInfo()
	row := func(key, value string) gui.View {
		return gui.Row(gui.ContainerCfg{
			Padding:    gui.PaddingNone,
			VAlign:     gui.VAlignMiddle,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: key, TextStyle: theme.TextStyleLabel, MinWidth: 80}),
				gui.Text(gui.TextCfg{Text: value, TextStyle: theme.TextStyleCode}),
			},
		})
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    theme.PaddingLarge,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: ai.Name, TextStyle: theme.TextStyleDisplay}),
			gui.Text(gui.TextCfg{
				Text:      "Read from the embedded appinfo.toml:",
				TextStyle: theme.TextStyleSecondary,
			}),
			row("id", ai.ID),
			row("version", ai.Version),
			row("build", ai.Build),
			row("title", w.Config.Title),
		},
	})
}
