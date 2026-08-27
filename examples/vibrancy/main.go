// This example demonstrates a translucent, blurred native window backdrop on macOS.
// Vibrancy demonstrates a translucent, blurred native window backdrop on
// macOS via w.SetWindowVibrancy. The window BgColor is translucent (alpha <
// 255) so the NSVisualEffectView behind the content shows through. On other
// platforms SetWindowVibrancy is a no-op and the window renders normally.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type App struct {
	Material gui.VibrancyMaterial
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State: &App{Material: gui.VibrancyUnderWindow},
		Title: "vibrancy",
		Width: 360,
		// Near-transparent background so the vibrancy backdrop dominates;
		// a faint tint keeps text legible over the blur.
		BgColor: gui.RGBA(20, 20, 30, 24),
		Height:  260,
		OnInit: func(w *gui.Window) {
			w.SetWindowVibrancy(gui.State[App](w).Material)
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

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Vibrant window (macOS)",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Button(gui.ButtonCfg{
				ID: "vibrancy_button",
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Cycle material"}),
				},
				OnClick: func(ctx gui.EventCtx) {
					app := gui.State[App](ctx.Window)
					// Cycle through the materials, wrapping back to Sidebar.
					app.Material++
					if app.Material > gui.VibrancyUnderWindow {
						app.Material = gui.VibrancySidebar
					}
					ctx.Window.SetWindowVibrancy(app.Material)
				},
			}),
		},
	})
}
