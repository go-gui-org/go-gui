package gui

import (
	"math"
	"time"
)

// Gesture recognition thresholds. Timing values are time.Duration;
// compare them against monotonic nanos via int64().
const (
	gestureTapTimeout   = 300 * time.Millisecond
	gestureDoubleTapGap = 300 * time.Millisecond
	gestureLongPressDur = 500 * time.Millisecond
	// gestureSwipeStale bounds how long after the last pan move a
	// lift still counts as a swipe. Past it the finger was held
	// still, so the EMA velocity is stale and the lift ends the
	// pan instead of flinging.
	gestureSwipeStale              = 100 * time.Millisecond
	gesturePanDist         float32 = 10
	gesturePinchDist       float32 = 5
	gestureRotateAngle     float32 = 0.087 // ~5 degrees
	gestureSwipeVelocity   float32 = 500   // px/s
	gestureDoubleTapRadius float32 = 30
	gestureVelocitySmooth  float32 = 0.2 // EMA factor

	gestureLongPressAnimID = "__gesture_long_press__"
)

// gestureState is per-window gesture recognizer state.
// Stored directly on ViewState. Zero value = idle.
type gestureState struct {

	// Clock injection for tests (nil = time.Now).
	nowFn      func() int64
	touches    [8]trackedTouch
	numTouches int
	// overflowTouches counts distinct touches dropped while the
	// tracked set was full, so their later release still balances
	// the count instead of wedging numTouches above zero.
	overflowTouches int

	// Timing (monotonic nanos).
	beganTime   int64
	lastTapTime int64
	prevTime    int64

	// Flags.
	singleTouchID uint64
	lastTapX      float32
	lastTapY      float32

	// Pan/swipe.
	startX, startY float32
	prevX, prevY   float32
	velocityX      float32
	velocityY      float32

	// Long-press arm-time snapshots (avoid closure allocation).
	longPressSX, longPressSY float32

	// Pinch.
	initialSpan float32
	prevSpan    float32
	scale       float32

	// Rotate.
	initialAngle float32
	prevAngle    float32
	rotation     float32

	// Phase tracking.
	gestureType GestureType

	mouseEmitted bool
	recognized   bool
	// pinchBegan mirrors rotateBegan: pinch and rotate are
	// independent sub-states of one multi-touch, tracked
	// separately so both get their Began and Ended.
	pinchBegan  bool
	rotateBegan bool
}

type trackedTouch struct {
	id   uint64
	x, y float32
}

func (gs *gestureState) now() int64 {
	if gs.nowFn != nil {
		return gs.nowFn()
	}
	return time.Now().UnixNano()
}

func (gs *gestureState) reset() {
	nowFn := gs.nowFn
	lastTapTime := gs.lastTapTime
	lastTapX := gs.lastTapX
	lastTapY := gs.lastTapY
	*gs = gestureState{
		nowFn:       nowFn,
		lastTapTime: lastTapTime,
		lastTapX:    lastTapX,
		lastTapY:    lastTapY,
	}
}

// handleTouch processes raw touch events, runs the gesture state
// machine, synthesizes mouse events for backward compatibility,
// and dispatches recognized gestures.
func (w *Window) handleTouch(layout *Layout, e *Event) {
	if e == nil || layout == nil {
		return
	}
	if e.NumTouches < 0 {
		e.NumTouches = 0
	} else if e.NumTouches > len(e.Touches) {
		e.NumTouches = len(e.Touches)
	}
	gs := &w.viewState.gesture

	switch e.Type {
	case EventTouchesBegan:
		handleTouchBegan(gs, layout, e, w)
	case EventTouchesMoved:
		handleTouchMoved(gs, layout, e, w)
	case EventTouchesEnded:
		handleTouchEnded(gs, layout, e, w)
	case EventTouchesCancelled:
		handleTouchCancelled(gs, layout, e, w)
	}
	// The last finger lifted: there is no pointer any more, so nothing
	// may stay hovered (sticky hover on touch screens).
	if (e.Type == EventTouchesEnded || e.Type == EventTouchesCancelled) &&
		gs.numTouches == 0 && gs.overflowTouches == 0 {
		w.pointerLifted()
	}
}

