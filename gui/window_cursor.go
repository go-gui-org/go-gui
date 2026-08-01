package gui

// window_cursor.go — mouse cursor management.
//
// Every MouseCursor the backends implement has an exported setter. The set is
// deliberately complete rather than as-needed: an embedder driving the cursor
// from an external protocol (a terminal widget honoring OSC 22, a canvas app
// with its own hit regions) has no way to add one, and falling back to Arrow
// for a shape every backend already loads is a worse result than the platform
// can give.

// setMouseCursor sets the mouse cursor shape.
func (w *Window) setMouseCursor(cursor MouseCursor) {
	w.viewState.mouseCursor = cursor
}

// SetMouseCursorArrow sets the cursor to the default arrow.
func (w *Window) SetMouseCursorArrow() { w.setMouseCursor(CursorArrow) }

// SetMouseCursorIBeam sets the cursor to a text I-beam.
func (w *Window) SetMouseCursorIBeam() { w.setMouseCursor(CursorIBeam) }

// SetMouseCursorCrosshair sets the cursor to a crosshair.
func (w *Window) SetMouseCursorCrosshair() { w.setMouseCursor(CursorCrosshair) }

// SetMouseCursorPointingHand sets the cursor to a pointing hand.
func (w *Window) SetMouseCursorPointingHand() { w.setMouseCursor(CursorPointingHand) }

// SetMouseCursorAll sets the cursor to a resize-all indicator.
func (w *Window) SetMouseCursorAll() { w.setMouseCursor(CursorResizeAll) }

// SetMouseCursorNS sets the cursor to a north-south resize.
func (w *Window) SetMouseCursorNS() { w.setMouseCursor(CursorResizeNS) }

// SetMouseCursorEW sets the cursor to an east-west resize.
func (w *Window) SetMouseCursorEW() { w.setMouseCursor(CursorResizeEW) }

// SetMouseCursorResizeNESW sets the cursor to a NE-SW diagonal resize.
func (w *Window) SetMouseCursorResizeNESW() { w.setMouseCursor(CursorResizeNESW) }

// SetMouseCursorResizeNWSE sets the cursor to a NW-SE diagonal resize.
func (w *Window) SetMouseCursorResizeNWSE() { w.setMouseCursor(CursorResizeNWSE) }

// SetMouseCursorNotAllowed sets the cursor to a not-allowed indicator.
func (w *Window) SetMouseCursorNotAllowed() { w.setMouseCursor(CursorNotAllowed) }
