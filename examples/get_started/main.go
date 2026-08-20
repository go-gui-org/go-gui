// This example demonstrates the smallest stateful go-gui app: one button and one counter.
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
	w := gui.SimpleWindow("Get Started", 300, 300, &App{}, func(w *gui.Window) {
		w.UpdateView(mainView)
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Label("Hello GUI! 😀🚀🎉👍", gui.CurrentTheme().B1),
			gui.TextButton("gs_counter", fmt.Sprintf("%d Clicks", app.Clicks), func(ctx gui.EventCtx) {
				// Update the typed window state; the next frame reads it back.
				gui.State[App](ctx.Window).Clicks++
			}),
			// A primary button is the accent-filled call to action —
			// the convention is one per surface (docs/style-guide.md).
			gui.TextButtonVariant("gs_primary", "Reset", gui.ButtonPrimary, func(ctx gui.EventCtx) {
				gui.State[App](ctx.Window).Clicks = 0
			}),
		},
	})
}
