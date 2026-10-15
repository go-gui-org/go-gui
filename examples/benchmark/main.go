// This example demonstrates frame costs while rendering large widget batches.
// Benchmark measures frame costs while rendering large batches of
// widgets.
package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"time"

	"log"
	"os"

	"github.com/go-gui-org/go-gui/examples/internal/framestats"
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const (
	typeButton = "Button"
	typeText   = "Text"
	typeToggle = "Toggle"
	typeMixed  = "Mixed"
)

// warmupFrames is how many frame times the readout waits for before it shows
// numbers. A p99 of a few frames is noise.
const warmupFrames = 500

// readoutEvery is how often the readout text is rebuilt. Summary sorts up to
// framestats.Size values, so it does not run on each frame.
const readoutEvery = 500 * time.Millisecond

type App struct {
	LastFrame  time.Time
	WidgetType string
	// Frames keeps the wall time between view rebuilds. View, Layout and
	// Render keep go-gui's CPU phase times of the same frames.
	Frames      framestats.Stats
	View        framestats.Stats
	Layout      framestats.Stats
	Render      framestats.Stats
	WidgetCount int
	Running     bool
	// Labels prebuilds per-widget text on count/type change so the
	// frame path measures layout, not formatting.
	Labels     []string
	LabelsType string
	// Readout is the metrics text, rebuilt every readoutEvery.
	Readout   [2]string
	ReadoutAt time.Time
}

func (a *App) ResetAvgs() {
	a.Frames.Reset()
	a.View.Reset()
	a.Layout.Reset()
	a.Render.Reset()
	a.LastFrame = time.Time{}
	a.Readout = [2]string{}
	a.ReadoutAt = time.Time{}
}

// sample records one frame that ends at now. t is go-gui's timing of the
// previous frame. The phase times are kept only with a frame time, so all four
// rings hold the same frames.
//
// Every frame is kept, however short. An older filter dropped frames of 1 ms or
// less to guard 1e9/dt; with frames over total time it would drop fast frames
// from the sum and, above 1000 FPS, keep the readout in warmup forever.
func (a *App) sample(now time.Time, t gui.FrameTimings) {
	last := a.LastFrame
	a.LastFrame = now
	if last.IsZero() {
		return
	}
	// Keep the frame time, not 1/dt. The mean of per-frame FPS values is
	// too high when frame times vary (#1024).
	a.Frames.Add(now.Sub(last))
	a.View.Add(t.ViewGen)
	a.Layout.Add(t.LayoutArrange)
	a.Render.Add(t.RenderBuild)
}

// readout returns the two metrics lines. It rebuilds them at most every
// readoutEvery.
func (a *App) readout(now time.Time) [2]string {
	if a.Frames.Len() < warmupFrames {
		return [2]string{"warming up…", ""}
	}
	if !a.ReadoutAt.IsZero() && now.Sub(a.ReadoutAt) < readoutEvery {
		return a.Readout
	}
	a.ReadoutAt = now
	f := a.Frames.Summary()
	v, l, r := a.View.Summary(), a.Layout.Summary(), a.Render.Summary()
	a.Readout = [2]string{
		fmt.Sprintf("FPS: %5.0f   p50: %5.2f ms   p99: %5.2f ms   max: %6.2f ms   Widgets: %5s",
			f.FPS, framestats.Ms(f.P50), framestats.Ms(f.P99), framestats.Ms(f.Max),
			commaInt(a.WidgetCount)),
		fmt.Sprintf("View: %9s us (p99 %9s)   Layout: %9s us (p99 %9s)   Render: %9s us (p99 %9s)",
			commaFloat(us(v.Mean)), commaFloat(us(v.P99)),
			commaFloat(us(l.Mean)), commaFloat(us(l.P99)),
			commaFloat(us(r.Mean)), commaFloat(us(r.P99))),
	}
	return a.Readout
}

