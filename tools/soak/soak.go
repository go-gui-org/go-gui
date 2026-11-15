// Package soak runs a headless Go-Gui window through a long simulated
// user session and measures allocations per frame and the live heap
// (#1262).
//
// A soak run acts as a user. It clicks, types accented text, emoji and
// Arabic, presses Escape, Tab and arrow keys, scrolls, resizes the
// window, switches the theme and sometimes waits. Each frame advances
// the window clock by 1/60 s with [gui.Window.TestStep], so ten
// simulated minutes take only as much wall time as the frames cost,
// and a seed replays the same session. Every RenderEvery frames the
// frame is drawn with the software renderer, so the render path runs
// too, not only layout.
//
// At the end, [Report] gives allocations per frame (mean and p99), the
// heap at the start and end, and the slope of the heap over time.
// [Report.Check] turns those into a pass or fail against [Gates].
//
// A second mode, [ModeRandom], sends fully random raw events (mouse,
// key, text, touch, resize) to find panics, for example a State[T]
// type mismatch.
//
// The package drives a window that the caller builds. An app in
// package main cannot be imported, so its own test calls [Run]; see
// examples/showcase/soak_test.go.
package soak

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// FrameDt is the simulated time of one frame: one display refresh at
// 60 Hz.
const FrameDt = time.Second / 60

// Mode selects how a run makes input.
type Mode int

const (
	// ModeScripted acts as a user: it clicks, types, presses keys and
	// scrolls on widgets the current layout contains. Allocation and
	// heap numbers come from this mode.
	ModeScripted Mode = iota
	// ModeRandom sends fully random raw events to Window.EventFn. Its
	// purpose is to find panics; its allocation numbers mean little.
	ModeRandom
)

// String returns the mode name used by the -soak.mode flag.
func (m Mode) String() string {
	if m == ModeRandom {
		return "random"
	}
	return "scripted"
}

// ParseMode is the inverse of [Mode.String].
func ParseMode(s string) (Mode, error) {
	switch s {
	case "scripted", "":
		return ModeScripted, nil
	case "random":
		return ModeRandom, nil
	}
	return 0, fmt.Errorf("soak: unknown mode %q (want scripted or random)", s)
}

// Config sets up a run. The zero value is a ten-minute scripted run
// with a time-based seed.
type Config struct {
	// Seed for the random source. Zero picks one from the wall clock.
	// The report always carries the seed in use, so a failure can be
	// repeated.
	Seed uint64
	// Duration is the simulated length of the run. Default 10 minutes.
	Duration time.Duration
	// Warmup is the simulated time before measuring starts: caches
	// fill and the first visits to each page allocate their state.
	// Default 60 s. When it is not shorter than Duration, a fifth of
	// Duration is used, so a short run still measures something.
	Warmup time.Duration
	// Mode selects scripted or random input.
	Mode Mode
	// Scale is the device pixel ratio for the software renders.
	// Default 1.
	Scale float32
	// RenderEvery is the number of frames between software renders.
	// Default 60 (one each simulated second). The soft renderer
	// allocates its own image each call, which a GPU backend does not,
	// so render frames are not in the allocation counts.
	RenderEvery int
	// SkipID reports IDs the scripted actions must not click, type into
	// or scroll: widgets that open the network, play audio or open
	// native dialogs. Nil skips nothing. Random mode does not use it: it
	// sends raw events at random coordinates and can reach any widget.
	SkipID func(id string) bool
	// Logf receives progress lines. Nil discards them.
	Logf func(format string, args ...any)
	// OnAction, when set, receives each action as the run takes it.
	// Tests use it to compare two runs with the same seed.
	OnAction func(frame int, action string)
}

// heapSamples is the target number of heap samples after warm-up. More
// samples make the slope steadier; each costs a full GC.
const heapSamples = 60

// heapWindows is the number of time windows whose lowest heap sample
// gives Report.HeapSlope.
const heapWindows = 8

// maxHeapSampleEvery caps the interval between heap samples.
const maxHeapSampleEvery = 10 * time.Second

// lastActions is how many actions a panic report keeps.
const lastActions = 20

func (c Config) withDefaults() Config {
	if c.Seed == 0 {
		c.Seed = uint64(time.Now().UnixNano())
	}
	if c.Duration <= 0 {
		c.Duration = 10 * time.Minute
	}
	if c.Warmup <= 0 {
		c.Warmup = 60 * time.Second
	}
	if c.Warmup >= c.Duration {
		c.Warmup = c.Duration / 5
	}
	if c.Scale <= 0 {
		c.Scale = 1
	}
	if c.RenderEvery <= 0 {
		c.RenderEvery = 60
	}
	if c.SkipID == nil {
		c.SkipID = func(string) bool { return false }
	}
	if c.Logf == nil {
		c.Logf = func(string, ...any) {}
	}
	return c
}

