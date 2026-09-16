// This example demonstrates custom switch looks built from hover and press
// state read while the view is built (advanced: build-time interaction state).
//
// It ports go-shirei's custom-toggles demo. Four looks:
//
//   - Material. A tinted track and a solid accent knob when on.
//   - Green. An iOS-like green track and a white knob.
//   - Checkmark. A white knob with a check mark when on, a dark knob when off.
//   - Labelled. A knob taller than its track, with ON or OFF on the free side.
//
// Each switch is a gui.Interactive. It gives the look function the hover and
// press state; the app gives it the on/off value. Behavior is the same as a
// button: a click, Space or Enter flips the value. The switch's outer size
// never changes, only what is inside it moves.
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

	gui.SetTheme(gui.ThemeLight)

	w := gui.NewWindow(gui.WindowCfg{
		State:  newApp(),
		Title:  "Custom Toggles",
		Width:  640,
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

// switchNames lists every switch in the example. The name is the key of its
// value in App.on and the text written to the log.
var switchNames = []string{
	"Default A", "Default B",
	"Wi-Fi", "Bluetooth", "Airplane",
	"Green A", "Green B",
	"Check A", "Check B",
	"Label A", "Label B",
}

// App holds the value of each switch and the last action.
type App struct {
	on  map[string]bool
	log string

	// Flip handlers are built once, so a frame does not allocate a new
	// closure per switch.
	flip map[string]func(gui.EventCtx)
}

func newApp() *App {
	app := &App{
		log: "Flip a switch.",
		on: map[string]bool{
			"Default A": true,
			"Wi-Fi":     true,
			"Airplane":  true,
			"Green A":   true,
			"Check A":   true,
			"Label A":   true,
		},
		flip: map[string]func(gui.EventCtx){},
	}
	for _, name := range switchNames {
		app.flip[name] = func(ctx gui.EventCtx) {
			app.on[name] = !app.on[name]
			if app.on[name] {
				app.log = name + " → on"
			} else {
				app.log = name + " → off"
			}
			ctx.Consume()
		}
	}
	return app
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
		Spacing:    gui.SomeF(16),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Custom toggles from IsHovered / IsPressed", TextStyle: title}),

			sectionTitle("Default gui.Switch, for comparison"),
			group("default", gui.ContainerCfg{},
				gui.Switch(gui.SwitchCfg{ID: "a", Label: "Default A",
					Selected: app.on["Default A"], OnClick: app.flip["Default A"]}),
				gui.Switch(gui.SwitchCfg{ID: "b", Label: "Default B",
					Selected: app.on["Default B"], OnClick: app.flip["Default B"]}),
			),

			sectionTitle("Material (colored knob)"),
			gui.Column(gui.ContainerCfg{
				ID:         "material",
				Spacing:    gui.SomeF(12),
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					labelRow("Wi-Fi", materialToggle("wifi", "Wi-Fi", app.on["Wi-Fi"],
						materialPurple, materialPurpleTrack, app.flip["Wi-Fi"])),
					labelRow("Bluetooth", materialToggle("bluetooth", "Bluetooth",
						app.on["Bluetooth"], materialPurple, materialPurpleTrack,
						app.flip["Bluetooth"])),
					labelRow("Airplane", materialToggle("airplane", "Airplane",
						app.on["Airplane"], materialTeal, materialTealTrack,
						app.flip["Airplane"])),
				},
			}),

			sectionTitle("Green track (close to iOS)"),
			group("green", gui.ContainerCfg{},
				greenToggle("a", "Green A", app.on["Green A"], app.flip["Green A"]),
				greenToggle("b", "Green B", app.on["Green B"], app.flip["Green B"]),
			),

			sectionTitle("Check mark in the knob"),
			group("check", gui.ContainerCfg{},
				checkToggle("a", "Check A", app.on["Check A"], app.flip["Check A"]),
				checkToggle("b", "Check B", app.on["Check B"], app.flip["Check B"]),
			),

			sectionTitle("Knob larger than the track, with ON / OFF"),
			group("label", gui.ContainerCfg{},
				labelToggle("a", "Label A", app.on["Label A"], app.flip["Label A"]),
				labelToggle("b", "Label B", app.on["Label B"], app.flip["Label B"]),
			),

			gui.Text(gui.TextCfg{ID: "log", Text: app.log}),
		},
	})
}

func sectionTitle(s string) gui.View {
	return gui.Text(gui.TextCfg{Text: s, TextStyle: gui.CurrentTheme().TextStyleLabel})
}

// group is one ID-bearing row. The ID scopes the switches inside it, so "a"
// in the green row is "page:green:a" and "a" in the check row is
// "page:check:a".
func group(id string, cfg gui.ContainerCfg, content ...gui.View) gui.View {
	cfg.ID = id
	cfg.Spacing = gui.SomeF(20)
	cfg.Padding = gui.PaddingNone
	cfg.SizeBorder = gui.NoBorder
	cfg.VAlign = gui.VAlignMiddle
	cfg.Content = content
	return gui.Row(cfg)
}

