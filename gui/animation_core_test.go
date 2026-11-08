package gui

import (
	"testing"
	"time"
)

func TestDurationProgressZero(t *testing.T) {
	p, done := durationProgress(time.Now(), 0)
	if p != 1.0 || !done {
		t.Errorf("got (%f,%v), want (1.0,true)", p, done)
	}
}

func TestDurationProgressComplete(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	p, done := durationProgress(start, 1*time.Second)
	if p != 1.0 || !done {
		t.Errorf("got (%f,%v), want (1.0,true)", p, done)
	}
}

func TestDurationProgressMidway(t *testing.T) {
	start := time.Now().Add(-50 * time.Millisecond)
	p, done := durationProgress(start, 1*time.Second)
	if done {
		t.Error("should not be done")
	}
	if p <= 0 || p >= 1 {
		t.Errorf("progress = %f, want 0 < p < 1", p)
	}
}

func TestMaxRefreshKindUpgrade(t *testing.T) {
	got := maxAnimationRefreshKind(AnimationRefreshNone, AnimationRefreshLayout)
	if got != AnimationRefreshLayout {
		t.Errorf("got %d, want %d", got, AnimationRefreshLayout)
	}
}

func TestMaxRefreshKindDowngrade(t *testing.T) {
	got := maxAnimationRefreshKind(AnimationRefreshLayout, AnimationRefreshNone)
	if got != AnimationRefreshLayout {
		t.Errorf("got %d, want %d", got, AnimationRefreshLayout)
	}
}

func TestMaxRefreshKindEqual(t *testing.T) {
	got := maxAnimationRefreshKind(AnimationRefreshRenderOnly, AnimationRefreshRenderOnly)
	if got != AnimationRefreshRenderOnly {
		t.Errorf("got %d, want %d", got, AnimationRefreshRenderOnly)
	}
}
