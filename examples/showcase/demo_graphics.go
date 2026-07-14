package main

import (
	"sort"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func demoRectangle(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Width:   100,
				Height:  60,
				Sizing:  gui.FixedFixed,
				Color:   t.ColorActive,
				Radius:  gui.NoRadius,
				HAlign:  gui.HAlignCenter,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Sharp", TextStyle: t.N2})},
			}),
			gui.Column(gui.ContainerCfg{
				Width:   100,
				Height:  60,
				Sizing:  gui.FixedFixed,
				Color:   t.ColorSelect,
				Radius:  gui.SomeF(8),
				HAlign:  gui.HAlignCenter,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Rounded", TextStyle: t.N2})},
			}),
			gui.Column(gui.ContainerCfg{
				Width:       100,
				Height:      60,
				Sizing:      gui.FixedFixed,
				Color:       gui.ColorTransparent,
				ColorBorder: t.ColorActive,
				SizeBorder:  gui.SomeF(2),
				Radius:      gui.SomeF(4),
				HAlign:      gui.HAlignCenter,
				VAlign:      gui.VAlignMiddle,
				Content:     []gui.View{gui.Text(gui.TextCfg{Text: "Border", TextStyle: t.N2})},
			}),
			gui.Column(gui.ContainerCfg{
				Width:  100,
				Height: 60,
				Sizing: gui.FixedFixed,
				Color:  t.ColorHover,
				Radius: gui.SomeF(30),
				HAlign: gui.HAlignCenter,
				VAlign: gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Pill", TextStyle: t.N2}),
				},
			}),
		},
	})
}

func demoIcons(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	keys := make([]string, 0, len(gui.IconLookup))
	for key := range gui.IconLookup {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	cellMinWidth := float32(0)
	for _, key := range keys {
		labelWidth := w.TextWidth(strings.TrimPrefix(key, "icon_"), t.N5)
		if labelWidth > cellMinWidth {
			cellMinWidth = labelWidth
		}
	}

	cols := 5
	rows := make([]gui.View, 0, (len(keys)+cols-1)/cols)
	for i := 0; i < len(keys); i += cols {
		end := i + cols
		end = min(end, len(keys))

		icons := make([]gui.View, 0, end-i)
		for _, key := range keys[i:end] {
			icons = append(icons, gui.Column(gui.ContainerCfg{
				MinWidth: cellMinWidth,
				Padding:  gui.NoPadding,
				HAlign:   gui.HAlignCenter,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: gui.IconLookup[key], TextStyle: t.Icon1}),
					gui.Text(gui.TextCfg{Text: strings.TrimPrefix(key, "icon_"), TextStyle: t.N5}),
				},
			}))
		}

		rows = append(rows, gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Padding: gui.NoPadding,
			Spacing: gui.NoSpacing,
			Content: icons,
		}))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.Some(t.SpacingSmall),
		Padding: gui.NoPadding,
		Content: rows,
	})
}

