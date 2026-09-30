// This example demonstrates the smallest stateful go-gui app: one button and one counter.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// App holds the window state. The view reads it on every frame.
type App struct {
	Clicks int
}

func main() {
	w := gui.SimpleWindow("Get Started", 300, 300, &App{}, func(w *gui.Window) {
		w.SetView(mainView)
	})
	backend.Run(w)
}

// mainView builds the UI from the current state. It runs again for each frame.
func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Label("Hello GUI!", gui.CurrentTheme().TextStyleDisplay),
			gui.Label(fmt.Sprintf("%d Clicks", app.Clicks), gui.TextStyle{}),
			gui.TextButton("Click Me", func(ctx gui.EventCtx) {
				// Change the state. The next frame shows the new count.
				gui.State[App](ctx.Window).Clicks++
			}),
		},
	})
}