// Panic is one panic a run caught. The run stops at the first one: the
// window may hold a lock or half-built state after it.
type Panic struct {
	Frame       int
	Value       string
	Stack       string
	LastActions []string
}

// Report is the result of a run.
type Report struct {
	Seed   uint64
	Mode   Mode
	Frames int // frames stepped, warm-up included
	// Measured is the number of frames after warm-up whose allocations
	// were counted.
	Measured int
	// AllocsMean and AllocsP99 are heap allocations (mallocs) per
	// TestStep frame after warm-up. Input dispatch and software renders
	// are not counted here.
	AllocsMean float64
	AllocsP99  float64
	// EventAllocsMean is the mean of mallocs per input action after
	// warm-up, the Test* call and the frame it settles. Advisory.
	EventAllocsMean float64
	// SessionAllocsPerFrame is every malloc after warm-up (frames and
	// input actions) divided by the measured frames: the cost of the
	// session per frame shown, the number CUI's soak reports. A Test*
	// action settles its own frame, so a quiet session has a mean of
	// near 0 in AllocsMean and its real cost here.
	SessionAllocsPerFrame float64
	// HeapStart and HeapEnd are the live heap (bytes, after a GC) at the
	// first and last sample after warm-up.
	HeapStart, HeapEnd uint64
	// HeapSlope is the growth of the heap floor after warm-up, in bytes
	// per simulated minute: the samples are split into heapWindows
	// windows of time, and HeapSlope is the least-squares slope of the
	// lowest sample in each. The live heap of an app moves with the
	// page on screen (a demo with images holds more than one with
	// text), and that noise is larger than a slow leak in the raw
	// samples. Page state adds peaks; only kept memory raises the floor.
	HeapSlope float64
	// HeapSlopeRaw is the least-squares slope over every sample, in
	// bytes per simulated minute. Advisory: it includes the page noise.
	HeapSlopeRaw float64
	// HeapSamples is the number of heap samples the slope uses.
	HeapSamples int
	// Actions is the number of input actions taken.
	Actions int
	// RenderErrors counts software renders that failed (for example a
	// random resize to zero size). They are not failures of the run.
	RenderErrors int
	// Wall is the wall time of the run. Advisory: it moves with
	// machine load.
	Wall time.Duration
	// Panic is the first panic, or nil.
	Panic *Panic
}

// String formats the report for a log.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "soak %s seed=%d frames=%d measured=%d actions=%d wall=%s\n",
		r.Mode, r.Seed, r.Frames, r.Measured, r.Actions, r.Wall.Round(time.Millisecond))
	fmt.Fprintf(&b, "  allocs/frame mean=%.3f p99=%.0f  allocs/action mean=%.1f  session allocs/frame=%.1f\n",
		r.AllocsMean, r.AllocsP99, r.EventAllocsMean, r.SessionAllocsPerFrame)
	fmt.Fprintf(&b, "  heap start=%s end=%s floor slope=%s/min raw slope=%s/min (%d samples)",
		kib(float64(r.HeapStart)), kib(float64(r.HeapEnd)), kib(r.HeapSlope),
		kib(r.HeapSlopeRaw), r.HeapSamples)
	if r.RenderErrors > 0 {
		fmt.Fprintf(&b, "\n  render errors=%d", r.RenderErrors)
	}
	if r.Panic != nil {
		fmt.Fprintf(&b, "\n  PANIC at frame %d: %s\n  last actions:\n    %s\n%s",
			r.Panic.Frame, r.Panic.Value,
			strings.Join(r.Panic.LastActions, "\n    "), r.Panic.Stack)
	}
	return b.String()
}

func kib(v float64) string { return fmt.Sprintf("%.1fKiB", v/1024) }

// Gates are the limits [Report.Check] applies. A zero limit is off.
type Gates struct {
	// MaxHeapSlope is the largest allowed heap slope after warm-up, in
	// bytes per simulated minute.
	MaxHeapSlope float64
	// MaxAllocsMean is the largest allowed mean of mallocs per frame
	// (Report.AllocsMean).
	MaxAllocsMean float64
	// MaxSessionAllocsPerFrame is the largest allowed
	// Report.SessionAllocsPerFrame.
	MaxSessionAllocsPerFrame float64
}

// ErrGate is wrapped by every gate failure [Report.Check] returns.
var ErrGate = errors.New("soak: gate failed")

