package main

import (
	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/highlight"
)

func demoText(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	wrapSample := "Wrap mode collapses repeated spaces and wraps words to fit the available width."
	keepSpacesSample := "wrap_keep_spaces keeps    repeated spaces.\nColumns:\nName\tRole\nAlex\tDesigner\nRiley\tEngineer"
	emojiSample := "Emoji: 😀🚀🎉👍🏽👩‍💻🧑‍🚀"
	graphemeSample := "Multi-grapheme: 👨‍👩‍👧‍👦🇺🇸1️⃣ café"
	i18nSample := "i18n: English | Español | العربية | हिन्दी | 日本語 | 한국어 | עברית | ไทย"

	content := []gui.View{
		gui.Text(gui.TextCfg{
			ID:        "text-intro",
			Text:      "Text supports style variants, alignment, wrapping modes, tabs, and selection/copy.",
			TextStyle: t.N5,
			Mode:      gui.TextModeWrap,
		}),
		gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.Some(t.SpacingMedium),
			Padding: gui.NoPadding,
			VAlign:  gui.VAlignMiddle,
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: "Theme n3 text", TextStyle: t.N3}),
				gui.Text(gui.TextCfg{Text: "Theme b3 text", TextStyle: t.B3}),
				gui.Text(gui.TextCfg{Text: "Theme i3 text", TextStyle: t.I3}),
				gui.Text(gui.TextCfg{Text: "Theme m3 text", TextStyle: t.M3}),
			},
		}),
		gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.Some(t.SpacingMedium),
			Padding: gui.NoPadding,
			VAlign:  gui.VAlignMiddle,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text: "Underlined",
					TextStyle: gui.TextStyle{
						Color:     t.N4.Color,
						Size:      t.N4.Size,
						Underline: true,
					},
				}),
				gui.Text(gui.TextCfg{
					Text: "Strikethrough",
					TextStyle: gui.TextStyle{
						Color:         t.N4.Color,
						Size:          t.N4.Size,
						Strikethrough: true,
					},
				}),
				gui.Text(gui.TextCfg{
					Text: "Background color",
					TextStyle: gui.TextStyle{
						Color:   gui.White,
						Size:    t.N4.Size,
						BgColor: gui.RGB(27, 54, 93),
					},
				}),
			},
		}),
		textDemoCard("", "Emoji, Multi-grapheme, and i18n", 0, []gui.View{
			gui.Text(gui.TextCfg{Text: emojiSample, TextStyle: t.N4, Mode: gui.TextModeWrap}),
			gui.Text(gui.TextCfg{Text: graphemeSample, TextStyle: t.N4, Mode: gui.TextModeWrap}),
			gui.Text(gui.TextCfg{Text: i18nSample, TextStyle: t.N4, Mode: gui.TextModeWrap}),
			gui.Text(gui.TextCfg{
				Text:   "RTL sample: العربية עברית",
				Mode:   gui.TextModeWrap,
				Sizing: gui.FillFit,
				TextStyle: gui.TextStyle{
					Color: t.N4.Color,
					Size:  t.N4.Size,
					Align: gui.TextAlignRight,
				},
			}),
		}),
		gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.Some(t.SpacingMedium),
			Padding: gui.NoPadding,
			VAlign:  gui.VAlignTop,
			Content: []gui.View{
				textDemoCard("", "mode: .wrap + alignment", 260, []gui.View{
					gui.Text(gui.TextCfg{
						ID:        "text-wrap-sample",
						Text:      wrapSample,
						Mode:      gui.TextModeWrap,
						Sizing:    gui.FillFit,
						TextStyle: gui.TextStyle{Color: t.N5.Color, Size: t.N5.Size, Align: gui.TextAlignLeft},
					}),
					gui.Text(gui.TextCfg{
						Text:      "Center aligned text",
						Mode:      gui.TextModeWrap,
						Sizing:    gui.FillFit,
						TextStyle: gui.TextStyle{Color: t.N5.Color, Size: t.N5.Size, Align: gui.TextAlignCenter},
					}),
					gui.Text(gui.TextCfg{
						Text:      "Right aligned text",
						Mode:      gui.TextModeWrap,
						Sizing:    gui.FillFit,
						TextStyle: gui.TextStyle{Color: t.N5.Color, Size: t.N5.Size, Align: gui.TextAlignRight},
					}),
				}),
				textDemoCard("", "mode: .wrap_keep_spaces", 260, []gui.View{
					gui.Text(gui.TextCfg{
						ID:        "text-wrap-keep-spaces",
						Text:      keepSpacesSample,
						Mode:      gui.TextModeWrapKeepSpaces,
						Sizing:    gui.FillFit,
						TabSize:   8,
						TextStyle: t.M5,
					}),
				}),
			},
		}),
		textDemoCard("", "", 0, []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Focus/select/copy: click inside block, drag selection, then Cmd/Ctrl+C.",
				TextStyle: t.N5,
				Mode:      gui.TextModeWrap,
			}),
			gui.Text(gui.TextCfg{
				ID:        "text-selectable-block",
				Focusable: true,
				FocusSkip: false,
				Mode:      gui.TextModeMultiline,
				Sizing:    gui.FillFit,
				Text:      "Selectable text block\n- Click to focus\n- Drag to select range\n- Copy with Cmd/Ctrl+C",
				TextStyle: gui.TextStyle{
					Color:   t.N4.Color,
					Size:    t.N4.Size,
					BgColor: t.ColorPanel,
				},
			}),
		}),
	}

	content = append(content,
		textDemoCard("", "Transforms", 0, []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFixed,
				Height:  t.B4.Size * 4,
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignTop,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						ID:   "text-transform-rotation",
						Text: "Rotated text via TextStyle.RotationRadians",
						TextStyle: gui.TextStyle{
							Color:           t.B4.Color,
							Size:            t.B4.Size,
							Typeface:        t.B4.Typeface,
							RotationRadians: 0.35,
						},
					}),
				},
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFixed,
				Height:  t.B4.Size * 4,
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignTop,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						ID:   "text-transform-affine",
						Text: "Affine text: skew + translate",
						TextStyle: gui.TextStyle{
							Color:    t.B4.Color,
							Size:     t.B4.Size,
							Typeface: t.B4.Typeface,
							AffineTransform: &glyph.AffineTransform{
								XX: 1.0,
								XY: -0.35,
								YX: 0.15,
								YY: 1.0,
								X0: 24,
								Y0: 0,
							},
						},
					}),
				},
			}),
		}),
		textDemoCard("", "Gradient Text", 0, []gui.View{
			gui.Text(gui.TextCfg{
				ID:   "text-gradient-horizontal",
				Text: "Horizontal Rainbow Gradient",
				Mode: gui.TextModeWrap,
				TextStyle: gui.TextStyle{
					Color:    t.B2.Color,
					Size:     t.B2.Size,
					Typeface: t.B2.Typeface,
					Gradient: &glyph.GradientConfig{
						Direction: glyph.GradientHorizontal,
						Stops: []glyph.GradientStop{
							{Color: glyph.Color{R: 255, G: 0, B: 0, A: 255}, Position: 0.0},
							{Color: glyph.Color{R: 255, G: 200, B: 0, A: 255}, Position: 0.33},
							{Color: glyph.Color{R: 0, G: 180, B: 255, A: 255}, Position: 0.66},
							{Color: glyph.Color{R: 180, G: 0, B: 255, A: 255}, Position: 1.0},
						},
					},
				},
			}),
			gui.Text(gui.TextCfg{
				ID:   "text-gradient-vertical",
				Text: "Vertical Sunset Gradient",
				Mode: gui.TextModeWrap,
				TextStyle: gui.TextStyle{
					Color:    t.B2.Color,
					Size:     t.B2.Size,
					Typeface: t.B2.Typeface,
					Gradient: &glyph.GradientConfig{
						Direction: glyph.GradientVertical,
						Stops: []glyph.GradientStop{
							{Color: glyph.Color{R: 255, G: 100, B: 100, A: 255}, Position: 0.0},
							{Color: glyph.Color{R: 255, G: 200, B: 80, A: 255}, Position: 0.5},
							{Color: glyph.Color{R: 180, G: 80, B: 200, A: 255}, Position: 1.0},
						},
					},
				},
			}),
		}),
		textDemoCard("", "Outlined & Hollow Text", 0, []gui.View{
			gui.Text(gui.TextCfg{
				Text: "Outlined text (fill + stroke)",
				Mode: gui.TextModeWrap,
				TextStyle: gui.TextStyle{
					Color:       t.B2.Color,
					Size:        t.B2.Size,
					Typeface:    t.B2.Typeface,
					StrokeWidth: 1.5,
					StrokeColor: gui.Red,
				},
			}),
			gui.Text(gui.TextCfg{
				Text: "Hollow text (stroke only)",
				Mode: gui.TextModeWrap,
				TextStyle: gui.TextStyle{
					Color:       gui.ColorTransparent,
					Size:        t.B2.Size,
					Typeface:    t.B2.Typeface,
					StrokeWidth: 1.5,
					StrokeColor: t.TextStyleDef.Color,
				},
			}),
		}),
		textDemoCard("", "Curved Text (SVG textPath)", 0, []gui.View{
			gui.Svg(gui.SvgCfg{
				ID: "text-curved-svg",
				SvgData: `<svg viewBox="0 0 500 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <path id="curve" d="M20,80 Q250,10 480,80" fill="none"/>
  </defs>
  <text font-size="18" fill="#3399cc" font-weight="600">
    <textPath href="#curve" startOffset="50%" text-anchor="middle">Text flowing along a curved path</textPath>
  </text>
</svg>`,
				Width:  500,
				Height: 100,
			}),
		}),
	)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.Some(t.SpacingSmall),
		Padding: gui.NoPadding,
		Content: content,
	})
}

