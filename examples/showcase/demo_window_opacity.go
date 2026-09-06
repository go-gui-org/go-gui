package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
)

// demoWindowOpacity drives Window.SetWindowOpacity, the whole-window
// fade added in #516. It is deliberately not a widget demo: the control
// under test is the window this showcase is running in, so the effect
// is visible on the frame, the title bar and every panel at once —
// which is exactly what tells it apart from per-pixel transparency.
func demoWindowOpacity(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: "Drag the slider to fade this entire window, " +
					"content included. The fade is applied by the " +
					"compositor above the GL or Metal surface, so the " +
					"title bar goes with it and no widget has to know.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NoPadding,
				Spacing: gui.SomeF(12),
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Slider(gui.SliderCfg{
						ID:    "showcase-window-opacity",
						Label: "Window opacity",
						Value: app.WindowOpacity,
						// Not 0: a fully invisible showcase is one the
						// user cannot find again to drag back.
						Min:   0.3,
						Max:   1,
						Step:  0.05,
						Width: 220,
						OnChange: func(v float32, ctx gui.EventCtx) {
							appState(ctx.Window).WindowOpacity = v
							ctx.Window.SetWindowOpacity(v)
							ctx.Consume()
						},
					}),
					gui.Text(gui.TextCfg{
						Text:      fmt.Sprintf("%.0f%%", app.WindowOpacity*100),
						TextStyle: t.N3,
					}),
					gui.Button(gui.ButtonCfg{
						ID:       "showcase-window-opacity-reset",
						Disabled: app.WindowOpacity >= 1,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Restore",
								TextStyle: t.N3,
							}),
						},
						OnClick: func(ctx gui.EventCtx) {
							appState(ctx.Window).WindowOpacity = 1
							ctx.Window.SetWindowOpacity(1)
							ctx.Consume()
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text: "This composes with, and is separate from, " +
					"WindowCfg.Transparent: that one is per-pixel alpha " +
					"fixed when the window is made, this one is a " +
					"runtime setter. Window.WindowOpacity reads the " +
					"value back, which is what a fade animation steps " +
					"from.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			gui.Text(gui.TextCfg{
				Text: "Honored on macOS and X11, and on Windows for a " +
					"window not created Transparent. Ignored on web, " +
					"iOS and Android, where the slider moves and " +
					"nothing fades; gui.Debug reports why.",
				TextStyle: t.TextStyleSecondary,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}
