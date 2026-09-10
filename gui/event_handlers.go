package gui

import "slices"

// Event consumption convention, stated once because the two spellings
// look interchangeable and are not. A shape callback holding an
// EventCtx reports by calling ctx.Consume(); one that does not lets
// the event travel on, and there is no ctx.Bubble. Dispatch internals
// and (e, w) helpers below them hold no ctx, so they set e.IsHandled
// directly — including the spacebar/enter-to-click pre-marks, which
// claim the key for click activation before the callback runs.
//
// "Nothing is marked handled for you" describes propagation, not the
// state of the flag on arrival. Three dispatch-internal claims land
// before a callback runs, so a callback must not read
// ctx.Event.IsHandled as "an earlier handler took this":
//
//  1. Taking focus on mouse-down (mouseDownHandlerDepth) marks the
//     press handled on the way past, then still calls the shape's own
//     OnMouseDown and OnClick. A focusable widget therefore sees the
//     flag already set in its own click handler.
//  2. The spacebar-to-click path claims the space before calling
//     OnClick (charHandlerDepth).
//  3. The enter-to-click path claims the key the same way
//     (keydownHandlerDepth).
//
// What the convention guarantees is the other direction: dispatch never
// infers consumption from the fact that a callback ran, so an ancestor
// keeps receiving the event until some callback calls Consume.

// maxEventDepth caps recursion depth for the tree walks below. A chain
// of single-child layouts would otherwise recurse until the stack gives
// out, and the tree is not always the app's own: markdown and SVG build
// subtrees out of documents the app did not write. Real trees nest dozens
// deep at most; past this the walk stops descending, so the frame drops
// input rather than the process.
const maxEventDepth = 256

// overMaxDepth reports whether a tree walk has descended past the
// depth budget.
func overMaxDepth(depth int) bool {
	return depth > maxEventDepth
}

// modKeyboard selects the keyboard bits of Event.Modifiers, dropping the
// held-mouse-button bits.
//
// Scroll dispatch matches modifiers exactly — ModNone scrolls
// vertically, ModShift horizontally — and two backends OR the buttons
// currently held into the same field (ModLMB/ModRMB/ModMMB, set in
// backend/internal/winkey and backend/web). Matched unmasked, a wheel
// turn while any button is down equals neither case, so scrolling during
// a drag worked on macOS and silently did nothing on Windows and the
// web. Mask first, then match.
const modKeyboard = ModShift | ModCtrl | ModAlt | ModSuper

// A nil root is tolerated at every entry point below and nowhere else.
// Production callers always pass an address — &w.layout, or a z-layer
// child of it — so the check is for callers outside the frame loop;
// keyupHandler has accepted nil since it was written and the rest now
// agree with it. The recursive halves never re-check, because they only
// ever receive &layout.Children[i].

// charHandler handles character input events (typing).
// Traverses forward (depth-first) and delivers to focused element.
func charHandler(layout *Layout, e *Event, w *Window) {
	if layout == nil {
		return
	}
	charHandlerDepth(layout, e, w, 0)
}

