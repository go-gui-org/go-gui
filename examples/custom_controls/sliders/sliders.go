// Package sliders demonstrates custom slider looks drawn with SliderCfg.Look
// (advanced: build-time interaction state, parts placed after layout).
//
// It ports go-shirei's custom-sliders demo. Three looks:
//
//   - Apple. A gray capsule, a white fill and a round white knob.
//   - Material. A thick track filled up to the value and a thin blade.
//   - Windows XP. A thin sunken trough and a green-capped knob.
//
// go-shirei splits a slider into ProcessSlider, which handles the input and
// returns the handle position, and paint code, which the app writes. In go-gui
// the input is the stock gui.Slider, and the paint code is its Look. Look gets
// the hover, press and focus state and the value as a fraction, and returns
// three views: Track, Fill and Handle. The slider places Fill and Handle on
// the track after layout, so drag, keys, the wheel and the screen reader value
// all come from gui.Slider.
package sliders

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
)

// track names one custom slider. The id is its ID leaf; the name is the key
// of its value and its screen reader label.
type track struct {
	id   string
	name string
}

// Slider sizes. go-shirei's sliders are fixed width, so these are too.
const (
	appleW, appleH       = 280, 24
	materialW, materialH = 320, 32
	materialBladeW       = 4
	xpW, xpH             = 260, 24
	xpHandleW            = 12
)

var tracks = []track{
	{id: "display", name: "Display"},
	{id: "sound", name: "Sound"},
	{id: "call", name: "Call volume"},
	{id: "media", name: "Media volume"},
	{id: "xp", name: "XP"},
}

// Every slider runs from 0 to 100 and steps by 5. The wheel moves a
// gui.Slider by one unit per line, so a range of 0 to 1 would jump to an end
// on the first turn.
const (
	valueMax = 100
	keyStep  = 5
)

// App holds the value of each slider, 0 to 100.
type App struct {
	value map[string]float32

	// Change handlers are built once per slider, so a frame does not
	// allocate new closures.
	change map[string]func(float32, gui.EventCtx)
}

// New returns the page state with its starting values.
func New() *App {
	app := &App{
		value: map[string]float32{
			"Default":      45,
			"Display":      72,
			"Sound":        35,
			"Call volume":  65,
			"Media volume": 28,
			"XP":           40,
		},
		change: map[string]func(float32, gui.EventCtx){},
	}
	for name := range app.value {
		app.change[name] = func(v float32, ctx gui.EventCtx) {
			app.value[name] = v
			ctx.Consume()
		}
	}
	return app
}

// slider is the part every slider on the page shares. The look sets the size
// and the parts.
func slider(app *App, t track, width, height float32, look func(gui.SliderLookState) gui.SliderParts) gui.View {
	return gui.Slider(gui.SliderCfg{
		ID:       t.id,
		A11YCfg:  gui.A11YCfg{A11YLabel: t.name},
		Value:    app.value[t.name],
		Min:      0,
		Max:      valueMax,
		Step:     keyStep,
		Width:    width,
		Height:   height,
		Sizing:   gui.FixedFixed,
		OnChange: app.change[t.name],
		Look:     look,
	})
}

// View builds the page. The caller owns app, so the page can sit in a
// window whose state is a different type.
func View(w *gui.Window, app *App) gui.View {
	title := gui.CurrentTheme().TextStyleDef
	title.Size = 18

	return gui.Column(gui.ContainerCfg{
		ID:         "page",
		Scrollable: true,
		Sizing:     gui.FillFill,
		Color:      pageBG,
		Padding:    gui.PadAll(28),
		Spacing:    gui.SomeF(20),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Custom sliders from the public API", TextStyle: title}),

			section("Default", "gui.Slider, for comparison"),
			valueRow(app, "Default", gui.Slider(gui.SliderCfg{
				ID:       "default",
				Value:    app.value["Default"],
				Min:      0,
				Max:      valueMax,
				Step:     keyStep,
				Width:    240,
				OnChange: app.change["Default"],
			})),

			section("Apple-style", "Filled capsule and round knob"),
			appleCard(app),

			section("Material", "Thick track, thin blade, fill up to the value"),
			materialCard(app),

			section("Windows XP", "Thin trough and chunky handle"),
			valueRow(app, "XP", xpSlider(app, tracks[4])),
		},
	})
}

