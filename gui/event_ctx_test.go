package gui

import "testing"

// sizedWindow returns a window big enough that PointerOverApp accepts
// test coordinates; the zero Window reports a 0x0 viewport and mouse
// moves are dropped before dispatch.
func sizedWindow() *Window {
	w := &Window{}
	w.windowWidth, w.windowHeight = 200, 200
	return w
}

// clickableShape builds a root layout with one hit-testable child that
// carries the given handlers, positioned so a click at (5,5) lands on
// both the child and the root.
func clickableShape(eh *eventHandlers) *Layout {
	return &Layout{
		Shape: &Shape{
			shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
		},
		Children: []Layout{
			{Shape: &Shape{
				events:    eh,
				shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
			}},
		},
	}
}

// --- Nil-Event safety ---

func TestEventCtxNilEventMethods(t *testing.T) {
	t.Parallel()
	ctx := EventCtx{Layout: &Layout{}, Window: &Window{}}
	// All three must be callable from AmendLayout/OnScroll, which carry
	// no originating event.
	ctx.Consume()
	ctx.Bubble()
	if ctx.Handled() {
		t.Error("Handled() must be false with a nil Event")
	}
}

// --- Consume class ---

func TestConsumeClassHandledByDefault(t *testing.T) {
	t.Parallel()
	called := false
	root := clickableShape(&eventHandlers{
		OnClick: func(EventCtx) { called = true },
	})
	e := &Event{MouseX: 5, MouseY: 5}
	mouseDownHandler(root, false, e, &Window{})
	if !called {
		t.Fatal("OnClick not called")
	}
	if !e.IsHandled {
		t.Error("consume-class callback should not have to mark handled")
	}
}

func TestConsumeClassBubbleOptsOut(t *testing.T) {
	t.Parallel()
	root := clickableShape(&eventHandlers{
		OnClick: func(ctx EventCtx) { ctx.Bubble() },
	})
	e := &Event{MouseX: 5, MouseY: 5}
	mouseDownHandler(root, false, e, &Window{})
	if e.IsHandled {
		t.Error("ctx.Bubble() should restore propagation")
	}
}

// TestBubbleDoesNotUnhandleEarlierConsumer pins the documented limit of
// Bubble: it opts out of this callback's auto-consume only, because
// callRelative re-applies the incoming handled flag.
func TestBubbleDoesNotUnhandleEarlierConsumer(t *testing.T) {
	t.Parallel()
	layout := &Layout{Shape: &Shape{
		shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
	}}
	e := &Event{MouseX: 5, MouseY: 5, IsHandled: true}
	callRelative(layout, e, &Window{},
		func(ctx EventCtx) { ctx.Bubble() }, evConsume)
	if !e.IsHandled {
		t.Error("Bubble must not un-handle an event a prior handler consumed")
	}
}

// --- Notify class ---

func TestNotifyClassStillBubbles(t *testing.T) {
	t.Parallel()
	called := false
	root := focusedChild("f1", &eventHandlers{
		OnKeyDown: func(EventCtx) { called = true },
	})
	w := &Window{}
	w.SetFocus("f1")
	e := &Event{KeyCode: KeyA}
	keydownHandler(root, e, w)
	if !called {
		t.Fatal("OnKeyDown not called")
	}
	if e.IsHandled {
		t.Error("notify-class callback must not auto-consume")
	}
}

func TestNotifyClassConsumeStops(t *testing.T) {
	t.Parallel()
	root := focusedChild("f1", &eventHandlers{
		OnKeyDown: func(ctx EventCtx) { ctx.Consume() },
	})
	w := &Window{}
	w.SetFocus("f1")
	e := &Event{KeyCode: KeyA}
	keydownHandler(root, e, w)
	if !e.IsHandled {
		t.Error("ctx.Consume() should stop propagation")
	}
}

// TestKeyDownLeavesTabForTraversal is the reason OnKeyDown is notify:
// a widget that handles only Enter must leave Tab to the traversal
// handler.
func TestKeyDownLeavesTabForTraversal(t *testing.T) {
	t.Parallel()
	root := focusedChild("f1", &eventHandlers{
		OnKeyDown: func(ctx EventCtx) {
			if ctx.Event.KeyCode == KeyEnter {
				ctx.Consume()
			}
		},
	})
	w := &Window{}
	w.SetFocus("f1")
	e := &Event{KeyCode: KeyTab}
	keydownHandler(root, e, w)
	if e.IsHandled {
		t.Error("Tab must stay unhandled so traversal can claim it")
	}
}

// --- Class parameter reaches every shared helper correctly ---

func TestMouseMoveDoesNotAutoConsume(t *testing.T) {
	t.Parallel()
	root := clickableShape(&eventHandlers{
		OnMouseMove: func(EventCtx) {},
	})
	e := &Event{MouseX: 5, MouseY: 5}
	mouseMoveHandler(root, e, sizedWindow())
	if e.IsHandled {
		t.Error("OnMouseMove must not auto-consume (hover-on-container)")
	}
}

func TestFocusedMouseScrollDoesNotAutoConsume(t *testing.T) {
	t.Parallel()
	// callRelative is shared with the consume-class mouse path, so the
	// scroll side has to be asserted on its own.
	layout := &Layout{Shape: &Shape{
		shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
	}}
	e := &Event{MouseX: 5, MouseY: 5, ScrollY: 3}
	handled := callRelative(layout, e, &Window{},
		func(EventCtx) {}, evNotify)
	if handled || e.IsHandled {
		t.Error("OnMouseScroll must stay unhandled so scrolling cascades")
	}
}

