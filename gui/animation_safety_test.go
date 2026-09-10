package gui

import "testing"

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

// TestUpdateSpringDivergenceStops guards the fixed-timestep divergence
// path: a stiffness the 16ms step cannot integrate drives velocity to
// +Inf and position to NaN. NaN fails every threshold comparison, so
// without an explicit finite check the animation never retires — it
// pushes NaN through OnValue and forces a layout refresh every tick,
// forever.
func TestUpdateSpringDivergenceStops(t *testing.T) {
	var last float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { last = v })
	sp.Config = SpringCfg{Stiffness: 20000, Damping: 10, Mass: 1, Threshold: 0.01}
	sp.SpringTo(0, 1)

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	// Divergence is reached in ~72 ticks; 500 leaves ample margin
	// without letting a non-retiring spring run the test forever.
	for range 500 {
		if !updateSpring(sp, 0.016, &ac) {
			break
		}
	}
	if !sp.stopped {
		t.Fatalf("diverged spring never stopped: pos=%v vel=%v",
			sp.state.position, sp.state.velocity)
	}
	if !f32IsFinite(sp.state.position) || !f32IsFinite(sp.state.velocity) {
		t.Fatalf("state left non-finite: pos=%v vel=%v",
			sp.state.position, sp.state.velocity)
	}
	if sp.state.position != sp.state.target {
		t.Errorf("position = %v, want target %v",
			sp.state.position, sp.state.target)
	}
	if !f32IsFinite(last) {
		t.Errorf("OnValue received non-finite value %v", last)
	}
}
