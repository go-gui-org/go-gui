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
	// The focus store holds effective IDs, so compare on idKey, not on
	// the leaf the widget was written with. reservedDialogID above stays
	// a leaf comparison: a dialog is its own float root, where leaf and
	// effID are equal.
	return w.IsFocus(layout.Shape.idKey())
}

// executeFocusCallback delivers a keyboard event to the focused
// target. class names the event for the debug check; it no longer
// selects a dispatch rule, because there is only one.
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
	callback(EventCtx{layout, e, w})
	if class.named() {
		debugUnconsumed(class, layout, e, w)
	}
	return e.IsHandled
}

// callRelative translates mouse coordinates to shape-relative,
// calls the callback, restores coordinates, and propagates
// IsHandled. Assumes layout.Shape and callback are non-nil.
//
// class names the event for the debug check and no longer selects a
// dispatch rule. The pre-mark that used to land here — and the
// save/restore ordering it forced — is gone with spec §4.3b.
func callRelative(
	layout *Layout, e *Event, w *Window,
	callback ShapeCallback, class evClass,
) bool {
	saved := *e
	e.MouseX = saved.MouseX - layout.Shape.X
	e.MouseY = saved.MouseY - layout.Shape.Y
	callback(EventCtx{layout, e, w})
	handled := e.IsHandled
	*e = saved
	if handled {
		e.IsHandled = true
	}
	// The debug check runs on the restored event: its ancestor test
	// needs the coordinates in the enclosing shape's space, not the
	// shape-relative ones the callback saw.
	if class.named() {
		debugUnconsumed(class, layout, e, w)
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
