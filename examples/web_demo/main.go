// Web_demo is a go-gui app that runs in the browser via wasm.
// Same source code pattern as get_started — proves cross-platform
// compilation with no wasm-specific code.
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
		Title:  "web_demo",
		Width:  640,
		Height: 480,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Hello from WASM!",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Button(gui.ButtonCfg{
				ID: "web_demo_button",
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%d Clicks", app.Clicks),
					}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					gui.State[App](w).Clicks++
					e.IsHandled = true
				},
			}),
		},
	})
}
