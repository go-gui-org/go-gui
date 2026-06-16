package gui

import (
	"testing"
	"time"
)

func TestAnimationAdd(t *testing.T) {
	w := &Window{}
	tw := NewTweenAnimation("t1", 0, 1, func(float32, *Window) {})
	w.AnimationAdd(tw)
	if !w.HasAnimation("t1") {
		t.Error("animation not found")
	}
}

func TestAnimationRemove(t *testing.T) {
	w := &Window{}
	tw := NewTweenAnimation("t1", 0, 1, func(float32, *Window) {})
	w.AnimationAdd(tw)
	w.AnimationRemove("t1")
	if w.HasAnimation("t1") {
		t.Error("animation should be removed")
	}
}

func TestAnimationReplace(t *testing.T) {
	w := &Window{}
	tw1 := NewTweenAnimation("t1", 0, 50, func(float32, *Window) {})
	tw2 := NewTweenAnimation("t1", 0, 100, func(float32, *Window) {})
	w.AnimationAdd(tw1)
	w.AnimationAdd(tw2)
	a := w.animations["t1"].(*TweenAnimation)
	if a.To != 100 {
		t.Error("should replace with new animation")
	}
}

func TestUpdateAnimate(t *testing.T) {
	called := false
	a := &Animate{
		AnimID: "a",
		Callback: func(_ *Animate, _ *Window) {
			called = true
		},
		Delay: 0,
		start: time.Now().Add(-time.Second),
	}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateAnimate(a, &ac)
	if !ok {
		t.Error("should return true")
	}
	runQueuedCommands(deferred)
	if !called {
		t.Error("callback not called")
	}
	if !a.stopped {
		t.Error("should be stopped (no repeat)")
	}
}

func TestUpdateBlinkCursor(t *testing.T) {
	b := NewBlinkCursorAnimation()
	b.start = time.Now().Add(-time.Second)
	w := &Window{}
	ok := updateBlinkCursor(b, w)
	if !ok {
		t.Error("should return true after delay")
	}
}

func TestAnimationAddResumesIdleTicker(t *testing.T) {
	w := &Window{
		windowAnimation: windowAnimation{
			animationResumeCh: make(chan struct{}, 1),
		},
	}
	tw := NewTweenAnimation("t1", 0, 1, func(float32, *Window) {})
	w.AnimationAdd(tw)

	select {
	case <-w.animationResumeCh:
	default:
		t.Error("animationAdd should signal resume when map was empty")
	}
}

func TestAnimationAddNoResumeWhenNotEmpty(t *testing.T) {
	w := &Window{
		windowAnimation: windowAnimation{
			animationResumeCh: make(chan struct{}, 1),
		},
	}
	tw1 := NewTweenAnimation("t1", 0, 1, func(float32, *Window) {})
	tw2 := NewTweenAnimation("t2", 0, 1, func(float32, *Window) {})
	w.AnimationAdd(tw1)
	// Drain the resume signal from first add.
	<-w.animationResumeCh

	w.AnimationAdd(tw2)
	select {
	case <-w.animationResumeCh:
		t.Error("should not signal resume when animations already exist")
	default:
	}
}

func TestWakeMainNilSafe(t *testing.T) {
	w := &Window{}
	// Should not panic when wakeMainFn is nil.
	w.wakeMain()
}

func TestWakeMainCallsFn(t *testing.T) {
	w := &Window{}
	called := false
	w.wakeMainFn = func() { called = true }
	w.wakeMain()
	if !called {
		t.Error("wakeMain should call wakeMainFn")
	}
}

func TestAnimationAddViewBound_PopulatesHeartbeat(t *testing.T) {
	w := &Window{}
	tw := NewTweenAnimation("vb1", 0, 1, func(float32, *Window) {})
	w.animationAddViewBound(tw)
	if _, ok := w.animViewBound["vb1"]; !ok {
		t.Error("animViewBound entry not created")
	}
}

func TestTouchViewBoundAnimation_MissingReturnsFalse(t *testing.T) {
	w := &Window{}
	if w.touchViewBoundAnimation("nonexistent") {
		t.Error("should return false for non-existent animation")
	}
}

func TestTouchViewBoundAnimation_ExistsUpdatesHeartbeat(t *testing.T) {
	w := &Window{}
	tw := NewTweenAnimation("vb2", 0, 1, func(float32, *Window) {})
	w.animationAddViewBound(tw)
	old := w.animViewBound["vb2"]
	time.Sleep(time.Millisecond)
	if !w.touchViewBoundAnimation("vb2") {
		t.Error("should return true for existing view-bound animation")
	}
	if w.animViewBound["vb2"] <= old {
		t.Error("heartbeat should advance after touch")
	}
}

func TestTouchViewBoundAnimation_NonViewBoundReturnsTrue(t *testing.T) {
	w := &Window{}
	tw := NewTweenAnimation("nv1", 0, 1, func(float32, *Window) {})
	w.AnimationAdd(tw)
	if !w.touchViewBoundAnimation("nv1") {
		t.Error("should return true for non-view-bound animation that exists")
	}
	if w.animViewBound != nil {
		if _, ok := w.animViewBound["nv1"]; ok {
			t.Error("non-view-bound animation must not be added to animViewBound")
		}
	}
}

func TestAnimationRemove_CleansViewBound(t *testing.T) {
	w := &Window{}
	tw := NewTweenAnimation("vb3", 0, 1, func(float32, *Window) {})
	w.animationAddViewBound(tw)
	w.AnimationRemove("vb3")
	if w.animViewBound != nil {
		if _, ok := w.animViewBound["vb3"]; ok {
			t.Error("animViewBound entry should be removed by AnimationRemove")
		}
	}
}

func TestViewBoundStaleEviction(t *testing.T) {
	w := &Window{
		windowAnimation: windowAnimation{
			animationStop:     make(chan struct{}),
			animationDone:     make(chan struct{}),
			animationResumeCh: make(chan struct{}, 1),
		},
	}
	anim := &Animate{
		AnimID:   "stale1",
		Delay:    time.Hour,
		Repeat:   true,
		Callback: func(*Animate, *Window) {},
	}
	w.mu.Lock()
	w.animationAddViewBound(anim)
	w.animViewBound["stale1"] = 0 // Unix epoch — always stale
	w.mu.Unlock()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !w.HasAnimation("stale1") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(w.animationStop)
	<-w.animationDone

	if w.HasAnimation("stale1") {
		t.Error("stale view-bound animation should have been evicted by the loop")
	}
}

func TestAnimateRepeatNoDrift(t *testing.T) {
	a := &Animate{
		AnimID:   "drift",
		Delay:    100 * time.Millisecond,
		Repeat:   true,
		Callback: func(*Animate, *Window) {},
	}
	a.start = time.Now().Add(-150 * time.Millisecond)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateAnimate(a, &ac)
	// start should advance by Delay, not reset to Now().
	if a.start.After(time.Now().Add(-10 * time.Millisecond)) {
		t.Error("start should not reset to Now(); should advance by Delay")
	}
}
