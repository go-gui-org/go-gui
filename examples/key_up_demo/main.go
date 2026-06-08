package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	keyDownCount int
	keyUpCount   int
	lastKeyDown  gui.KeyCode
	lastKeyUp    gui.KeyCode
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "Key Up Demo",
		Width:  400,
		Height: 300,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	ww, wh := w.WindowSize()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SomeF(10),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: "Press any keys to see key down/up events!",
			}),
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("Key Down: %d (Last: %v)", app.keyDownCount, app.lastKeyDown),
			}),
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("Key Up: %d (Last: %v)", app.keyUpCount, app.lastKeyUp),
			}),
			gui.Input(gui.InputCfg{
				IDFocus: 1,
				Text:    "Type here to test key up events...",
				OnKeyDown: func(layout *gui.Layout, e *gui.Event, w *gui.Window) {
					app := gui.State[App](w)
					app.keyDownCount++
					app.lastKeyDown = e.KeyCode
					w.UpdateWindow()
				},
				OnKeyUp: func(layout *gui.Layout, e *gui.Event, w *gui.Window) {
					app := gui.State[App](w)
					app.keyUpCount++
					app.lastKeyUp = e.KeyCode
					w.UpdateWindow()
				},
			}),
		},
	})
}