func section(title, sub string) gui.View {
	return gui.Column(gui.ContainerCfg{
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(4),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: title, TextStyle: gui.CurrentTheme().B3}),
			gui.Text(gui.TextCfg{Text: sub, TextStyle: gui.CurrentTheme().TextStyleSecondary}),
		},
	})
}

// valueRow puts a value label to the right of a slider.
func valueRow(app *App, name string, slider gui.View) gui.View {
	return valueRowStyled(app, name, slider, gui.CurrentTheme().TextStyleSecondary)
}

// valueRowStyled is valueRow with the label style given, for a dark card.
func valueRowStyled(app *App, name string, slider gui.View, style gui.TextStyle) gui.View {
	return gui.Row(gui.ContainerCfg{
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(12),
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			slider,
			gui.Text(gui.TextCfg{Text: fmt.Sprintf("%.0f", app.value[name]), TextStyle: style}),
		},
	})
}

// strip is an in-flow bar of a fixed height and the full width.
func strip(h float32, c gui.Color) gui.View {
	return gui.Rectangle(gui.RectangleCfg{Height: h, Sizing: gui.FillFixed, Color: c})
}

var (
	pageBG = gui.Hex(0xf2f3f7)
	white  = gui.Hex(0xffffff)

	// Shared shadows. A BoxShadow is a pointer, so sharing costs nothing
	// per frame.
	knobShadow = &gui.BoxShadow{Color: gui.RGBA(0, 0, 0, 110), OffsetY: 1, BlurRadius: 5}
	focusRing  = &gui.BoxShadow{Color: gui.RGBA(10, 132, 255, 160), Spread: 3}
)

// --- Apple -----------------------------------------------------------------

var (
	appleCardBG   = gui.Hex(0x2e2e2e)
	appleTrack    = gui.Hex(0x666666)
	appleIcon     = gui.Hex(0x333333)
	appleText     = gui.Hex(0xebebeb)
	applePress    = gui.Hex(0xe6e6e6)
	appleKnobEdge = gui.Hex(0xcfcfcf)
)

func appleCard(app *App) gui.View {
	label := gui.CurrentTheme().B5
	label.Color = appleText
	row := func(name, icon string, t track) []gui.View {
		return []gui.View{
			gui.Text(gui.TextCfg{Text: name, TextStyle: label}),
			valueRowStyled(app, name, appleSlider(app, t, icon), label),
		}
	}
	return gui.Column(gui.ContainerCfg{
		ID:         "apple",
		Color:      appleCardBG,
		Radius:     gui.SomeF(12),
		Padding:    gui.PadAll(14),
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(10),
		Content: append(row("Display", gui.IconSunnyO, tracks[0]),
			row("Sound", gui.IconSpeaker, tracks[1])...),
	})
}

