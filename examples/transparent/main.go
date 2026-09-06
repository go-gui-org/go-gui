// This example demonstrates a window whose background is see-through, and
// a whole-window fade on top of it.
//
// WindowCfg.Transparent is per-pixel: the window's alpha channel reaches
// the compositor, so the desktop behind shows through wherever the content
// is not opaque. Window.SetWindowOpacity is a different mechanism — the
// compositor fades everything the window draws, content included — so the
// slider dims the card too, which the card's own alpha cannot do.
//
// Unlike vibrancy there is no blur. Transparency works on macOS, Windows
// and X11; the fade works on macOS and X11, and on Windows only for a
// window not created Transparent (see docs/specs/window-opacity.md). On
// X11 both need a running compositing manager.
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
//
// Opacity mirrors the window fade. The window already holds the value,
// but the slider needs somewhere writable to stage the drag, so the
// mirror is seeded from the window in OnInit and written through to it
// in OnChange.
type App struct {
	Opacity float32
	Opaque  bool
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
			// The window is the authority on its own fade, so the
			// slider starts from what it reports rather than from a
			// number repeated here.
			gui.State[App](w).Opacity = w.WindowOpacity()
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
					gui.Slider(gui.SliderCfg{
						ID:    "opacity",
						Label: "Window opacity",
						Value: app.Opacity,
						Min:   0.2, // below this the window is hard to find again
						Max:   1,
						Step:  0.05,
						Width: 200,
						OnChange: func(v float32, ctx gui.EventCtx) {
							gui.State[App](ctx.Window).Opacity = v
							ctx.Window.SetWindowOpacity(v)
							ctx.Consume()
						},
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
