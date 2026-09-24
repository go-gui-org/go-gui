package gui

// drag_scroll.go — drag-to-scroll for Scrollable containers
// (issue #783). A container with ContainerCfg.DragScroll pans its
// content when the pointer drags it, the same gesture touch users
// already have through the gesture pan fallback.
//
// A press is claimed in the press walk before children dispatch, so
// a pan can suppress the press-point click: past the drag threshold
// the pointer owns a scroll, while a tap replays the press through
// normal dispatch for the child untouched. Touch takes the same
// path — its synthesized mouse events run the same walk — and the
// gesture pan fallback skips DragScroll containers, so one finger
// never scrolls twice.

// dragPanState tracks one in-progress drag-to-scroll press. Zero
// value is idle. One slot is enough: a window holds one pressed
// button at a time (mouseButtonHeld), and a second finger cancels
// the single-touch gesture before it could start another.
type dragPanState struct {
	scrollID string
	// start is where the press landed; last is the previous move.
	// Deltas come from last, not from the event's MouseDX/DY:
	// touch-synthesized moves carry no per-event delta. A tap
	// replays from start, so no second copy of the press point.
	startX, startY float32
	lastX, lastY   float32
	modifiers      Modifier
	active         bool
	panned         bool
}

// dragPanIntercept claims a left press on a DragScroll scrollable
// before children dispatch. Reports whether it claimed the press.
// A claim takes the mouse lock and marks the event handled, so the
// subtree walk and the ancestors both stand down; a tap replays the
// press on release, a pan suppresses it.
//
// The claim passes through when a nested DragScroll scrollable or a
// scrollbar part sits under the pointer: the inner container owns
// its presses, and a thumb or gutter owns its own.
func dragPanIntercept(layout *Layout, e *Event, w *Window) bool {
	s := layout.Shape
	if s == nil || e == nil || w == nil {
		return false
	}
	if e.dragPanReplay || e.MouseButton != MouseLeft {
		return false
	}
	if !s.DragScroll || !s.Scrollable || s.Disabled {
		return false
	}
	if w.mouseIsLocked() {
		return false
	}
	if !s.PointInShape(e.MouseX, e.MouseY) {
		return false
	}
	if dragPanOnScrollbar(layout, e.MouseX, e.MouseY) {
		return false
	}
	// Nested check covers descendants only: the container itself
	// always contains the point this far, so asking it would
	// claim every press for the outer container and starve the
	// inner one.
	for i := range layout.Children {
		if dragPanNestedClaim(&layout.Children[i], e.MouseX, e.MouseY, 0) {
			return false
		}
	}
	id := s.idKey()
	if id == "" {
		return false
	}
	w.viewState.dragPan = dragPanState{
		scrollID:  id,
		startX:    e.MouseX,
		startY:    e.MouseY,
		lastX:     e.MouseX,
		lastY:     e.MouseY,
		modifiers: e.Modifiers,
		active:    true,
	}
	w.MouseLock(MouseLockCfg{
		MouseMove: dragPanOnMouseMove,
		MouseUp:   dragPanOnMouseUp,
		Cancel:    dragPanOnCancel,
	})
	e.IsHandled = true
	return true
}

// dragPanNestedClaim reports whether a nested DragScroll scrollable
// under the point owns this press instead. Scrollbar subtrees hold
// no DragScroll container, so they need no exclusion here. Depth is
// capped like every other dispatch walk.
func dragPanNestedClaim(layout *Layout, x, y float32, depth int) bool {
	if overMaxDepth(depth) {
		return false
	}
	s := layout.Shape
	if s == nil {
		return false
	}
	if s.DragScroll && s.Scrollable && !s.Disabled &&
		s.PointInShape(x, y) {
		return true
	}
	for i := range layout.Children {
		if dragPanNestedClaim(&layout.Children[i], x, y, depth+1) {
			return true
		}
	}
	return false
}

// dragPanOnScrollbar reports whether a scrollbar part of this
// container sits under the point. Scrollbars are appended children
// of the scrollable; their shapes carry a scrollbar orientation,
// and everything nested under one (track, gutter, thumb) belongs
// to the scrollbar drag, never to the content pan.
func dragPanOnScrollbar(layout *Layout, x, y float32) bool {
	for i := range layout.Children {
		if dragPanScrollbarAt(&layout.Children[i], x, y, 0) {
			return true
		}
	}
	return false
}

func dragPanScrollbarAt(layout *Layout, x, y float32, depth int) bool {
	if overMaxDepth(depth) {
		return false
	}
	s := layout.Shape
	if s == nil || !s.PointInShape(x, y) {
		return false
	}
	if s.scrollbarOrientation != scrollbarNone {
		return true
	}
	for i := range layout.Children {
		if dragPanScrollbarAt(&layout.Children[i], x, y, depth+1) {
			return true
		}
	}
	return false
}

