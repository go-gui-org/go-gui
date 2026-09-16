// This example demonstrates custom slider looks built by hand from the public
// API (advanced: build-time interaction state, mouse lock drag).
//
// It ports go-shirei's custom-sliders demo. Three looks:
//
//   - Apple. A gray capsule, a white fill and a round white knob.
//   - Material. A thick track filled up to the value and a thin blade.
//   - Windows XP. A thin sunken trough and a green-capped knob.
//
// go-shirei splits a slider into ProcessSlider, which handles the input and
// returns the handle position, and paint code, which the app writes. go-gui
// has no ProcessSlider, so sliderShell below does its job. The parts of a look
// are floats placed at offsets computed from the value; the width is fixed, so
// the offsets are known while the view is built and no AmendLayout is needed.
//
// What a behavior helper would remove (input for the design issue):
//
//   - valueAt and handleX: the pointer-to-value and value-to-offset math.
//   - The drag: OnMouseDown sets the value, saves the track's window X and
//     starts a MouseLock whose MouseMove converts window coordinates back.
//     OnMouseDown, not OnClick: gui.Debug expects an Interactive root with
//     OnClick to be a button, activated by Space and Enter.
//   - The keys: arrows, Home and End, clamped, in OnKeyDown.
//   - An ID on every float part. A float is hit-tested in its own layer, so
//     an ID-less part hides hover and press from the slider (#661).
//
// Not possible with the public API, so left out:
//
//   - The mouse wheel. ContainerCfg has no OnMouseScroll; OnScroll is for
//     scroll containers only.
//   - The value for a screen reader. The slider role is set, but the value,
//     minimum and maximum live in an unexported field of ContainerCfg.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeLight)

	w := gui.NewWindow(gui.WindowCfg{
		State:  newApp(),
		Title:  "Custom Sliders",
		Width:  720,
		Height: 700,
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

// track is the fixed geometry of one slider. inset is half the handle width:
// the handle's center runs from inset to width-inset, so the handle never
// leaves the track.
type track struct {
	id    string
	name  string
	width float32
	inset float32
}

// Slider sizes. Each look is fixed width, like go-shirei's ProcessSlider.
const (
	appleW, appleH       = 280, 24
	materialW, materialH = 320, 32
	materialBladeW       = 4
	xpW, xpH             = 260, 24
	xpHandleW            = 12
)

// tracks lists every custom slider. The id is its ID leaf; the name is the key
// of its value and its screen reader label.
var tracks = []track{
	{id: "display", name: "Display", width: appleW, inset: appleH / 2},
	{id: "sound", name: "Sound", width: appleW, inset: appleH / 2},
	{id: "call", name: "Call volume", width: materialW, inset: materialBladeW / 2},
	{id: "media", name: "Media volume", width: materialW, inset: materialBladeW / 2},
	{id: "xp", name: "XP", width: xpW, inset: xpHandleW / 2},
}

// keyStep is how far one arrow key moves a value.
const keyStep = 0.05

// App holds the value of each slider, 0 to 1.
type App struct {
	value map[string]float32

	// Handlers are built once per slider, so a frame does not allocate
	// new closures.
	press map[string]func(gui.EventCtx)
	keys  map[string]func(gui.EventCtx)
}

func newApp() *App {
	app := &App{
		value: map[string]float32{
			"Default":      0.45,
			"Display":      0.72,
			"Sound":        0.35,
			"Call volume":  0.65,
			"Media volume": 0.28,
			"XP":           0.4,
		},
		press: map[string]func(gui.EventCtx){},
		keys:  map[string]func(gui.EventCtx){},
	}
	for _, t := range tracks {
		app.press[t.name] = app.pressHandler(t)
		app.keys[t.name] = app.keyHandler(t.name)
	}
	return app
}

// valueAt converts a pointer X, relative to the track's left edge, to a value.
func valueAt(t track, x float32) float32 {
	return clamp01((x - t.inset) / (t.width - 2*t.inset))
}

// handleX is the left edge of a handle 2*inset wide at value v, relative to
// the track's left edge. It is go-shirei's SliderState.HandleX.
func handleX(t track, v float32) float32 {
	return v * (t.width - 2*t.inset)
}

// pressHandler sets the value under the pointer and starts a drag.
func (app *App) pressHandler(t track) func(gui.EventCtx) {
	return func(ctx gui.EventCtx) {
		// OnMouseDown coordinates are relative to the shape, but a mouse lock
		// reports window coordinates. Save the track's window X for the
		// lock to subtract.
		left := ctx.Layout.Shape.X
		app.value[t.name] = valueAt(t, ctx.Event.MouseX)
		ctx.Window.MouseLock(gui.MouseLockCfg{
			MouseMove: func(c gui.EventCtx) {
				app.value[t.name] = valueAt(t, c.Event.MouseX-left)
			},
			MouseUp: func(c gui.EventCtx) {
				c.Window.MouseUnlock()
			},
		})
		ctx.Consume()
	}
}

// keyHandler moves the value by keyStep with the arrow keys, and to an end
// with Home and End. Up and Right increase the value.
func (app *App) keyHandler(name string) func(gui.EventCtx) {
	return func(ctx gui.EventCtx) {
		if ctx.Event.Modifiers != gui.ModNone {
			return
		}
		v := app.value[name]
		switch ctx.Event.KeyCode {
		case gui.KeyLeft, gui.KeyDown:
			v -= keyStep
		case gui.KeyRight, gui.KeyUp:
			v += keyStep
		case gui.KeyHome:
			v = 0
		case gui.KeyEnd:
			v = 1
		default:
			return
		}
		app.value[name] = clamp01(v)
		ctx.Consume()
	}
}

func clamp01(v float32) float32 {
	return min(max(v, 0), 1)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
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
				ID:    "default",
				Value: app.value["Default"],
				Min:   0,
				Max:   1,
				Step:  keyStep,
				Width: 240,
				OnChange: func(v float32, ctx gui.EventCtx) {
					app.value["Default"] = v
					ctx.Consume()
				},
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
			gui.Text(gui.TextCfg{Text: fmt.Sprintf("%.2f", app.value[name]), TextStyle: style}),
		},
	})
}

