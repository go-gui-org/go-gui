package main

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func TestViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{WidgetCount: 100, WidgetType: typeButton, Running: true},
		Width:  1024,
		Height: 768,
	})
	_ = benchView(w).GenerateLayout(w)
}

// TestReadoutFPSIsFramesOverTime is the regression test for #1024. Frames of 2 ms
// and 30 ms alternate. The mean of per-frame FPS values reads about 266. The
// true rate is 2 frames per 32 ms = 62.5 FPS.
func TestReadoutFPSIsFramesOverTime(t *testing.T) {
	t.Parallel()
	app := &App{WidgetCount: 100}
	now := time.Unix(0, 0)
	if got := app.readout(now)[0]; got != "warming up…" {
		t.Errorf("before warmup: %q, want warming up…", got)
	}
	app.sample(now, gui.FrameTimings{})
	for i := range warmupFrames {
		dt := 2 * time.Millisecond
		if i%2 == 1 {
			dt = 30 * time.Millisecond
		}
		now = now.Add(dt)
		app.sample(now, gui.FrameTimings{RenderBuild: time.Millisecond})
	}
	got := app.readout(now)
	if !strings.HasPrefix(got[0], "FPS:    62 ") {
		t.Errorf("line 1 = %q, want FPS 62", got[0])
	}
	for _, want := range []string{"p50:  2.00 ms", "p99: 30.00 ms", "max:  30.00 ms"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("line 1 = %q, want %q", got[0], want)
		}
	}
	if !strings.Contains(got[1], "Render:   1,000.0 us (p99   1,000.0)") {
		t.Errorf("line 2 = %q, want render mean and p99 1,000.0", got[1])
	}
}

// TestReadoutThrottled checks that the readout is not rebuilt on each frame.
func TestReadoutThrottled(t *testing.T) {
	t.Parallel()
	app := &App{}
	now := time.Unix(0, 0)
	for range warmupFrames + 1 {
		app.sample(now, gui.FrameTimings{})
		now = now.Add(4 * time.Millisecond)
	}
	first := app.readout(now)
	app.sample(now.Add(100*time.Millisecond), gui.FrameTimings{}) // a long frame
	if got := app.readout(now.Add(100 * time.Millisecond)); got != first {
		t.Errorf("readout changed within readoutEvery: %q", got[0])
	}
	if got := app.readout(now.Add(readoutEvery)); got == first {
		t.Error("readout not rebuilt after readoutEvery")
	}
}

// TestCommaFloatCarry is a regression test: a fraction that rounds up to 1.0
// lost its carry, so 12.97 printed as "12.0".
func TestCommaFloatCarry(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   float64
		want string
	}{
		{12.97, "13.0"},
		{999.96, "1,000.0"},
		{1234.5, "1,234.5"},
		{0.04, "0.0"},
		{-1234.56, "-1,234.6"},
		{math.Inf(1), "+Inf"},
		{math.NaN(), "NaN"},
	} {
		if got := commaFloat(c.in); got != c.want {
			t.Errorf("commaFloat(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSampleKeepsFastFrames is a regression test: frames of 1 ms or less were
// dropped, so above 1000 FPS the readout never left warmup, and the dropped
// time was lost from the FPS sum.
func TestSampleKeepsFastFrames(t *testing.T) {
	t.Parallel()
	app := &App{}
	now := time.Unix(0, 0)
	app.sample(now, gui.FrameTimings{})
	for range warmupFrames {
		now = now.Add(500 * time.Microsecond) // 2000 FPS
		app.sample(now, gui.FrameTimings{RenderBuild: time.Microsecond})
	}
	if app.Frames.Len() != warmupFrames || app.Render.Len() != warmupFrames {
		t.Errorf("Frames.Len = %d, Render.Len = %d, want %d each",
			app.Frames.Len(), app.Render.Len(), warmupFrames)
	}
	if got := app.readout(now)[0]; !strings.HasPrefix(got, "FPS:  2000 ") {
		t.Errorf("line 1 = %q, want FPS 2000", got)
	}
}

// TestResetAvgs checks that a reset drops every ring and the cached readout.
func TestResetAvgs(t *testing.T) {
	t.Parallel()
	app := &App{}
	now := time.Unix(0, 0)
	for range warmupFrames + 1 {
		app.sample(now, gui.FrameTimings{})
		now = now.Add(4 * time.Millisecond)
	}
	_ = app.readout(now)
	app.ResetAvgs()
	if app.Frames.Len()+app.View.Len()+app.Layout.Len()+app.Render.Len() != 0 {
		t.Error("rings not empty after ResetAvgs")
	}
	if !app.LastFrame.IsZero() || !app.ReadoutAt.IsZero() || app.Readout != ([2]string{}) {
		t.Error("LastFrame or readout cache not cleared")
	}
	if got := app.readout(now)[0]; got != "warming up…" {
		t.Errorf("readout after reset = %q, want warming up…", got)
	}
}

// TestTimedRunWritesOneRow drives a timed run with a fake clock: no row in
// warmup, no row before the measured time, then exactly one row, and the
// run keeps reporting done after it (Window.Close is asynchronous).
func TestTimedRunWritesOneRow(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	app := &App{WidgetCount: 5000, WidgetType: typeMixed}
	app.run = &run{out: &out, warmup: time.Second, measure: time.Second}
	now := time.Unix(0, 0)
	frames := 0
	for !app.run.step(app, now) {
		app.sample(now, gui.FrameTimings{RenderBuild: 2 * time.Millisecond})
		now = now.Add(10 * time.Millisecond)
		frames++
		if frames > 1000 {
			t.Fatal("run never finished")
		}
	}
	if !app.run.step(app, now) {
		t.Fatal("step after done returned false")
	}
	row := out.String()
	if strings.Count(row, "\n") != 1 {
		t.Fatalf("want one row, got %q", row)
	}
	cols := strings.Split(strings.TrimSpace(row), ",")
	if want := strings.Count(runCSVHeader, ",") + 1; len(cols) != want {
		t.Fatalf("row has %d columns, header %d: %q", len(cols), want, row)
	}
	if cols[0] != typeMixed || cols[1] != "5000" || cols[4] != "100.0" ||
		cols[5] != "10.000" || cols[10] != "2000.0" {
		t.Errorf("row = %q, want Mixed,5000,…,fps 100.0,p50 10.000,…,render 2000.0", row)
	}
}

func TestValidType(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{typeButton, typeText, typeToggle, typeMixed} {
		if !validType(typ) {
			t.Errorf("validType(%q) = false", typ)
		}
	}
	if validType("button") {
		t.Error(`validType("button") = true; types are case-sensitive`)
	}
}
