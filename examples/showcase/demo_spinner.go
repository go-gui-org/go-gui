package main

import "github.com/go-gui-org/go-gui/gui"

func demoSpinner(w *gui.Window) gui.View {
	t := gui.CurrentTheme()

	type entry struct {
		label  string
		curve  gui.CurveType
		rotate bool
		color  gui.Color
	}

	active := t.ColorActive
	success := t.Cfg.ColorSuccess
	warning := t.Cfg.ColorWarning
	errClr := t.Cfg.ColorError
	sel := t.ColorSelect

	entries := []entry{
		{"Original Thinking", gui.CurveOriginalThinking, true, active},
		{"Thinking Five", gui.CurveThinkingFive, false, success},
		{"Thinking Nine", gui.CurveThinkingNine, false, warning},
		{"Rose Orbit", gui.CurveRoseOrbit, true, errClr},
		{"Rose", gui.CurveRose, true, active},
		{"Rose Two", gui.CurveRoseTwo, false, sel},
		{"Rose Three", gui.CurveRoseThree, false, success},
		{"Rose Four", gui.CurveRoseFour, false, active},
		{"Lissajous", gui.CurveLissajous, false, warning},
		{"Lemniscate", gui.CurveLemniscate, false, active},
		{"Hypotrochoid", gui.CurveHypotrochoid, true, success},
		{"3-Petal Spiral", gui.CurveThreePetalSpiral, false, errClr},
		{"4-Petal Spiral", gui.CurveFourPetalSpiral, false, active},
		{"5-Petal Spiral", gui.CurveFivePetalSpiral, false, sel},
		{"6-Petal Spiral", gui.CurveSixPetalSpiral, false, warning},
		{"Butterfly", gui.CurveButterfly, false, success},
		{"Cardioid", gui.CurveCardioid, false, errClr},
		{"Cardioid Heart", gui.CurveCardioidHeart, false, active},
		{"Heart Wave", gui.CurveHeartWave, false, warning},
		{"Spiral", gui.CurveSpiral, false, sel},
		{"Fourier", gui.CurveFourier, false, active},
	}

	cells := make([]gui.View, len(entries))
	for i, e := range entries {
		cells[i] = gui.Column(gui.ContainerCfg{
			Sizing:     gui.FitFit,
			Padding:    gui.Some(t.PaddingMedium),
			SizeBorder: gui.NoBorder,
			HAlign:     gui.HAlignCenter,
			Spacing:    gui.SomeF(6),
			Content: []gui.View{
				gui.Spinner(gui.SpinnerCfg{
					ID:        "sp-" + e.label,
					CurveType: e.curve,
					Size:      100,
					Color:     e.color,
					Rotate:    e.rotate,
				}, w),
				gui.Text(gui.TextCfg{
					Text:      e.label,
					TextStyle: t.N4,
				}),
			},
		})
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(12),
		Content: []gui.View{
			gui.Wrap(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SomeF(8),
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content:    cells,
			}),
		},
	})
}
