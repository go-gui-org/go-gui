package gui

// ShapeCallback is the type for shape event callbacks.
type ShapeCallback = func(*Layout, *Event, *Window)

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

func executeFocusCallback(
	layout *Layout, e *Event, w *Window,
	callback ShapeCallback,
) bool {
	if !isFocusedTarget(layout, w) {
		return false
	}
	if callback == nil {
		return false
	}
	callback(layout, e, w)
	return e.IsHandled
}

// callRelative translates mouse coordinates to shape-relative,
// calls the callback, restores coordinates, and propagates
// IsHandled. Assumes layout.Shape and callback are non-nil.
func callRelative(
	layout *Layout, e *Event, w *Window,
	callback ShapeCallback,
) bool {
	saved := *e
	e.MouseX = saved.MouseX - layout.Shape.X
	e.MouseY = saved.MouseY - layout.Shape.Y
	callback(layout, e, w)
	handled := e.IsHandled
	*e = saved
	if handled {
		e.IsHandled = true
	}
	return handled
}

// executeMouseCallback executes a callback if the mouse is
// within shape bounds. Coordinates are made relative before
// calling. Returns true if handled.
func executeMouseCallback(
	layout *Layout, e *Event, w *Window,
	callback ShapeCallback,
) bool {
	if layout.Shape == nil ||
		!layout.Shape.PointInShape(e.MouseX, e.MouseY) {
		return false
	}
	if callback == nil {
		return false
	}
	return callRelative(layout, e, w, callback)
}

// isChildEnabled checks if a child layout should receive events.
func isChildEnabled(child *Layout) bool {
	return child.Shape != nil && !child.Shape.Disabled
}
