package gui

import (
	"math"
	"testing"
)

func TestUpdateAnimateNilCallbackStops(t *testing.T) {
	a := &Animate{AnimID: "a"}
	deferred := make([]queuedCommand, 0, 1)
	ac := newAnimationCommands(&deferred)
	ok := updateAnimate(a, &ac)
	if ok {
		t.Fatal("expected no update when callback is nil")
	}
	if !a.stopped {
		t.Fatal("expected animate to stop when callback is nil")
	}
}

func TestUpdateTweenNilOnValueStops(t *testing.T) {
	tw := NewTweenAnimation("t", 0, 1, nil)
	deferred := make([]queuedCommand, 0, 1)
	ac := newAnimationCommands(&deferred)
	ok := updateTween(tw, &ac)
	if ok {
		t.Fatal("expected no update when OnValue is nil")
	}
	if !tw.stopped {
		t.Fatal("expected tween to stop when OnValue is nil")
	}
}

func TestUpdateKeyframeNilOnValueStops(t *testing.T) {
	kf := NewKeyframeAnimation("k", []Keyframe{{At: 0, Value: 0}}, nil)
	deferred := make([]queuedCommand, 0, 1)
	ac := newAnimationCommands(&deferred)
	ok := updateKeyframe(kf, &ac)
	if ok {
		t.Fatal("expected no update when OnValue is nil")
	}
	if !kf.stopped {
		t.Fatal("expected keyframe to stop when OnValue is nil")
	}
}

func TestUpdateSpringNilOnValueStops(t *testing.T) {
	sp := NewSpringAnimation("s", nil)
	sp.SpringTo(0, 1)
	deferred := make([]queuedCommand, 0, 1)
	ac := newAnimationCommands(&deferred)
	ok := updateSpring(sp, 0.016, &ac)
	if ok {
		t.Fatal("expected no update when OnValue is nil")
	}
	if !sp.stopped {
		t.Fatal("expected spring to stop when OnValue is nil")
	}
}

// A stiffness of 20000 with Mass 1 is too stiff for a 16 ms Euler step:
// before #1017 it diverged and snapped to its target. The exact step
// cannot diverge, so the spring rings around the target and settles there.
func TestUpdateSpringHighStiffnessSettles(t *testing.T) {
	var last float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { last = v })
	sp.Config = SpringCfg{Stiffness: 20000, Damping: 10, Mass: 1, Threshold: 0.01}
	sp.SpringTo(0, 1)

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	// Damping 10 settles in about 2 s (125 ticks); 500 leaves margin
	// without letting a spring that never settles run the test forever.
	ticks := 0
	for range 500 {
		deferred = deferred[:0]
		if !updateSpring(sp, 0.016, &ac) {
			break
		}
		runQueuedCommands(deferred)
		ticks++
		// The swing never goes past the start: 0 to 1, so at most 2.
		if f32Abs(sp.state.position) > 2 {
			t.Fatalf("tick %d: position = %v, want |x| <= 2",
				ticks, sp.state.position)
		}
	}
	if ticks < 10 {
		t.Errorf("settled after %d ticks, want a motion, not a snap", ticks)
	}
	if !sp.stopped || sp.state.position != sp.state.target {
		t.Fatalf("stopped=%v pos=%v, want settled at target",
			sp.stopped, sp.state.position)
	}
	if last != 1 {
		t.Errorf("last value = %v, want 1", last)
	}
}

// A NaN start gives a NaN step. NaN fails every rest comparison, so
// without the finite check the spring would never retire and would send
// NaN to OnValue every tick. It must snap to its target and retire.
func TestUpdateSpringNaNStartSnapsToTarget(t *testing.T) {
	var got []float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { got = append(got, v) })
	done := false
	sp.OnDone = func(*Window) { done = true }
	sp.SpringTo(float32(math.NaN()), 1)

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateSpring(sp, 0.016, &ac)
	runQueuedCommands(deferred)
	if !sp.stopped || !done {
		t.Fatalf("stopped=%v done=%v, want retired", sp.stopped, done)
	}
	if sp.state.position != 1 {
		t.Errorf("position = %v, want target 1", sp.state.position)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("OnValue got %v, want [1]", got)
	}
}