func demoGradient(_ *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SomeF(12),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(12),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:  120,
						Height: 80,
						Sizing: gui.FixedFixed,
						Gradient: &gui.GradientDef{
							Direction: gui.GradientToRight,
							Stops: []gui.GradientStop{
								{Pos: 0, Color: gui.ColorFromString("#3b82f6")},
								{Pos: 1, Color: gui.ColorFromString("#8b5cf6")},
							},
						},
						Radius: gui.SomeF(8),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Horizontal",
								TextStyle: gui.TextStyle{Color: gui.RGB(255, 255, 255), Size: 14},
							}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:  120,
						Height: 80,
						Sizing: gui.FixedFixed,
						Gradient: &gui.GradientDef{
							Direction: gui.GradientToTop,
							Stops: []gui.GradientStop{
								{Pos: 0, Color: gui.ColorFromString("#f97316")},
								{Pos: 1, Color: gui.ColorFromString("#ef4444")},
							},
						},
						Radius: gui.SomeF(8),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Vertical",
								TextStyle: gui.TextStyle{Color: gui.RGB(255, 255, 255), Size: 14},
							}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:  120,
						Height: 80,
						Sizing: gui.FixedFixed,
						Gradient: &gui.GradientDef{
							Direction: gui.GradientToTopRight,
							Stops: []gui.GradientStop{
								{Pos: 0, Color: gui.ColorFromString("#10b981")},
								{Pos: 0.5, Color: gui.ColorFromString("#3b82f6")},
								{Pos: 1, Color: gui.ColorFromString("#8b5cf6")},
							},
						},
						Radius: gui.SomeF(8),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Diagonal",
								TextStyle: gui.TextStyle{Color: gui.RGB(255, 255, 255), Size: 14},
							}),
						},
					}),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(12),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:  120,
						Height: 80,
						Sizing: gui.FixedFixed,
						Gradient: &gui.GradientDef{
							Type: gui.GradientRadial,
							Stops: []gui.GradientStop{
								{Pos: 0, Color: gui.ColorFromString("#facc15")},
								{Pos: 1, Color: gui.ColorFromString("#f97316")},
							},
						},
						Radius: gui.SomeF(8),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Radial",
								TextStyle: gui.TextStyle{Color: gui.RGB(255, 255, 255), Size: 14},
							}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:  120,
						Height: 80,
						Sizing: gui.FixedFixed,
						Gradient: &gui.GradientDef{
							Type: gui.GradientRadial,
							Stops: []gui.GradientStop{
								{Pos: 0, Color: gui.ColorFromString("#ffffff")},
								{Pos: 0.4, Color: gui.ColorFromString("#ec4899")},
								{Pos: 1, Color: gui.ColorFromString("#7c3aed")},
							},
						},
						Radius: gui.SomeF(8),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Radial Multi",
								TextStyle: gui.TextStyle{Color: gui.RGB(255, 255, 255), Size: 14},
							}),
						},
					}),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(12),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:      120,
						Height:     80,
						Sizing:     gui.FixedFixed,
						SizeBorder: gui.SomeF(2),
						BorderGradient: &gui.GradientDef{
							Direction: gui.GradientToRight,
							Stops: []gui.GradientStop{
								{Pos: 0, Color: gui.ColorFromString("#3b82f6")},
								{Pos: 1, Color: gui.ColorFromString("#8b5cf6")},
							},
						},
						Radius: gui.SomeF(8),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Border H",
								TextStyle: gui.TextStyle{Size: 14},
							}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:      120,
						Height:     80,
						Sizing:     gui.FixedFixed,
						SizeBorder: gui.SomeF(2),
						BorderGradient: &gui.GradientDef{
							Direction: gui.GradientToBottom,
							Stops: []gui.GradientStop{
								{Pos: 0, Color: gui.ColorFromString("#f97316")},
								{Pos: 1, Color: gui.ColorFromString("#ef4444")},
							},
						},
						Radius: gui.SomeF(8),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Border V",
								TextStyle: gui.TextStyle{Size: 14},
							}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:      120,
						Height:     80,
						Sizing:     gui.FixedFixed,
						SizeBorder: gui.SomeF(2),
						BorderGradient: &gui.GradientDef{
							Direction: gui.GradientToTopRight,
							Stops: []gui.GradientStop{
								{Pos: 0, Color: gui.ColorFromString("#10b981")},
								{Pos: 0.5, Color: gui.ColorFromString("#3b82f6")},
								{Pos: 1, Color: gui.ColorFromString("#8b5cf6")},
							},
						},
						Radius: gui.SomeF(8),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{
								Text:      "Border Diag",
								TextStyle: gui.TextStyle{Size: 14},
							}),
						},
					}),
				},
			}),
		},
	})
}

func demoBoxShadows(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	cardColor := t.ColorBackground
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.Some(t.SpacingMedium),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "offset_x/offset_y move the shadow. blur_radius controls softness.",
				TextStyle: t.N5,
				Mode:      gui.TextModeWrap,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(40),
				Padding: gui.NoPadding,
				Content: []gui.View{
					showcaseShadowCard("Soft depth", "Blur 12, Y 3", cardColor, gui.RGBA(0, 0, 0, 40), 0, 3, 12),
					showcaseShadowCard("Elevated", "Blur 22, Y 10", cardColor, gui.RGBA(0, 0, 0, 55), 0, 10, 22),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(40),
				Padding: gui.NoPadding,
				Content: []gui.View{
					showcaseShadowCard("Directional", "Blur 10, X 8, Y 8", cardColor, gui.RGBA(0, 0, 0, 65), 8, 8, 10),
					showcaseShadowCard("Blue glow", "Blur 24, no offset", cardColor, gui.RGBA(80, 120, 255, 85), 0, 0, 24),
				},
			}),
		},
	})
}

