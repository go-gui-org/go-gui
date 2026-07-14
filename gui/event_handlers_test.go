package gui

import "testing"

// focusedChild builds a layout with one focused child that has events.
func focusedChild(focusID string, eh *eventHandlers) *Layout {
	return &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true,
				ID:        focusID,
				events:    eh,
				shapeClip: drawClip{
					X: 0, Y: 0, Width: 100, Height: 100,
				},
			}},
		},
	}
}

func TestCharHandler(t *testing.T) {
	t.Parallel()
	t.Run("delivers", func(t *testing.T) {
		t.Parallel()
		called := false
		root := focusedChild("f1", &eventHandlers{
			OnChar: func(_ *Layout, e *Event, _ *Window) {
				called = true
				e.IsHandled = true
			},
		})
		w := &Window{}
		w.SetFocus("f1")
		e := &Event{CharCode: 'a'}
		charHandler(root, e, w)
		if !called {
			t.Error("OnChar not called")
		}
		if !e.IsHandled {
			t.Error("event not handled")
		}
	})
	t.Run("skips_disabled", func(t *testing.T) {
		t.Parallel()
		called := false
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				{Shape: &Shape{
					Focusable: true, ID: "f1",
					Disabled: true,
					events: &eventHandlers{
						OnChar: func(_ *Layout, e *Event, _ *Window) {
							called = true
							e.IsHandled = true
						},
					},
				}},
			},
		}
		w := &Window{}
		w.SetFocus("f1")
		e := &Event{CharCode: 'a'}
		charHandler(root, e, w)
		if called {
			t.Error("should skip disabled")
		}
	})
}

func TestKeydownHandlerDelivers(t *testing.T) {
	t.Parallel()
	called := false
	root := focusedChild("f1", &eventHandlers{
		OnKeyDown: func(_ *Layout, e *Event, _ *Window) {
			called = true
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.SetFocus("f1")
	e := &Event{KeyCode: KeyEnter}
	keydownHandler(root, e, w)
	if !called {
		t.Error("OnKeyDown not called")
	}
}

func TestKeyupHandlerDelivers(t *testing.T) {
	t.Parallel()
	called := false
	root := focusedChild("f1", &eventHandlers{
		OnKeyUp: func(_ *Layout, e *Event, _ *Window) {
			called = true
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.SetFocus("f1")
	e := &Event{KeyCode: KeyEnter}
	keyupHandler(root, e, w)
	if !called {
		t.Error("OnKeyUp not called")
	}
}

func TestKeyupHandler_NilLayoutNoPanic(t *testing.T) {
	t.Parallel()
	// Should not panic when layout is nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("keyupHandler panicked with nil layout: %v", r)
		}
	}()

	w := &Window{}
	e := &Event{KeyCode: KeyEnter}
	keyupHandler(nil, e, w)
}

func TestKeyupHandler_ExcessiveChildren(t *testing.T) {
	t.Parallel()
	// Should return early when children count exceeds limit
	called := false
	root := &Layout{
		Children: make([]Layout, 10001), // Exceeds the 10,000 limit
	}

	w := &Window{}
	e := &Event{KeyCode: KeyEnter}
	keyupHandler(root, e, w)

	// Handler should not be called due to early return
	if called {
		t.Error("Handler should not be called with excessive children")
	}
}

func TestKeydownHandlerFallbackScroll(t *testing.T) {
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{
				Shape: &Shape{
					Focusable: true, ID: "f1",
					IDScroll: 1,
					Width:    100,
					Height:   100,
					Axis:     AxisTopToBottom,
				},
				Children: []Layout{
					{Shape: &Shape{
						shapeType: shapeRectangle, Height: 500,
					}},
				},
			},
		},
	}
	w := &Window{}
	w.SetFocus("f1")
	guiTheme.ScrollDeltaLine = 20
	e := &Event{KeyCode: KeyDown, Modifiers: ModNone}
	keydownHandler(root, e, w)
	if !e.IsHandled {
		t.Error("scroll fallback should handle KeyDown")
	}
}

func TestKeyDownScrollHandlerArrows(t *testing.T) {
	guiTheme.ScrollDeltaLine = 20
	guiTheme.ScrollDeltaPage = 100
	guiTheme.ScrollMultiplier = 1

	layout := &Layout{
		Shape: &Shape{
			IDScroll: 1, Width: 100, Height: 100,
			Axis: AxisTopToBottom,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle,
				Width: 500, Height: 500}},
		},
	}
	w := &Window{}

	tests := []struct {
		name string
		key  KeyCode
		mod  Modifier
	}{
		{"down", KeyDown, ModNone},
		{"page_down", KeyPageDown, ModNone},
		{"end", KeyEnd, ModNone},
		{"up", KeyUp, ModNone},
		{"page_up", KeyPageUp, ModNone},
		{"home", KeyHome, ModNone},
		{"shift+right", KeyRight, ModShift},
		{"shift+left", KeyLeft, ModShift},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &Event{KeyCode: tc.key, Modifiers: tc.mod}
			keyDownScrollHandler(layout, e, w)
			if !e.IsHandled {
				t.Errorf("%s not handled", tc.name)
			}
		})
	}
}

