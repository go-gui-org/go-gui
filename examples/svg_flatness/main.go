// Svg_flatness visualizes the FlatnessTolerance knob on SvgCfg. A
// curve-heavy logo is rendered at five increasing tolerance values:
// higher tolerance = coarser polyline approximation = fewer
// triangles = visible faceting on Bezier curves. The default is 0
// (use the renderer's built-in 0.15 floor).
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// Curve-heavy demo: a wavy spiral with tight bends. Faceting is
// most visible on long sweeping cubics; we also add a small-radius
// rounded-rect stroke to amplify polyline-edge effects on stroked
// curves.
const curveDemo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<path d="M 50,10
		C 90,10 90,50 50,50
		C 10,50 10,90 50,90
		C 90,90 90,60 50,60
		C 30,60 30,40 50,40 Z"
		fill="#22c55e" stroke="#0f172a" stroke-width="2"/>
	<path d="M 10,10 Q 50,30 90,10 Q 70,50 90,90 Q 50,70 10,90 Q 30,50 10,10 Z"
		fill="none" stroke="#a855f7" stroke-width="3"/>
</svg>`

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		Width:  900,
		Height: 320,
		Title:  "FlatnessTolerance",
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func view(w *gui.Window) gui.View {
	tolerances := []float32{0, 2, 8, 25, 60}
	cells := []gui.View{}
	for _, tol := range tolerances {
		title := "default"
		if tol > 0 {
			title = fmt.Sprintf("tol=%.1f", tol)
		}
		cells = append(cells, gui.Column(gui.ContainerCfg{
			Padding: gui.Some(gui.PaddingTwoFive),
			Sizing:  gui.FillFit,
			HAlign:  gui.HAlignCenter,
			Content: []gui.View{
				gui.Svg(gui.SvgCfg{
					SvgData:           curveDemo,
					FlatnessTolerance: tol,
					Sizing:            gui.FixedFixed,
					Width:             160, Height: 160,
				}),
				gui.Text(gui.TextCfg{Text: title}),
			},
		}))
	}
	return gui.Row(gui.ContainerCfg{Sizing: gui.FillFill, Content: cells})
}
