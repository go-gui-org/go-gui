package gui

import (
	"math"
	"slices"
	"time"
)

// Gesture recognition thresholds.
const (
	gestureTapTimeout      int64   = 300_000_000 // 300ms nanos
	gestureDoubleTapGap    int64   = 300_000_000
	gestureLongPressDur            = 500 * time.Millisecond
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
	rotateBegan  bool
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
	if e.NumTouches > len(e.Touches) {
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
		gs.gestureType = GestureNone

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
				emitGesture(gs, GesturePan, GesturePhaseBegan,
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

		spanDelta := span - gs.initialSpan
		if spanDelta > gesturePinchDist || spanDelta < -gesturePinchDist {
			if gs.prevSpan > 0 {
				gs.scale *= span / gs.prevSpan
			}
			gs.prevSpan = span
			phase := GesturePhaseChanged
			if gs.gestureType != GesturePinch {
				gs.gestureType = GesturePinch
				phase = GesturePhaseBegan
				gs.scale = span / gs.initialSpan
			}
			w.scratch.gestureEvent = gestureEvent(gs, GesturePinch, phase, cx, cy)
			w.scratch.gestureEvent.PinchScale = gs.scale
			gestureHandler(layout, &w.scratch.gestureEvent, w)
		}

		angleDelta := normalizeAngle(angle - gs.initialAngle)
		if angleDelta > gestureRotateAngle ||
			angleDelta < -gestureRotateAngle {
			delta := normalizeAngle(angle - gs.prevAngle)
			gs.rotation += delta
			gs.prevAngle = angle
			phase := GesturePhaseChanged
			if gs.gestureType != GestureRotate &&
				gs.gestureType != GesturePinch {
				gs.gestureType = GestureRotate
				gs.rotateBegan = true
				phase = GesturePhaseBegan
			} else if !gs.rotateBegan {
				gs.rotateBegan = true
				w.scratch.gestureEvent = gestureEvent(
					gs, GestureRotate, GesturePhaseBegan, cx, cy)
				w.scratch.gestureEvent.GestureRotation = gs.rotation
				gestureHandler(layout, &w.scratch.gestureEvent, w)
			}
			w.scratch.gestureEvent = gestureEvent(gs, GestureRotate, phase, cx, cy)
			w.scratch.gestureEvent.GestureRotation = gs.rotation
			gestureHandler(layout, &w.scratch.gestureEvent, w)
		}
	}
}

func handleTouchEnded(
	gs *gestureState, layout *Layout, e *Event, w *Window,
) {
	// Compute centroid before removing touches so end events
	// have accurate coordinates.
	cx, cy := touchCentroid(gs)

	// Remove ended touches.
	for i := range e.NumTouches {
		if !e.Touches[i].Changed {
			continue
		}
		removeTrackedTouch(gs, e.Touches[i].Identifier)
	}

	if gs.numTouches == 0 {
		// All fingers up.
		cancelLongPress(w)

		if gs.recognized && gs.gestureType == GesturePan {
			vel := float32(math.Sqrt(float64(
				gs.velocityX*gs.velocityX +
					gs.velocityY*gs.velocityY)))
			if vel > gestureSwipeVelocity {
				emitGestureSwipe(gs, layout, w)
			} else {
				emitGesture(gs, GesturePan, GesturePhaseEnded,
					gs.prevX, gs.prevY, layout, w)
			}
		} else if gs.recognized &&
			gs.gestureType == GesturePinch {
			w.scratch.gestureEvent = gestureEvent(gs, GesturePinch,
				GesturePhaseEnded, cx, cy)
			w.scratch.gestureEvent.PinchScale = gs.scale
			gestureHandler(layout, &w.scratch.gestureEvent, w)
		} else if gs.recognized &&
			gs.gestureType == GestureRotate {
			w.scratch.gestureEvent = gestureEvent(gs, GestureRotate,
				GesturePhaseEnded, cx, cy)
			w.scratch.gestureEvent.GestureRotation = gs.rotation
			gestureHandler(layout, &w.scratch.gestureEvent, w)
		} else if gs.recognized &&
			gs.gestureType == GestureLongPress {
			emitGesture(gs, GestureLongPress, GesturePhaseEnded,
				gs.startX, gs.startY, layout, w)
		} else if !gs.recognized {
			// Possible tap.
			now := gs.now()
			dur := now - gs.beganTime
			if dur < gestureTapTimeout {
				dx := gs.startX - gs.lastTapX
				dy := gs.startY - gs.lastTapY
				gap := now - gs.lastTapTime
				if gs.lastTapTime > 0 &&
					gap < gestureDoubleTapGap &&
					dx*dx+dy*dy <
						gestureDoubleTapRadius*gestureDoubleTapRadius {
					emitGesture(gs, GestureDoubleTap,
						GesturePhaseEnded,
						gs.startX, gs.startY, layout, w)
					gs.lastTapTime = 0
				} else {
					emitGesture(gs, GestureTap,
						GesturePhaseEnded,
						gs.startX, gs.startY, layout, w)
					gs.lastTapTime = now
					gs.lastTapX = gs.startX
					gs.lastTapY = gs.startY
				}
			}
		}

		// Synthesize mouse up for compat.
		if gs.mouseEmitted {
			synthMouse(EventMouseUp, gs.startX, gs.startY,
				MouseLeft, layout, w)
		}
		gs.reset()
		return
	}

	// 2→1 finger transition: end pinch/rotate, start pan.
	if gs.gestureType == GesturePinch ||
		gs.gestureType == GestureRotate {
		tcx, tcy := touchCentroid(gs)
		if gs.gestureType == GesturePinch {
			w.scratch.gestureEvent = gestureEvent(gs, GesturePinch,
				GesturePhaseEnded, tcx, tcy)
			w.scratch.gestureEvent.PinchScale = gs.scale
			gestureHandler(layout, &w.scratch.gestureEvent, w)
		} else {
			w.scratch.gestureEvent = gestureEvent(gs, GestureRotate,
				GesturePhaseEnded, tcx, tcy)
			w.scratch.gestureEvent.GestureRotation = gs.rotation
			gestureHandler(layout, &w.scratch.gestureEvent, w)
		}
		// Transition to single-touch pan.
		t := gs.touches[0]
		gs.gestureType = GesturePan
		gs.prevX = t.x
		gs.prevY = t.y
		gs.startX = t.x
		gs.startY = t.y
		gs.velocityX = 0
		gs.velocityY = 0
		gs.singleTouchID = t.id
		emitGesture(gs, GesturePan, GesturePhaseBegan,
			t.x, t.y, layout, w)
	}
}

func handleTouchCancelled(
	gs *gestureState, layout *Layout, e *Event, w *Window,
) {
	cancelLongPress(w)
	if gs.recognized && gs.gestureType != GestureNone {
		cx, cy := touchCentroid(gs)
		emitGesture(gs, gs.gestureType, GesturePhaseCancelled,
			cx, cy, layout, w)
	}
	if gs.mouseEmitted {
		synthMouse(EventMouseUp, gs.startX, gs.startY,
			MouseLeft, layout, w)
	}
	gs.reset()
}

// --- Long press ---

func armLongPress(gs *gestureState, layout *Layout, w *Window) {
	startX := gs.startX
	startY := gs.startY
	w.AnimationAdd(&Animate{
		AnimID: gestureLongPressAnimID,
		Delay:  gestureLongPressDur,
		Repeat: false,
		Callback: func(_ *Animate, w *Window) {
			gst := &w.viewState.gesture
			if gst.numTouches != 1 || gst.recognized {
				return
			}
			t := gst.touches[0]
			dx := t.x - startX
			dy := t.y - startY
			if dx*dx+dy*dy > gesturePanDist*gesturePanDist {
				return
			}
			gst.recognized = true
			gst.gestureType = GestureLongPress
			ly := &w.layout
			if w.dialogCfg.visible && len(w.layout.Children) > 0 {
				ly = &w.layout.Children[len(w.layout.Children)-1]
			}
			emitGesture(gst, GestureLongPress, GesturePhaseBegan,
				startX, startY, ly, w)
		},
	})
}

func cancelLongPress(w *Window) {
	w.AnimationRemove(gestureLongPressAnimID)
}

// --- Gesture event construction and dispatch ---

func gestureEvent(
	gs *gestureState, gt GestureType, phase GesturePhase,
	cx, cy float32,
) Event {
	return Event{
		Type:           EventGesture,
		GestureType:    gt,
		GesturePhase:   phase,
		CentroidX:      cx,
		CentroidY:      cy,
		GestureTouches: gs.numTouches,
		VelocityX:      gs.velocityX,
		VelocityY:      gs.velocityY,
	}
}

func emitGesture(
	gs *gestureState, gt GestureType, phase GesturePhase,
	cx, cy float32, layout *Layout, w *Window,
) {
	w.scratch.gestureEvent = gestureEvent(gs, gt, phase, cx, cy)
	gestureHandler(layout, &w.scratch.gestureEvent, w)
}

func emitGestureWithDelta(
	gs *gestureState, gt GestureType, phase GesturePhase,
	cx, cy, dx, dy float32, layout *Layout, w *Window,
) {
	w.scratch.gestureEvent = gestureEvent(gs, gt, phase, cx, cy)
	w.scratch.gestureEvent.GestureDX = dx
	w.scratch.gestureEvent.GestureDY = dy
	gestureHandler(layout, &w.scratch.gestureEvent, w)
}

func emitGestureSwipe(
	gs *gestureState, layout *Layout, w *Window,
) {
	w.scratch.gestureEvent = gestureEvent(gs, GestureSwipe,
		GesturePhaseEnded, gs.prevX, gs.prevY)
	w.scratch.gestureEvent.VelocityX = gs.velocityX
	w.scratch.gestureEvent.VelocityY = gs.velocityY
	gestureHandler(layout, &w.scratch.gestureEvent, w)
}

// gestureHandler dispatches a gesture event to the layout tree.
// Reverse traversal (topmost first), same pattern as mouse
// handlers. Falls back to scroll for unhandled pan gestures.
//
// Centroid coordinates are carried in CentroidX/CentroidY.
// For rotated containers, they are temporarily mapped through
// the inverse rotation via MouseX/MouseY fields.
func gestureHandler(layout *Layout, e *Event, w *Window) {
	ox, oy := rotateCentroidInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		gestureHandler(&layout.Children[i], e, w)
		if e.IsHandled {
			e.CentroidX, e.CentroidY = ox, oy
			return
		}
	}
	e.CentroidX, e.CentroidY = ox, oy
	if layout.Shape == nil {
		return
	}
	if !layout.Shape.PointInShape(e.CentroidX, e.CentroidY) {
		return
	}
	if layout.Shape.hasEvents() &&
		layout.Shape.events.OnGesture != nil {
		layout.Shape.events.OnGesture(layout, e, w)
		if e.IsHandled {
			return
		}
	}
	// Pan fallback: auto-scroll containers.
	if e.GestureType == GesturePan &&
		e.GesturePhase == GesturePhaseChanged &&
		layout.Shape.IDScroll > 0 {
		scrollVertical(layout, e.GestureDY, w)
		scrollHorizontal(layout, e.GestureDX, w)
		e.IsHandled = true
	}
}

