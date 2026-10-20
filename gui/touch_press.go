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
	// took reports a shape on the path that takes the press (a focusable
	// control, or one with a press or click handler). The press walk
	// stops there and never reaches the siblings under it, so the scan
	// stops too.
	took bool
	// flung are the containers the touch stopped mid-fling (input), and
	// caught reports one of them is on the path: a tap there only stops
	// the fling.
	flung  []string
	caught bool
}

// decided reports whether a path named a rule for the press.
func (s *touchPressScan) decided() bool {
	return s.now || s.v || s.h || s.bar != scrollbarNone
}

// touchPressDown presses for the finger that went down at x, y: now,
// or held back when it lands in a scrollable that can scroll. flung are
// the containers this touch stopped mid-fling.
func touchPressDown(gs *gestureState, layout *Layout, w *Window, x, y float32, flung []string) {
	gs.touchPressDrop()
	s := touchPressScan{flung: flung}
	if !w.mouseIsLocked() {
		touchPressScanAt(layout, x, y, 0, &s)
	}
	if !s.now && (s.v || s.h || s.bar != scrollbarNone) {
		gs.pressHeld = true
		// The IDs are scratch, reused by the next touch: keep only the
		// answer.
		s.flung = nil
		gs.pressScan = s
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
	gs.pressScan = touchPressScan{}
}

// touchPressOnPan settles a held press when the pan is recognized: a
// pan along an axis the scrollable can scroll drops it, any other
// sends it. On a scrollbar the bar decides: a pan along it sends the
// press, a pan across drops it. dx, dy is the finger's travel from the
// press.
//
// A pan goes across a scrollable only when it runs clearly across its
// axis, touchPressCrossRatio times farther than along it. A finger
// rarely moves straight: a diagonal swipe on a vertical list that runs
// a bit more sideways than down is a scroll, and with a plain
// |dy| >= |dx| test it pressed the row first (#1116). On a scrollable
// that scrolls both ways every pan is a scroll.
func touchPressOnPan(gs *gestureState, layout *Layout, w *Window, dx, dy float32) {
	if !gs.pressHeld {
		return
	}
	ax, ay := f32Abs(dx), f32Abs(dy)
	if bar := gs.pressScan.bar; bar != scrollbarNone {
		// A bar is thin: the dominant axis decides, with no bias.
		if (ay >= ax) == (bar == scrollbarVertical) {
			touchPressSend(gs, layout, w)
		} else {
			gs.touchPressDrop()
		}
		return
	}
	if (gs.pressScan.v && ax <= touchPressCrossRatio*ay) ||
		(gs.pressScan.h && ay <= touchPressCrossRatio*ax) {
		gs.touchPressDrop()
		return
	}
	touchPressSend(gs, layout, w)
}

// touchPressScanAt walks the paths under x, y a press can reach, as the
// press walk does, and records what they hold in s. Reports whether the
// subtree holds the point. The press walk falls through a child that
// does not take the press to the siblings under it, so a path that names
// no rule and takes no press (an empty overlay, say) does not end the
// scan: the list under it still holds the press back. A path that takes
// the press (a slider in an overlay) ends it, as it ends the press walk.
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
			// A child that takes the press hides the siblings under it
			// from the press walk: a slider in an overlay over a list
			// gets the press, so the list must not hold it back.
			if s.decided() || s.took {
				break
			}
		}
	}
	if !hit && !sh.PointInShape(x, y) {
		return false
	}
	touchPressNote(layout, s)
	// The press walk runs a shape's own focus and handlers only when the
	// point is in the shape itself, not in a child that reaches past it.
	// A handler that does not consume is counted as taking the press:
	// the scan cannot know at touch-down what the callback will do. A touch
	// press is a left press; the press walk's own checks decide, so a
	// right-click-only OnClick does not count.
	if sh.PointInShape(x, y) && (pressFocuses(sh, MouseLeft) ||
		pressOnMouseDown(sh) != nil || pressOnClick(sh, MouseLeft) != nil) {
		s.took = true
	}
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
	if len(s.flung) > 0 && slices.Contains(s.flung, sh.idKey()) {
		s.caught = true
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
