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
					OnChar: func(ctx EventCtx) {
						called = true
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
		Shape: &Shape{shapeClip: drawClip{Width: 800, Height: 600}},
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
					OnChar: func(ctx EventCtx) {
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
					OnChar: func(ctx EventCtx) {
						mainCalled = true
					},
				},
			}},
			{Shape: &Shape{
				Focusable: true, ID: "f2",
				events: &eventHandlers{
					OnChar: func(ctx EventCtx) {
						dialogCalled = true
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
					OnMouseScroll: func(ctx EventCtx) {
						focusedCalled = true
						ctx.Consume()
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
	pinScrollMultiplier(w, 1)
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
					OnKeyUp: func(ctx EventCtx) {
						called = true
						ctx.Consume()
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
		OnKeyUp: func(ctx EventCtx) {
			called = true
			ctx.Consume()
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

// --- IME composition and file-drop routing ---

func TestEventFnRoutesIMEComposition(t *testing.T) {
	w := newEventTestWindow()
	w.layout = Layout{Shape: &Shape{}}

	e := &Event{
		Type:      EventIMEComposition,
		IMEText:   "かん",
		IMEStart:  0,
		IMELength: 1,
	}
	w.EventFn(e)

	if !w.IMEComposing() {
		t.Fatal("IME composition state should be set")
	}
	if w.IMECompText() != "かん" {
		t.Fatalf("IMECompText = %q, want かん", w.IMECompText())
	}
	if !e.IsHandled {
		t.Error("IME composition event should be marked handled")
	}
}

func TestEventFnIMECompositionEmptyClears(t *testing.T) {
	w := newEventTestWindow()
	w.layout = Layout{Shape: &Shape{}}
	w.imeUpdate(&Event{Type: EventIMEComposition, IMEText: "字"})
	if !w.IMEComposing() {
		t.Fatal("precondition: composing")
	}

	// An empty preedit commits/clears the composition.
	w.EventFn(&Event{Type: EventIMEComposition, IMEText: ""})
	if w.IMEComposing() {
		t.Fatal("empty composition event should clear the preedit")
	}
}

func TestHandleIMECompositionEventDirect(t *testing.T) {
	// The window-level wrapper updates IME state and marks the event
	// handled, matching imeCompositionHandler.
	w := newEventTestWindow()
	e := &Event{
		Type:      EventIMEComposition,
		IMEText:   "abc",
		IMEStart:  1,
		IMELength: 2,
	}
	w.handleIMECompositionEvent(&Layout{Shape: &Shape{}}, e)
	if w.IMECompText() != "abc" {
		t.Fatalf("IMECompText = %q, want abc", w.IMECompText())
	}
	if w.IMECompCursor() != 1 || w.IMECompSelLen() != 2 {
		t.Fatalf("clause = %d/%d, want 1/2",
			w.IMECompCursor(), w.IMECompSelLen())
	}
	if !e.IsHandled {
		t.Error("wrapper must mark the event handled")
	}
}

func TestEventFnRoutesFileDropped(t *testing.T) {
	w := newEventTestWindow()
	var gotPath string
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
				events: &eventHandlers{
					OnFileDrop: func(ctx EventCtx) {
						gotPath = ctx.Event.FilePath
						ctx.Consume()
					},
				},
			}},
		},
	}

	e := &Event{
		Type:     EventFileDropped,
		MouseX:   50,
		MouseY:   50,
		FilePath: "/tmp/dropped.txt",
	}
	w.EventFn(e)

	if gotPath != "/tmp/dropped.txt" {
		t.Fatalf("OnFileDrop received %q, want /tmp/dropped.txt", gotPath)
	}
	if !e.IsHandled {
		t.Error("file drop should be marked handled")
	}
}

func TestEventFnFileDroppedAllowedWhenUnfocused(t *testing.T) {
	// eventAllowed admits file drops to an unfocused window: drags
	// land before any focus transfer.
	w := newEventTestWindow()
	w.focused = false
	called := false
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
				events: &eventHandlers{
					OnFileDrop: func(ctx EventCtx) { called = true },
				},
			}},
		},
	}
	w.EventFn(&Event{
		Type:     EventFileDropped,
		MouseX:   50,
		MouseY:   50,
		FilePath: "/tmp/x.txt",
	})
	if !called {
		t.Error("file drop must reach the layout while unfocused")
	}
}

func TestHandleFileDroppedEventDirect(t *testing.T) {
	// The window-level wrapper routes to the topmost enabled child
	// under the pointer.
	w := newEventTestWindow()
	var hit string
	layout := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
				events: &eventHandlers{
					OnFileDrop: func(ctx EventCtx) { hit = "first" },
				},
			}},
			{Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
				events: &eventHandlers{
					OnFileDrop: func(ctx EventCtx) {
						hit = "second"
						ctx.Consume()
					},
				},
			}},
		},
	}
	e := &Event{MouseX: 75, MouseY: 75, FilePath: "/tmp/a.txt"}
	w.handleFileDroppedEvent(layout, e)
	if hit != "second" {
		t.Fatalf("hit = %q, want second (topmost under pointer)", hit)
	}
	if !e.IsHandled {
		t.Error("wrapper should propagate IsHandled")
	}
}

func TestEventFnFileDroppedFrozen(t *testing.T) {
	// Time-travel freeze gates every user event, including drops.
	w := newEventTestWindow()
	w.frozen.Store(true)
	called := false
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
				events: &eventHandlers{
					OnFileDrop: func(ctx EventCtx) { called = true },
				},
			}},
		},
	}
	w.EventFn(&Event{Type: EventFileDropped, MouseX: 50, MouseY: 50})
	if called {
		t.Error("file drop must be gated while frozen")
	}
}

