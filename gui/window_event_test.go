package gui

import "testing"

func newEventTestWindow() *Window {
	return &Window{
		focused:      true,
		windowWidth:  800,
		windowHeight: 600,
	}
}

func TestEventFnRoutesChar(t *testing.T) {
	w := newEventTestWindow()
	called := false
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f1",
				events: &eventHandlers{
					OnChar: func(_ *Layout, e *Event, _ *Window) {
						called = true
						e.IsHandled = true
					},
				},
			}},
		},
	}
	w.SetFocus("f1")
	e := &Event{Type: EventChar, CharCode: 'x'}
	w.EventFn(e)
	if !called {
		t.Error("char not routed")
	}
}

func TestEventFnTabCyclesFocus(t *testing.T) {
	w := newEventTestWindow()
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{Focusable: true, ID: "f10"}},
			{Shape: &Shape{Focusable: true, ID: "f20"}},
			{Shape: &Shape{Focusable: true, ID: "f30"}},
		},
	}
	w.SetFocus("f10")

	e := &Event{Type: EventKeyDown, KeyCode: KeyTab}
	w.EventFn(e)
	if w.FocusID() != "f20" {
		t.Errorf("tab: got %q, want f20", w.FocusID())
	}

	e = &Event{
		Type:      EventKeyDown,
		KeyCode:   KeyTab,
		Modifiers: ModShift,
	}
	w.EventFn(e)
	if w.FocusID() != "f10" {
		t.Errorf("shift+tab: got %q, want f10", w.FocusID())
	}
}

func TestEventFnMouseDownSetsFocus(t *testing.T) {
	w := newEventTestWindow()
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f7",
				shapeClip: drawClip{X: 0, Y: 0,
					Width: 100, Height: 100},
			}},
		},
	}
	e := &Event{
		Type:   EventMouseDown,
		MouseX: 50,
		MouseY: 50,
	}
	w.EventFn(e)
	if w.FocusID() != "f7" {
		t.Errorf("focus: got %q, want f7", w.FocusID())
	}
}

func TestEventFnClearsSelectOnUnhandledClick(t *testing.T) {
	w := newEventTestWindow()
	w.layout = Layout{Shape: &Shape{}}
	ss := StateMap[string, bool](w, nsSelect, capModerate)
	ss.Set("open", true)

	e := &Event{
		Type:   EventMouseDown,
		MouseX: 50,
		MouseY: 50,
	}
	w.EventFn(e)
	if ss.Len() != 0 {
		t.Error("select state should be cleared")
	}
}

func TestEventFnBlocksWhenUnfocused(t *testing.T) {
	w := newEventTestWindow()
	w.focused = false
	called := false
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f1",
				events: &eventHandlers{
					OnChar: func(_ *Layout, _ *Event, _ *Window) {
						called = true
					},
				},
			}},
		},
	}
	w.SetFocus("f1")

	// Char should be blocked.
	e := &Event{Type: EventChar, CharCode: 'a'}
	w.EventFn(e)
	if called {
		t.Error("char should be blocked when unfocused")
	}

	// Right-click should be allowed.
	e = &Event{
		Type:        EventMouseDown,
		MouseButton: MouseRight,
		MouseX:      50,
		MouseY:      50,
	}
	w.EventFn(e) // should not panic

	// Focused event should be allowed.
	e = &Event{Type: EventFocused}
	w.EventFn(e)
	if !w.focused {
		t.Error("focused event should set w.focused")
	}

	// Scroll should be allowed.
	w.focused = false
	e = &Event{Type: EventMouseScroll}
	w.EventFn(e) // should not panic
}

func TestEventFnFocusedUnfocused(t *testing.T) {
	w := newEventTestWindow()
	e := &Event{Type: EventUnfocused}
	w.EventFn(e)
	if w.focused {
		t.Error("should be unfocused")
	}
	e = &Event{Type: EventFocused}
	w.EventFn(e)
	if !w.focused {
		t.Error("should be focused")
	}
}

func TestEventFnResized(t *testing.T) {
	w := newEventTestWindow()
	e := &Event{
		Type:         EventResized,
		WindowWidth:  1024,
		WindowHeight: 768,
	}
	w.EventFn(e)
	if w.windowWidth != 1024 || w.windowHeight != 768 {
		t.Errorf("size: got %dx%d, want 1024x768",
			w.windowWidth, w.windowHeight)
	}
}

func TestEventFnDialogModalRouting(t *testing.T) {
	w := newEventTestWindow()
	mainCalled := false
	dialogCalled := false
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f1",
				events: &eventHandlers{
					OnChar: func(_ *Layout, e *Event, _ *Window) {
						mainCalled = true
						e.IsHandled = true
					},
				},
			}},
			{Shape: &Shape{
				Focusable: true, ID: "f2",
				events: &eventHandlers{
					OnChar: func(_ *Layout, e *Event, _ *Window) {
						dialogCalled = true
						e.IsHandled = true
					},
				},
			}},
		},
	}
	w.dialogCfg.visible = true
	w.SetFocus("f2")

	e := &Event{Type: EventChar, CharCode: 'a'}
	w.EventFn(e)
	if mainCalled {
		t.Error("main should not receive events in dialog mode")
	}
	if !dialogCalled {
		t.Error("dialog should receive events")
	}
}