func showcaseShadowCard(title, note string, bg, shadowColor gui.Color, shadowOffsetX, shadowOffsetY, shadowBlur float32) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Width:       170,
		Height:      96,
		Sizing:      gui.FixedFixed,
		Padding:     gui.SomeP(10, 10, 10, 10),
		Spacing:     gui.SomeF(2),
		Radius:      gui.SomeF(10),
		Color:       bg,
		ColorBorder: t.ColorBorder,
		SizeBorder:  gui.SomeF(1),
		Shadow: &gui.BoxShadow{
			Color:      shadowColor,
			OffsetX:    shadowOffsetX,
			OffsetY:    shadowOffsetY,
			BlurRadius: shadowBlur,
		},
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: title, TextStyle: t.B5}),
			gui.Text(gui.TextCfg{
				Text:      note,
				TextStyle: t.N5,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}

func demoSvg(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(16),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(16),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Svg(gui.SvgCfg{
						ID:     "svg-circle",
						Width:  100,
						Height: 100,
						Sizing: gui.FixedFixed,
						SvgData: `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <circle cx="50" cy="50" r="40" fill="#3b82f6" opacity="0.8"/>
  <circle cx="50" cy="50" r="25" fill="#8b5cf6" opacity="0.8"/>
  <circle cx="50" cy="50" r="10" fill="#ec4899"/>
</svg>`,
					}),
					gui.Svg(gui.SvgCfg{
						ID:     "svg-star",
						Width:  100,
						Height: 100,
						Sizing: gui.FixedFixed,
						SvgData: `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <polygon points="50,5 61,35 95,35 68,57 79,91 50,70 21,91 32,57 5,35 39,35"
    fill="#f59e0b" stroke="#d97706" stroke-width="2"/>
</svg>`,
					}),
					gui.Svg(gui.SvgCfg{
						ID:     "svg-bars",
						Width:  120,
						Height: 100,
						Sizing: gui.FixedFixed,
						SvgData: `<svg viewBox="0 0 120 100" xmlns="http://www.w3.org/2000/svg">
  <rect x="10" y="60" width="20" height="35" rx="3" fill="#10b981"/>
  <rect x="35" y="40" width="20" height="55" rx="3" fill="#3b82f6"/>
  <rect x="60" y="20" width="20" height="75" rx="3" fill="#8b5cf6"/>
  <rect x="85" y="50" width="20" height="45" rx="3" fill="#f59e0b"/>
</svg>`,
					}),
					gui.Svg(gui.SvgCfg{
						ID:      "svg-tiger",
						Width:   120,
						Height:  100,
						Sizing:  gui.FixedFixed,
						SvgData: embeddedText("assets/tiger.svg"),
					}),
					gui.Column(gui.ContainerCfg{
						Clip:       true,
						Width:      70,
						Height:     70,
						Sizing:     gui.FixedFixed,
						Padding:    gui.NoPadding,
						SizeBorder: gui.NoBorder,
						Content: []gui.View{
							gui.Svg(gui.SvgCfg{
								ID:     "svg-clip",
								Width:  100,
								Height: 100,
								Sizing: gui.FixedFixed,
								SvgData: `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <circle cx="50" cy="50" r="45" fill="#3b82f6"/>
  <circle cx="50" cy="50" r="30" fill="#8b5cf6"/>
  <circle cx="50" cy="50" r="15" fill="#ec4899"/>
</svg>`,
							}),
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{Text: "Gradient spread methods (linearGradient + spreadMethod)", TextStyle: t.B4}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(16),
				Padding: gui.NoPadding,
				Content: []gui.View{
					svgSpreadSample(t, "svg-spread-pad", "pad", "pad"),
					svgSpreadSample(t, "svg-spread-reflect", "reflect", "reflect"),
					svgSpreadSample(t, "svg-spread-repeat", "repeat", "repeat"),
				},
			}),
			gui.Text(gui.TextCfg{Text: "Radial gradients (centered + focal offset)", TextStyle: t.B4}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(16),
				Padding: gui.NoPadding,
				Content: []gui.View{
					svgRadialSample(t, "svg-radial-center", "center",
						`<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <radialGradient id="rg" cx="50%" cy="50%" r="50%">
      <stop offset="0%" stop-color="#fbbf24"/>
      <stop offset="100%" stop-color="#7c3aed"/>
    </radialGradient>
  </defs>
  <rect width="100" height="100" fill="url(#rg)"/>
</svg>`),
					svgRadialSample(t, "svg-radial-focal", "focal fx/fy",
						`<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <radialGradient id="rg" cx="50%" cy="50%" r="55%" fx="25%" fy="25%">
      <stop offset="0%" stop-color="#f8fafc"/>
      <stop offset="100%" stop-color="#1e40af"/>
    </radialGradient>
  </defs>
  <circle cx="50" cy="50" r="45" fill="url(#rg)"/>
</svg>`),
					svgRadialSample(t, "svg-radial-reflect", "radial reflect",
						`<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <radialGradient id="rg" cx="50%" cy="50%" r="20%" spreadMethod="reflect">
      <stop offset="0%" stop-color="#10b981"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </radialGradient>
  </defs>
  <rect width="100" height="100" fill="url(#rg)"/>
</svg>`),
				},
			}),
			gui.Text(gui.TextCfg{Text: "CSS selectors (class, attribute, sibling, :not, var, calc)", TextStyle: t.B4}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(16),
				Padding: gui.NoPadding,
				Content: []gui.View{
					svgRadialSample(t, "svg-css-class", "class",
						`<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <style>
    .dot { fill: #3b82f6; }
    .hot { fill: #ec4899; }
  </style>
  <circle class="dot" cx="25" cy="50" r="15"/>
  <circle class="hot" cx="55" cy="50" r="15"/>
  <circle class="dot" cx="85" cy="50" r="10"/>
</svg>`),
					svgRadialSample(t, "svg-css-attr", "[data-state]",
						`<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <style>
    rect { fill: #475569; }
    rect[data-state="active"] { fill: #10b981; }
  </style>
  <rect x="10" y="20" width="20" height="60"/>
  <rect x="40" y="20" width="20" height="60" data-state="active"/>
  <rect x="70" y="20" width="20" height="60"/>
</svg>`),
					svgRadialSample(t, "svg-css-sibling", "sibling +",
						`<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <style>
    circle { fill: #1e293b; }
    circle + circle { fill: #f59e0b; }
  </style>
  <circle cx="20" cy="50" r="12"/>
  <circle cx="50" cy="50" r="12"/>
  <circle cx="80" cy="50" r="12"/>
</svg>`),
					svgRadialSample(t, "svg-css-not", ":not()",
						`<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <style>
    rect:not(.skip) { fill: #8b5cf6; }
    rect.skip { fill: #1e293b; }
  </style>
  <rect x="10" y="35" width="20" height="30"/>
  <rect x="40" y="35" width="20" height="30" class="skip"/>
  <rect x="70" y="35" width="20" height="30"/>
</svg>`),
					svgRadialSample(t, "svg-css-var", "var() + calc()",
						`<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <style>
    svg { --accent: #06b6d4; --r: 18; }
    .v { fill: var(--accent, #888); r: calc(var(--r) - 4); }
  </style>
  <circle class="v" cx="25" cy="50"/>
  <circle class="v" cx="55" cy="50"/>
  <circle class="v" cx="85" cy="50"/>
</svg>`),
				},
			}),
		},
	})
}

