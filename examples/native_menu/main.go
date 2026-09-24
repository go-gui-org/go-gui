// This example demonstrates a native macOS menubar with an auto-wired Edit menu (advanced: native menus).
// Native menu demonstrates a native macOS menubar with
// auto-wired Edit menu, command integration, and status
// feedback.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type App struct {
	LastAction string
	Sidebar    bool
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	app := gui.NewApp()

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{LastAction: "Ready."},
		Title:  "Native Menu Demo",
		Width:  600,
		Height: 400,
		OnInit: func(w *gui.Window) {
			registerCommands(w)
			w.SetView(mainView)

			app.SetNativeMenubar(gui.NativeMenubarCfg{
				AppName:         "Native Menu Demo",
				IncludeEditMenu: true,
				Menus: []gui.NativeMenuCfg{
					{
						Title: "File",
						Items: []gui.NativeMenuItemCfg{
							{ID: "file.new", Text: "New", CommandID: "file.new",
								Shortcut: gui.Shortcut{Key: gui.KeyN, Modifiers: gui.ModSuper}},
							{ID: "file.open", Text: "Open", CommandID: "file.open",
								Shortcut: gui.Shortcut{Key: gui.KeyO, Modifiers: gui.ModSuper}},
							{ID: "file.save", Text: "Save", CommandID: "file.save",
								Shortcut: gui.Shortcut{Key: gui.KeyS, Modifiers: gui.ModSuper}},
							{Separator: true},
							{ID: "file.quit", Text: "Quit"},
						},
					},
					{
						Title: "View",
						Items: []gui.NativeMenuItemCfg{
							{ID: "view.sidebar", Text: "Toggle Sidebar"},
						},
					},
					{
						Title: "Help",
						Items: []gui.NativeMenuItemCfg{
							{ID: "help.about", Text: "About"},
						},
					},
				},
				OnAction: func(id string) {
					// file.quit has no CommandID: quit here through
					// the same DispatchQuitRequest the backend runs
					// for an OS quit event.
					if id == "file.quit" {
						gui.DispatchQuitRequest(app)
						return
					}
					w.QueueCommand(func(w *gui.Window) {
						gui.State[App](w).LastAction =
							"Action: " + id
					})
				},
			})
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.RunApp(app, w)
}

func registerCommands(w *gui.Window) {
	if err := w.RegisterCommands(
		gui.Command{
			ID:       "file.new",
			Label:    "New",
			Shortcut: gui.Shortcut{Key: gui.KeyN, Modifiers: gui.ModSuper},
			Global:   true,
			Execute: func(_ *gui.Event, w *gui.Window) {
				gui.State[App](w).LastAction = "New file"
			},
		},
		gui.Command{
			ID:       "file.open",
			Label:    "Open",
			Shortcut: gui.Shortcut{Key: gui.KeyO, Modifiers: gui.ModSuper},
			Global:   true,
			Execute: func(_ *gui.Event, w *gui.Window) {
				gui.State[App](w).LastAction = "Open file"
			},
		},
		gui.Command{
			ID:       "file.save",
			Label:    "Save",
			Shortcut: gui.Shortcut{Key: gui.KeyS, Modifiers: gui.ModSuper},
			Global:   true,
			Execute: func(_ *gui.Event, w *gui.Window) {
				gui.State[App](w).LastAction = "Saved"
			},
		},
	); err != nil {
		log.Fatalf("register commands: %v", err)
	}
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		// Structural wrapper: an unset border still reserves height.
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Rectangle(gui.RectangleCfg{
				Height: 40,
				Sizing: gui.FillFixed,
				// Primitive border: 0 is an explicit no-border.
				SizeBorder: 0,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Native Menu Demo",
				TextStyle: theme.TextStyleDisplay,
			}),
			gui.Text(gui.TextCfg{
				Text:      app.LastAction,
				TextStyle: theme.TextStyleCode,
			}),
		},
	})
}