func TestEventFnFiresOnEvent(t *testing.T) {
	w := newEventTestWindow()
	w.layout = Layout{Shape: &Shape{}}
	fired := false
	w.OnEvent = func(_ *Event, _ *Window) {
		fired = true
	}
	// Unhandled key_down.
	e := &Event{Type: EventKeyDown, KeyCode: KeyF5}
	w.EventFn(e)
	if !fired {
		t.Error("OnEvent should fire for unhandled events")
	}
}

func TestEventFnPreservesTooltipID(t *testing.T) {
	w := newEventTestWindow()
	w.layout = Layout{Shape: &Shape{}}
	w.viewState.tooltip.id = "tip1"
	e := &Event{Type: EventKeyDown, KeyCode: KeyA}
	w.EventFn(e)
	if w.viewState.tooltip.id != "tip1" {
		t.Error("tooltip ID should be preserved")
	}
}

func TestEventFnNilEventNoPanic(t *testing.T) {
	_ = t
	w := newEventTestWindow()
	w.layout = Layout{Shape: &Shape{}}
	w.EventFn(nil)
}

func TestEventFnStampsFrameCount(t *testing.T) {
	w := newEventTestWindow()
	w.layout = Layout{Shape: &Shape{}}

	// Before any FrameFn, frameCount is 0.
	e := &Event{Type: EventMouseMove}
	w.EventFn(e)
	if e.FrameCount != 0 {
		t.Errorf("before FrameFn: got %d, want 0", e.FrameCount)
	}

	// Advance two frames.
	w.FrameFn()
	w.FrameFn()
	e2 := &Event{Type: EventMouseDown, MouseX: 50, MouseY: 50}
	w.EventFn(e2)
	if e2.FrameCount != 2 {
		t.Errorf("after 2 FrameFn: got %d, want 2", e2.FrameCount)
	}
}

func TestEventFnMouseScrollFocusedHandlerPrecedence(t *testing.T) {
	w := newEventTestWindow()
	focusedCalled := false
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f11",
				events: &eventHandlers{
					OnMouseScroll: func(_ *Layout, e *Event, _ *Window) {
						focusedCalled = true
						e.IsHandled = true
					},
				},
			}},
			{Shape: &Shape{
				Scrollable: true,
				ID:         "1",
				Width:      100,
				Height:     50,
				shapeClip: drawClip{
					X: 0, Y: 0, Width: 100, Height: 50,
				},
			}, Children: []Layout{
				{Shape: &Shape{Height: 200}},
			}},
		},
	}
	w.SetFocus("f11")
	guiTheme.ScrollMultiplier = 1
	e := &Event{
		Type:      EventMouseScroll,
		MouseX:    25,
		MouseY:    20,
		ScrollY:   -10,
		Modifiers: ModNone,
	}
	w.EventFn(e)
	if !focusedCalled {
		t.Error("focused OnMouseScroll should be called")
	}
	if !e.IsHandled {
		t.Error("focused OnMouseScroll should mark event as handled")
	}
}

func TestEventFnRoutesKeyUp(t *testing.T) {
	w := newEventTestWindow()
	called := false
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f1",
				events: &eventHandlers{
					OnKeyUp: func(_ *Layout, e *Event, _ *Window) {
						called = true
						e.IsHandled = true
					},
				},
			}},
		},
	}
	w.SetFocus("f1")
	e := &Event{
		Type:    EventKeyUp,
		KeyCode: KeyEnter,
	}
	w.EventFn(e)
	if !called {
		t.Error("OnKeyUp should be called for EventKeyUp")
	}
	if !e.IsHandled {
		t.Error("OnKeyUp should mark event as handled")
	}
}

func TestKeyUpEventFlow_WindowToInput(t *testing.T) {
	// Integration test for key up event flow from Window.EventFn to input widget callback
	called := false
	w := newEventTestWindow()

	// Create an input widget with OnKeyUp handler
	input := Input(InputCfg{
		ID: "test-input",
		OnKeyUp: func(_ *Layout, e *Event, _ *Window) {
			called = true
			e.IsHandled = true
		},
	})

	// Set up window layout with the input
	w.layout = generateViewLayout(input, w)
	w.SetFocus("test-input")

	// Send key up event through window
	e := &Event{
		Type:    EventKeyUp,
		KeyCode: KeyEnter,
	}
	w.EventFn(e)

	// Verify the input's OnKeyUp handler was called
	if !called {
		t.Error("Input widget OnKeyUp should be called through window event flow")
	}
	if !e.IsHandled {
		t.Error("Event should be marked as handled by input widget")
	}
}