// labelRow puts a label on the left and the switch on the right.
func labelRow(label string, control gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Width:      240,
		Sizing:     gui.FixedFit,
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: textStyle(textDark, 14)}),
			gui.Row(gui.ContainerCfg{Sizing: gui.FillFit, Padding: gui.PaddingNone, SizeBorder: gui.NoBorder}),
			control,
		},
	})
}

// textStyle returns the theme's body style with a color and size.
func textStyle(c gui.Color, size float32) gui.TextStyle {
	ts := gui.CurrentTheme().TextStyleDef
	ts.Color = c
	ts.Size = size
	return ts
}

// switchShell is the part every custom switch shares: the ID that makes it a
// hover and press target, click and keyboard activation, and the switch
// role with its on/off state. The look goes in cfg.
func switchShell(cfg gui.ContainerCfg, label string, on bool, onClick func(gui.EventCtx)) gui.ContainerCfg {
	cfg.OnClick = onClick
	cfg.ClickOnSpace = true
	cfg.ClickOnEnter = true
	cfg.Focusable = true
	cfg.A11YRole = gui.AccessRoleSwitchToggle
	if on {
		cfg.A11YState = gui.AccessStateChecked
	}
	cfg.A11YCfg = gui.A11YCfg{A11YLabel: label}
	cfg.SizeBorder = gui.NoBorder
	cfg.Sizing = gui.FixedFixed
	return cfg
}

// hoverShade makes a track a little lighter when on and a little darker when
// off, as go-shirei does, so hover reads on both.
func hoverShade(track gui.Color, on bool, s gui.InteractionState) gui.Color {
	switch {
	case !s.Hovered && !s.Pressed:
		return track
	case on:
		return lighten(track, 0.12)
	default:
		return darken(track, 0.94)
	}
}

var (
	pageBG   = gui.Hex(0xf4f4f6)
	textDark = gui.Hex(0x404040)
	white    = gui.Hex(0xffffff)

	// knobShadow is shared by every knob. A BoxShadow is a pointer, so
	// sharing it costs nothing per frame.
	knobShadow = &gui.BoxShadow{Color: gui.RGBA(0, 0, 0, 46), OffsetY: 1, BlurRadius: 4}
)

// --- Material --------------------------------------------------------------

var (
	materialPurple      = gui.Hex(0x8541c8)
	materialPurpleTrack = gui.Hex(0xc7b9d5)
	materialTeal        = gui.Hex(0x30a69a)
	materialTealTrack   = gui.Hex(0xb9d5d2)
	materialOffTrack    = gui.Hex(0xc7c7c7)
)

// materialToggle is a Material switch: a tinted track and a solid accent
// knob when on, a gray track and a white knob when off.
func materialToggle(id, label string, on bool, accent, onTrack gui.Color, onClick func(gui.EventCtx)) gui.View {
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		const w, h, knob = 48, 28, 22
		track, knobColor := materialOffTrack, white
		if on {
			track, knobColor = onTrack, accent
		}
		return knobTrack(switchShell(gui.ContainerCfg{
			ID:      id,
			Width:   w,
			Height:  h,
			Radius:  gui.SomeF(h / 2),
			Color:   hoverShade(track, on, s),
			Padding: gui.PadAll((h - knob) / 2),
		}, label, on, onClick), on, knobCircle(knob, knobColor, nil))
	})
}

// knobTrack lays out a track with its knob at the left end when off and the
// right end when on. The knob stays in the normal flow, so the track's
// alignment moves it and no float or AmendLayout is needed.
func knobTrack(cfg gui.ContainerCfg, on bool, knob gui.View) gui.View {
	cfg.VAlign = gui.VAlignMiddle
	cfg.HAlign = gui.HAlignLeft
	if on {
		cfg.HAlign = gui.HAlignRight
	}
	cfg.Content = []gui.View{knob}
	return gui.Row(cfg)
}

// knobCircle is a round knob with the shared shadow. content, when set, is
// centered inside it.
func knobCircle(size float32, c gui.Color, content []gui.View) gui.View {
	return gui.Circle(gui.ContainerCfg{
		Width:      size,
		Height:     size,
		Sizing:     gui.FixedFixed,
		Color:      c,
		Shadow:     knobShadow,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PaddingNone,
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Content:    content,
	})
}

// --- Green -----------------------------------------------------------------

var (
	greenOn  = gui.Hex(0x37be56)
	greenOff = gui.Hex(0xe6e6e6)
)

