package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
)

//go:embed docs/*.md locales/*.json assets/*
var showcaseFS embed.FS

var showcaseLocaleIDs = []string{"en-US", "de-DE", "ar-SA", "ja-JP"}
var showcaseLocaleLabels = []string{"EN", "DE", "AR", "JA"}

var docPageFiles = map[string]string{
	"welcome":  "docs/welcome.md",
	"commands": "docs/commands.md",
}

func loadEmbeddedLocales() {
	for _, name := range []string{
		"locales/de-DE.json",
		"locales/ar-SA.json",
		"locales/ja-JP.json",
	} {
		data, err := showcaseFS.ReadFile(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "showcase: failed to read %s: %v\n", name, err)
			continue
		}
		locale, err := gui.LocaleParse(string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "showcase: failed to parse %s: %v\n", name, err)
			continue
		}
		gui.LocaleRegister(locale)
	}
}

func showcaseLocaleAt(idx int) (gui.Locale, bool) {
	if idx < 0 || idx >= len(showcaseLocaleIDs) {
		return gui.Locale{}, false
	}
	locale, ok := gui.LocaleGet(showcaseLocaleIDs[idx])
	return locale, ok
}

func localeLabel(idx int) string {
	if idx < 0 || idx >= len(showcaseLocaleLabels) {
		return "EN"
	}
	return showcaseLocaleLabels[idx]
}

func localeCount() int {
	return len(showcaseLocaleIDs)
}

func docPageSource(id string) string {
	file, ok := docPageFiles[id]
	if !ok {
		return ""
	}
	data, err := showcaseFS.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(data)
}

func embeddedText(name string) string {
	data, err := showcaseFS.ReadFile(name)
	if err != nil {
		return ""
	}
	return string(data)
}

func floatString(v float32) string {
	return fmt.Sprintf("%.1f", v)
}

func relatedExamplePaths(id string) []string {
	examples, ok := relatedExampleMap[id]
	if !ok || len(examples) == 0 {
		if file, ok := docPageFiles[id]; ok {
			return []string{"examples/showcase/" + file}
		}
		return []string{"examples/showcase/main.go"}
	}
	return append([]string(nil), examples...)
}

func relatedExamples(id string) string {
	return strings.Join(relatedExamplePaths(id), ", ")
}

