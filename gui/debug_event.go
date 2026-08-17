package gui

import "strconv"

// Unconsumed-event check.
//
// §4.3b of docs/specs/developer-ergonomics.md deleted the
// consume/notify split in v0.55.0: one rule, nothing is pre-marked,
// every callback consumes explicitly. The compile-time half of that
// change was loud — ctx.Bubble() disappeared and its call sites stopped
// building. This check covers the other half, which is silent: a
// callback that acts on an event and forgets to consume it lets that
// event carry on to an ancestor. No compile error, no panic, just a
// click that fires twice.
//
// Reading the source cannot answer it. Whether an unconsumed event
// reaches a second handler is a property of the layout tree at dispatch
// time, so the check is evaluated where the answer is knowable: right
// after a callback returns, ask whether the event is still unhandled
// and whether an ancestor would receive it. Both true is a double
// dispatch.
//
// The one false positive to know about is deliberate pass-through — a
// handler that inspects an event, decides it is not its own, and leaves
// it alone. That is now the ordinary way to decline, and it is
// indistinguishable from forgetting. Findings are a list to read, not
// a list of bugs.
//
// Turn it on with gui.Debug(true) or GOGUI_DEBUG=1, exercise the app,
// and read the findings off stderr. Each is reported once per window.
// [DebugCategories] can enable this check without the frame audits.

// debugUnconsumed reports one dispatch where a callback acted on an
// event, did not consume it, and an ancestor will therefore also
// receive it.
//
// Called from callRelative, executeFocusCallback and the gesture
// dispatch after every named callback.
//
// No-op unless the unconsumed category is on, which is one atomic load.
func debugUnconsumed(
	class evClass, layout *Layout, e *Event, w *Window,
) {
	// Dispatch already guards on named(); repeated here so the function
	// is safe to call directly and cannot be the reason an unnamed
	// class gets a made-up ancestor rule.
	if !class.named() {
		return
	}
	if DebugCategory(debugMask.Load())&DebugUnconsumed == 0 || w == nil ||
		e == nil {
		return
	}
	// A consumed event is going nowhere, so there is nothing to report.
	if e.IsHandled {
		return
	}
	anc := class.ancestorHandler(layout, e, w)
	if anc == nil {
		return
	}
	self := debugShapeName(layout)
	other := debugShapeName(anc)
	w.debugWarn(debugCheckUnconsumed, class.name()+":"+self+">"+other,
		"%s on %s did not consume the event and %s above it %s, so both "+
			"will run. Call ctx.Consume() in the %s handler if it means "+
			"to act on the event; ignore this if it means to pass it on",
		class.name(), self, other, class.ancestorReason(anc, e, w), self)
}

// ancestorHandler returns the nearest ancestor of layout that would
// have received this event had the callback left it unhandled, or nil.
//
// This mirrors what dispatch does, in reverse. The mouse handlers
// recurse depth-first into children and only run a shape's own callback
// once every child has declined, so "the next handler" is exactly the
// nearest enclosing shape with a live callback. The event's coordinates
// are back in the enclosing space by the time this runs — callRelative
// restores them before calling.
func (c evClass) ancestorHandler(layout *Layout, e *Event, w *Window) *Layout {
	for anc := layout.Parent; anc != nil; anc = anc.Parent {
		s := anc.Shape
		// Dispatch skips disabled subtrees (isChildEnabled), so a
		// disabled ancestor is not a second handler.
		if s == nil || s.Disabled {
			continue
		}
		// A focusable ancestor is a second handler even with no
		// callback at all: mouseDownHandler takes focus on the way past
		// (and marks the event handled doing so), so an unconsumed click
		// on a non-focusable child would move focus to the ancestor —
		// a behaviour change with no OnClick anywhere in sight. Checked
		// before hasEvents(), which such an ancestor need not have.
		if c == evClick && s.stealsFocusOn(e) {
			return anc
		}
		if !s.hasEvents() {
			continue
		}
		if c.wouldReach(anc, e, w) {
			return anc
		}
	}
	return nil
}

// ancestorReason describes, for the finding text, what the ancestor
// would do with the event. Distinguishing the two matters: "also
// handles OnClick" sends the reader looking for a callback that, in the
// focus case, does not exist.
func (c evClass) ancestorReason(anc *Layout, e *Event, w *Window) string {
	s := anc.Shape
	if s.hasEvents() && c.wouldReach(anc, e, w) {
		return "also handles " + c.name()
	}
	return "would take focus on " + c.name()
}

// stealsFocusOn reports whether mouseDownHandler would take focus for
// this shape, which is the branch at gui/event_handlers.go:216. Right
// clicks are excluded there and so are excluded here.
func (s *Shape) stealsFocusOn(e *Event) bool {
	return s.Focusable && s.ID != "" && e.MouseButton != MouseRight &&
		s.PointInShape(e.MouseX, e.MouseY)
}

// wouldReach reports whether this ancestor's own dispatch condition
// holds for the event. Each named event has a different one.
func (c evClass) wouldReach(anc *Layout, e *Event, w *Window) bool {
	s := anc.Shape
	ev := s.events
	switch c {
	case evClick:
		// The button filter is part of the dispatch condition
		// (mouseDownHandler), so an ancestor listening for right-click
		// only is not reached by a left-click.
		if ev.OnClick == nil ||
			(ev.clickButton != 0 && e.MouseButton != ev.clickButton) {
			return false
		}
		return s.PointInShape(e.MouseX, e.MouseY)
	case evMouseDown:
		return ev.OnMouseDown != nil && s.PointInShape(e.MouseX, e.MouseY)
	case evMouseUp:
		return ev.OnMouseUp != nil && s.PointInShape(e.MouseX, e.MouseY)
	case evFileDrop:
		return ev.OnFileDrop != nil && s.PointInShape(e.MouseX, e.MouseY)
	case evGesture:
		// gestureHandler hit-tests the centroid, not the mouse.
		return ev.OnGesture != nil && s.PointInShape(e.CentroidX, e.CentroidY)
	case evChar:
		// charHandler delivers only to the focused target, and a window
		// has one focus ID — so this is nearly always false, and the
		// dialog shape (reservedDialogID, which isFocusedTarget treats
		// as focused unconditionally) is the one case that fires.
		return ev.OnChar != nil && isFocusedTarget(anc, w)
	}
	return false
}

