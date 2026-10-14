package main

import (
	"fmt"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

const (
	transformFlipW float32 = 160
	transformFlipH float32 = 100
	// playgroundSliderInset is the symmetric side inset of the
	// playground slider block: ~10% of the default detail width per
	// side, so the tracks read ~20% shorter and stay centered. The
	// layout engine has no fractional widths, so this stays a fixed
	// inset rather than a proportion.
	playgroundSliderInset float32 = 60
)

func demoTransform(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "ContainerCfg.Transform moves, turns, scales or skews a container and everything in it. Layout ignores it: the container keeps its place, and so do its neighbours. Pointer input follows the drawn view.",
				TextStyle: t.TextStyleBody,
				Mode:      gui.TextModeWrap,
			}),
			sectionLabel(t, "Basics"),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.NoPadding,
				Content: []gui.View{
					transformSample(t, "Translate(24, 12)", gui.Translate(24, 12)),
					transformSample(t, "Rotate(15)", gui.Rotate(15)),
					transformSample(t, "Scale(1.3)", gui.Scale(1.3)),
					transformSample(t, "Skew(12, 6)", gui.Skew(12, 6)),
				},
			}),
			sectionLabel(t, "Origin"),
			gui.Text(gui.TextCfg{
				Text:      "Every part turns about the origin, the center by default. Origin takes fractions of the box: (0, 0) is the top-left corner.",
				TextStyle: t.TextStyleCaption,
				Mode:      gui.TextModeWrap,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.NoPadding,
				Content: []gui.View{
					transformSample(t, "top-left", gui.Rotate(30).Origin(0, 0)),
					transformSample(t, "center", gui.Rotate(30)),
					transformSample(t, "bottom-right", gui.Rotate(30).Origin(1, 1)),
				},
			}),
			sectionLabel(t, "Mirror and perspective"),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				Padding: gui.NoPadding,
				Content: []gui.View{
					transformSample(t, "ScaleXY(-1, 1)", gui.ScaleXY(-1, 1)),
					transformSample(t, "RotateX(35)", gui.Perspective(500).RotateX(35)),
					transformSample(t, "RotateY(-35)", gui.Perspective(500).RotateY(-35)),
				},
			}),
			sectionLabel(t, "Playground"),
			// A corrupt restore (NaN/Inf) must reach neither the
			// transform nor the readout; the 0.01 floor keeps a
			// corrupt zero scale from collapsing the preview.
			// (max alone is not enough: max(NaN, x) is NaN.)
			func() gui.View {
				tx := safeFloat(app.TransformTX, 0)
				ty := safeFloat(app.TransformTY, 0)
				rot := safeFloat(app.TransformRotate, 0)
				scl := max(safeFloat(app.TransformScale, 1), 0.01)
				mx := safeFloat(app.TransformMirrorX, 1)
				rx := safeFloat(app.TransformRotX, 0)
				ry := safeFloat(app.TransformRotY, 0)
				persp := safeFloat(app.TransformPersp, 500)
				if persp <= 0 {
					persp = 500
				}
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFit,
					Spacing:    gui.SpacingMedium,
					Padding:    gui.NoPadding,
					SizeBorder: gui.NoBorder,
					Content: []gui.View{
						gui.Text(gui.TextCfg{
							ID:        "transform-readout",
							Text:      fmt.Sprintf("Translate(%.0f, %.0f).Rotate(%.0f).Scale(%.2f).MirrorX(%.2f).RotateX(%.0f).RotateY(%.0f).Perspective(%.0f)", tx, ty, rot, scl, mx, rx, ry, persp),
							TextStyle: t.TextStyleBody,
						}),
						gui.Row(gui.ContainerCfg{
							Sizing:     gui.FillFit,
							Padding:    gui.NoPadding,
							SizeBorder: gui.NoBorder,
							Spacing:    gui.NoSpacing,
							HAlign:     gui.HAlignCenter,
							Content: []gui.View{
								gui.Column(gui.ContainerCfg{
									Width:       240,
									Height:      150,
									Sizing:      gui.FixedFixed,
									ColorBorder: t.ColorBorder,
									SizeBorder:  gui.BorderThin,
									Radius:      gui.RadiusSmall,
									HAlign:      gui.HAlignCenter,
									VAlign:      gui.VAlignMiddle,
									Padding:     gui.NoPadding,
									Content: []gui.View{
										transformPlaygroundBox(t, app.TransformPlaygroundText, gui.Translate(tx, ty).Perspective(persp).RotateX(rx).RotateY(ry).Rotate(rot).ScaleXY(scl*mx, scl)),
									},
								}),
							},
						}),
						gui.Column(gui.ContainerCfg{
							Sizing:     gui.FillFit,
							Spacing:    gui.SpacingMedium,
							Padding:    gui.NewPadding(0, playgroundSliderInset, 0, playgroundSliderInset),
							SizeBorder: gui.NoBorder,
							Content: []gui.View{
								transformSlider(t, "transform-rotate", "Rotate", rot, -180, 180, func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).TransformRotate = v
								}),
								transformSlider(t, "transform-scale", "Scale", scl, 0.2, 2, func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).TransformScale = v
								}),
								transformSlider(t, "transform-tx", "Translate X", tx, -40, 40, func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).TransformTX = v
								}),
								transformSlider(t, "transform-ty", "Translate Y", ty, -40, 40, func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).TransformTY = v
								}),
								transformSlider(t, "transform-mirror", "Mirror X", mx, -1, 1, func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).TransformMirrorX = v
								}),
								transformSlider(t, "transform-rotx", "Rotate X", rx, -60, 60, func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).TransformRotX = v
								}),
								transformSlider(t, "transform-roty", "Rotate Y", ry, -60, 60, func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).TransformRotY = v
								}),
								transformSlider(t, "transform-persp", "Perspective", persp, 200, 1200, func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).TransformPersp = v
								}),
							},
						}),
					},
				})
			}(),
			gui.Button(gui.ButtonCfg{
				ID:      "transform-reset",
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Reset", TextStyle: t.TextStyleBody})},
				OnClick: func(ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.TransformRotate = 0
					a.TransformScale = 1
					a.TransformTX = 0
					a.TransformTY = 0
					a.TransformMirrorX = 1
					a.TransformRotX = 0
					a.TransformRotY = 0
					a.TransformPersp = 500
				},
			}),
			sectionLabel(t, "Animated"),
			gui.Text(gui.TextCfg{
				Text:      "TransformFn is read on every draw and wins over Transform: a tween moves the view in render-only frames with no rebuild. HoverLift owns that tween for a hover lift; the flip card runs its own.",
				TextStyle: t.TextStyleCaption,
				Mode:      gui.TextModeWrap,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingLarge,
				Padding: gui.NoPadding,
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					transformLiftCard(t, app),
					transformFlipCard(w, t),
				},
			}),
		},
	})
}

