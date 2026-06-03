// Native menu demonstrates a native macOS menubar with
// auto-wired Edit menu, command integration, and status
// feedback.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	LastAction string
	Sidebar    bool
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	app := gui.NewApp()

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{LastAction: "Ready."},
		Title:  "Native Menu Demo",
		Width:  600,
		Height: 400,
		OnInit: func(w *gui.Window) {
			registerCommands(w)
			w.UpdateView(mainView)

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
					w.QueueCommand(func(w *gui.Window) {
						gui.State[App](w).LastAction =
							fmt.Sprintf("Action: %s", id)
					})
				},
			})
		},
	})

	backend.RunApp(app, w)
}

func registerCommands(w *gui.Window) {
	_ = w.RegisterCommands(
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
	)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		HAlign: gui.HAlignCenter,
		Content: []gui.View{
			gui.Rectangle(gui.RectangleCfg{
				Height: 40,
				Sizing: gui.FillFixed,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Native Menu Demo",
				TextStyle: theme.B1,
			}),
			gui.Text(gui.TextCfg{
				Text:      app.LastAction,
				TextStyle: theme.M3,
			}),
		},
	})
}
