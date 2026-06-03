// The blur demo shows how blur radius can create glows and
// soft-edged shapes.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

func main() {
	gui.SetTheme(gui.ThemeDarkNoPadding)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Gaussian Blur / Glow Demo",
		Width:  800,
		Height: 800,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(_ *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FitFit,
		Spacing: gui.Some[float32](60),
		Padding: gui.SomeP(40, 40, 40, 40),
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			// Each row highlights a different blur and radius combination.
			gui.Text(gui.TextCfg{
				Text: "Soft Shapes & Glows",
				TextStyle: gui.TextStyle{
					Size:  30,
					Color: gui.White,
				},
			}),
			gui.Row(gui.ContainerCfg{
				Spacing: gui.Some[float32](40),
				Content: []gui.View{
					// Soft Green Glow / Orb
					gui.Column(gui.ContainerCfg{
						Width:      150,
						Height:     150,
						Radius:     gui.Some[float32](75),
						Color:      gui.RGBA(0, 255, 0, 150),
						BlurRadius: 20,
						HAlign:     gui.HAlignCenter,
						VAlign:     gui.VAlignMiddle,
						Content:    []gui.View{gui.Text(gui.TextCfg{Text: "Soft Orb"})},
					}),
					// Soft Rounded Rect
					gui.Column(gui.ContainerCfg{
						Width:      150,
						Height:     150,
						Radius:     gui.Some[float32](20),
						Color:      gui.RGBA(255, 100, 100, 200),
						BlurRadius: 10,
						HAlign:     gui.HAlignCenter,
						VAlign:     gui.VAlignMiddle,
						Content:    []gui.View{gui.Text(gui.TextCfg{Text: "Soft Rect"})},
					}),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Spacing: gui.Some[float32](40),
				Content: []gui.View{
					// Large blur
					gui.Column(gui.ContainerCfg{
						Width:      200,
						Height:     100,
						Radius:     gui.Some[float32](10),
						Color:      gui.Blue,
						BlurRadius: 50,
						HAlign:     gui.HAlignCenter,
						VAlign:     gui.VAlignMiddle,
						Content:    []gui.View{gui.Text(gui.TextCfg{Text: "Heavy Glow"})},
					}),
				},
			}),
		},
	})
}
