package gui

// ShapeCallback is the type for shape event callbacks. It is a type
// alias so untyped closure literals in Cfg structs compile without a
// conversion.
type ShapeCallback = func(EventCtx)

// isFocusedTarget reports whether the layout has keyboard focus
// (or is the reserved dialog).
func isFocusedTarget(layout *Layout, w *Window) bool {
	if layout.Shape == nil {
		return false
	}
	if layout.Shape.ID == reservedDialogID {
		return true
	}
	if !layout.Shape.Focusable || layout.Shape.ID == "" {
		return false
	}
	return w.IsFocus(layout.Shape.ID)
}

// executeFocusCallback delivers a keyboard event to the focused
// target. class states whether the event is consume- or notify-class;
// the helper serves both (OnChar is consume, OnKeyDown/OnKeyUp are
// notify), so the caller must say which.
func executeFocusCallback(
	layout *Layout, e *Event, w *Window,
	callback ShapeCallback, class evClass,
) bool {
	if !isFocusedTarget(layout, w) {
		return false
	}
	if callback == nil {
		return false
	}
	if class == evConsume {
		e.IsHandled = true
	}
	callback(EventCtx{layout, e, w})
	return e.IsHandled
}

// callRelative translates mouse coordinates to shape-relative,
// calls the callback, restores coordinates, and propagates
// IsHandled. Assumes layout.Shape and callback are non-nil.
//
// class states whether the event is consume- or notify-class. The
// consume pre-mark must land AFTER saved := *e: marking before the
// save would copy IsHandled = true into saved, and the restore below
// would then silently undo any ctx.Bubble() the callback performed.
func callRelative(
	layout *Layout, e *Event, w *Window,
	callback ShapeCallback, class evClass,
) bool {
	saved := *e
	e.MouseX = saved.MouseX - layout.Shape.X
	e.MouseY = saved.MouseY - layout.Shape.Y
	if class == evConsume {
		e.IsHandled = true // pre-mark AFTER the save
	}
	callback(EventCtx{layout, e, w})
	handled := e.IsHandled // a ctx.Bubble() in the callback shows up here
	*e = saved
	if handled {
		e.IsHandled = true
	}
	return handled
}

// executeMouseCallback executes a callback if the mouse is
// within shape bounds. Coordinates are made relative before
// calling. Returns true if handled. class is passed through to
// callRelative — this helper serves both classes (OnClick/OnMouseUp/
// OnFileDrop consume, OnMouseMove notifies).
func executeMouseCallback(
	layout *Layout, e *Event, w *Window,
	callback ShapeCallback, class evClass,
) bool {
	if layout.Shape == nil ||
		!layout.Shape.PointInShape(e.MouseX, e.MouseY) {
		return false
	}
	if callback == nil {
		return false
	}
	return callRelative(layout, e, w, callback, class)
}

// isChildEnabled checks if a child layout should receive events.
func isChildEnabled(child *Layout) bool {
	return child.Shape != nil && !child.Shape.Disabled
}