func TestMouseDownHandler(t *testing.T) {
	t.Parallel()
	t.Run("delivers", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				{Shape: &Shape{
					shapeClip: drawClip{X: 0, Y: 0,
						Width: 100, Height: 100},
					events: &eventHandlers{
						OnClick: func(_ *Layout, e *Event, _ *Window) {
							clicked = true
							e.IsHandled = true
						},
					},
				}},
			},
		}
		w := &Window{windowWidth: 800, windowHeight: 600}
		e := &Event{MouseX: 50, MouseY: 50, Type: EventMouseDown}
		mouseDownHandler(root, false, e, w)
		if !clicked {
			t.Error("OnClick not called")
		}
	})
	t.Run("sets_focus", func(t *testing.T) {
		t.Parallel()
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				{Shape: &Shape{
					Focusable: true, ID: "f42",
					shapeClip: drawClip{X: 0, Y: 0,
						Width: 100, Height: 100},
				}},
			},
		}
		w := &Window{windowWidth: 800, windowHeight: 600}
		e := &Event{MouseX: 50, MouseY: 50}
		mouseDownHandler(root, false, e, w)
		if w.FocusID() != "f42" {
			t.Errorf("focus: got %q, want f42", w.FocusID())
		}
	})
	t.Run("respects_mouse_lock", func(t *testing.T) {
		t.Parallel()
		lockCalled := false
		w := &Window{windowWidth: 800, windowHeight: 600}
		w.MouseLock(MouseLockCfg{
			MouseDown: func(_ *Layout, e *Event, _ *Window) {
				lockCalled = true
				e.IsHandled = true
			},
		})
		root := &Layout{Shape: &Shape{}}
		e := &Event{MouseX: 50, MouseY: 50}
		mouseDownHandler(root, false, e, w)
		if !lockCalled {
			t.Error("mouse lock should intercept")
		}
	})
	t.Run("reverse_order", func(t *testing.T) {
		t.Parallel()
		var hitID string
		mkChild := func(id string, x float32) Layout {
			return Layout{Shape: &Shape{
				ID: id,
				shapeClip: drawClip{X: x, Y: 0,
					Width: 100, Height: 100},
				events: &eventHandlers{
					OnClick: func(l *Layout, e *Event, _ *Window) {
						hitID = l.Shape.ID
						e.IsHandled = true
					},
				},
			}}
		}
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				mkChild("first", 0),
				mkChild("second", 0),
			},
		}
		w := &Window{windowWidth: 800, windowHeight: 600}
		e := &Event{MouseX: 50, MouseY: 50}
		mouseDownHandler(root, false, e, w)
		if hitID != "second" {
			t.Errorf("hit: got %q, want second", hitID)
		}
	})
}

