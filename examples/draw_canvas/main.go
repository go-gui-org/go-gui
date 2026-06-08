// Draw_canvas demonstrates the DrawCanvas widget with a line chart.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

var data = []float32{2, 5, 3, 8, 6, 4, 7, 9, 5, 10, 8, 6, 11, 7}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Draw Canvas — Line Chart",
		Width:  640,
		Height: 480,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Padding: gui.Some(gui.CurrentTheme().PaddingLarge),
		Spacing: gui.SomeF(16),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Line Chart",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:      "chart",
				Version: 1,
				Width:   560,
				Height:  360,
				Color:   gui.RGBA(30, 30, 40, 255),
				Radius:  8,
				Padding: gui.Some(gui.Padding{Top: 30, Right: 40, Bottom: 40, Left: 50}),
				OnDraw:  drawChart,
			}),
		},
	})
}

func drawChart(dc *gui.DrawContext) {
	cw := dc.Width
	ch := dc.Height

	// Grid lines.
	gridColor := gui.RGBA(80, 80, 100, 255)
	rows := 5
	for i := range rows + 1 {
		y := ch * float32(i) / float32(rows)
		dc.Line(0, y, cw, y, gridColor, 1)
	}
	cols := len(data) - 1
	for i := range cols + 1 {
		x := cw * float32(i) / float32(cols)
		dc.Line(x, 0, x, ch, gridColor, 1)
	}

	// Data range.
	mn, mx := data[0], data[0]
	for _, v := range data {
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
	pts := make([]float32, 0, len(data)*2)
	for i, v := range data {
		x := cw * float32(i) / float32(len(data)-1)
		y := ch - ch*(v-mn)/span
		pts = append(pts, x, y)
	}

	// Filled area under curve (trapezoid strips to avoid concave fan artifacts).
	fillColor := gui.RGBA(70, 130, 220, 60)
	for i := 0; i+3 < len(pts); i += 2 {
		dc.FilledPolygon([]float32{
			pts[i], pts[i+1],
			pts[i+2], pts[i+3],
			pts[i+2], ch,
			pts[i], ch,
		}, fillColor)
	}

	// Line.
	dc.Polyline(pts, gui.RGBA(70, 130, 220, 255), 2.5)

	// Dot markers.
	for i := 0; i < len(pts); i += 2 {
		dc.FilledCircle(pts[i], pts[i+1], 4, gui.RGBA(220, 220, 255, 255))
	}
}
