package gui

import (
	"testing"
	"time"
)

// focusedBlinkWindow returns a window whose input "in" holds focus
// and whose caret-blink animation is registered, the state an app
// sits in after SetFocus on an input (issue #929).
func focusedBlinkWindow(t *testing.T) *Window {
	t.Helper()
	w := newTestWindow()
	w.layout = imeEditTargetLayout("in")
	w.SetFocus("in")
	w.syncBlinkCursor()
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation missing for a focused input")
	}
	return w
}

// blinkTick runs one animation-loop tick for the blink animation the
// way animationLoop does: Update under animMu, then the stopped
// animation is dropped from the map. It returns the queued commands.
func blinkTick(w *Window) []queuedCommand {
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	w.animMu.Lock()
	defer w.animMu.Unlock()
	a, ok := w.animations[blinkCursorAnimationID]
	if !ok {
		return deferred
	}
	a.Update(w, 0, &ac)
	if a.IsStopped() {
		delete(w.animations, blinkCursorAnimationID)
	}
	return deferred
}

// ageBlinkActivity moves the blink animation's last caret activity
// back by d, as if the user had not touched the caret for that long.
func ageBlinkActivity(t *testing.T, w *Window, d time.Duration) {
	t.Helper()
	w.animMu.Lock()
	defer w.animMu.Unlock()
	b, ok := w.animations[blinkCursorAnimationID].(*BlinkCursorAnimation)
	if !ok {
		t.Fatal("blink animation missing")
	}
	b.activity = b.activity.Add(-d)
}

// After blinkCursorIdleTimeout without caret activity the caret goes
// solid and the animation retires, so an idle window with a focused
// input stops rendering twice a second (issue #929).
func TestBlinkCursorParksAfterIdleTimeout(t *testing.T) {
	w := focusedBlinkWindow(t)
	// Caret caught mid-blink: parking must leave it visible.
	w.viewState.inputCursorOn.Store(false)
	ageBlinkActivity(t, w, blinkCursorIdleTimeout+time.Second)

	deferred := blinkTick(w)

	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation still registered after idle timeout")
	}
	if !w.inputCursorOn() {
		t.Fatal("caret left hidden when the blink parked")
	}
	if len(deferred) != 1 {
		t.Fatalf("parking queued %d commands, want the 1 caret patch",
			len(deferred))
	}
	// The per-frame gate must not re-register the animation it just
	// retired: that would restart the 600 ms wakeups on the next frame.
	w.syncBlinkCursor()
	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("frame sync re-registered a parked blink animation")
	}
}

// Short of the timeout the blink keeps running.
func TestBlinkCursorKeepsBlinkingBeforeIdleTimeout(t *testing.T) {
	w := focusedBlinkWindow(t)
	ageBlinkActivity(t, w, blinkCursorIdleTimeout-time.Second)

	blinkTick(w)

	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation retired before the idle timeout")
	}
}

// Caret activity (typing, clicking, dragging in the field) wakes a
// parked caret: the next frame registers the animation again.
func TestBlinkCursorResumesOnCaretActivity(t *testing.T) {
	w := focusedBlinkWindow(t)
	ageBlinkActivity(t, w, blinkCursorIdleTimeout+time.Second)
	blinkTick(w)

	resetBlinkCursorVisible(w)
	w.syncBlinkCursor()

	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("caret activity did not resume a parked blink")
	}
}

// Moving focus to another input resumes the blink: the new caret
// starts its own idle period.
func TestBlinkCursorResumesOnFocusChange(t *testing.T) {
	w := focusedBlinkWindow(t)
	ageBlinkActivity(t, w, blinkCursorIdleTimeout+time.Second)
	blinkTick(w)

	w.layout = imeEditTargetLayout("other")
	w.SetFocus("other")
	w.syncBlinkCursor()

	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("focus change did not resume a parked blink")
	}
}

// A focus change while the caret still blinks restarts the idle
// period too: the new caret must not inherit the old caret's
// untouched time and park early (9 s idle in "in", Tab, then park
// 1 s later in "other").
func TestBlinkCursorFocusChangeRestartsIdlePeriod(t *testing.T) {
	w := focusedBlinkWindow(t)
	ageBlinkActivity(t, w, blinkCursorIdleTimeout+time.Second)

	w.layout = imeEditTargetLayout("other")
	w.SetFocus("other")
	w.syncBlinkCursor()
	blinkTick(w)

	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("new focus parked on the previous caret's idle time")
	}
}

// Consumers re-assert focus from inside View, which runs on every
// rebuild. Only a real focus change resumes the blink; a re-assert on
// the widget that already holds focus must leave the park alone, or
// the idle timeout would never hold for those apps.
func TestBlinkCursorStaysParkedOnFocusReassert(t *testing.T) {
	w := focusedBlinkWindow(t)
	ageBlinkActivity(t, w, blinkCursorIdleTimeout+time.Second)
	blinkTick(w)

	w.SetFocus("in")
	w.syncBlinkCursor()

	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("focus re-assert resumed a parked blink")
	}
}

// Window refocus resumes the blink, like caret activity.
func TestBlinkCursorResumesOnWindowRefocus(t *testing.T) {
	w := focusedBlinkWindow(t)
	ageBlinkActivity(t, w, blinkCursorIdleTimeout+time.Second)
	blinkTick(w)

	w.handleUnfocusedEvent()
	w.syncBlinkCursor()
	w.handleFocusedEvent()
	w.syncBlinkCursor()

	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("window refocus did not resume a parked blink")
	}
}

// A Pulsar reuses the blink animation as its clock and is not a
// caret, so the idle timeout must not stop it.
func TestBlinkCursorIdleTimeoutSparesPulsar(t *testing.T) {
	w := focusedBlinkWindow(t)
	w.animationAddViewBound(&Animate{
		AnimID: pulsarAnimationID,
		Delay:  blinkCursorAnimationDelay,
		Repeat: true,
	})
	ageBlinkActivity(t, w, blinkCursorIdleTimeout+time.Second)

	blinkTick(w)

	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("idle timeout stopped the blink a Pulsar depends on")
	}
}

// A Pulsar mounted while the caret is parked re-registers the blink
// animation and toggles the shared inputCursorOn. When it unmounts,
// the frame sync retires the animation again; the parked caret must
// come back solid, not stay hidden at whatever phase the last
// toggle left.
func TestBlinkCursorParkedCaretSolidAfterPulsarUnmount(t *testing.T) {
	w := focusedBlinkWindow(t)
	ageBlinkActivity(t, w, blinkCursorIdleTimeout+time.Second)
	blinkTick(w)

	// Pulsar mounts: same registrations view_pulsar.go makes.
	w.AnimationAdd(newBlinkCursorAnimation())
	w.animationAddViewBound(&Animate{
		AnimID: pulsarAnimationID,
		Delay:  blinkCursorAnimationDelay,
		Repeat: true,
	})
	// Its blink tick leaves the caret phase off.
	w.viewState.inputCursorOn.Store(false)
	// Pulsar unmounts: its view-bound heartbeat is evicted.
	w.AnimationRemove(pulsarAnimationID)

	w.syncBlinkCursor()

	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation kept for a parked caret after Pulsar left")
	}
	if !w.inputCursorOn() {
		t.Fatal("parked caret left hidden after Pulsar unmount")
	}
}
