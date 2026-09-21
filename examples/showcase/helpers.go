package main

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// safeFloat returns v when finite; otherwise returns fallback.
// Guards against NaN/Inf seeping into view state from corrupt
// restores or arithmetic that produced invalid values.
func safeFloat(v, fallback float32) float32 {
	f := float64(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fallback
	}
	return v
}

// line returns a horizontal divider rule. The theme's divider color
// applies; the top inset is the demo's breathing room above the rule.
func line() gui.View {
	return gui.Separator(gui.SeparatorCfg{
		Inset: gui.NewPadding(3, 0, 0, 0),
	})
}

func demoBox(label string, color gui.Color) gui.View {
	return demoBoxSized(label, color, 60, 40)
}

func demoBoxSized(label string, color gui.Color, w, h float32) gui.View {
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Width:  w,
		Height: h,
		Sizing: gui.FixedFixed,
		Color:  color,
		Radius: gui.SomeF(4),
		// Fixed-size box: theme container padding would fill
		// the whole box and pin the label to its bottom edge.
		Padding: gui.NoPadding,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		// Static demo labels: centre each on its own ink, as
		// Button and Badge do, instead of on the line box.
		AmendLayout: gui.OpticalCenterText,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.TextStyleBodyLarge}),
		},
	})
}
