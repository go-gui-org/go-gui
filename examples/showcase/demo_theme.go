package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

func demoThemeGen(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	strategies := []string{"mono", "complement", "analogous", "triadic", "warm", "cool"}
	pickerColor := app.ThemeGenSeed
	if app.ThemeGenPickText {
		pickerColor = app.ThemeGenText
	}

	pickText := app.ThemeGenPickText
	strategyViews := make([]gui.View, len(strategies))
	for i, strategy := range strategies {
		selected := app.ThemeGenStrategy == strategy
		color := t.ColorInterior
		textStyle := t.N3
		if selected {
			color = t.ColorActive
			textStyle.Color = gui.White
		}
		sv := strategy
		strategyViews[i] = gui.Button(gui.ButtonCfg{
			ID:       "strat-" + sv,
			Color:    color,
			Disabled: pickText,
			Padding:  gui.NewPadding(4, 10, 4, 10),
			Radius:   gui.SomeF(12),
			Content:  []gui.View{gui.Text(gui.TextCfg{Text: strategyLabel(sv), TextStyle: textStyle})},
			OnClick: func(ctx gui.EventCtx) {
				appState(ctx.Window).ThemeGenStrategy = sv
				applyGenTheme(ctx.Window)
			},
		})
	}

	title := "Pick a seed color to generate a full theme."
	if app.ThemeGenName != "" {
		title = app.ThemeGenName
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: title, TextStyle: t.N3}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(16),
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignTop,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Sizing:  gui.FitFit,
						Spacing: gui.SomeF(10),
						Padding: gui.NoPadding,
						Content: []gui.View{
							gui.ColorPicker(gui.ColorPickerCfg{
								ID:    "theme-gen-cp",
								Color: pickerColor,
								OnColorChange: func(c gui.Color, ctx gui.EventCtx) {
									app := appState(ctx.Window)
									if app.ThemeGenPickText {
										app.ThemeGenText = c
									} else {
										app.ThemeGenSeed = c
									}
									applyGenTheme(ctx.Window)
								},
							}),
							gui.Row(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SomeF(12),
								Padding: gui.NoPadding,
								Content: []gui.View{
									themeGenNumField(t, themeGenField{
										ID: "theme-gen-radius", Label: "Radius",
										Text: app.ThemeGenRadiusText, Value: app.ThemeGenRadius,
										Max: 30, Disabled: pickText,
										SetText:  func(a *ShowcaseApp, text string) { a.ThemeGenRadiusText = text },
										SetValue: func(a *ShowcaseApp, v float32) { a.ThemeGenRadius = v },
									}),
									themeGenNumField(t, themeGenField{
										ID: "theme-gen-border", Label: "Border",
										Text: app.ThemeGenBorderText, Value: app.ThemeGenBorder,
										Max: 10, Disabled: pickText,
										SetText:  func(a *ShowcaseApp, text string) { a.ThemeGenBorderText = text },
										SetValue: func(a *ShowcaseApp, v float32) { a.ThemeGenBorder = v },
									}),
									themeGenNumField(t, themeGenField{
										ID: "theme-gen-pad", Label: "Pad",
										Text: app.ThemeGenPadText, Value: app.ThemeGenPad,
										Max: 40, Disabled: pickText,
										SetText:  func(a *ShowcaseApp, text string) { a.ThemeGenPadText = text },
										SetValue: func(a *ShowcaseApp, v float32) { a.ThemeGenPad = v },
									}),
								},
							}),
							// Scrollbar geometry gets its own row: it is a
							// separate decision from the shape knobs above,
							// and five fields on one line overflow the column.
							gui.Row(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SomeF(12),
								Padding: gui.NoPadding,
								Content: []gui.View{
									themeGenNumField(t, themeGenField{
										ID: "theme-gen-scrollbar", Label: "Scrollbar",
										Text: app.ThemeGenScrollbarText, Value: app.ThemeGenScrollbar,
										Max: 24, Disabled: pickText,
										SetText:  func(a *ShowcaseApp, text string) { a.ThemeGenScrollbarText = text },
										SetValue: func(a *ShowcaseApp, v float32) { a.ThemeGenScrollbar = v },
									}),
									themeGenNumField(t, themeGenField{
										ID: "theme-gen-scroll-gap", Label: "Offset",
										Text: app.ThemeGenScrollGapText, Value: app.ThemeGenScrollGap,
										Max: 12, Disabled: pickText,
										SetText:  func(a *ShowcaseApp, text string) { a.ThemeGenScrollGapText = text },
										SetValue: func(a *ShowcaseApp, v float32) { a.ThemeGenScrollGap = v },
									}),
								},
							}),
						},
					}),
					gui.Column(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Spacing: gui.SomeF(10),
						Padding: gui.NoPadding,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Palette", TextStyle: t.B3}),
							gui.Wrap(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SomeF(4),
								Padding: gui.NoPadding,
								Content: strategyViews,
							}),
							gui.Toggle(gui.ToggleCfg{
								ID:       "theme-gen-pick-text",
								Label:    "Edit text color",
								Selected: app.ThemeGenPickText,
								OnClick: func(ctx gui.EventCtx) {
									appState(ctx.Window).ThemeGenPickText = !appState(ctx.Window).ThemeGenPickText
								},
							}),
							gui.Row(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SomeF(8),
								Padding: gui.NoPadding,
								Content: []gui.View{
									gui.Button(gui.ButtonCfg{
										ID:      "btn-reset-dark",
										Padding: gui.NewPadding(6, 12, 6, 12),
										Content: []gui.View{gui.Text(gui.TextCfg{Text: "Reset Dark", TextStyle: t.N3})},
										OnClick: func(ctx gui.EventCtx) {
											ctx.Window.SetTheme(gui.ThemeDark)
											syncThemeGenFromCfg(appState(ctx.Window), gui.ThemeDark.Cfg)
										},
									}),
									gui.Button(gui.ButtonCfg{
										ID:      "btn-reset-light",
										Padding: gui.NewPadding(6, 12, 6, 12),
										Content: []gui.View{gui.Text(gui.TextCfg{Text: "Reset Light", TextStyle: t.N3})},
										OnClick: func(ctx gui.EventCtx) {
											ctx.Window.SetTheme(gui.ThemeLight)
											syncThemeGenFromCfg(appState(ctx.Window), gui.ThemeLight.Cfg)
										},
									}),
								},
							}),
							gui.Row(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SomeF(8),
								Padding: gui.NoPadding,
								Content: []gui.View{
									gui.Button(gui.ButtonCfg{
										ID:      "btn-theme-save",
										Padding: gui.NewPadding(6, 12, 6, 12),
										Content: []gui.View{gui.Text(gui.TextCfg{Text: "Save Theme", TextStyle: t.N3})},
										OnClick: func(ctx gui.EventCtx) {
											ctx.Window.NativeSaveDialog(gui.NativeSaveDialogCfg{
												Title:            "Save Theme",
												DefaultName:      "theme.json",
												DefaultExtension: "json",
												Filters: []gui.NativeFileFilter{
													{Name: "JSON", Extensions: []string{"json"}},
												},
												ConfirmOverwrite: true,
												OnDone: func(result gui.NativeDialogResult, w *gui.Window) {
													if result.Status != gui.DialogOK || len(result.Paths) == 0 {
														return
													}
													app := appState(w)
													cfg := generateThemeCfg(
														app.ThemeGenSeed,
														app.ThemeGenStrategy,
														gui.CurrentTheme().TitlebarDark,
														app.ThemeGenTint,
														app.ThemeGenText,
														themeGenSizesOf(app),
													)
													path := result.Paths[0].Path
													if err := themeCfgSave(path, cfg); err != nil {
														app.ThemeGenName = err.Error()
														return
													}
													app.ThemeGenName = filepath.Base(path)
												},
											})
										},
									}),
									gui.Button(gui.ButtonCfg{
										ID:      "btn-theme-load",
										Padding: gui.NewPadding(6, 12, 6, 12),
										Content: []gui.View{gui.Text(gui.TextCfg{Text: "Load Theme", TextStyle: t.N3})},
										OnClick: func(ctx gui.EventCtx) {
											ctx.Window.NativeOpenDialog(gui.NativeOpenDialogCfg{
												Title: "Load Theme",
												Filters: []gui.NativeFileFilter{
													{Name: "JSON", Extensions: []string{"json"}},
												},
												OnDone: func(result gui.NativeDialogResult, w *gui.Window) {
													if result.Status != gui.DialogOK || len(result.Paths) == 0 {
														return
													}
													path := result.Paths[0].Path
													cfg, err := themeCfgLoad(path)
													if err != nil {
														appState(w).ThemeGenName = err.Error()
														return
													}
													w.SetTheme(gui.ThemeMaker(cfg))
													app := appState(w)
													syncThemeGenFromCfg(app, cfg)
													app.ThemeGenName = filepath.Base(path)
												},
											})
										},
									}),
								},
							}),
							gui.Text(gui.TextCfg{
								Text:      fmt.Sprintf("Tint: %.0f%%", app.ThemeGenTint),
								TextStyle: t.N3,
							}),
							gui.Slider(gui.SliderCfg{
								ID:       "theme-gen-tint",
								Disabled: pickText,
								Value:    app.ThemeGenTint,
								Min:      0,
								Max:      100,
								Width:    140,
								Sizing:   gui.FixedFit,
								OnChange: func(v float32, ctx gui.EventCtx) {
									appState(ctx.Window).ThemeGenTint = v
									applyGenTheme(ctx.Window)
								},
							}),
						},
					}),
				},
			}),
			themeContrastPreview(),
		},
	})
}

