// Draw_canvas demonstrates the DrawCanvas widget: a line chart, and
// the gradient fills DrawContext offers alongside its flat ones.
package main

import (
	"math"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

var data = []float32{2, 5, 3, 8, 6, 4, 7, 9, 5, 10, 8, 6, 11, 7}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Draw Canvas — Line Chart",
		Width:  640,
		Height: 480,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {

	return gui.Column(gui.ContainerCfg{
		// The three canvases are taller than the window, so the column
		// scrolls. A scrollable container needs an ID: the scroll offset
		// is stored against it.
		ID:         "canvases",
		Scrollable: true,
		Sizing:     gui.FillFill,
		Padding:    gui.CurrentTheme().PaddingLarge,

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
			gui.Text(gui.TextCfg{
				Text:      "Per-Vertex Colors",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:      "vertex_colors",
				Version: 1,
				Width:   560,
				Height:  140,
				Color:   gui.RGBA(30, 30, 40, 255),
				Radius:  8,
				Padding: gui.PadAll(16),
				OnDraw:  drawVertexColors,
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

// drawVertexColors shows FillTrianglesColors, the primitive underneath
// the gradient fills. The caller supplies the geometry and one color
// per vertex; nothing is evaluated on the way through.
//
// Both tiles are shadings a gradient cannot express. A gradient's level
// sets are conic curves nested inside one another and stepped linearly,
// so a sweep around a center and a bilinear corner blend are both out
// of reach however the stops are arranged.
func drawVertexColors(dc *gui.DrawContext) {
	const (
		size = 96
		gap  = 24
	)
	y := (dc.Height - size) / 2

	// A bilinear blend: four corners, four colors, two triangles. The
	// shared diagonal is why the colors have to be per vertex — the two
	// triangles meet along it and their vertex colors agree there, so
	// there is no seam.
	x := float32(0)
	tl := gui.RGB(240, 120, 60)
	tr := gui.RGB(250, 210, 90)
	br := gui.RGB(90, 150, 240)
	bl := gui.RGB(150, 80, 220)
	dc.FillTrianglesColors([]float32{
		x, y, x + size, y, x + size, y + size,
		x, y, x + size, y + size, x, y + size,
	}, []gui.Color{tl, tr, br, tl, br, bl})

	// A sweep: hue as a function of angle around the center, laid down
	// as a triangle fan. The color varies *along* every circle centered
	// here and is constant along every radius, which is the opposite of
	// what a radial gradient does.
	const seg = 64
	x += size + gap
	cx, cy := x+size/2, y+size/2
	tris := make([]float32, 0, seg*6)
	cols := make([]gui.Color, 0, seg*3)
	white := gui.RGB(250, 250, 255)
	for i := range seg {
		a0 := float64(i) * 2 * math.Pi / seg
		a1 := float64(i+1) * 2 * math.Pi / seg
		tris = append(tris,
			cx, cy,
			cx+size/2*float32(math.Cos(a0)), cy+size/2*float32(math.Sin(a0)),
			cx+size/2*float32(math.Cos(a1)), cy+size/2*float32(math.Sin(a1)))
		cols = append(cols, white, sweepColor(a0), sweepColor(a1))
	}
	dc.FillTrianglesColors(tris, cols)
}

// sweepColor is the hue wheel: three primaries 120° apart, each fading
// linearly to nothing over the 120° on either side of it.
func sweepColor(a float64) gui.Color {
	ramp := func(offset float64) uint8 {
		d := math.Abs(math.Mod(a-offset+3*math.Pi, 2*math.Pi) - math.Pi)
		return uint8(255 * max(0, 1-d/(2*math.Pi/3)))
	}
	return gui.RGB(ramp(0), ramp(2*math.Pi/3), ramp(4*math.Pi/3))
}
