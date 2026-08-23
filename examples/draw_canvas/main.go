// Draw_canvas demonstrates the DrawCanvas widget: a line chart, and
// the gradient fills DrawContext offers alongside its flat ones.
package main

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

var data = []float32{2, 5, 3, 8, 6, 4, 7, 9, 5, 10, 8, 6, 11, 7}

func main() {
	gui.SetTheme(gui.ThemeDark)

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

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.CurrentTheme().PaddingLarge,

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
				Padding: gui.NewPadding(30, 40, 40, 50),
				OnDraw:  drawChart,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Gradient Fills",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:      "gradients",
				Version: 1,
				Width:   560,
				Height:  140,
				Color:   gui.RGBA(30, 30, 40, 255),
				Radius:  8,
				Padding: gui.PadAll(16),
				OnDraw:  drawGradients,
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

// drawGradients shows the six gradient fills. Each takes a
// *gui.CanvasGradient in place of the flat Color its plain twin takes;
// geometry left at zero is derived from the shape being filled, so the
// radial cases below name only their stops.
func drawGradients(dc *gui.DrawContext) {
	const (
		size = 96
		gap  = 16
	)
	y := (dc.Height - size) / 2
	x := float32(0)
	next := func() float32 {
		v := x
		x += size + gap
		return v
	}

	warm := []gui.GradientStop{
		{Color: gui.RGB(240, 120, 60), Pos: 0},
		{Color: gui.RGB(250, 210, 90), Pos: 0.5},
		{Color: gui.RGB(90, 150, 240), Pos: 1},
	}
	// A glow: opaque at the center, fading to nothing at the rim. This
	// is the case that used to need a stack of concentric discs.
	glow := []gui.GradientStop{
		{Color: gui.RGBA(255, 220, 140, 255), Pos: 0},
		{Color: gui.RGBA(255, 160, 60, 120), Pos: 0.55},
		{Color: gui.RGBA(255, 120, 40, 0), Pos: 1},
	}

	// Linear, explicit endpoints: left to right across the square.
	r := next()
	dc.FilledRectGradient(r, y, size, size, &gui.CanvasGradient{
		X1: r, Y1: y, X2: r + size, Y2: y, Stops: warm,
	})

	// Rounded rect, endpoints left unset: runs top to bottom.
	r = next()
	dc.FilledRoundedRectGradient(r, y, size, size, 14,
		&gui.CanvasGradient{Stops: warm})

	// Radial with no geometry at all: centered on the circle, its
	// radius matched to it.
	r = next()
	dc.FilledCircleGradient(r+size/2, y+size/2, size/2,
		&gui.CanvasGradient{Radial: true, Stops: glow})

	// A pie slice.
	r = next()
	dc.FilledArcGradient(r+size/2, y+size/2, size/2, size/2,
		-math.Pi/2, 1.6*math.Pi,
		&gui.CanvasGradient{Radial: true, Stops: warm})

	// A convex polygon: a hexagon.
	r = next()
	hex := make([]float32, 0, 12)
	for i := range 6 {
		a := float64(i)*math.Pi/3 - math.Pi/2
		hex = append(hex,
			r+size/2+size/2*float32(math.Cos(a)),
			y+size/2+size/2*float32(math.Sin(a)))
	}
	dc.FilledPolygonGradient(hex, &gui.CanvasGradient{Stops: warm})

	// Caller-supplied geometry, the escape hatch the five above are
	// built on. Repeat tiles the ramp across the shape.
	r = next()
	dc.FillTrianglesGradient([]float32{
		r, y + size, r + size/2, y, r + size, y + size,
	}, &gui.CanvasGradient{
		X1: r, Y1: 0, X2: r + size/3, Y2: 0,
		Spread: gui.SpreadRepeat, Stops: warm,
	})
}
