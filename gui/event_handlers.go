package gui

import "slices"

// charHandler handles character input events (typing).
// Traverses forward (depth-first) and delivers to focused element.
func charHandler(layout *Layout, e *Event, w *Window) {
	for i := range layout.Children {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		charHandler(&layout.Children[i], e, w)
		if e.IsHandled {
			return
		}
	}
	if layout.Shape == nil {
		return
	}
	var onChar ShapeCallback
	if layout.Shape.HasEvents() {
		onChar = layout.Shape.Events.OnChar
	}
	executeFocusCallback(layout, e, w, onChar)
}

// imeCompositionHandler handles IME composition events.
// Updates the per-window IME state for the focused input.
func imeCompositionHandler(_ *Layout, e *Event, w *Window) {
	w.imeUpdate(e)
	e.IsHandled = true
}

// keydownHandler handles key down events (special keys, shortcuts).
// Traverses forward and delivers to focused element. Falls back to
// keyboard scroll if the focused scroll container has no handler.
func keydownHandler(layout *Layout, e *Event, w *Window) {
	for i := range layout.Children {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		keydownHandler(&layout.Children[i], e, w)
		if e.IsHandled {
			return
		}
	}
	if layout.Shape == nil || !isFocusedTarget(layout, w) {
		return
	}
	var onKeyDown ShapeCallback
	if layout.Shape.HasEvents() {
		onKeyDown = layout.Shape.Events.OnKeyDown
	}
	executeFocusCallback(layout, e, w, onKeyDown)
	if e.IsHandled {
		return
	}
	if layout.Shape.IDScroll > 0 {
		keyDownScrollHandler(layout, e, w)
	}
}

// keyupHandler handles key up events.
// Traverses forward and delivers to focused element.
func keyupHandler(layout *Layout, e *Event, w *Window) {
	// Guard against nil layout to prevent panic
	if layout == nil {
		return
	}

	// Guard against excessive children count to prevent DoS
	if len(layout.Children) > 10000 {
		return
	}

	for i := range layout.Children {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		keyupHandler(&layout.Children[i], e, w)
		if e.IsHandled {
			return
		}
	}
	if layout.Shape == nil || !isFocusedTarget(layout, w) {
		return
	}
	var onKeyUp ShapeCallback
	if layout.Shape.HasEvents() {
		onKeyUp = layout.Shape.Events.OnKeyUp
	}
	executeFocusCallback(layout, e, w, onKeyUp)
}

// keyDownScrollHandler handles keyboard-based scrolling.
// Supports arrow keys, page up/down, and home/end.
const (
	ScrollDeltaHome = 10_000_000
)

func keyDownScrollHandler(layout *Layout, e *Event, w *Window) {
	deltaLine := guiTheme.ScrollDeltaLine
	deltaPage := guiTheme.ScrollDeltaPage

	switch e.Modifiers {
	case ModNone:
		switch e.KeyCode {
		case KeyUp:
			e.IsHandled = scrollVertical(layout, deltaLine, w)
		case KeyDown:
			e.IsHandled = scrollVertical(layout, -deltaLine, w)
		case KeyHome:
			e.IsHandled = scrollVertical(layout, ScrollDeltaHome, w)
		case KeyEnd:
			e.IsHandled = scrollVertical(layout, -ScrollDeltaHome, w)
		case KeyPageUp:
			e.IsHandled = scrollVertical(layout, deltaPage, w)
		case KeyPageDown:
			e.IsHandled = scrollVertical(layout, -deltaPage, w)
		}
	case ModShift:
		switch e.KeyCode {
		case KeyLeft:
			e.IsHandled = scrollHorizontal(layout, deltaLine, w)
		case KeyRight:
			e.IsHandled = scrollHorizontal(layout, -deltaLine, w)
		}
	}
}