// Check returns nil when the run passed: no panic, and every gate in g
// that is set holds. Allocation gates are hard; wall time is never
// checked.
func (r Report) Check(g Gates) error {
	var fails []string
	if r.Panic != nil {
		fails = append(fails, fmt.Sprintf("panic at frame %d: %s", r.Panic.Frame, r.Panic.Value))
	}
	if g.MaxHeapSlope > 0 && r.HeapSamples >= 2 && r.HeapSlope > g.MaxHeapSlope {
		fails = append(fails, fmt.Sprintf("heap floor slope %s/min > %s/min",
			kib(r.HeapSlope), kib(g.MaxHeapSlope)))
	}
	if g.MaxAllocsMean > 0 && r.Measured > 0 && r.AllocsMean > g.MaxAllocsMean {
		fails = append(fails, fmt.Sprintf("allocs/frame mean %.3f > %.3f",
			r.AllocsMean, g.MaxAllocsMean))
	}
	if g.MaxSessionAllocsPerFrame > 0 && r.Measured > 0 &&
		r.SessionAllocsPerFrame > g.MaxSessionAllocsPerFrame {
		fails = append(fails, fmt.Sprintf("session allocs/frame %.1f > %.1f",
			r.SessionAllocsPerFrame, g.MaxSessionAllocsPerFrame))
	}
	if len(fails) == 0 {
		return nil
	}
	return fmt.Errorf("%w (seed %d): %s", ErrGate, r.Seed, strings.Join(fails, "; "))
}

// runner holds the state of one run.
type runner struct {
	w   *gui.Window
	cfg Config
	rng *rand.Rand

	frame      int // frames stepped so far
	frames     int // frames in the run
	warmFrames int
	sampleEach int // frames between heap samples

	allocs      []uint64 // per measured frame
	eventAllocs []uint64 // per measured action
	overhead    uint64   // harness mallocs inside the current action
	heapT       []float64
	heapB       []float64
	recent      []string // ring of the last lastActions actions
	recentAt    int
	report      Report
	darkTheme   bool
	ms          runtime.MemStats
	stopped     bool
}

// Run drives w for cfg.Duration of simulated time and reports what it
// measured. The error is for a run that could not start (the first
// render failed); a panic during the run is in Report.Panic, and
// [Report.Check] applies the gates.
//
// w must be a headless window built for tests ([gui.NewTestWindow]),
// with its view installed. Run calls TestStep, which switches w to a
// manual clock for good.
func Run(w *gui.Window, cfg Config) (Report, error) {
	cfg = cfg.withDefaults()
	r := &runner{
		w:          w,
		cfg:        cfg,
		rng:        rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15)),
		frames:     int(cfg.Duration / FrameDt),
		warmFrames: int(cfg.Warmup / FrameDt),
		darkTheme:  true,
	}
	measured := r.frames - r.warmFrames
	every := min(maxHeapSampleEvery, (cfg.Duration-cfg.Warmup)/heapSamples)
	r.sampleEach = max(1, int(every/FrameDt))
	r.allocs = make([]uint64, 0, measured)
	r.report = Report{Seed: cfg.Seed, Mode: cfg.Mode}
	cfg.Logf("soak: seed=%d mode=%s duration=%s warmup=%s", cfg.Seed, cfg.Mode,
		cfg.Duration, cfg.Warmup)

	start := time.Now()
	// Pin the manual clock first. An animation that the first frame
	// adds (caret blink, spinner, SVG) would otherwise start the
	// wall-clock animation goroutine, and its ticks would make the
	// session depend on scheduling, not on the seed.
	w.TestStep(0)
	// A backend reports OS focus at startup; without it no caret
	// blinks and the window drops most key events.
	w.EventFn(&gui.Event{Type: gui.EventFocused})
	// The first render installs the software text measurer and SVG
	// parser, so layout uses real glyph metrics from here on.
	if _, err := soft.RenderToImage(w, cfg.Scale); err != nil {
		return r.report, fmt.Errorf("soak: first render: %w", err)
	}

	for r.frame < r.frames && !r.stopped {
		n := r.act()
		for range n {
			if r.frame >= r.frames || r.stopped {
				break
			}
			r.step()
		}
	}
	r.report.Wall = time.Since(start)
	r.finish()
	return r.report, nil
}

// measuring reports whether the current frame is past warm-up.
func (r *runner) measuring() bool { return r.frame >= r.warmFrames }

