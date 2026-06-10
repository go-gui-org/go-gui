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
	layout := &w.layout
	if w.dialogCfg.visible && len(w.layout.Children) > 0 {
		layout = &w.layout.Children[len(w.layout.Children)-1]
	}

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
	w.UpdateWindow()
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
}

func (w *Window) handleUnfocusedEvent() {
	w.focused = false
	w.imeClear()
}

func (w *Window) handleKeyDownEvent(layout *Layout, e *Event) {
	// Global commands fire before focus dispatch.
	w.commandDispatch(e, true)
	if !e.IsHandled {
		keydownHandler(layout, e, w)
	}
	if !e.IsHandled && e.KeyCode == KeyTab &&
		e.Modifiers == ModShift {
		if shape, ok := layout.PreviousFocusable(w); ok {
			w.SetIDFocus(shape.IDFocus)
		}
	} else if !e.IsHandled && e.KeyCode == KeyTab {
		if shape, ok := layout.NextFocusable(w); ok {
			w.SetIDFocus(shape.IDFocus)
		}
	}
	// Non-global commands fire as fallback.
	if !e.IsHandled {
		w.commandDispatch(e, false)
	}
}

func (w *Window) handleKeyUpEvent(layout *Layout, e *Event) {
	keyupHandler(layout, e, w)
}

func (w *Window) handleMouseDownEvent(layout *Layout, e *Event) {
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
		w.SetIDFocus(0)
	}
	if !e.IsHandled {
		mouseDownHandler(layout, false, e, w)
	}
	if !e.IsHandled {
		ss := StateMap[string, bool](w, nsSelect, capModerate)
		ss.Clear()
		cs := StateMap[string, bool](w, nsCombobox, capModerate)
		cs.Clear()
	}
}

func (w *Window) handleMouseMoveEvent(layout *Layout, e *Event) {
	w.setMouseCursor(CursorArrow)
	w.viewState.menuKeyNav = false
	w.viewState.mousePosX = e.MouseX
	w.viewState.mousePosY = e.MouseY
	mouseMoveHandler(layout, e, w)
}

func (w *Window) handleMouseUpEvent(layout *Layout, e *Event) {
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
	return e.Type == EventFocused ||
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
	c := clearStateMap[uint32, string](w, nsMenu)
	return a || b || c
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
