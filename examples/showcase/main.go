// This example demonstrates the full go-gui widget gallery with a dev-mode inspector (advanced: inspector).
// Package main implements a faithful showcase port for the Go-Gui framework.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const (
	scrollCatalog = "catalog"
	scrollDetail  = "detail"
)

const catalogWidth float32 = 300

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	gui.SetMarkdownExternalAPIsEnabled(true)

	app := gui.NewApp()
	app.ExitMode = gui.ExitOnMainClose

	w := gui.NewWindow(gui.WindowCfg{
		State:  newShowcaseApp(),
		Title:  "Gui Showcase " + gui.Version,
		Width:  950,
		Height: 700,
		OnInit: func(w *gui.Window) {
			loadEmbeddedLocales()
			sa := appState(w)
			syncThemeGenFromCfg(sa, gui.CurrentTheme().Cfg)
			_ = w.RegisterCommands(
				gui.Command{
					ID: "sc.greet", Label: "Greet", Icon: gui.IconBell,
					Shortcut: gui.Shortcut{Key: gui.KeyF5},
					Execute: func(_ *gui.Event, w *gui.Window) {
						w.Toast(gui.ToastCfg{Title: "Command", Body: "Hello from CommandButton!"})
					},
				},
				gui.Command{
					ID: "sc.count", Label: "Count", Icon: gui.IconPlus,
					Shortcut: gui.Shortcut{Key: gui.KeyF5, Modifiers: gui.ModShift},
					Execute: func(_ *gui.Event, w *gui.Window) {
						appState(w).CmdButtonCount++
					},
				},
				gui.Command{
					ID: "sc.disabled", Label: "Delete", Icon: gui.IconTrash,
					CanExecute: func(_ *gui.Window) bool { return false },
				},
			)
			w.UpdateView(mainView)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	defer cleanupEmbeddedAssets()
	backend.RunApp(app, w)
}

func mainView(w *gui.Window) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.NoPadding,
		Spacing: gui.NoSpacing,
		Content: []gui.View{catalogPanel(w), detailPanel(w)},
	})
}