func TestMouseLockHandlers(t *testing.T) {
	t.Parallel()
	t.Run("move", func(t *testing.T) {
		t.Parallel()
		lockCalled := false
		w := &Window{windowWidth: 800, windowHeight: 600}
		w.MouseLock(MouseLockCfg{
			MouseMove: func(_ *Layout, _ *Event, _ *Window) {
				lockCalled = true
			},
		})
		root := &Layout{Shape: &Shape{}}
		e := &Event{MouseX: 50, MouseY: 50}
		mouseMoveHandler(root, e, w)
		if !lockCalled {
			t.Error("mouse lock should intercept move")
		}
	})
	t.Run("up", func(t *testing.T) {
		t.Parallel()
		lockCalled := false
		w := &Window{windowWidth: 800, windowHeight: 600}
		w.MouseLock(MouseLockCfg{
			MouseUp: func(_ *Layout, _ *Event, _ *Window) {
				lockCalled = true
			},
		})
		root := &Layout{Shape: &Shape{}}
		e := &Event{MouseX: 50, MouseY: 50}
		mouseUpHandler(root, e, w)
		if !lockCalled {
			t.Error("mouse lock should intercept up")
		}
	})
}

func TestMouseMoveHandlerSkipsOutOfWindow(t *testing.T) {
	t.Parallel()
	called := false
	root := &Layout{
		Shape: &Shape{
			shapeClip: drawClip{X: 0, Y: 0,
				Width: 100, Height: 100},
			events: &eventHandlers{
				OnMouseMove: func(_ *Layout, e *Event, _ *Window) {
					called = true
					e.IsHandled = true
				},
			},
		},
	}
	w := &Window{windowWidth: 800, windowHeight: 600}
	e := &Event{MouseX: -10, MouseY: 50}
	mouseMoveHandler(root, e, w)
	if called {
		t.Error("should skip out-of-window move")
	}
}

func TestMouseScrollHandlerVertical(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	root := &Layout{Shape: &Shape{
		IDScroll: 1,
		Width:    100,
		Height:   50,
		shapeClip: drawClip{X: 0, Y: 0,
			Width: 100, Height: 50},
	}, Children: []Layout{
		{Shape: &Shape{shapeType: shapeRectangle, Height: 200}},
	}}
	w := &Window{windowWidth: 800, windowHeight: 600}
	e := &Event{
		MouseX:    50,
		MouseY:    25,
		ScrollY:   -10,
		Modifiers: ModNone,
	}
	mouseScrollHandler(root, e, w)
	if !e.IsHandled {
		t.Error("vertical scroll not handled")
	}
}

func TestMouseScrollHandlerHorizontalShift(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	root := &Layout{Shape: &Shape{
		IDScroll: 1,
		Width:    50,
		Height:   100,
		shapeClip: drawClip{X: 0, Y: 0,
			Width: 50, Height: 100},
		Axis: AxisLeftToRight,
	}, Children: []Layout{
		{Shape: &Shape{shapeType: shapeRectangle, Width: 200}},
	}}
	w := &Window{windowWidth: 800, windowHeight: 600}
	e := &Event{
		MouseX:    25,
		MouseY:    50,
		ScrollX:   -10,
		Modifiers: ModShift,
	}
	mouseScrollHandler(root, e, w)
	if !e.IsHandled {
		t.Error("horizontal scroll not handled")
	}
}

func TestMouseScrollHandlerFocusedOnMouseScroll(t *testing.T) {
	t.Parallel()
	called := false
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f5",
				events: &eventHandlers{
					OnMouseScroll: func(_ *Layout, e *Event, _ *Window) {
						called = true
						e.IsHandled = true
					},
				},
			}},
		},
	}
	w := &Window{windowWidth: 800, windowHeight: 600}
	w.SetFocus("f5")
	e := &Event{MouseX: 50, MouseY: 50, ScrollY: -10}
	mouseScrollHandler(root, e, w)
	if !called {
		t.Error("focused OnMouseScroll should be called")
	}
}