func svgSpreadSample(t gui.Theme, id, label, spread string) gui.View {
	data := `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="g" x1="0%" y1="50%" x2="33%" y2="50%" spreadMethod="` + spread + `">
      <stop offset="0%" stop-color="#3b82f6"/>
      <stop offset="100%" stop-color="#ec4899"/>
    </linearGradient>
  </defs>
  <rect width="100" height="100" fill="url(#g)"/>
</svg>`
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FitFit,
		Spacing:    gui.SomeF(4),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.B5}),
			gui.Svg(gui.SvgCfg{ID: id, Width: 100, Height: 100, Sizing: gui.FixedFixed, SvgData: data}),
		},
	})
}

func svgRadialSample(t gui.Theme, id, label, data string) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FitFit,
		Spacing:    gui.SomeF(4),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.B5}),
			gui.Svg(gui.SvgCfg{ID: id, Width: 100, Height: 100, Sizing: gui.FixedFixed, SvgData: data}),
		},
	})
}

func demoImage(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	imgPath := embeddedAssetPath("assets/image_clip_face.jpg")
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(24),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Sizing:     gui.FitFit,
						Spacing:    gui.SomeF(8),
						Padding:    gui.NoPadding,
						SizeBorder: gui.NoBorder,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Default", TextStyle: t.B4}),
							gui.Image(gui.ImageCfg{
								Src:    imgPath,
								Width:  120,
								Height: 120,
							}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Sizing:     gui.FitFit,
						Spacing:    gui.SomeF(8),
						Padding:    gui.NoPadding,
						SizeBorder: gui.NoBorder,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Rounded (radius: 10)", TextStyle: t.B4}),
							gui.Column(gui.ContainerCfg{
								Clip:       true,
								Radius:     gui.SomeF(10),
								Width:      120,
								Height:     120,
								Sizing:     gui.FixedFixed,
								Padding:    gui.NoPadding,
								SizeBorder: gui.NoBorder,
								Content: []gui.View{
									gui.Image(gui.ImageCfg{
										Src:    imgPath,
										Width:  120,
										Height: 120,
									}),
								},
							}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Sizing:     gui.FitFit,
						Spacing:    gui.SomeF(8),
						Padding:    gui.NoPadding,
						SizeBorder: gui.NoBorder,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Circle", TextStyle: t.B4}),
							gui.Circle(gui.ContainerCfg{
								Clip:       true,
								Width:      120,
								Height:     120,
								Sizing:     gui.FixedFixed,
								Padding:    gui.NoPadding,
								SizeBorder: gui.NoBorder,
								Content: []gui.View{
									gui.Image(gui.ImageCfg{
										Src:    imgPath,
										Width:  120,
										Height: 120,
									}),
								},
							}),
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "Embedded: assets/image_clip_face.jpg",
				TextStyle: t.N4,
			}),
		},
	})
}