// themeGenField describes one numeric knob on the theme maker page.
// The five knobs differ only in label, ID, range and which state
// fields they write, so they share one builder instead of repeating
// the NumericInput block five times.
type themeGenField struct {
	SetText  func(app *ShowcaseApp, text string)
	SetValue func(app *ShowcaseApp, v float32)
	ID       string
	Label    string
	Text     string
	Value    float32
	Max      float64
	Disabled bool
}

// themeGenNumField builds one labelled numeric knob. SetText runs on
// every keystroke so a half-typed value survives the frame; SetValue
// plus the theme rebuild run only once the value parses.
func themeGenNumField(t gui.Theme, f themeGenField) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FitFit,
		Spacing: gui.SomeF(6),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: f.Label, TextStyle: t.N3}),
			gui.NumericInput(gui.NumericInputCfg{
				ID:       f.ID,
				Disabled: f.Disabled,
				Text:     f.Text,
				Value:    gui.Some(float64(f.Value)),
				Decimals: 1,
				Min:      gui.Some(0.0),
				Max:      gui.Some(f.Max),
				Width:    80,
				Sizing:   gui.FixedFit,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					f.SetText(appState(ctx.Window), text)
				},
				OnValueCommit: func(value gui.Opt[float64], text string, ctx gui.EventCtx) {
					app := appState(ctx.Window)
					f.SetText(app, text)
					if v, ok := value.Value(); ok {
						f.SetValue(app, float32(v))
						applyGenTheme(ctx.Window)
					}
				},
			}),
		},
	})
}

