package gui

// ShapeCallback is the type for shape event callbacks. It is a type
// alias so untyped closure literals in Cfg structs compile without a
// conversion.
type shapeCallback = func(EventCtx)

// isFocusedTarget reports whether the layout has keyboard focus
// (or is the reserved dialog).
func isFocusedTarget(layout *Layout, w *Window) bool {
	if layout.Shape == nil {
		return false
	}
	if layout.Shape.idKey() == reservedDialogID {
		return true
	}
	if !layout.Shape.canTakeFocus() {
		return false
	}
	// The focus store holds effective IDs, so compare on idKey, not on
	// the leaf the widget was written with. The reserved-dialog arm
	// above matches on idKey for the same reason: a dialog is its own
	// float root, where leaf and effID are equal, while a scoped
	// widget whose leaf merely spells the reserved ID addresses
	// "scope:___dialog_reserved_do_not_use___" and must not inherit
	// the dialog's dispatch.
	return w.IsFocus(layout.Shape.idKey())
}

// executeFocusCallback delivers a keyboard event to the focused
// target. class names the event for the debug check; it no longer
// selects a dispatch rule, because there is only one.
func executeFocusCallback(
	layout *Layout, e *Event, w *Window,
	callback shapeCallback, class evClass,
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
//
// Only the two coordinates are saved, not the whole Event. This used to
// copy *e out and back, which is 352 bytes each way to change eight,
// and — the reason it matters — the restore reverted every field a
// callback had written, with IsHandled reinstated by hand afterwards as
// the single exception. A callback that wrote anything else to the event
// had its write silently undone. Saving the two fields dispatch itself
// changes leaves the callback's own writes alone.
func callRelative(
	layout *Layout, e *Event, w *Window,
	callback shapeCallback, class evClass,
) bool {
	savedX, savedY := e.MouseX, e.MouseY
	e.MouseX = savedX - layout.Shape.X
	e.MouseY = savedY - layout.Shape.Y
	// Sound fires before the callback and independently of whether the
	// callback consumes: the cue confirms the widget was activated, it
	// is not a propagation decision (issue #446).
	if class == evClick {
		playShapeSound(layout, w)
	}
	callback(EventCtx{layout, e, w})
	handled := e.IsHandled
	e.MouseX, e.MouseY = savedX, savedY
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
// calling. Returns true if handled. class names the event for the
// unconsumed-event debug check, nothing more: the callback itself
// decides consumption with ctx.Consume().
func executeMouseCallback(
	layout *Layout, e *Event, w *Window,
	callback shapeCallback, class evClass,
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
