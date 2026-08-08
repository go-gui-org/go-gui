package gui

import "strconv"

// Event-model collapse check.
//
// §4.3b of docs/specs/developer-ergonomics.md proposes deleting the
// consume/notify split: one rule, nothing is pre-marked, every callback
// consumes explicitly. The compile-time half of that change is loud —
// ctx.Bubble() disappears and its call sites stop building. The other
// half is silent, and it is the reason the collapse is still deferred:
// a consume-class callback that relied on the pre-mark and never called
// ctx.Consume() would, under the collapse, leave the event unhandled
// and let it reach an ancestor's handler. No compile error, no panic,
// just a click that now fires twice.
//
// Counting call sites cannot measure this. §4.3.1 counted 138
// consume-class sites in examples/ alone and concluded that most sit on
// widgets with no clickable ancestor — but which ones those are is a
// property of the layout tree at dispatch time, not of the source.
//
// So the check is a counterfactual, evaluated where the answer is
// actually knowable: right after a consume-class callback returns, ask
// (a) did it rely on the pre-mark, and (b) is there an ancestor that
// would have received this same event. Both true is a site that changes
// behaviour under the collapse. Everything else is safe to convert.
//
// Turn it on with gui.Debug(true) or GOGUI_DEBUG=1, exercise the app,
// and read the findings off stderr. Each is reported once per window.

// debugCollapse reports one dispatch as a collapse hazard when the
// callback relied on the consume-class pre-mark and an ancestor would
// also have received the event.
//
// Called from callRelative, executeFocusCallback and the gesture
// dispatch after every named consume-class callback. explicit is the
// callback's own ctx.Consume() decision, passed in because callRelative
// restores the event before calling and would otherwise have handed
// back the enclosing frame's value.
//
// No-op unless [Debug] is on, which is one atomic load.
func debugCollapse(
	class evClass, layout *Layout, e *Event, w *Window, explicit bool,
) {
	// Dispatch already guards on named(); repeated here so the function
	// is safe to call directly and cannot be the reason an unnamed
	// class gets a made-up ancestor rule.
	if !class.named() {
		return
	}
	if !debugEnabled.Load() || w == nil || e == nil {
		return
	}
	// Bubbling already means the callback opted out, and an explicit
	// Consume() already means it would keep working verbatim. Only the
	// silent middle — still handled, never asked for — is at risk.
	if !e.IsHandled || explicit {
		return
	}
	anc := class.ancestorHandler(layout, e, w)
	if anc == nil {
		return
	}
	self := debugShapeName(layout)
	other := debugShapeName(anc)
	w.debugWarn(debugCheckEventCollapse, class.name()+":"+self+">"+other,
		"%s on %s relies on automatic handling and %s above it also "+
			"handles %s; under the one-rule event model (spec §4.3b) the "+
			"event would reach both. Add ctx.Consume() to the %s handler "+
			"to make the current behaviour explicit",
		class.name(), self, other, class.name(), self)
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
		if s == nil || s.Disabled || !s.hasEvents() {
			continue
		}
		if c.wouldReach(anc, e, w) {
			return anc
		}
	}
	return nil
}

// wouldReach reports whether this ancestor's own dispatch condition
// holds for the event. Each consume-class event has a different one.
func (c evClass) wouldReach(anc *Layout, e *Event, w *Window) bool {
	s := anc.Shape
	ev := s.events
	switch c {
	case evClick:
		// The button filter is part of the dispatch condition
		// (mouseDownHandler), so an ancestor listening for right-click
		// only is not reached by a left-click.
		if ev.OnClick == nil ||
			(ev.ClickButton != 0 && e.MouseButton != ev.ClickButton) {
			return false
		}
		return s.PointInShape(e.MouseX, e.MouseY)
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
	case evMouseUp:
		return "OnMouseUp"
	case evFileDrop:
		return "OnFileDrop"
	case evGesture:
		return "OnGesture"
	}
	return "event"
}

// TestEventCollapse sweeps the window for sites that would change
// behaviour under the one-rule event model of spec §4.3b, and returns
// one finding per site.
//
// It renders a frame, then for every shape carrying a consume-class
// callback it synthesizes that event at the shape's centre and runs it
// through real dispatch with the collapse check armed. A finding means
// the callback relied on automatic handling while an ancestor would
// also have received the event.
//
// **This fires the app's callbacks.** Sweeping a window presses every
// button in it, in tree order, and whatever state that mutates stays
// mutated. Call it on a window built for the purpose, not on one an
// assertion still depends on.
//
// Empty result means the window is collapse-safe as rendered. It does
// not mean the app is: a hazard behind a tab, a dialog, or a scrolled
// region is only visible once that state is on screen, so drive the app
// into each interesting state and sweep again.
//
// The debug gate is turned on for the duration and restored after.
func (w *Window) TestEventCollapse() []string {
	root := w.TestRender(nil)
	if root == nil {
		return nil
	}
	var found []string
	prevOn := debugEnabled.Load()
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
	w.sweepCollapse(root)
	return found
}

// sweepCollapse walks the tree depth-first, dispatching one synthetic
// event per consume-class callback it finds.
//
// Dispatch starts from the root every time rather than from the shape,
// because hit-testing, focus and the topmost-first traversal are the
// parts that decide who actually receives the event — starting halfway
// down would skip exactly the logic the check reasons about.
func (w *Window) sweepCollapse(root *Layout) {
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
			&Event{MouseX: cx, MouseY: cy, MouseButton: ev.ClickButton}, w)
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
		w.SetFocus(s.ID)
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
