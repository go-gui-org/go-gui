package gui

import "slices"

// maxEventChildren caps traversal depth to prevent DoS from
// maliciously deep or wide layout trees.
const maxEventChildren = 10000

// overMaxChildren reports whether layout has excessive children.
func overMaxChildren(layout *Layout) bool {
	return len(layout.Children) > maxEventChildren
}

// charHandler handles character input events (typing).
// Traverses forward (depth-first) and delivers to focused element.
func charHandler(layout *Layout, e *Event, w *Window) {
	if overMaxChildren(layout) {
		return
	}
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
	var events *eventHandlers
	if layout.Shape.hasEvents() {
		onChar = layout.Shape.events.OnChar
		events = layout.Shape.events
	}
	if executeFocusCallback(layout, e, w, onChar) {
		return
	}
	// Spacebar-to-click: when ClickOnSpace is set, fire OnClick
	// on spacebar instead of requiring a separate OnChar wrapper.
	if events != nil &&
		events.ClickOnSpace &&
		e.CharCode == CharSpace &&
		events.OnClick != nil {
		if isFocusedTarget(layout, w) {
			events.OnClick(layout, e, w)
			e.IsHandled = true
		}
	}
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

	// Guard against excessive children count to prevent DoS.
	if overMaxChildren(layout) {
		return
	}

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
	var events *eventHandlers
	if layout.Shape.hasEvents() {
		onKeyDown = layout.Shape.events.OnKeyDown
		events = layout.Shape.events
	}
	executeFocusCallback(layout, e, w, onKeyDown)
	if e.IsHandled {
		return
	}
	// Enter-to-click: when ClickOnEnter is set, fire OnClick on
	// Enter key instead of requiring a separate OnKeyDown wrapper.
	if events != nil &&
		events.ClickOnEnter &&
		e.KeyCode == KeyEnter &&
		events.OnClick != nil {
		events.OnClick(layout, e, w)
		e.IsHandled = true
		return
	}
	if layout.Shape.Scrollable {
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
	if overMaxChildren(layout) {
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
	if layout.Shape.hasEvents() {
		onKeyUp = layout.Shape.events.OnKeyUp
	}
	executeFocusCallback(layout, e, w, onKeyUp)
}

// keyDownScrollHandler handles keyboard-based scrolling.
// Supports arrow keys, page up/down, and home/end.
const (
	scrollDeltaHome = 10_000_000
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
			e.IsHandled = scrollVertical(layout, scrollDeltaHome, w)
		case KeyEnd:
			e.IsHandled = scrollVertical(layout, -scrollDeltaHome, w)
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
		if layout.Shape.Focusable && layout.Shape.ID != "" &&
			e.MouseButton != MouseRight {
			w.SetFocus(layout.Shape.ID)
			e.IsHandled = true
		}
		var onClick ShapeCallback
		if layout.Shape.hasEvents() {
			events := layout.Shape.events
			if events.ClickButton == 0 ||
				e.MouseButton == events.ClickButton {
				onClick = events.OnClick
			}
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
	if layout.Shape.hasEvents() {
		onMouseMove = layout.Shape.events.OnMouseMove
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
	if layout.Shape.hasEvents() {
		onMouseUp = layout.Shape.events.OnMouseUp
	}
	executeMouseCallback(layout, e, w, onMouseUp)
}

func focusedScrollTarget(layout *Layout, w *Window) *Layout {
	if w == nil {
		return nil
	}
	focusID := w.FocusID()
	if focusID == "" {
		return nil
	}
	ly, ok := FindLayoutByFocusID(layout, focusID)
	if !ok || ly.Shape == nil || !ly.Shape.hasEvents() ||
		ly.Shape.events.OnMouseScroll == nil {
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
		if callRelative(ly, e, w, ly.Shape.events.OnMouseScroll) {
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
	if layout.Shape.hasEvents() &&
		layout.Shape.events.OnMouseScroll != nil {
		if layout.Shape.PointInShape(e.MouseX, e.MouseY) {
			layout.Shape.events.OnMouseScroll(layout, e, w)
			if e.IsHandled {
				return
			}
		}
	}
	// Handle scroll on scroll container under cursor. Discrete mouse
	// wheels (ScrollPrecise == false) ease toward their target via
	// scrollSmoothBy; trackpad/precise deltas already carry OS
	// momentum and scroll instantly.
	if layout.Shape.Scrollable {
		if layout.Shape.PointInShape(e.MouseX, e.MouseY) {
			switch e.Modifiers {
			case ModShift:
				if e.ScrollPrecise {
					e.IsHandled = scrollHorizontal(layout, e.ScrollX, w)
				} else {
					e.IsHandled = scrollSmoothBy(w, layout, scrollAxisX, e.ScrollX)
				}
			case ModNone:
				if e.ScrollPrecise {
					e.IsHandled = scrollVertical(layout, e.ScrollY, w)
				} else {
					e.IsHandled = scrollSmoothBy(w, layout, scrollAxisY, e.ScrollY)
				}
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
	if layout.Shape.hasEvents() {
		onFileDrop = layout.Shape.events.OnFileDrop
	}
	executeMouseCallback(layout, e, w, onFileDrop)
}
