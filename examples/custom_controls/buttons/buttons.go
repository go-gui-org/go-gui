// Package buttons demonstrates custom button looks built from hover and press
// state read while the view is built (advanced: build-time interaction state).
//
// It ports go-shirei's custom-buttons demo (issue #587). Three looks:
//
//   - Flat buttons. The filled ones need no new API: gui.Button already takes
//     per-state colors. The outline button changes its text color on hover,
//     which a color set cannot express, so it reads the hover state instead.
//   - Windows 98 buttons. A press swaps the bevel and moves the label down and
//     right by 1px. That changes padding, not just color.
//   - Windows XP buttons. Hover adds a gold rim; a press flips the gradient.
//
// Each custom button is a gui.Interactive. It gives the look function the
// hover and press state of the button, and the look function builds the
// matching view. The rule
// that keeps this stable: change only what is inside the button's bounds.
// Every look here keeps its outer size fixed.
package buttons

import (
	"github.com/go-gui-org/go-gui/gui"
)

// App holds the click log and the three "disable the target" switches.
type App struct {
	log           string
	flatDisabled  bool
	win98Disabled bool
	xpDisabled    bool

	// Click handlers are built once, so a frame does not allocate a new
	// closure per button.
	onClick map[string]func(gui.EventCtx)
	toggle  map[string]func(gui.EventCtx)
}

// New returns the page state with its starting values.
func New() *App {
	app := &App{log: "Click a button."}
	app.onClick = map[string]func(gui.EventCtx){}
	for _, name := range []string{
		"flat Primary", "flat Success", "flat Danger", "flat Outline", "flat target",
		"98 OK", "98 Cancel", "98 Apply", "98 target",
		"XP OK", "XP Cancel", "XP Apply", "XP target",
	} {
		app.onClick[name] = func(ctx gui.EventCtx) {
			app.log = "Clicked: " + name
			ctx.Consume()
		}
	}
	app.toggle = map[string]func(gui.EventCtx){
		"flat": func(ctx gui.EventCtx) { app.flatDisabled = !app.flatDisabled; ctx.Consume() },
		"98":   func(ctx gui.EventCtx) { app.win98Disabled = !app.win98Disabled; ctx.Consume() },
		"xp":   func(ctx gui.EventCtx) { app.xpDisabled = !app.xpDisabled; ctx.Consume() },
	}
	return app
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
		Padding:    gui.PadAll(24),
		Spacing:    gui.SomeF(14),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Custom buttons from IsHovered / IsPressed", TextStyle: title}),

			sectionTitle("Default gui.Button, for comparison"),
			row("default", gui.ColorTransparent,
				gui.Button(gui.ButtonCfg{ID: "primary", Label: "Primary", Variant: gui.ButtonPrimary}),
				gui.Button(gui.ButtonCfg{ID: "secondary", Label: "Secondary"}),
				gui.Button(gui.ButtonCfg{ID: "disabled", Label: "Disabled", Disabled: true}),
			),

			sectionTitle("Flat (Material / Bootstrap)"),
			row("flat", gui.ColorTransparent,
				flatButton("primary", "Primary", flatPrimary, false, app.onClick["flat Primary"]),
				flatButton("success", "Success", flatSuccess, false, app.onClick["flat Success"]),
				flatButton("danger", "Danger", flatDanger, false, app.onClick["flat Danger"]),
				outlineButton("outline", "Outline", flatPrimary, app.onClick["flat Outline"]),
				flatButton("disabled", "Disabled", flatPrimary, true, nil),
			),
			toggleRow("flat-toggle", "Disable the next flat button", app.flatDisabled,
				app.toggle["flat"],
				flatButton("target", "Toggle target", flatPrimary, app.flatDisabled,
					app.onClick["flat target"])),

			sectionTitle("Windows 98 (raised bevel)"),
			row("win98", gui.Hex(0xbfbfbf),
				win98Button("ok", "OK", false, app.onClick["98 OK"]),
				win98Button("cancel", "Cancel", false, app.onClick["98 Cancel"]),
				win98Button("apply", "Apply", false, app.onClick["98 Apply"]),
				win98Button("disabled", "Disabled", true, nil),
			),
			toggleRow("win98-toggle", "Disable the next 98 button", app.win98Disabled,
				app.toggle["98"],
				win98Button("target", "Toggle target", app.win98Disabled,
					app.onClick["98 target"])),

			sectionTitle("Windows XP Luna (XP.css)"),
			row("xp", gui.Hex(0xece9d8),
				xpButton("ok", "OK", false, app.onClick["XP OK"]),
				xpButton("cancel", "Cancel", false, app.onClick["XP Cancel"]),
				xpButton("apply", "Apply", false, app.onClick["XP Apply"]),
				xpButton("disabled", "Disabled", true, nil),
			),
			toggleRow("xp-toggle", "Disable the next XP button", app.xpDisabled,
				app.toggle["xp"],
				xpButton("target", "Toggle target", app.xpDisabled,
					app.onClick["XP target"])),

			gui.Text(gui.TextCfg{ID: "log", Text: app.log}),
		},
	})
}