var relatedExampleMap = map[string][]string{
	"audio":               {"examples/showcase/demo_audio.go"},
	"welcome":             {"examples/showcase/demo_welcome.go", "examples/showcase/docs/welcome.md"},
	"commands":            {"examples/showcase/docs/commands.md", "examples/command_demo/main.go"},
	"button":              {"examples/showcase/demo_feedback.go"},
	"input":               {"examples/showcase/demo_input.go", "examples/multiline_input/main.go"},
	"toggle":              {"examples/showcase/demo_selection.go"},
	"switch":              {"examples/showcase/demo_selection.go"},
	"radio":               {"examples/showcase/demo_selection.go"},
	"radio_group":         {"examples/showcase/demo_selection.go"},
	"combobox":            {"examples/showcase/demo_selection.go"},
	"select":              {"examples/showcase/demo_selection.go"},
	"listbox":             {"examples/showcase/demo_selection.go", "examples/listbox/main.go"},
	"slider":              {"examples/showcase/demo_selection.go"},
	"progress_bar":        {"examples/showcase/demo_feedback.go"},
	"pulsar":              {"examples/showcase/demo_feedback.go"},
	"toast":               {"examples/showcase/demo_feedback.go"},
	"native_notification": {"examples/showcase/demo_overlays.go", "examples/dialogs/main.go"},
	"badge":               {"examples/showcase/demo_feedback.go"},
	"skeleton":            {"examples/showcase/demo_feedback.go"},
	"breadcrumb":          {"examples/showcase/demo_navigation.go"},
	"menus":               {"examples/showcase/demo_navigation.go", "examples/menu_demo/main.go", "examples/context_menu/main.go"},
	"dialog":              {"examples/showcase/demo_overlays.go", "examples/dialogs/main.go"},
	"tree":                {"examples/showcase/demo_data.go"},
	"drag_reorder":        {"examples/showcase/demo_selection.go"},
	"printing":            {"examples/showcase/demo_layout.go"},
	"text":                {"examples/showcase/demo_text.go"},
	"rtf":                 {"examples/showcase/demo_text.go", "examples/rtf/main.go"},
	"table":               {"examples/showcase/demo_data.go"},
	"data_grid":           {"examples/showcase/demo_data.go", "examples/data_grid_data_source/main.go"},
	"data_source":         {"examples/showcase/demo_data.go", "examples/data_grid_data_source/main.go"},
	"numeric_input":       {"examples/showcase/demo_input.go"},
	"forms":               {"examples/showcase/demo_input.go"},
	"date_picker":         {"examples/showcase/demo_input.go", "examples/date_picker_options/main.go"},
	"input_date":          {"examples/showcase/demo_input.go", "examples/date_picker_options/main.go"},
	"date_picker_roller":  {"examples/showcase/demo_input.go"},
	"svg":                 {"examples/showcase/demo_graphics.go", "examples/svg/main.go"},
	"image":               {"examples/showcase/demo_graphics.go"},
	"expand_panel":        {"examples/showcase/demo_layout.go"},
	"icons":               {"examples/showcase/demo_graphics.go"},
	"gradient":            {"examples/showcase/demo_graphics.go", "examples/gradient_demo/main.go"},
	"box_shadows":         {"examples/showcase/demo_graphics.go", "examples/shadow_demo/main.go"},
	"shader":              {"examples/showcase/demo_graphics.go", "examples/custom_shader/main.go"},
	"draw_canvas":         {"examples/showcase/demo_graphics.go", "examples/draw_canvas/main.go"},
	"animations":          {"examples/showcase/demo_animations.go", "examples/animations/main.go"},
	"color_picker":        {"examples/showcase/demo_input.go", "examples/color_picker/main.go"},
	"theme_gen":           {"examples/showcase/demo_theme.go", "examples/showcase/main.go"},
	"markdown":            {"examples/showcase/demo_text.go", "examples/markdown/main.go", "examples/markdown/markdown_source.md"},
	"tab_control":         {"examples/showcase/demo_navigation.go"},
	"command_palette":     {"examples/showcase/demo_navigation.go"},
	"context_menu":        {"examples/showcase/demo_overlays.go", "examples/context_menu/main.go"},
	"tooltip":             {"examples/showcase/demo_overlays.go"},
	"rectangle":           {"examples/showcase/demo_graphics.go", "examples/gradient_demo/main.go"},
	"scrollbar":           {"examples/showcase/demo_layout.go", "examples/scroll_demo/main.go"},
	"splitter":            {"examples/showcase/demo_layout.go"},
	"rotated_box":         {"examples/showcase/demo_layout.go"},
	"row":                 {"examples/showcase/demo_layout.go"},
	"column_demo":         {"examples/showcase/demo_layout.go"},
	"wrap_panel":          {"examples/showcase/demo_layout.go"},
	"overflow_panel":      {"examples/showcase/demo_layout.go"},
	"sidebar":             {"examples/showcase/demo_layout.go"},
	"dock_layout":         {"examples/showcase/demo_dock_layout.go", "examples/dock_layout/main.go"},
	"multi_window":        {"examples/showcase/demo_layout.go", "examples/multi_window/main.go"},
	"command_button":      {"examples/showcase/demo_feedback.go", "examples/command_demo/main.go"},
	"theme_picker":        {"examples/showcase/demo_feedback.go"},
}

func themeCfgSave(path string, cfg gui.ThemeCfg) error {
	data, err := json.Marshal(themeCfgJSONFromCfg(cfg))
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func themeCfgLoad(path string) (gui.ThemeCfg, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return gui.ThemeCfg{}, err
	}
	var stored themeCfgJSON
	if err := json.Unmarshal(data, &stored); err != nil {
		return gui.ThemeCfg{}, err
	}
	return stored.toThemeCfg(), nil
}

