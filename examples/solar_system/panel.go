package main

import "github.com/go-gui-org/go-gui/gui"

const (
	// panelLiftPx is how far above center a selected planet is placed,
	// so the info panel does not cover it. camera.go converts it back
	// to world units.
	panelLiftPx = 110

	// panelWidth bounds the fact sheet so six stat cards sit in one row
	// and the fun fact wraps rather than stretching to the window.
	panelWidth = 760

	dotSize    = 14
	sunDotSize = 20
)

// zoomControls is the +/- pair, floated top-right.
func zoomControls(theme gui.Theme) gui.View {
	return gui.Row(gui.ContainerCfg{
		Float:       true,
		FloatAnchor: gui.FloatTopRight,
		FloatTieOff: gui.FloatTopRight,
		Padding:     theme.PaddingMedium,
		SizeBorder:  gui.NoBorder,
		Spacing:     gui.SomeF(theme.SpacingSmall),
		Content: []gui.View{
			zoomButton("solar_zoom_in", "+", "Zoom in", keyZoomStep),
			zoomButton("solar_zoom_out", "-", "Zoom out", 1/keyZoomStep),
		},
	})
}

func zoomButton(id, label, a11y string, factor float32) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:      id,
		Label:   label,
		Width:   34,
		Height:  30,
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
func infoPanel(a *App, theme gui.Theme) gui.View {
	content := make([]gui.View, 0, 2)
	if b := bodyAt(a.Selected); b != nil {
		content = append(content, factSheet(b, theme))
	}
	content = append(content, navDots(a, theme))

	return gui.Column(gui.ContainerCfg{
		Float:       true,
		FloatAnchor: gui.FloatBottomCenter,
		FloatTieOff: gui.FloatBottomCenter,
		HAlign:      gui.HAlignCenter,
		Padding:     theme.PaddingMedium,
		SizeBorder:  gui.NoBorder,
		Spacing:     gui.SomeF(theme.SpacingSmall),
		Content:     content,
	})
}

// factSheet is the selected body's name, six stat cards, and its fun
// fact. The sun goes through it unchanged: it carries the same copy
// fields, which is why it reuses Planet.
func factSheet(p *Planet, theme gui.Theme) gui.View {
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
		Radius:      gui.SomeF(10),
		Padding:     theme.PaddingLarge,
		// Fixed rather than max width: the fun fact wraps, and wrapping
		// needs a definite width to resolve against.
		Width:   panelWidth,
		HAlign:  gui.HAlignCenter,
		Spacing: gui.SomeF(theme.SpacingSmall),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      p.Name,
				TextStyle: theme.B1,
			}),
			gui.Row(gui.ContainerCfg{
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Spacing:    gui.SomeF(theme.SpacingSmall),
				Content: []gui.View{
					statCard("Diameter", p.Diameter, theme),
					statCard("Mass", p.Mass, theme),
					statCard("Distance", p.Distance, theme),
					statCard("Day", p.DayLength, theme),
					statCard("Gravity", p.Gravity, theme),
					statCard("Temp", p.Temperature, theme),
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      p.FunFact,
				TextStyle: factStyle,
				Mode:      gui.TextModeWrap,
				Sizing:    gui.FillFit,
			}),
		},
	})
}

// statCard is one label/value pair. The label takes the theme's label
// role rather than a locally spelled alpha.
func statCard(label, value string, theme gui.Theme) gui.View {
	return gui.Column(gui.ContainerCfg{
		Color:      theme.ColorInterior,
		Radius:     gui.SomeF(6),
		Padding:    theme.PaddingSmall,
		SizeBorder: gui.NoBorder,
		Sizing:     gui.FillFit,
		Spacing:    gui.SomeF(2),
		HAlign:     gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: theme.TextStyleLabel,
			}),
			gui.Text(gui.TextCfg{
				Text:      value,
				TextStyle: theme.B4,
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
		Spacing:    gui.SomeF(theme.SpacingSmall),
		Content:    dots,
	})
}

// navDot is one jump target. Color is the body's own at low opacity
// until it is the selection, which takes the theme accent.
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
		Radius:  gui.SomeF(size / 2),
		Padding: gui.NoPadding,
		Colors:  gui.Flat(color),
		A11YCfg: gui.A11YCfg{A11YLabel: name},
		OnClick: func(ctx gui.EventCtx) {
			selectBody(state(ctx.Window), sel)
			ctx.Consume()
		},
	})
}