func handleTouchBegan(
	gs *gestureState, layout *Layout, e *Event, w *Window,
) {
	// Add new touches to tracked set.
	for i := range e.NumTouches {
		if !e.Touches[i].Changed {
			continue
		}
		addTrackedTouch(gs, e.Touches[i])
	}

	if gs.numTouches == 1 {
		// First finger down.
		t := gs.touches[0]
		gs.beganTime = gs.now()
		gs.prevTime = gs.beganTime
		gs.startX = t.x
		gs.startY = t.y
		gs.prevX = t.x
		gs.prevY = t.y
		gs.singleTouchID = t.id
		gs.velocityX = 0
		gs.velocityY = 0
		gs.recognized = false
		gs.gestureType = gestureNone

		// Arm long-press timer.
		armLongPress(gs, layout, w)

		// Synthesize mouse down.
		synthMouse(EventMouseDown, t.x, t.y, MouseLeft, layout, w)
		gs.mouseEmitted = true
	} else {
		// Second+ finger: cancel single-touch gesture.
		cancelLongPress(w)
		if gs.mouseEmitted {
			t := gs.touches[0]
			synthMouse(EventMouseUp, t.x, t.y, MouseLeft, layout, w)
			gs.mouseEmitted = false
		}
		gs.recognized = true

		if gs.numTouches >= 2 {
			// Initialize pinch/rotate from first two touches.
			gs.initialSpan = touchSpan(gs)
			gs.prevSpan = gs.initialSpan
			// Fuzzed coords near max float32 overflow the
			// span to Inf; reseed to 0 so the move path
			// re-baselines instead of emitting from Inf.
			if !f32IsFinite(gs.initialSpan) {
				gs.initialSpan = 0
				gs.prevSpan = 0
			}
			gs.scale = 1
			gs.initialAngle = touchAngle(gs)
			gs.prevAngle = gs.initialAngle
			gs.rotation = 0
		}
	}
}

