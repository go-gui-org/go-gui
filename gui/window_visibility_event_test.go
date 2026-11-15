package gui

import (
	"strings"
	"testing"
	"time"
)

// visibilityRecorder counts EventVisibilityChanged on w and records the
// two getters as the handler saw them.
type visibilityRecorder struct {
	events   int
	visible  bool
	occluded bool
}

func recordVisibility(w *Window) *visibilityRecorder {
	r := &visibilityRecorder{}
	w.OnEvent = func(e *Event, win *Window) {
		if e.Type != EventVisibilityChanged {
			return
		}
		r.events++
		r.visible = win.IsVisible()
		r.occluded = win.IsOccluded()
	}
	return r
}

// TestVisibilityEventOncePerChange: the event follows the combined state
// IsVisible() && !IsOccluded(), not each flag. X11 and macOS report an
// occlusion for the app's own Hide and Win32 does not; both orders and
// both shapes must send one event per real change (issue #1170).
func TestVisibilityEventOncePerChange(t *testing.T) {
	type step struct {
		name string
		do   func(w *Window)
		want int // events after the step
	}
	hide := func(w *Window) { w.Hide() }
	show := func(w *Window) { w.Show() }
	occlude := func(w *Window) { DispatchWindowOccluded(w, true) }
	reveal := func(w *Window) { DispatchWindowOccluded(w, false) }
	tests := []struct {
		name  string
		steps []step
	}{
		{"hide only (Win32)", []step{
			{"hide", hide, 1},
			{"hide again", hide, 1},
			{"show", show, 2},
			{"show again", show, 2},
		}},
		{"hide then occluded (X11, macOS)", []step{
			{"hide", hide, 1},
			{"occluded", occlude, 1},
			{"show, still unmapped", show, 1},
			{"mapped", reveal, 2},
		}},
		{"minimize", []step{
			{"occluded", occlude, 1},
			{"occluded again", occlude, 1},
			{"restored", reveal, 2},
			{"restored again", reveal, 2},
		}},
		{"hide while minimized", []step{
			{"occluded", occlude, 1},
			{"hide", hide, 1},
			{"restored, still hidden", reveal, 1},
			{"show", show, 2},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Window{}
			r := recordVisibility(w)
			for _, s := range tt.steps {
				s.do(w)
				if r.events != s.want {
					t.Fatalf("after %q: %d events, want %d", s.name, r.events, s.want)
				}
			}
		})
	}
}

// TestVisibilityEventHandlerReadsNewState: the handler runs inside the
// call that changed the state and reads the new state from the getters.
func TestVisibilityEventHandlerReadsNewState(t *testing.T) {
	w := &Window{}
	r := recordVisibility(w)

	DispatchWindowOccluded(w, true)
	if r.events != 1 || !r.visible || !r.occluded {
		t.Errorf("minimize: events=%d visible=%v occluded=%v, want 1 true true",
			r.events, r.visible, r.occluded)
	}
	if !w.IsOccluded() {
		t.Error("IsOccluded = false after an occluded report")
	}

	DispatchWindowOccluded(w, false)
	if r.events != 2 || !r.visible || r.occluded {
		t.Errorf("restore: events=%d visible=%v occluded=%v, want 2 true false",
			r.events, r.visible, r.occluded)
	}

	w.Hide()
	if r.events != 3 || r.visible || r.occluded {
		t.Errorf("hide: events=%d visible=%v occluded=%v, want 3 false false",
			r.events, r.visible, r.occluded)
	}
}

// occludingPlatform reports an occlusion from inside HideWindow and
// ShowWindow, the way AppKit answers orderOut: before the call returns.
type occludingPlatform struct {
	noopNativePlatform
	w *Window
}

func (p *occludingPlatform) HideWindow() { DispatchWindowOccluded(p.w, true) }
func (p *occludingPlatform) ShowWindow() { DispatchWindowOccluded(p.w, false) }

// TestVisibilityEventNestedOcclusionReport: Hide from a click handler can
// make the OS report the occlusion before Hide returns. The nested
// report sends the one event; Hide's own check then finds nothing new.
func TestVisibilityEventNestedOcclusionReport(t *testing.T) {
	w := &Window{}
	w.nativePlatform = &occludingPlatform{w: w}
	r := recordVisibility(w)

	w.Hide()
	if r.events != 1 || r.visible || !r.occluded {
		t.Errorf("hide: events=%d visible=%v occluded=%v, want 1 false true",
			r.events, r.visible, r.occluded)
	}
	w.Show()
	if r.events != 2 || !r.visible || r.occluded {
		t.Errorf("show: events=%d visible=%v occluded=%v, want 2 true false",
			r.events, r.visible, r.occluded)
	}
}

