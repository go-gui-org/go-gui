package main

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func demoButton(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(8),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("Clicks: %d", app.ButtonClicks),
				TextStyle: gui.CurrentTheme().N3,
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: buttonFeatureRows(w),
			}),
		},
	})
}

func buttonFeatureRows(w *gui.Window) []gui.View {
	app := gui.State[ShowcaseApp](w)
	buttonText := fmt.Sprintf("%d Clicks Given", app.ButtonClicks)
	buttonWidth := float32(160)
	progress := float32(math.Mod(float64(app.ButtonClicks)/25.0, 1.0))
	copyLabel := "Copy to clipboard"
	if time.Now().Before(app.ButtonCopyUntil) {
		copyLabel = "Copied ✓"
	}

	return []gui.View{
		buttonFeatureRow("Plain ole button", gui.Button(gui.ButtonCfg{
			ID:         "showcase-button-plain",
			MinWidth:   buttonWidth,
			MaxWidth:   buttonWidth,
			SizeBorder: gui.NoBorder,
			Content:    []gui.View{gui.Text(gui.TextCfg{Text: buttonText})},
			OnClick:    showcaseButtonClick,
		})),
		buttonFeatureRow("Disabled button", gui.Button(gui.ButtonCfg{
			ID:       "showcase-button-disabled",
			MinWidth: buttonWidth,
			MaxWidth: buttonWidth,
			Disabled: true,
			Content:  []gui.View{gui.Text(gui.TextCfg{Text: buttonText})},
		})),
		buttonFeatureRow("With border", gui.Button(gui.ButtonCfg{
			ID:         "showcase-button-border",
			MinWidth:   buttonWidth,
			MaxWidth:   buttonWidth,
			SizeBorder: gui.SomeF(2),
			Content:    []gui.View{gui.Text(gui.TextCfg{Text: buttonText})},
			OnClick:    showcaseButtonClick,
		})),
		buttonFeatureRow("With other content", gui.Button(gui.ButtonCfg{
			ID:          "showcase-button-progress",
			MinWidth:    200,
			MaxWidth:    200,
			Color:       gui.RGB(195, 105, 0),
			ColorHover:  gui.RGB(195, 105, 0),
			ColorClick:  gui.RGB(205, 115, 0),
			SizeBorder:  gui.SomeF(2),
			ColorBorder: gui.RGB(160, 160, 160),
			Padding:     gui.Some(gui.CurrentTheme().PaddingMedium),
			VAlign:      gui.Some(gui.VAlignMiddle),
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: strconv.Itoa(app.ButtonClicks), MinWidth: 25}),
				gui.ProgressBar(gui.ProgressBarCfg{
					ID:      "showcase-button-progress",
					Percent: progress,
					Width:   75,
					Height:  gui.CurrentTheme().TextStyleDef.Size,
				}),
			},
			OnClick: showcaseButtonClick,
		})),
		buttonFeatureRow("Copy feedback", gui.Button(gui.ButtonCfg{
			ID:       "showcase-button-copy",
			MinWidth: buttonWidth,
			MaxWidth: buttonWidth,
			Content:  []gui.View{gui.Text(gui.TextCfg{Text: copyLabel})},
			OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
				incrementButtonClicks(w)
				gui.State[ShowcaseApp](w).ButtonCopyUntil = time.Now().Add(2 * time.Second)
				e.IsHandled = true
			},
		})),
	}
}

func buttonFeatureRow(label string, button gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: gui.CurrentTheme().N3}),
			gui.Row(gui.ContainerCfg{Sizing: gui.FillFit, Padding: gui.NoPadding}),
			button,
		},
	})
}

func incrementButtonClicks(w *gui.Window) {
	gui.State[ShowcaseApp](w).ButtonClicks++
}

func showcaseButtonClick(_ *gui.Layout, e *gui.Event, w *gui.Window) {
	incrementButtonClicks(w)
	e.IsHandled = true
}

func demoProgressBar(_ *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.ProgressBar(gui.ProgressBarCfg{ID: "showcase-pb-25", Percent: 0.25, TextShow: true, Sizing: gui.FillFit}),
			gui.ProgressBar(gui.ProgressBarCfg{ID: "showcase-pb-50", Percent: 0.50, TextShow: true, Sizing: gui.FillFit}),
			gui.ProgressBar(gui.ProgressBarCfg{ID: "showcase-pb-75", Percent: 0.75, TextShow: true, Sizing: gui.FillFit}),
			gui.ProgressBar(gui.ProgressBarCfg{ID: "showcase-pb-indef", Indefinite: true, Sizing: gui.FillFit}),
		},
	})
}

func demoPulsar(w *gui.Window) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(8),
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Loading", TextStyle: gui.CurrentTheme().N3}),
			gui.Pulsar(gui.PulsarCfg{
				Text1:     gui.IconSmile,
				Text2:     gui.IconSmileAlt,
				TextStyle: gui.CurrentTheme().Icon1,
			}, w),
		},
	})
}

