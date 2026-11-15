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
	// Tray is the tray icon, or nil when the desktop has none. Without
	// one, nothing could bring a hidden window back.
	Tray *gui.SystemTrayHandle
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	app := gui.NewApp()
	app.ExitMode = gui.ExitOnTrayRemoved

	w := gui.NewWindow(gui.WindowCfg{
		State:          &App{Status: "Running. Close window — tray keeps app alive."},
		Title:          "System Tray Demo",
		Width:          500,
		Height:         300,
		OnCloseRequest: hideToTrayOrClose,
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
						quitFromTray(app, tray)
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
			if err == nil {
				gui.State[App](w).Tray = tray
			} else {
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

// hideToTrayOrClose answers a click on the window's close button.
// Closing hides instead of destroying: the window stays registered, so
// the tray menu can bring it back with Show, and ExitOnTrayRemoved
// keeps the app alive. Where that cannot work, closing quits the app,
// so the close button never does nothing: with no tray (a desktop
// without one, such as stock GNOME), and where a window cannot be
// hidden (native Wayland), which Hide shows by leaving IsVisible true.
// The tray goes too, so no icon stays behind whose "Show Window" has
// no window to show.
func hideToTrayOrClose(w *gui.Window) {
	tray := gui.State[App](w).Tray
	if tray != nil {
		w.Hide()
		if !w.IsVisible() {
			return
		}
	}
	quitFromTray(w.App(), tray)
	w.Close() // also when the window is not registered with an app
}

// quitFromTray ends the app. ExitOnTrayRemoved exits only when no
// window and no tray remain, so the tray and every window go. The
// windows are closed with Close, not with a quit request: a quit
// request asks OnCloseRequest, which may only hide the window.
func quitFromTray(app *gui.App, tray *gui.SystemTrayHandle) {
	app.RemoveSystemTray(tray)
	for _, w := range app.Windows() {
		w.Close()
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