func main() {
	pprofAddr := flag.String("pprof", "", "pprof listen address, e.g. localhost:6060 (empty disables)")
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	if *pprofAddr != "" {
		go func() {
			addr := *pprofAddr
			fmt.Println("pprof: http://" + addr + "/debug/pprof/")
			// #nosec G104,G114 — debug pprof server, intentional
			if err := http.ListenAndServe(addr, nil); err != nil { //nolint:errcheck
				log.Printf("pprof: %v", err)
			}
		}()
	}

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:   &App{WidgetCount: 100, WidgetType: typeButton, Running: true},
		Title:   "Benchmark",
		Width:   1024,
		Height:  768,
		Timings: true,
		OnInit: func(w *gui.Window) {
			w.SetView(benchView)
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

func benchView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	countOptions := []string{"10", "100", "1000", "5000"}
	typeOptions := []string{typeButton, typeText, typeToggle, typeMixed}

	if app.Running {
		// Sample the frame cadence here, in the per-frame path, not
		// from an animation callback: a repeating animation fires on
		// its own ticker (animationCycle, 16 ms — ~62.5 FPS), so it
		// caps the measurement at the ticker's rate. The view runs
		// once per rebuilt frame, so the wall-clock delta between
		// consecutive calls is the true frame time.
		// What FPS actually reports is view-to-view wall time on a
		// pump-forced loop (see InvalidateLayout below): it includes
		// the pump overhead itself, not just widget cost.
		//
		// w.Timings() reports the previous frame's phases: the view
		// runs at the start of the next frame's rebuild, after the
		// last frame's Update stored them.
		app.sample(time.Now(), w.Timings())

		// Re-arm the next frame: the backend loop sleeps while no
		// refresh is pending, so an uncapped pump must request a
		// rebuild every frame. A full layout refresh is required —
		// a render-only refresh never runs the view again, and
		// InvalidateRender from here would stall after one frame.
		// The forced pump is part of what FPS above measures.
		w.InvalidateLayout()
	}

	// Labels are formatted here, on count/type change, so the
	// per-widget loop below measures layout rather than Sprintf.
	if len(app.Labels) != app.WidgetCount || app.LabelsType != app.WidgetType {
		app.Labels = make([]string, app.WidgetCount)
		for i := range app.Labels {
			app.Labels[i] = benchLabel(app.WidgetType, i)
		}
		app.LabelsType = app.WidgetType
	}

	selectedCount := strconv.Itoa(app.WidgetCount)

	var btnLabel string
	if app.Running {
		btnLabel = "Stop"
	} else {
		btnLabel = "Start"
	}

	widgets := make([]gui.View, app.WidgetCount)
	for i := range app.WidgetCount {
		widgets[i] = makeWidget(app.WidgetType, i, app.Labels[i])
	}

	metrics := app.readout(time.Now())

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PaddingSmall,
		Content: []gui.View{
			// Controls row.
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Count:",
						TextStyle: theme.TextStyleTitleSmall,
					}),
					gui.Select(gui.SelectCfg{
						ID:       "bench-count",
						Selected: []string{selectedCount},
						Items:    countOptions,
						OnSelect: func(sel []string, ctx gui.EventCtx) {
							if len(sel) == 0 {
								return
							}
							n, err := strconv.Atoi(sel[0])
							if err != nil {
								return // keep the current selection
							}
							app := gui.State[App](ctx.Window)
							app.WidgetCount = n
							app.ResetAvgs()
						},
					}),
					gui.Text(gui.TextCfg{
						Text:      "Type:",
						TextStyle: theme.TextStyleTitleSmall,
					}),
					gui.Select(gui.SelectCfg{
						ID:       "bench-type",
						Selected: []string{app.WidgetType},
						Items:    typeOptions,
						OnSelect: func(sel []string, ctx gui.EventCtx) {
							if len(sel) > 0 {
								app := gui.State[App](ctx.Window)
								app.WidgetType = sel[0]
								app.ResetAvgs()
							}
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID: "bench_run",
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: btnLabel}),
						},
						OnClick: func(ctx gui.EventCtx) {
							app := gui.State[App](ctx.Window)
							app.Running = !app.Running
							if app.Running {
								// First sample must start fresh, or the
								// idle pause shows as one long frame.
								app.LastFrame = time.Time{}
							} else {
								app.ResetAvgs()
							}
						},
					}),
				},
			}),
			// Metrics rows.
			gui.Text(gui.TextCfg{
				Text:      metrics[0],
				TextStyle: theme.Mono(theme.TextStyleBodySmall),
			}),
			gui.Text(gui.TextCfg{
				Text:      metrics[1],
				TextStyle: theme.Mono(theme.TextStyleBodySmall),
			}),
			// Widget area.
			gui.Column(gui.ContainerCfg{
				ID:         "widget-scroll",
				Sizing:     gui.FillFill,
				Scrollable: true,
				Content: []gui.View{
					gui.Wrap(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Spacing: gui.SpacingSmall,
						Content: widgets,
					}),
				},
			}),
		},
	})
}