type themeCfgJSON struct {
	TextStyleDef     textStyleJSON `json:"text_style_def"`
	Name             string        `json:"name"`
	SizeBorder       float32       `json:"size_border"`
	Radius           float32       `json:"radius"`
	RadiusSmall      float32       `json:"radius_small"`
	RadiusMedium     float32       `json:"radius_medium"`
	RadiusLarge      float32       `json:"radius_large"`
	ColorBackground  colorJSON     `json:"color_background"`
	ColorPanel       colorJSON     `json:"color_panel"`
	ColorInterior    colorJSON     `json:"color_interior"`
	ColorHover       colorJSON     `json:"color_hover"`
	ColorFocus       colorJSON     `json:"color_focus"`
	ColorActive      colorJSON     `json:"color_active"`
	ColorBorder      colorJSON     `json:"color_border"`
	ColorBorderFocus colorJSON     `json:"color_border_focus"`
	ColorSelect      colorJSON     `json:"color_select"`
	TitlebarDark     bool          `json:"titlebar_dark"`
}

type colorJSON struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"`
}

type affineTransformJSON struct {
	XX float32 `json:"xx"`
	XY float32 `json:"xy"`
	YX float32 `json:"yx"`
	YY float32 `json:"yy"`
	X0 float32 `json:"x0"`
	Y0 float32 `json:"y0"`
}

type gradientStopJSON struct {
	Color    colorJSON `json:"color"`
	Position float32   `json:"position"`
}

type gradientConfigJSON struct {
	Stops     []gradientStopJSON      `json:"stops"`
	Direction glyph.GradientDirection `json:"direction"`
}

type textStyleJSON struct {
	AffineTransform *affineTransformJSON `json:"affine_transform,omitempty"`
	Gradient        *gradientConfigJSON  `json:"gradient,omitempty"`
	Family          string               `json:"family"`
	Size            float32              `json:"size"`
	LineSpacing     float32              `json:"line_spacing"`
	LetterSpacing   float32              `json:"letter_spacing"`
	RotationRadians float32              `json:"rotation_radians"`
	StrokeWidth     float32              `json:"stroke_width"`
	Color           colorJSON            `json:"color"`
	BgColor         colorJSON            `json:"bg_color"`
	StrokeColor     colorJSON            `json:"stroke_color"`
	Align           gui.TextAlignment    `json:"align"`
	Underline       bool                 `json:"underline"`
	Strikethrough   bool                 `json:"strikethrough"`
}

func themeCfgJSONFromCfg(cfg gui.ThemeCfg) themeCfgJSON {
	return themeCfgJSON{
		Name:             cfg.Name,
		ColorBackground:  colorJSONFromColor(cfg.ColorBackground),
		ColorPanel:       colorJSONFromColor(cfg.ColorPanel),
		ColorInterior:    colorJSONFromColor(cfg.ColorInterior),
		ColorHover:       colorJSONFromColor(cfg.ColorHover),
		ColorFocus:       colorJSONFromColor(cfg.ColorFocus),
		ColorActive:      colorJSONFromColor(cfg.ColorActive),
		ColorBorder:      colorJSONFromColor(cfg.ColorBorder),
		ColorBorderFocus: colorJSONFromColor(cfg.ColorBorderFocus),
		ColorSelect:      colorJSONFromColor(cfg.ColorSelect),
		TitlebarDark:     cfg.TitlebarDark,
		SizeBorder:       cfg.SizeBorder,
		Radius:           cfg.Radius,
		RadiusSmall:      cfg.RadiusSmall,
		RadiusMedium:     cfg.RadiusMedium,
		RadiusLarge:      cfg.RadiusLarge,
		TextStyleDef:     textStyleJSONFromTextStyle(cfg.TextStyleDef),
	}
}

func (cfg themeCfgJSON) toThemeCfg() gui.ThemeCfg {
	return gui.ThemeCfg{
		Name:             cfg.Name,
		ColorBackground:  cfg.ColorBackground.toColor(),
		ColorPanel:       cfg.ColorPanel.toColor(),
		ColorInterior:    cfg.ColorInterior.toColor(),
		ColorHover:       cfg.ColorHover.toColor(),
		ColorFocus:       cfg.ColorFocus.toColor(),
		ColorActive:      cfg.ColorActive.toColor(),
		ColorBorder:      cfg.ColorBorder.toColor(),
		ColorBorderFocus: cfg.ColorBorderFocus.toColor(),
		ColorSelect:      cfg.ColorSelect.toColor(),
		TitlebarDark:     cfg.TitlebarDark,
		SizeBorder:       cfg.SizeBorder,
		Radius:           cfg.Radius,
		RadiusSmall:      cfg.RadiusSmall,
		RadiusMedium:     cfg.RadiusMedium,
		RadiusLarge:      cfg.RadiusLarge,
		TextStyleDef:     cfg.TextStyleDef.toTextStyle(),
	}
}