func handleTouchMoved(
	gs *gestureState, layout *Layout, e *Event, w *Window,
) {
	// Update tracked positions.
	for i := range e.NumTouches {
		if !e.Touches[i].Changed {
			continue
		}
		updateTrackedTouch(gs, e.Touches[i])
	}

	if gs.numTouches == 1 {
		t := gs.touches[0]
		if !gs.recognized {
			dx := t.x - gs.startX
			dy := t.y - gs.startY
			dist := dx*dx + dy*dy
			if dist > gesturePanDist*gesturePanDist {
				// Pan recognized.
				cancelLongPress(w)
				gs.recognized = true
				gs.gestureType = GesturePan
				gs.prevTime = gs.now()
				gs.prevX = gs.startX
				gs.prevY = gs.startY
				emitGesture(gs, GesturePan, gesturePhaseBegan,
					t.x, t.y, layout, w)
				return
			} else {
				// Below threshold: synthesize mouse move.
				synthMouse(EventMouseMove, t.x, t.y, MouseLeft,
					layout, w)
			}
		}
		if gs.recognized && gs.gestureType == GesturePan {
			gdx := t.x - gs.prevX
			gdy := t.y - gs.prevY
			now := gs.now()
			dt := float32(now-gs.prevTime) / 1e9
			if dt < 0.001 {
				dt = 0.001
			} else if dt > 0.1 {
				dt = 0.1
			}
			gs.velocityX = gestureVelocitySmooth*gdx/dt +
				(1-gestureVelocitySmooth)*gs.velocityX
			gs.velocityY = gestureVelocitySmooth*gdy/dt +
				(1-gestureVelocitySmooth)*gs.velocityY
			// A huge-but-finite jump (fuzzed coords) overflows
			// the EMA to Inf; reset instead of emitting NaN/Inf
			// velocity on the pan event.
			if !f32IsFinite(gs.velocityX) || !f32IsFinite(gs.velocityY) {
				gs.velocityX = 0
				gs.velocityY = 0
			}
			gs.prevTime = now
			gs.prevX = t.x
			gs.prevY = t.y
			emitGestureWithDelta(gs, GesturePan, GesturePhaseChanged,
				t.x, t.y, gdx, gdy, layout, w)
			synthMouse(EventMouseMove, t.x, t.y, MouseLeft,
				layout, w)
		}
		return
	}

	// Multi-touch: pinch and rotate.
	if gs.numTouches >= 2 {
		cx, cy := touchCentroid(gs)
		span := touchSpan(gs)
		angle := touchAngle(gs)

		// Fingers that land co-located leave no span to
		// measure against; reseed the baseline instead of
		// dividing by zero below. The span must be finite:
		// reseeding from an overflowed span would poison the
		// baseline to Inf and collapse the next scale to 0.
		if gs.initialSpan <= 0 && span > 0 && f32IsFinite(span) {
			gs.initialSpan = span
			gs.prevSpan = span
		}
		// A span overflowed to Inf (fuzzed coords near max
		// float32) cannot measure a pinch; skip pinch but
		// still try rotate below, so scale never emits Inf.
		if f32IsFinite(span) {
			spanDelta := span - gs.initialSpan
			if spanDelta > gesturePinchDist || spanDelta < -gesturePinchDist {
				if gs.prevSpan > 0 {
					gs.scale *= span / gs.prevSpan
				}
				gs.prevSpan = span
				phase := GesturePhaseChanged
				if !gs.pinchBegan {
					gs.pinchBegan = true
					gs.gestureType = GesturePinch
					phase = gesturePhaseBegan
					if gs.initialSpan > 0 {
						gs.scale = span / gs.initialSpan
					} else {
						gs.scale = 1
					}
				}
				// Either update above can overflow to Inf on
				// fuzzed coords; emit 1 instead of non-finite.
				if !f32IsFinite(gs.scale) {
					gs.scale = 1
				}
				emitPinch(gs, phase, cx, cy, layout, w)
			}
		}

		angleDelta := normalizeAngle(angle - gs.initialAngle)
		if angleDelta > gestureRotateAngle ||
			angleDelta < -gestureRotateAngle {
			delta := normalizeAngle(angle - gs.prevAngle)
			gs.rotation += delta
			gs.prevAngle = angle
			if !gs.rotateBegan {
				gs.rotateBegan = true
				if gs.gestureType != GesturePinch {
					gs.gestureType = GestureRotate
				}
				emitRotate(gs, gesturePhaseBegan, cx, cy,
					layout, w)
			} else {
				emitRotate(gs, GesturePhaseChanged, cx, cy,
					layout, w)
			}
		}
	}
}

func handleTouchEnded(
	gs *gestureState, layout *Layout, e *Event, w *Window,
) {
	// Fold the event's own positions in first, so the release
	// point below is where the fingers lifted, not where the
	// last move left them.
	for i := range e.NumTouches {
		if !e.Touches[i].Changed {
			continue
		}
		updateTrackedTouch(gs, e.Touches[i])
	}
	// Compute centroid before removing touches so end events
	// have accurate coordinates.
	cx, cy := gs.startX, gs.startY
	if gs.numTouches > 0 {
		cx, cy = touchCentroid(gs)
	}

	// Remove ended touches.
	for i := range e.NumTouches {
		if !e.Touches[i].Changed {
			continue
		}
		removeTrackedTouch(gs, e.Touches[i].Identifier)
	}

	if gs.numTouches == 0 && gs.overflowTouches == 0 {
		endAllTouches(gs, layout, w, cx, cy)
		return
	}

	transitionAfterLift(gs, layout, w)
}