// appleSlider is a gray capsule as tall as its knob. The white fill ends at
// the knob's center, under the knob, and carries the icon at its left end.
func appleSlider(app *App, t track, icon string) gui.View {
	iconStyle := gui.CurrentTheme().Icon4
	iconStyle.Color = appleIcon
	iconStyle.Size = 14
	return slider(app, t, appleW, appleH, func(s gui.SliderLookState) gui.SliderParts {
		knobColor := white
		if s.Pressed {
			knobColor = applePress
		}
		shadow := knobShadow
		if s.Focused {
			shadow = focusRing
		}
		return gui.SliderParts{
			Track: bar(gui.FillFixed, 0, appleH, appleH/2, appleTrack),
			// The fill's length is set after layout, and its children keep
			// the place they were laid out in, so the icon stays at the left.
			Fill: gui.Row(gui.ContainerCfg{
				ID:         "fill",
				Height:     appleH,
				Sizing:     gui.FixedFixed,
				Radius:     gui.SomeF(appleH / 2),
				Color:      white,
				SizeBorder: gui.NoBorder,
				Padding:    gui.PaddingNone,
				Content: []gui.View{gui.Row(gui.ContainerCfg{
					Width:      appleH,
					Height:     appleH,
					Sizing:     gui.FixedFixed,
					Padding:    gui.PaddingNone,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content:    []gui.View{gui.Text(gui.TextCfg{Text: icon, TextStyle: iconStyle})},
				})},
			}),
			Handle: gui.Circle(gui.ContainerCfg{
				ID:     "knob",
				Width:  appleH,
				Height: appleH,
				Sizing: gui.FixedFixed,
				Color:  knobColor,
				Shadow: shadow,
				// A hairline keeps the white knob apart from the white fill
				// where a renderer draws no shadow.
				ColorBorder: appleKnobEdge,
				SizeBorder:  gui.SomeF(1),
				Padding:     gui.PaddingNone,
			}),
		}
	})
}

// --- Material --------------------------------------------------------------

var (
	materialCardBG = gui.Hex(0xf5f0fa)
	materialEmpty  = gui.Hex(0xe3d4f0)
	materialFill   = gui.Hex(0x6a3a99)
	materialBlade  = gui.Hex(0x4f2a78)
	materialText   = gui.Hex(0x333333)
)

func materialCard(app *App) gui.View {
	label := gui.CurrentTheme().B5
	label.Color = materialText
	iconStyle := gui.CurrentTheme().Icon4
	iconStyle.Color = materialText
	iconStyle.Size = 16
	row := func(name, icon string, t track) gui.View {
		return gui.Column(gui.ContainerCfg{
			Padding:    gui.PaddingNone,
			SizeBorder: gui.NoBorder,
			Spacing:    gui.SomeF(8),
			Content: []gui.View{
				gui.Row(gui.ContainerCfg{
					Padding:    gui.PaddingNone,
					SizeBorder: gui.NoBorder,
					Spacing:    gui.SomeF(10),
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: icon, TextStyle: iconStyle}),
						gui.Text(gui.TextCfg{Text: name, TextStyle: label}),
					},
				}),
				valueRow(app, name, materialSlider(app, t)),
			},
		})
	}
	return gui.Column(gui.ContainerCfg{
		ID:         "material",
		Color:      materialCardBG,
		Radius:     gui.SomeF(8),
		Padding:    gui.PadAll(16),
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(16),
		Content: []gui.View{
			row("Call volume", gui.IconPhone, tracks[2]),
			row("Media volume", gui.IconSpeaker, tracks[3]),
		},
	})
}

// materialSlider draws the empty track, the fill up to the blade's center and
// the blade. go-shirei rounds only the outer end of the fill. The fill here is
// laid out as wide as the track, holding a full-width capsule; the slider then
// shortens it to the value, and Clip cuts the capsule square at the blade.
func materialSlider(app *App, t track) gui.View {
	return slider(app, t, materialW, materialH, func(s gui.SliderLookState) gui.SliderParts {
		const trackH, bladeH = 16, 28
		blade, bladeW := materialBlade, float32(materialBladeW)
		if s.Pressed || s.Focused {
			// A wider blade shows focus and the drag, as Material does.
			bladeW = 6
		}
		if s.Hovered && !s.Pressed {
			blade = lighten(materialBlade, 0.15)
		}
		return gui.SliderParts{
			Track: bar(gui.FillFixed, 0, trackH, trackH/2, materialEmpty),
			Fill: gui.Row(gui.ContainerCfg{
				Width:      materialW,
				Height:     trackH,
				Sizing:     gui.FixedFixed,
				Clip:       true,
				SizeBorder: gui.NoBorder,
				Padding:    gui.PaddingNone,
				Content:    []gui.View{bar(gui.FillFixed, 0, trackH, trackH/2, materialFill)},
			}),
			Handle: bar(gui.FixedFixed, bladeW, bladeH, 2, blade),
		}
	})
}

