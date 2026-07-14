// Vibrancy demonstrates a translucent, blurred native window backdrop on
// macOS via w.SetWindowVibrancy. The window BgColor is translucent (alpha <
// 255) so the NSVisualEffectView behind the content shows through. On other
// platforms SetWindowVibrancy is a no-op and the window renders normally.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	Material gui.VibrancyMaterial
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

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

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Vibrant window (macOS)",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Button(gui.ButtonCfg{
				Focusable: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Cycle material"}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					app := gui.State[App](w)
					// Cycle through the materials, wrapping back to Sidebar.
					app.Material++
					if app.Material > gui.VibrancyUnderWindow {
						app.Material = gui.VibrancySidebar
					}
					w.SetWindowVibrancy(app.Material)
					e.IsHandled = true
				},
			}),
		},
	})
}