// mouseDownHandler handles mouse button press events.
// Traverses reverse (topmost first) and delivers to element under
// cursor. Also handles focus changes on click.
func mouseDownHandler(
	layout *Layout, inHandler bool, e *Event, w *Window,
) {
	// Check mouse lock (only at top level).
	if !inHandler {
		if w.viewState.mouseLock.MouseDown != nil {
			w.viewState.mouseLock.MouseDown(layout, e, w)
			return
		}
	}
	// Traverse children in reverse (topmost/last child first).
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		mouseDownHandler(&layout.Children[i], true, e, w)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil {
		return
	}
	if layout.Shape.PointInShape(e.MouseX, e.MouseY) {
		if layout.Shape.IDFocus > 0 && e.MouseButton != MouseRight {
			w.SetIDFocus(layout.Shape.IDFocus)
			e.IsHandled = true
		}
		var onClick ShapeCallback
		if layout.Shape.HasEvents() {
			onClick = layout.Shape.Events.OnClick
		}
		executeMouseCallback(layout, e, w, onClick)
	}
}

// mouseMoveHandler handles mouse movement events.
// Traverses reverse (topmost first).
func mouseMoveHandler(layout *Layout, e *Event, w *Window) {
	if w.viewState.mouseLock.MouseMove != nil {
		w.viewState.mouseLock.MouseMove(layout, e, w)
		return
	}
	if !w.PointerOverApp(e) {
		return
	}
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		mouseMoveHandler(&layout.Children[i], e, w)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil {
		return
	}
	var onMouseMove ShapeCallback
	if layout.Shape.HasEvents() {
		onMouseMove = layout.Shape.Events.OnMouseMove
	}
	executeMouseCallback(layout, e, w, onMouseMove)
}

// mouseUpHandler handles mouse button release events.
// Traverses reverse (topmost first).
func mouseUpHandler(layout *Layout, e *Event, w *Window) {
	if w.viewState.mouseLock.MouseUp != nil {
		w.viewState.mouseLock.MouseUp(layout, e, w)
		return
	}
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		mouseUpHandler(&layout.Children[i], e, w)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil {
		return
	}
	var onMouseUp ShapeCallback
	if layout.Shape.HasEvents() {
		onMouseUp = layout.Shape.Events.OnMouseUp
	}
	executeMouseCallback(layout, e, w, onMouseUp)
}

func focusedScrollTarget(layout *Layout, w *Window) *Layout {
	if w == nil {
		return nil
	}
	idFocus := w.IDFocus()
	if idFocus == 0 {
		return nil
	}
	ly, ok := FindLayoutByIDFocus(layout, idFocus)
	if !ok || ly.Shape == nil || !ly.Shape.HasEvents() ||
		ly.Shape.Events.OnMouseScroll == nil {
		return nil
	}
	return ly
}

// mouseScrollHandler handles mouse wheel scroll events.
// Delivers to the focused element's OnMouseScroll handler first.
// If no focused handler exists, traverses reverse (topmost first)
// and falls back to the scroll container under cursor.
func mouseScrollHandler(layout *Layout, e *Event, w *Window) {
	if ly := focusedScrollTarget(layout, w); ly != nil {
		if callRelative(ly, e, w, ly.Shape.Events.OnMouseScroll) {
			return
		}
	}
	mouseScrollFallbackHandler(layout, e, w)
}

func mouseScrollFallbackHandler(layout *Layout, e *Event, w *Window) {
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		mouseScrollFallbackHandler(&layout.Children[i], e, w)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil || layout.Shape.Disabled {
		return
	}
	// Deliver to OnMouseScroll handler under cursor.
	if layout.Shape.HasEvents() &&
		layout.Shape.Events.OnMouseScroll != nil {
		if layout.Shape.PointInShape(e.MouseX, e.MouseY) {
			layout.Shape.Events.OnMouseScroll(layout, e, w)
			if e.IsHandled {
				return
			}
		}
	}
	// Handle scroll on scroll container under cursor.
	if layout.Shape.IDScroll > 0 {
		if layout.Shape.PointInShape(e.MouseX, e.MouseY) {
			switch e.Modifiers {
			case ModShift:
				e.IsHandled = scrollHorizontal(
					layout, e.ScrollX, w)
			case ModNone:
				e.IsHandled = scrollVertical(
					layout, e.ScrollY, w)
			}
		}
	}
}

// fileDropHandler handles file-drop events. Does not change focus.
func fileDropHandler(layout *Layout, e *Event, w *Window) {
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		fileDropHandler(&layout.Children[i], e, w)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil {
		return
	}
	var onFileDrop ShapeCallback
	if layout.Shape.HasEvents() {
		onFileDrop = layout.Shape.Events.OnFileDrop
	}
	executeMouseCallback(layout, e, w, onFileDrop)
}