func demoDrawCanvas(w *gui.Window) gui.View {
	chartData := []float32{2, 5, 3, 8, 6, 4, 7, 9, 5, 10, 8, 6, 11, 7}
	barData := []float32{40, 65, 50, 80, 55, 70}
	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SomeF(8),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: "Line chart with joined polyline, dashed grid," +
					" filled area, and text labels.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			demoDrawCanvasLineChart(chartData),
			gui.Text(gui.TextCfg{
				Text: "Bar chart with rounded-rect bars and" +
					" dashed reference line.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			demoDrawCanvasBarChart(barData),
			gui.Text(gui.TextCfg{
				Text: "Image tiles drawn via DrawContext.Image." +
					" Tab to focus, then arrow keys move the marker.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			demoDrawCanvasInteractive(app),
		},
	})
}

func demoDrawCanvasInteractive(app *ShowcaseApp) gui.View {
	tileSrc := embeddedAssetPath("assets/arch.png")
	const (
		canvasW float32 = 480
		canvasH float32 = 280
		tileH   float32 = 80
		tileGap float32 = 12
		markerR float32 = 10
		step    float32 = 12
	)

	return gui.DrawCanvas(gui.DrawCanvasCfg{
		ID:      "showcase-draw-canvas-interactive",
		Version: app.DrawCanvasVersion,
		Width:   canvasW,
		Height:  canvasH,
		Color:   gui.RGBA(30, 30, 40, 255),
		Radius:  8,
		Clip:    true,
		Padding: gui.Some(gui.Padding{
			Top: 20, Right: 20, Bottom: 20, Left: 20,
		}),
		Focusable:       true,
		A11YLabel:       "Interactive draw canvas",
		A11YDescription: "Arrow keys move the marker",
		OnDraw: func(dc *gui.DrawContext) {
			// Row of image tiles across the top.
			const tiles = 4
			tileW := (dc.Width - tileGap*float32(tiles-1)) /
				float32(tiles)
			for i := range tiles {
				x := float32(i) * (tileW + tileGap)
				dc.Image(x, 0, tileW, tileH, tileSrc,
					gui.Opt[float32]{}, gui.Color{})
			}

			// Crosshair guides for the marker.
			mx := app.DrawCanvasMarkerX
			my := app.DrawCanvasMarkerY
			guide := gui.RGBA(80, 80, 100, 255)
			dc.DashedLine(0, my, dc.Width, my, guide, 1, 6, 4)
			dc.DashedLine(mx, tileH+tileGap, mx, dc.Height,
				guide, 1, 6, 4)

			// Marker.
			dc.FilledCircle(mx, my, markerR,
				gui.RGBA(255, 200, 80, 255))
			dc.Circle(mx, my, markerR, gui.White, 1.5)

			// Hint.
			hint := gui.TextStyle{
				Size:  11,
				Color: gui.RGBA(220, 220, 255, 255),
			}
			dc.Text(6, dc.Height-14,
				"arrow keys move marker", hint)
		},
		OnKeyDown: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			app := gui.State[ShowcaseApp](w)
			dx, dy := float32(0), float32(0)
			switch e.KeyCode {
			case gui.KeyLeft:
				dx = -step
			case gui.KeyRight:
				dx = step
			case gui.KeyUp:
				dy = -step
			case gui.KeyDown:
				dy = step
			default:
				return
			}
			// Content area = canvas size - padding (20 each side).
			// Seed with finite defaults so a corrupt restore
			// (NaN/Inf) cannot lock the marker off-canvas.
			cx := safeFloat(app.DrawCanvasMarkerX, canvasW/2)
			cy := safeFloat(app.DrawCanvasMarkerY, canvasH/2)
			minY := tileH + tileGap + markerR
			app.DrawCanvasMarkerX = min(canvasW-40-markerR,
				max(markerR, cx+dx))
			app.DrawCanvasMarkerY = min(canvasH-40-markerR,
				max(minY, cy+dy))
			app.DrawCanvasVersion++
			e.IsHandled = true
		},
	})
}

