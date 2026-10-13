package gui

import (
	"testing"
	"time"
)

// A 240 Hz display sends a pulse every ~4.17 ms. Each pulse must tick
// at once, with the real time since the previous tick as dt (#1018).
func TestAnimationPacerTicksOnEach240HzPulse(t *testing.T) {
	var p animationPacer
	period := time.Second / 240
	now := time.Unix(1000, 0)
	p.resume(now)
	for i := range 10 {
		now = now.Add(period)
		if wait := p.pulse(now); wait != 0 {
			t.Fatalf("pulse %d: wait %v, want an immediate tick", i, wait)
		}
		dt := p.tick(now)
		want := float32(period.Seconds())
		if diff := dt - want; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("pulse %d: dt %v, want %v", i, dt, want)
		}
	}
}

// A backend without vsync sends pulses as fast as it draws. A pulse that
// comes sooner than animationMinTick after the last tick waits out the
// rest of the floor.
func TestAnimationPacerFloorsFastPulses(t *testing.T) {
	var p animationPacer
	now := time.Unix(1000, 0)
	p.resume(now)
	if wait := p.pulse(now.Add(500 * time.Microsecond)); wait != animationMinTick-500*time.Microsecond {
		t.Fatalf("wait %v, want %v", wait, animationMinTick-500*time.Microsecond)
	}
	if wait := p.pulse(now.Add(animationMinTick)); wait != 0 {
		t.Fatalf("pulse at the floor: wait %v, want 0", wait)
	}
}

// A pulse left in the buffer while the loop was parked must not tick at
// once with a made-up dt: after a resume the floor applies, and the
// first dt is the real time since the resume.
func TestAnimationPacerResumeMeasuresFromResume(t *testing.T) {
	var p animationPacer
	now := time.Unix(1000, 0)
	p.resume(now)
	if wait := p.pulse(now.Add(100 * time.Microsecond)); wait == 0 {
		t.Fatal("stale pulse right after resume ticked at once")
	}
	dt := p.tick(now.Add(3 * time.Millisecond))
	if want := float32(0.003); dt-want > 1e-6 || want-dt > 1e-6 {
		t.Fatalf("dt %v, want %v", dt, want)
	}
}

// countingAnimation reports each Update's dt on ticks. It never stops,
// so the loop keeps ticking it.
type countingAnimation struct {
	ticks chan float32
}

func (*countingAnimation) ID() string                        { return "count" }
func (*countingAnimation) RefreshKind() AnimationRefreshKind { return AnimationRefreshRenderOnly }
func (*countingAnimation) IsStopped() bool                   { return false }
func (*countingAnimation) SetStart(time.Time)                {}
func (c *countingAnimation) Update(_ *Window, dt float32, _ *AnimationCommands) bool {
	select {
	case c.ticks <- dt:
	default:
	}
	return false
}

func newPulseTestWindow() *Window {
	return &Window{
		windowAnimation: windowAnimation{
			animationStop:     make(chan struct{}),
			animationDone:     make(chan struct{}),
			animationResumeCh: make(chan struct{}, 1),
			animationPulseCh:  make(chan struct{}, 1),
		},
	}
}

// waitTick waits for the next tick of anim and returns its dt.
func waitTick(t *testing.T, anim *countingAnimation) float32 {
	t.Helper()
	select {
	case dt := <-anim.ticks:
		return dt
	case <-time.After(time.Second):
		t.Fatal("no tick within 1s")
		return 0
	}
}

// The real loop ticks on frame pulses, not only on the 16 ms watchdog.
// Each pulse waits for its tick before the next is sent. The watchdog
// spaces its ticks animationCycle apart, so it can never give 30 ticks
// in less than 30 cycles; only the pulses can.
func TestAnimationLoopTicksOnFramePulse(t *testing.T) {
	w := newPulseTestWindow()
	anim := &countingAnimation{ticks: make(chan float32, 1)}
	w.AnimationAdd(anim)
	defer w.stopAnimationLoop()

	const pulses = 30
	begin := time.Now()
	for range pulses {
		w.pulseAnimationFrame()
		waitTick(t, anim)
	}
	if took := time.Since(begin); took >= pulses*animationCycle {
		t.Fatalf("%d pulses took %v: the watchdog, not the pulses, ticked",
			pulses, took)
	}
}

// A pulse sent while the loop is parked stays in the buffer. On resume
// it must not tick at once with a 16 ms dt: the first dt is the time
// since the resume, at least the floor.
func TestAnimationLoopStalePulseAfterResume(t *testing.T) {
	w := newPulseTestWindow()
	w.ensureAnimationLoop()
	defer w.stopAnimationLoop()
	w.pulseAnimationFrame() // parked: nobody reads it

	anim := &countingAnimation{ticks: make(chan float32, 1)}
	begin := time.Now()
	w.AnimationAdd(anim)
	dt := waitTick(t, anim)
	elapsed := float32(time.Since(begin).Seconds())
	if dt > elapsed {
		t.Fatalf("first dt %v longer than the %v since the resume", dt, elapsed)
	}
	if dt < float32(animationMinTick.Seconds()) {
		t.Fatalf("first dt %v shorter than the floor", dt)
	}
}

// A window that drew a frame pulses the loop on its next FrameFn; one
// that drew nothing does not.
func TestFrameFnPulsesAfterPresentedFrame(t *testing.T) {
	w := &Window{
		windowAnimation: windowAnimation{
			animationPulseCh: make(chan struct{}, 1),
		},
	}
	w.FrameFn() // first frame: nothing presented before it
	select {
	case <-w.animationPulseCh:
		t.Fatal("pulse before any frame was presented")
	default:
	}
	w.markLayoutRefresh(refreshInitial)
	if !w.FrameFn() {
		t.Fatal("frame with a pending refresh should present")
	}
	w.FrameFn()
	select {
	case <-w.animationPulseCh:
	default:
		t.Fatal("no pulse after a presented frame")
	}
}

// A hidden window presents nothing, so it sends no pulse (#943): its
// animations keep their clocks on the watchdog alone.
func TestFrameFnHiddenWindowSendsNoPulse(t *testing.T) {
	w := &Window{
		windowAnimation: windowAnimation{
			animationPulseCh: make(chan struct{}, 1),
		},
	}
	w.markLayoutRefresh(refreshInitial)
	if !w.FrameFn() {
		t.Fatal("frame with a pending refresh should present")
	}
	w.occluded.Store(true)
	w.FrameFn() // pulses for the frame presented before the hide
	<-w.animationPulseCh
	w.markLayoutRefresh(refreshInitial)
	w.FrameFn()
	select {
	case <-w.animationPulseCh:
		t.Fatal("pulse from a hidden window")
	default:
	}
}
