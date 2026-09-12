package gui

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// Repeating keyframes drop the stall backlog like Animate: one done tick
// resyncs instead of draining a missed interval per tick.
func TestKeyframeRepeatDropsBacklogAfterStall(t *testing.T) {
	kf := NewKeyframeAnimation("k-stall",
		[]Keyframe{
			{At: 0, Value: 0},
			{At: 1, Value: 100, Easing: EaseLinear},
		},
		func(float32, *Window) {},
	)
	kf.Repeat = true
	kf.Duration = 16 * time.Millisecond
	kf.SetStart(time.Now().Add(-5 * time.Second))

	deferred := make([]queuedCommand, 0, 16)
	ac := newAnimationCommands(&deferred)
	updateKeyframe(kf, &ac)

	if behind := time.Since(kf.start); behind > kf.Duration {
		t.Errorf("start still %v behind, want under %v", behind, kf.Duration)
	}
	if kf.stopped {
		t.Error("repeating keyframe should not stop")
	}
}

// A non-positive repeating Duration is always done: retire as a one-shot
// instead of returning true every tick forever.
func TestKeyframeRepeatNonPositiveDurationRetires(t *testing.T) {
	for _, d := range []time.Duration{0, -100 * time.Millisecond} {
		done := false
		kf := NewKeyframeAnimation("k-zero",
			[]Keyframe{{At: 0, Value: 0}, {At: 1, Value: 100}},
			func(float32, *Window) {},
		)
		kf.Repeat = true
		kf.Duration = d
		kf.OnDone = func(*Window) { done = true }
		kf.SetStart(time.Now().Add(-time.Second))

		deferred := make([]queuedCommand, 0, 4)
		ac := newAnimationCommands(&deferred)
		updateKeyframe(kf, &ac)
		runQueuedCommands(deferred)
		if !kf.stopped {
			t.Errorf("duration %v: repeating keyframe should retire", d)
		}
		if !done {
			t.Errorf("duration %v: OnDone should run", d)
		}
	}
}

// A NaN easing sample is dropped: nothing queued, no refresh, still live.
func TestKeyframeNaNEasingDropsFrame(t *testing.T) {
	nanEase := func(float32) float32 { return float32(math.NaN()) }
	kf := NewKeyframeAnimation("k-nan",
		[]Keyframe{{At: 0, Value: 0}, {At: 1, Value: 100, Easing: nanEase}},
		func(float32, *Window) {},
	)
	kf.Duration = time.Hour // mid-flight, not done
	kf.SetStart(time.Now())

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	if updateKeyframe(kf, &ac) {
		t.Error("NaN sample should not request a refresh")
	}
	if len(deferred) != 0 {
		t.Errorf("queued %d commands, want 0", len(deferred))
	}
	if kf.stopped {
		t.Error("should stay live after a dropped frame")
	}
}

// A non-finite completion value is skipped but OnDone still runs.
func TestKeyframeNaNFinalValueSkipped(t *testing.T) {
	var got []float32
	done := false
	kf := NewKeyframeAnimation("k-nanfinal",
		[]Keyframe{{At: 0, Value: 0}, {At: 1, Value: float32(math.NaN())}},
		func(v float32, _ *Window) { got = append(got, v) },
	)
	kf.OnDone = func(*Window) { done = true }
	kf.SetStart(time.Now().Add(-time.Second))

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateKeyframe(kf, &ac)
	runQueuedCommands(deferred)
	for _, v := range got {
		if math.IsNaN(float64(v)) {
			t.Errorf("OnValue received NaN")
		}
	}
	if !done {
		t.Error("OnDone should run")
	}
	if !kf.stopped {
		t.Error("should stop")
	}
}

// Zero-value tween easing matches the constructor default (EaseOutCubic).
func TestTweenNilEasingMatchesConstructor(t *testing.T) {
	var got float32
	tw := &TweenAnimation{
		AnimID:   "t-nil",
		Duration: 300 * time.Millisecond,
		From:     0,
		To:       100,
		OnValue:  func(v float32, _ *Window) { got = v },
	}
	tw.SetStart(time.Now().Add(-150 * time.Millisecond))

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateTween(tw, &ac)
	runQueuedCommands(deferred)
	// EaseOutCubic(0.5) = 0.875 → 87.5; linear would give ~50.
	if got < 75 || got > 95 {
		t.Errorf("value = %v, want ~87.5 (EaseOutCubic)", got)
	}
}