// name is the callback spelling, for the finding text and the
// warn-once key.
func (c evClass) name() string {
	switch c {
	case evClick:
		return "OnClick"
	case evChar:
		return "OnChar"
	case evMouseDown:
		return "OnMouseDown"
	case evMouseUp:
		return "OnMouseUp"
	case evFileDrop:
		return "OnFileDrop"
	case evGesture:
		return "OnGesture"
	}
	return "event"
}

// TestUnconsumedEvents sweeps the window for handlers that act on an
// event without consuming it while an ancestor would also receive it,
// and returns one finding per site.
//
// It renders a frame, then for every shape carrying one of the six
// hit-tested callbacks it synthesizes that event at the shape's centre
// and runs it through real dispatch with the check armed.
//
// **This fires the app's callbacks.** Sweeping a window presses every
// button in it, in tree order, and whatever state that mutates stays
// mutated. Call it on a window built for the purpose, not on one an
// assertion still depends on.
//
// Read the findings, do not simply drive them to zero: a handler that
// inspects an event, decides it is not its own and passes it on is
// indistinguishable from one that forgot, and both are reported.
//
// An empty result covers the window as rendered, not the app: a site
// behind a tab, a dialog or a scrolled region is only visible once that
// state is on screen, so drive the app into each interesting state and
// sweep again.
//
// The debug gate is turned on for the duration and restored after.
//
// Named TestUnconsumedEvents before v0.55.0, when it measured the cost of
// a collapse that has since happened.
// exportaudit:keep — public testing API in the documented Test* family
func (w *Window) TestUnconsumedEvents() []string {
	root := w.TestRender(nil)
	if root == nil {
		return nil
	}
	var found []string
	prevOn := DebugEnabled()
	// A fresh warn-once map: a sweep should report the window in front
	// of it, not skip what an earlier sweep or a stray frame reported.
	prevWarned := w.debug.warned
	w.debug.warned = nil
	w.debug.collect = &found
	Debug(true)
	defer func() {
		Debug(prevOn)
		w.debug.collect = nil
		w.debug.warned = prevWarned
	}()
	w.sweepUnconsumed(root)
	return found
}

// sweepUnconsumed walks the tree depth-first, dispatching one synthetic
// event per consume-class callback it finds.
//
// Dispatch starts from the root every time rather than from the shape,
// because hit-testing, focus and the topmost-first traversal are the
// parts that decide who actually receives the event — starting halfway
// down would skip exactly the logic the check reasons about.
func (w *Window) sweepUnconsumed(root *Layout) {
	var walk func(l *Layout)
	walk = func(l *Layout) {
		if s := l.Shape; s != nil && s.hasEvents() && !s.Disabled {
			w.sweepShape(root, l)
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	walk(root)
}

// sweepShape fires one synthetic event per consume-class callback on
// this shape. OnFileDrop is skipped: synthesizing a drop means inventing
// a file path, and an app that acts on it would touch the filesystem.
func (w *Window) sweepShape(root, l *Layout) {
	s := l.Shape
	ev := s.events
	cx := s.shapeClip.X + s.shapeClip.Width/2
	cy := s.shapeClip.Y + s.shapeClip.Height/2
	if ev.OnClick != nil {
		mouseDownHandler(root, false,
			&Event{MouseX: cx, MouseY: cy, MouseButton: ev.clickButton}, w)
	}
	if ev.OnMouseDown != nil {
		mouseDownHandler(root, false, &Event{MouseX: cx, MouseY: cy}, w)
	}
	if ev.OnMouseUp != nil {
		mouseUpHandler(root, &Event{MouseX: cx, MouseY: cy}, w)
	}
	if ev.OnGesture != nil {
		gestureHandler(root, &Event{CentroidX: cx, CentroidY: cy}, w)
	}
	// OnChar reaches only the focused target, so focus has to move
	// there first. Restored afterwards: a sweep should not leave the
	// window focused on whatever it happened to visit last.
	if ev.OnChar != nil && s.ID != "" {
		prev := w.FocusID()
		// idKey, not ID: focus is keyed by resolved identity, and
		// focusing a bare leaf under a scoped ancestor would deliver the
		// probe nowhere and silently under-report.
		w.SetFocus(s.idKey())
		charHandler(root, &Event{CharCode: 'x'}, w)
		w.SetFocus(prev)
	}
}

// debugShapeName identifies a shape in a finding. Prefers the ID; falls
// back to the position, which is not stable across a resize but is
// enough to find the widget on screen and to keep two unnamed siblings
// from sharing a warn-once key.
func debugShapeName(layout *Layout) string {
	s := layout.Shape
	if s == nil {
		return "<no shape>"
	}
	if s.ID != "" {
		return strconv.Quote(s.ID)
	}
	return "the ID-less shape at " +
		strconv.FormatFloat(float64(s.X), 'g', -1, 32) + "," +
		strconv.FormatFloat(float64(s.Y), 'g', -1, 32)
}
