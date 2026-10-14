package main

import (
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// Not parallel: SetTheme writes global theme state that building a
// layout reads, and the layout tests below share it.
func TestMainViewNoPanic(t *testing.T) {
	gui.SetTheme(gui.ThemeDark)
	app := newBenchApp(func(string) string { return "" })
	app.Rows = 3
	w := gui.NewWindow(gui.WindowCfg{State: app, Width: 1000, Height: 700})
	_ = mainView(w).GenerateLayout(w)
}

func TestNewBenchAppParams(t *testing.T) {
	t.Parallel()
	vals := map[string]string{"rows": "7", "cols": "x", "asset": "tiger"}
	app := newBenchApp(func(k string) string { return vals[k] })
	if app.Rows != 7 || app.Cols != 6 || len(app.Assets) != 1 {
		t.Fatalf("got rows=%d cols=%d assets=%d, want 7, 6 (default), 1",
			app.Rows, app.Cols, len(app.Assets))
	}
}

// Not parallel: it builds a layout, which reads the global theme.
func TestBenchTransformParam(t *testing.T) {
	for _, name := range []string{"rotate", "tilt", "persp", "lift"} {
		if benchTransform(name).IsIdentity() {
			t.Errorf("xf=%s gave the identity", name)
		}
	}
	if !benchTransform("").IsIdentity() || !benchTransform("bogus").IsIdentity() {
		t.Error("an unset or unknown xf must leave cells untransformed")
	}
	app := newBenchApp(func(k string) string { return map[string]string{"xf": "persp", "rows": "2"}[k] })
	w := gui.NewWindow(gui.WindowCfg{State: app, Width: 1000, Height: 700})
	_ = mainView(w).GenerateLayout(w)
}

func TestScrollPctTriangleWave(t *testing.T) {
	t.Parallel()
	p := 8 * time.Second
	for _, tc := range []struct {
		at   time.Duration
		want float32
	}{
		{0, 0},
		{2 * time.Second, 0.5},
		{4 * time.Second, 1},
		{6 * time.Second, 0.5},
		{8 * time.Second, 0},
	} {
		if got := scrollPct(tc.at, p); got != tc.want {
			t.Errorf("scrollPct(%v) = %v, want %v", tc.at, got, tc.want)
		}
	}
	if got := scrollPct(time.Second, 0); got != 0 {
		t.Errorf("zero period = %v, want 0", got)
	}
}
