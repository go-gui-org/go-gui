// This example benchmarks scrolling an SVG-heavy page in the browser.
// Web_svg_bench scrolls a grid of SVGs at a fixed rate so frame times
// can be compared across the web backend's SVG paths (#1045). The page
// in index.html collects the timings; serve.go builds and serves it.
//
// URL parameters (web) — all optional:
//
//	rows    grid rows             (default 40)
//	cols    grid columns          (default 6)
//	cell    cell size in px       (default 140)
//	asset   tiger|flower|logo|icon|mix (default mix)
//	period  seconds top→bottom    (default 8)
//	xf      rotate|tilt|persp|lift: wrap each cell in a clipping
//	        container with that view transform (default none)
package main

import (
	_ "embed"
	"flag"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

//go:embed assets/tiger.svg
var tigerSvg string

//go:embed assets/sample_transparent.svg
var flowerSvg string

//go:embed assets/gradient_logo.svg
var logoSvg string

//go:embed assets/transparent_icon.svg
var iconSvg string

const scrollID = "bench_scroll"

// BenchApp holds the workload parameters, fixed at startup.
type BenchApp struct {
	Rows   int
	Cols   int
	Cell   float32
	Assets []string
	Period time.Duration
	// Xform wraps each cell in a clipping container with this view
	// transform, to exercise SVG drawing inside transforms. The zero
	// value leaves cells unwrapped.
	Xform gui.Transform
	start time.Time
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	app := newBenchApp(queryParam)
	w := gui.NewWindow(gui.WindowCfg{
		State:  app,
		Title:  "web_svg_bench",
		Width:  1000,
		Height: 700,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
			startScroll(w)
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

// newBenchApp reads the workload parameters through param, which
// returns "" for an unset name.
func newBenchApp(param func(string) string) *BenchApp {
	app := &BenchApp{
		Rows:   intParam(param("rows"), 40),
		Cols:   intParam(param("cols"), 6),
		Cell:   float32(intParam(param("cell"), 140)),
		Period: time.Duration(intParam(param("period"), 8)) * time.Second,
	}
	app.Xform = benchTransform(param("xf"))
	switch param("asset") {
	case "tiger":
		app.Assets = []string{tigerSvg}
	case "flower":
		app.Assets = []string{flowerSvg}
	case "logo":
		app.Assets = []string{logoSvg}
	case "icon":
		app.Assets = []string{iconSvg}
	default:
		app.Assets = []string{tigerSvg, flowerSvg, logoSvg, iconSvg}
	}
	return app
}

// benchTransform maps the xf parameter to a view transform.
func benchTransform(name string) gui.Transform {
	switch name {
	case "rotate":
		return gui.Rotate(20)
	case "tilt":
		return gui.Skew(12, 0)
	case "persp":
		return gui.Perspective(600).RotateY(35)
	case "lift":
		return gui.Translate(0, -6).Scale(1.05)
	default:
		return gui.Transform{}
	}
}

func intParam(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return def
}

// startScroll moves the grid on a triangle wave, top to bottom and
// back, once per Period. Position follows wall time, not frame count,
// so a slow renderer covers the same distance in fewer frames rather
// than scrolling slower.
func startScroll(w *gui.Window) {
	app := gui.State[BenchApp](w)
	app.start = time.Now()
	w.AnimationAdd(&gui.Animate{
		AnimID: "bench_scroll_driver",
		Repeat: true,
		Callback: func(_ *gui.Animate, w *gui.Window) {
			a := gui.State[BenchApp](w)
			w.ScrollVerticalToPct(scrollID, scrollPct(time.Since(a.start), a.Period))
		},
	})
}

// scrollPct maps elapsed time to a 0→1→0 triangle wave of the given
// period.
func scrollPct(elapsed, period time.Duration) float32 {
	if period <= 0 {
		return 0
	}
	phase := math.Mod(elapsed.Seconds(), period.Seconds()) / period.Seconds()
	return float32(1 - math.Abs(2*phase-1))
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[BenchApp](w)
	rows := make([]gui.View, app.Rows)
	for r := range rows {
		cells := make([]gui.View, app.Cols)
		for c := range cells {
			svg := gui.Svg(gui.SvgCfg{
				SvgData: app.Assets[(r*app.Cols+c)%len(app.Assets)],
				Sizing:  gui.FixedFixed,
				Width:   app.Cell,
				Height:  app.Cell,
			})
			if app.Xform.IsIdentity() {
				cells[c] = svg
				continue
			}
			cells[c] = gui.Column(gui.ContainerCfg{
				SizeBorder: gui.NoBorder,
				Clip:       true,
				Transform:  app.Xform,
				Content:    []gui.View{svg},
			})
		}
		rows[r] = gui.Row(gui.ContainerCfg{
			SizeBorder: gui.NoBorder,
			Spacing:    gui.SpacingMedium,
			Content:    cells,
		})
	}
	return gui.Column(gui.ContainerCfg{
		ID:         scrollID,
		Scrollable: true,
		Sizing:     gui.FillFill,
		Padding:    gui.PaddingSmall,
		Spacing:    gui.SpacingMedium,
		Content:    rows,
	})
}