// Non-finite tween endpoints never reach OnValue.
func TestTweenNonFiniteEndpointsDropped(t *testing.T) {
	tw := NewTweenAnimation("t-nan",
		float32(math.NaN()), float32(math.Inf(1)),
		func(v float32, _ *Window) {
			t.Errorf("OnValue received %v", v)
		})
	tw.SetStart(time.Now())

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	if updateTween(tw, &ac) {
		t.Error("poisoned endpoints should not request a refresh")
	}
	if tw.stopped {
		t.Error("should stay live until duration ends")
	}

	tw2 := NewTweenAnimation("t-nanto",
		0, float32(math.NaN()),
		func(v float32, _ *Window) {
			if math.IsNaN(float64(v)) {
				t.Errorf("OnValue received NaN")
			}
		})
	done := false
	tw2.OnDone = func(*Window) { done = true }
	tw2.SetStart(time.Now().Add(-time.Second))
	deferred = deferred[:0]
	ac = newAnimationCommands(&deferred)
	updateTween(tw2, &ac)
	runQueuedCommands(deferred)
	if !done || !tw2.stopped {
		t.Error("completion should still run OnDone and stop")
	}
}

// Non-finite spring config falls back instead of snapping instantly.
func TestSpringNaNConfigFallsBackToDefault(t *testing.T) {
	var got []float32
	sp := NewSpringAnimation("s-nan", func(v float32, _ *Window) {
		got = append(got, v)
	})
	sp.Config = SpringCfg{
		Stiffness: float32(math.NaN()),
		Damping:   float32(math.NaN()),
		Mass:      float32(math.NaN()),
		Threshold: float32(math.NaN()),
	}
	sp.SpringTo(0, 1)

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateSpring(sp, 0.016, &ac)
	if sp.stopped {
		t.Fatal("sanitized spring should not snap on the first tick")
	}
	runQueuedCommands(deferred)
	if len(got) != 1 || got[0] == 1 {
		t.Errorf("got %v, want one mid-flight sample", got)
	}
}

// Negative spring config falls back instead of diverging.
func TestSpringNegativeConfigFallsBack(t *testing.T) {
	var got float32
	sp := NewSpringAnimation("s-neg", func(v float32, _ *Window) { got = v })
	sp.Config = SpringCfg{Stiffness: -50, Damping: -5, Mass: 1, Threshold: 0.01}
	sp.SpringTo(0, 1)

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateSpring(sp, 0.016, &ac)
	if sp.stopped {
		t.Error("negative config should sanitize, not snap")
	}
	runQueuedCommands(deferred)
	if math.IsNaN(float64(got)) || math.IsInf(float64(got), 0) {
		t.Errorf("sample = %v, want finite", got)
	}
	if got == 1 {
		t.Error("sanitized spring should ease in, not arrive on tick one")
	}
}

// A non-finite spring target retires without emitting.
func TestSpringNaNTargetRetiresCleanly(t *testing.T) {
	sp := NewSpringAnimation("s-nantarget", func(v float32, _ *Window) {
		t.Errorf("OnValue received %v", v)
	})
	done := false
	sp.OnDone = func(*Window) { done = true }
	sp.SpringTo(0, float32(math.NaN()))

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateSpring(sp, 0.016, &ac)
	runQueuedCommands(deferred)
	if !sp.stopped || !done {
		t.Error("should retire with OnDone and no value")
	}
}

// A NaN easing hook cannot poison a transition's progress.
func TestTransitionNaNEasingFallsBack(t *testing.T) {
	tb := &transitionBase{
		duration: time.Hour,
		easing:   func(float32) float32 { return float32(math.NaN()) },
	}
	tb.SetStart(time.Now())
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateTransition(tb, &ac)
	if math.IsNaN(float64(tb.progress)) {
		t.Fatal("progress is NaN")
	}
	if tb.progress < 0 || tb.progress > 1 {
		t.Errorf("progress = %v, want linear fallback in [0,1]", tb.progress)
	}
}