// rotateCentroidInverse applies the inverse rotation for
// containers that use QuarterTurns, operating on centroid
// coordinates.
func rotateCentroidInverse(
	s *Shape, e *Event,
) (origX, origY float32) {
	origX, origY = e.CentroidX, e.CentroidY
	if s == nil || s.QuarterTurns == 0 {
		return
	}
	cx := s.X + s.Width/2
	cy := s.Y + s.Height/2
	dx, dy := e.CentroidX-cx, e.CentroidY-cy
	switch s.QuarterTurns {
	case 1:
		e.CentroidX = cx + dy
		e.CentroidY = cy - dx
	case 2:
		e.CentroidX = cx - dx
		e.CentroidY = cy - dy
	case 3:
		e.CentroidX = cx - dy
		e.CentroidY = cy + dx
	}
	return
}

// synthMouse creates a synthetic mouse event and dispatches it
// through the normal mouse handler pipeline.
func synthMouse(
	typ EventType, x, y float32, btn MouseButton,
	layout *Layout, w *Window,
) {
	w.scratch.gestureEvent = Event{
		Type:        typ,
		MouseX:      x,
		MouseY:      y,
		MouseButton: btn,
	}
	switch typ {
	case EventMouseDown:
		mouseDownHandler(layout, false, &w.scratch.gestureEvent, w)
	case EventMouseMove:
		mouseMoveHandler(layout, &w.scratch.gestureEvent, w)
	case EventMouseUp:
		mouseUpHandler(layout, &w.scratch.gestureEvent, w)
	}
}

