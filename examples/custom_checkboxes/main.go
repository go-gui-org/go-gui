// This example demonstrates custom checkbox looks built from hover and press
// state read while the view is built (advanced: build-time interaction state).
//
// It ports go-shirei's custom-checkboxes demo. Two looks:
//
//   - Material. A rounded box with a 2px border that fills with the accent
//     color and shows a white check mark when checked.
//   - Windows XP (Luna). A sunken box: blue frame, gray-to-white face, a gold
//     rim on hover, a darker face while pressed, and a green check mark.
//
// Each checkbox is a gui.Interactive around a row that holds the box and its
// label, so a click on the label also toggles it. The app owns the checked
// value; the look reads it together with the hover and press state. The
// outer size never changes.
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
		Title:  "Custom Checkboxes",
		Width:  680,
		Height: 600,
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

// checkNames lists every value in App.on. A name is the label shown and the
// text written to the log.
var checkNames = []string{
	"Default A", "Default B",
	"Show notifications", "Connect to Wi-Fi automatically", "Sync in the background",
	"Material target",
	"Show all options", "Play Windows startup sound", "Hide icons when desktop is locked",
	"XP target",
	// The two "disable the next box" switches.
	"Disable Material", "Disable XP",
}

// App holds each checked value and the last action.
type App struct {
	on  map[string]bool
	log string

	// Flip handlers are built once, so a frame does not allocate a new
	// closure per checkbox.
	flip map[string]func(gui.EventCtx)
}

func newApp() *App {
	app := &App{
		log: "Toggle a checkbox.",
		on: map[string]bool{
			"Default A":                         true,
			"Show notifications":                true,
			"Sync in the background":            true,
			"Material target":                   true,
			"Show all options":                  true,
			"Hide icons when desktop is locked": true,
			"XP target":                         true,
		},
		flip: map[string]func(gui.EventCtx){},
	}
	for _, name := range checkNames {
		app.flip[name] = func(ctx gui.EventCtx) {
			app.on[name] = !app.on[name]
			if app.on[name] {
				app.log = name + " → checked"
			} else {
				app.log = name + " → unchecked"
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
			gui.Text(gui.TextCfg{Text: "Custom checkboxes from IsHovered / IsPressed", TextStyle: title}),

			sectionTitle("Default gui.Toggle, for comparison"),
			group("default", gui.ColorTransparent, 8,
				gui.Toggle(gui.ToggleCfg{ID: "a", Label: "Enable notifications",
					Selected: app.on["Default A"], OnClick: app.flip["Default A"]}),
				gui.Toggle(gui.ToggleCfg{ID: "b", Label: "Play sound",
					Selected: app.on["Default B"], OnClick: app.flip["Default B"]}),
			),

			sectionTitle("Material Design inspired"),
			group("material", gui.ColorTransparent, 10,
				materialCheck("notify", "Show notifications", app, materialPrimary, false),
				materialCheck("wifi", "Connect to Wi-Fi automatically", app, materialPrimary, false),
				materialCheck("sync", "Sync in the background", app, materialTeal, false),
				materialCheck("disabled", "Show notifications", app, materialPrimary, true),
			),
			toggleRow("material-toggle", "Disable the next Material box", "Disable Material", app,
				materialCheck("target", "Material target", app, materialPrimary,
					app.on["Disable Material"])),

			sectionTitle("Windows XP (Luna / XP.css inspired)"),
			group("xp", xpSurface, 8,
				xpCheck("options", "Show all options", app, false),
				xpCheck("sounds", "Play Windows startup sound", app, false),
				xpCheck("icons", "Hide icons when desktop is locked", app, false),
				xpCheck("disabled", "Show all options", app, true),
			),
			toggleRow("xp-toggle", "Disable the next XP box", "Disable XP", app,
				xpCheck("target", "XP target", app, app.on["Disable XP"])),

			gui.Text(gui.TextCfg{ID: "log", Text: app.log}),
		},
	})
}

func sectionTitle(s string) gui.View {
	return gui.Text(gui.TextCfg{Text: s, TextStyle: gui.CurrentTheme().TextStyleLabel})
}

// group is one ID-bearing column. The ID scopes the checkboxes inside it, so
// "disabled" in the Material group is "page:material:disabled" and in the XP
// group "page:xp:disabled".
func group(id string, bg gui.Color, spacing float32, content ...gui.View) gui.View {
	pad, border := gui.PaddingNone, gui.NoBorder
	if bg != gui.ColorTransparent {
		// The XP group is a dialog surface with a thin frame.
		pad, border = gui.PadAll(14), gui.SomeF(1)
	}
	return gui.Column(gui.ContainerCfg{
		ID:          id,
		Color:       bg,
		ColorBorder: xpSurfaceBorder,
		SizeBorder:  border,
		Radius:      gui.SomeF(0),
		Padding:     pad,
		Spacing:     gui.SomeF(spacing),
		Content:     content,
	})
}

