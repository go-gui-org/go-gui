package main

import (
	"flag"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// sortState is threaded through the table header and the row ordering so a
// header click and the -sort flag pick the same column.
type sortState struct {
	Column int
	Desc   bool
}

// App is the window's typed state slot. The sampler goroutine writes it under
// the window lock; the view reads it (the view already runs under that lock).
type App struct {
	Snapshot    *Snapshot
	Err         error
	Store       *ProcessStore
	Selected    *Process
	LastRefresh time.Time
	Filter      string
	Interval    time.Duration
	Sort        sortState
	TreeMode    bool
}

// Sample interval presets shown by the toolbar radio group.
var intervalLabels = []string{"0.5s", "1s", "2s", "5s"}

func main() {
	once := flag.Bool("once", false, "print one terminal report and exit instead of opening the GUI")
	limit := flag.Int("limit", 10, "number of processes to print in -once mode")
	sortBy := flag.String("sort", "cpu", "sort column: cpu, mem, pid, name, user, state, threads")
	refresh := flag.Duration("refresh", time.Second, "GUI sample interval (snapped to 0.5s/1s/2s/5s)")
	flag.Parse()

	if *once {
		runOnce(*limit, *sortBy)
		return
	}

	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	col := sortColumnIndex(*sortBy)
	state := &App{
		Store:    NewProcessStore(),
		Interval: snapInterval(*refresh),
		Sort:     sortState{Column: col, Desc: colDefs[col].desc},
	}

	w := gui.NewWindow(gui.WindowCfg{
		State:  state,
		Title:  "Process Monitor",
		Width:  1100,
		Height: 700,
		OnInit: func(w *gui.Window) {
			// Register the view once; the sampler re-runs it via UpdateWindow.
			w.UpdateView(rootView)
			startSampler(w)
		},
	})

	backend.Run(w)
}

// startSampler launches the background sampling loop. Sampling spawns a
// subprocess (ps/tasklist), so it must run off the frame thread. Each pass
// takes a snapshot, then publishes it under the window lock and requests a
// layout refresh; the backend's idle poll repaints within ~100ms.
func startSampler(w *gui.Window) {
	go func() {
		for {
			snap, err := Collect()

			w.Lock()
			app := gui.State[App](w)
			app.Snapshot = snap
			app.Err = err
			if snap != nil {
				app.LastRefresh = snap.Time
				app.Store.Update(snap, app.Selected)
				// Auto-select the top row on the first successful sample.
				if app.Selected == nil {
					less := colDefs[app.Sort.Column].less
					rows := visibleRows(app.Store.Processes(), "", less, app.Sort.Desc, false)
					if len(rows) > 0 {
						app.Selected = rows[0]
					}
				}
			}
			interval := app.Interval
			// UpdateWindow (not UpdateView) re-runs the view against fresh state
			// WITHOUT clearing the state registry, so the filter input keeps
			// focus and the process list keeps its scroll position.
			w.UpdateWindow()
			w.Unlock()

			if interval <= 0 {
				interval = time.Second
			}
			time.Sleep(interval)
		}
	}()
}

// sortColumnIndex maps a -sort flag value to a column index.
func sortColumnIndex(key string) int {
	switch key {
	case "pid":
		return colPID
	case "mem", "rss":
		return colRSS
	case "user":
		return colUser
	case "state":
		return colState
	case "threads", "thr":
		return colThreads
	case "name":
		return colName
	default: // "cpu" and anything unrecognized
		return colCPU
	}
}

// snapInterval rounds an arbitrary duration to the nearest toolbar preset so
// the radio selection always matches the running interval.
func snapInterval(d time.Duration) time.Duration {
	presets := []time.Duration{
		500 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second,
	}
	if d <= 0 {
		return time.Second
	}
	best, bestDiff := presets[0], absDuration(d-presets[0])
	for _, p := range presets[1:] {
		if diff := absDuration(d - p); diff < bestDiff {
			best, bestDiff = p, diff
		}
	}
	return best
}

// intervalLabel renders a duration as its preset label.
func intervalLabel(d time.Duration) string {
	switch d {
	case 500 * time.Millisecond:
		return "0.5s"
	case 2 * time.Second:
		return "2s"
	case 5 * time.Second:
		return "5s"
	default:
		return "1s"
	}
}

// intervalFromLabel is the inverse of intervalLabel.
func intervalFromLabel(s string) time.Duration {
	switch s {
	case "0.5s":
		return 500 * time.Millisecond
	case "2s":
		return 2 * time.Second
	case "5s":
		return 5 * time.Second
	default:
		return time.Second
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
