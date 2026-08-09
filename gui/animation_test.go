package gui

import (
	"testing"
	"time"
)

func TestTransitionBaseIsStopped(t *testing.T) {
	tb := &transitionBase{stopped: true}
	if !tb.IsStopped() {
		t.Error("should report stopped")
	}
	tb.stopped = false
	if tb.IsStopped() {
		t.Error("should report not stopped")
	}
}

func TestUpdateTransitionAlreadyStopped(t *testing.T) {
	tb := &transitionBase{stopped: true}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	if updateTransition(tb, &ac) {
		t.Error("should return false when already stopped")
	}
}

func TestUpdateTransitionCompletes(t *testing.T) {
	doneCalled := false
	tb := &transitionBase{
		duration: 50 * time.Millisecond,
		easing:   EaseLinear,
		OnDone:   func(*Window) { doneCalled = true },
		start:    time.Now().Add(-time.Second),
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateTransition(tb, &ac)
	if !ok {
		t.Error("should return true on completion")
	}
	if tb.progress != 1.0 {
		t.Errorf("progress = %f, want 1.0", tb.progress)
	}
	if !tb.stopped {
		t.Error("should be stopped after completion")
	}
	runQueuedCommands(deferred)
	if !doneCalled {
		t.Error("OnDone should be called")
	}
}

func TestUpdateTransitionNilEasingDefaultsToEaseOutCubic(t *testing.T) {
	tb := &transitionBase{
		duration: 10 * time.Second,
		easing:   nil,
		start:    time.Now().Add(-50 * time.Millisecond),
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateTransition(tb, &ac)
	if !ok {
		t.Error("should return true while in progress")
	}
	if tb.progress <= 0 || tb.progress >= 1 {
		t.Errorf("progress = %f, want 0 < p < 1", tb.progress)
	}
}

func TestUpdateTransitionWithEasing(t *testing.T) {
	tb := &transitionBase{
		duration: 10 * time.Second,
		easing:   easeInQuad,
		start:    time.Now().Add(-50 * time.Millisecond),
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateTransition(tb, &ac)
	if !ok {
		t.Error("should return true while in progress")
	}
	if tb.progress <= 0 || tb.progress >= 1 {
		t.Errorf("progress = %f, want 0 < p < 1", tb.progress)
	}
}

func TestUpdateTransitionNilOnDone(t *testing.T) {
	tb := &transitionBase{
		duration: 50 * time.Millisecond,
		start:    time.Now().Add(-time.Second),
		OnDone:   nil,
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateTransition(tb, &ac)
	if !ok {
		t.Error("should return true on completion")
	}
	if len(deferred) != 0 {
		t.Error("nil OnDone should not queue a command")
	}
}

func TestBlinkCursorAnimationIsStopped(t *testing.T) {
	b := newBlinkCursorAnimation()
	if b.IsStopped() {
		t.Error("new blink should not be stopped")
	}
	b.stopped = true
	if !b.IsStopped() {
		t.Error("should report stopped")
	}
}

func TestBlinkCursorAnimationUpdate(t *testing.T) {
	b := newBlinkCursorAnimation()
	b.start = time.Now().Add(-time.Second)
	w := &Window{}
	ok := b.Update(w, 0, nil)
	if !ok {
		t.Error("should return true after delay elapsed")
	}
}

func TestAnimateIsStopped(t *testing.T) {
	a := &Animate{AnimID: "test"}
	if a.IsStopped() {
		t.Error("new animate should not be stopped")
	}
	a.stopped = true
	if !a.IsStopped() {
		t.Error("should report stopped")
	}
}

func TestAnimateRefreshKindDefault(t *testing.T) {
	a := &Animate{AnimID: "test"}
	if a.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("default RefreshKind = %d, want %d",
			a.RefreshKind(), AnimationRefreshLayout)
	}
}

func TestAnimateRefreshKindCustom(t *testing.T) {
	a := &Animate{AnimID: "test", Refresh: animationRefreshRenderOnly}
	if a.RefreshKind() != animationRefreshRenderOnly {
		t.Errorf("RefreshKind = %d, want %d",
			a.RefreshKind(), animationRefreshRenderOnly)
	}
}

func TestAnimateUpdateCallsCallback(t *testing.T) {
	called := false
	a := &Animate{
		AnimID:   "test",
		Callback: func(_ *Animate, _ *Window) { called = true },
		Delay:    0,
		start:    time.Now().Add(-time.Second),
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := a.Update(nil, 0, &ac)
	if !ok {
		t.Error("should return true")
	}
	runQueuedCommands(deferred)
	if !called {
		t.Error("callback not invoked")
	}
}

func TestAnimateUpdateAlreadyStopped(t *testing.T) {
	a := &Animate{
		AnimID:  "test",
		stopped: true,
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := a.Update(nil, 0, &ac)
	if ok {
		t.Error("should return false when stopped")
	}
}

func TestAnimateUpdateNilCallback(t *testing.T) {
	a := &Animate{
		AnimID:   "test",
		Callback: nil,
		start:    time.Now().Add(-time.Second),
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := a.Update(nil, 0, &ac)
	if ok {
		t.Error("nil callback should return false")
	}
	if !a.stopped {
		t.Error("nil callback should stop animation")
	}
}

func TestTransitionBaseSetStart(t *testing.T) {
	tb := &transitionBase{}
	now := time.Now()
	tb.SetStart(now)
	if tb.start != now {
		t.Error("SetStart should update start time")
	}
}

func TestBlinkCursorAnimationID(t *testing.T) {
	b := newBlinkCursorAnimation()
	if b.ID() != blinkCursorAnimationID {
		t.Errorf("ID = %q, want %q", b.ID(), blinkCursorAnimationID)
	}
}

func TestBlinkCursorAnimationRefreshKind(t *testing.T) {
	b := newBlinkCursorAnimation()
	if b.RefreshKind() != animationRefreshRenderOnly {
		t.Errorf("RefreshKind = %d, want %d",
			b.RefreshKind(), animationRefreshRenderOnly)
	}
}

func TestBlinkCursorAnimationSetStart(t *testing.T) {
	b := newBlinkCursorAnimation()
	now := time.Now()
	b.SetStart(now)
	if b.start != now {
		t.Error("SetStart should update start time")
	}
}

func TestAnimateID(t *testing.T) {
	a := &Animate{AnimID: "myid"}
	if a.ID() != "myid" {
		t.Errorf("ID = %q, want %q", a.ID(), "myid")
	}
}

func TestAnimateSetStart(t *testing.T) {
	a := &Animate{}
	now := time.Now()
	a.SetStart(now)
	if a.start != now {
		t.Error("SetStart should update start time")
	}
}

func TestCommandMarkLayoutRefresh(t *testing.T) {
	w := &Window{}
	commandMarkLayoutRefresh(w)
	if !w.refreshLayout {
		t.Error("should set refreshLayout")
	}
}

func TestCommandMarkRenderOnlyRefresh(t *testing.T) {
	w := &Window{}
	commandMarkRenderOnlyRefresh(w)
	if !w.refreshRenderOnly {
		t.Error("should set refreshRenderOnly")
	}
}

func TestAnimationCommandsAppendOnDoneNilFn(t *testing.T) {
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ac.appendOnDone(nil)
	if len(deferred) != 0 {
		t.Error("nil fn should not queue")
	}
}

func TestAnimationCommandsAppendOnDoneWithFn(t *testing.T) {
	called := false
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ac.appendOnDone(func(*Window) { called = true })
	if len(deferred) != 1 {
		t.Fatal("should queue one command")
	}
	runQueuedCommands(deferred)
	if !called {
		t.Error("fn not called")
	}
}

func TestAnimationCommandsAppendOnValue(t *testing.T) {
	var got float32
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ac.appendOnValue(func(v float32, _ *Window) { got = v }, 42)
	if len(deferred) != 1 {
		t.Fatal("should queue one command")
	}
	runQueuedCommands(deferred)
	if got != 42 {
		t.Errorf("got %f, want 42", got)
	}
}
