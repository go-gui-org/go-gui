package gui

import "slices"

// touch_press.go — a touch press inside a scrollable waits until the
// finger says whether it is a tap or a scroll.
//
// A press dispatches OnClick, so a finger that lands on a row and then
// scrolls the list picked the row first: a touch-down was a click. A
// finger down inside a scrollable that can scroll therefore holds its
// synthesized mouse press back:
//
//  1. A lift before the pan threshold is a tap: the press goes out at
//     the touch point, then the release.
//  2. A pan along an axis the scrollable can scroll is a scroll: the
//     press is dropped, and nothing under the finger sees it.
//  3. A pan along the other axis goes to the widget: the press goes
//     out at the touch point, then the moves (a horizontal slider in a
//     vertical list still drags).
//  4. A long press sends the press before the gesture, as a press
//     sent at touch-down always came first.
//  5. A second finger drops it: a pinch is not a click.
//
// A press on a scrollbar waits too, but its axis is the bar's: a pan
// along the bar sends the press, so the thumb drags, and a pan across it
// drops the press, so the content scrolls instead of the thumb or track
// taking the finger. A bar is thin, and a scroll that started on one was
// lost. A tap sends the press at the lift, so the track still jumps.
//
// The press is not sent after a short hold, as platforms that click on
// release do: here the press is the click, and a finger that rests and
// then scrolls would pick the row again. A press inside a DragScroll
// container (which claims its press itself, drag_scroll.go), outside any
// scrollable, or while the mouse is locked goes out at touch-down, as
// before.

// touchPressScan is what the paths a press under a touch point can
// reach hold.
type touchPressScan struct {
	// v and h report a scrollable on the path that can scroll on
	// that axis.
	v, h bool
	// bar is the orientation of a scrollbar part on the path.
	bar ScrollbarOrientation
	// now reports a part that takes its press at touch-down.
	now bool
}

// decided reports whether a path named a rule for the press.
func (s *touchPressScan) decided() bool {
	return s.now || s.v || s.h || s.bar != scrollbarNone
}

// touchPressDown presses for the finger that went down at x, y: now,
// or held back when it lands in a scrollable that can scroll.
func touchPressDown(gs *gestureState, layout *Layout, w *Window, x, y float32) {
	gs.touchPressDrop()
	var s touchPressScan
	if !w.mouseIsLocked() {
		touchPressScanAt(layout, x, y, 0, &s)
	}
	if !s.now && (s.v || s.h || s.bar != scrollbarNone) {
		gs.pressHeld = true
		gs.pressHeldV = s.v
		gs.pressHeldH = s.h
		gs.pressHeldBar = s.bar
		// The finger is the pointer now, as the press would have
		// made it: the next synthesized move takes its delta from
		// here, not from where a mouse last was.
		w.pointerAt(x, y)
		return
	}
	synthMouse(EventMouseDown, x, y, MouseLeft, layout, w)
	gs.mouseEmitted = true
}

// touchPressSend sends a held press at the touch point.
func touchPressSend(gs *gestureState, layout *Layout, w *Window) {
	if !gs.pressHeld {
		return
	}
	gs.touchPressDrop()
	synthMouse(EventMouseDown, gs.startX, gs.startY, MouseLeft, layout, w)
	gs.mouseEmitted = true
}

// touchPressDrop forgets a held press: nothing receives it.
func (gs *gestureState) touchPressDrop() {
	gs.pressHeld = false
	gs.pressHeldV = false
	gs.pressHeldH = false
	gs.pressHeldBar = scrollbarNone
}

// touchPressOnPan settles a held press when the pan is recognized: a
// pan along an axis the scrollable can scroll drops it, any other
// sends it. On a scrollbar the bar decides: a pan along it sends the
// press, a pan across drops it. dx, dy is the finger's travel from the
// press.
func touchPressOnPan(gs *gestureState, layout *Layout, w *Window, dx, dy float32) {
	if !gs.pressHeld {
		return
	}
	vertical := f32Abs(dy) >= f32Abs(dx)
	if gs.pressHeldBar != scrollbarNone {
		if vertical == (gs.pressHeldBar == scrollbarVertical) {
			touchPressSend(gs, layout, w)
		} else {
			gs.touchPressDrop()
		}
		return
	}
	if (vertical && gs.pressHeldV) || (!vertical && gs.pressHeldH) {
		gs.touchPressDrop()
		return
	}
	touchPressSend(gs, layout, w)
}

// touchPressScanAt walks the paths under x, y a press can reach, as the
// press walk does, and records what they hold in s. Reports whether the
// subtree holds the point. The press walk falls through a child that
// does not take the press to the siblings under it, so a path that names
// no rule (an overlay with no scrollable, say) does not end the scan:
// the list under it still holds the press back.
func touchPressScanAt(layout *Layout, x, y float32, depth int, s *touchPressScan) bool {
	if overMaxDepth(depth) {
		return false
	}
	sh := layout.Shape
	if sh == nil {
		return false
	}
	// A view transform moves the shape and its children.
	x, y = xfCoordsInverse(sh, x, y)
	if pointerMissesSubtree(sh, x, y) {
		return false
	}
	// The children of a quarter turn are in its unturned frame.
	cx, cy := rotateCoordsInverse(sh, x, y)
	hit := false
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		if touchPressScanAt(&layout.Children[i], cx, cy, depth+1, s) {
			hit = true
			if s.decided() {
				break
			}
		}
	}
	if !hit && !sh.PointInShape(x, y) {
		return false
	}
	touchPressNote(layout, s)
	return true
}

// touchPressNote records one shape on the path under the touch.
func touchPressNote(layout *Layout, s *touchPressScan) {
	sh := layout.Shape
	if sh.scrollbarOrientation != scrollbarNone {
		// A thumb or gutter: the bar's axis settles the press.
		s.bar = sh.scrollbarOrientation
		return
	}
	if !sh.Scrollable || sh.Disabled {
		return
	}
	if sh.DragScroll {
		s.now = true
		return
	}
	if sh.ScrollMode != ScrollHorizontalOnly && scrollMaxOffsetY(layout) < 0 {
		s.v = true
	}
	if sh.ScrollMode != ScrollVerticalOnly && scrollMaxOffsetX(layout) < 0 {
		s.h = true
	}
}