func colorJSONFromColor(c gui.Color) colorJSON {
	return colorJSON{R: c.R, G: c.G, B: c.B, A: c.A}
}

func (c colorJSON) toColor() gui.Color {
	return gui.RGBA(c.R, c.G, c.B, c.A)
}

func affineTransformJSONFromGlyph(
	t *glyph.AffineTransform,
) *affineTransformJSON {
	if t == nil {
		return nil
	}
	return &affineTransformJSON{
		XX: t.XX, XY: t.XY, YX: t.YX, YY: t.YY, X0: t.X0, Y0: t.Y0,
	}
}

func (t *affineTransformJSON) toGlyph() *glyph.AffineTransform {
	if t == nil {
		return nil
	}
	return &glyph.AffineTransform{
		XX: t.XX, XY: t.XY, YX: t.YX, YY: t.YY, X0: t.X0, Y0: t.Y0,
	}
}

func gradientConfigJSONFromGlyph(
	g *glyph.GradientConfig,
) *gradientConfigJSON {
	if g == nil {
		return nil
	}
	stops := make([]gradientStopJSON, 0, len(g.Stops))
	for _, stop := range g.Stops {
		stops = append(stops, gradientStopJSON{
			Color:    colorJSON{R: stop.Color.R, G: stop.Color.G, B: stop.Color.B, A: stop.Color.A},
			Position: stop.Position,
		})
	}
	return &gradientConfigJSON{
		Stops:     stops,
		Direction: g.Direction,
	}
}

func (g *gradientConfigJSON) toGlyph() *glyph.GradientConfig {
	if g == nil {
		return nil
	}
	stops := make([]glyph.GradientStop, 0, len(g.Stops))
	for _, stop := range g.Stops {
		stops = append(stops, glyph.GradientStop{
			Color: glyph.Color{
				R: stop.Color.R,
				G: stop.Color.G,
				B: stop.Color.B,
				A: stop.Color.A,
			},
			Position: stop.Position,
		})
	}
	return &glyph.GradientConfig{
		Stops:     stops,
		Direction: g.Direction,
	}
}

func textStyleJSONFromTextStyle(ts gui.TextStyle) textStyleJSON {
	return textStyleJSON{
		Family:          ts.Family,
		Color:           colorJSONFromColor(ts.Color),
		BgColor:         colorJSONFromColor(ts.BgColor),
		Size:            ts.Size,
		LineSpacing:     ts.LineSpacing,
		LetterSpacing:   ts.LetterSpacing,
		Align:           ts.Align,
		Underline:       ts.Underline,
		Strikethrough:   ts.Strikethrough,
		RotationRadians: ts.RotationRadians,
		AffineTransform: affineTransformJSONFromGlyph(ts.AffineTransform),
		Gradient:        gradientConfigJSONFromGlyph(ts.Gradient),
		StrokeWidth:     ts.StrokeWidth,
		StrokeColor:     colorJSONFromColor(ts.StrokeColor),
	}
}

func (ts textStyleJSON) toTextStyle() gui.TextStyle {
	return gui.TextStyle{
		Family:          ts.Family,
		Color:           ts.Color.toColor(),
		BgColor:         ts.BgColor.toColor(),
		Size:            ts.Size,
		LineSpacing:     ts.LineSpacing,
		LetterSpacing:   ts.LetterSpacing,
		Align:           ts.Align,
		Underline:       ts.Underline,
		Strikethrough:   ts.Strikethrough,
		RotationRadians: ts.RotationRadians,
		AffineTransform: ts.AffineTransform.toGlyph(),
		Gradient:        ts.Gradient.toGlyph(),
		StrokeWidth:     ts.StrokeWidth,
		StrokeColor:     ts.StrokeColor.toColor(),
	}
}
