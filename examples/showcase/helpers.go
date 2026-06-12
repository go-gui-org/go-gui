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

func line() gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Height:     1,
		Padding:    gui.SomeP(3, 0, 0, 0),
		SizeBorder: gui.NoBorder,
		Radius:     gui.NoRadius,
		Color:      gui.CurrentTheme().ColorActive,
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
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.N2}),
		},
	})
}
