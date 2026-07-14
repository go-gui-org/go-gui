//go:build ios

// Command ios_demo is an iOS demo app for go-gui.
// Compiled as a c-archive and linked into a native Xcode project.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ios"
)

type App struct {
	Clicks int
}

func init() {
	w := gui.NewWindow(gui.WindowCfg{
		State: &App{},
		OnInit: func(w *gui.Window) {
			w.UpdateView(view)
		},
	})
	ios.SetWindow(w)
}

func view(w *gui.Window) gui.View {
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
				Text:      "Go-Gui on iOS",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Text(gui.TextCfg{
				Text: "Tap the button to increment.",
			}),
			gui.Button(gui.ButtonCfg{
				Focusable: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%d Clicks",
							app.Clicks),
					}),
				},
				OnClick: func(_ *gui.Layout,
					e *gui.Event, w *gui.Window) {
					gui.State[App](w).Clicks++
					e.IsHandled = true
				},
			}),
		},
	})
}

func main() {}