func demoDrawCanvasLineChart(chartData []float32) gui.View {
	return gui.DrawCanvas(gui.DrawCanvasCfg{
		ID:      "showcase-draw-canvas-line",
		Version: 1,
		Width:   480,
		Height:  280,
		Color:   gui.RGBA(30, 30, 40, 255),
		Radius:  8,
		Padding: gui.Some(gui.Padding{
			Top: 24, Right: 24, Bottom: 24, Left: 24,
		}),
		OnDraw: func(dc *gui.DrawContext) {
			cw := dc.Width
			ch := dc.Height
			gridColor := gui.RGBA(80, 80, 100, 255)

			// Dashed horizontal grid.
			rows := 5
			for i := range rows + 1 {
				y := ch * float32(i) / float32(rows)
				dc.DashedLine(0, y, cw, y,
					gridColor, 1, 6, 4)
			}
			// Dashed vertical grid.
			cols := len(chartData) - 1
			for i := range cols + 1 {
				x := cw * float32(i) / float32(cols)
				dc.DashedLine(x, 0, x, ch,
					gridColor, 1, 6, 4)
			}

			// Data range.
			mn, mx := chartData[0], chartData[0]
			for _, v := range chartData {
				if v < mn {
					mn = v
				}
				if v > mx {
					mx = v
				}
			}
			span := mx - mn
			if span == 0 {
				span = 1
			}

			// Build polyline points.
			pts := make([]float32, 0, len(chartData)*2)
			for i, v := range chartData {
				x := cw * float32(i) / float32(len(chartData)-1)
				y := ch - ch*(v-mn)/span
				pts = append(pts, x, y)
			}

			// Filled area under curve.
			fillColor := gui.RGBA(70, 130, 220, 60)
			for i := 0; i+3 < len(pts); i += 2 {
				dc.FilledPolygon([]float32{
					pts[i], pts[i+1],
					pts[i+2], pts[i+3],
					pts[i+2], ch,
					pts[i], ch,
				}, fillColor)
			}

			// Joined polyline (miter joins at vertices).
			dc.PolylineJoined(pts,
				gui.RGBA(70, 130, 220, 255), 2.5)

			// Dot markers.
			for i := 0; i < len(pts); i += 2 {
				dc.FilledCircle(pts[i], pts[i+1], 4,
					gui.RGBA(220, 220, 255, 255))
			}

			// Text label at peak.
			peakIdx := 0
			for i, v := range chartData {
				if v > chartData[peakIdx] {
					peakIdx = i
				}
			}
			labelStyle := gui.TextStyle{
				Size:  11,
				Color: gui.RGBA(220, 220, 255, 255),
			}
			label := "peak"
			lw := dc.TextWidth(label, labelStyle)
			px := pts[peakIdx*2]
			py := pts[peakIdx*2+1]
			dc.Text(px-lw/2, py-18, label, labelStyle)
		},
	})
}

func demoDrawCanvasBarChart(barData []float32) gui.View {
	return gui.DrawCanvas(gui.DrawCanvasCfg{
		ID:      "showcase-draw-canvas-bar",
		Version: 1,
		Width:   480,
		Height:  220,
		Color:   gui.RGBA(30, 30, 40, 255),
		Radius:  8,
		Padding: gui.Some(gui.Padding{
			Top: 24, Right: 24, Bottom: 24, Left: 24,
		}),
		OnDraw: func(dc *gui.DrawContext) {
			cw := dc.Width
			ch := dc.Height

			// Find max for scaling.
			mx := barData[0]
			for _, v := range barData {
				if v > mx {
					mx = v
				}
			}
			if mx == 0 {
				mx = 1
			}

			n := len(barData)
			gap := cw * 0.04
			barW := (cw - gap*float32(n+1)) / float32(n)
			barColor := gui.RGBA(90, 180, 130, 255)

			// Bars with rounded corners.
			for i, v := range barData {
				x := gap + float32(i)*(barW+gap)
				h := ch * v / mx
				y := ch - h
				dc.FilledRoundedRect(x, y, barW, h,
					6, barColor)
			}

			// Dashed average reference line.
			avg := float32(0)
			for _, v := range barData {
				avg += v
			}
			avg /= float32(n)
			refY := ch - ch*avg/mx
			dc.DashedLine(0, refY, cw, refY,
				gui.RGBA(255, 200, 80, 200), 1.5, 8, 5)

			// "avg" label.
			labelStyle := gui.TextStyle{
				Size:  11,
				Color: gui.RGBA(255, 200, 80, 200),
			}
			dc.Text(4, refY-16, "avg", labelStyle)
		},
	})
}

