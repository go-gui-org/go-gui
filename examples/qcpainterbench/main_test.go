package main

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Tests: testAll, Count: 2, Start: time.Now()},
		Width:  winW,
		Height: winH,
	})
	_ = mainView(w).GenerateLayout(w)
}

func TestSecs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   float64
		want time.Duration
	}{
		{1.5, 1500 * time.Millisecond},
		{-1, 0},
		{math.NaN(), 0},
		{math.Inf(1), maxSeconds * time.Second},
		{1e300, maxSeconds * time.Second},
	} {
		if got := secs(c.in); got != c.want {
			t.Errorf("secs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseCounts(t *testing.T) {
	t.Parallel()
	got, err := parseCounts("1, 2,512")
	if err != nil || len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 512 {
		t.Fatalf("parseCounts = %v, %v", got, err)
	}
	for _, bad := range []string{"", "0", "-1", "x", "1,,2", "4097"} {
		if _, err := parseCounts(bad); err == nil {
			t.Errorf("parseCounts(%q) = nil error", bad)
		}
	}
}

// TestSweepPhases drives a sweep with a fake clock: each count runs warmup, then
// measure, and the sweep reports done after the last count.
func TestSweepPhases(t *testing.T) {
	t.Parallel()
	sw, err := newSweep("1,4", time.Second, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	sw.out = &out
	app := &App{Tests: testDefault}
	now := time.Unix(0, 0)
	frames := 0
	for !sw.step(app, now, gui.FrameTimings{}) {
		if frames > 1000 {
			t.Fatal("sweep did not finish")
		}
		now = now.Add(100 * time.Millisecond)
		frames++
	}
	if app.Count != 4 {
		t.Errorf("last count = %d, want 4", app.Count)
	}
	// Two counts of 1 s warmup + 2 s measured at 100 ms per frame.
	if frames < 55 || frames > 65 {
		t.Errorf("frames = %d, want about 60", frames)
	}
	// One CSV row per count, each with every column of the header.
	rows := strings.Split(strings.TrimSpace(out.String()), "\n")
	cols := strings.Count(csvHeader, ",")
	if len(rows) != 2 || !strings.HasPrefix(rows[0], "1,") ||
		!strings.HasPrefix(rows[1], "4,") {
		t.Fatalf("rows = %q, want one for count 1 then one for count 4", rows)
	}
	for _, r := range rows {
		if strings.Count(r, ",") != cols {
			t.Errorf("row %q has %d commas, want %d", r, strings.Count(r, ","), cols)
		}
	}
}

func TestNewSweepRejects(t *testing.T) {
	t.Parallel()
	if _, err := newSweep("1,x", 0, time.Second); err == nil {
		t.Error("bad count list: nil error")
	}
	if _, err := newSweep("1", 0, 0); err == nil {
		t.Error("zero measured time: nil error")
	}
}

// TestTogglePauseResumes checks that the clock does not jump on resume: the time
// spent paused is added to Start.
func TestTogglePauseResumes(t *testing.T) {
	t.Parallel()
	start := time.Now().Add(-10 * time.Second)
	app := &App{Start: start}
	togglePause(app)
	if !app.Paused {
		t.Fatal("not paused after first toggle")
	}
	app.PausedAt = app.PausedAt.Add(-5 * time.Second) // paused 5 s ago
	togglePause(app)
	if app.Paused {
		t.Fatal("still paused after second toggle")
	}
	if shift := app.Start.Sub(start); shift < 5*time.Second || shift > 6*time.Second {
		t.Errorf("Start moved %v, want about 5s", shift)
	}
}

func TestMeterWindows(t *testing.T) {
	t.Parallel()
	var m meter
	now := time.Unix(0, 0)
	for range 4 * 60 { // 4 s at 60 frames per second
		m.frame(now, gui.FrameTimings{RenderBuild: time.Millisecond})
		now = now.Add(time.Second / 60)
	}
	if m.filled != 1 || m.fps < 59 || m.fps > 61 {
		t.Errorf("filled %d, fps %.1f, want 1 window near 60", m.filled, m.fps)
	}
	if us := m.lastCPU.avgUs(m.lastCPU.render); us != 1000 {
		t.Errorf("render avg = %.0f µs, want 1000", us)
	}
}

// TestSweepStepAfterDone is a regression test. Window.Close is asynchronous, so
// the view can run again after the sweep reports done. That extra step indexed
// past the last render count and panicked.
func TestSweepStepAfterDone(t *testing.T) {
	t.Parallel()
	sw, err := newSweep("1", 0, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	sw.out = io.Discard
	app := &App{}
	now := time.Unix(0, 0)
	for !sw.step(app, now, gui.FrameTimings{}) {
		now = now.Add(100 * time.Millisecond)
	}
	for range 3 {
		if !sw.step(app, now, gui.FrameTimings{}) {
			t.Fatal("step after done = false, want true")
		}
	}
}