func TestMouseScrollUnhandledCascadesToScrollContainer(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	// Focused handler does NOT set IsHandled — scroll should
	// cascade to the scroll container fallback.
	focusCalled := false
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{
				Focusable: true, ID: "f7",
				events: &eventHandlers{
					OnMouseScroll: func(_ *Layout, _ *Event, _ *Window) {
						focusCalled = true
						// deliberately not setting e.IsHandled
					},
				},
			}},
			{Shape: &Shape{
				IDScroll: 1,
				Width:    100,
				Height:   50,
				shapeClip: drawClip{
					X: 0, Y: 0, Width: 100, Height: 50,
				},
			}, Children: []Layout{
				{Shape: &Shape{shapeType: shapeRectangle, Height: 200}},
			}},
		},
	}
	w := &Window{windowWidth: 800, windowHeight: 600}
	w.SetFocus("f7")
	e := &Event{
		MouseX: 50, MouseY: 25,
		ScrollY: -10, Modifiers: ModNone,
	}
	mouseScrollHandler(root, e, w)
	if !focusCalled {
		t.Error("focused handler should still be called")
	}
	if !e.IsHandled {
		t.Error("scroll container should handle the event")
	}
}

func TestMouseScrollFallbackRespectsIsHandled(t *testing.T) {
	t.Parallel()
	// Layout callback sets IsHandled — should stop propagation
	// and NOT reach the scroll container.
	handlerCalled := false
	root := &Layout{
		Shape: &Shape{
			IDScroll: 1,
			Width:    200, Height: 100,
			shapeClip: drawClip{
				X: 0, Y: 0, Width: 200, Height: 100,
			},
		},
		Children: []Layout{
			{Shape: &Shape{
				Width: 200, Height: 100,
				shapeClip: drawClip{
					X: 0, Y: 0, Width: 200, Height: 100,
				},
				events: &eventHandlers{
					OnMouseScroll: func(_ *Layout, e *Event, _ *Window) {
						handlerCalled = true
						e.IsHandled = true
					},
				},
			}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 400}},
		},
	}
	w := &Window{windowWidth: 800, windowHeight: 600}
	e := &Event{
		MouseX: 50, MouseY: 25,
		ScrollY: -10, Modifiers: ModNone,
	}
	mouseScrollFallbackHandler(root, e, w)
	if !handlerCalled {
		t.Error("child handler should be called")
	}
	if !e.IsHandled {
		t.Error("event should be handled by child")
	}
}

func TestMouseScrollFallbackUnhandledReachesContainer(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	// Layout callback does NOT set IsHandled — scroll should
	// fall through to the parent scroll container.
	root := &Layout{
		Shape: &Shape{
			IDScroll: 1,
			Width:    200, Height: 100,
			shapeClip: drawClip{
				X: 0, Y: 0, Width: 200, Height: 100,
			},
		},
		Children: []Layout{
			{Shape: &Shape{
				Width: 200, Height: 100,
				shapeClip: drawClip{
					X: 0, Y: 0, Width: 200, Height: 100,
				},
				events: &eventHandlers{
					OnMouseScroll: func(_ *Layout, _ *Event, _ *Window) {
						// deliberately not setting e.IsHandled
					},
				},
			}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 400}},
		},
	}
	w := &Window{windowWidth: 800, windowHeight: 600}
	e := &Event{
		MouseX: 50, MouseY: 25,
		ScrollY: -10, Modifiers: ModNone,
	}
	mouseScrollFallbackHandler(root, e, w)
	if !e.IsHandled {
		t.Error("scroll container should handle unhandled event")
	}
}

