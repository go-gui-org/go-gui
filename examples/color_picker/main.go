// The color picker example shows the composable color components:
// one HSLA value in app state, edited four ways — channel sliders, a
// saturation × lightness plane, a hue wheel, and text fields. Every
// control reads and writes the same value, so they all stay in sync
// without any of them holding state of its own.
//
// gui.ColorPicker packages the same components into the one
// arrangement most apps want; the last panel shows it for comparison.
package main

import (
	"strconv"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	// Color is the one value every control below edits. It is an
	// HSLA, not a Color, because RGBA cannot carry hue through a gray
	// or through black — drag lightness to the bottom and back and an
	// RGBA-held value would come back red.
	Color gui.HSLA

	ShowPacked bool
	LightTheme bool
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State: &App{
			Color: gui.HSLA{H: 210, S: 0.7, L: 0.55, A: 0.85},
		},
		Title:  "Color Picker",
		Width:  900,
		Height: 620,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	t := gui.CurrentTheme()

	panels := []gui.View{
		card("Channel sliders", "Tracks show what each slider picks.",
			slidersPanel(app)),
		card("Saturation × Lightness", "The S/L plane at the current hue.",
			planePanel(app)),
		card("Hue wheel", "Angle = hue, radius = saturation.",
			wheelPanel(app)),
	}
	if app.ShowPacked {
		panels = append(panels, card("gui.ColorPicker",
			"The same components, pre-arranged.", packedPanel(app)))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: t.PaddingMedium,
		Spacing: gui.Some(t.SpacingMedium),
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				VAlign:  gui.VAlignMiddle,
				Sizing:  gui.FitFit,
				Padding: gui.NoPadding,
				Spacing: gui.Some(t.SpacingMedium),
				Content: []gui.View{
					toggleTheme(app),
					togglePacked(app),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FitFit,
				Padding: gui.NoPadding,
				Spacing: gui.Some(t.SpacingMedium),
				Content: panels,
			}),
			previewBar(app),
		},
	})
}

// setColor is the one writer every control shares. Because the value
// is an HSLA, a control that drives saturation to zero does not lose
// the hue the others are showing.
func setColor(ctx gui.EventCtx, v gui.HSLA) {
	gui.State[App](ctx.Window).Color = v
}

// card wraps a panel in a titled box.
func card(title, sub string, body gui.View) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FitFit,
		Padding: t.PaddingMedium,
		Spacing: gui.Some(t.SpacingMedium),
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Padding: gui.NoPadding,
				Spacing: gui.SomeF(2),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: title, TextStyle: t.N2}),
					gui.Text(gui.TextCfg{Text: sub, TextStyle: t.N4}),
				},
			}),
			body,
		},
	})
}

const sliderWidth = 240

func slidersPanel(app *App) gui.View {
	return gui.Column(gui.ContainerCfg{
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(10),
		Content: []gui.View{
			channelRow(app, "hue", "Hue", gui.ChannelHue),
			channelRow(app, "sat", "Saturation", gui.ChannelSaturation),
			channelRow(app, "light", "Lightness", gui.ChannelLightness),
			channelRow(app, "alpha", "Alpha", gui.ChannelAlpha),
		},
	})
}

// channelRow pairs a slider with its name and current value. The
// component draws only the control — a caller that wants a label
// places it, which is what keeps the same slider usable in a compact
// toolbar and in a labeled panel like this one.
func channelRow(
	app *App, id, label string, ch gui.ColorChannel,
) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(3),
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Width:   sliderWidth,
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: label, TextStyle: t.N3,
					}),
					// Fill pushes the value to the right edge.
					gui.Text(gui.TextCfg{
						Text:      channelText(app.Color, ch),
						TextStyle: valueStyle(t),
						Sizing:    gui.FillFit,
					}),
				},
			}),
			gui.ColorChannelSlider(gui.ColorChannelSliderCfg{
				ID:      gui.ScopeID("channels", id),
				Channel: ch,
				Value:   app.Color,
				Width:   sliderWidth,
				OnChange: func(v gui.HSLA, ctx gui.EventCtx) {
					setColor(ctx, v)
				},
			}),
		},
	})
}