// themeGenDefaultScrollGap is the scrollbar edge inset a theme that
// leaves ThemeCfg.SizeScrollbarGap unset renders with. Spelled here
// because the library constant behind it is unexported.
const themeGenDefaultScrollGap float32 = 3

// themeGenSizes carries the theme maker's numeric knobs. A struct
// rather than five trailing float32 parameters, which no call site
// could read.
type themeGenSizes struct {
	Radius    float32
	Border    float32
	Pad       float32
	Scrollbar float32
	ScrollGap float32
}

// themeGenSizesOf snapshots the knobs out of app state.
func themeGenSizesOf(app *ShowcaseApp) themeGenSizes {
	return themeGenSizes{
		Radius:    app.ThemeGenRadius,
		Border:    app.ThemeGenBorder,
		Pad:       app.ThemeGenPad,
		Scrollbar: app.ThemeGenScrollbar,
		ScrollGap: app.ThemeGenScrollGap,
	}
}

// themeContrastPreview renders the same widgets under the light preset
// while the rest of the window keeps its own theme. gui.Themed scopes a
// theme to one subtree; its builder runs at layout-generation time,
// which is what lets the widgets inside resolve their defaults from the
// scoped theme rather than the window's.
func themeContrastPreview() gui.View {
	light, ok := gui.ThemeGet("light")
	if !ok {
		return nil
	}
	return gui.Themed(light, func(w *gui.Window) gui.View {
		lt := gui.CurrentTheme()
		return gui.Column(gui.ContainerCfg{
			ID:      "theme-preview",
			Sizing:  gui.FillFit,
			Spacing: gui.SomeF(8),
			Padding: gui.PadAll(12),
			Color:   lt.ColorPanel,
			Radius:  gui.SomeF(8),
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      "Scoped theme (light) — window theme unchanged",
					TextStyle: lt.N3,
				}),
				gui.Row(gui.ContainerCfg{
					Sizing:  gui.FillFit,
					Spacing: gui.SomeF(8),
					Padding: gui.NoPadding,
					Content: []gui.View{
						gui.Button(gui.ButtonCfg{
							ID:      "theme-preview-btn",
							Content: []gui.View{gui.Text(gui.TextCfg{Text: "Button"})},
						}),
						gui.Switch(gui.SwitchCfg{ID: "theme-preview-switch"}),
						gui.Slider(gui.SliderCfg{
							ID:     "theme-preview-slider",
							Value:  40,
							Width:  120,
							Sizing: gui.FixedFit,
						}),
					},
				}),
			},
		})
	})
}