func TestFileDropHandler(t *testing.T) {
	t.Parallel()

	t.Run("delivers_to_topmost_child", func(t *testing.T) {
		t.Parallel()
		var hitID string
		mkChild := func(id string) Layout {
			return Layout{Shape: &Shape{
				ID: id,
				shapeClip: drawClip{X: 0, Y: 0,
					Width: 100, Height: 100},
				events: &eventHandlers{
					OnFileDrop: func(l *Layout, e *Event, _ *Window) {
						hitID = l.Shape.ID
						e.IsHandled = true
					},
				},
			}}
		}
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				mkChild("first"),
				mkChild("second"),
			},
		}
		w := &Window{}
		e := &Event{MouseX: 50, MouseY: 50, FilePath: "/tmp/test.txt"}
		fileDropHandler(root, e, w)
		if hitID != "second" {
			t.Errorf("hit: got %q, want second", hitID)
		}
		if !e.IsHandled {
			t.Error("event not handled")
		}
	})

	t.Run("stops_propagation", func(t *testing.T) {
		t.Parallel()
		parentCalled := false
		root := &Layout{
			Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0,
					Width: 200, Height: 200},
				events: &eventHandlers{
					OnFileDrop: func(_ *Layout, _ *Event, _ *Window) {
						parentCalled = true
					},
				},
			},
			Children: []Layout{
				{Shape: &Shape{
					shapeClip: drawClip{X: 0, Y: 0,
						Width: 100, Height: 100},
					events: &eventHandlers{
						OnFileDrop: func(_ *Layout, e *Event, _ *Window) {
							e.IsHandled = true
						},
					},
				}},
			},
		}
		w := &Window{}
		e := &Event{MouseX: 50, MouseY: 50, FilePath: "/tmp/a.txt"}
		fileDropHandler(root, e, w)
		if parentCalled {
			t.Error("parent should not be called after child handles")
		}
	})

	t.Run("nil_shape_no_panic", func(t *testing.T) {
		t.Parallel()
		root := &Layout{
			Shape: nil,
			Children: []Layout{
				{Shape: nil},
			},
		}
		w := &Window{}
		e := &Event{MouseX: 50, MouseY: 50, FilePath: "/tmp/a.txt"}
		fileDropHandler(root, e, w)
		if e.IsHandled {
			t.Error("nil shape should not handle")
		}
	})

	t.Run("no_callback_passes_through", func(t *testing.T) {
		t.Parallel()
		root := &Layout{
			Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0,
					Width: 100, Height: 100},
			},
		}
		w := &Window{}
		e := &Event{MouseX: 50, MouseY: 50, FilePath: "/tmp/a.txt"}
		fileDropHandler(root, e, w)
		if e.IsHandled {
			t.Error("no callback should leave event unhandled")
		}
	})

	t.Run("skips_disabled_child", func(t *testing.T) {
		t.Parallel()
		called := false
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				{Shape: &Shape{
					Disabled: true,
					shapeClip: drawClip{X: 0, Y: 0,
						Width: 100, Height: 100},
					events: &eventHandlers{
						OnFileDrop: func(_ *Layout, e *Event, _ *Window) {
							called = true
							e.IsHandled = true
						},
					},
				}},
			},
		}
		w := &Window{}
		e := &Event{MouseX: 50, MouseY: 50, FilePath: "/tmp/a.txt"}
		fileDropHandler(root, e, w)
		if called {
			t.Error("should skip disabled child")
		}
	})

	t.Run("no_children_no_panic", func(t *testing.T) {
		t.Parallel()
		root := &Layout{
			Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0,
					Width: 100, Height: 100},
			},
		}
		w := &Window{}
		e := &Event{MouseX: 50, MouseY: 50, FilePath: "/tmp/a.txt"}
		fileDropHandler(root, e, w)
		// No panic is the assertion.
	})

	t.Run("restores_mouse_coords", func(t *testing.T) {
		t.Parallel()
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				{Shape: &Shape{
					shapeClip: drawClip{X: 10, Y: 20,
						Width: 100, Height: 100},
					events: &eventHandlers{
						OnFileDrop: func(_ *Layout, e *Event, _ *Window) {
							e.IsHandled = true
						},
					},
				}},
			},
		}
		w := &Window{}
		e := &Event{MouseX: 50, MouseY: 50, FilePath: "/tmp/a.txt"}
		fileDropHandler(root, e, w)
		if e.MouseX != 50 || e.MouseY != 50 {
			t.Errorf("coords not restored: got (%g, %g), want (50, 50)",
				e.MouseX, e.MouseY)
		}
	})
}

func TestMakeContainerEventsOnFileDropAlone(t *testing.T) {
	t.Parallel()
	called := false
	cfg := &ContainerCfg{
		OnFileDrop: func(_ *Layout, _ *Event, _ *Window) {
			called = true
		},
	}
	eh := makeContainerEvents(cfg)
	if eh == nil {
		t.Fatal("expected non-nil eventHandlers")
	}
	if eh.OnFileDrop == nil {
		t.Fatal("OnFileDrop not wired")
	}
	eh.OnFileDrop(nil, nil, nil)
	if !called {
		t.Error("OnFileDrop callback not invoked")
	}
}

