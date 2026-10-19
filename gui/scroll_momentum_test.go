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
	_, vel, ok := w.viewState.gesture.panReleaseVelocity()
	if !ok {
		t.Fatal("no release velocity after five moves")
	}
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
	// One 16 ms tick at ~3000 px/s covers far more than 5 px: the edge
	// is reached in the first frame, not approached, and the applied
	// offset never passes it.
	driveScrollSmooth(w, 1)
	if got := offsetY(w); got != maxOffset {
		t.Fatalf("offset after one frame = %v, want the edge %v", got, maxOffset)
	}
	if flingArmed(w) {
		t.Fatal("fling still running on the edge")
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

// TestScrollEaseStep checks the shared ease: tau's step covers 1-1/e of
// the distance, the step is the same in time at any tick rate, and the
// wheel's time constant keeps its 25% per 16 ms.
func TestScrollEaseStep(t *testing.T) {
	got := scrollEaseStep(scrollMomentumTau, scrollMomentumTau)
	if want := float32(1 - 1/math.E); math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("step over tau = %v, want %v", got, want)
	}
	// Four 4 ms ticks cover what one 16 ms tick does.
	one := scrollEaseStep(0.016, scrollMomentumTau)
	s := scrollEaseStep(0.004, scrollMomentumTau)
	four := 1 - (1-s)*(1-s)*(1-s)*(1-s)
	if math.Abs(float64(one-four)) > 1e-6 {
		t.Fatalf("16 ms step %v, four 4 ms steps %v", one, four)
	}
	if got := scrollEaseStep(float32(math.NaN()), scrollMomentumTau); math.Abs(float64(got-one)) > 1e-6 {
		t.Fatalf("NaN dt step = %v, want the nominal 16 ms step %v", got, one)
	}
	if got := scrollEaseStep(0.016, scrollSmoothTau); math.Abs(float64(got-scrollSmoothFactor)) > 1e-6 {
		t.Fatalf("wheel step per 16 ms = %v, want %v", got, scrollSmoothFactor)
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

// TestSlowDragDoesNotFling checks the catch-up at pan recognition does
// not count as speed. A finger that crawled 11 px in a second, then moved
// 1 px, moves at ~12 px/s; the pan's smoothed velocity reads the 12 px
// catch-up as one frame (150 px/s), which flung 60 px. The release
// velocity reads the finger's real positions.
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
	if _, vy, _ := gs.panReleaseVelocity(); f32Abs(vy) >= scrollMomentumMinVelocity {
		t.Fatalf("release velocity of a crawling finger = %v px/s, want under %v",
			vy, scrollMomentumMinVelocity)
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

// TestFlickReleaseVelocity checks a short fast flick reads at its speed:
// three 32 px moves 16 ms apart are 2000 px/s. The pan's smoothed
// velocity, seeded at 0, read about half that and glided half as far.
func TestFlickReleaseVelocity(t *testing.T) {
	w := panWindow()
	panTo(w, 190, 0, 158, 126, 94)
	_, vy, ok := w.viewState.gesture.panReleaseVelocity()
	if !ok || math.Abs(float64(vy+2000)) > 1 {
		t.Fatalf("release velocity = %v, %v; want -2000 px/s", vy, ok)
	}
}

// TestReleaseVelocityWindow checks only the last 100 ms count: a slow
// start does not dilute a fast end.
func TestReleaseVelocityWindow(t *testing.T) {
	var gs gestureState
	gs.panSampleReset(0, 0, 0)
	gs.panSampleAdd(500_000_000, 0, 10) // a slow half second
	gs.panSampleAdd(516_000_000, 0, 42) // then 2000 px/s
	gs.panSampleAdd(532_000_000, 0, 74)
	gs.panSampleAdd(548_000_000, 0, 106)
	if _, vy, _ := gs.panReleaseVelocity(); math.Abs(float64(vy-2000)) > 1 {
		t.Fatalf("release velocity = %v, want 2000 px/s", vy)
	}
	gs.panSampleReset(0, 0, 0)
	if _, _, ok := gs.panReleaseVelocity(); ok {
		t.Fatal("one sample gave a velocity")
	}
}

// TestRejectedWheelKeepsFling checks a wheel request the smoother turns
// down leaves a fling a fling. Making it a wheel ease first sent it on
// toward the fling's unclamped target with no edge stop.
func TestRejectedWheelKeepsFling(t *testing.T) {
	w := panWindow()
	w.scrollY().Set("1", -700)
	if !scrollMomentumStart(w, "1", scrollAxisY, -3000) {
		t.Fatal("setup: no fling")
	}
	ly, _ := findScrollLayout(w, "1")
	maxOffset := scrollMaxOffsetY(ly)
	e := w.scrollSmooth.findEntry("1", scrollAxisY)
	// A zero step lands on the current offset: rejected.
	if scrollSmoothArm(w, "1", scrollAxisY, offsetY(w), maxOffset, 0, true) {
		t.Fatal("a no-op wheel request was accepted")
	}
	if e.tau != scrollMomentumTau {
		t.Fatalf("rejected request turned the fling into a wheel ease (tau %v)", e.tau)
	}
	driveScrollSmooth(w, 1000)
	if got := offsetY(w); got < maxOffset || got > 0 {
		t.Fatalf("offset %v left the range [%v, 0]", got, maxOffset)
	}
}

// TestFlingShiftKeepsDirection checks a scroll-anchor shift mid-fling
// moves the glide with the content. Shifting only the displayed offset
// left the target behind, and the glide turned back toward it.
func TestFlingShiftKeepsDirection(t *testing.T) {
	w := panWindow()
	w.scrollY().Set("1", -100)
	scrollMomentumStart(w, "1", scrollAxisY, -1000) // aims at -500
	driveScrollSmooth(w, 2)
	// Rows above measured taller: the content moved 500 px down.
	scrollSmoothShiftY(w, "1", -500)
	w.scrollY().Set("1", offsetY(w)-500)
	prev := offsetY(w)
	for range 200 {
		driveScrollSmooth(w, 1)
		got := offsetY(w)
		if got > prev {
			t.Fatalf("glide reversed after the shift: %v then %v", prev, got)
		}
		prev = got
	}
}

// TestFlingFollowsGrowingContent checks a fling glides on into rows a
// list appended, instead of stopping on the end it saw at the lift.
func TestFlingFollowsGrowingContent(t *testing.T) {
	w := panWindow()
	w.scrollY().Set("1", -600) // 200 px from the end (-800)
	scrollMomentumStart(w, "1", scrollAxisY, -3000)
	driveScrollSmooth(w, 1)
	// The list appends rows: the end moves to -2800.
	w.layout.Children[0].Children[0].Shape.Height = 3000
	driveScrollSmooth(w, 1000)
	if got := offsetY(w); got >= -800 {
		t.Fatalf("fling stopped at %v, on the old end (-800)", got)
	}
}

// TestFlingStopsOnShrunkContent checks a fling stops on the new end of
// content that shrank under it, instead of writing offsets past it.
func TestFlingStopsOnShrunkContent(t *testing.T) {
	w := panWindow()
	w.scrollY().Set("1", -300)
	scrollMomentumStart(w, "1", scrollAxisY, -1000)
	driveScrollSmooth(w, 1)
	// Rows are removed: the end moves up to -100.
	w.layout.Children[0].Children[0].Shape.Height = 300
	driveScrollSmooth(w, 1)
	if got := offsetY(w); got != -100 {
		t.Fatalf("offset after the shrink = %v, want the new end -100", got)
	}
	if flingArmed(w) {
		t.Fatal("fling kept running past the new end")
	}
}

// TestPaletteShowStopsFling checks a direct offset write ends a running
// fling instead of being overwritten on the next tick: the command
// palette opens at the top.
func TestPaletteShowStopsFling(t *testing.T) {
	w := panWindow()
	id := ScopeID("pal", "scroll")
	w.animMu.Lock()
	w.scrollSmooth = &scrollSmoothAnimation{}
	e := w.scrollSmooth.entryFor(id, scrollAxisY)
	e.active, e.tau, e.current, e.target = true, scrollMomentumTau, -50, -400
	w.animMu.Unlock()
	commandPaletteShow("pal", w)
	if e := w.scrollSmooth.findEntry(id, scrollAxisY); e.active {
		t.Fatal("opening the palette left its fling running")
	}
	driveScrollSmooth(w, 10)
	if got := w.scrollY().GetOr(id, 1); got != 0 {
		t.Fatalf("palette offset = %v, want 0", got)
	}
}

// TestFlingCrossesOldEndAfterGrowth checks the boundary timing of a list
// that grows as a fling reaches its old end: the rows are appended, then
// the next tick crosses the old end before any apply has read the new
// one. Clamping in the tick against the end seen at the lift settled
// the fling there and retired it.
func TestFlingCrossesOldEndAfterGrowth(t *testing.T) {
	w := panWindow()
	w.scrollY().Set("1", -795) // 5 px from the end (-800)
	if !scrollMomentumStart(w, "1", scrollAxisY, -3000) {
		t.Fatal("setup: no fling")
	}
	// Rows are appended before the first tick: the end moves to -2800.
	w.layout.Children[0].Children[0].Shape.Height = 3000
	driveScrollSmooth(w, 1000)
	if got := offsetY(w); got >= -800 {
		t.Fatalf("fling stopped at %v, on the old end (-800)", got)
	}
}