// dragPanOnMouseMove pans the claimed scrollable toward the
// pointer. Runs under the mouse lock with window-absolute
// coordinates. Pre-threshold moves only watch the distance; the
// first move past it transfers the press from the tap target to
// the drag and shows the grabbing cursor.
func dragPanOnMouseMove(ctx EventCtx) {
	w := ctx.Window
	e := ctx.Event
	if w == nil || e == nil {
		return
	}
	st := w.viewState.dragPan
	if !st.active {
		return
	}
	// A poisoned coordinate moves nothing and teaches the press
	// nothing: keep the last good position so the next valid move
	// deltas from there instead of from NaN.
	if !f32IsFinite(e.MouseX) || !f32IsFinite(e.MouseY) {
		return
	}
	dx := e.MouseX - st.lastX
	dy := e.MouseY - st.lastY
	st.lastX = e.MouseX
	st.lastY = e.MouseY
	if !st.panned {
		if f32Abs(e.MouseX-st.startX) < dragReorderThreshold &&
			f32Abs(e.MouseY-st.startY) < dragReorderThreshold {
			w.viewState.dragPan = st
			return
		}
		st.panned = true
		// The press belongs to the drag now: a release must not
		// read as held on the tap target.
		w.viewState.pressTargetID = ""
		w.setMouseCursor(CursorGrabbing)
	}
	ly, ok := findScrollLayout(w, st.scrollID)
	if !ok || ly.Shape == nil {
		// The container left the tree mid-drag. Keep the lock so
		// the release still unwinds through dragPanOnMouseUp.
		w.viewState.dragPan = st
		return
	}
	// Physical deltas: the content follows the pointer, so no RTL
	// mirror and no scroll multiplier — a 1:1 drag, not a wheel.
	if dx != 0 && f32IsFinite(dx) &&
		ly.Shape.ScrollMode != ScrollVerticalOnly {
		w.scrollHorizontalBy(st.scrollID, dx)
	}
	if dy != 0 && f32IsFinite(dy) &&
		ly.Shape.ScrollMode != ScrollHorizontalOnly {
		w.scrollVerticalBy(st.scrollID, dy)
	}
	w.viewState.dragPan = st
	w.InvalidateLayout()
}

// dragPanOnMouseUp ends the press. A pan keeps its offset and
// stays silent; a tap replays the press through normal dispatch
// so the child clicks exactly as a plain press would, then runs
// the real release for its OnMouseUp handlers.
func dragPanOnMouseUp(ctx EventCtx) {
	w := ctx.Window
	e := ctx.Event
	if w == nil {
		return
	}
	st := w.viewState.dragPan
	w.viewState.dragPan = dragPanState{}
	w.viewState.mouseButtonHeld = MouseInvalid
	w.viewState.pressTargetID = ""
	w.MouseUnlock()
	if !st.active || e == nil {
		w.InvalidateLayout()
		return
	}
	if st.panned {
		w.InvalidateLayout()
		return
	}
	// The view changed under the press (an OnScroll SetView, for
	// example): the tap target is gone, so unwind silently rather
	// than replaying a press into a new tree.
	if _, ok := findScrollLayout(w, st.scrollID); !ok {
		w.InvalidateLayout()
		return
	}
	down := *e
	down.Type = EventMouseDown
	down.MouseX = st.startX
	down.MouseY = st.startY
	down.MouseButton = MouseLeft
	down.Modifiers = st.modifiers
	down.FrameCount = w.frameCount
	down.IsHandled = false
	down.dragPanReplay = true
	root := dialogRoute(w)
	mouseDownHandler(root, false, &down, w)
	mouseUpHandler(root, e, w)
	w.InvalidateLayout()
}

// dragPanOnCancel unwinds a press the platform revoked. The offset
// stays where the pan left it and no click fires: a cancellation
// is neither a tap nor a committed drop.
func dragPanOnCancel(w *Window) {
	if w == nil {
		return
	}
	w.viewState.dragPan = dragPanState{}
	w.InvalidateLayout()
}

// wrapDragScrollHover chains the grab affordance ahead of the
// caller's own hover handler. The grab shows only while content
// overflows: a container that fits has nothing to drag. Called
// from container() for DragScroll containers only, so containers
// that never pan pay no closure.
func wrapDragScrollHover(cfg *ContainerCfg) {
	user := cfg.OnHover
	leaf := cfg.ID
	cfg.OnHover = func(ctx EventCtx) {
		if id := ctx.EffID(leaf); id != "" &&
			dragScrollOverflows(ctx.Window, id) {
			ctx.Window.setMouseCursor(CursorGrab)
		}
		if user != nil {
			user(ctx)
		}
	}
}

// dragScrollOverflows reports whether the scrollable hides content
// on either axis. Missing IDs answer false: hover can outrun the
// frame that stamps them.
func dragScrollOverflows(w *Window, id string) bool {
	if w == nil {
		return false
	}
	if ly, ok := findScrollLayout(w, id); ok && ly.Shape != nil {
		return scrollMaxOffsetX(ly) < 0 || scrollMaxOffsetY(ly) < 0
	}
	return false
}