// greenToggle is an iOS-like switch: a green track when on, a pale gray one
// when off, and a white knob in both.
func greenToggle(id, label string, on bool, onClick func(gui.EventCtx)) gui.View {
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		const w, h, pad = 52, 30, 2
		track := greenOff
		if on {
			track = greenOn
		}
		return knobTrack(switchShell(gui.ContainerCfg{
			ID:      id,
			Width:   w,
			Height:  h,
			Radius:  gui.SomeF(h / 2),
			Color:   hoverShade(track, on, s),
			Padding: gui.PadAll(pad),
		}, label, on, onClick), on, knobCircle(h-pad*2, white, nil))
	})
}

// --- Check mark ------------------------------------------------------------

var (
	checkPurple  = gui.Hex(0x613a88)
	checkOff     = gui.Hex(0xe0e0e0)
	checkKnobOff = gui.Hex(0x737373)
	checkTick    = gui.Hex(0x591b98)
)

// checkToggle shows a check mark in a white knob when on. Off, the knob is
// solid dark gray with no mark.
func checkToggle(id, label string, on bool, onClick func(gui.EventCtx)) gui.View {
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		const w, h, pad = 50, 28, 3
		const knob = h - pad*2
		track, knobColor := checkOff, checkKnobOff
		var mark []gui.View
		if on {
			track, knobColor = checkPurple, white
			ts := gui.CurrentTheme().Icon4
			ts.Color = checkTick
			ts.Size = knob * 0.7
			mark = []gui.View{gui.Text(gui.TextCfg{Text: gui.IconCheck, TextStyle: ts})}
		}
		return knobTrack(switchShell(gui.ContainerCfg{
			ID:      id,
			Width:   w,
			Height:  h,
			Radius:  gui.SomeF(h / 2),
			Color:   hoverShade(track, on, s),
			Padding: gui.PadAll(pad),
		}, label, on, onClick), on, knobCircle(knob, knobColor, mark))
	})
}

// --- Labelled --------------------------------------------------------------

var (
	labelGreen    = gui.Hex(0x31c453)
	labelOffTrack = gui.Hex(0xc6d1dd)
	labelOffText  = gui.Hex(0x7b8c9d)
)

// labelToggle has a knob taller than its track, so the knob cannot sit inside
// the track. The track stays in the normal flow and the knob floats over it:
// a float is drawn after the tree it floats in, so it lands on top. The text
// sits in the half of the track the knob leaves free.
func labelToggle(id, label string, on bool, onClick func(gui.EventCtx)) gui.View {
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		const knob, trackW, trackH = 28, 64, 22
		const outerH = knob + 4
		track, text, word := labelOffTrack, labelOffText, "OFF"
		textAlign, knobX := gui.HAlignRight, float32(1)
		if on {
			track, text, word = labelGreen, white, "ON"
			textAlign, knobX = gui.HAlignLeft, trackW-knob-1
		}
		ts := gui.CurrentTheme().B6
		ts.Color = text
		ts.Size = 11

		knobView := gui.Circle(gui.ContainerCfg{
			// A float is hit-tested in its own layer. Without an ID the
			// pointer over the knob would find no hover target, and the
			// track would lose its hover shade. "knob" joins the switch's
			// scope, so the switch still counts as hovered. A click needs
			// no ID: it reaches the switch either way (#661).
			ID:           "knob",
			Float:        true,
			FloatOffsetX: knobX,
			FloatOffsetY: (outerH - knob) / 2,
			Width:        knob,
			Height:       knob,
			Sizing:       gui.FixedFixed,
			Color:        white,
			Shadow:       knobShadow,
			SizeBorder:   gui.NoBorder,
			Padding:      gui.PaddingNone,
		})

		cfg := switchShell(gui.ContainerCfg{
			ID:     id,
			Width:  trackW,
			Height: outerH,
			// The track is centered in the taller box that the knob needs.
			Padding: gui.NewPadding((outerH-trackH)/2, 0, (outerH-trackH)/2, 0),
		}, label, on, onClick)
		cfg.Content = []gui.View{
			gui.Row(gui.ContainerCfg{
				Width:      trackW,
				Height:     trackH,
				Sizing:     gui.FixedFixed,
				Radius:     gui.SomeF(trackH / 2),
				Color:      hoverShade(track, on, s),
				SizeBorder: gui.NoBorder,
				// Inset from the rounded end, so the text stays out of
				// the half under the knob.
				Padding: gui.NewPadding(0, 10, 0, 10),
				HAlign:  textAlign,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{gui.Text(gui.TextCfg{Text: word, TextStyle: ts})},
			}),
			knobView,
		}
		return gui.Column(cfg)
	})
}

// darken scales the RGB channels by f.
func darken(c gui.Color, f float32) gui.Color {
	return gui.RGBA(uint8(float32(c.R)*f), uint8(float32(c.G)*f), uint8(float32(c.B)*f), c.A)
}

// lighten moves each RGB channel toward white by f.
func lighten(c gui.Color, f float32) gui.Color {
	up := func(v uint8) uint8 { return v + uint8(float32(255-v)*f) }
	return gui.RGBA(up(c.R), up(c.G), up(c.B), c.A)
}
