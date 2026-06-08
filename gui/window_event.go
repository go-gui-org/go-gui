package gui

// EventFn handles user events, dispatching to child views.
// Called by the backend event loop.
//
//nolint:gocyclo // event dispatch switch
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

	// Focus gate: block events when unfocused except right-click,
	// focused, scroll, and touch.
	if !w.focused &&
		(e.Type != EventMouseDown || e.MouseButton != MouseRight) &&
		e.Type != EventFocused &&
		e.Type != EventMouseScroll &&
		e.Type != EventTouchesBegan &&
		e.Type != EventTouchesMoved &&
		e.Type != EventTouchesEnded &&
		e.Type != EventFileDropped &&
		e.Type != EventTouchesCancelled {
		return
	}

	if inspectorSupported && e.Type == EventKeyDown &&
		e.KeyCode == KeyF12 {
		inspectorToggle(w)
		e.IsHandled = true
		return
	}
	if inspectorSupported && w.inspectorEnabled &&
		e.Type == EventKeyDown &&
		e.Modifiers == ModAlt {
		switch e.KeyCode {
		case KeyLeft:
			inspectorResize(inspectorResizeStep, w)
			e.IsHandled = true
			return
		case KeyRight:
			inspectorResize(-inspectorResizeStep, w)
			e.IsHandled = true
			return
		case KeyUp:
			inspectorToggleSide(w)
			e.IsHandled = true
			return
		}
	}

	// Top-level layout children represent z-axis layers.
	// Dialogs are modal: route events to last child (dialog layer).
	layout := &w.layout
	if w.dialogCfg.visible && len(w.layout.Children) > 0 {
		layout = &w.layout.Children[len(w.layout.Children)-1]
	}

	switch e.Type {
	case EventChar:
		w.imeClear()
		charHandler(layout, e, w)

	case EventIMEComposition:
		imeCompositionHandler(layout, e, w)

	case EventFocused:
		w.focused = true

	case EventUnfocused:
		w.focused = false
		w.imeClear()

	case EventKeyDown:
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

	case EventKeyUp:
		keyupHandler(layout, e, w)

	case EventMouseDown:
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

	case EventMouseMove:
		w.setMouseCursor(CursorArrow)
		w.viewState.menuKeyNav = false
		w.viewState.mousePosX = e.MouseX
		w.viewState.mousePosY = e.MouseY
		mouseMoveHandler(layout, e, w)

	case EventMouseUp:
		mouseUpHandler(layout, e, w)

	case EventMouseScroll:
		mouseScrollHandler(layout, e, w)

	case EventResized:
		w.windowWidth = e.WindowWidth
		w.windowHeight = e.WindowHeight

	case EventFileDropped:
		fileDropHandler(layout, e, w)

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