func charHandlerDepth(layout *Layout, e *Event, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	for i := range layout.Children {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		charHandlerDepth(&layout.Children[i], e, w, depth+1)
		if e.IsHandled {
			return
		}
	}
	if layout.Shape == nil {
		return
	}
	var onChar shapeCallback
	var events *eventHandlers
	if layout.Shape.hasEvents() {
		onChar = layout.Shape.events.OnChar
		events = layout.Shape.events
	}
	// Delivers to the focused target, which consumes explicitly.
	if executeFocusCallback(layout, e, w, onChar, evChar) {
		return
	}
	// Spacebar-to-click: when ClickOnSpace is set, fire OnClick
	// on spacebar instead of requiring a separate OnChar wrapper.
	// The space is claimed for click activation before the call, so
	// it never also types; an OnClick that declines cannot release
	// it back to the character path.
	if events != nil &&
		events.clickOnSpace &&
		e.CharCode == charSpace &&
		events.OnClick != nil {
		if isFocusedTarget(layout, w) {
			e.IsHandled = true
			playShapeSound(layout, w)
			events.OnClick(EventCtx{layout, e, w})
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
	if layout == nil {
		return
	}
	keydownHandlerDepth(layout, e, w, 0)
}

func keydownHandlerDepth(layout *Layout, e *Event, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}

	for i := range layout.Children {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		keydownHandlerDepth(&layout.Children[i], e, w, depth+1)
		if e.IsHandled {
			return
		}
	}
	if layout.Shape == nil || !isFocusedTarget(layout, w) {
		return
	}
	var onKeyDown shapeCallback
	var events *eventHandlers
	if layout.Shape.hasEvents() {
		onKeyDown = layout.Shape.events.OnKeyDown
		events = layout.Shape.events
	}
	// Nothing is pre-marked: OnKeyDown receives every key, so an
	// implicit claim here would silently kill tab traversal and
	// accelerators in any widget that has a key handler.
	executeFocusCallback(layout, e, w, onKeyDown, evNotify)
	if e.IsHandled {
		return
	}
	// Enter-to-click: when ClickOnEnter is set, fire OnClick on
	// Enter key instead of requiring a separate OnKeyDown wrapper.
	// Claimed here, the way the spacebar path above claims its key:
	// the surrounding OnKeyDown dispatch never pre-marks.
	if events != nil &&
		events.clickOnEnter &&
		e.KeyCode == KeyEnter &&
		events.OnClick != nil {
		e.IsHandled = true
		playShapeSound(layout, w)
		events.OnClick(EventCtx{layout, e, w})
		return
	}
	if layout.Shape.Scrollable {
		keyDownScrollHandler(layout, e, w)
	}
}

// keyupHandler handles key up events.
// Traverses forward and delivers to focused element.
func keyupHandler(layout *Layout, e *Event, w *Window) {
	if layout == nil {
		return
	}
	keyupHandlerDepth(layout, e, w, 0)
}

func keyupHandlerDepth(layout *Layout, e *Event, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}

	for i := range layout.Children {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		keyupHandlerDepth(&layout.Children[i], e, w, depth+1)
		if e.IsHandled {
			return
		}
	}
	if layout.Shape == nil || !isFocusedTarget(layout, w) {
		return
	}
	var onKeyUp shapeCallback
	if layout.Shape.hasEvents() {
		onKeyUp = layout.Shape.events.OnKeyUp
	}
	// OnKeyUp, like OnKeyDown, is never pre-marked.
	executeFocusCallback(layout, e, w, onKeyUp, evNotify)
}

// keyDownScrollHandler handles keyboard-based scrolling.
// Supports arrow keys, page up/down, and home/end.
const (
	scrollDeltaHome = 10_000_000
)

func keyDownScrollHandler(layout *Layout, e *Event, w *Window) {
	// Post-generation read: name the window rather than the installed
	// frame cache, which belongs to whichever window generated last.
	th := w.themeRef()
	deltaLine := th.ScrollDeltaLine
	deltaPage := th.ScrollDeltaPage

	switch e.Modifiers & modKeyboard {
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
	if layout == nil {
		return
	}
	mouseDownHandlerDepth(layout, inHandler, e, w, 0)
}

func mouseDownHandlerDepth(
	layout *Layout, inHandler bool, e *Event, w *Window, depth int,
) {
	if overMaxDepth(depth) {
		return
	}
	// Check mouse lock (only at top level).
	if !inHandler {
		if w.viewState.mouseLock.mouseDown != nil {
			w.viewState.mouseLock.mouseDown(EventCtx{layout, e, w})
			return
		}
	}
	// Traverse children in reverse (topmost/last child first).
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		mouseDownHandlerDepth(&layout.Children[i], true, e, w, depth+1)
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
		if layout.Shape.canTakeFocus() &&
			e.MouseButton != MouseRight {
			w.SetFocus(layout.Shape.idKey())
			e.IsHandled = true
		}
		var onMouseDown shapeCallback
		if layout.Shape.hasEvents() {
			onMouseDown = layout.Shape.events.OnMouseDown
		}
		// evMouseDown names the event for the unconsumed-event debug
		// check; the callback itself consumes explicitly.
		executeMouseCallback(layout, e, w, onMouseDown, evMouseDown)
		var onClick shapeCallback
		if layout.Shape.hasEvents() {
			events := layout.Shape.events
			if events.clickButton == 0 ||
				e.MouseButton == events.clickButton {
				onClick = events.OnClick
			}
		}
		// evClick additionally plays the shape's click cue before
		// the callback, independently of whether it consumes.
		executeMouseCallback(layout, e, w, onClick, evClick)
	}
}

// mouseMoveHandler handles mouse movement events.
// Traverses reverse (topmost first).
func mouseMoveHandler(layout *Layout, e *Event, w *Window) {
	if layout == nil {
		return
	}
	mouseMoveHandlerDepth(layout, e, w, 0)
}