// --- Touch tracking helpers ---

func addTrackedTouch(gs *gestureState, tp TouchPoint) {
	// Update existing touch if already tracked.
	for i := range gs.numTouches {
		if gs.touches[i].id == tp.Identifier {
			gs.touches[i].x = tp.PosX
			gs.touches[i].y = tp.PosY
			return
		}
	}
	if gs.numTouches >= len(gs.touches) {
		return
	}
	gs.touches[gs.numTouches] = trackedTouch{
		id: tp.Identifier, x: tp.PosX, y: tp.PosY,
	}
	gs.numTouches++
}

func updateTrackedTouch(gs *gestureState, tp TouchPoint) {
	for i := range gs.numTouches {
		if gs.touches[i].id == tp.Identifier {
			gs.touches[i].x = tp.PosX
			gs.touches[i].y = tp.PosY
			return
		}
	}
}

func removeTrackedTouch(gs *gestureState, id uint64) {
	for i := range gs.numTouches {
		if gs.touches[i].id == id {
			gs.numTouches--
			gs.touches[i] = gs.touches[gs.numTouches]
			gs.touches[gs.numTouches] = trackedTouch{}
			return
		}
	}
}

func touchCentroid(gs *gestureState) (float32, float32) {
	if gs.numTouches == 0 {
		return 0, 0
	}
	var sx, sy float32
	for i := range gs.numTouches {
		sx += gs.touches[i].x
		sy += gs.touches[i].y
	}
	n := float32(gs.numTouches)
	return sx / n, sy / n
}

func touchSpan(gs *gestureState) float32 {
	if gs.numTouches < 2 {
		return 0
	}
	dx := gs.touches[1].x - gs.touches[0].x
	dy := gs.touches[1].y - gs.touches[0].y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func touchAngle(gs *gestureState) float32 {
	if gs.numTouches < 2 {
		return 0
	}
	dx := gs.touches[1].x - gs.touches[0].x
	dy := gs.touches[1].y - gs.touches[0].y
	return float32(math.Atan2(float64(dy), float64(dx)))
}

// normalizeAngle wraps an angle to [-pi, pi].
func normalizeAngle(a float32) float32 {
	return float32(math.Remainder(float64(a), 2*math.Pi))
}
