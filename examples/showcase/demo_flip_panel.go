package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
)

const (
	flipPanelW float32 = 160
	flipPanelH float32 = 100
)

// demoFlipPanel shows gui.FlipPanel (#1020). The app owns which face
// each panel shows: OnFlip inverts a bool in app state, and the panel
// runs the turn. Every panel here except the last also turns with
// Enter or Space when it has focus.
func demoFlipPanel(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "FlipPanel turns over in 3D to show its back face. Click a panel, or focus it and press Enter or Space. A second click during a turn sends it back smoothly. The face that is turned away takes no clicks, has no tab stops and is hidden from screen readers. When the system asks for reduced motion, the faces cross-fade instead.",
				TextStyle: t.TextStyleBody,
				Mode:      gui.TextModeWrap,
			}),
			sectionLabel(t, "Axis"),
			flipPanelRow(
				flipPanelCaptioned(t, "About Y (default)", gui.FlipPanel(gui.FlipPanelCfg{
					ID:      "flip-y",
					Sizing:  gui.FixedFixed,
					Width:   flipPanelW,
					Height:  flipPanelH,
					Flipped: app.FlipPanelY,
					Front:   flipPanelFace(t, "Front", t.ColorPanel),
					Back:    flipPanelFace(t, "Back", t.ColorActive),
					A11YCfg: gui.A11YCfg{A11YLabel: "Flip about Y"},
					OnFlip: func(ctx gui.EventCtx) {
						a := appState(ctx.Window)
						a.FlipPanelY = !a.FlipPanelY
					},
				})),
				flipPanelCaptioned(t, "About X", gui.FlipPanel(gui.FlipPanelCfg{
					ID:      "flip-x",
					Sizing:  gui.FixedFixed,
					Width:   flipPanelW,
					Height:  flipPanelH,
					Axis:    gui.FlipAxisX,
					Flipped: app.FlipPanelX,
					Front:   flipPanelFace(t, "Front", t.ColorPanel),
					Back:    flipPanelFace(t, "Back", t.ColorActive),
					A11YCfg: gui.A11YCfg{A11YLabel: "Flip about X"},
					OnFlip: func(ctx gui.EventCtx) {
						a := appState(ctx.Window)
						a.FlipPanelX = !a.FlipPanelX
					},
				})),
			),
			sectionLabel(t, "Options"),
			flipPanelRow(
				flipPanelCaptioned(t, "Controls on the back", flipPanelDetails(t, app)),
				flipPanelCaptioned(t, "Bouncy spring", gui.FlipPanel(gui.FlipPanelCfg{
					ID:      "flip-bouncy",
					Sizing:  gui.FixedFixed,
					Width:   flipPanelW,
					Height:  flipPanelH,
					Spring:  gui.SpringBouncy,
					Flipped: app.FlipPanelBouncy,
					Front:   flipPanelFace(t, "Spring", t.ColorPanel),
					Back:    flipPanelFace(t, "Bouncy", t.ColorActive),
					A11YCfg: gui.A11YCfg{A11YLabel: "Bouncy spring"},
					OnFlip: func(ctx gui.EventCtx) {
						a := appState(ctx.Window)
						a.FlipPanelBouncy = !a.FlipPanelBouncy
					},
				})),
				flipPanelCaptioned(t, "Click only", gui.FlipPanel(gui.FlipPanelCfg{
					ID:     "flip-click-only",
					Sizing: gui.FixedFixed,
					Width:  flipPanelW,
					Height: flipPanelH,
					// Out of the tab order: the panel turns only on a
					// click.
					FocusDisabled: true,
					Flipped:       app.FlipPanelClickOnly,
					Front:         flipPanelFace(t, "No tab stop", t.ColorPanel),
					Back:          flipPanelFace(t, "Back", t.ColorActive),
					A11YCfg:       gui.A11YCfg{A11YLabel: "Click only"},
					OnFlip: func(ctx gui.EventCtx) {
						a := appState(ctx.Window)
						a.FlipPanelClickOnly = !a.FlipPanelClickOnly
					},
				})),
			),
		},
	})
}

// flipPanelDetails has a button on its back face. The button consumes
// its click, so pressing it counts a like and does not turn the panel.
// A click anywhere else on the back turns it over again.
func flipPanelDetails(t gui.Theme, app *ShowcaseApp) gui.View {
	back := gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PaddingMedium,
		Spacing: gui.SpacingSmall,
		Radius:  gui.RadiusMedium,
		Color:   t.ColorActive,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID: "flip-like",
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      fmt.Sprintf("Like (%d)", app.FlipPanelLikes),
						TextStyle: t.TextStyleBody,
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					appState(ctx.Window).FlipPanelLikes++
					ctx.Consume()
				},
			}),
		},
	})
	return gui.FlipPanel(gui.FlipPanelCfg{
		ID:      "flip-details",
		Sizing:  gui.FixedFixed,
		Width:   flipPanelW,
		Height:  flipPanelH,
		Flipped: app.FlipPanelDetails,
		Front:   flipPanelFace(t, "Details", t.ColorPanel),
		Back:    back,
		A11YCfg: gui.A11YCfg{A11YLabel: "Controls on the back"},
		OnFlip: func(ctx gui.EventCtx) {
			a := appState(ctx.Window)
			a.FlipPanelDetails = !a.FlipPanelDetails
		},
	})
}

// flipPanelFace is a plain face: a colored, rounded box with a label.
// It fills the panel, so both faces match the panel's fixed size.
func flipPanelFace(t gui.Theme, label string, color gui.Color) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PaddingMedium,
		Radius:  gui.RadiusMedium,
		Color:   color,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.TextStyleTitleSmall}),
		},
	})
}

// flipPanelCaptioned puts a caption above a panel.
func flipPanelCaptioned(t gui.Theme, caption string, panel gui.View) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FitFit,
		Spacing:    gui.SpacingSmall,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: caption, TextStyle: t.TextStyleCaption.Bold()}),
			panel,
		},
	})
}

// flipPanelRow lays captioned panels side by side.
func flipPanelRow(content ...gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SpacingLarge,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content:    content,
	})
}