func demoBlur(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SomeF(12),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "BlurRadius adds a Gaussian blur to a shape's fill. Higher values produce softer edges.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(40),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:      150,
						Height:     150,
						Sizing:     gui.FixedFixed,
						Radius:     gui.SomeF(75),
						Color:      gui.RGBA(0, 255, 0, 150),
						BlurRadius: 20,
						HAlign:     gui.HAlignCenter,
						VAlign:     gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Soft Orb", TextStyle: t.N2}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:      150,
						Height:     150,
						Sizing:     gui.FixedFixed,
						Radius:     gui.SomeF(20),
						Color:      gui.RGBA(255, 100, 100, 200),
						BlurRadius: 10,
						HAlign:     gui.HAlignCenter,
						VAlign:     gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Soft Rect", TextStyle: t.N2}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:      200,
						Height:     100,
						Sizing:     gui.FixedFixed,
						Radius:     gui.SomeF(10),
						Color:      gui.RGBA(60, 120, 255, 255),
						BlurRadius: 50,
						HAlign:     gui.HAlignCenter,
						VAlign:     gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Heavy Glow", TextStyle: t.N2}),
						},
					}),
				},
			}),
		},
	})
}

func demoShader(w *gui.Window) gui.View {
	// Keep frame loop hot so shader params animate continuously.
	// QueueCommand defers to next frame to avoid locking w.mu
	// (view functions run under that lock).
	w.QueueCommand(func(w *gui.Window) {
		if !w.HasAnimation("shader_tick") {
			w.AnimationAdd(&gui.Animate{
				AnimID:   "shader_tick",
				Repeat:   true,
				Callback: func(_ *gui.Animate, _ *gui.Window) {},
			})
		}
	})

	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)
	elapsed := float32(time.Since(app.ShaderStartTime).Milliseconds()) / 1000.0

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Custom fragment shaders (Metal + GLSL). Params[0] is animated time.",
				TextStyle: t.N3,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(20),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:  200,
						Height: 200,
						Sizing: gui.FixedFixed,
						Radius: gui.SomeF(16),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Shader: &gui.Shader{
							Metal: `
								float t = in.p0.x;
								float2 st = in.uv * 0.5 + 0.5;
								float3 c = 0.5 + 0.5 * cos(t + st.xyx + float3(0,2,4));
								float4 frag_color = float4(c, 1.0);
							`,
							GLSL: `
								float t = p0.x;
								vec2 st = uv * 0.5 + 0.5;
								vec3 c = 0.5 + 0.5 * cos(t + st.xyx + vec3(0,2,4));
								vec4 frag_color = vec4(c, 1.0);
							`,
							Params: []float32{elapsed},
						},
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Rainbow", TextStyle: t.N2})},
					}),
					gui.Column(gui.ContainerCfg{
						Width:  200,
						Height: 200,
						Sizing: gui.FixedFixed,
						Radius: gui.SomeF(16),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Shader: &gui.Shader{
							Metal: `
								float t = in.p0.x;
								float2 st = in.uv * 3.0;
								float v = sin(st.x + t) + sin(st.y + t)
									+ sin(st.x + st.y + t)
									+ sin(length(st) + 1.5 * t);
								v = v * 0.25 + 0.5;
								float3 c = float3(
									sin(v * 3.14159),
									sin(v * 3.14159 + 2.094),
									sin(v * 3.14159 + 4.188));
								c = c * 0.5 + 0.5;
								float4 frag_color = float4(c, 1.0);
							`,
							GLSL: `
								float t = p0.x;
								vec2 st = uv * 3.0;
								float v = sin(st.x + t) + sin(st.y + t)
									+ sin(st.x + st.y + t)
									+ sin(length(st) + 1.5 * t);
								v = v * 0.25 + 0.5;
								vec3 c = vec3(
									sin(v * 3.14159),
									sin(v * 3.14159 + 2.094),
									sin(v * 3.14159 + 4.188));
								c = c * 0.5 + 0.5;
								vec4 frag_color = vec4(c, 1.0);
							`,
							Params: []float32{elapsed},
						},
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Plasma", TextStyle: t.N2})},
					}),
				},
			}),
		},
	})
}