func demoBadge(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	badgeRow := func(content []gui.View) gui.View {
		return gui.Row(gui.ContainerCfg{
			Sizing:     gui.FillFit,
			Spacing:    gui.SomeF(t.SpacingSmall),
			Padding:    gui.NoPadding,
			SizeBorder: gui.NoBorder,
			VAlign:     gui.VAlignMiddle,
			Content:    content,
		})
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SomeF(t.SpacingSmall),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Variants", TextStyle: t.B4}),
			badgeRow([]gui.View{
				gui.Badge(gui.BadgeCfg{Label: "5"}),
				gui.Badge(gui.BadgeCfg{Label: "3", Variant: gui.BadgeInfo}),
				gui.Badge(gui.BadgeCfg{Label: "12", Variant: gui.BadgeSuccess}),
				gui.Badge(gui.BadgeCfg{Label: "7", Variant: gui.BadgeWarning}),
				gui.Badge(gui.BadgeCfg{Label: "99", Variant: gui.BadgeError}),
			}),
			gui.Text(gui.TextCfg{Text: "Max cap", TextStyle: t.B4}),
			badgeRow([]gui.View{
				gui.Badge(gui.BadgeCfg{Label: "5", Max: 99}),
				gui.Badge(gui.BadgeCfg{Label: "150", Max: 99, Variant: gui.BadgeError}),
				gui.Badge(gui.BadgeCfg{Label: "1000", Max: 999, Variant: gui.BadgeInfo}),
			}),
			gui.Text(gui.TextCfg{Text: "Dot mode", TextStyle: t.B4}),
			badgeRow([]gui.View{
				gui.Badge(gui.BadgeCfg{Dot: true}),
				gui.Badge(gui.BadgeCfg{Dot: true, Variant: gui.BadgeInfo}),
				gui.Badge(gui.BadgeCfg{Dot: true, Variant: gui.BadgeSuccess}),
				gui.Badge(gui.BadgeCfg{Dot: true, Variant: gui.BadgeWarning}),
				gui.Badge(gui.BadgeCfg{Dot: true, Variant: gui.BadgeError}),
			}),
		},
	})
}

func demoCommandButton(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("Count: %d", app.CmdButtonCount),
				TextStyle: t.N3,
			}),
			sectionLabel(t, "Auto-labeled with shortcut hint"),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(8),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.CommandButton("sc.greet", gui.ButtonCfg{ID: "cb-greet"}),
					gui.CommandButton("sc.count", gui.ButtonCfg{ID: "cb-count"}),
				},
			}),
			sectionLabel(t, "Auto-disabled via CanExecute"),
			gui.CommandButton("sc.disabled", gui.ButtonCfg{ID: "cb-disabled"}),
		},
	})
}

func demoThemePicker(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	t := gui.CurrentTheme()
	result := app.ThemePickerResult
	if result == "" {
		result = "(none)"
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Palette icon that opens a dropdown of registered themes.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			gui.ThemePicker(gui.ThemePickerCfg{
				ID:          "theme-picker-demo",
				FloatAnchor: gui.FloatBottomLeft,
				FloatTieOff: gui.FloatTopLeft,
				OnSelect: func(name string, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).ThemePickerResult = name
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "Selected: " + result,
				TextStyle: t.N3,
			}),
		},
	})
}

func demoSkeleton(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SomeF(t.SpacingSmall),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			sectionLabel(t, "Text Lines"),
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(6),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Skeleton(gui.SkeletonCfg{ID: "sk-line1", Sizing: gui.FillFixed, Height: 14}),
					gui.Skeleton(gui.SkeletonCfg{ID: "sk-line2", Sizing: gui.FixedFixed, Width: 260, Height: 14}),
					gui.Skeleton(gui.SkeletonCfg{ID: "sk-line3", Sizing: gui.FixedFixed, Width: 180, Height: 14}),
				},
			}),

			line(),
			sectionLabel(t, "Circle Avatar"),
			gui.Skeleton(gui.SkeletonCfg{
				ID:      "sk-avatar",
				Variant: gui.SkeletonCircle,
				Width:   48,
				Height:  48,
			}),

			line(),
			sectionLabel(t, "Card Layout"),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(12),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignTop,
				Content: []gui.View{
					gui.Skeleton(gui.SkeletonCfg{
						ID:      "sk-card-avatar",
						Variant: gui.SkeletonCircle,
						Width:   48,
						Height:  48,
					}),
					gui.Column(gui.ContainerCfg{
						Sizing:     gui.FillFit,
						Spacing:    gui.SomeF(6),
						Padding:    gui.NoPadding,
						SizeBorder: gui.NoBorder,
						Content: []gui.View{
							gui.Skeleton(gui.SkeletonCfg{ID: "sk-card-l1", Sizing: gui.FillFixed, Height: 14}),
							gui.Skeleton(gui.SkeletonCfg{ID: "sk-card-l2", Sizing: gui.FixedFixed, Width: 200, Height: 14}),
							gui.Skeleton(gui.SkeletonCfg{ID: "sk-card-l3", Sizing: gui.FixedFixed, Width: 140, Height: 14}),
						},
					}),
				},
			}),

			line(),
			sectionLabel(t, "Custom Colors"),
			gui.Skeleton(gui.SkeletonCfg{
				ID:             "sk-custom",
				Sizing:         gui.FillFixed,
				Height:         24,
				Color:          gui.RGB(60, 60, 80),
				ColorHighlight: gui.RGB(100, 100, 140),
			}),
		},
	})
}

func demoToast(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(8),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      "btn-toast",
				Padding: gui.SomeP(8, 16, 8, 16),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Show Toast", TextStyle: t.N3}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					w.Toast(gui.ToastCfg{Title: "showcase", Body: "Hello from showcase!"})
					e.IsHandled = true
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "btn-toast-dismiss",
				Padding: gui.SomeP(8, 16, 8, 16),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Dismiss All", TextStyle: t.N3}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					w.ToastDismissAll()
					e.IsHandled = true
				},
			}),
		},
	})
}
