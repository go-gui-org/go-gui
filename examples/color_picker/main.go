// The color picker example combines color editing with runtime
// theme switching.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	Color      gui.Color
	ShowHSV    bool
	LightTheme bool
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Color: gui.RGBA(0x3D, 0x81, 0x7C, 255), ShowHSV: true},
		Title:  "Color Picker",
		Width:  300,
		Height: 490,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	t := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Padding: gui.Some(t.PaddingMedium),
		Spacing: gui.Some(t.SpacingMedium),
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				VAlign:  gui.VAlignMiddle,
				Sizing:  gui.FitFit,
				Padding: gui.NoPadding,
				Spacing: gui.Some(t.SpacingMedium),
				Content: []gui.View{
					toggleTheme(app),
					toggleHSV(app),
				},
			}),
			gui.ColorPicker(gui.ColorPickerCfg{
				ID:      "picker",
				Color:   app.Color,
				ShowHSV: app.ShowHSV,
				OnColorChange: func(c gui.Color, _ *gui.Event, w *gui.Window) {
					// Keep the picker controlled by the window state.
					gui.State[App](w).Color = c
				},
			}),
		},
	})
}

func toggleHSV(app *App) gui.View {
	return gui.Switch(gui.SwitchCfg{
		Label:    "HSV",
		Selected: app.ShowHSV,
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			a := gui.State[App](w)
			a.ShowHSV = !a.ShowHSV
		},
	})
}

func toggleTheme(app *App) gui.View {
	t := gui.CurrentTheme()
	return gui.Toggle(gui.ToggleCfg{
		TextSelect:   gui.IconMoon,
		TextUnselect: gui.IconSunnyO,
		TextStyle:    t.Icon3,
		Padding:      gui.Some(t.PaddingSmall),
		Selected:     app.LightTheme,
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			a := gui.State[App](w)
			a.LightTheme = !a.LightTheme
			// Switching the theme is enough; the next frame re-renders the picker.
			if a.LightTheme {
				w.SetTheme(gui.ThemeLight.WithBorders(true))
			} else {
				w.SetTheme(gui.ThemeDark.WithBorders(true))
			}
		},
	})
}
