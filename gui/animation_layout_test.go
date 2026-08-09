package gui

import (
	"testing"
	"time"
)

func TestCaptureLayoutSnapshots(t *testing.T) {
	layout := Layout{
		Shape: &Shape{ID: "root", X: 0, Y: 0, Width: 800, Height: 600},
		Children: []Layout{
			{Shape: &Shape{ID: "a", X: 10, Y: 10, Width: 100, Height: 50}},
			{Shape: &Shape{X: 20, Y: 20, Width: 30, Height: 30}}, // no ID
		},
	}
	snaps := captureLayoutSnapshots(layout)
	if len(snaps) != 2 {
		t.Errorf("got %d snapshots, want 2 (root + a)", len(snaps))
	}
	if s, ok := snaps["a"]; !ok || s.x != 10 {
		t.Error("snapshot 'a' wrong")
	}
}

func TestLayoutTransitionUpdate(t *testing.T) {
	lt := &layoutTransition{
		transitionBase: transitionBase{
			duration: 200 * time.Millisecond,
			easing:   EaseOutCubic,
		},
		snapshots: make(map[string]posSnapshot),
	}
	lt.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateTransition(&lt.transitionBase, &ac)
	if !ok {
		t.Error("should update")
	}
	if !lt.stopped {
		t.Error("should be stopped after duration")
	}
}

func TestApplyTransitionRecursive(t *testing.T) {
	layout := Layout{
		Shape: &Shape{ID: "box", X: 100, Y: 100, Width: 200, Height: 200},
	}
	lt := &layoutTransition{
		transitionBase: transitionBase{progress: 0.5},
		snapshots: map[string]posSnapshot{
			"box": {x: 0, y: 0, width: 100, height: 100},
		},
	}
	applyTransitionRecursive(&layout, lt)
	// At progress=0.5: Lerp(0, 100, 0.5) = 50
	if layout.Shape.X != 50 {
		t.Errorf("X = %f, want 50", layout.Shape.X)
	}
}

func TestLayoutTransitionID(t *testing.T) {
	lt := &layoutTransition{}
	if lt.ID() != layoutTransitionID {
		t.Errorf("ID = %q, want %q", lt.ID(), layoutTransitionID)
	}
}

func TestLayoutTransitionRefreshKind(t *testing.T) {
	lt := &layoutTransition{}
	if lt.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("RefreshKind = %d, want %d", lt.RefreshKind(), AnimationRefreshLayout)
	}
}

func TestLayoutTransitionUpdateInterface(t *testing.T) {
	lt := &layoutTransition{
		transitionBase: transitionBase{
			duration: 200 * time.Millisecond,
			easing:   EaseOutCubic,
		},
		snapshots: make(map[string]posSnapshot),
	}
	lt.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := lt.Update(nil, 0, &ac)
	if !ok {
		t.Error("Update should return true")
	}
	if !lt.stopped {
		t.Error("should be stopped")
	}
}

func TestLayoutTransitionOnDone(t *testing.T) {
	done := false
	lt := &layoutTransition{
		transitionBase: transitionBase{
			duration: 200 * time.Millisecond,
			easing:   EaseOutCubic,
			OnDone:   func(*Window) { done = true },
		},
		snapshots: make(map[string]posSnapshot),
	}
	lt.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateTransition(&lt.transitionBase, &ac)
	runQueuedCommands(deferred)
	if !done {
		t.Error("OnDone not called")
	}
}