func TestDrawCanvasOnFileDropWired(t *testing.T) {
	t.Parallel()
	called := false
	w := &Window{}
	v := DrawCanvas(DrawCanvasCfg{
		ID:    "dc-drop",
		Width: 100, Height: 100,
		OnFileDrop: func(_ *Layout, _ *Event, _ *Window) {
			called = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil {
		t.Fatal("expected non-nil events")
	}
	if layout.Shape.events.OnFileDrop == nil {
		t.Fatal("OnFileDrop not wired on DrawCanvas")
	}
	layout.Shape.events.OnFileDrop(nil, nil, nil)
	if !called {
		t.Error("OnFileDrop callback not invoked")
	}
}

func TestCharHandler_ClickOnSpace(t *testing.T) {
	t.Parallel()
	t.Run("fires", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := focusedChild("f1", &eventHandlers{
			ClickOnSpace: true,
			OnClick: func(_ *Layout, e *Event, _ *Window) {
				clicked = true
				e.IsHandled = true
			},
		})
		w := &Window{}
		w.SetFocus("f1")
		e := &Event{CharCode: CharSpace}
		charHandler(root, e, w)
		if !clicked {
			t.Error("ClickOnSpace should fire OnClick via charHandler")
		}
		if !e.IsHandled {
			t.Error("event should be handled")
		}
	})
	t.Run("ignores_non_space", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := focusedChild("f1", &eventHandlers{
			ClickOnSpace: true,
			OnClick: func(_ *Layout, e *Event, _ *Window) {
				clicked = true
				e.IsHandled = true
			},
		})
		w := &Window{}
		w.SetFocus("f1")
		e := &Event{CharCode: 'x'}
		charHandler(root, e, w)
		if clicked {
			t.Error("non-space char should not fire ClickOnSpace")
		}
	})
	t.Run("requires_focus", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := focusedChild("f1", &eventHandlers{
			ClickOnSpace: true,
			OnClick: func(_ *Layout, e *Event, _ *Window) {
				clicked = true
				e.IsHandled = true
			},
		})
		w := &Window{}
		// No SetIDFocus — widget is not focused.
		e := &Event{CharCode: CharSpace}
		charHandler(root, e, w)
		if clicked {
			t.Error("unfocused widget should not fire ClickOnSpace")
		}
	})
	t.Run("nil_onclick_no_panic", func(t *testing.T) {
		t.Parallel()
		root := focusedChild("f1", &eventHandlers{
			ClickOnSpace: true,
			OnClick:      nil,
		})
		w := &Window{}
		w.SetFocus("f1")
		e := &Event{CharCode: CharSpace}
		// Must not panic.
		charHandler(root, e, w)
		if e.IsHandled {
			t.Error("nil OnClick should leave event unhandled")
		}
	})
}

func TestKeydownHandler_ClickOnEnter(t *testing.T) {
	t.Parallel()
	t.Run("fires", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := focusedChild("f1", &eventHandlers{
			ClickOnEnter: true,
			OnClick: func(_ *Layout, e *Event, _ *Window) {
				clicked = true
				e.IsHandled = true
			},
		})
		w := &Window{}
		w.SetFocus("f1")
		e := &Event{KeyCode: KeyEnter}
		keydownHandler(root, e, w)
		if !clicked {
			t.Error("ClickOnEnter should fire OnClick via keydownHandler")
		}
		if !e.IsHandled {
			t.Error("event should be handled")
		}
	})
	t.Run("ignores_non_enter", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := focusedChild("f1", &eventHandlers{
			ClickOnEnter: true,
			OnClick: func(_ *Layout, e *Event, _ *Window) {
				clicked = true
				e.IsHandled = true
			},
		})
		w := &Window{}
		w.SetFocus("f1")
		e := &Event{KeyCode: KeyA}
		keydownHandler(root, e, w)
		if clicked {
			t.Error("non-Enter key should not fire ClickOnEnter")
		}
	})
	t.Run("requires_focus", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := focusedChild("f1", &eventHandlers{
			ClickOnEnter: true,
			OnClick: func(_ *Layout, e *Event, _ *Window) {
				clicked = true
				e.IsHandled = true
			},
		})
		w := &Window{}
		// No SetIDFocus — widget is not focused.
		e := &Event{KeyCode: KeyEnter}
		keydownHandler(root, e, w)
		if clicked {
			t.Error("unfocused widget should not fire ClickOnEnter")
		}
	})
	t.Run("nil_onclick_no_panic", func(t *testing.T) {
		t.Parallel()
		root := focusedChild("f1", &eventHandlers{
			ClickOnEnter: true,
			OnClick:      nil,
		})
		w := &Window{}
		w.SetFocus("f1")
		e := &Event{KeyCode: KeyEnter}
		// Must not panic.
		keydownHandler(root, e, w)
		if e.IsHandled {
			t.Error("nil OnClick should leave event unhandled")
		}
	})
}

