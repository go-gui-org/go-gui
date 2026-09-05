// This example demonstrates a window whose background is see-through.
// Transparent shows WindowCfg.Transparent: the window's alpha channel
// reaches the compositor, so the desktop behind shows through wherever the
// content is not opaque. Unlike vibrancy there is no blur, and it works on
// macOS, Windows and X11. On X11 it needs a running compositing manager.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// App tracks the tint the card is drawn with, so the difference between a
// see-through window and an opaque widget inside it stays visible.
type App struct {
	Opaque bool
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "transparent",
		Width:  380,
		Height: 240,
		// Transparent on its own is enough: an unset BgColor is treated
		// as fully clear rather than taking the opaque theme background.
		Transparent: true,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	t := gui.CurrentTheme()

	// The card is the only opaque thing in the window, so everything
	// around it is desktop. Toggling its alpha shows the window itself
	// is see-through, not just its edges.
	card := t.ColorPanel
	if !app.Opaque {
		card = gui.RGBA(card.R, card.G, card.B, 160)
	}

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		// Structural wrapper: an unset border still reserves height.
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Color:   card,
				Padding: gui.PadAll(t.SpacingLarge),
				Spacing: gui.SomeF(t.SpacingMedium),
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "See-through window",
						TextStyle: t.B1,
					}),
					gui.Text(gui.TextCfg{
						Text:      "Drag me over something colourful.",
						TextStyle: t.TextStyleSecondary,
					}),
					gui.Button(gui.ButtonCfg{
						ID: "toggle_card",
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Toggle card opacity"}),
						},
						OnClick: func(ctx gui.EventCtx) {
							gui.State[App](ctx.Window).Opaque = !app.Opaque
							ctx.Consume()
						},
					}),
				},
			}),
		},
	})
}
