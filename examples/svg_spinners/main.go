// Svg_spinners showcases a curated subset of built-in
// SvgSpinner kinds. The full catalog of 100+ assets is
// available via the SvgSpinnerKind enum; the gallery renders
// only a representative selection so every cell stays visible
// on screen and the framework is not asked to re-layout 100+
// animated SVGs per mouse event.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct{}

const (
	cellsPerRow = 4
	cellSize    = 140
	spinnerSize = 72
)

// Curated showcase — one from each major family. SMIL kinds
// (phase 0-5): rotate, opacity / attr keyframes, spline
// easing, syncbase begins, animateTransform translate/scale.
// CSS-stylesheet kinds (phase 6+): @keyframes rotate /
// translate / opacity, animation-delay per element, linear-
// Gradient strokes, feGaussianBlur filters. Stroke-dash
// keyframes (ring-resize, jump, heart-pulse) and 3D rotateX/Y
// (square) remain out of scope.
var showcase = []gui.SvgSpinnerKind{
	gui.SvgSpinner90Ring,
	gui.SvgSpinner180Ring,
	gui.SvgSpinner270Ring,
	gui.SvgSpinner6DotsRotate,
	gui.SvgSpinner8DotsRotate,
	gui.SvgSpinner3DotsBounce,
	gui.SvgSpinner3DotsFade,
	gui.SvgSpinnerBars,
	gui.SvgSpinnerCog01,
	gui.SvgSpinner4DotsRotate,
	gui.SvgSpinnerGooeyBalls1,
	gui.SvgSpinnerCircleFade,
	gui.SvgSpinnerSpinnerMultiple,
	gui.SvgSpinner90RingWithGradient,
	gui.SvgSpinnerBlocksShuffle4,
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "svg_spinners",
		Width:  cellsPerRow*cellSize + 65,
		Height: 4 * cellSize,
		OnInit: func(w *gui.Window) {
			if useIsolation {
				w.UpdateView(isolatedView)
				return
			}
			w.UpdateView(mainView)
		},
	})
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	rows := make([]gui.View, 0, len(showcase)/cellsPerRow+1)
	for i := 0; i < len(showcase); i += cellsPerRow {
		cells := make([]gui.View, 0, cellsPerRow)
		for j := 0; j < cellsPerRow && i+j < len(showcase); j++ {
			cells = append(cells, cell(showcase[i+j]))
		}
		rows = append(rows, gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			HAlign:  gui.HAlignCenter,
			Content: cells,
		}))
	}
	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignCenter,
		Padding: gui.Some(gui.PaddingSmall),
		Content: rows,
	})
}

func cell(k gui.SvgSpinnerKind) gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:   cellSize,
		Height:  cellSize,
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignCenter,
		Padding: gui.Some(gui.PaddingSmall),
		Content: []gui.View{
			gui.SvgSpinner(gui.SvgSpinnerCfg{
				Kind:   k,
				Width:  spinnerSize,
				Height: spinnerSize,
			}),
			gui.Row(gui.ContainerCfg{
				Width:  cellSize - 8,
				Sizing: gui.FixedFit,
				HAlign: gui.HAlignCenter,
				Clip:   true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: gui.SvgSpinnerName(k),
						Clip: true,
					}),
				},
			}),
		},
	})
}
