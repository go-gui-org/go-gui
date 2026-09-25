// This example demonstrates a tray icon with a menu and re-show and quit actions (advanced: system tray).
// System tray demonstrates a tray icon with menu. Closing the
// window keeps the app alive via ExitOnTrayRemoved. The tray
// menu can re-show the window or quit.
package main

import (
	_ "embed"
	"log"

	"flag"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

//go:embed icon.png
var trayIcon []byte

type App struct {
	Status string
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	app := gui.NewApp()
	app.ExitMode = gui.ExitOnTrayRemoved

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Status: "Running. Close window — tray keeps app alive."},
		Title:  "System Tray Demo",
		Width:  500,
		Height: 300,
		// Closing hides instead of destroying: the window stays
		// registered, so the tray menu can bring it back with
		// Show and ExitOnTrayRemoved keeps the app alive.
		OnCloseRequest: func(w *gui.Window) {
			w.Hide()
		},
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)

			var tray *gui.SystemTrayHandle
			tray, err := app.SetSystemTray(gui.SystemTrayCfg{
				Tooltip: "Go-GUI Tray Demo",
				IconPNG: trayIcon,
				Menu: []gui.NativeMenuItemCfg{
					{ID: "show", Text: "Show Window"},
					{ID: "prefs", Text: "Preferences"},
					{Separator: true},
					{ID: "quit", Text: "Quit"},
				},
				OnAction: func(id string) {
					switch id {
					case "quit":
						// Quit means removing the tray too:
						// ExitOnTrayRemoved exits only when no
						// window and no tray remain.
						app.RemoveSystemTray(tray)
						gui.DispatchQuitRequest(app)
						return
					case "show":
						w.QueueCommand(func(w *gui.Window) {
							s := gui.State[App](w)
							if w.IsVisible() {
								s.Status =
									"Window already visible."
								return
							}
							w.Show()
							s.Status = "Window shown from tray."
						})
						return
					}
					w.QueueCommand(func(w *gui.Window) {
						s := gui.State[App](w)
						s.Status = "Tray action: " + id
					})
				},
			})
			if err != nil {
				log.Printf("tray: %v", err)
				// The status line promises the tray keeps the
				// app alive; without a tray that is false, so
				// say the app runs tray-less instead.
				w.QueueCommand(func(w *gui.Window) {
					gui.State[App](w).Status =
						"Running without tray: " + err.Error()
				})
			}
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
				Text:      "System Tray Demo",
				TextStyle: theme.TextStyleDisplay,
			}),
			gui.Text(gui.TextCfg{
				Text:      app.Status,
				TextStyle: theme.TextStyleCode,
			}),
		},
	})
}
