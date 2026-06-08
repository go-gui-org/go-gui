// Benchmark measures frame costs while rendering large batches of
// widgets.
package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	animID     = "benchmark-tick"
	bufSize    = 500
	typeButton = "Button"
	typeText   = "Text"
	typeToggle = "Toggle"
	typeMixed  = "Mixed"
)

type RollingAvg struct {
	buf  [bufSize]float64
	pos  int
	full bool
}

func (r *RollingAvg) Add(v float64) {
	r.buf[r.pos] = v
	r.pos++
	if r.pos >= bufSize {
		r.pos = 0
		r.full = true
	}
}

func (r *RollingAvg) Avg() float64 {
	n := r.pos
	if r.full {
		n = bufSize
	}
	if n == 0 {
		return 0
	}
	var sum float64
	for i := range n {
		sum += r.buf[i]
	}
	return sum / float64(n)
}

func (r *RollingAvg) Full() bool { return r.full }

func (r *RollingAvg) Reset() {
	r.pos = 0
	r.full = false
}

type App struct {
	LastFrame   time.Time
	WidgetType  string
	FPS         RollingAvg
	ViewAvg     RollingAvg
	LayoutAvg   RollingAvg
	RenderAvg   RollingAvg
	WidgetCount int
	Running     bool
}

func (a *App) ResetAvgs() {
	a.FPS.Reset()
	a.ViewAvg.Reset()
	a.LayoutAvg.Reset()
	a.RenderAvg.Reset()
	a.LastFrame = time.Time{}
}

func main() {
	pprofPort := flag.String("pprof", "6060", "pprof HTTP server port")
	flag.Parse()

	go func() {
		addr := "localhost:" + *pprofPort
		fmt.Println("pprof: http://" + addr + "/debug/pprof/")
		http.ListenAndServe(addr, nil) //nolint:errcheck,gosec
	}()

	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:   &App{WidgetCount: 100, WidgetType: typeButton, Running: true},
		Title:   "Benchmark",
		Width:   1024,
		Height:  768,
		Timings: true,
		OnInit: func(w *gui.Window) {
			w.UpdateView(benchView)
			startAnimation(w)
		},
	})

	backend.Run(w)
}

func startAnimation(w *gui.Window) {
	gui.State[App](w).LastFrame = time.Now()
	w.AnimationAdd(&gui.Animate{
		AnimID: animID,
		Delay:  0,
		Repeat: true,
		Callback: func(_ *gui.Animate, w *gui.Window) {
			app := gui.State[App](w)
			// Sample timings every frame so the averages track the live view.
			now := time.Now()
			if !app.LastFrame.IsZero() {
				dt := now.Sub(app.LastFrame)
				if dt > time.Millisecond {
					app.FPS.Add(1e9 / float64(dt))
				}
			}
			app.LastFrame = now

			t := w.Timings()
			app.ViewAvg.Add(float64(t.ViewGen.Microseconds()))
			app.LayoutAvg.Add(float64(t.LayoutArrange.Microseconds()))
			app.RenderAvg.Add(float64(t.RenderBuild.Microseconds()))
		},
	})
}

func benchView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	countOptions := []string{"10", "100", "1000", "5000"}
	typeOptions := []string{typeButton, typeText, typeToggle, typeMixed}

	selectedCount := strconv.Itoa(app.WidgetCount)

	var btnLabel string
	if app.Running {
		btnLabel = "Stop"
	} else {
		btnLabel = "Start"
	}

	widgets := make([]gui.View, app.WidgetCount)
	for i := range app.WidgetCount {
		widgets[i] = makeWidget(app.WidgetType, i)
	}

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Padding: gui.SomeP(8, 8, 8, 8),
		Content: []gui.View{
			// Controls row.
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.Some[float32](8),
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Count:",
						TextStyle: theme.B3,
					}),
					gui.Select(gui.SelectCfg{
						ID:       "bench-count",
						Selected: []string{selectedCount},
						Options:  countOptions,
						IDFocus:  1,
						OnSelect: func(sel []string, _ *gui.Event, w *gui.Window) {
							if len(sel) > 0 {
								if n, err := strconv.Atoi(sel[0]); err == nil {
									app := gui.State[App](w)
									app.WidgetCount = n
									app.ResetAvgs()
								}
							}
						},
					}),
					gui.Text(gui.TextCfg{
						Text:      "Type:",
						TextStyle: theme.B3,
					}),
					gui.Select(gui.SelectCfg{
						ID:       "bench-type",
						Selected: []string{app.WidgetType},
						Options:  typeOptions,
						IDFocus:  2,
						OnSelect: func(sel []string, _ *gui.Event, w *gui.Window) {
							if len(sel) > 0 {
								app := gui.State[App](w)
								app.WidgetType = sel[0]
								app.ResetAvgs()
							}
						},
					}),
					gui.Button(gui.ButtonCfg{
						IDFocus: 3,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: btnLabel}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							app := gui.State[App](w)
							app.Running = !app.Running
							if app.Running {
								startAnimation(w)
							} else {
								w.AnimationRemove(animID)
								app.ResetAvgs()
							}
							e.IsHandled = true
						},
					}),
				},
			}),
			// Metrics row.
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("FPS: %5.0f   View: %9s us   Layout: %9s us   Render: %9s us   Widgets: %5s",
					fmtAvg(&app.FPS), commaFloat(fmtAvg(&app.ViewAvg)), commaFloat(fmtAvg(&app.LayoutAvg)),
					commaFloat(fmtAvg(&app.RenderAvg)), commaInt(app.WidgetCount)),
				TextStyle: theme.M4,
			}),
			// Widget area.
			gui.Column(gui.ContainerCfg{
				Sizing:   gui.FillFill,
				IDScroll: 1,
				Content: []gui.View{
					gui.Wrap(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Spacing: gui.Some[float32](4),
						Content: widgets,
					}),
				},
			}),
		},
	})
}

func makeWidget(typ string, i int) gui.View {
	switch typ {
	case typeButton:
		return gui.Button(gui.ButtonCfg{
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: fmt.Sprintf("Btn %d", i)}),
			},
		})
	case typeText:
		return gui.Text(gui.TextCfg{
			Text: fmt.Sprintf("Label %d", i),
		})
	case typeToggle:
		return gui.Toggle(gui.ToggleCfg{
			Label:    fmt.Sprintf("Opt %d", i),
			Selected: i%2 == 0,
		})
	case typeMixed:
		switch i % 4 {
		case 0:
			return gui.Button(gui.ButtonCfg{
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: fmt.Sprintf("Btn %d", i)}),
				},
			})
		case 1:
			return gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("Label %d", i),
			})
		case 2:
			return gui.Toggle(gui.ToggleCfg{
				Label:    fmt.Sprintf("Opt %d", i),
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

func fmtAvg(r *RollingAvg) float64 {
	if !r.Full() {
		return 0
	}
	return r.Avg()
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
	whole := int(f)
	frac := fmt.Sprintf("%.1f", f-float64(whole))
	// frac is "0.X" — take from the decimal point
	return commaInt(whole) + frac[1:]
}