// toggleRow pairs a default gui.Toggle that disables a target with that
// target, to show a checkbox changing between enabled and disabled.
func toggleRow(id, label, key string, app *App, target gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		ID:         id,
		Padding:    gui.PaddingNone,
		Spacing:    gui.SomeF(16),
		SizeBorder: gui.NoBorder,
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			gui.Toggle(gui.ToggleCfg{ID: "disable", Label: label,
				Selected: app.on[key], OnClick: app.flip[key]}),
			target,
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

// checkMark is the icon font's check glyph in a color and size.
func checkMark(c gui.Color, size float32) gui.View {
	ts := gui.CurrentTheme().Icon4
	ts.Color = c
	ts.Size = size
	return gui.Text(gui.TextCfg{Text: gui.IconCheck, TextStyle: ts})
}

// checkShell is the part every custom checkbox shares: the ID that makes it a
// hover and press target, click and keyboard activation, and the checkbox
// role with its checked state. The look goes in cfg.
func checkShell(cfg gui.ContainerCfg, label string, checked, disabled bool, onClick func(gui.EventCtx)) gui.ContainerCfg {
	cfg.OnClick = onClick
	cfg.ClickOnSpace = true
	cfg.ClickOnEnter = true
	cfg.Focusable = true
	cfg.Disabled = disabled
	cfg.A11YRole = gui.AccessRoleCheckbox
	if checked {
		cfg.A11YState = gui.AccessStateChecked
	}
	cfg.A11YCfg = gui.A11YCfg{A11YLabel: label}
	cfg.Padding = gui.PaddingNone
	cfg.SizeBorder = gui.NoBorder
	cfg.VAlign = gui.VAlignMiddle
	return cfg
}

var (
	pageBG    = gui.Hex(0xf4f4f6)
	white     = gui.Hex(0xffffff)
	textDark  = gui.Hex(0x262626)
	textMuted = gui.Hex(0x8c8c8c)
)

// --- Material --------------------------------------------------------------

var (
	materialPrimary = gui.Hex(0x2277d3)
	materialTeal    = gui.Hex(0x29a397)
	materialBorder  = gui.Hex(0x8c8c8c)
	materialMuted   = gui.Hex(0xbfbfbf)
	materialOffFill = gui.Hex(0xf0f0f0)
)

// materialCheck is a Material checkbox. key names its value in App.on; the
// label is key too, except for the disabled sample, which shares a value
// with another box as in go-shirei.
func materialCheck(id, key string, app *App, accent gui.Color, disabled bool) gui.View {
	label := key
	if id == "disabled" {
		label = "Disabled sample"
	}
	checked, onClick := app.on[key], app.flip[key]
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		const size, radius = 18, 2
		border, bg, text := materialBorder, white, textDark
		switch {
		case disabled:
			border, bg, text = materialMuted, materialOffFill, textMuted
			if checked {
				bg, border = gui.Hex(0xc7c7c7), gui.Hex(0xc7c7c7)
			}
		case checked:
			bg = accent
			switch {
			case s.Armed:
				bg = darken(accent, 0.88)
			case s.Hovered:
				bg = lighten(accent, 0.12)
			}
			border = bg
		case s.Armed:
			border, bg = accent, mix(white, accent, 0.18)
		case s.Hovered:
			border, bg = accent, mix(white, accent, 0.08)
		}

		var mark []gui.View
		if checked {
			mark = []gui.View{checkMark(white, 14)}
		}
		return gui.Row(checkShell(gui.ContainerCfg{
			ID:      id,
			Spacing: gui.SomeF(10),
			Content: []gui.View{
				gui.Row(gui.ContainerCfg{
					Width:       size,
					Height:      size,
					Sizing:      gui.FixedFixed,
					Radius:      gui.SomeF(radius),
					Color:       bg,
					ColorBorder: border,
					SizeBorder:  gui.SomeF(2),
					Padding:     gui.PaddingNone,
					HAlign:      gui.HAlignCenter,
					VAlign:      gui.VAlignMiddle,
					Content:     mark,
				}),
				gui.Text(gui.TextCfg{Text: label, TextStyle: textStyle(text, 14)}),
			},
		}, label, checked, disabled, onClick))
	})
}

// --- Windows XP ------------------------------------------------------------