func sectionTitle(s string) gui.View {
	return gui.Text(gui.TextCfg{Text: s, TextStyle: gui.CurrentTheme().TextStyleLabel})
}

// row is one ID-bearing group. The ID scopes the buttons inside it, so "ok"
// in the 98 row is "page:win98:ok" and "ok" in the XP row is "page:xp:ok".
func row(id string, bg gui.Color, content ...gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		ID:         id,
		Color:      bg,
		Padding:    gui.PadAll(12),
		Spacing:    gui.SomeF(10),
		SizeBorder: gui.NoBorder,
		Content:    content,
	})
}

func toggleRow(id, label string, selected bool, onClick func(gui.EventCtx), target gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		ID:         id,
		Padding:    gui.NewPadding(0, 12, 0, 12),
		Spacing:    gui.SomeF(10),
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Toggle(gui.ToggleCfg{ID: "disable", Label: label, Selected: selected, OnClick: onClick}),
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

// buttonShell is the part every custom button shares: the ID that makes it
// a hover and press target, click and keyboard activation, and the
// accessibility role. The look goes in content.
func buttonShell(cfg gui.ContainerCfg, label string, disabled bool, onClick func(gui.EventCtx)) gui.ContainerCfg {
	cfg.OnClick = onClick
	cfg.ClickOnSpace = true
	cfg.ClickOnEnter = true
	cfg.Focusable = true
	cfg.Disabled = disabled
	cfg.A11YRole = gui.AccessRoleButton
	cfg.A11YCfg = gui.A11YCfg{A11YLabel: label}
	return cfg
}

// --- Flat ------------------------------------------------------------------

var (
	flatPrimary = gui.Hex(0x0d6efd)
	flatSuccess = gui.Hex(0x198754)
	flatDanger  = gui.Hex(0xdc3545)
	flatMuted   = gui.Hex(0xc7c7c7)
)

// flatButton needs no build-time state: only the fill changes, and
// gui.Button already takes a color per state.
func flatButton(id, label string, accent gui.Color, disabled bool, onClick func(gui.EventCtx)) gui.View {
	face := accent
	if disabled {
		face = flatMuted
	}
	return gui.Button(gui.ButtonCfg{
		ID:         id,
		Disabled:   disabled,
		OnClick:    onClick,
		Radius:     gui.SomeF(4),
		SizeBorder: gui.NoBorder,
		Padding:    gui.NewPadding(8, 16, 8, 16),
		Colors: gui.ColorSet{
			Base:  face,
			Hover: darken(accent, 0.9),
			Click: darken(accent, 0.8),
		},
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: textStyle(gui.White, 13)}),
		},
	})
}

