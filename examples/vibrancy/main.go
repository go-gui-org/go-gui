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
			w.SetView(mainView)
		},
	})

	if *screenshot != "" {
		// Native vibrancy is a window property, not pixels: a capture
		// holds the content without the backdrop.
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

// vibrancyCycle is every exported material: the rest of the enum is
// unexported, so a raw ++ would land on values the app cannot name.
var vibrancyCycle = []gui.VibrancyMaterial{
	gui.VibrancySidebar,
	gui.VibrancyUnderWindow,
}

func materialName(m gui.VibrancyMaterial) string {
	switch m {
	case gui.VibrancySidebar:
		return "Sidebar"
	case gui.VibrancyUnderWindow:
		return "Under Window"
	default:
		return "Unknown"
	}
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Vibrant window (macOS)",
				TextStyle: gui.CurrentTheme().TextStyleDisplay,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Material: " + materialName(app.Material),
				TextStyle: gui.CurrentTheme().TextStyleSecondary,
			}),
			gui.Button(gui.ButtonCfg{
				ID: "vibrancy_button",
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Cycle material"}),
				},
				OnClick: func(ctx gui.EventCtx) {
					app := gui.State[App](ctx.Window)
					next := vibrancyCycle[0]
					for i, m := range vibrancyCycle {
						if m == app.Material {
							next = vibrancyCycle[(i+1)%len(vibrancyCycle)]
							break
						}
					}
					app.Material = next
					ctx.Window.SetWindowVibrancy(app.Material)
				},
			}),
		},
	})
}
