package gui

// window_idle_move.go — pointer moves that need no rebuild (#973).
//
// EventFn asks for a full layout rebuild after every event. For a pointer
// move that is usually waste: the view phase is the per-frame allocator,
// and a pointer crossing an idle window paid for it on every frame.
//
// A move is idle when nothing the last arranged frame encodes can depend on
// it. The hover state is computed during arrange, not at event time:
// layoutArrange fires OnHover and OnMouseLeave from the pointer position and
// records the IsHovered target (layout_arrange.go). So the check replays
// those decisions against the arranged tree, for the position the last
// arrange saw and for the new one, and calls the move idle only when every
// answer is the same. Anything it cannot prove unchanged makes it rebuild.
// When the check is wrong, the cost is a rebuild that was not needed, never
// a stale frame.
//
// An idle move still records the pointer position, so the next real pass
// hit-tests where the pointer is now. It does not reset the cursor, so it
// is idle only when the cursor already is the one a real move would leave:
// the arrow, or the cursor the last arrange chose. A cursor set at event
// time (an RTF link's OnMouseMove, a drag) differs, and the move rebuilds.

// idleMovePoints holds the three positions the walk compares. Passed by
// value: a pointer to it would make the recursive walk move it to the heap.
type idleMovePoints struct {
	// oldX/Y is mousePosX/Y as the last arrange pass saw it: where
	// OnHover and OnMouseLeave last ran.
	oldX, oldY float32
	// mouseX/Y is mousePosX/Y after the move. A touch drag does not move
	// it (pointerAt), so for a touch it equals the current mousePosX/Y.
	mouseX, mouseY float32
	// moveX/Y is where the move is dispatched: the event position for a
	// mouse move, the finger for a touch. OnMouseMove fires there.
	// prevMoveX/Y is the previous dispatch point (pointerX/Y before the
	// move).
	moveX, moveY         float32
	prevMoveX, prevMoveY float32
	// rawOld and rawNew are oldX/Y and mouseX/Y before any rotation:
	// a pointerAmend hook tests the window position, unrotated.
	rawOldX, rawOldY, rawNewX, rawNewY float32
}

// idleMoveAt reports whether a pointer move needs no rebuild. (mouseX,
// mouseY) is the mouse position after the move; (moveX, moveY) is where the
// move is dispatched and where the hover target is recorded from. mouse is
// false for a touch drag, whose synthesized move resets no cursor.
//
// Main-thread only, like EventFn. Allocates nothing.
func (w *Window) idleMoveAt(mouseX, mouseY, moveX, moveY float32, mouse bool) bool {
	layers := w.layout.Children
	if len(layers) == 0 {
		// No arranged frame yet: nothing to compare against.
		return false
	}
	// A drag sends moves to its lock handler. An app OnEvent sees every
	// unhandled event and may act on it. The first move after keyboard
	// menu navigation ends that mode, which the menu shows.
	if w.mouseIsLocked() || w.OnEvent != nil || w.viewState.menuKeyNav {
		return false
	}
	// A NaN position compares false against every bound, so it would read
	// as "outside everything" at both points and pass as idle. Nothing can
	// be proved about it: rebuild, as before #973.
	if !f32IsFinite(mouseX) || !f32IsFinite(mouseY) ||
		!f32IsFinite(moveX) || !f32IsFinite(moveY) {
		return false
	}
	// The first move into the window starts hover.
	if !w.viewState.pointerInWindow {
		return false
	}
	// A real mouse move resets the cursor to the arrow, then the arrange
	// pass sets it again. Skip only when that would change nothing. A
	// synthesized touch move resets nothing (synthMouse).
	if mouse && w.viewState.mouseCursor != w.viewState.arrangedCursor {
		return false
	}
	// Tooltip state is driven by AmendLayout hooks and timers that read
	// the pointer position. While any tooltip is pending or shown, every
	// move rebuilds, so a leave is never missed.
	if ts := &w.viewState.tooltip; ts.hoverID != "" || ts.id != "" || ts.text != "" {
		return false
	}
	// IsHovered reads this target during generation.
	if interactionTargetAt(layers, moveX, moveY, w) != w.viewState.hoverTargetID {
		return false
	}
	p := idleMovePoints{
		oldX: w.viewState.arrangedMouseX, oldY: w.viewState.arrangedMouseY,
		mouseX: mouseX, mouseY: mouseY,
		moveX: moveX, moveY: moveY,
		prevMoveX: w.viewState.pointerX, prevMoveY: w.viewState.pointerY,
		rawOldX: w.viewState.arrangedMouseX, rawOldY: w.viewState.arrangedMouseY,
		rawNewX: mouseX, rawNewY: mouseY,
	}
	for i := range layers {
		if !idleMoveWalk(&layers[i], p, 0) {
			return false
		}
	}
	return true
}

