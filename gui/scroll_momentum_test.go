package gui

import (
	"math"
	"testing"
)

// panWindow builds a window over scrollLayout (a 200 px viewport on 1000
// px of content, scroll id "1") with the shipped line multiplier, so a
// touch path that still took it would move 20x.
func panWindow() *Window {
	w := &Window{}
	w.animations = make(map[string]Animation)
	w.layout = *scrollLayout()
	w.viewState.gesture.nowFn = fixedClock(0)
	pinScrollMultiplier(w, scrollMultiplier)
	return w
}

// panTo moves touch 1 through ys, one move per 16 ms from t0 (nanos),
// after a press at startY. Returns the time of the last move.
func panTo(w *Window, startY float32, t0 int64, ys ...float32) int64 {
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(t0)
	w.handleTouch(&w.layout, touchEvent(EventTouchesBegan, 1, 100, startY))
	t := t0
	for _, y := range ys {
		t += 16_000_000
		gs.nowFn = fixedClock(t)
		w.handleTouch(&w.layout, touchEvent(EventTouchesMoved, 1, 100, y))
	}
	return t
}

func offsetY(w *Window) float32 {
	v, _ := w.scrollY().Get("1")
	return v
}

// TestPanScrollFollowsFinger checks the content moves exactly as far as
// the finger, whatever ScrollMultiplier says (#1110). The pan is
// recognized past 10 px; its first move catches up the distance to
// there, so the content stays under the finger, and each move after
// that scrolls by its own distance.
func TestPanScrollFollowsFinger(t *testing.T) {
	w := panWindow()
	// 150 → 139 crosses the threshold; nothing scrolls yet.
	panTo(w, 150, 0, 139)
	if got := offsetY(w); got != 0 {
		t.Fatalf("offset at recognition = %v, want 0", got)
	}
	gs := &w.viewState.gesture
	tt := int64(16_000_000)
	move := func(y float32) {
		tt += 16_000_000
		gs.nowFn = fixedClock(tt)
		w.handleTouch(&w.layout, touchEvent(EventTouchesMoved, 1, 100, y))
	}
	// The first move catches up to the finger: 150 → 138 is 12 px.
	move(138)
	if got := offsetY(w); got != -12 {
		t.Fatalf("offset after the first move = %v, want -12 (the finger's travel)", got)
	}
	// Then 1:1: nine 1 px moves add nine pixels, not 9 x 20.
	for y := float32(137); y >= 129; y-- {
		move(y)
	}
	if got := offsetY(w); got != -21 {
		t.Fatalf("offset after 21 px of finger = %v, want -21", got)
	}
}

// TestPanLiftFlings checks a lift while the finger moves keeps the
// content gliding the same way, slowing, and stopping (#1110).
func TestPanLiftFlings(t *testing.T) {
	w := panWindow()
	last := panTo(w, 190, 0, 179, 169, 159, 149, 139)
	atLift := offsetY(w)
	vel := w.viewState.gesture.velocityY
	w.viewState.gesture.nowFn = fixedClock(last)
	w.handleTouch(&w.layout, touchEvent(EventTouchesEnded, 1, 100, 139))

	e := w.scrollSmooth.findEntry("1", scrollAxisY)
	if e == nil || !e.active || e.tau != scrollMomentumTau {
		t.Fatalf("no fling armed after a fast lift: %+v", e)
	}
	// The glide covers v·tau from the lift.
	if want := atLift + vel*scrollMomentumTau; math.Abs(float64(e.target-want)) > 0.01 {
		t.Fatalf("fling target = %v, want %v (v %v x tau)", e.target, want, vel)
	}
	driveScrollSmooth(w, 1000)
	end := offsetY(w)
	if end >= atLift {
		t.Fatalf("content did not glide on: %v at lift, %v after", atLift, end)
	}
	if f32Abs(end-e.target) > scrollSmoothSnap {
		t.Fatalf("glide ended at %v, want its target %v", end, e.target)
	}
}

// TestPanStaleLiftDoesNotFling checks a finger held still before the
// lift leaves the content where it is.
func TestPanStaleLiftDoesNotFling(t *testing.T) {
	w := panWindow()
	last := panTo(w, 190, 0, 179, 169, 159)
	w.viewState.gesture.nowFn = fixedClock(last + 2*int64(gestureSwipeStale))
	w.handleTouch(&w.layout, touchEvent(EventTouchesEnded, 1, 100, 159))
	if ss := w.scrollSmooth; ss != nil {
		if e := ss.findEntry("1", scrollAxisY); e != nil && e.active {
			t.Fatalf("stale lift flung: %+v", e)
		}
	}
}

