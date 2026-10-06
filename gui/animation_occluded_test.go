package gui

import (
	"testing"
	"time"
)

// stubRenderAnim is an Animation that reports an update every tick and
// queues no callbacks: the shape of a keyframe spinner. Its only output
// is the refresh request, which is what a hidden window must suppress.
type stubRenderAnim struct{ id string }

func (s *stubRenderAnim) ID() string                                       { return s.id }
func (s *stubRenderAnim) RefreshKind() AnimationRefreshKind                { return AnimationRefreshRenderOnly }
func (s *stubRenderAnim) IsStopped() bool                                  { return false }
func (s *stubRenderAnim) SetStart(time.Time)                               {}
func (s *stubRenderAnim) Update(*Window, float32, *AnimationCommands) bool { return true }

// animationTickOnce runs one animation-loop tick with fresh scratch.
func animationTickOnce(w *Window) {
	deferred := make([]queuedCommand, 0, 8)
	stoppedIDs := make([]string, 0, 4)
	ac := newAnimationCommands(&deferred)
	w.animationTick(&ac, &stoppedIDs)
}

// TestAnimationTickZeroAlloc: a tick reuses the loop's scratch. Taking
// the scratch by value, or building AnimationCommands per tick, moves a
// header to the heap every tick: Update takes it through an interface.
func TestAnimationTickZeroAlloc(t *testing.T) {
	w := &Window{}
	w.AnimationAdd(&stubRenderAnim{id: "spin"})
	deferred := make([]queuedCommand, 0, 8)
	stoppedIDs := make([]string, 0, 4)
	ac := newAnimationCommands(&deferred)
	tick := func() {
		w.animationTick(&ac, &stoppedIDs)
		w.flushCommands()
	}
	tick() // grow the command queue once
	if n := testing.AllocsPerRun(100, tick); n != 0 {
		t.Errorf("animationTick allocs/run = %v, want 0", n)
	}
}

// TestAnimationTickOccludedQueuesNoFrame: while the window is hidden a
// tick still runs Update, but queues no refresh and does not wake the
// main thread (issue #943).
func TestAnimationTickOccludedQueuesNoFrame(t *testing.T) {
	w := &Window{}
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })
	w.AnimationAdd(&stubRenderAnim{id: "spin"})
	DispatchWindowOccluded(w, true)
	wakes = 0

	animationTickOnce(w)

	if n := w.pendingCommandCount(); n != 0 {
		t.Errorf("hidden tick queued %d commands, want 0", n)
	}
	if wakes != 0 {
		t.Errorf("hidden tick woke main %d times, want 0", wakes)
	}
}

// TestAnimationTickVisibleQueuesFrame is the control: the same tick on a
// visible window queues the refresh and wakes the main thread.
func TestAnimationTickVisibleQueuesFrame(t *testing.T) {
	w := &Window{}
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })
	w.AnimationAdd(&stubRenderAnim{id: "spin"})

	animationTickOnce(w)

	if n := w.pendingCommandCount(); n != 1 {
		t.Errorf("visible tick queued %d commands, want 1", n)
	}
	if wakes != 1 {
		t.Errorf("visible tick woke main %d times, want 1", wakes)
	}
}

// TestAnimationTickOccludedHoldsStaleTimer: a hidden window builds no
// view, so no widget touches its heartbeat. The tick must hold the
// stale-cancel timer, or every spinner is cancelled after 2s hidden and
// restarts at phase 0 on show.
func TestAnimationTickOccludedHoldsStaleTimer(t *testing.T) {
	w := &Window{}
	w.animationAddViewBound(&stubRenderAnim{id: "spin"})
	w.animMu.Lock()
	w.animViewBound["spin"] = time.Now().Add(-time.Hour)
	w.animMu.Unlock()
	DispatchWindowOccluded(w, true)

	animationTickOnce(w)

	if !w.HasAnimation("spin") {
		t.Fatal("view-bound animation cancelled while window hidden")
	}
	// The heartbeat is renewed, so the first ticks after show do not
	// cancel it before the first frame touches it again.
	w.animMu.Lock()
	seen := w.animViewBound["spin"]
	w.animMu.Unlock()
	if viewBoundNow().Sub(seen) > time.Second {
		t.Errorf("heartbeat not renewed while hidden: %v old", viewBoundNow().Sub(seen))
	}

	// Once shown, the stale check applies again.
	DispatchWindowOccluded(w, false)
	w.animMu.Lock()
	w.animViewBound["spin"] = time.Now().Add(-time.Hour)
	w.animMu.Unlock()
	animationTickOnce(w)
	if w.HasAnimation("spin") {
		t.Error("stale view-bound animation kept after window shown")
	}
}

