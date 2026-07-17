package main

import (
	"errors"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// seededApp builds app state from a synthetic snapshot so view tests never
// shell out to the real process collector.
func seededApp() *App {
	app := &App{
		Store:    NewProcessStore(),
		Interval: time.Second,
		Sort:     sortState{Column: colCPU, Desc: true},
	}
	snap := &Snapshot{
		Time:             time.Unix(1_700_000_000, 0),
		TotalMemoryBytes: 16 << 30,
		UsedMemoryBytes:  8 << 30,
		Processes: []ProcInfo{
			{PID: 1, PPID: 0, Name: "init", User: "root", State: "S", RSSBytes: 10 << 20, Threads: 1},
			{PID: 100, PPID: 1, Name: "app", User: "me", State: "R", RSSBytes: 200 << 20, CPUPercent: 12.5, MemPercent: 1.2, Threads: 8},
			{PID: 101, PPID: 100, Name: "child", User: "me", State: "S", RSSBytes: 50 << 20, CPUPercent: 1.0, MemPercent: 0.3, Threads: 2},
		},
	}
	app.Store.Update(snap, nil)
	app.Snapshot = snap
	app.LastRefresh = snap.Time

	rows := visibleRows(app.Store.Processes(), "", colDefs[colCPU].less, true, false)
	if len(rows) > 0 {
		app.Selected = rows[0]
	}
	return app
}

func buildView(t *testing.T, app *App) gui.Layout {
	t.Helper()
	w := gui.NewWindow(gui.WindowCfg{State: app, Width: 1100, Height: 700})
	// GenerateViewLayout recurses the whole tree, exercising every cell and
	// chart builder (nil backend: TextMeasurer/SvgParser are nil, as in all
	// headless example tests).
	return gui.GenerateViewLayout(rootView(w), w)
}

func TestRootViewBuildsFlatAndTree(t *testing.T) {
	t.Parallel()
	for _, tree := range []bool{false, true} {
		app := seededApp()
		app.TreeMode = tree
		layout := buildView(t, app)
		if len(layout.Children) == 0 {
			t.Fatalf("expected a populated layout tree (tree=%v)", tree)
		}
	}
}

func TestRootViewEmptyAndErrorStates(t *testing.T) {
	t.Parallel()

	// No snapshot yet: the "collecting" / "waiting" placeholders must build.
	_ = buildView(t, &App{Store: NewProcessStore(), Interval: time.Second})

	// Sample error path.
	_ = buildView(t, &App{
		Store:    NewProcessStore(),
		Interval: time.Second,
		Err:      errors.New("boom"),
	})
}

func TestRootViewFilterNoMatches(t *testing.T) {
	t.Parallel()
	app := seededApp()
	app.Filter = "does-not-exist-xyz"
	_ = buildView(t, app)
}