// TestTouchCatchesFling checks a new touch stops gliding content where
// it is, and a tap does not restart it.
func TestTouchCatchesFling(t *testing.T) {
	w := panWindow()
	last := panTo(w, 190, 0, 179, 169, 159, 149, 139)
	w.viewState.gesture.nowFn = fixedClock(last)
	w.handleTouch(&w.layout, touchEvent(EventTouchesEnded, 1, 100, 139))
	driveScrollSmooth(w, 3) // part of the glide
	caught := offsetY(w)

	w.viewState.gesture.nowFn = fixedClock(last + 100_000_000)
	w.handleTouch(&w.layout, touchEvent(EventTouchesBegan, 2, 50, 50))
	if e := w.scrollSmooth.findEntry("1", scrollAxisY); e != nil && e.active {
		t.Fatalf("touch did not stop the fling: %+v", e)
	}
	w.handleTouch(&w.layout, touchEvent(EventTouchesEnded, 2, 50, 50))
	driveScrollSmooth(w, 100)
	if got := offsetY(w); got != caught {
		t.Fatalf("content moved after the catch: %v, want %v", got, caught)
	}
}

// TestFlingStopsAtEdge checks a fling toward a near edge keeps its speed
// and stops on the edge, instead of slowing early because the edge is
// close.
func TestFlingStopsAtEdge(t *testing.T) {
	w := panWindow()
	ly, _ := findScrollLayout(w, "1")
	maxOffset := scrollMaxOffsetY(ly)
	w.scrollY().Set("1", maxOffset+5) // 5 px from the end
	if !scrollMomentumStart(w, "1", scrollAxisY, -3000) {
		t.Fatal("fling toward the end did not start")
	}
	deferred := make([]queuedCommand, 0, 2)
	ac := newAnimationCommands(&deferred)
	w.scrollSmooth.Update(w, 0.016, &ac)
	e := w.scrollSmooth.findEntry("1", scrollAxisY)
	// One 16 ms tick at ~3000 px/s covers far more than 5 px: the edge
	// is reached in the first tick, not approached.
	if e.current != maxOffset || !e.settled {
		t.Fatalf("after one tick current = %v settled %v, want the edge %v",
			e.current, e.settled, maxOffset)
	}
	for i := range deferred { // the main-goroutine apply
		if deferred[i].windowFn != nil {
			deferred[i].windowFn(w)
		}
	}
	if got := offsetY(w); got != maxOffset {
		t.Fatalf("applied offset = %v, want the edge %v", got, maxOffset)
	}
	// Resting on an edge, a fling into it does nothing.
	driveScrollSmooth(w, 10)
	if scrollMomentumStart(w, "1", scrollAxisY, -3000) {
		t.Fatal("fling into the edge it rests on started")
	}
}

// TestFlingThresholds checks a slow lift does not fling and a wild one
// is capped.
func TestFlingThresholds(t *testing.T) {
	w := panWindow()
	w.scrollY().Set("1", -400)
	if scrollMomentumStart(w, "1", scrollAxisY, scrollMomentumMinVelocity/2) {
		t.Fatal("a lift below the minimum velocity flung")
	}
	if !scrollMomentumStart(w, "1", scrollAxisY, 1e9) {
		t.Fatal("a fast lift did not fling")
	}
	e := w.scrollSmooth.findEntry("1", scrollAxisY)
	if want := -400 + scrollMomentumMaxVelocity*scrollMomentumTau; e.target != want {
		t.Fatalf("capped target = %v, want %v", e.target, want)
	}
}

// TestWheelTakesOverFling checks a wheel notch during a fling eases from
// where the content is, not from the far point the fling aimed at.
func TestWheelTakesOverFling(t *testing.T) {
	w := panWindow()
	w.scrollY().Set("1", -400)
	scrollMomentumStart(w, "1", scrollAxisY, -2000)
	driveScrollSmooth(w, 2)
	at := offsetY(w)
	ly, _ := findScrollLayout(w, "1")
	if !scrollSmoothBy(w, ly, scrollAxisY, 1) { // one line up
		t.Fatal("wheel during a fling did nothing")
	}
	e := w.scrollSmooth.findEntry("1", scrollAxisY)
	if e.tau != 0 {
		t.Fatal("wheel left the entry flinging")
	}
	if want := at + scrollMultiplier; e.target != want {
		t.Fatalf("wheel target = %v, want %v (the displayed offset plus one line)", e.target, want)
	}
}