// TestVisibilityEventReachesUnfocusedWindow: a minimized window lost
// focus first, and on web blur comes before visibilitychange. The event
// must pass the unfocused gate either way.
func TestVisibilityEventReachesUnfocusedWindow(t *testing.T) {
	for _, focused := range []bool{false, true} {
		w := &Window{}
		w.focused = focused
		r := recordVisibility(w)
		DispatchWindowOccluded(w, true)
		if r.events != 1 {
			t.Errorf("focused=%v: %d events, want 1", focused, r.events)
		}
	}
}

func TestVisibilityEventNotSnapshotted(t *testing.T) {
	if shouldSnapshot(EventVisibilityChanged) {
		t.Error("EventVisibilityChanged takes a time-travel snapshot")
	}
}

// TestVisibilityEventDestroyedWindowSilent: a late occlusion report for a
// window the backend already tore down sends nothing.
func TestVisibilityEventDestroyedWindowSilent(t *testing.T) {
	w := &Window{}
	r := recordVisibility(w)
	w.destroyed.Store(true)
	DispatchWindowOccluded(w, true)
	if r.events != 0 {
		t.Errorf("destroyed window got %d events, want 0", r.events)
	}
}

func TestIsOccludedNilSafe(t *testing.T) {
	var w *Window
	if w.IsOccluded() {
		t.Error("IsOccluded = true for nil window, want false")
	}
}

// TestAnimationParksWhileOccludedWithoutWake: a window with no wake
// function (Android, iOS, web) has nobody to run queued callbacks while
// the app is hidden. A view-bound keyframe animation must queue nothing
// then, stay registered, and go on at the phase the clock gives after
// show (issue #1170).
func TestAnimationParksWhileOccludedWithoutWake(t *testing.T) {
	w := &Window{}
	// A manual clock (#1262): the phase after show then depends on the
	// four seconds added below, not on how long the test was scheduled.
	w.startManualClock()
	w.animationResumeCh = make(chan struct{}, 1)
	var got float32 = -1
	kf := &KeyframeAnimation{
		AnimID:    "spin",
		Duration:  10 * time.Second,
		Keyframes: []Keyframe{{At: 0, Value: 0}, {At: 1, Value: 1}},
		OnValue:   func(v float32, _ *Window) { got = v },
	}
	w.animationAddViewBound(kf)
	<-w.animationResumeCh // the add's own resume
	DispatchWindowOccluded(w, true)
	w.flushCommands()

	deferred := make([]queuedCommand, 0, 8)
	stoppedIDs := make([]string, 0, 4)
	ac := newAnimationCommands(&deferred)
	dt := float32(animationCycle.Seconds())
	for range 3 {
		if !w.animationTick(&ac, &stoppedIDs, dt) {
			t.Fatal("hidden tick did not ask the loop to park")
		}
	}
	if n := w.pendingCommandCount(); n != 0 {
		t.Errorf("hidden ticks queued %d commands, want 0", n)
	}
	if !w.HasAnimation("spin") {
		t.Fatal("animation dropped while hidden")
	}

	// Four seconds pass while hidden. No frame touched the heartbeat.
	w.manualOffset.Add(int64(4 * time.Second))
	w.animMu.Lock()
	w.animViewBound["spin"] = w.viewBoundNow().Add(-time.Hour)
	w.animMu.Unlock()

	DispatchWindowOccluded(w, false)
	if len(w.animationResumeCh) != 1 {
		t.Error("show did not resume the animation loop")
	}
	w.flushCommands()

	if w.animationTick(&ac, &stoppedIDs, dt) {
		t.Error("tick after show reported idle")
	}
	if !w.HasAnimation("spin") {
		t.Fatal("animation cancelled as stale on the first tick after show")
	}
	w.flushCommands()
	if got < 0.399 || got > 0.401 {
		t.Errorf("value after show = %v, want 0.4 (4s of 10s)", got)
	}
}

// TestAnimationTicksWhileOccludedWithWake is the control: with a wake
// function the same hidden tick still queues the callback (issue #943).
func TestAnimationTicksWhileOccludedWithWake(t *testing.T) {
	w := &Window{}
	w.SetWakeMainFn(func() {})
	w.animationAddViewBound(&KeyframeAnimation{
		AnimID:    "spin",
		Duration:  10 * time.Second,
		Keyframes: []Keyframe{{At: 0, Value: 0}, {At: 1, Value: 1}},
		OnValue:   func(float32, *Window) {},
	})
	DispatchWindowOccluded(w, true)
	w.flushCommands()

	deferred := make([]queuedCommand, 0, 8)
	stoppedIDs := make([]string, 0, 4)
	ac := newAnimationCommands(&deferred)
	if w.animationTick(&ac, &stoppedIDs, float32(animationCycle.Seconds())) {
		t.Error("hidden tick on a wakeable window reported idle")
	}
	if n := w.pendingCommandCount(); n != 1 {
		t.Errorf("hidden tick queued %d commands, want 1", n)
	}
}

