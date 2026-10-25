package gui

// touch_pan_axis.go — which scrollables a touch pan moves, on which axes
// (#1118).
//
// A finger never moves straight. A vertical pan over a table that
// scrolls sideways drifts a pixel or two across per move, and the pan
// fallback used to stop at the first scrollable that moved on either
// axis: the table took the drift, and the page around it barely moved.
// Two rules settle it, as on iOS and Android:
//
//  1. The axis lock decides which axes the pan scrolls. A pan that
//     starts clearly along one axis, touchPressCrossRatio times farther
//     along it than across, scrolls only on that axis until the lift. A
//     diagonal start scrolls both.
//  2. Per-axis routing decides which scrollable takes each axis: the
//     innermost one under the finger that moves on it. A diagonal pan
//     over the table scrolls the table sideways and the page vertically.
//
// The lock applies to scrolling only. An OnGesture handler still reads
// the finger's raw deltas, so a canvas that pans in two dimensions keeps
// both.

// panAxis is the axes a touch pan scrolls.
type panAxis uint8

const (
	// panAxisFree scrolls both axes: a diagonal start, or a pan with no
	// recognition step (the finger left after a pinch).
	panAxisFree panAxis = iota
	panAxisVertical
	panAxisHorizontal
)

// panAxisLock is the axis lock for a pan recognized after the finger
// travelled dx, dy from the press. The ratio is the one that sends a
// held press across a scrollable (touchPressOnPan), so a pan that drags
// a slider across a list does not also scroll the list.
func panAxisLock(dx, dy float32) panAxis {
	ax, ay := f32Abs(dx), f32Abs(dy)
	switch {
	case ay > touchPressCrossRatio*ax:
		return panAxisVertical
	case ax > touchPressCrossRatio*ay:
		return panAxisHorizontal
	}
	return panAxisFree
}

// panScrollFallback scrolls a scrollable under a touch pan by the part
// of the move no scrollable inside it took. An axis it moves is zeroed
// on the event, so the scrollables around it, and their OnGesture
// handlers, see only what is left; the event is handled once nothing is
// left. An axis counts as taken only when an offset actually moved, so a
// pan over a container already at its limit still reaches ancestors.
func panScrollFallback(layout *Layout, e *Event, w *Window) {
	dx, dy := panScrollDeltas(e, w)
	movedV := dy != 0 && scrollVerticalPixels(layout, dy, w)
	movedH := dx != 0 && scrollHorizontalPixels(layout, dx, w)
	if !movedV && !movedH {
		return
	}
	e.panMoved = true
	if movedV {
		e.GestureDY = 0
	}
	if movedH {
		e.GestureDX = 0
	}
	// The lift flings the container this pan last moved on each axis.
	panScrolled(&w.viewState.gesture, layout, movedV, movedH)
	if rx, ry := panScrollDeltas(e, w); rx == 0 && ry == 0 {
		e.IsHandled = true
	}
}

// panScrollDeltas is the part of a pan move a scrollable may take: the
// event's deltas on the axes the pan locked, trimmed for a scrollable
// around an active DragScroll pan (dragPanOuterDeltas).
func panScrollDeltas(e *Event, w *Window) (float32, float32) {
	dx, dy := e.GestureDX, e.GestureDY
	switch w.viewState.gesture.panAxis {
	case panAxisVertical:
		dx = 0
	case panAxisHorizontal:
		dy = 0
	}
	if w.viewState.dragPan.active {
		dx, dy = dragPanOuterDeltas(w, dx, dy)
	}
	return dx, dy
}