// outlineButton fills on hover and turns its text white. A text color is
// not part of a color set, so the look is picked while the view is built.
func outlineButton(id, label string, accent gui.Color, onClick func(gui.EventCtx)) gui.View {
	// gui.Interactive reads the state by the effective ID ("flat:outline"
	// here, inside the "flat" row), so the look only names the leaf.
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		return outlineLook(id, label, accent, s, onClick)
	})
}

func outlineLook(id, label string, accent gui.Color, s gui.InteractionState, onClick func(gui.EventCtx)) gui.View {
	bg, text := gui.White, accent
	switch {
	case s.Armed:
		bg, text = darken(accent, 0.85), gui.White
	case s.Hovered:
		bg, text = accent, gui.White
	}
	cfg := buttonShell(gui.ContainerCfg{
		ID:          id,
		Color:       bg,
		ColorBorder: accent,
		SizeBorder:  gui.SomeF(1.5),
		Radius:      gui.SomeF(4),
		Padding:     gui.NewPadding(7, 15, 7, 15),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: textStyle(text, 13)}),
		},
	}, label, false, onClick)
	return gui.Row(cfg)
}

// --- Windows 98 ------------------------------------------------------------

var (
	win98Face      = gui.Hex(0xd4d0c8)
	win98FaceHover = gui.Hex(0xdcd8d0)
	win98FacePress = gui.Hex(0xc8c4bc)
	win98Light     = gui.Hex(0xffffff)
	win98Shadow    = gui.Hex(0x737373)
	win98Frame     = gui.Hex(0x404040)
	win98Text      = gui.Hex(0x1a1a1a)
	win98Muted     = gui.Hex(0x8c8c8c)
)

// win98Button is a raised 3D push button. The bevel is two nested
// containers: a dark one padded on the bottom and right, a light one padded
// on the top and left. A press swaps the two and moves the label padding by
// 1px, so the label shifts down and right while the outer size stays fixed.
func win98Button(id, label string, disabled bool, onClick func(gui.EventCtx)) gui.View {
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		return win98Look(id, label, disabled, s, onClick)
	})
}

func win98Look(id, label string, disabled bool, s gui.InteractionState, onClick func(gui.EventCtx)) gui.View {
	face, light, shadow, text := win98Face, win98Light, win98Shadow, win98Text
	top, right, bottom, left := float32(5), float32(14), float32(5), float32(14)
	// The disabled case comes first. A disabled shape is never hovered,
	// but a press held from before the toggle stays recorded.
	switch {
	case disabled:
		text = win98Muted
	// "Armed": pressed and the pointer still over the button. Dragging off
	// a held button pops it back up, as on Windows.
	case s.Armed:
		light, shadow = shadow, light
		face = win98FacePress
		top, right, bottom, left = 6, 13, 4, 15
	case s.Hovered:
		face = win98FaceHover
	}

	cfg := buttonShell(gui.ContainerCfg{
		ID:         id,
		Color:      win98Frame,
		Radius:     gui.SomeF(0),
		Padding:    gui.PadAll(1),
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			bevel(shadow, gui.NewPadding(0, 1, 1, 0),
				bevel(light, gui.NewPadding(1, 0, 0, 1),
					gui.Row(gui.ContainerCfg{
						Color:      face,
						Radius:     gui.SomeF(0),
						Padding:    gui.NewPadding(top, right, bottom, left),
						SizeBorder: gui.NoBorder,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: label, TextStyle: textStyle(text, 12)}),
						},
					}))),
		},
	}, label, disabled, onClick)
	return gui.Column(cfg)
}

func bevel(c gui.Color, pad gui.Padding, content gui.View) gui.View {
	return gui.Column(gui.ContainerCfg{
		Color:      c,
		Radius:     gui.SomeF(0),
		Padding:    pad,
		SizeBorder: gui.NoBorder,
		Content:    []gui.View{content},
	})
}

// --- Windows XP ------------------------------------------------------------

