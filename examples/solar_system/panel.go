package main

import (
	"strconv"

	"github.com/go-gui-org/go-gui/gui"
)

const (
	// panelLiftPx is how far above center a selected planet is placed,
	// so the info panel does not cover it. camera.go converts it back
	// to world units.
	panelLiftPx = 110

	// panelWidth bounds the fact sheet so six stat cards sit in one row
	// and the fun fact wraps rather than stretching to the window.
	panelWidth = 760

	// panelMargin is the gap kept between the fact sheet and each side
	// of a window too narrow for panelWidth. Below panelWidth plus two
	// margins, the sheet shrinks to fit and the cards go two rows of
	// three, so nothing runs off the edge on a phone-width window.
	panelMargin = 16

	dotSize    = 14
	sunDotSize = 20

	// controlH is the height every top-right control shares, so the
	// text buttons line up with the +/- pair.
	controlH = 30
)

// controls is the top-right row: mute, pause, the speed readout and
// the +/- zoom pair.
func controls(a *App, theme gui.Theme) gui.View {
	mute := "Mute"
	if a.Muted {
		mute = "Unmute"
	}
	pause := "Pause"
	if a.Paused {
		pause = "Play"
	}
	return gui.Row(gui.ContainerCfg{
		Float:       true,
		FloatAnchor: gui.FloatTopRight,
		FloatTieOff: gui.FloatTopRight,
		Padding:     theme.PaddingMedium,
		SizeBorder:  gui.NoBorder,
		Spacing:     gui.SpacingSmall,
		VAlign:      gui.VAlignMiddle,
		Content: []gui.View{
			textButton("solar_mute", mute, "Toggle sound (M)",
				func(ctx gui.EventCtx) { toggleMute(state(ctx.Window), ctx.Window) }),
			textButton("solar_pause", pause, "Pause or resume the orbits (Space)",
				func(ctx gui.EventCtx) { togglePause(state(ctx.Window), ctx.Window) }),
			gui.Text(gui.TextCfg{
				ID:        "solar_speed",
				Text:      speedLabel(a.Speed),
				TextStyle: theme.TextStyleLabel,
			}),
			zoomButton("solar_zoom_in", "+", "Zoom in", keyZoomStep),
			zoomButton("solar_zoom_out", "-", "Zoom out", 1/keyZoomStep),
		},
	})
}

// speedLabel is the readout beside the pause button: "1x", "0.25x".
// It runs once per view pass, not per frame, so the string is fine.
func speedLabel(speed float32) string {
	return strconv.FormatFloat(float64(speed), 'g', -1, 32) + "x"
}

// textButton is a control-row button with a word for a label. The
// handler consumes the click itself so each caller does not repeat it.
func textButton(id, label, a11y string, act func(gui.EventCtx)) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:      id,
		Label:   label,
		Height:  controlH,
		A11YCfg: gui.A11YCfg{A11YLabel: a11y},
		OnClick: func(ctx gui.EventCtx) {
			act(ctx)
			ctx.Consume()
		},
	})
}

func zoomButton(id, label, a11y string, factor float32) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:      id,
		Label:   label,
		Width:   34,
		Height:  controlH,
		A11YCfg: gui.A11YCfg{A11YLabel: a11y},
		OnClick: func(ctx gui.EventCtx) {
			state(ctx.Window).applyUserZoom(factor)
			ctx.Consume()
		},
	})
}

// infoPanel is the bottom sheet, floated over the canvas. The nav dots
// are always present so the user can jump straight to a planet from the
// full-system view; the fact sheet above them appears only once
// something is selected.
//
// winW is the window width, or 0 when it is not yet known (a test
// window that has had no resize event), which reads as wide.
func infoPanel(a *App, theme gui.Theme, winW int) gui.View {
	content := make([]gui.View, 0, 2)
	if b := bodyAt(a.Selected); b != nil {
		content = append(content, factSheet(b, theme, winW))
	}
	content = append(content, navDots(a, theme))

	return gui.Column(gui.ContainerCfg{
		Float:       true,
		FloatAnchor: gui.FloatBottomCenter,
		FloatTieOff: gui.FloatBottomCenter,
		HAlign:      gui.HAlignCenter,
		Padding:     theme.PaddingMedium,
		SizeBorder:  gui.NoBorder,
		Spacing:     gui.SpacingSmall,
		Content:     content,
	})
}

// factSheet is the selected body's name, six stat cards, and its fun
// fact, with a close button in the corner. The sun goes through it
// unchanged: it carries the same copy fields, which is why it reuses
// Planet.
func factSheet(p *Planet, theme gui.Theme, winW int) gui.View {
	width, narrow := sheetWidth(winW)
	// The fun fact is centred on its own style rather than by the
	// card's HAlign: it fills the card's width so it has somewhere to
	// wrap, which leaves HAlign nothing to move. Align centres each
	// wrapped line inside that width. Copied so the theme's shared
	// role style is left alone.
	factStyle := theme.TextStyleSecondary
	factStyle.Align = gui.TextAlignCenter

	return gui.Column(gui.ContainerCfg{
		ID:          "solar_facts",
		Color:       gui.RGBA(14, 18, 32, 235),
		ColorBorder: theme.ColorBorder,
		Radius:      gui.RadiusLarge,
		Padding:     theme.PaddingLarge,
		// Fixed rather than max width: the fun fact wraps, and wrapping
		// needs a definite width to resolve against. FixedFit makes the
		// width bind: on a Fit axis it is only a starting size, and the
		// unwrapped fun fact grows the sheet past a narrow window.
		Sizing:  gui.FixedFit,
		Width:   width,
		HAlign:  gui.HAlignCenter,
		Spacing: gui.SpacingSmall,
		Content: []gui.View{
			closeButton(theme),
			gui.Text(gui.TextCfg{
				Text:      p.Name,
				TextStyle: theme.TextStyleDisplay,
			}),
			statRows(p, theme, narrow),
			gui.Text(gui.TextCfg{
				Text:      p.FunFact,
				TextStyle: factStyle,
				Mode:      gui.TextModeWrap,
				Sizing:    gui.FillFit,
			}),
		},
	})
}

