// This example ports Qt's qcpainterbench canvas benchmark to go-gui.
//
// Qcpainterbench draws six 2D workloads (ruler, gauges, line graphs, bar graphs,
// icons with text, a rotating flower) N times per frame and reports frames per
// second. This port draws the same workloads through DrawCanvas, so the numbers
// characterize go-gui's canvas tessellation and GPU paths.
//
// Run it interactively, or pass -sweep for a scripted run that prints one CSV row
// per render count. See README.md for how to read the numbers.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// Window size. Qt's window is 375×667; this one is wider. Every workload scales with
// the canvas size, so results are comparable only at the same size.
const (
	winW = 667
	winH = 667
)

var colBackground = gui.RGBA(0x40, 0x40, 0x40, 255)

// maxCount caps the render count. Qt's selector stops at 512; the cap leaves room
// for faster machines but stops a typo such as -count 1000000 from hanging the
// window on one frame.
const maxCount = 4096

// maxSeconds caps -seconds and -warmup. A larger value is a typo, and a very large
// float would overflow time.Duration.
const maxSeconds = 3600

// renderCounts are the render counts the selector offers, as in Qt.
var renderCounts = []string{"1", "2", "4", "8", "16", "32", "64", "128", "256", "512"}

// testNames labels the test bits, in bit order.
var testNames = [...]string{"Ruler", "Circles", "Lines", "Bars", "Icons", "Flower"}

// App is the window state.
type App struct {
	scene scene
	meter meter
	sweep *sweep // nil in interactive mode

	Tests int // mask of test bits
	Count int // render count: passes per frame

	// The animation clock. T is the time the canvas draws at. It follows the wall
	// clock from Start, and stands still while Paused.
	Start    time.Time
	PausedAt time.Time
	Paused   bool
	T        float32
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	tests := flag.Int("tests", testDefault,
		"mask of tests: 1 ruler, 2 circles, 4 lines, 8 bars, 16 icons, 32 flower")
	count := flag.Int("count", 1, "render count: passes per frame")
	sweepList := flag.String("sweep", "",
		"comma-separated render counts to run in turn, printing CSV, then exit")
	seconds := flag.Float64("seconds", 5, "sweep: measured seconds per render count")
	warmup := flag.Float64("warmup", 2, "sweep: unmeasured seconds per render count")
	novsync := flag.Bool("novsync", false,
		"present without vsync, so FPS is not capped at the display refresh rate")
	flag.Parse()

	app := &App{Tests: *tests & testAll, Count: min(max(*count, 1), maxCount), Start: time.Now()}
	if *sweepList != "" {
		sw, err := newSweep(*sweepList, secs(*warmup), secs(*seconds))
		if err != nil {
			log.Fatalf("-sweep: %v", err)
		}
		app.sweep = sw
		app.Count = sw.counts[0]
		fmt.Println(csvHeader)
	}

	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:   app,
		Title:   "qcpainterbench",
		Width:   winW,
		Height:  winH,
		Timings: true,
		// Qt runs this benchmark with vsync off and reports raw FPS.
		VSyncOff: *novsync,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
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
	app := gui.State[App](w)
	now := time.Now()

	// The view runs once per frame, so this is where frames are counted and the
	// animation clock advances. An animation callback would run on go-gui's 16 ms
	// ticker and cap the measurement near 62.5 FPS.
	timings := w.Timings()
	app.meter.frame(now, timings)
	if !app.Paused {
		app.T = float32(now.Sub(app.Start).Seconds())
		// Qt's animation runs 0..360 s and loops.
		for app.T >= 360 {
			app.Start = app.Start.Add(360 * time.Second)
			app.T -= 360
		}
	}
	if app.sweep != nil {
		if done := app.sweep.step(app, now, timings); done {
			w.Close()
			return gui.Column(gui.ContainerCfg{})
		}
	}

	// Request the next frame now. The backend sleeps while no refresh is pending,
	// so an uncapped benchmark must ask for a rebuild every frame. It must be a
	// layout refresh: a render-only refresh never runs this function again.
	w.InvalidateLayout()

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PaddingNone,
		Spacing: gui.SpacingPx(0),
		Content: []gui.View{
			controls(app),
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:     "qcpb-canvas",
				Sizing: gui.FillFill,
				Color:  colBackground,
				Clip:   true,
				// The canvas animates every frame, so the cache never hits;
				// AlwaysRedraw says so instead of bumping Version.
				AlwaysRedraw: true,
				OnDraw: func(dc *gui.DrawContext) {
					app.scene.paint(dc, dc.Width, dc.Height, app.T, app.Tests,
						app.Count)
				},
				// Qt pauses and resumes the animation on a click.
				OnClick: func(ctx gui.EventCtx) {
					togglePause(gui.State[App](ctx.Window))
				},
			}),
		},
	})
}

// controls is the top bar: the FPS readout, the render count and the test
// toggles.
func controls(app *App) gui.View {
	theme := gui.CurrentTheme()
	toggles := make([]gui.View, 0, len(testNames))
	for i, name := range testNames {
		bit := 1 << i
		toggles = append(toggles, gui.Toggle(gui.ToggleCfg{
			ID:       "qcpb-test-" + name,
			Label:    name,
			Selected: app.Tests&bit != 0,
			OnClick: func(ctx gui.EventCtx) {
				a := gui.State[App](ctx.Window)
				a.Tests ^= bit
				a.meter.reset()
			},
		}))
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.PaddingSmall,
		Spacing: gui.SpacingSmall,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.PaddingNone,
				Spacing: gui.SpacingSmall,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      app.meter.String(),
						TextStyle: theme.Mono(theme.TextStyleBodySmall),
					}),
					gui.Select(gui.SelectCfg{
						ID:       "qcpb-count",
						Selected: []string{strconv.Itoa(app.Count)},
						Items:    renderCounts,
						OnSelect: func(sel []string, ctx gui.EventCtx) {
							if len(sel) == 0 {
								return
							}
							n, err := strconv.Atoi(sel[0])
							if err != nil {
								return
							}
							a := gui.State[App](ctx.Window)
							a.Count = n
							a.meter.reset()
						},
					}),
				},
			}),
			gui.Wrap(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.PaddingNone,
				Spacing: gui.SpacingSmall,
				Content: toggles,
			}),
		},
	})
}

func togglePause(app *App) {
	now := time.Now()
	if app.Paused {
		// Resume where the clock stopped.
		app.Start = app.Start.Add(now.Sub(app.PausedAt))
	} else {
		app.PausedAt = now
	}
	app.Paused = !app.Paused
}

// secs converts a seconds flag to a Duration. NaN and negative values become 0,
// and values above maxSeconds are capped, so the conversion cannot overflow.
func secs(s float64) time.Duration {
	if !(s > 0) { // also catches NaN
		return 0
	}
	return time.Duration(min(s, maxSeconds) * float64(time.Second))
}

// parseCounts parses a comma-separated list of render counts in 1..maxCount.
func parseCounts(list string) ([]int, error) {
	var out []int
	for f := range strings.SplitSeq(list, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 || n > maxCount {
			return nil, fmt.Errorf("bad render count %q", f)
		}
		out = append(out, n)
	}
	return out, nil
}