// idleMoveWalk reports whether no shape in layout reacts to the move. It
// looks at every shape, not only the ones dispatch would reach: a shape that
// dispatch would skip only costs a rebuild here, never a stale frame.
func idleMoveWalk(layout *Layout, p idleMovePoints, depth int) bool {
	if overMaxDepth(depth) {
		return false
	}
	s := layout.Shape
	if s == nil {
		return true
	}
	if s.hasEvents() && !idleMoveShape(s, p) {
		return false
	}
	// Children of a rotated container are tested in its unrotated frame,
	// as layoutHoverDepth and mouse dispatch do. The raw positions stay.
	if s.QuarterTurns > 0 {
		p.oldX, p.oldY = rotateCoordsInverse(s, p.oldX, p.oldY)
		p.mouseX, p.mouseY = rotateCoordsInverse(s, p.mouseX, p.mouseY)
		p.moveX, p.moveY = rotateCoordsInverse(s, p.moveX, p.moveY)
		p.prevMoveX, p.prevMoveY = rotateCoordsInverse(s, p.prevMoveX, p.prevMoveY)
	}
	for i := range layout.Children {
		if !idleMoveWalk(&layout.Children[i], p, depth+1) {
			return false
		}
	}
	return true
}

// idleMoveShape reports whether this shape's handlers ignore the move.
func idleMoveShape(s *Shape, p idleMovePoints) bool {
	ev := s.events
	// OnMouseMove runs at event time and may change any state. Leaving
	// its shape counts too: the handler may have set state or a cursor
	// that only a rebuild clears.
	if ev.OnMouseMove != nil &&
		(s.PointInShape(p.moveX, p.moveY) || s.PointInShape(p.prevMoveX, p.prevMoveY)) {
		return false
	}
	// OnHover and OnMouseLeave run during arrange, never on a disabled
	// shape (layoutHoverDepth, layoutMouseLeaveDepth).
	if !s.Disabled {
		if ev.OnHover != nil || ev.hover.isSet() {
			was := s.PointInShape(p.oldX, p.oldY)
			now := s.PointInShape(p.mouseX, p.mouseY)
			// Entering or leaving changes which OnHover fires. Staying
			// inside matters too, unless the hover only depends on being
			// inside: an app OnHover may read the pointer position. A
			// HoverStyle alone runs no app code, so it is static (#977).
			static := ev.OnHover == nil || ev.hoverStatic
			if was != now || (now && !static) {
				return false
			}
		}
		if ev.OnMouseLeave != nil && s.ID != "" &&
			s.PointInShape(p.oldX, p.oldY) != s.PointInShape(p.mouseX, p.mouseY) {
			return false
		}
	}
	// An AmendLayout that tests the pointer against the shape's bounds
	// (WithTooltip) must see a move into or out of them.
	// It tests the arranged bounds, not the clip PointInShape reads.
	if ev.pointerAmend {
		b := drawClip{X: s.X, Y: s.Y, Width: s.Width, Height: s.Height}
		if pointInRectangle(p.rawOldX, p.rawOldY, b) || pointInRectangle(p.rawNewX, p.rawNewY, b) {
			return false
		}
	}
	return true
}

// skipIdleMove records an idle move: the pointer position and nothing else.
// mouse is false for a touch, which does not move mousePosX/Y. EventFn
// reads idleMove to skip the rebuild it would otherwise ask for.
func (w *Window) skipIdleMove(x, y float32, mouse bool) {
	if mouse {
		w.viewState.mousePosX = x
		w.viewState.mousePosY = y
	}
	w.pointerAt(x, y)
	w.idleMove = true
}

// takeIdleMove reports whether the event just dispatched was an idle move,
// counts it, and clears the mark.
func (w *Window) takeIdleMove() bool {
	if !w.idleMove {
		return false
	}
	w.idleMove = false
	w.idleMovesSkipped++
	w.idleMovesPending++
	return true
}