// endAllTouches closes every in-flight gesture once the last
// finger lifts: pan (or a swipe when the velocity is fresh),
// each multi-touch sub-state, a long press, or a tap.
func endAllTouches(
	gs *gestureState, layout *Layout, w *Window,
	cx, cy float32,
) {
	cancelLongPress(w)

	if gs.recognized && gs.gestureType == GesturePan {
		now := gs.now()
		vel := float32(math.Sqrt(float64(
			gs.velocityX*gs.velocityX +
				gs.velocityY*gs.velocityY)))
		if vel > gestureSwipeVelocity &&
			now-gs.prevTime < int64(gestureSwipeStale) {
			emitGestureSwipe(gs, layout, w)
		} else {
			emitGesture(gs, GesturePan, gesturePhaseEnded,
				gs.prevX, gs.prevY, layout, w)
		}
	}
	if gs.pinchBegan || gs.gestureType == GesturePinch {
		emitPinch(gs, gesturePhaseEnded, cx, cy,
			layout, w)
	}
	if gs.rotateBegan || gs.gestureType == GestureRotate {
		emitRotate(gs, gesturePhaseEnded, cx, cy,
			layout, w)
	}
	if gs.recognized &&
		gs.gestureType == GestureLongPress {
		emitGesture(gs, GestureLongPress, gesturePhaseEnded,
			gs.startX, gs.startY, layout, w)
	} else if !gs.recognized {
		endMaybeTap(gs, layout, w)
	}

	// Synthesize mouse up for compat, at the release
	// point — after a drag the press point is stale.
	if gs.mouseEmitted {
		synthMouse(EventMouseUp, cx, cy,
			MouseLeft, layout, w)
	}
	gs.reset()
}

// endMaybeTap emits a tap (or the second half of a double tap)
// for a press that never became another gesture.
func endMaybeTap(gs *gestureState, layout *Layout, w *Window) {
	now := gs.now()
	dur := now - gs.beganTime
	if dur >= int64(gestureTapTimeout) {
		return
	}
	dx := gs.startX - gs.lastTapX
	dy := gs.startY - gs.lastTapY
	gap := now - gs.lastTapTime
	if gs.lastTapTime > 0 &&
		gap < int64(gestureDoubleTapGap) &&
		dx*dx+dy*dy <
			gestureDoubleTapRadius*gestureDoubleTapRadius {
		emitGesture(gs, GestureDoubleTap,
			gesturePhaseEnded,
			gs.startX, gs.startY, layout, w)
		gs.lastTapTime = 0
		return
	}
	emitGesture(gs, GestureTap,
		gesturePhaseEnded,
		gs.startX, gs.startY, layout, w)
	gs.lastTapTime = now
	gs.lastTapX = gs.startX
	gs.lastTapY = gs.startY
}

// transitionAfterLift runs when fingers remain down after a
// release: a pinch or rotate hands over to pan, and a leftover
// finger that never was the single touch reseeds as a fresh
// press at its own position.
func transitionAfterLift(
	gs *gestureState, layout *Layout, w *Window,
) {
	// 2→1 finger transition: end pinch/rotate, start pan.
	if gs.pinchBegan || gs.rotateBegan ||
		gs.gestureType == GesturePinch ||
		gs.gestureType == GestureRotate {
		tcx, tcy := touchCentroid(gs)
		if gs.pinchBegan || gs.gestureType == GesturePinch {
			emitPinch(gs, gesturePhaseEnded, tcx, tcy,
				layout, w)
		}
		if gs.rotateBegan || gs.gestureType == GestureRotate {
			emitRotate(gs, gesturePhaseEnded, tcx, tcy,
				layout, w)
		}
		// Transition to single-touch pan.
		t := gs.touches[0]
		gs.gestureType = GesturePan
		gs.pinchBegan = false
		gs.rotateBegan = false
		gs.prevX = t.x
		gs.prevY = t.y
		gs.startX = t.x
		gs.startY = t.y
		gs.velocityX = 0
		gs.velocityY = 0
		gs.singleTouchID = t.id
		emitGesture(gs, GesturePan, gesturePhaseBegan,
			t.x, t.y, layout, w)
		return
	}

	reseedRemainingFinger(gs, layout, w)
}

