package gui

// EventFn handles user events, dispatching to child views.
// Called by the backend event loop.
func (w *Window) EventFn(e *Event) {
	if e == nil {
		return
	}
	// Time-travel read-only scrub: while frozen, the app window
	// ignores all user input. The debug window drives restore
	// through QueueCommand, which bypasses this gate because
	// commands flush in FrameFn, not EventFn.
	if w.frozen.Load() {
		return
	}
	e.FrameCount = w.frameCount

	if !w.eventAllowed(e) {
		return
	}

	if w.inspectorKeyHook(e) {
		return
	}
	if w.inspectorResizeHook(e) {
		return
	}

	// Top-level layout children represent z-axis layers.
	// Dialogs are modal: route events to last child (dialog layer).
	layout := dialogRoute(w)

	switch e.Type {
	case EventChar:
		w.handleCharEvent(layout, e)
	case EventIMEComposition:
		w.handleIMECompositionEvent(layout, e)
	case EventFocused:
		w.handleFocusedEvent()
	case EventUnfocused:
		w.handleUnfocusedEvent()
	case EventKeyDown:
		w.handleKeyDownEvent(layout, e)
	case EventKeyUp:
		w.handleKeyUpEvent(layout, e)
	case EventMouseDown:
		w.handleMouseDownEvent(layout, e)
	case EventMouseMove:
		w.handleMouseMoveEvent(layout, e)
	case EventMouseUp:
		w.handleMouseUpEvent(layout, e)
	case EventMouseLeave:
		w.pointerLeftWindow()
	case EventMouseScroll:
		w.handleMouseScrollEvent(layout, e)
	case EventResized:
		w.handleResizedEvent(e)
	case EventFileDropped:
		w.handleFileDroppedEvent(layout, e)
	case EventTouchesBegan, EventTouchesMoved,
		EventTouchesEnded, EventTouchesCancelled:
		w.handleTouch(layout, e)
	default:
		// Unhandled event type.
	}

	if !e.IsHandled && w.OnEvent != nil {
		w.OnEvent(e, w)
	}
	w.captureSnapshot(e)
	w.InvalidateLayout()
}

func (w *Window) inspectorKeyHook(e *Event) bool {
	if !inspectorSupported || e.Type != EventKeyDown || e.KeyCode != KeyF12 {
		return false
	}
	inspectorToggle(w)
	e.IsHandled = true
	return true
}

func (w *Window) inspectorResizeHook(e *Event) bool {
	if !inspectorSupported || !w.inspectorEnabled ||
		e.Type != EventKeyDown ||
		e.Modifiers != ModAlt {
		return false
	}
	switch e.KeyCode {
	case KeyLeft:
		inspectorResize(inspectorResizeStep, w)
		e.IsHandled = true
		return true
	case KeyRight:
		inspectorResize(-inspectorResizeStep, w)
		e.IsHandled = true
		return true
	case KeyUp:
		inspectorToggleSide(w)
		e.IsHandled = true
		return true
	}
	return false
}

func (w *Window) handleCharEvent(layout *Layout, e *Event) {
	w.imeClear()
	charHandler(layout, e, w)
}

func (w *Window) handleIMECompositionEvent(layout *Layout, e *Event) {
	imeCompositionHandler(layout, e, w)
}

func (w *Window) handleFocusedEvent() {
	w.focused = true
	// The blink animation was retired while the window sat in the
	// background (syncBlinkCursor). Come back with the caret solid and
	// a fresh 600 ms phase rather than resuming mid-blink; the rebuild
	// EventFn queues re-registers the animation.
	resetBlinkCursorVisible(w)
}

func (w *Window) handleUnfocusedEvent() {
	w.focused = false
	w.imeClear()
	// The key up of a held Space goes to the window that now has focus,
	// so the press would never end: cancel it without a click.
	w.clearKeyPress()
	// The release of a held pointer goes there too, and a drag would
	// keep its lock with no button held: cancel the gesture without
	// committing it, and close popups that belong to this window.
	// MouseCancel clears the press target and the held button with
	// the lock, so IsPressed cannot stick on (#691).
	w.MouseCancel()
	dismissPopups(w)
	dismissFieldPopups(w)
}

func (w *Window) handleKeyDownEvent(layout *Layout, e *Event) {
	// Escape cancels a held Space press without a click, and still
	// goes on to whatever else handles it (closing a dialog, say).
	if e.KeyCode == KeyEscape {
		w.clearKeyPress()
	}
	// Global commands fire before focus dispatch.
	w.commandDispatch(e, true)
	if !e.IsHandled {
		keydownHandler(layout, e, w)
	}
	if !e.IsHandled && e.KeyCode == KeyTab {
		// Match on the keyboard bits only, like scroll dispatch:
		// two backends OR held mouse buttons into Modifiers, and a
		// Ctrl/Alt/Super chord is the focused widget's (or the
		// OS's), not a traversal. Shift+Ctrl+Tab therefore goes
		// nowhere rather than falling through to the next stop.
		switch e.Modifiers & modKeyboard {
		case ModShift:
			if shape, ok := layout.previousFocusable(w); ok {
				w.SetFocus(shape.idKey())
			}
		case ModNone:
			if shape, ok := layout.nextFocusable(w); ok {
				w.SetFocus(shape.idKey())
			}
		}
	}
	// Non-global commands fire as fallback.
	if !e.IsHandled {
		w.commandDispatch(e, false)
	}
}

func (w *Window) handleKeyUpEvent(layout *Layout, e *Event) {
	keyupHandler(layout, e, w)
	// A Space release ends the press whether or not it clicked: the
	// widget may have turned disabled, or an OnKeyUp consumed the key.
	if e.KeyCode == KeySpace {
		w.clearKeyPress()
	}
}