// transformSample is a captioned stage: a bordered box that keeps its
// place, with the transformed box drawn inside it. The border shows
// that layout ignores the transform.
func transformSample(t gui.Theme, caption string, xf gui.Transform) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FitFit,
		Spacing:    gui.SpacingSmall,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: caption, TextStyle: t.TextStyleCaption.Bold()}),
			gui.Column(gui.ContainerCfg{
				Width:       140,
				Height:      100,
				Sizing:      gui.FixedFixed,
				ColorBorder: t.ColorBorder,
				SizeBorder:  gui.BorderThin,
				Radius:      gui.RadiusSmall,
				HAlign:      gui.HAlignCenter,
				VAlign:      gui.VAlignMiddle,
				Padding:     gui.NoPadding,
				Content: []gui.View{
					transformBox(t, "Abc", t.ColorSelect, xf),
				},
			}),
		},
	})
}

// transformBox is the small labeled box a sample or the playground
// draws through a transform.
func transformBox(t gui.Theme, label string, color gui.Color, xf gui.Transform) gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:     72,
		Height:    44,
		Sizing:    gui.FixedFixed,
		Color:     color,
		Radius:    gui.RadiusSmall,
		Padding:   gui.NoPadding,
		HAlign:    gui.HAlignCenter,
		VAlign:    gui.VAlignMiddle,
		Transform: xf,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.TextStyleBody}),
		},
	})
}

// transformPlaygroundBox is the playground's sample: a small dialog
// box with a text field and Cancel/OK buttons, drawn through the
// playground transform. A realistic control mix shows what a
// transform does to everyday content better than a bare box.
func transformPlaygroundBox(t gui.Theme, text string, xf gui.Transform) gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:       180,
		Sizing:      gui.FixedFit,
		Color:       t.ColorPanel,
		ColorBorder: t.ColorBorder,
		SizeBorder:  gui.BorderThin,
		Radius:      gui.RadiusSmall,
		Padding:     gui.PaddingSmall,
		Spacing:     gui.SpacingSmall,
		HAlign:      gui.HAlignCenter,
		VAlign:      gui.VAlignMiddle,
		Transform:   xf,
		Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID:          "transform-playground-input",
				Sizing:      gui.FillFit,
				Text:        text,
				Placeholder: "Type here...",
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					appState(ctx.Window).TransformPlaygroundText = s
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NoPadding,
				Spacing: gui.SpacingSmall,
				HAlign:  gui.HAlignRight,
				Content: []gui.View{
					// Both buttons clear the field: a click that lands
					// through a mirror or a perspective turn proves the
					// pointer follows the drawn view (hit testing). The
					// buttons take explicit IDs so tests can click them.
					gui.Button(gui.ButtonCfg{
						ID:      "transform-playground-cancel",
						Variant: gui.ButtonSecondary,
						Label:   "Cancel",
						OnClick: func(ctx gui.EventCtx) {
							appState(ctx.Window).TransformPlaygroundText = ""
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "transform-playground-ok",
						Variant: gui.ButtonPrimary,
						Label:   "OK",
						OnClick: func(ctx gui.EventCtx) {
							appState(ctx.Window).TransformPlaygroundText = ""
						},
					}),
				},
			}),
		},
	})
}

