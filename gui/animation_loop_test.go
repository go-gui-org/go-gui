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

func TestAnimationAddEmptyIDPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("AnimationAdd with empty ID should panic")
		}
	}()
	w := &Window{}
	w.AnimationAdd(&Animate{Callback: func(*Animate, *Window) {}})
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
	b := newBlinkCursorAnimation()
	b.start = time.Now().Add(-time.Second)
	w := &Window{}
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateBlinkCursor(b, w, &ac)
	if !ok {
		t.Error("should return true after delay")
	}
	if len(deferred) != 1 {
		t.Errorf("toggle should queue the caret patch, got %d", len(deferred))
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

// TestUpdateAnimateRepeatDropsBacklogAfterStall pins the stall behaviour:
// a repeating animation whose start sits far in the past fires once and
// resyncs, rather than draining the missed intervals one per tick. Before
// the resync it queued one callback on every one of these back-to-back
// updates and start stayed seconds behind.
func TestUpdateAnimateRepeatDropsBacklogAfterStall(t *testing.T) {
	a := &Animate{
		AnimID:   "a",
		Delay:    16 * time.Millisecond,
		Repeat:   true,
		Callback: func(*Animate, *Window) {},
	}
	// Five seconds of missed ticks — a minimized window or a debugger
	// break. At one 16ms step per tick that is a backlog of ~312.
	a.SetStart(time.Now().Add(-5 * time.Second))

	deferred := make([]queuedCommand, 0, 16)
	ac := newAnimationCommands(&deferred)
	for range 10 {
		updateAnimate(a, &ac)
	}

	if len(deferred) != 1 {
		t.Errorf("queued %d callbacks, want 1", len(deferred))
	}
	if behind := time.Since(a.start); behind > a.Delay {
		t.Errorf("start still %v behind, want under %v", behind, a.Delay)
	}
	if a.stopped {
		t.Error("repeating animation should not stop")
	}
}

// TestUpdateAnimateRepeatKeepsCadence guards the other side: a tick that
// arrives on time must still step by exactly one Delay, so a long
// interval does not drift toward the tick period.
func TestUpdateAnimateRepeatKeepsCadence(t *testing.T) {
	a := &Animate{
		AnimID:   "a",
		Delay:    100 * time.Millisecond,
		Repeat:   true,
		Callback: func(*Animate, *Window) {},
	}
	start := time.Now().Add(-110 * time.Millisecond)
	a.SetStart(start)

	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	if !updateAnimate(a, &ac) {
		t.Fatal("expected the animation to fire")
	}
	if want := start.Add(a.Delay); !a.start.Equal(want) {
		t.Errorf("start = %v, want %v (one exact Delay step)", a.start, want)
	}
}

// TestUpdateBlinkCursorDropsBacklogAfterStall pins the same rule for the
// caret: one toggle after a stall, not a strobe back to the present.
func TestUpdateBlinkCursorDropsBacklogAfterStall(t *testing.T) {
	w := &Window{}
	b := newBlinkCursorAnimation()
	b.SetStart(time.Now().Add(-5 * time.Second))

	deferred := make([]queuedCommand, 0, 16)
	ac := newAnimationCommands(&deferred)
	for range 10 {
		updateBlinkCursor(b, w, &ac)
	}

	if len(deferred) != 1 {
		t.Errorf("queued %d caret toggles, want 1", len(deferred))
	}
	if !w.viewState.inputCursorOn.Load() {
		t.Error("caret should have toggled exactly once, to visible")
	}
}

// TestViewBoundHeartbeatIgnoresScrubClock pins the virtual clock 10
// minutes in the past (a time-travel scrub), stamps a view-bound
// heartbeat, resumes, and asserts the heartbeat is wall-clock fresh.
// Before the fix the stamp used w.Now(), so resume() compared live time
// against a T-10min stamp and cancelled every visible widget's anim.
func TestViewBoundHeartbeatIgnoresScrubClock(t *testing.T) {
	w := &Window{}
	past := time.Now().Add(-10 * time.Minute)
	w.setVirtualNow(&past)

	tw := NewTweenAnimation("vb-scrub", 0, 1, func(float32, *Window) {})
	w.animationAddViewBound(tw)
	if !w.touchViewBoundAnimation("vb-scrub") {
		w.setVirtualNow(nil)
		t.Fatal("touch should succeed while scrub-pinned")
	}

	seen := w.animViewBound["vb-scrub"]
	if seen-past.UnixNano() < int64(9*time.Minute) {
		w.setVirtualNow(nil)
		t.Errorf("heartbeat followed the scrub clock (seen-past = %v)",
			time.Duration(seen-past.UnixNano()))
	}

	w.setVirtualNow(nil) // resume(): virtual pin cleared, live time back
	seen = w.animViewBound["vb-scrub"]
	if age := viewBoundNow() - seen; age > animViewBoundStale {
		t.Errorf("heartbeat looks stale after resume (age = %v)", time.Duration(age))
	}
}

// TestViewBoundStaleStillEvictsWhilePinned covers the inverse: a
// departed animation (heartbeat 3s old) must still read stale while a
// scrub pin holds w.Now() in the past. A pinned comparison would go
// negative and leak the animation for the scrub's duration.
func TestViewBoundStaleStillEvictsWhilePinned(t *testing.T) {
	w := &Window{}
	tw := NewTweenAnimation("vb-pinned", 0, 1, func(float32, *Window) {})
	w.animationAddViewBound(tw)
	w.animMu.Lock()
	w.animViewBound["vb-pinned"] = time.Now().Add(-3 * time.Second).UnixNano()
	w.animMu.Unlock()

	past := time.Now().Add(-10 * time.Minute)
	w.setVirtualNow(&past)
	defer w.setVirtualNow(nil)

	w.animMu.Lock()
	seen := w.animViewBound["vb-pinned"]
	w.animMu.Unlock()
	if viewBoundNow()-seen <= animViewBoundStale {
		t.Error("stale heartbeat should still read stale while scrub-pinned")
	}
}
