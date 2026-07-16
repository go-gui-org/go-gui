// Get_started is the smallest stateful go-gui app: one button
// and one counter.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	Clicks int
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "Get Started",
		Width:  300,
		Height: 300,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)

	// FillFill fills the window and tracks resize automatically — no
	// need to fetch WindowSize() or set an explicit Width/Height.
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Hello GUI! 😀🚀🎉👍",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Button(gui.ButtonCfg{
				ID: "gs_counter",
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%d Clicks", app.Clicks),
					}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					// Update the typed window state; the next frame reads it back.
					gui.State[App](w).Clicks++
					e.IsHandled = true
				},
			}),
		},
	})
}