// step runs one frame: TestStep with allocations counted, a software
// render every RenderEvery frames, and a heap sample when due.
func (r *runner) step() {
	measure := r.measuring()
	var before uint64
	if measure {
		runtime.ReadMemStats(&r.ms)
		before = r.ms.Mallocs
	}
	r.guard("frame", func() { r.w.TestStep(FrameDt) })
	if r.stopped {
		return
	}
	if measure {
		runtime.ReadMemStats(&r.ms)
		r.allocs = append(r.allocs, r.ms.Mallocs-before)
	}
	r.frame++
	if r.frame%r.cfg.RenderEvery == 0 {
		r.guard("render", func() {
			if _, err := soft.RenderToImage(r.w, r.cfg.Scale); err != nil {
				r.report.RenderErrors++
			}
		})
	}
	if measure && (r.frame-r.warmFrames)%r.sampleEach == 0 {
		runtime.GC()
		runtime.ReadMemStats(&r.ms)
		r.heapT = append(r.heapT, float64(r.frame)*FrameDt.Minutes())
		r.heapB = append(r.heapB, float64(r.ms.HeapAlloc))
	}
	if r.frame%(r.frames/10+1) == 0 {
		// The last heap sample shows the curve's shape in the log: a
		// cache that fills levels off, a leak keeps climbing.
		heap := "-"
		if n := len(r.heapB); n > 0 {
			heap = kib(r.heapB[n-1])
		}
		r.cfg.Logf("soak: %d/%d frames, heap %s", r.frame, r.frames, heap)
	}
}

// act takes one input action and returns how many frames to step after
// it. The action's allocations (dispatch plus the frames Test* settles)
// are counted apart from the frames.
func (r *runner) act() int {
	measure := r.measuring()
	var before uint64
	r.overhead = 0
	if measure {
		runtime.ReadMemStats(&r.ms)
		before = r.ms.Mallocs
	}
	var frames int
	r.guard("action", func() {
		if r.cfg.Mode == ModeRandom {
			frames = r.randomEvent()
		} else {
			frames = r.scriptedAction()
		}
	})
	if measure && !r.stopped {
		runtime.ReadMemStats(&r.ms)
		// The harness's own allocations (action strings, ID lists) are
		// not the app's cost.
		r.eventAllocs = append(r.eventAllocs, r.ms.Mallocs-before-r.overhead)
	}
	r.report.Actions++
	return frames
}

// record builds an action's text, adds it to the recent ring and passes
// it to OnAction. It takes a func so the formatting runs inside
// excluded and does not count as the app's allocations.
func (r *runner) record(text func() string) {
	r.excluded(func() {
		action := text()
		if r.cfg.OnAction != nil {
			r.cfg.OnAction(r.frame, action)
		}
		if len(r.recent) < lastActions {
			r.recent = append(r.recent, action)
			return
		}
		r.recent[r.recentAt] = action
		r.recentAt = (r.recentAt + 1) % lastActions
	})
}

// excluded runs fn, the harness's own work inside an action, and adds
// its mallocs to r.overhead, which act subtracts from the action.
func (r *runner) excluded(fn func()) {
	if !r.measuring() {
		fn()
		return
	}
	runtime.ReadMemStats(&r.ms)
	before := r.ms.Mallocs
	fn()
	runtime.ReadMemStats(&r.ms)
	r.overhead += r.ms.Mallocs - before
}

// lastActionsInOrder returns the recent ring oldest first.
func (r *runner) lastActionsInOrder() []string {
	out := make([]string, 0, len(r.recent))
	out = append(out, r.recent[r.recentAt:]...)
	return append(out, r.recent[:r.recentAt]...)
}

// guard runs fn and turns a panic into the report's Panic, which stops
// the run.
func (r *runner) guard(what string, fn func()) {
	defer func() {
		if v := recover(); v != nil {
			r.stopped = true
			r.report.Panic = &Panic{
				Frame:       r.frame,
				Value:       fmt.Sprintf("%s: %v", what, v),
				Stack:       string(debug.Stack()),
				LastActions: r.lastActionsInOrder(),
			}
		}
	}()
	fn()
}

// finish computes the report numbers from the samples.
func (r *runner) finish() {
	r.report.Frames = r.frame
	r.report.Measured = len(r.allocs)
	r.report.AllocsMean = mean(r.allocs)
	r.report.AllocsP99 = percentile(r.allocs, 0.99)
	r.report.EventAllocsMean = mean(r.eventAllocs)
	if n := len(r.allocs); n > 0 {
		r.report.SessionAllocsPerFrame = (mean(r.allocs)*float64(n) +
			mean(r.eventAllocs)*float64(len(r.eventAllocs))) / float64(n)
	}
	r.report.HeapSamples = len(r.heapB)
	if n := len(r.heapB); n > 0 {
		r.report.HeapStart = uint64(r.heapB[0])
		r.report.HeapEnd = uint64(r.heapB[n-1])
	}
	r.report.HeapSlopeRaw = slope(r.heapT, r.heapB)
	r.report.HeapSlope = slope(windowMinima(r.heapT, r.heapB, heapWindows))
}