// sliderShell is the part every custom slider shares: a fixed size, focus,
// press-to-drag, the keys and the slider role. The look goes in cfg.Content.
func sliderShell(app *App, cfg gui.ContainerCfg, t track, height float32) gui.ContainerCfg {
	cfg.Width = t.width
	cfg.Height = height
	cfg.Sizing = gui.FixedFixed
	cfg.Padding = gui.PaddingNone
	cfg.SizeBorder = gui.NoBorder
	cfg.Focusable = true
	cfg.OnMouseDown = app.press[t.name]
	cfg.OnKeyDown = app.keys[t.name]
	cfg.A11YRole = gui.AccessRoleSlider
	cfg.A11YCfg = gui.A11YCfg{A11YLabel: t.name}
	return cfg
}

// part is one floating piece of a look, placed relative to the slider's
// top-left corner. The ID keeps hover and press on the slider while the
// pointer is over the part (#661).
func part(id string, x, y, w, h, radius float32, c gui.Color, content ...gui.View) gui.View {
	return gui.Column(gui.ContainerCfg{
		ID:           id,
		Float:        true,
		FloatOffsetX: x,
		FloatOffsetY: y,
		Width:        w,
		Height:       h,
		Sizing:       gui.FixedFixed,
		Radius:       gui.SomeF(radius),
		Color:        c,
		SizeBorder:   gui.NoBorder,
		Padding:      gui.PaddingNone,
		// No theme gap between the in-flow pieces of a part.
		Spacing: gui.SomeF(0),
		Content: content,
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

// appleSlider is a gray capsule as tall as its knob. A white fill runs to the
// knob's right edge, so the knob hides the fill's end.
func appleSlider(app *App, t track, icon string) gui.View {
	id := t.id
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		const knob = appleH
		v := app.value[t.name]
		hx := handleX(t, v)
		knobColor := white
		if s.Pressed {
			knobColor = applePress
		}
		shadow := knobShadow
		if s.Focused {
			shadow = focusRing
		}
		iconStyle := gui.CurrentTheme().Icon4
		iconStyle.Color = appleIcon
		iconStyle.Size = 14

		cfg := sliderShell(app, gui.ContainerCfg{
			ID:     id,
			Color:  appleTrack,
			Radius: gui.SomeF(appleH / 2),
		}, t, appleH)
		cfg.Content = []gui.View{
			part("fill", 0, 0, hx+knob, appleH, appleH/2, white),
			gui.Circle(gui.ContainerCfg{
				ID:           "knob",
				Float:        true,
				FloatOffsetX: hx,
				Width:        knob,
				Height:       knob,
				Sizing:       gui.FixedFixed,
				Color:        knobColor,
				Shadow:       shadow,
				// A hairline keeps the white knob apart from the white fill
				// where a renderer draws no shadow.
				ColorBorder: appleKnobEdge,
				SizeBorder:  gui.SomeF(1),
				Padding:     gui.PaddingNone,
			}),
			// The icon floats over the left end, on top of the fill.
			part("icon", 0, 0, appleH, appleH, 0, gui.ColorTransparent,
				gui.Row(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					Padding:    gui.PaddingNone,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content:    []gui.View{gui.Text(gui.TextCfg{Text: icon, TextStyle: iconStyle})},
				})),
		}
		return gui.Row(cfg)
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
// the blade. go-shirei rounds only the outer corners of each track half. A
// go-gui radius applies to all four corners, so a square patch covers the
// fill's rounded end at the blade.
func materialSlider(app *App, t track) gui.View {
	id := t.id
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		const trackH, bladeH = 16, 28
		const capR = trackH / 2
		trackY := float32(materialH-trackH) / 2
		fillW := handleX(t, app.value[t.name]) + t.inset

		blade, bladeW := materialBlade, float32(materialBladeW)
		if s.Pressed || s.Focused {
			// A wider blade shows focus and the drag, as Material does.
			bladeW = 6
		}
		if s.Hovered && !s.Pressed {
			blade = lighten(materialBlade, 0.15)
		}

		cfg := sliderShell(app, gui.ContainerCfg{ID: id}, t, materialH)
		cfg.Content = []gui.View{
			part("empty", 0, trackY, t.width, trackH, capR, materialEmpty),
			part("fill", 0, trackY, max(fillW, trackH), trackH, capR, materialFill),
			part("seam", max(fillW-capR, 0), trackY, min(fillW, capR), trackH, 0, materialFill),
			part("blade", fillW-bladeW/2, (materialH-bladeH)/2, bladeW, bladeH, 2, blade),
		}
		return gui.Row(cfg)
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
// handle turn orange under the pointer, as Luna controls do.
func xpSlider(app *App, t track) gui.View {
	id := t.id
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		const troughH, handleH, capH = 6, xpH, 3
		hx := handleX(t, app.value[t.name])
		caps := xpGreen
		if s.Hovered || s.Pressed || s.Focused {
			caps = xpHot
		}

		cfg := sliderShell(app, gui.ContainerCfg{ID: id}, t, xpH)
		cfg.Content = []gui.View{
			// Dark top edge, light floor: the trough reads as sunken.
			part("trough", 0, (xpH-troughH)/2, t.width, troughH, troughH/2, xpTrough,
				strip(2, xpTroughDark),
				gui.Rectangle(gui.RectangleCfg{Sizing: gui.FillFill, Color: xpTrough}),
				strip(2, white),
			),
			part("handle", hx, 0, xpHandleW, handleH, 3, xpOutline,
				gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					Padding:    gui.PadAll(1),
					SizeBorder: gui.NoBorder,
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
				})),
		}
		return gui.Row(cfg)
	})
}

// lighten moves each RGB channel toward white by f.
func lighten(c gui.Color, f float32) gui.Color {
	up := func(v uint8) uint8 { return v + uint8(float32(255-v)*f) }
	return gui.RGBA(up(c.R), up(c.G), up(c.B), c.A)
}