// TestFrameFnOccludedSkipsRender: a hidden window runs queued commands so
// app callbacks keep their timing, but builds and presents no frame. The
// refresh flag stays set for the frame on show.
func TestFrameFnOccludedSkipsRender(t *testing.T) {
	w := &Window{}
	ran := false
	w.QueueCommand(func(*Window) { ran = true })
	w.markLayoutRefresh()
	DispatchWindowOccluded(w, true)

	if w.FrameFn() {
		t.Error("FrameFn on hidden window returned true, want no render")
	}
	if !ran {
		t.Error("queued command did not run while hidden")
	}
	if !w.refreshLayout.Load() {
		t.Error("layout refresh flag cleared while hidden")
	}
}

// TestDispatchWindowOccludedShowRefreshes: on show the window asks for one
// full frame and wakes the main thread, even when nothing changed while
// hidden.
func TestDispatchWindowOccludedShowRefreshes(t *testing.T) {
	w := &Window{}
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })
	DispatchWindowOccluded(w, true)
	if w.refreshLayout.Load() {
		t.Fatal("hide marked a layout refresh")
	}

	DispatchWindowOccluded(w, false)

	if !w.refreshLayout.Load() {
		t.Error("show did not mark a layout refresh")
	}
	if wakes != 1 {
		t.Errorf("show woke main %d times, want 1", wakes)
	}
	// A repeated show is a no-op: backends may report the same state
	// twice (minimize + occlusion on macOS).
	w.refreshLayout.Store(false)
	DispatchWindowOccluded(w, false)
	if w.refreshLayout.Load() || wakes != 1 {
		t.Error("repeated show refreshed again")
	}
}

func TestDispatchWindowOccludedNilSafe(t *testing.T) {
	DispatchWindowOccluded(nil, true)
}

// stubValueAnim queues an OnValue callback every tick and asks for no
// refresh: the shape of a tween driving app state.
type stubValueAnim struct {
	stubRenderAnim
	onValue func(float32, *Window)
}

func (s *stubValueAnim) Update(_ *Window, _ float32, ac *AnimationCommands) bool {
	ac.AppendOnValue(s.onValue, 1)
	return true
}

// TestAnimationTickOccludedStillQueuesCallbacks: hiding suppresses the
// refresh only. App callbacks keep their timing, and the main thread is
// woken to run them, or the queue would grow for as long as the window
// stays hidden.
func TestAnimationTickOccludedStillQueuesCallbacks(t *testing.T) {
	w := &Window{}
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })
	got := 0
	w.AnimationAdd(&stubValueAnim{
		stubRenderAnim: stubRenderAnim{id: "tween"},
		onValue:        func(float32, *Window) { got++ },
	})
	DispatchWindowOccluded(w, true)
	wakes = 0

	animationTickOnce(w)

	if n := w.pendingCommandCount(); n != 1 {
		t.Errorf("hidden tick queued %d commands, want 1 (callback, no refresh)", n)
	}
	if wakes != 1 {
		t.Errorf("hidden tick woke main %d times, want 1", wakes)
	}
	if w.FrameFn() {
		t.Error("FrameFn on hidden window returned true, want no render")
	}
	if got != 1 {
		t.Errorf("callback ran %d times while hidden, want 1", got)
	}
}