func TestOnCharAutoConsumesButKeyUpDoesNot(t *testing.T) {
	t.Parallel()
	// Both go through executeFocusCallback; only OnChar is consume.
	charRoot := focusedChild("f1", &eventHandlers{
		OnChar: func(EventCtx) {},
	})
	w := &Window{}
	w.SetFocus("f1")
	ce := &Event{CharCode: 'a'}
	charHandler(charRoot, ce, w)
	if !ce.IsHandled {
		t.Error("OnChar should auto-consume")
	}

	upRoot := focusedChild("f1", &eventHandlers{
		OnKeyUp: func(EventCtx) {},
	})
	ue := &Event{KeyCode: KeyA}
	keyupHandler(upRoot, ue, w)
	if ue.IsHandled {
		t.Error("OnKeyUp must not auto-consume")
	}
}

// --- Keyboard activation ---

func TestClickOnSpaceActivatesOnceAndConsumes(t *testing.T) {
	t.Parallel()
	n := 0
	root := focusedChild("f1", &eventHandlers{
		ClickOnSpace: true,
		OnClick:      func(EventCtx) { n++ },
	})
	w := &Window{}
	w.SetFocus("f1")
	e := &Event{CharCode: CharSpace}
	charHandler(root, e, w)
	if n != 1 {
		t.Errorf("OnClick fired %d times, want 1", n)
	}
	if !e.IsHandled {
		t.Error("spacebar activation should consume")
	}
}

func TestClickOnEnterActivatesOnceAndConsumes(t *testing.T) {
	t.Parallel()
	n := 0
	root := focusedChild("f1", &eventHandlers{
		ClickOnEnter: true,
		OnClick:      func(EventCtx) { n++ },
	})
	w := &Window{}
	w.SetFocus("f1")
	e := &Event{KeyCode: KeyEnter}
	keydownHandler(root, e, w)
	if n != 1 {
		t.Errorf("OnClick fired %d times, want 1", n)
	}
	if !e.IsHandled {
		t.Error("Enter activation should consume")
	}
}

// --- Nested move notifications ---

// TestNestedMouseMoveFiresOnChildAndAncestor is the hover-on-container
// pattern: nested shapes both want the notification, which only works
// because OnMouseMove is notify-class. (OnHover itself is topmost-only
// by design in layoutHover, independently of the consume/notify split.)
func TestNestedMouseMoveFiresOnChildAndAncestor(t *testing.T) {
	t.Parallel()
	child, parent := false, false
	root := &Layout{
		Shape: &Shape{
			ID:        "outer",
			events:    &eventHandlers{OnMouseMove: func(EventCtx) { parent = true }},
			shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
		},
		Children: []Layout{
			{Shape: &Shape{
				ID:        "inner",
				events:    &eventHandlers{OnMouseMove: func(EventCtx) { child = true }},
				shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
			}},
		},
	}
	e := &Event{MouseX: 5, MouseY: 5}
	mouseMoveHandler(root, e, sizedWindow())
	if !child || !parent {
		t.Errorf("move child=%v parent=%v, want both true", child, parent)
	}
}

// --- Window.OnEvent reachability ---

// TestOnEventSkippedForConsumedClick records the visible behaviour
// change: OnEvent is the last-resort hook and only fires on unhandled
// events, so a click any widget handles no longer reaches it.
func TestOnEventSkippedForConsumedClick(t *testing.T) {
	t.Parallel()
	sniff := func(root *Layout) int {
		n := 0
		w := sizedWindow()
		w.focused = true // EventFn drops mouse-down on an unfocused window
		w.OnEvent = func(*Event, *Window) { n++ }
		w.layout = *root
		w.EventFn(&Event{
			Type: EventMouseDown, MouseX: 5, MouseY: 5,
			MouseButton: MouseLeft,
		})
		return n
	}
	if got := sniff(clickableShape(&eventHandlers{
		OnClick: func(EventCtx) {},
	})); got != 0 {
		t.Errorf("consumed click reached OnEvent %d times, want 0", got)
	}
	if got := sniff(clickableShape(&eventHandlers{})); got != 1 {
		t.Errorf("unhandled click reached OnEvent %d times, want 1", got)
	}
}

// --- Reentrancy ---

// TestReentrantDispatchKeepsOuterCtxUsable asserts the strong property:
// after a nested synchronous dispatch, the outer frame's Event is the
// same pointer with the same shape-relative coordinates, and consuming
// from the outer frame still takes effect.
func TestReentrantDispatchKeepsOuterCtxUsable(t *testing.T) {
	t.Parallel()
	inner := &Layout{Shape: &Shape{
		X: 40, Y: 40,
		shapeClip: drawClip{X: 40, Y: 40, Width: 20, Height: 20},
	}}
	outer := &Layout{Shape: &Shape{
		X: 10, Y: 10,
		shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
	}}
	w := &Window{}
	e := &Event{MouseX: 50, MouseY: 50}

	var samePtr, sameCoords bool
	callRelative(outer, e, w, func(ctx EventCtx) {
		x, y := ctx.Event.MouseX, ctx.Event.MouseY
		// Nested synchronous dispatch, as an OnClick that scrolls does.
		callRelative(inner, ctx.Event, w, func(EventCtx) {}, evNotify)
		samePtr = ctx.Event == e
		sameCoords = ctx.Event.MouseX == x && ctx.Event.MouseY == y
		ctx.Bubble()
		ctx.Consume()
	}, evConsume)

	if !samePtr {
		t.Error("outer ctx.Event pointer changed across nested dispatch")
	}
	if !sameCoords {
		t.Error("outer coordinates not restored after nested dispatch")
	}
	if !e.IsHandled {
		t.Error("outer ctx.Consume() had no effect after nested dispatch")
	}
}
