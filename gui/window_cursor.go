package gui

// window_cursor.go — mouse cursor management.

// setMouseCursor sets the mouse cursor shape.
func (w *Window) setMouseCursor(cursor MouseCursor) {
	w.viewState.mouseCursor = cursor
}

// SetMouseCursorArrow sets the cursor to the default arrow.
func (w *Window) SetMouseCursorArrow() { w.setMouseCursor(CursorArrow) }

// setMouseCursorIBeam sets the cursor to a text I-beam.
func (w *Window) setMouseCursorIBeam() { w.setMouseCursor(CursorIBeam) }

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

// setMouseCursorResizeNESW sets the cursor to a NE-SW resize.
func (w *Window) setMouseCursorResizeNESW() { w.setMouseCursor(CursorResizeNESW) }

// setMouseCursorResizeNWSE sets the cursor to a NW-SE resize.
func (w *Window) setMouseCursorResizeNWSE() { w.setMouseCursor(CursorResizeNWSE) }

// setMouseCursorNotAllowed sets the cursor to a not-allowed indicator.
func (w *Window) setMouseCursorNotAllowed() { w.setMouseCursor(CursorNotAllowed) }