func transformSlider(t gui.Theme, id, label string, value, min, max float32, onChange func(float32, gui.EventCtx)) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.TextStyleBodySmall}),
			gui.Slider(gui.SliderCfg{
				ID:       id,
				Value:    value,
				Min:      min,
				Max:      max,
				Sizing:   gui.FillFit,
				OnChange: onChange,
			}),
		},
	})
}

// transformLiftCard rises while the pointer is over it. The lift
// amount lives in app state; HoverLift moves it toward 0 or 1 and
// hands back a TransformFn that lerps the transform from it.
func transformLiftCard(t gui.Theme, app *ShowcaseApp) gui.View {
	lifted := gui.Translate(0, -6).Scale(1.05)
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FitFit,
		Spacing:    gui.SpacingSmall,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Hover me", TextStyle: t.TextStyleCaption.Bold()}),
			gui.HoverLift("transform-lift", lifted, &app.TransformLift,
				func(xf func() gui.Transform) gui.View {
					return gui.Column(gui.ContainerCfg{
						ID:          "transform-lift",
						Width:       transformFlipW,
						Height:      transformFlipH,
						Sizing:      gui.FixedFixed,
						Padding:     gui.PaddingMedium,
						Radius:      gui.RadiusMedium,
						Color:       t.ColorPanel,
						ColorBorder: t.ColorBorder,
						SizeBorder:  gui.BorderThin,
						HAlign:      gui.HAlignCenter,
						VAlign:      gui.VAlignMiddle,
						TransformFn: xf,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Lift", TextStyle: t.TextStyleTitleSmall}),
						},
					})
				}),
		},
	})
}

// transformFlipCard turns over in 3D on a click. Both faces sit on
// the same spot: the back face is moved left by one card width onto
// the front. Each face hides while its back is toward the viewer, so
// only one shows at a time.
func transformFlipCard(w *gui.Window, t gui.Theme) gui.View {
	angle := func() float32 { return appState(w).TransformAngle }
	front := func() gui.Transform {
		return gui.Perspective(600).RotateY(angle()).BackfaceHidden()
	}
	back := func() gui.Transform {
		return gui.Translate(-transformFlipW, 0).Perspective(600).
			RotateY(angle() + 180).BackfaceHidden()
	}
	face := func(id, label string, color gui.Color, fn func() gui.Transform) gui.View {
		return gui.Column(gui.ContainerCfg{
			ID:          id,
			Width:       transformFlipW,
			Height:      transformFlipH,
			Sizing:      gui.FixedFixed,
			Padding:     gui.PaddingMedium,
			Radius:      gui.RadiusMedium,
			Color:       color,
			HAlign:      gui.HAlignCenter,
			VAlign:      gui.VAlignMiddle,
			TransformFn: fn,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: label, TextStyle: t.TextStyleTitleSmall}),
			},
		})
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FitFit,
		Spacing:    gui.SpacingSmall,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Click to flip", TextStyle: t.TextStyleCaption.Bold()}),
			gui.Row(gui.ContainerCfg{
				ID:         "transform-flip",
				Width:      transformFlipW,
				Height:     transformFlipH,
				Sizing:     gui.FixedFixed,
				Padding:    gui.PaddingNone,
				Spacing:    gui.NoSpacing,
				SizeBorder: gui.NoBorder,
				OnClick:    transformFlip,
				Content: []gui.View{
					face("transform-flip-front", "Front", t.ColorPanel, front),
					face("transform-flip-back", "Back", t.ColorActive, back),
				},
			}),
		},
	})
}

// transformFlip starts the flip animation toward the other face. The
// angle is read through TransformFn when the card is drawn, so the
// tween moves the card in render-only frames with no rebuild.
func transformFlip(ctx gui.EventCtx) {
	s := appState(ctx.Window)
	s.TransformFlipped = !s.TransformFlipped
	var to float32
	if s.TransformFlipped {
		to = 180
	}
	tw := gui.NewTweenAnimation("transform-flip", s.TransformAngle, to, func(v float32, w *gui.Window) {
		appState(w).TransformAngle = v
	})
	tw.Duration = 500 * time.Millisecond
	tw.Easing = gui.EaseInOutCubic
	tw.Refresh = gui.AnimationRefreshRenderOnly
	ctx.Window.AnimationAdd(tw)
	ctx.Consume()
}
