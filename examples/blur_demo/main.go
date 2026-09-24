// This example demonstrates blur radius on shapes to create glows and soft-edged shapes.
// The blur demo shows how blur radius can create glows and
// soft-edged shapes.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark.WithPadding(false))

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Gaussian Blur / Glow Demo",
		Width:  800,
		Height: 800,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
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

func mainView(_ *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.Some[float32](60),
		Padding: gui.NewPadding(40, 40, 40, 40),
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			// Each row highlights a different blur and radius combination.
			gui.Text(gui.TextCfg{
				Text:      "Soft Shapes & Glows",
				TextStyle: theme.TextStyleDisplay,
			}),
			gui.Row(gui.ContainerCfg{
				Spacing:    gui.Some[float32](40),
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					// Soft Green Glow / Orb
					gui.Column(gui.ContainerCfg{
						Width:  150,
						Height: 150,
						Radius: gui.Some[float32](75),
						Color:  gui.RGBA(0, 255, 0, 150),
						// Glow via Shadow, not BlurRadius: blur would
						// soften the label too, a shadow only glows
						// outside the box.
						Shadow: &gui.BoxShadow{
							BlurRadius: 20,
							Color:      gui.RGBA(0, 255, 0, 150),
						},
						HAlign:  gui.HAlignCenter,
						VAlign:  gui.VAlignMiddle,
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Soft Orb"})},
					}),
					// Soft Rounded Rect
					gui.Column(gui.ContainerCfg{
						Width:  150,
						Height: 150,
						Radius: gui.Some[float32](20),
						Color:  gui.RGBA(255, 100, 100, 200),
						Shadow: &gui.BoxShadow{
							BlurRadius: 10,
							Color:      gui.RGBA(255, 100, 100, 200),
						},
						HAlign:  gui.HAlignCenter,
						VAlign:  gui.VAlignMiddle,
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Soft Rect"})},
					}),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Spacing:    gui.Some[float32](40),
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					// Large blur
					gui.Column(gui.ContainerCfg{
						Width:  200,
						Height: 100,
						Radius: gui.Some[float32](10),
						Color:  gui.Blue,
						Shadow: &gui.BoxShadow{
							BlurRadius: 50,
							Color:      gui.Blue,
						},
						HAlign:  gui.HAlignCenter,
						VAlign:  gui.VAlignMiddle,
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Heavy Glow"})},
					}),
				},
			}),
		},
	})
}