// valueStyle right-aligns the readout so the numbers line up down the
// column as they change width.
func valueStyle(t gui.Theme) gui.TextStyle {
	s := t.N3
	s.Align = gui.TextAlignRight
	return s
}

// channelText formats one channel the way its units read: degrees for
// hue, percent for saturation and lightness, a fraction for alpha.
func channelText(v gui.HSLA, ch gui.ColorChannel) string {
	switch ch {
	case gui.ChannelHue:
		return strconv.Itoa(int(v.H + 0.5))
	case gui.ChannelSaturation:
		return strconv.Itoa(int(v.S*100 + 0.5))
	case gui.ChannelLightness:
		return strconv.Itoa(int(v.L*100 + 0.5))
	default:
		return strconv.FormatFloat(float64(v.A), 'f', 2, 32)
	}
}

func planePanel(app *App) gui.View {
	return gui.ColorPlane(gui.ColorPlaneCfg{
		ID:    "plane",
		Value: app.Color,
		Size:  200,
		OnChange: func(v gui.HSLA, ctx gui.EventCtx) {
			setColor(ctx, v)
		},
	})
}

func wheelPanel(app *App) gui.View {
	return gui.ColorWheel(gui.ColorWheelCfg{
		ID:    "wheel",
		Value: app.Color,
		Size:  200,
		OnChange: func(v gui.HSLA, ctx gui.EventCtx) {
			setColor(ctx, v)
		},
	})
}

// packedPanel shows gui.ColorPicker driven by the same value. Its API
// is RGBA in and RGBA out, so the conversion happens here — and the
// hue it holds internally is what keeps that conversion honest.
func packedPanel(app *App) gui.View {
	return gui.ColorPicker(gui.ColorPickerCfg{
		ID:      "packed",
		Color:   app.Color.Color(),
		ShowHSL: true,
		OnColorChange: func(c gui.Color, ctx gui.EventCtx) {
			setColor(ctx, gui.ColorToHSLA(c))
		},
	})
}

// previewBar shows the value as a swatch and as text, the two ways a
// user checks that the controls agree.
func previewBar(app *App) gui.View {
	t := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FitFit,
		VAlign:  gui.VAlignMiddle,
		Padding: t.PaddingMedium,
		Spacing: gui.Some(t.SpacingMedium),
		Content: []gui.View{
			gui.ColorSwatch(gui.ColorSwatchCfg{
				ID:     "preview",
				Color:  app.Color.Color(),
				Width:  96,
				Height: 56,
			}),
			gui.Column(gui.ContainerCfg{
				Padding: gui.NoPadding,
				Spacing: gui.SomeF(4),
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      app.Color.String(),
						TextStyle: t.N2,
					}),
					gui.Text(gui.TextCfg{
						Text:      app.Color.Color().Hex(),
						TextStyle: t.N4,
					}),
				},
			}),
			gui.ColorFields(gui.ColorFieldsCfg{
				ID:      "fields",
				Value:   app.Color,
				ShowHSL: true,
				OnChange: func(v gui.HSLA, ctx gui.EventCtx) {
					setColor(ctx, v)
				},
			}),
		},
	})
}

func togglePacked(app *App) gui.View {
	return gui.Switch(gui.SwitchCfg{
		ID:       "color_picker_toggle_packed",
		Label:    "gui.ColorPicker",
		Selected: app.ShowPacked,
		OnClick: func(ctx gui.EventCtx) {
			a := gui.State[App](ctx.Window)
			a.ShowPacked = !a.ShowPacked
		},
	})
}

func toggleTheme(app *App) gui.View {
	t := gui.CurrentTheme()
	return gui.Toggle(gui.ToggleCfg{
		ID:           "color_picker_toggle_theme",
		TextSelect:   gui.IconMoon,
		TextUnselect: gui.IconSunnyO,
		TextStyle:    t.Icon3,
		Padding:      t.PaddingSmall,

		Selected: app.LightTheme,
		OnClick: func(ctx gui.EventCtx) {
			a := gui.State[App](ctx.Window)
			a.LightTheme = !a.LightTheme
			// Switching the theme is enough; the next frame re-renders.
			if a.LightTheme {
				ctx.Window.SetTheme(gui.ThemeLight.WithBorders(true))
			} else {
				ctx.Window.SetTheme(gui.ThemeDark.WithBorders(true))
			}
		},
	})
}