var (
	xpBorder      = gui.Hex(0x003c74)
	xpBorderMuted = gui.Hex(0xc9c7ba)
	xpGoldOuter   = gui.Hex(0xe5a01a)
	xpGoldInner   = gui.Hex(0xfdd889)
	xpText        = gui.Hex(0x222222)
	xpMuted       = gui.Hex(0xa19e93)
	xpFaceMuted   = gui.Hex(0xf4f3ee)

	// Built once: a GradientDef is a pointer, so sharing it costs nothing
	// per frame.
	xpIdle = &gui.GradientDef{Direction: gui.GradientToBottom, Stops: []gui.GradientStop{
		{Color: gui.Hex(0xffffff), Pos: 0},
		{Color: gui.Hex(0xecebe5), Pos: 0.86},
		{Color: gui.Hex(0xd8d0c4), Pos: 1},
	}}
	xpPressed = &gui.GradientDef{Direction: gui.GradientToBottom, Stops: []gui.GradientStop{
		{Color: gui.Hex(0xcdcac3), Pos: 0},
		{Color: gui.Hex(0xe3e3db), Pos: 0.08},
		{Color: gui.Hex(0xf2f2f1), Pos: 1},
	}}
)

// xpButton is a Luna command button: blue frame, white-to-beige gradient,
// a gold rim while hovered or pressed, and an inverted gradient while
// pressed. The rim is always there and only its color changes, so hover
// never changes the button's size.
func xpButton(id, label string, disabled bool, onClick func(gui.EventCtx)) gui.View {
	return gui.Interactive(id, func(s gui.InteractionState) gui.View {
		return xpLook(id, label, disabled, s, onClick)
	})
}

func xpLook(id, label string, disabled bool, s gui.InteractionState, onClick func(gui.EventCtx)) gui.View {
	// A press held from before a disable toggle stays recorded, so the
	// rim still checks disabled.
	lit := !disabled && (s.Hovered || s.Pressed)

	border, text := xpBorder, xpText
	grad := xpIdle
	var faceColor gui.Color
	rimOuter, rimInner := gui.ColorTransparent, gui.ColorTransparent
	top, right, bottom, left := float32(4), float32(12), float32(4), float32(12)
	switch {
	case disabled:
		border, text, grad, faceColor = xpBorderMuted, xpMuted, nil, xpFaceMuted
	case s.Armed:
		grad = xpPressed
		top, right, bottom, left = 5, 11, 3, 13
	}
	// Idle, the rims and the face are see-through, so the outer gradient
	// paints the whole button in one piece. Lit, the gold rims cover it, so the
	// face repaints the gradient inside them.
	faceGrad, faceFill := (*gui.GradientDef)(nil), faceColor
	if lit {
		rimOuter, rimInner = xpGoldOuter, xpGoldInner
		faceGrad = grad
	}

	// The blue frame is the body's 1px border, drawn over the gradient.
	cfg := buttonShell(gui.ContainerCfg{
		ID:          id,
		MinWidth:    75,
		Radius:      gui.SomeF(3),
		Color:       faceColor,
		Gradient:    grad,
		ColorBorder: border,
		SizeBorder:  gui.SomeF(1),
		Padding:     gui.PaddingNone,
		Content: []gui.View{rim(rimOuter, gui.NewPadding(0, 3, 3, 0),
			rim(rimInner, gui.NewPadding(3, 0, 0, 3),
				gui.Row(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					Color:      faceFill,
					Gradient:   faceGrad,
					Radius:     gui.SomeF(2),
					Padding:    gui.NewPadding(top, right, bottom, left),
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: label, TextStyle: textStyle(text, 11)}),
					},
				})))},
	}, label, disabled, onClick)
	return gui.Column(cfg)
}

func rim(c gui.Color, pad gui.Padding, content gui.View) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Color:      c,
		Radius:     gui.SomeF(2),
		Padding:    pad,
		SizeBorder: gui.NoBorder,
		Content:    []gui.View{content},
	})
}

// darken scales the RGB channels by f.
func darken(c gui.Color, f float32) gui.Color {
	return gui.RGBA(uint8(float32(c.R)*f), uint8(float32(c.G)*f), uint8(float32(c.B)*f), c.A)
}
