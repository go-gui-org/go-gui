// System tray demonstrates a tray icon with menu. Closing the
// window keeps the app alive via ExitOnTrayRemoved. The tray
// menu can re-show the window or quit.
package main

import (
	_ "embed"
	"fmt"
	"log"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

//go:embed icon.png
var trayIcon []byte

type App struct {
	Status string
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	app := gui.NewApp()
	app.ExitMode = gui.ExitOnTrayRemoved

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Status: "Running. Close window — tray keeps app alive."},
		Title:  "System Tray Demo",
		Width:  500,
		Height: 300,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)

			_, err := app.SetSystemTray(gui.SystemTrayCfg{
				Tooltip: "Go-GUI Tray Demo",
				IconPNG: trayIcon,
				Menu: []gui.NativeMenuItemCfg{
					{ID: "show", Text: "Show Window"},
					{ID: "prefs", Text: "Preferences"},
					{Separator: true},
					{ID: "quit", Text: "Quit"},
				},
				OnAction: func(id string) {
					w.QueueCommand(func(w *gui.Window) {
						s := gui.State[App](w)
						s.Status = fmt.Sprintf(
							"Tray action: %s", id)
					})
				},
			})
			if err != nil {
				log.Printf("tray: %v", err)
			}
		},
	})

	backend.RunApp(app, w)
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
				Text:      "System Tray Demo",
				TextStyle: theme.B1,
			}),
			gui.Text(gui.TextCfg{
				Text:      app.Status,
				TextStyle: theme.M3,
			}),
		},
	})
}
