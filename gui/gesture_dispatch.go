package gui

import "slices"

// gesture_dispatch.go — gesture event construction and dispatch,
// split out of gesture.go to hold the large-files gate
// (see scripts/large-files.sh).

func gestureEvent(
	gs *gestureState, gt GestureType, phase gesturePhase,
	cx, cy float32,
) Event {
	return Event{
		Type:           eventGesture,
		GestureType:    gt,
		GesturePhase:   phase,
		CentroidX:      cx,
		CentroidY:      cy,
		gestureTouches: gs.numTouches,
		VelocityX:      gs.velocityX,
		VelocityY:      gs.velocityY,
	}
}

func emitGesture(
	gs *gestureState, gt GestureType, phase gesturePhase,
	cx, cy float32, layout *Layout, w *Window,
) {
	w.scratch.gestureEvent = gestureEvent(gs, gt, phase, cx, cy)
	gestureHandler(layout, &w.scratch.gestureEvent, w)
}

func emitGestureWithDelta(
	gs *gestureState, gt GestureType, phase gesturePhase,
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
		gesturePhaseEnded, gs.prevX, gs.prevY)
	w.scratch.gestureEvent.VelocityX = gs.velocityX
	w.scratch.gestureEvent.VelocityY = gs.velocityY
	gestureHandler(layout, &w.scratch.gestureEvent, w)
}

func emitPinch(
	gs *gestureState, phase gesturePhase,
	cx, cy float32, layout *Layout, w *Window,
) {
	w.scratch.gestureEvent = gestureEvent(gs, GesturePinch,
		phase, cx, cy)
	w.scratch.gestureEvent.PinchScale = gs.scale
	gestureHandler(layout, &w.scratch.gestureEvent, w)
}

func emitRotate(
	gs *gestureState, phase gesturePhase,
	cx, cy float32, layout *Layout, w *Window,
) {
	w.scratch.gestureEvent = gestureEvent(gs, GestureRotate,
		phase, cx, cy)
	w.scratch.gestureEvent.GestureRotation = gs.rotation
	gestureHandler(layout, &w.scratch.gestureEvent, w)
}

// dialogRoute returns the dialog layer while a modal dialog is
// visible, the same routing EventFn applies before dispatch (see
// window_event.go). The long-press timer fires outside the event
// path, so it re-derives the route at fire time instead of
// capturing a layout that a later frame may have rebuilt.
func dialogRoute(w *Window) *Layout {
	if w == nil {
		return nil
	}
	ly := &w.layout
	if w.dialogCfg.visible && len(w.layout.Children) > 0 {
		ly = &w.layout.Children[len(w.layout.Children)-1]
	}
	return ly
}

// gestureHandler dispatches a gesture event to the layout tree.
// Reverse traversal (topmost first), same pattern as mouse
// handlers. Falls back to scroll for unhandled pan gestures.
//
// Centroid coordinates are carried in CentroidX/CentroidY.
// For rotated containers, they are temporarily mapped through
// the inverse rotation and restored on the way out.
func gestureHandler(layout *Layout, e *Event, w *Window) {
	if layout == nil {
		return
	}
	gestureHandlerDepth(layout, e, w, 0)
}

func gestureHandlerDepth(
	layout *Layout, e *Event, w *Window, depth int,
) {
	if overMaxDepth(depth) {
		return
	}
	ox, oy := rotateCentroidInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		gestureHandlerDepth(&layout.Children[i], e, w, depth+1)
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
		layout.Shape.events.OnGesture(EventCtx{layout, e, w})
		debugUnconsumed(evGesture, layout, e, w)
		if e.IsHandled {
			return
		}
	}
	// Pan fallback: auto-scroll containers. Handled only when
	// an offset actually moved, so a pan over a container
	// already at its limit still reaches ancestors.
	if e.GestureType == GesturePan &&
		e.GesturePhase == GesturePhaseChanged &&
		layout.Shape.Scrollable {
		movedV := scrollVertical(layout, e.GestureDY, w)
		movedH := scrollHorizontal(layout, e.GestureDX, w)
		if movedV || movedH {
			e.IsHandled = true
		}
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
//
// This is the second entry into mouseDownHandler/mouseUpHandler
// besides EventFn's handleMouseDown/UpEvent (which own the held-button
// state machine for backend events). Touch-synthesized presses and
// releases update that state here, so mixed mouse+touch input cannot
// leave hover synthesis reporting a button nobody is holding — e.g. a
// touch release after a backend mouse press must clear the hold, and a
// touch press must record one. The dev-mode unconsumed sweep
// (debug_event.go) also reaches the traversal handlers directly, but
// only for inspection, so it must not run through this path.
func synthMouse(
	typ EventType, x, y float32, btn MouseButton,
	layout *Layout, w *Window,
) {
	w.scratch.gestureEvent = Event{
		Type:        typ,
		MouseX:      x,
		MouseY:      y,
		MouseButton: btn,
		// Stamped the way EventFn stamps a backend event. Consumers
		// time multi-click gestures by differencing this — datagrid's
		// double-click-to-edit and double-click-to-autofit both do —
		// and a synthetic event left at frame 0 stored 0 as the last
		// click frame, which their "> 0" sentinel then read as "no
		// prior click". Double-tap was inert on touch.
		FrameCount: w.frameCount,
	}
	switch typ {
	case EventMouseDown:
		w.viewState.mouseButtonHeld = btn
		w.pointerAt(x, y)
		w.recordPressTarget(&w.scratch.gestureEvent)
		mouseDownHandler(layout, false, &w.scratch.gestureEvent, w)
	case EventMouseMove:
		w.pointerAt(x, y)
		mouseMoveHandler(layout, &w.scratch.gestureEvent, w)
	case EventMouseUp:
		w.viewState.mouseButtonHeld = MouseInvalid
		w.viewState.pressTargetID = ""
		mouseUpHandler(layout, &w.scratch.gestureEvent, w)
	}
}