func makeWidget(typ string, i int, label string) gui.View {
	switch typ {
	case typeButton:
		return gui.Button(gui.ButtonCfg{
			ID: gui.ScopeIDN("benchmark", "btn", i),
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: label}),
			},
		})
	case typeText:
		return gui.Text(gui.TextCfg{
			Text: label,
		})
	case typeToggle:
		return gui.Toggle(gui.ToggleCfg{
			ID:       gui.ScopeIDN("benchmark", "toggle", i),
			Label:    label,
			Selected: i%2 == 0,
		})
	case typeMixed:
		switch i % 4 {
		case 0:
			return gui.Button(gui.ButtonCfg{
				ID: gui.ScopeIDN("benchmark", "btn", i),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: label}),
				},
			})
		case 1:
			return gui.Text(gui.TextCfg{
				Text: label,
			})
		case 2:
			return gui.Toggle(gui.ToggleCfg{
				ID:       gui.ScopeIDN("benchmark", "toggle", i),
				Label:    label,
				Selected: i%2 == 0,
			})
		default:
			return gui.ProgressBar(gui.ProgressBarCfg{
				ID:       fmt.Sprintf("bench-pb-%d", i),
				Percent:  float32(i%100) / 100.0,
				Width:    80,
				Height:   20,
				TextShow: true,
			})
		}
	default:
		return gui.Text(gui.TextCfg{Text: fmt.Sprintf("? %d", i)})
	}
}

// benchLabel formats one widget's text. Called on count/type change,
// not per frame, so formatting stays out of the layout cost.
func benchLabel(typ string, i int) string {
	switch typ {
	case typeButton:
		return fmt.Sprintf("Btn %d", i)
	case typeText:
		return fmt.Sprintf("Label %d", i)
	case typeToggle:
		return fmt.Sprintf("Opt %d", i)
	case typeMixed:
		switch i % 4 {
		case 0:
			return fmt.Sprintf("Btn %d", i)
		case 1:
			return fmt.Sprintf("Label %d", i)
		case 2:
			return fmt.Sprintf("Opt %d", i)
		}
	}
	return ""
}

// us converts d to microseconds.
func us(d time.Duration) float64 {
	return float64(d) / float64(time.Microsecond)
}

func commaInt(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		return "-" + commaInt(-n)
	}
	if len(s) <= 3 {
		return s
	}
	return commaInt(n/1000) + "," + s[len(s)-3:]
}

func commaFloat(f float64) string {
	if f < 0 {
		return "-" + commaFloat(-f)
	}
	// Round the whole value first, so a fraction that rounds up to 1.0
	// carries into the integer part (12.97 is "13.0", not "12.0").
	s := strconv.FormatFloat(f, 'f', 1, 64)
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return s // NaN or Inf: no decimal point to split at
	}
	whole, err := strconv.Atoi(s[:dot])
	if err != nil {
		return s // too large for int; leave it without commas
	}
	return commaInt(whole) + s[dot:]
}
