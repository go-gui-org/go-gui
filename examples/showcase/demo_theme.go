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
	app := gui.State[ShowcaseApp](w)
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
			Padding:  gui.SomeP(4, 10, 4, 10),
			Radius:   gui.SomeF(12),
			Content:  []gui.View{gui.Text(gui.TextCfg{Text: strategyLabel(sv), TextStyle: textStyle})},
			OnClick: func(ctx gui.EventCtx) {
				gui.State[ShowcaseApp](ctx.Window).ThemeGenStrategy = sv
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
									app := gui.State[ShowcaseApp](ctx.Window)
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
									gui.Column(gui.ContainerCfg{
										Sizing:  gui.FitFit,
										Spacing: gui.SomeF(6),
										Padding: gui.NoPadding,
										Content: []gui.View{
											gui.Text(gui.TextCfg{Text: "Radius", TextStyle: t.N3}),
											gui.NumericInput(gui.NumericInputCfg{
												ID:       "theme-gen-radius",
												Disabled: pickText,
												Text:     app.ThemeGenRadiusText,
												Value:    gui.Some(float64(app.ThemeGenRadius)),
												Decimals: 1,
												Min:      gui.Some(0.0),
												Max:      gui.Some(30.0),
												Width:    80,
												Sizing:   gui.FixedFit,
												OnTextChanged: func(text string, ctx gui.EventCtx) {
													gui.State[ShowcaseApp](ctx.Window).ThemeGenRadiusText = text
												},
												OnValueCommit: func(value gui.Opt[float64], text string, ctx gui.EventCtx) {
													app := gui.State[ShowcaseApp](ctx.Window)
													app.ThemeGenRadiusText = text
													if v, ok := value.Value(); ok {
														app.ThemeGenRadius = float32(v)
														applyGenTheme(ctx.Window)
													}
												},
											}),
										},
									}),
									gui.Column(gui.ContainerCfg{
										Sizing:  gui.FitFit,
										Spacing: gui.SomeF(6),
										Padding: gui.NoPadding,
										Content: []gui.View{
											gui.Text(gui.TextCfg{Text: "Border", TextStyle: t.N3}),
											gui.NumericInput(gui.NumericInputCfg{
												ID:       "theme-gen-border",
												Disabled: pickText,
												Text:     app.ThemeGenBorderText,
												Value:    gui.Some(float64(app.ThemeGenBorder)),
												Decimals: 1,
												Min:      gui.Some(0.0),
												Max:      gui.Some(10.0),
												Width:    80,
												Sizing:   gui.FixedFit,
												OnTextChanged: func(text string, ctx gui.EventCtx) {
													gui.State[ShowcaseApp](ctx.Window).ThemeGenBorderText = text
												},
												OnValueCommit: func(value gui.Opt[float64], text string, ctx gui.EventCtx) {
													app := gui.State[ShowcaseApp](ctx.Window)
													app.ThemeGenBorderText = text
													if v, ok := value.Value(); ok {
														app.ThemeGenBorder = float32(v)
														applyGenTheme(ctx.Window)
													}
												},
											}),
										},
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
									gui.State[ShowcaseApp](ctx.Window).ThemeGenPickText = !gui.State[ShowcaseApp](ctx.Window).ThemeGenPickText
								},
							}),
							gui.Row(gui.ContainerCfg{
								Sizing:  gui.FillFit,
								Spacing: gui.SomeF(8),
								Padding: gui.NoPadding,
								Content: []gui.View{
									gui.Button(gui.ButtonCfg{
										ID:      "btn-reset-dark",
										Padding: gui.SomeP(6, 12, 6, 12),
										Content: []gui.View{gui.Text(gui.TextCfg{Text: "Reset Dark", TextStyle: t.N3})},
										OnClick: func(ctx gui.EventCtx) {
											ctx.Window.SetTheme(gui.ThemeDark.WithBorders(true))
											syncThemeGenFromCfg(gui.State[ShowcaseApp](ctx.Window), gui.ThemeDark.WithBorders(true).Cfg)
										},
									}),
									gui.Button(gui.ButtonCfg{
										ID:      "btn-reset-light",
										Padding: gui.SomeP(6, 12, 6, 12),
										Content: []gui.View{gui.Text(gui.TextCfg{Text: "Reset Light", TextStyle: t.N3})},
										OnClick: func(ctx gui.EventCtx) {
											ctx.Window.SetTheme(gui.ThemeLight.WithBorders(true))
											syncThemeGenFromCfg(gui.State[ShowcaseApp](ctx.Window), gui.ThemeLight.WithBorders(true).Cfg)
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
										Padding: gui.SomeP(6, 12, 6, 12),
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
													app := gui.State[ShowcaseApp](w)
													cfg := generateThemeCfg(
														app.ThemeGenSeed,
														app.ThemeGenStrategy,
														gui.CurrentTheme().TitlebarDark,
														app.ThemeGenTint,
														app.ThemeGenText,
														app.ThemeGenRadius,
														app.ThemeGenBorder,
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
										Padding: gui.SomeP(6, 12, 6, 12),
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
														gui.State[ShowcaseApp](w).ThemeGenName = err.Error()
														return
													}
													w.SetTheme(gui.ThemeMaker(cfg))
													app := gui.State[ShowcaseApp](w)
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
									gui.State[ShowcaseApp](ctx.Window).ThemeGenTint = v
									applyGenTheme(ctx.Window)
								},
							}),
						},
					}),
				},
			}),
		},
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
	app.ThemeGenText = cfg.TextStyleDef.Color
	app.ThemeGenPickText = false
}

func applyGenTheme(w *gui.Window) {
	app := gui.State[ShowcaseApp](w)
	cfg := generateThemeCfg(
		app.ThemeGenSeed,
		app.ThemeGenStrategy,
		gui.CurrentTheme().TitlebarDark,
		app.ThemeGenTint,
		app.ThemeGenText,
		app.ThemeGenRadius,
		app.ThemeGenBorder,
	)
	w.SetTheme(gui.ThemeMaker(cfg))
}

func generateThemeCfg(seed gui.Color, strategy string, isDark bool, tint float32, textColor gui.Color, radius, border float32) gui.ThemeCfg {
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
	cfg.SizeBorder = border
	cfg.Radius = radius
	cfg.RadiusSmall = radius * 0.64
	cfg.RadiusMedium = radius
	cfg.RadiusLarge = radius * 1.36
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