func TestMouseDownBackgroundClearsFocus(t *testing.T) {
	t.Parallel()
	newFocusWindow := func() *Window {
		w := newEventTestWindow()
		w.layout = Layout{
			Shape: &Shape{shapeClip: drawClip{Width: 800, Height: 600}},
			Children: []Layout{
				{Shape: &Shape{
					Focusable: true, ID: "in",
					shapeClip: drawClip{Width: 100, Height: 100},
				}},
				{Shape: &Shape{
					Focusable: true, ID: "other",
					shapeClip: drawClip{X: 200, Width: 100, Height: 100},
				}},
				{Shape: &Shape{
					shapeClip: drawClip{X: 400, Width: 100, Height: 100},
					events: &eventHandlers{
						OnClick: func(ctx EventCtx) { ctx.Consume() },
					},
				}},
				// A focusable scroll container whose scrollbar child
				// consumes the press before the container's own focus
				// take runs (listbox, scrollable multiline input).
				{
					Shape: &Shape{
						Focusable: true, ID: "list",
						shapeClip: drawClip{Y: 200, Width: 100, Height: 100},
					},
					Children: []Layout{{Shape: &Shape{
						shapeClip: drawClip{X: 90, Y: 200, Width: 10, Height: 100},
						events: &eventHandlers{
							OnClick: func(ctx EventCtx) { ctx.Consume() },
						},
					}}},
				},
			},
		}
		return w
	}
	t.Run("background_clears", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("in")
		w.EventFn(&Event{
			Type: EventMouseDown, MouseButton: MouseLeft,
			MouseX: 700, MouseY: 500,
		})
		if got := w.FocusID(); got != "" {
			t.Errorf("background click: focus = %q, want empty", got)
		}
	})
	t.Run("same_input_keeps", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("in")
		w.EventFn(&Event{
			Type: EventMouseDown, MouseButton: MouseLeft,
			MouseX: 50, MouseY: 50,
		})
		if got := w.FocusID(); got != "in" {
			t.Errorf("input click: focus = %q, want in", got)
		}
	})
	t.Run("other_focusable_moves", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("in")
		w.EventFn(&Event{
			Type: EventMouseDown, MouseButton: MouseLeft,
			MouseX: 250, MouseY: 50,
		})
		if got := w.FocusID(); got != "other" {
			t.Errorf("second input click: focus = %q, want other", got)
		}
	})
	t.Run("right_click_preserves", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("in")
		w.EventFn(&Event{
			Type: EventMouseDown, MouseButton: MouseRight,
			MouseX: 700, MouseY: 500,
		})
		if got := w.FocusID(); got != "in" {
			t.Errorf("right background click: focus = %q, want in", got)
		}
	})
	t.Run("consumed_nonfocusable_keeps", func(t *testing.T) {
		t.Parallel()
		// A widget that consumes the press but cannot take focus (a
		// FocusDisabled button, a custom keypad key) leaves focus
		// where it is, like a macOS button that refuses first
		// responder (issue #770).
		w := newFocusWindow()
		w.SetFocus("in")
		w.EventFn(&Event{
			Type: EventMouseDown, MouseButton: MouseLeft,
			MouseX: 450, MouseY: 50,
		})
		if got := w.FocusID(); got != "in" {
			t.Errorf("consumed non-focusable click: focus = %q, want in", got)
		}
	})
	t.Run("touch_tap_consumed_nonfocusable_keeps", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("in")
		synthMouse(EventMouseDown, 450, 50, MouseLeft, &w.layout, w)
		if got := w.FocusID(); got != "in" {
			t.Errorf("consumed non-focusable tap: focus = %q, want in", got)
		}
	})
	t.Run("consumed_child_of_focused_keeps", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("list")
		w.EventFn(&Event{
			Type: EventMouseDown, MouseButton: MouseLeft,
			MouseX: 95, MouseY: 250,
		})
		if got := w.FocusID(); got != "list" {
			t.Errorf("scrollbar click: focus = %q, want list", got)
		}
	})
	t.Run("callback_reassert_keeps", func(t *testing.T) {
		t.Parallel()
		// A splitter or slider handle re-asserts the focus it already
		// holds from its press callback. The ID does not change, so only
		// the set count shows the press was claimed.
		w := newFocusWindow()
		w.layout.Children[2].Shape.events.OnClick = func(ctx EventCtx) {
			ctx.Window.SetFocus("in")
			ctx.Consume()
		}
		w.SetFocus("in")
		w.EventFn(&Event{
			Type: EventMouseDown, MouseButton: MouseLeft,
			MouseX: 450, MouseY: 50,
		})
		if got := w.FocusID(); got != "in" {
			t.Errorf("re-asserting click: focus = %q, want in", got)
		}
	})
	t.Run("nil_layout_no_panic", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("in")
		w.blurUnclaimedPress(nil, &Event{MouseButton: MouseLeft},
			w.viewState.focusSetCount)
		if got := w.FocusID(); got != "in" {
			t.Errorf("nil layout: focus = %q, want in", got)
		}
	})
	t.Run("touch_tap_background_clears", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("in")
		synthMouse(EventMouseDown, 700, 500, MouseLeft, &w.layout, w)
		if got := w.FocusID(); got != "" {
			t.Errorf("background tap: focus = %q, want empty", got)
		}
	})
	t.Run("touch_tap_consumed_child_of_focused_keeps", func(t *testing.T) {
		t.Parallel()
		w := newFocusWindow()
		w.SetFocus("list")
		synthMouse(EventMouseDown, 95, 250, MouseLeft, &w.layout, w)
		if got := w.FocusID(); got != "list" {
			t.Errorf("scrollbar tap: focus = %q, want list", got)
		}
	})
}