// TestScrollMomentumStep checks the decay covers 1-1/e of the distance
// in tau, at any tick rate.
func TestScrollMomentumStep(t *testing.T) {
	got := scrollMomentumStep(scrollMomentumTau, scrollMomentumTau)
	if want := float32(1 - 1/math.E); math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("step over tau = %v, want %v", got, want)
	}
	// Four 4 ms ticks cover what one 16 ms tick does.
	one := scrollMomentumStep(0.016, scrollMomentumTau)
	s := scrollMomentumStep(0.004, scrollMomentumTau)
	four := 1 - (1-s)*(1-s)*(1-s)*(1-s)
	if math.Abs(float64(one-four)) > 1e-6 {
		t.Fatalf("16 ms step %v, four 4 ms steps %v", one, four)
	}
	if got := scrollMomentumStep(float32(math.NaN()), scrollMomentumTau); !(got > 0 && got < 1) {
		t.Fatalf("NaN dt step = %v, want the nominal 16 ms step", got)
	}
}

// flingArmed reports whether a fling is running on scroll id "1".
func flingArmed(w *Window) bool {
	if w.scrollSmooth == nil {
		return false
	}
	e := w.scrollSmooth.findEntry("1", scrollAxisY)
	return e != nil && e.active && e.tau > 0
}

// TestSlowDragDoesNotFling checks the catch-up at pan recognition is
// not timed as one frame. A finger that crawled 11 px in a second, then
// moved 1 px, moves at ~12 px/s; counting the 12 px catch-up as one
// 16 ms frame read it as 150 px/s and flung 60 px.
func TestSlowDragDoesNotFling(t *testing.T) {
	w := panWindow()
	gs := &w.viewState.gesture
	w.handleTouch(&w.layout, touchEvent(EventTouchesBegan, 1, 100, 150))
	gs.nowFn = fixedClock(1_000_000_000)
	w.handleTouch(&w.layout, touchEvent(EventTouchesMoved, 1, 100, 139))
	gs.nowFn = fixedClock(1_016_000_000)
	w.handleTouch(&w.layout, touchEvent(EventTouchesMoved, 1, 100, 138))
	// The catch-up still scrolls: the content stays under the finger.
	if got := offsetY(w); got != -12 {
		t.Fatalf("offset = %v, want -12", got)
	}
	if v := f32Abs(gs.velocityY); v >= scrollMomentumMinVelocity {
		t.Fatalf("velocity of a crawling finger = %v px/s, want under %v",
			v, scrollMomentumMinVelocity)
	}
	w.handleTouch(&w.layout, touchEvent(EventTouchesEnded, 1, 100, 138))
	if flingArmed(w) {
		t.Fatal("a slow drag flung")
	}
}

// TestHandoffDoesNotFlingOldPan checks a finger left behind by another
// does not inherit that finger's scroll container. A pans the list, B
// lands outside it, A lifts, B pans fast where nothing scrolls and
// lifts: the list must not fling with B's velocity.
func TestHandoffDoesNotFlingOldPan(t *testing.T) {
	w := panWindow()
	gs := &w.viewState.gesture
	// A pans the list (it covers y 0..200).
	tm := panTo(w, 150, 0, 139, 129, 119)
	if offsetY(w) == 0 {
		t.Fatal("setup: A did not scroll the list")
	}
	// B lands below the list; A lifts.
	tm += 16_000_000
	gs.nowFn = fixedClock(tm)
	w.handleTouch(&w.layout, twoTouchEvent(EventTouchesBegan,
		1, 100, 119, 2, 100, 300))
	tm += 16_000_000
	gs.nowFn = fixedClock(tm)
	w.handleTouch(&w.layout, &Event{
		Type: EventTouchesEnded, NumTouches: 1,
		Touches: [8]TouchPoint{{Identifier: 1, PosX: 100, PosY: 119,
			ToolType: TouchToolFinger, Changed: true}},
	})
	// B flicks where nothing scrolls, and lifts while moving.
	for _, y := range []float32{320, 340, 360, 380} {
		tm += 16_000_000
		gs.nowFn = fixedClock(tm)
		w.handleTouch(&w.layout, touchEvent(EventTouchesMoved, 2, 100, y))
	}
	w.handleTouch(&w.layout, touchEvent(EventTouchesEnded, 2, 100, 380))
	if flingArmed(w) {
		t.Fatal("B's lift flung the list A had scrolled")
	}
}