// Snapshot capture stops at the shared depth budget.
func TestCaptureSnapshotsRespectsDepthBudget(t *testing.T) {

	const chainLen = 1000
	root := &Layout{Shape: &Shape{}}
	cur := root
	for i := range chainLen {
		cur.Children = []Layout{{Shape: &Shape{ID: fmt.Sprintf("n%d", i)}}}
		cur = &cur.Children[0]
	}
	got := captureLayoutSnapshots(root)
	if len(got) >= chainLen {
		t.Errorf("captured %d snapshots, want under %d", len(got), chainLen)
	}
	if len(got) > maxEventDepth+1 {
		t.Errorf("captured %d snapshots, want at most %d", len(got), maxEventDepth+1)
	}
}

// A nil shape never panics capture; the walk skips it and continues.
func TestCaptureSnapshotsNilSafe(t *testing.T) {
	root := &Layout{Shape: &Shape{ID: "root"}}
	root.Children = []Layout{{Shape: nil}, {Shape: &Shape{ID: "leaf"}}}
	got := captureLayoutSnapshots(root)
	if len(got) != 2 {
		t.Errorf("captured %d snapshots, want 2", len(got))
	}
	var nilRoot *Layout
	if out := captureLayoutSnapshots(nilRoot); len(out) != 0 {
		t.Errorf("nil root captured %d snapshots, want 0", len(out))
	}
}

// Out-of-order waypoints stay bounded: the segment fraction is clamped
// so a custom easing hook never sees input far outside [0,1]. With the
// waypoints below, progress 0.5 lands in slice-segment [0.2,0.4] with a
// raw fraction of 1.5, which EaseInQuad would amplify to 2.25.
func TestInterpolateUnsortedAtStaysBounded(t *testing.T) {
	kfs := []Keyframe{
		{At: 0.6, Value: 60},
		{At: 0.2, Value: 20},
		{At: 0.4, Value: 40, Easing: EaseInQuad},
	}
	if got := interpolateKeyframes(kfs, 0.5); got < 20 || got > 60 {
		t.Errorf("got %v, want within [20,60]", got)
	}
}

// Negative transition durations take the default instead of retiring.
func TestNegativeTransitionDurationsDefault(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{}}
	w.AnimateLayout(LayoutTransitionCfg{Duration: -100 * time.Millisecond})
	lt, ok := w.animations[layoutTransitionID].(*layoutTransition)
	if !ok {
		t.Fatal("layout transition not registered")
	}
	if lt.duration != 200*time.Millisecond {
		t.Errorf("duration = %v, want 200ms", lt.duration)
	}
	ht := NewHeroTransition(HeroTransitionCfg{Duration: -time.Second})
	if ht.duration != 300*time.Millisecond {
		t.Errorf("hero duration = %v, want 300ms", ht.duration)
	}
}

// A stopped smoother releases its entries instead of retaining one per
// scrollable ever touched.
func TestScrollSmoothReleasesEntriesOnStop(t *testing.T) {
	ss := &scrollSmoothAnimation{
		entries: []scrollSmoothEntry{
			{id: "a", axis: scrollAxisY},
			{id: "b", axis: scrollAxisX},
		},
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	if ss.Update(nil, 0, &ac) {
		t.Error("idle smoother should not request a refresh")
	}
	if !ss.stopped {
		t.Error("idle smoother should stop")
	}
	if len(ss.entries) != 0 {
		t.Errorf("retained %d entries, want 0", len(ss.entries))
	}
}

// An unapplied final value survives the stop: the tick must not drop a
// dirty entry the main thread has not flushed yet, or the ease ends
// fractionally short of its target.
func TestScrollSmoothKeepsDirtyEntriesOnStop(t *testing.T) {
	ss := &scrollSmoothAnimation{
		entries: []scrollSmoothEntry{
			{id: "a", axis: scrollAxisY, current: -50, dirty: true},
		},
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	if ss.Update(nil, 0, &ac) {
		t.Error("idle smoother should not request a refresh")
	}
	if !ss.stopped {
		t.Error("idle smoother should stop")
	}
	if len(ss.entries) != 1 {
		t.Fatal("dirty entry dropped before its final value was applied")
	}
}