// TestVisibilityEventWakes: a hide asks for no frame and wakes nothing
// (the #943 rule). Show marks the shown refresh and wakes once, so what
// the handlers changed is drawn.
func TestVisibilityEventWakes(t *testing.T) {
	w := &Window{}
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })
	r := recordVisibility(w)

	w.Hide()
	if r.events != 1 || wakes != 0 || w.layoutPending() {
		t.Errorf("hide: events=%d wakes=%d pending=%v, want 1 0 false",
			r.events, wakes, w.layoutPending())
	}
	w.Show()
	if r.events != 2 || wakes != 1 || !w.layoutPending() {
		t.Errorf("show: events=%d wakes=%d pending=%v, want 2 1 true",
			r.events, wakes, w.layoutPending())
	}
}

// TestVisibilityEventHeldWhileFrozen: EventFn drops events while the
// time-travel debugger has the window frozen. A change in that time is
// sent on resume; one that was undone before resume sends nothing.
func TestVisibilityEventHeldWhileFrozen(t *testing.T) {
	w := &Window{}
	r := recordVisibility(w)

	w.frozen.Store(true)
	DispatchWindowOccluded(w, true)
	if r.events != 0 {
		t.Fatalf("frozen window got %d events, want 0", r.events)
	}
	w.resume()
	w.flushCommands()
	if r.events != 1 || !r.occluded {
		t.Errorf("after resume: events=%d occluded=%v, want 1 true", r.events, r.occluded)
	}

	w.frozen.Store(true)
	DispatchWindowOccluded(w, false)
	DispatchWindowOccluded(w, true)
	w.resume()
	w.flushCommands()
	if r.events != 1 {
		t.Errorf("hidden and shown while frozen: %d events, want 1", r.events)
	}
}

// noHidePlatform is a platform with no window manager to hide a window
// with, like web, iOS and Android.
type noHidePlatform struct {
	noopNativePlatform
	hides int
}

func (p *noHidePlatform) HideWindow()         { p.hides++ }
func (p *noHidePlatform) CanHideWindow() bool { return false }

// TestHideWhereNoWindowCanBeHidden: the user still sees the app, so
// IsVisible stays true and no event tells a player to pause. The call
// is reported, so the app author learns why nothing happened.
func TestHideWhereNoWindowCanBeHidden(t *testing.T) {
	buf := captureDebugMask(t, DebugWindowDegraded)
	w := &Window{}
	plat := &noHidePlatform{}
	w.nativePlatform = plat
	r := recordVisibility(w)

	w.Hide()
	if !w.IsVisible() || r.events != 0 || plat.hides != 0 {
		t.Errorf("hide: visible=%v events=%d platform calls=%d, want true 0 0",
			w.IsVisible(), r.events, plat.hides)
	}
	if !strings.Contains(buf.String(), "cannot hide a window") {
		t.Errorf("no finding for the refused Hide: %q", buf.String())
	}
	w.Show()
	if !w.IsVisible() || r.events != 0 {
		t.Errorf("show: visible=%v events=%d, want true 0", w.IsVisible(), r.events)
	}

	// The OS side still counts there: the app going to the background.
	DispatchWindowOccluded(w, true)
	if r.events != 1 {
		t.Errorf("occlusion: %d events, want 1", r.events)
	}
}

// TestHideBeforeBackendWhereNoWindowCanBeHidden: a Hide made before the
// backend attached found no platform to ask and marked the window
// hidden. When a platform that cannot hide attaches, the mark is
// cleared and the app is told, so IsVisible does not stay false for a
// window the user sees.
func TestHideBeforeBackendWhereNoWindowCanBeHidden(t *testing.T) {
	w := &Window{}
	r := recordVisibility(w)

	w.Hide()
	if w.IsVisible() || r.events != 1 {
		t.Fatalf("before attach: visible=%v events=%d, want false 1", w.IsVisible(), r.events)
	}
	w.SetNativePlatform(&noHidePlatform{})
	if !w.IsVisible() || r.events != 2 || !r.visible {
		t.Errorf("after attach: visible=%v events=%d handler saw visible=%v, want true 2 true",
			w.IsVisible(), r.events, r.visible)
	}
}

// TestHideBeforeBackendWhereWindowCanBeHidden is the control: a platform
// that can hide leaves the mark as it was, and sends nothing.
func TestHideBeforeBackendWhereWindowCanBeHidden(t *testing.T) {
	w := &Window{}
	r := recordVisibility(w)

	w.Hide()
	w.SetNativePlatform(&recordingVisibilityPlatform{})
	if w.IsVisible() || r.events != 1 {
		t.Errorf("after attach: visible=%v events=%d, want false 1", w.IsVisible(), r.events)
	}
}