// bar is a plain colored container.
func bar(sizing gui.Sizing, w, h, radius float32, c gui.Color) gui.View {
	return gui.Row(gui.ContainerCfg{
		Width:      w,
		Height:     h,
		Sizing:     sizing,
		Radius:     gui.SomeF(radius),
		Color:      c,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PaddingNone,
	})
}

// --- Windows XP ------------------------------------------------------------

var (
	xpTrough     = gui.Hex(0xece9d8)
	xpTroughDark = gui.Hex(0x9d9d9d)
	xpOutline    = gui.Hex(0x4f4f4f)
	xpGreen      = gui.Hex(0x31a331)
	xpHot        = gui.Hex(0xf8b636)
	xpFace       = gui.Hex(0xf4f3ee)
	xpRidgeLo    = gui.RGBA(0, 0, 0, 50)
)

// xpSlider is a thin sunken trough under a raised handle. The caps of the
// handle turn orange under the pointer, as Luna controls do. It has no fill.
func xpSlider(app *App, t track) gui.View {
	return slider(app, t, xpW, xpH, func(s gui.SliderLookState) gui.SliderParts {
		const troughH, capH = 6, 3
		caps := xpGreen
		if s.Hovered || s.Pressed || s.Focused {
			caps = xpHot
		}
		return gui.SliderParts{
			// Dark top edge, light floor: the trough reads as sunken.
			Track: gui.Column(gui.ContainerCfg{
				Height:     troughH,
				Sizing:     gui.FillFixed,
				Radius:     gui.SomeF(troughH / 2),
				Color:      xpTrough,
				SizeBorder: gui.NoBorder,
				Padding:    gui.PaddingNone,
				Spacing:    gui.SomeF(0),
				Content: []gui.View{
					strip(2, xpTroughDark),
					gui.Rectangle(gui.RectangleCfg{Sizing: gui.FillFill, Color: xpTrough}),
					strip(2, white),
				},
			}),
			Handle: gui.Column(gui.ContainerCfg{
				ID:         "handle",
				Width:      xpHandleW,
				Height:     xpH,
				Sizing:     gui.FixedFixed,
				Radius:     gui.SomeF(3),
				Color:      xpOutline,
				SizeBorder: gui.NoBorder,
				Padding:    gui.PadAll(1),
				Spacing:    gui.SomeF(0),
				Content: []gui.View{
					strip(capH, caps),
					// Face: light ridge left, dark ridge right.
					gui.Row(gui.ContainerCfg{
						Sizing:     gui.FillFill,
						Padding:    gui.PaddingNone,
						SizeBorder: gui.NoBorder,
						Spacing:    gui.SomeF(0),
						Color:      xpFace,
						Content: []gui.View{
							gui.Rectangle(gui.RectangleCfg{Width: 2, Sizing: gui.FixedFill, Color: white}),
							gui.Rectangle(gui.RectangleCfg{Sizing: gui.FillFill, Color: xpFace}),
							gui.Rectangle(gui.RectangleCfg{Width: 2, Sizing: gui.FixedFill, Color: xpRidgeLo}),
						},
					}),
					strip(capH, caps),
				},
			}),
		}
	})
}

// lighten moves each RGB channel toward white by f.
func lighten(c gui.Color, f float32) gui.Color {
	up := func(v uint8) uint8 { return v + uint8(float32(255-v)*f) }
	return gui.RGBA(up(c.R), up(c.G), up(c.B), c.A)
}