func demoColorFilter(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()

	// Pastel/mixed colors so filters produce a visible difference.
	colorContent := func(label string) []gui.View {
		return []gui.View{
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Height:     20,
				Color:      gui.RGBA(200, 130, 100, 255),
				SizeBorder: gui.NoBorder,
				Radius:     gui.SomeF(4),
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Height:     20,
				Color:      gui.RGBA(100, 180, 140, 255),
				SizeBorder: gui.NoBorder,
				Radius:     gui.SomeF(4),
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Height:     20,
				Color:      gui.RGBA(120, 130, 200, 255),
				SizeBorder: gui.NoBorder,
				Radius:     gui.SomeF(4),
			}),
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.N4}),
		}
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SomeF(16),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			// Photo filter strip.
			gui.Text(gui.TextCfg{
				Text: "Color matrix transforms applied as a post-processing pass. " +
					"Each container below renders content into an FBO, applies a 4×4 color transform, " +
					"and composites back.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(12),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:       120,
						Sizing:      gui.FixedFit,
						Padding:     gui.SomeP(8, 8, 8, 8),
						Radius:      gui.SomeF(6),
						Color:       t.ColorPanel,
						ColorFilter: gui.ColorFilterGrayscale(),
						Content:     colorContent("Grayscale"),
					}),
					gui.Column(gui.ContainerCfg{
						Width:       120,
						Sizing:      gui.FixedFit,
						Padding:     gui.SomeP(8, 8, 8, 8),
						Radius:      gui.SomeF(6),
						Color:       t.ColorPanel,
						ColorFilter: gui.ColorFilterSepia(),
						Content:     colorContent("Sepia"),
					}),
					gui.Column(gui.ContainerCfg{
						Width:       120,
						Sizing:      gui.FixedFit,
						Padding:     gui.SomeP(8, 8, 8, 8),
						Radius:      gui.SomeF(6),
						Color:       t.ColorPanel,
						ColorFilter: gui.ColorFilterContrast(1.5),
						Content:     colorContent("Contrast"),
					}),
					gui.Column(gui.ContainerCfg{
						Width:       120,
						Sizing:      gui.FixedFit,
						Padding:     gui.SomeP(8, 8, 8, 8),
						Radius:      gui.SomeF(6),
						Color:       t.ColorPanel,
						ColorFilter: gui.ColorFilterSaturate(2.0),
						Content:     colorContent("Saturate"),
					}),
				},
			}),

			// Blur + color filter combined.
			gui.Text(gui.TextCfg{
				Text:      "Blur + color filter: gaussian blur combined with color matrix",
				TextStyle: t.N3,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(12),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:   120,
						Sizing:  gui.FixedFit,
						Padding: gui.SomeP(8, 8, 8, 8),
						Radius:  gui.SomeF(6),
						Color:   t.ColorPanel,
						Content: colorContent("Original"),
					}),
					gui.Column(gui.ContainerCfg{
						Width:       120,
						Sizing:      gui.FixedFit,
						Padding:     gui.SomeP(8, 8, 8, 8),
						Radius:      gui.SomeF(6),
						Color:       t.ColorPanel,
						BlurRadius:  4,
						ColorFilter: gui.ColorFilterGrayscale(),
						Content:     colorContent("Blur+Gray"),
					}),
					gui.Column(gui.ContainerCfg{
						Width:       120,
						Sizing:      gui.FixedFit,
						Padding:     gui.SomeP(8, 8, 8, 8),
						Radius:      gui.SomeF(6),
						Color:       t.ColorPanel,
						BlurRadius:  4,
						ColorFilter: gui.ColorFilterSepia(),
						Content:     colorContent("Blur+Sepia"),
					}),
					gui.Column(gui.ContainerCfg{
						Width:       120,
						Sizing:      gui.FixedFit,
						Padding:     gui.SomeP(8, 8, 8, 8),
						Radius:      gui.SomeF(6),
						Color:       t.ColorPanel,
						BlurRadius:  4,
						ColorFilter: gui.ColorFilterHueRotate(90),
						Content:     colorContent("Blur+Hue90"),
					}),
				},
			}),

			// Bloom glow.
			gui.Text(gui.TextCfg{
				Text:      "Bloom glow: blur + brightness boost + multi-layer composite",
				TextStyle: t.N3,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(24),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:       100,
						Height:      100,
						Sizing:      gui.FixedFixed,
						Color:       gui.RGBA(0, 255, 128, 255),
						BlurRadius:  15,
						ColorFilter: gui.ColorFilterBrightness(1.3),
						Radius:      gui.SomeF(50),
						HAlign:      gui.HAlignCenter,
						VAlign:      gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Glow", TextStyle: t.N2}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:       100,
						Height:      100,
						Sizing:      gui.FixedFixed,
						Color:       gui.RGBA(255, 50, 200, 255),
						BlurRadius:  15,
						ColorFilter: gui.ColorFilterBrightness(1.3),
						Radius:      gui.SomeF(50),
						HAlign:      gui.HAlignCenter,
						VAlign:      gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Glow", TextStyle: t.N2}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Width:       100,
						Height:      100,
						Sizing:      gui.FixedFixed,
						Color:       gui.RGBA(50, 150, 255, 255),
						BlurRadius:  15,
						ColorFilter: gui.ColorFilterBrightness(1.3),
						Radius:      gui.SomeF(50),
						HAlign:      gui.HAlignCenter,
						VAlign:      gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Glow", TextStyle: t.N2}),
						},
					}),
				},
			}),
		},
	})
}