// Luna colors, from XP.css (https://botoxparty.github.io/XP.css/).
var (
	xpSurface       = gui.Hex(0xece9d8)
	xpSurfaceBorder = gui.Hex(0xbab8ab)
	xpBorder        = gui.Hex(0x1d5281)
	xpBorderMuted   = gui.Hex(0xcac8bb)
	xpGoldLo        = gui.Hex(0xf8b636)
	xpGoldHi        = gui.Hex(0xfedf9c)
	xpTickGreen     = gui.Hex(0x22a122)
	xpText          = gui.Hex(0x222222)

	// Built once: a GradientDef is a pointer, so sharing it costs nothing
	// per frame.
	xpFaceIdle = &gui.GradientDef{Direction: gui.GradientToBottomRight, Stops: []gui.GradientStop{
		{Color: gui.Hex(0xdcdcd7), Pos: 0},
		{Color: gui.Hex(0xffffff), Pos: 1},
	}}
	xpFacePressed = &gui.GradientDef{Direction: gui.GradientToBottomRight, Stops: []gui.GradientStop{
		{Color: gui.Hex(0xb0b0a7), Pos: 0},
		{Color: gui.Hex(0xe3e1d2), Pos: 1},
	}}
)

// xpCheck is a Luna checkbox. It stays sunken in every state; only the face
// and the rim react to hover and press. The rim is always there and only
// changes color, so hover never changes the size.
func xpCheck(id, key string, app *App, disabled bool) gui.View {
	label := key
	if id == "disabled" {
		label = "Disabled sample"
	}
	checked, onClick := app.on[key], app.flip[key]
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		border, text, tick := xpBorder, xpText, xpTickGreen
		face, faceColor := xpFaceIdle, gui.Color{}
		switch {
		case disabled:
			border, text, tick = xpBorderMuted, textMuted, xpBorderMuted
			face, faceColor = nil, white
		case s.Armed:
			face = xpFacePressed
		}
		// Idle, the rim is see-through, so the face gradient shows through
		// it. Hovered and not pressed, it turns gold: darker on the bottom
		// and right, lighter on the top and left, a warm sunken glow.
		rimLo, rimHi := gui.ColorTransparent, gui.ColorTransparent
		if !disabled && s.Hovered && !s.Armed {
			rimLo, rimHi = xpGoldLo, xpGoldHi
		}
		var mark []gui.View
		if checked {
			mark = []gui.View{checkMark(tick, 10)}
		}

		box := gui.Column(gui.ContainerCfg{
			// The frame is a 1px pad of border color around the well.
			Color:      border,
			Radius:     gui.SomeF(0),
			Padding:    gui.PadAll(1),
			SizeBorder: gui.NoBorder,
			Content: []gui.View{gui.Column(gui.ContainerCfg{
				Color:      faceColor,
				Gradient:   face,
				Radius:     gui.SomeF(0),
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					bevel(rimLo, gui.NewPadding(0, 1, 1, 0),
						bevel(rimHi, gui.NewPadding(1, 0, 0, 1),
							gui.Row(gui.ContainerCfg{
								Width:      11,
								Height:     11,
								Sizing:     gui.FixedFixed,
								Radius:     gui.SomeF(0),
								Padding:    gui.PaddingNone,
								SizeBorder: gui.NoBorder,
								HAlign:     gui.HAlignCenter,
								VAlign:     gui.VAlignMiddle,
								Content:    mark,
							}))),
				},
			})},
		})

		return gui.Row(checkShell(gui.ContainerCfg{
			ID:      id,
			Spacing: gui.SomeF(6),
			Content: []gui.View{
				box,
				gui.Text(gui.TextCfg{Text: label, TextStyle: textStyle(text, 12)}),
			},
		}, label, checked, disabled, onClick))
	})
}

// bevel pads content with one color on the sides pad names.
func bevel(c gui.Color, pad gui.Padding, content gui.View) gui.View {
	return gui.Column(gui.ContainerCfg{
		Color:      c,
		Radius:     gui.SomeF(0),
		Padding:    pad,
		SizeBorder: gui.NoBorder,
		Content:    []gui.View{content},
	})
}

// darken scales the RGB channels by f.
func darken(c gui.Color, f float32) gui.Color {
	return gui.RGBA(uint8(float32(c.R)*f), uint8(float32(c.G)*f), uint8(float32(c.B)*f), c.A)
}

// lighten moves each RGB channel toward white by f.
func lighten(c gui.Color, f float32) gui.Color {
	return mix(c, white, f)
}

// mix moves each RGB channel of a toward b by f.
func mix(a, b gui.Color, f float32) gui.Color {
	ch := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*f) }
	return gui.RGBA(ch(a.R, b.R), ch(a.G, b.G), ch(a.B, b.B), a.A)
}