// reseedRemainingFinger treats the leftover finger as a fresh
// press when the original single touch lifted out from under
// it, ending any in-flight single-touch gesture first.
func reseedRemainingFinger(
	gs *gestureState, layout *Layout, w *Window,
) {
	if gs.numTouches != 1 ||
		gs.touches[0].id == gs.singleTouchID {
		return
	}
	t := gs.touches[0]
	if gs.recognized && gs.gestureType == GesturePan {
		emitGesture(gs, GesturePan, gesturePhaseEnded,
			gs.prevX, gs.prevY, layout, w)
	} else if gs.recognized &&
		gs.gestureType == GestureLongPress {
		emitGesture(gs, GestureLongPress,
			gesturePhaseEnded,
			gs.startX, gs.startY, layout, w)
	}
	gs.beganTime = gs.now()
	gs.prevTime = gs.beganTime
	gs.startX = t.x
	gs.startY = t.y
	gs.prevX = t.x
	gs.prevY = t.y
	gs.singleTouchID = t.id
	gs.velocityX = 0
	gs.velocityY = 0
	gs.recognized = false
	gs.gestureType = gestureNone
	// The earlier press already released its mouse half;
	// this finger is physically down, so press anew for
	// click compat and re-arm the long-press timer.
	armLongPress(gs, layout, w)
	synthMouse(EventMouseDown, t.x, t.y, MouseLeft,
		layout, w)
	gs.mouseEmitted = true
}

func handleTouchCancelled(
	gs *gestureState, layout *Layout, e *Event, w *Window,
) {
	cancelLongPress(w)
	cx, cy := gs.startX, gs.startY
	if gs.numTouches > 0 {
		cx, cy = touchCentroid(gs)
	}
	if gs.recognized {
		cancelled := false
		if gs.pinchBegan || gs.gestureType == GesturePinch {
			emitPinch(gs, gesturePhaseCancelled, cx, cy,
				layout, w)
			cancelled = true
		}
		if gs.rotateBegan || gs.gestureType == GestureRotate {
			emitRotate(gs, gesturePhaseCancelled, cx, cy,
				layout, w)
			cancelled = true
		}
		if !cancelled && gs.gestureType != gestureNone {
			emitGesture(gs, gs.gestureType,
				gesturePhaseCancelled,
				cx, cy, layout, w)
		}
	}
	if gs.mouseEmitted {
		synthMouse(EventMouseUp, cx, cy,
			MouseLeft, layout, w)
	}
	gs.reset()
}

// --- Long press ---

func armLongPress(gs *gestureState, _ *Layout, w *Window) {
	gs.longPressSX = gs.startX
	gs.longPressSY = gs.startY
	w.AnimationAdd(&Animate{
		AnimID:   gestureLongPressAnimID,
		Delay:    gestureLongPressDur,
		Repeat:   false,
		Callback: gestureLongPressFired,
	})
}

// gestureLongPressFired is a package-level callback that reads
// snapshotted longPressSX/longPressSY instead of capturing via
// a per-invocation closure (avoids heap allocation on every touch).
func gestureLongPressFired(_ *Animate, w *Window) {
	gst := &w.viewState.gesture
	if gst.numTouches != 1 || gst.recognized {
		return
	}
	t := gst.touches[0]
	dx := t.x - gst.longPressSX
	dy := t.y - gst.longPressSY
	if dx*dx+dy*dy > gesturePanDist*gesturePanDist {
		return
	}
	gst.recognized = true
	gst.gestureType = GestureLongPress
	emitGesture(gst, GestureLongPress, gesturePhaseBegan,
		gst.longPressSX, gst.longPressSY, dialogRoute(w), w)
}

func cancelLongPress(w *Window) {
	w.AnimationRemove(gestureLongPressAnimID)
}

// --- Gesture event construction and dispatch ---
//
// Construction, dispatch, dialog routing and mouse synthesis live
// in gesture_dispatch.go, split out to hold the large-files gate
// (see scripts/large-files.sh).

// Touch tracking (the tracked-touch set, span, angle) lives in
// gesture_touch.go, split out to hold the large-files gate
// (see scripts/large-files.sh).
