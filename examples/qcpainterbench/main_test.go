package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/examples/internal/framestats"
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
		// Every frame is 100 ms, so p50, p99 and max are all 100 ms.
		if !strings.HasSuffix(r, ",100.000,100.000,100.000") {
			t.Errorf("row %q, want p50/p99/max of 100 ms at the end", r)
		}
	}
}

// TestCSVHeaderColumns pins the column order (#1024). The percentile columns go
// at the end, so scripts that read the older columns still work.
func TestCSVHeaderColumns(t *testing.T) {
	t.Parallel()
	const old = "count,tests,frames,seconds,fps,frame_ms,view_us,layout_us,render_us"
	if want := old + ",p50_ms,p99_ms,max_ms"; csvHeader != want {
		t.Errorf("csvHeader = %q, want %q", csvHeader, want)
	}
}

// TestSweepStall checks that long frames in a measured phase show in p99 and
// max, while p50 stays at the common frame time.
func TestSweepStall(t *testing.T) {
	t.Parallel()
	sw, err := newSweep("1", 0, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	sw.out = &out
	app := &App{}
	now := time.Unix(0, 0)
	sw.step(app, now, gui.FrameTimings{}) // starts the count
	sw.step(app, now, gui.FrameTimings{}) // ends the zero warmup
	// Frames of 4 ms, with 30 ms stalls at frames 50, 100 and 150, until the
	// second is over: about 231 frames. Nearest-rank p99 is the 229th smallest,
	// so 3 stalls are enough to set it.
	frames := 0
	for {
		dt := 4 * time.Millisecond
		if frames == 50 || frames == 100 || frames == 150 {
			dt = 30 * time.Millisecond
		}
		now = now.Add(dt)
		frames++
		if sw.step(app, now, gui.FrameTimings{}) {
			break
		}
	}
	f := strings.Split(strings.TrimSpace(out.String()), ",")
	if len(f) != strings.Count(csvHeader, ",")+1 {
		t.Fatalf("row %q: wrong column count", out.String())
	}
	if p50, p99, mx := f[len(f)-3], f[len(f)-2], f[len(f)-1]; p50 != "4.000" ||
		p99 != "30.000" || mx != "30.000" {
		t.Errorf("p50 = %s, p99 = %s, max = %s, want 4.000, 30.000, 30.000", p50, p99, mx)
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
	// The readout keeps Qt's format first, then the p99 frame time (#1024).
	if got, want := m.String(), "  p99 16.7 ms"; !strings.HasPrefix(got, "Ø 60 | 60 fps") ||
		!strings.HasSuffix(got, want) {
		t.Errorf("String = %q, want Qt's format then %q", got, want)
	}
}

// TestMeterStall checks that a 30 ms frame in every 50 shows as p99.
func TestMeterStall(t *testing.T) {
	t.Parallel()
	var m meter
	now := time.Unix(0, 0)
	m.frame(now, gui.FrameTimings{})
	for i := 0; m.filled == 0; i++ {
		dt := 4 * time.Millisecond
		if i%50 == 7 {
			dt = 30 * time.Millisecond
		}
		now = now.Add(dt)
		m.frame(now, gui.FrameTimings{})
	}
	if got := m.String(); !strings.HasSuffix(got, "p99 30.0 ms") {
		t.Errorf("String = %q, want p99 30.0 ms", got)
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

// TestSweepWarmupNotMeasured checks that a long frame during warmup does not
// show in the percentile columns: the ring is reset when measuring starts.
func TestSweepWarmupNotMeasured(t *testing.T) {
	t.Parallel()
	sw, err := newSweep("1", time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	sw.out = &out
	app := &App{}
	now := time.Unix(0, 0)
	sw.step(app, now, gui.FrameTimings{}) // starts the count
	now = now.Add(500 * time.Millisecond) // a long warmup frame
	for !sw.step(app, now, gui.FrameTimings{}) {
		now = now.Add(4 * time.Millisecond)
	}
	if !strings.HasSuffix(strings.TrimSpace(out.String()), ",4.000,4.000,4.000") {
		t.Errorf("row %q, want p50/p99/max of 4 ms", out.String())
	}
}

// TestSweepRingLimit checks that a measured run with more frames than the ring
// keeps writes a note to errOut, and keeps the CSV row unchanged in shape.
func TestSweepRingLimit(t *testing.T) {
	t.Parallel()
	sw, err := newSweep("1", 0, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	sw.out, sw.errOut = &out, &errOut
	app := &App{}
	now := time.Unix(0, 0)
	for !sw.step(app, now, gui.FrameTimings{}) { // 5000 FPS
		now = now.Add(200 * time.Microsecond)
	}
	want := fmt.Sprintf("p50/p99/max cover the last %d of", framestats.Size)
	if !strings.Contains(errOut.String(), want) {
		t.Errorf("errOut = %q, want %q", errOut.String(), want)
	}
	if got := strings.Count(strings.TrimSpace(out.String()), ","); got != strings.Count(csvHeader, ",") {
		t.Errorf("row %q: %d commas, want %d", out.String(), got, strings.Count(csvHeader, ","))
	}
}

// TestMeterRingLimit checks that the readout says when the 2 s window has more
// frames than the ring keeps, so p99 covers only part of it.
func TestMeterRingLimit(t *testing.T) {
	t.Parallel()
	var m meter
	now := time.Unix(0, 0)
	for m.filled == 0 { // 4000 FPS: 8000 frames in the window
		m.frame(now, gui.FrameTimings{})
		now = now.Add(250 * time.Microsecond)
	}
	want := fmt.Sprintf(" (last %d frames)", framestats.Size)
	if got := m.String(); !strings.HasSuffix(got, want) {
		t.Errorf("String = %q, want suffix %q", got, want)
	}
	// Below the limit the readout has no note.
	var ok meter
	now = time.Unix(0, 0)
	for ok.filled == 0 {
		ok.frame(now, gui.FrameTimings{})
		now = now.Add(4 * time.Millisecond)
	}
	if got := ok.String(); strings.Contains(got, "last") {
		t.Errorf("String = %q, want no ring note at 250 FPS", got)
	}
}