func mouseMoveHandlerDepth(layout *Layout, e *Event, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	if w.viewState.mouseLock.MouseMove != nil {
		w.viewState.mouseLock.MouseMove(EventCtx{layout, e, w})
		return
	}
	if !w.pointerOverApp(e) {
		return
	}
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		mouseMoveHandlerDepth(&layout.Children[i], e, w, depth+1)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil {
		return
	}
	var onMouseMove shapeCallback
	if layout.Shape.hasEvents() {
		onMouseMove = layout.Shape.events.OnMouseMove
	}
	// evNotify carries no ancestor rule, so nested shapes
	// legitimately all want move notifications.
	executeMouseCallback(layout, e, w, onMouseMove, evNotify)
}

// mouseUpHandler handles mouse button release events.
// Traverses reverse (topmost first).
func mouseUpHandler(layout *Layout, e *Event, w *Window) {
	if layout == nil {
		return
	}
	mouseUpHandlerDepth(layout, e, w, 0)
}

func mouseUpHandlerDepth(layout *Layout, e *Event, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	if w.viewState.mouseLock.MouseUp != nil {
		w.viewState.mouseLock.MouseUp(EventCtx{layout, e, w})
		return
	}
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		mouseUpHandlerDepth(&layout.Children[i], e, w, depth+1)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil {
		return
	}
	var onMouseUp shapeCallback
	if layout.Shape.hasEvents() {
		onMouseUp = layout.Shape.events.OnMouseUp
	}
	// evMouseUp names the event for the unconsumed-event debug
	// check; the callback itself consumes explicitly.
	executeMouseCallback(layout, e, w, onMouseUp, evMouseUp)
}

func focusedScrollTarget(layout *Layout, w *Window) *Layout {
	if w == nil {
		return nil
	}
	focusID := w.FocusID()
	if focusID == "" {
		return nil
	}
	ly, ok := findLayoutByFocusID(layout, focusID)
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
	if layout == nil {
		return
	}
	if ly := focusedScrollTarget(layout, w); ly != nil {
		// Cascade-on-unhandled is the designed contract, so there
		// is no pre-mark here: an unhandled scroll falls through to
		// the container below.
		if callRelative(ly, e, w, ly.Shape.events.OnMouseScroll, evNotify) {
			return
		}
	}
	mouseScrollFallbackHandler(layout, e, w)
}

func mouseScrollFallbackHandler(layout *Layout, e *Event, w *Window) {
	mouseScrollFallbackHandlerDepth(layout, e, w, 0)
}

func mouseScrollFallbackHandlerDepth(layout *Layout, e *Event, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		mouseScrollFallbackHandlerDepth(&layout.Children[i], e, w, depth+1)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil || layout.Shape.Disabled {
		return
	}
	// Deliver to OnMouseScroll handler under cursor, through
	// executeMouseCallback like every other pointer dispatch. It used to
	// call the callback directly, which made the coordinate space depend
	// on which path reached the handler: the focused-target branch in
	// mouseScrollHandler goes through callRelative and handed the
	// callback shape-relative coordinates, while this branch handed it
	// screen-space ones. One callback, two meanings for MouseX, selected
	// by whether the shape happened to hold focus — so a canvas zooming
	// at the cursor zoomed at the wrong point as soon as it was clicked.
	//
	// Still no pre-mark: an unhandled scroll falls through to the scroll
	// container below.
	if layout.Shape.hasEvents() {
		if executeMouseCallback(layout, e, w,
			layout.Shape.events.OnMouseScroll, evNotify) {
			return
		}
	}
	// Handle scroll on scroll container under cursor. Discrete mouse
	// wheels (ScrollPrecise == false) ease toward their target via
	// scrollSmoothBy; trackpad/precise deltas already carry OS
	// momentum and scroll instantly.
	if layout.Shape.Scrollable {
		if layout.Shape.PointInShape(e.MouseX, e.MouseY) {
			switch e.Modifiers & modKeyboard {
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
	if layout == nil {
		return
	}
	fileDropHandlerDepth(layout, e, w, 0)
}

func fileDropHandlerDepth(layout *Layout, e *Event, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	ox, oy := rotateMouseInverse(layout.Shape, e)
	for i := range slices.Backward(layout.Children) {
		if !isChildEnabled(&layout.Children[i]) {
			continue
		}
		fileDropHandlerDepth(&layout.Children[i], e, w, depth+1)
		if e.IsHandled {
			e.MouseX, e.MouseY = ox, oy
			return
		}
	}
	e.MouseX, e.MouseY = ox, oy
	if layout.Shape == nil {
		return
	}
	var onFileDrop shapeCallback
	if layout.Shape.hasEvents() {
		onFileDrop = layout.Shape.events.OnFileDrop
	}
	// evFileDrop names the event for the unconsumed-event debug
	// check; the callback itself consumes explicitly.
	executeMouseCallback(layout, e, w, onFileDrop, evFileDrop)
}
