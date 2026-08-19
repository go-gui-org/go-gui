package gui

import (
	"testing"
	"time"
)

func TestSpringTo(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.SpringTo(10, 100)
	if sp.state.position != 10 {
		t.Errorf("position = %f", sp.state.position)
	}
	if sp.state.target != 100 {
		t.Errorf("target = %f", sp.state.target)
	}
}

func TestSpringRetarget(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.SpringTo(0, 50)
	sp.state.velocity = 5
	sp.retarget(200)
	if sp.state.target != 200 {
		t.Error("target not updated")
	}
	if sp.state.velocity != 5 {
		t.Error("velocity should be preserved")
	}
}

func TestSpringSettles(t *testing.T) {
	var got float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { got = v })
	sp.Config = SpringStiff
	sp.SpringTo(100, 100) // already at target
	sp.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateSpring(sp, 0.016, &ac)
	runQueuedCommands(deferred)
	if got != 100 {
		t.Errorf("expected 100, got %f", got)
	}
	if !sp.stopped {
		t.Error("should be at rest")
	}
}

func TestSpringStoppedSkips(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.stopped = true
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateSpring(sp, 0.016, &ac)
	if ok {
		t.Error("stopped spring should return false")
	}
}

func TestSpringID(t *testing.T) {
	sp := NewSpringAnimation("myspring", func(float32, *Window) {})
	if sp.ID() != "myspring" {
		t.Errorf("ID = %q, want myspring", sp.ID())
	}
}

func TestSpringRefreshKind(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	if sp.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("RefreshKind = %d, want %d", sp.RefreshKind(), AnimationRefreshLayout)
	}
}

func TestSpringIsStopped(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	if sp.IsStopped() {
		t.Error("new spring should not be stopped")
	}
	sp.stopped = true
	if !sp.IsStopped() {
		t.Error("should report stopped")
	}
}

func TestSpringSetStart(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	now := time.Now()
	sp.SetStart(now)
	if sp.start != now {
		t.Error("SetStart should update start time")
	}
}

func TestSpringUpdateInterface(t *testing.T) {
	var got float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { got = v })
	sp.Config = SpringStiff
	sp.SpringTo(100, 100) // already at target
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := sp.Update(nil, 0.016, &ac)
	if !ok {
		t.Error("Update should return true")
	}
	runQueuedCommands(deferred)
	if got != 100 {
		t.Errorf("got %f, want 100", got)
	}
}

func TestSpringThresholdUsesUpdatedPosition(t *testing.T) {
	var got float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { got = v })
	sp.Config = SpringCfg{Stiffness: 100, Damping: 10, Mass: 1, Threshold: 0.01}
	// Position very close to target: pre-update displacement is tiny,
	// but velocity will carry position past threshold after integration.
	sp.state.position = 100.005
	sp.state.velocity = -5
	sp.state.target = 100
	sp.state.atRest = false
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateSpring(sp, 0.016, &ac)
	// The spring should NOT be at rest because displacement is recomputed
	// after the position update.
	if sp.stopped && got == sp.state.target {
		// If stopped with exact target, the old (stale) displacement
		// was used. The fix ensures the post-update displacement is checked.
		t.Error("threshold should use post-update displacement")
	}
}