func (w *Window) handleMouseDownEvent(layout *Layout, e *Event) {
	w.viewState.mouseButtonHeld = e.MouseButton
	w.recordPressTarget(e)
	w.setMouseCursor(CursorArrow)
	if inspectorSupported && w.inspectorEnabled {
		panelW := inspectorPanelWidth(w)
		left := inspectorIsLeft(w)
		var inApp bool
		if left {
			inApp = e.MouseX > panelW+inspectorMargin
		} else {
			inApp = e.MouseX < float32(w.windowWidth)-panelW-inspectorMargin
		}
		if inApp {
			if picked := inspectorPickPath(&w.layout, e.MouseX, e.MouseY); picked != "" {
				inspectorSelect(picked, w)
			}
			e.IsHandled = true
		}
	}
	// Dismiss open popups on any mouse down. Cleared
	// before dispatch so handlers can re-open. Focus
	// is only cleared when a popup was actually open
	// to avoid interfering with normal focus flow.
	if dismissPopups(w) {
		w.ClearFocus()
	}
	// Inspector-handled presses never run the walk, so dev-tool clicks
	// leave app focus alone.
	if !e.IsHandled {
		claimed := w.viewState.focusSetCount
		mouseDownHandler(layout, false, e, w)
		w.blurUnclaimedPress(layout, e, claimed)
	}
	if !e.IsHandled {
		ss := StateMap[string, bool](w, nsSelect, capModerate)
		ss.Clear()
		cs := StateMap[string, bool](w, nsCombobox, capModerate)
		cs.Clear()
	}
}

// blurUnclaimedPress clears focus after a press walk that nothing
// claimed, matching native toolkits. claimed is focusSetCount taken
// before the walk. The count, not the ID, is the signal: composite
// widgets (splitter, slider) set focus from their press callbacks,
// including re-asserting the widget that already holds it, and a
// same-ID compare would blur those.
//
// Two presses keep focus:
//   - Right button, the same exclusion the focus take uses, so a
//     context-menu press keeps focus.
//   - A press inside the focused widget's own bounds. A child that
//     consumes the press (the scrollbar of a listbox or a scrollable
//     multiline input) stops the walk before the widget's own focus
//     take runs, and that press must not blur the widget it is in.
//
// Backend presses and touch taps share it, so both follow one rule.
func (w *Window) blurUnclaimedPress(layout *Layout, e *Event, claimed uint64) {
	if e.MouseButton == MouseRight || w.viewState.focusSetCount != claimed {
		return
	}
	// mouseDownHandler accepts a nil tree, so this must too.
	focusID := w.FocusID()
	if focusID == "" || layout == nil {
		return
	}
	if ly, ok := findLayoutByFocusID(layout, focusID); ok &&
		ly.Shape.PointInShape(e.MouseX, e.MouseY) {
		return
	}
	w.ClearFocus()
}

func (w *Window) handleMouseMoveEvent(layout *Layout, e *Event) {
	w.setMouseCursor(CursorArrow)
	w.viewState.menuKeyNav = false
	w.viewState.mousePosX = e.MouseX
	w.viewState.mousePosY = e.MouseY
	w.pointerAt(e.MouseX, e.MouseY)
	mouseMoveHandler(layout, e, w)
}

func (w *Window) handleMouseUpEvent(layout *Layout, e *Event) {
	w.viewState.mouseButtonHeld = MouseInvalid
	w.viewState.pressTargetID = ""
	mouseUpHandler(layout, e, w)
}

func (w *Window) handleMouseScrollEvent(layout *Layout, e *Event) {
	mouseScrollHandler(layout, e, w)
}

func (w *Window) handleResizedEvent(e *Event) {
	w.windowWidth = e.WindowWidth
	w.windowHeight = e.WindowHeight
}

func (w *Window) handleFileDroppedEvent(layout *Layout, e *Event) {
	fileDropHandler(layout, e, w)
}

// eventAllowed returns true when an unfocused window should still
// process this event (right-click, focus, scroll, touch, file drop).
func (w *Window) eventAllowed(e *Event) bool {
	if w.focused {
		return true
	}
	// A window exit must clear hover in a background window too, or
	// it comes back to the foreground still lit.
	return e.Type == EventFocused ||
		e.Type == EventMouseLeave ||
		e.Type == EventMouseScroll ||
		e.Type == EventTouchesBegan ||
		e.Type == EventTouchesMoved ||
		e.Type == EventTouchesEnded ||
		e.Type == EventTouchesCancelled ||
		e.Type == EventFileDropped ||
		(e.Type == EventMouseDown && e.MouseButton == MouseRight)
}

// dismissPopups clears all open popup state maps and returns
// true if any were open.
func dismissPopups(w *Window) bool {
	a := clearStateMap[string, contextMenuState](w, nsContextMenu)
	b := clearStateMap[string, rtfLinkMenuState](w, nsRtfLinkMenu)
	c := clearStateMap[string, string](w, nsMenu)
	return a || b || c
}

// dismissFieldPopups closes open select and combobox dropdowns. It
// stays separate from dismissPopups on purpose: the mouse-down path
// clears these only when nothing handled the press (so a click on
// the field itself still toggles), while window-unfocus and a modal
// dialog close them unconditionally — there is no toggle to protect.
func dismissFieldPopups(w *Window) {
	clearStateMap[string, bool](w, nsSelect)
	clearStateMap[string, bool](w, nsCombobox)
}

// clearStateMap clears a state map if it exists and is non-empty.
func clearStateMap[K comparable, V any](w *Window, ns string) bool {
	sm := StateMapRead[K, V](w, ns)
	if sm == nil || sm.Len() == 0 {
		return false
	}
	sm.Clear()
	return true
}