func textDemoCard(
	id, title string,
	width float32,
	content []gui.View,
) gui.View {
	t := gui.CurrentTheme()
	items := content
	if title != "" {
		items = append([]gui.View{
			gui.Text(gui.TextCfg{Text: title, TextStyle: t.B5}),
		}, content...)
	}
	cfg := gui.ContainerCfg{
		ID:          id,
		Sizing:      gui.FillFit,
		Color:       t.ColorPanel,
		ColorBorder: t.ColorBorder,
		SizeBorder:  gui.SomeF(1),
		Padding:     gui.Some(t.PaddingSmall),
		Spacing:     gui.Some(t.SpacingSmall),
		Content:     items,
	}
	if width > 0 {
		cfg.Width = width
		cfg.Sizing = gui.FixedFit
	}
	return gui.Column(cfg)
}

func demoRtf(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(16),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Click any block to focus it, drag to select, Cmd/Ctrl+C to copy.",
				TextStyle: t.N5,
				Mode:      gui.TextModeWrap,
			}),

			// Mixed inline styles in a single paragraph
			sectionLabel(t, "Mixed Inline Styles"),
			gui.RTF(gui.RtfCfg{
				Focusable: true,
				Mode:      gui.TextModeWrap,
				RichText: gui.RichText{
					Runs: []gui.RichTextRun{
						gui.RichRun("Rich text supports ", t.N3),
						gui.RichRun("bold", t.B3),
						gui.RichRun(", ", t.N3),
						gui.RichRun("italic", t.I3),
						gui.RichRun(", ", t.N3),
						gui.RichRun("monospace", t.M3),
						gui.RichRun(", and ", t.N3),
						gui.RichRun("colored", gui.TextStyle{
							Color: gui.ColorFromString("#3b82f6"),
							Size:  t.N3.Size,
						}),
						gui.RichRun(" text in a single paragraph.", t.N3),
					},
				},
			}),

			line(),

			// Underline and strikethrough
			sectionLabel(t, "Decorations"),
			gui.RTF(gui.RtfCfg{
				Focusable: true,
				Mode:      gui.TextModeWrap,
				RichText: gui.RichText{
					Runs: []gui.RichTextRun{
						gui.RichRun("Underlined", gui.TextStyle{
							Color: t.N3.Color, Size: t.N3.Size,
							Underline: true,
						}),
						gui.RichRun(" and ", t.N3),
						gui.RichRun("strikethrough", gui.TextStyle{
							Color: t.N3.Color, Size: t.N3.Size,
							Strikethrough: true,
						}),
						gui.RichRun(" within a single text block.", t.N3),
					},
				},
			}),

			line(),

			// Clickable link
			sectionLabel(t, "Links & Abbreviations"),
			gui.RTF(gui.RtfCfg{
				Focusable: true,
				Mode:      gui.TextModeWrap,
				RichText: gui.RichText{
					Runs: []gui.RichTextRun{
						gui.RichRun("Visit the ", t.N3),
						gui.RichLink("Go-Gui repository", "https://github.com/go-gui-org/go-gui", gui.TextStyle{
							Color:     gui.ColorFromString("#3b82f6"),
							Size:      t.N3.Size,
							Underline: true,
						}),
						gui.RichRun(" for more info. ", t.N3),
						gui.RichAbbr("RTF", "Rich Text Format", t.B3),
						gui.RichRun(" stands for Rich Text Format.", t.N3),
					},
				},
			}),

			line(),

			// Multi-line with breaks
			sectionLabel(t, "Line Breaks"),
			gui.RTF(gui.RtfCfg{
				Focusable: true,
				Mode:      gui.TextModeWrap,
				RichText: gui.RichText{
					Runs: []gui.RichTextRun{
						gui.RichRun("First line of text.", t.N3),
						gui.RichBr(),
						gui.RichRun("Second line after a break.", t.N3),
						gui.RichBr(),
						gui.RichRun("Third line with ", t.N3),
						gui.RichRun("mixed styles", t.B3),
						gui.RichRun(".", t.N3),
					},
				},
			}),
		},
	})
}

func demoMarkdown(w *gui.Window) gui.View {
	style := gui.DefaultMarkdownStyle()
	style.CodeHighlighter = highlight.Default()
	return w.Markdown(gui.MarkdownCfg{
		Focusable: true,
		Style:     style,
		Source:    embeddedText("docs/markdown_demo.md"),
	})
}

func sectionLabel(t gui.Theme, text string) gui.View {
	return gui.Text(gui.TextCfg{Text: text, TextStyle: t.B3})
}
