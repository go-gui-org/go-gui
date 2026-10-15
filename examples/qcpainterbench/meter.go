package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-gui-org/go-gui/examples/internal/framestats"
	"github.com/go-gui-org/go-gui/gui"
)

// meterWindow is how long the meter counts frames before it updates the readout.
// Qt uses 2 s windows and also shows the average of the last 3 windows (6 s).
const (
	meterWindow  = 2 * time.Second
	meterHistory = 3
)

// cpuTimes sums go-gui's CPU phase times over a number of frames.
type cpuTimes struct {
	frames               int
	view, layout, render time.Duration
}

func (c *cpuTimes) add(t gui.FrameTimings) {
	c.frames++
	c.view += t.ViewGen
	c.layout += t.LayoutArrange
	c.render += t.RenderBuild
}

// avgUs returns the mean of d over the counted frames, in microseconds.
func (c *cpuTimes) avgUs(d time.Duration) float64 {
	if c.frames == 0 {
		return 0
	}
	return float64(d.Microseconds()) / float64(c.frames)
}

// meter is the on-screen FPS counter.
//
// It counts view rebuilds, and the benchmark rebuilds the view once per frame
// (see mainView). That is closer to the real frame rate than Qt's counter, which
// counts ticks of a spinner animation.
//
// It also keeps the time of each frame, because the mean FPS hides stutter: one
// 30 ms stall per second hardly moves it (#1024). The readout adds the p99 frame
// time of the last window.
type meter struct {
	winStart time.Time
	last     time.Time // end of the previous frame
	cpu      cpuTimes
	ft       framestats.Stats // frame times of the current window

	fps     float64 // last full window
	history [meterHistory]float64
	filled  int // windows in history, up to meterHistory
	next    int // history slot the next window goes in
	lastCPU cpuTimes
	lastFT  framestats.Summary
}

// frame counts one frame. t is go-gui's timing of the previous frame: the view
// runs before the current frame's own phases finish.
func (m *meter) frame(now time.Time, t gui.FrameTimings) {
	if m.winStart.IsZero() {
		m.winStart, m.last = now, now
		return
	}
	m.cpu.add(t)
	m.ft.Add(now.Sub(m.last))
	m.last = now
	el := now.Sub(m.winStart)
	if el < meterWindow {
		return
	}
	m.fps = float64(m.cpu.frames) / el.Seconds()
	m.history[m.next] = m.fps
	m.next = (m.next + 1) % meterHistory
	m.filled = min(m.filled+1, meterHistory)
	m.lastCPU = m.cpu
	m.cpu = cpuTimes{}
	// Summary sorts the window's frame times, once per window.
	m.lastFT = m.ft.Summary()
	m.ft.Reset()
	m.winStart = now
}

// avg is the mean FPS of the windows in history.
func (m *meter) avg() float64 {
	if m.filled == 0 {
		return 0
	}
	var sum float64
	for _, v := range m.history[:m.filled] {
		sum += v
	}
	return sum / float64(m.filled)
}

// reset drops everything counted so far. A change of render count or tests
// starts a new measurement.
func (m *meter) reset() { *m = meter{} }

// String is the readout: the 6 s average, the last 2 s window, go-gui's mean
// CPU time per frame for building the render commands, and the p99 frame time
// of the last window. The part up to "fps" is Qt's format.
func (m *meter) String() string {
	if m.filled == 0 {
		return "measuring…"
	}
	s := fmt.Sprintf("Ø %.0f | %.0f fps  render %.0f µs  p99 %.1f ms",
		m.avg(), m.fps, m.lastCPU.avgUs(m.lastCPU.render), framestats.Ms(m.lastFT.P99))
	if m.lastFT.Frames < m.lastCPU.frames {
		// Above about 2048 FPS the ring holds only the end of the 2 s window.
		s += fmt.Sprintf(" (last %d frames)", m.lastFT.Frames)
	}
	return s
}

// csvHeader names the columns of a sweep row. frame_ms is 1000 / fps, a mean.
// p50_ms, p99_ms and max_ms come from the time of each measured frame (#1024).
// New columns go at the end, so scripts that read the older ones still work.
const csvHeader = "count,tests,frames,seconds,fps,frame_ms,view_us,layout_us,render_us," +
	"p50_ms,p99_ms,max_ms"

// sweep runs the benchmark at each render count in turn and prints one CSV row
// per count. Each count first runs for warmup, unmeasured, so caches and buffers
// reach steady state. It then runs for measure.
type sweep struct {
	out             io.Writer // where rows go; os.Stdout outside tests
	errOut          io.Writer // where notes go; os.Stderr outside tests
	counts          []int
	warmup, measure time.Duration

	idx        int
	phaseStart time.Time // start of the current count, warmup included
	measStart  time.Time // start of the measured part; zero during warmup
	last       time.Time // end of the previous frame
	cpu        cpuTimes
	ft         framestats.Stats // frame times of the measured part
}

func newSweep(list string, warmup, measure time.Duration) (*sweep, error) {
	counts, err := parseCounts(list)
	if err != nil {
		return nil, err
	}
	if measure <= 0 {
		return nil, errors.New("measured seconds must be positive")
	}
	return &sweep{out: os.Stdout, errOut: os.Stderr, counts: counts, warmup: max(warmup, 0), measure: measure}, nil
}

// step advances the sweep by one frame. It sets app.Count and returns true when
// every count has run.
func (s *sweep) step(app *App, now time.Time, t gui.FrameTimings) bool {
	// Window.Close is asynchronous: the view can run again after the sweep has
	// reported done. Keep reporting done.
	if s.idx >= len(s.counts) {
		return true
	}
	prev := s.last
	s.last = now
	if s.phaseStart.IsZero() {
		s.phaseStart = now
		app.Count = s.counts[s.idx]
		return false
	}
	if s.measStart.IsZero() {
		if now.Sub(s.phaseStart) >= s.warmup {
			s.measStart = now
			s.cpu = cpuTimes{}
			s.ft.Reset()
		}
		return false
	}
	s.cpu.add(t)
	s.ft.Add(now.Sub(prev))
	el := now.Sub(s.measStart)
	if el < s.measure {
		return false
	}
	fps := float64(s.cpu.frames) / el.Seconds()
	ft := s.ft.Summary()
	if ft.Frames < s.cpu.frames {
		// The ring keeps the last framestats.Size frames. Above about 800 FPS
		// over 5 s the percentile columns cover only the end of the run. Say
		// so on stderr, so the CSV on stdout keeps its columns.
		_, _ = fmt.Fprintf(s.errOut, "count %d: p50/p99/max cover the last %d of %d frames\n",
			app.Count, ft.Frames, s.cpu.frames)
	}
	// A failed write to stdout has nowhere better to be reported.
	_, _ = fmt.Fprintf(s.out, "%d,%d,%d,%.2f,%.1f,%.3f,%.0f,%.0f,%.0f,%.3f,%.3f,%.3f\n",
		app.Count, app.Tests, s.cpu.frames, el.Seconds(), fps, 1000/fps,
		s.cpu.avgUs(s.cpu.view), s.cpu.avgUs(s.cpu.layout),
		s.cpu.avgUs(s.cpu.render),
		framestats.Ms(ft.P50), framestats.Ms(ft.P99), framestats.Ms(ft.Max))

	s.idx++
	s.phaseStart, s.measStart = time.Time{}, time.Time{}
	if s.idx == len(s.counts) {
		return true
	}
	s.phaseStart = now
	app.Count = s.counts[s.idx]
	return false
}
