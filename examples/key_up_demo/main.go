// This example demonstrates key-down and key-up event handling.
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
	gui.SetTheme(gui.ThemeDark)

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

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
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
				ID:   "kud_input",
				Text: "Type here to test key up events...",
				OnKeyDown: func(ctx gui.EventCtx) {
					app := gui.State[App](ctx.Window)
					app.keyDownCount++
					app.lastKeyDown = ctx.Event.KeyCode
					ctx.Window.UpdateWindow()
				},
				OnKeyUp: func(ctx gui.EventCtx) {
					app := gui.State[App](ctx.Window)
					app.keyUpCount++
					app.lastKeyUp = ctx.Event.KeyCode
					ctx.Window.UpdateWindow()
				},
			}),
		},
	})
}