func strategyLabel(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func syncThemeGenFromCfg(app *ShowcaseApp, cfg gui.ThemeCfg) {
	app.ThemeGenSeed = cfg.ColorSelect
	app.ThemeGenTint = 0
	app.ThemeGenStrategy = "mono"
	app.ThemeGenRadius = cfg.Radius
	app.ThemeGenRadiusText = floatString(cfg.Radius)
	app.ThemeGenBorder = cfg.SizeBorder
	app.ThemeGenBorderText = floatString(cfg.SizeBorder)
	app.ThemeGenPad = cfg.Padding.Top
	app.ThemeGenPadText = floatString(cfg.Padding.Top)
	app.ThemeGenScrollbar = cfg.SizeScrollbar
	app.ThemeGenScrollbarText = floatString(cfg.SizeScrollbar)
	gap := cfg.SizeScrollbarGap.Get(themeGenDefaultScrollGap)
	app.ThemeGenScrollGap = gap
	app.ThemeGenScrollGapText = floatString(gap)
	app.ThemeGenText = cfg.TextStyleDef.Color
	app.ThemeGenPickText = false
}

func applyGenTheme(w *gui.Window) {
	app := appState(w)
	cfg := generateThemeCfg(
		app.ThemeGenSeed,
		app.ThemeGenStrategy,
		gui.CurrentTheme().TitlebarDark,
		app.ThemeGenTint,
		app.ThemeGenText,
		themeGenSizesOf(app),
	)
	w.SetTheme(gui.ThemeMaker(cfg))
}

func generateThemeCfg(
	seed gui.Color,
	strategy string,
	isDark bool,
	tint float32,
	textColor gui.Color,
	sizes themeGenSizes,
) gui.ThemeCfg {
	h, s, _ := seed.ToHSV()
	tintFactor := tint / 100.0

	ph := h
	ah := h
	accentS := max(min(s, 1.0), 0.5)
	accentV := float32(0.85)
	if !isDark {
		accentV = 0.65
	}

	switch strategy {
	case "complement":
		ah = wrapHue(h + 180)
	case "analogous":
		ah = wrapHue(h + 30)
	case "triadic":
		ah = wrapHue(h + 120)
	case "warm":
		ph = float32(math.Mod(float64(h), 60))
		ah = ph + 15
	case "cool":
		ph = 180 + float32(math.Mod(float64(h), 90))
		ah = ph + 20
	}

	var cfg gui.ThemeCfg
	if isDark {
		cfg = gui.ThemeDark.Cfg
		sTint := max(min(s, 1.0), 0.3) * tintFactor
		cfg.ColorBackground = gui.ColorFromHSV(ph, sTint, 0.19)
		cfg.ColorPanel = gui.ColorFromHSV(ph, sTint, 0.25)
		cfg.ColorInterior = gui.ColorFromHSV(ph, sTint, 0.29)
		cfg.ColorHover = gui.ColorFromHSV(ph, sTint, 0.33)
		cfg.ColorFocus = gui.ColorFromHSV(ah, sTint, 0.37)
		cfg.ColorActive = gui.ColorFromHSV(ah, sTint, 0.41)
		cfg.ColorBorder = gui.ColorFromHSV(ah, sTint*0.8, 0.39)
		cfg.ColorSelect = gui.ColorFromHSV(ah, accentS, accentV)
		cfg.ColorBorderFocus = gui.ColorFromHSV(ah, accentS*0.7, accentV*0.9)
		cfg.TextStyleDef.Color = textColor
	} else {
		cfg = gui.ThemeLight.Cfg
		sTint := max(min(s, 1.0), 0.3) * tintFactor * 0.5
		cfg.ColorBackground = gui.ColorFromHSV(ph, sTint*0.6, 0.96)
		cfg.ColorPanel = gui.ColorFromHSV(ph, sTint, 0.90)
		cfg.ColorInterior = gui.ColorFromHSV(ph, sTint, 0.86)
		cfg.ColorHover = gui.ColorFromHSV(ph, sTint, 0.82)
		cfg.ColorFocus = gui.ColorFromHSV(ah, sTint, 0.78)
		cfg.ColorActive = gui.ColorFromHSV(ah, sTint, 0.74)
		cfg.ColorBorder = gui.ColorFromHSV(ah, sTint*1.5, 0.55)
		cfg.ColorSelect = gui.ColorFromHSV(ah, accentS, accentV*0.75)
		cfg.ColorBorderFocus = gui.ColorFromHSV(ah, accentS*0.8, accentV*0.6)
		cfg.TextStyleDef.Color = textColor
	}
	cfg.Name = "generated"
	cfg.SizeBorder = sizes.Border
	cfg.Radius = sizes.Radius
	cfg.RadiusSmall = sizes.Radius * 0.64
	cfg.RadiusMedium = sizes.Radius
	cfg.RadiusLarge = sizes.Radius * 1.36
	// One Pad value drives the whole ladder, the same way one Radius
	// value drives the radius ladder above. PaddingField stays alone:
	// it is the text inset that sets form-control height, so scaling
	// it with container padding would break row alignment.
	cfg.Padding = gui.PadAll(sizes.Pad)
	cfg.PaddingSmall = gui.PadAll(sizes.Pad * 0.5)
	cfg.PaddingMedium = gui.PadAll(sizes.Pad)
	cfg.PaddingLarge = gui.PadAll(sizes.Pad * 1.5)
	cfg.SizeScrollbar = sizes.Scrollbar
	// Some, not the plain value: zero offset is a bar flush against
	// the edge, which must not read as "unset".
	cfg.SizeScrollbarGap = gui.SomeF(sizes.ScrollGap)
	return cfg
}

func wrapHue(h float32) float32 {
	for h >= 360 {
		h -= 360
	}
	for h < 0 {
		h += 360
	}
	return h
}
