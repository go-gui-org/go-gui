package gui

import "testing"

// MouseCancel is the seam backends use when the platform revokes
// mouse capture mid-drag (issue #110). It must unlock unconditionally
// and let the widget unwind without committing the drag.

func TestMouseCancelRunsHookAndUnlocks(t *testing.T) {
	t.Parallel()
	w := &Window{}
	cancelled := 0
	w.MouseLock(MouseLockCfg{
		MouseMove: func(EventCtx) {},
		MouseUp:   func(EventCtx) { t.Error("MouseUp ran on cancel") },
		Cancel:    func(*Window) { cancelled++ },
	})
	w.MouseCancel()
	if cancelled != 1 {
		t.Errorf("Cancel ran %d times, want 1", cancelled)
	}
	if w.mouseIsLocked() {
		t.Error("still locked after MouseCancel")
	}
}

func TestMouseCancelUnlocksWithoutHook(t *testing.T) {
	t.Parallel()
	w := &Window{}
	w.MouseLock(MouseLockCfg{MouseMove: func(EventCtx) {}})
	w.MouseCancel()
	if w.mouseIsLocked() {
		t.Error("a lock with no Cancel hook must still unlock")
	}
}

// The lock is cleared before the hook runs, so hooks that unlock
// themselves — the escape-key cancel paths do — stay correct.
func TestMouseCancelClearsLockBeforeHook(t *testing.T) {
	t.Parallel()
	w := &Window{}
	lockedInHook := true
	w.MouseLock(MouseLockCfg{
		MouseMove: func(EventCtx) {},
		Cancel: func(cw *Window) {
			lockedInHook = cw.mouseIsLocked()
			cw.MouseUnlock() // idempotent, as the real hooks do
		},
	})
	w.MouseCancel()
	if lockedInHook {
		t.Error("hook observed the lock still set")
	}
	if w.mouseIsLocked() {
		t.Error("still locked after MouseCancel")
	}
}

func TestMouseCancelUnlockedIsNoOp(t *testing.T) {
	t.Parallel()
	w := &Window{}
	w.MouseCancel() // must not panic
	if w.mouseIsLocked() {
		t.Error("MouseCancel locked an unlocked window")
	}
}

// A second cancel must not re-run the hook: capture loss can be
// reported more than once, and dock/reorder unwinds are not idempotent
// with respect to their own UpdateWindow ordering.
func TestMouseCancelTwiceRunsHookOnce(t *testing.T) {
	t.Parallel()
	w := &Window{}
	runs := 0
	w.MouseLock(MouseLockCfg{
		MouseMove: func(EventCtx) {},
		Cancel:    func(*Window) { runs++ },
	})
	w.MouseCancel()
	w.MouseCancel()
	if runs != 1 {
		t.Errorf("Cancel ran %d times, want 1", runs)
	}
}

// A cancelled drag must not keep receiving moves — the stuck-drag
// symptom in issue #110 was exactly this.
func TestMouseCancelStopsMoveDelivery(t *testing.T) {
	t.Parallel()
	w := &Window{windowWidth: 800, windowHeight: 600}
	moves := 0
	w.MouseLock(MouseLockCfg{
		MouseMove: func(EventCtx) { moves++ },
	})
	root := &Layout{Shape: &Shape{}}
	mouseMoveHandler(root, &Event{MouseX: 10, MouseY: 10}, w)
	w.MouseCancel()
	mouseMoveHandler(root, &Event{MouseX: 20, MouseY: 20}, w)
	if moves != 1 {
		t.Errorf("MouseMove ran %d times, want 1 (pre-cancel only)", moves)
	}
}