// sheetWidth is the fact sheet's width for a window winW wide, and
// whether that window is narrow enough to stack the cards.
func sheetWidth(winW int) (width float32, narrow bool) {
	if winW <= 0 || winW >= panelWidth+2*panelMargin {
		return panelWidth, false
	}
	return max(float32(winW-2*panelMargin), 0), true
}

// closeButton sits in the sheet's top-right corner. It is floated so
// it takes no space from the centred name. Escape and a click on empty
// space do the same thing, but a touch screen has no Escape key and
// nothing says that empty space closes the sheet.
func closeButton(theme gui.Theme) gui.View {
	return gui.Row(gui.ContainerCfg{
		Float:       true,
		FloatAnchor: gui.FloatTopRight,
		FloatTieOff: gui.FloatTopRight,
		Padding:     theme.PaddingSmall,
		SizeBorder:  gui.NoBorder,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:    "close",
				Label: "×",
				// FixedFixed, or the button's own padding widens it.
				Sizing:  gui.FixedFixed,
				Width:   controlH,
				Height:  controlH,
				A11YCfg: gui.A11YCfg{A11YLabel: "Close fact sheet"},
				OnClick: func(ctx gui.EventCtx) {
					selectBody(state(ctx.Window), ctx.Window, -1)
					ctx.Consume()
				},
			}),
		},
	})
}

// statRows lays out the six stat cards: one row when wide, two rows of
// three when narrow.
func statRows(p *Planet, theme gui.Theme, narrow bool) gui.View {
	cards := []gui.View{
		statCard("Diameter", p.Diameter, theme),
		statCard("Mass", p.Mass, theme),
		statCard("Distance", p.Distance, theme),
		statCard("Day", p.DayLength, theme),
		statCard("Gravity", p.Gravity, theme),
		statCard("Temp", p.Temperature, theme),
	}
	if !narrow {
		return cardRow(cards)
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SpacingSmall,
		Content:    []gui.View{cardRow(cards[:3]), cardRow(cards[3:])},
	})
}

// cardRow is one row of stat cards. Narrow rows fill the sheet so the
// cards split its width evenly.
func cardRow(cards []gui.View) gui.View {
	sizing := gui.FitFit
	if len(cards) < 6 {
		sizing = gui.FillFit
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:     sizing,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SpacingSmall,
		Content:    cards,
	})
}

// statCard is one label/value pair. The label takes the theme's label
// role rather than a locally spelled alpha.
func statCard(label, value string, theme gui.Theme) gui.View {
	return gui.Column(gui.ContainerCfg{
		Color:      theme.ColorInterior,
		Radius:     gui.RadiusMedium,
		Padding:    theme.PaddingSmall,
		SizeBorder: gui.NoBorder,
		Sizing:     gui.FillFit,
		Spacing:    gui.SpacingTight,
		HAlign:     gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: theme.TextStyleLabel,
			}),
			gui.Text(gui.TextCfg{
				Text:      value,
				TextStyle: theme.TextStyleBodySmall.Bold(),
			}),
		},
	})
}

// navDots is one dot for the sun and one per planet, in that order, so
// the row reads sun-outward like the system it stands for. The selected
// one takes the accent, and the sun's dot is larger because it is the
// one body that is not a planet.
func navDots(a *App, theme gui.Theme) gui.View {
	dots := make([]gui.View, 0, len(planets)+1)
	dots = append(dots, navDot(a, gui.ScopeID("solar", "dot", "sun"),
		selSun, sun.Name, sunDotSize, theme))
	for i := range planets {
		// ScopeIDN composes the numeric segment without building the
		// number as its own string. Never hand-join with ":".
		dots = append(dots, navDot(a, gui.ScopeIDN("solar", "dot", i), i,
			planets[i].Name, dotSize, theme))
	}

	return gui.Row(gui.ContainerCfg{
		Padding:    theme.PaddingSmall,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SpacingSmall,
		Content:    dots,
	})
}

// navDot is one jump target. Color is the body's own at low opacity
// until it is the selection, which takes the theme accent.
//
// Hovering a dot names its body with the canvas tooltip, the one a
// hovered planet gets. gui.WithTooltip opens below its target, and the
// dots sit on the bottom edge of the window, so it would open off
// screen. A Button reports no pointer leave either; how the name goes
// away without one is on App.DotHover.
func navDot(a *App, id string, sel int, name string, size float32,
	theme gui.Theme,
) gui.View {
	b := bodyAt(sel)
	color := b.Color.WithOpacity(0.45)
	if sel == a.Selected {
		color = theme.ColorAccent
	}
	return gui.Button(gui.ButtonCfg{
		ID:      id,
		Width:   size,
		Height:  size,
		Radius:  gui.RadiusPx(size / 2),
		Padding: gui.NoPadding,
		Colors:  gui.Flat(color),
		A11YCfg: gui.A11YCfg{A11YLabel: name},
		OnClick: func(ctx gui.EventCtx) {
			selectBody(state(ctx.Window), ctx.Window, sel)
			ctx.Consume()
		},
		// Runs in the layout pass of every frame the pointer is over
		// the dot. No refresh is needed: the canvas redraws every tick.
		OnHover: func(ctx gui.EventCtx) {
			state(ctx.Window).DotHover = sel
		},
	})
}