func TestCharHandler_ExcessiveChildren(t *testing.T) {
	t.Parallel()
	root := &Layout{
		Children: make([]Layout, maxEventChildren+1),
	}
	w := &Window{}
	e := &Event{CharCode: 'a'}
	// Must not panic or hang.
	charHandler(root, e, w)
	if e.IsHandled {
		t.Error("excessive children should return early, unhandled")
	}
}

func TestKeydownHandler_ExcessiveChildren(t *testing.T) {
	t.Parallel()
	root := &Layout{
		Children: make([]Layout, maxEventChildren+1),
	}
	w := &Window{}
	e := &Event{KeyCode: KeyEnter}
	// Must not panic or hang.
	keydownHandler(root, e, w)
	if e.IsHandled {
		t.Error("excessive children should return early, unhandled")
	}
}

func TestMouseDownHandler_ClickButtonFilter(t *testing.T) {
	t.Parallel()
	t.Run("non_zero_allows_matching_button", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				{Shape: &Shape{
					shapeClip: drawClip{X: 0, Y: 0,
						Width: 100, Height: 100},
					events: &eventHandlers{
						OnClick: func(_ *Layout, e *Event, _ *Window) {
							clicked = true
							e.IsHandled = true
						},
						ClickButton: MouseRight,
					},
				}},
			},
		}
		w := &Window{windowWidth: 800, windowHeight: 600}
		e := &Event{MouseX: 50, MouseY: 50,
			MouseButton: MouseRight}
		mouseDownHandler(root, false, e, w)
		if !clicked {
			t.Error("right click should fire when ClickButton=MouseRight")
		}
	})
	t.Run("non_zero_blocks_wrong_button", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				{Shape: &Shape{
					shapeClip: drawClip{X: 0, Y: 0,
						Width: 100, Height: 100},
					events: &eventHandlers{
						OnClick: func(_ *Layout, e *Event, _ *Window) {
							clicked = true
							e.IsHandled = true
						},
						ClickButton: MouseRight,
					},
				}},
			},
		}
		w := &Window{windowWidth: 800, windowHeight: 600}
		e := &Event{MouseX: 50, MouseY: 50,
			MouseButton: MouseLeft}
		mouseDownHandler(root, false, e, w)
		if clicked {
			t.Error("left click should not fire when ClickButton=MouseRight")
		}
	})
	t.Run("zero_clickbutton_allows_any", func(t *testing.T) {
		t.Parallel()
		clicked := false
		root := &Layout{
			Shape: &Shape{},
			Children: []Layout{
				{Shape: &Shape{
					shapeClip: drawClip{X: 0, Y: 0,
						Width: 100, Height: 100},
					events: &eventHandlers{
						OnClick: func(_ *Layout, e *Event, _ *Window) {
							clicked = true
							e.IsHandled = true
						},
						ClickButton: 0,
					},
				}},
			},
		}
		w := &Window{windowWidth: 800, windowHeight: 600}
		e := &Event{MouseX: 50, MouseY: 50,
			MouseButton: MouseRight}
		mouseDownHandler(root, false, e, w)
		if !clicked {
			t.Error("ClickButton=0 should allow any mouse button")
		}
	})
}
