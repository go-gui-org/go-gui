package gui

import "testing"

// gestureLayout builds a layout tree suitable for gesture dispatch
// testing: a root shape covering 0,0-400,400 with one child.
func gestureLayout(eh *eventHandlers) *Layout {
	return &Layout{
		Shape: &Shape{
			Width: 400, Height: 400,
			shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 400},
		},
		Children: []Layout{{
			Shape: &Shape{
				Width: 400, Height: 400,
				shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 400},
				events:    eh,
			},
		}},
	}
}

// scrollLayout builds a scrollable container for pan-to-scroll
// tests.
func scrollLayout() *Layout {
	return &Layout{
		Shape: &Shape{
			Width: 400, Height: 400,
			shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 400},
		},
		Children: []Layout{{
			Shape: &Shape{
				IDScroll: 1,
				Width:    400, Height: 200,
				Axis: AxisTopToBottom,
				shapeClip: drawClip{
					X: 0, Y: 0, Width: 400, Height: 200,
				},
			},
			Children: []Layout{{
				Shape: &Shape{
					shapeType: shapeRectangle,
					Width:     400, Height: 1000,
				},
			}},
		}},
	}
}

func touchEvent(
	typ EventType, id uint64, x, y float32,
) *Event {
	return &Event{
		Type:       typ,
		NumTouches: 1,
		Touches: [8]TouchPoint{{
			Identifier: id,
			PosX:       x,
			PosY:       y,
			ToolType:   TouchToolFinger,
			Changed:    true,
		}},
	}
}

func twoTouchEvent(
	typ EventType,
	id0 uint64, x0, y0 float32,
	id1 uint64, x1, y1 float32,
) *Event {
	return &Event{
		Type:       typ,
		NumTouches: 2,
		Touches: [8]TouchPoint{
			{Identifier: id0, PosX: x0, PosY: y0,
				ToolType: TouchToolFinger, Changed: true},
			{Identifier: id1, PosX: x1, PosY: y1,
				ToolType: TouchToolFinger, Changed: true},
		},
	}
}

// fixedClock returns a nowFn that returns the given time.
func fixedClock(nanos int64) func() int64 {
	return func() int64 { return nanos }
}

// --- Tap ---

func TestGestureTap(t *testing.T) {
	t.Parallel()
	var got GestureType
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			got = e.GestureType
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.viewState.gesture.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))

	// End within tap timeout.
	w.viewState.gesture.nowFn = fixedClock(100_000_000) // 100ms
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 100, 100))

	if got != GestureTap {
		t.Errorf("expected GestureTap, got %d", got)
	}
}

// --- Double Tap ---

func TestGestureDoubleTap(t *testing.T) {
	t.Parallel()
	var got GestureType
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			got = e.GestureType
			e.IsHandled = true
		},
	})
	w := &Window{}
	gs := &w.viewState.gesture

	// First tap.
	gs.nowFn = fixedClock(0)
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	gs.nowFn = fixedClock(50_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 100, 100))

	if got != GestureTap {
		t.Fatalf("first tap: expected GestureTap, got %d", got)
	}

	// Second tap within gap and radius.
	gs.nowFn = fixedClock(200_000_000)
	w.handleTouch(root, touchEvent(EventTouchesBegan, 2, 102, 102))
	gs.nowFn = fixedClock(250_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 2, 102, 102))

	if got != GestureDoubleTap {
		t.Errorf("expected GestureDoubleTap, got %d", got)
	}
}

// --- Long Press ---

func TestGestureLongPress(t *testing.T) {
	t.Parallel()
	var got GestureType
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			got = e.GestureType
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))

	// Verify animation was registered.
	anim, ok := w.animations[gestureLongPressAnimID]
	if !ok {
		t.Fatal("long press animation not registered")
	}

	// Simulate animation firing by calling the callback directly.
	a := anim.(*Animate)
	w.layout = *root // set layout for callback dispatch
	a.Callback(a, w)

	if got != GestureLongPress {
		t.Errorf("expected GestureLongPress, got %d", got)
	}
	if !gs.recognized {
		t.Error("expected recognized = true")
	}
}

// --- Pan ---

func TestGesturePan(t *testing.T) {
	t.Parallel()
	var phases []GesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			if e.GestureType == GesturePan {
				phases = append(phases, e.GesturePhase)
			}
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))

	// Move beyond threshold.
	gs.nowFn = fixedClock(16_000_000) // 16ms
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 115, 100))

	// Continue moving.
	gs.nowFn = fixedClock(32_000_000) // 32ms
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 130, 100))

	// End.
	gs.nowFn = fixedClock(500_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 130, 100))

	if len(phases) < 2 {
		t.Fatalf("expected at least 2 pan phases, got %d", len(phases))
	}
	if phases[0] != GesturePhaseBegan {
		t.Errorf("first phase: expected Began, got %d", phases[0])
	}
	if phases[len(phases)-1] != GesturePhaseEnded {
		t.Errorf("last phase: expected Ended, got %d",
			phases[len(phases)-1])
	}
}

// --- Swipe ---

func TestGestureSwipe(t *testing.T) {
	t.Parallel()
	var got GestureType
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			got = e.GestureType
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	// Move fast — large displacement in few frames builds velocity.
	gs.nowFn = fixedClock(16_000_000) // 16ms
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 150, 100))
	gs.nowFn = fixedClock(32_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 200, 100))
	gs.nowFn = fixedClock(48_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 250, 100))
	gs.nowFn = fixedClock(64_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 300, 100))

	gs.nowFn = fixedClock(100_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 300, 100))

	if got != GestureSwipe {
		t.Errorf("expected GestureSwipe, got %d", got)
	}
}

// --- Pinch ---

func TestGesturePinch(t *testing.T) {
	t.Parallel()
	var gotScale float32
	var gotPhase GesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			if e.GestureType == GesturePinch {
				gotScale = e.PinchScale
				gotPhase = e.GesturePhase
			}
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	// First finger.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 200))
	// Second finger — triggers pinch init.
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 100, 200, 2, 200, 200))

	// Spread apart (increase span).
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, 80, 200, 2, 250, 200))

	if gotScale <= 1.0 {
		t.Errorf("expected scale > 1.0, got %f", gotScale)
	}
	_ = gotPhase
}

// --- Rotate ---

func TestGestureRotate(t *testing.T) {
	t.Parallel()
	var gotRotation float32
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			if e.GestureType == GestureRotate {
				gotRotation = e.GestureRotation
			}
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	// Two fingers horizontal.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 150, 200))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 150, 200, 2, 250, 200))

	// Rotate: move finger 2 upward (creates angle change).
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, 150, 200, 2, 250, 150))

	if gotRotation == 0 {
		t.Error("expected non-zero rotation")
	}
}

// --- Rotate end ---

func TestGestureRotateEnd(t *testing.T) {
	t.Parallel()
	var gotPhase GesturePhase
	var gotRotation float32
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			if e.GestureType == GestureRotate {
				gotPhase = e.GesturePhase
				gotRotation = e.GestureRotation
			}
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	// Two fingers horizontal, centered at (200,200), span=100.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 150, 200))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 150, 200, 2, 250, 200))

	// Rotate ~30 degrees keeping span constant (~100px).
	// Fingers orbit the centroid at radius 50.
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, 157, 175, 2, 243, 225))

	// Lift all fingers.
	w.handleTouch(root, twoTouchEvent(EventTouchesEnded,
		1, 157, 175, 2, 243, 225))

	if gotPhase != GesturePhaseEnded {
		t.Errorf("expected Ended phase, got %d", gotPhase)
	}
	if gotRotation == 0 {
		t.Error("expected non-zero rotation in Ended event")
	}
}

// --- Rotate to single touch transition ---

func TestRotateToSingleTouchTransition(t *testing.T) {
	t.Parallel()
	var lastType GestureType
	var lastPhase GesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			lastType = e.GestureType
			lastPhase = e.GesturePhase
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	// Two fingers centered at (200,200), span=100.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 150, 200))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 150, 200, 2, 250, 200))

	// Rotate ~30 degrees keeping span constant.
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, 157, 175, 2, 243, 225))

	// Lift second finger.
	endEvt := &Event{
		Type:       EventTouchesEnded,
		NumTouches: 1,
		Touches: [8]TouchPoint{{
			Identifier: 2,
			PosX:       243, PosY: 225,
			ToolType: TouchToolFinger,
			Changed:  true,
		}},
	}
	w.handleTouch(root, endEvt)

	// Should have emitted Rotate/Ended then Pan/Began.
	if lastType != GesturePan || lastPhase != GesturePhaseBegan {
		t.Errorf("expected Pan/Began after lift, got %d/%d",
			lastType, lastPhase)
	}
}

// --- Single touch mouse compat ---

func TestSingleTouchMouseCompat(t *testing.T) {
	t.Parallel()
	var clicked bool
	root := gestureLayout(&eventHandlers{
		OnClick: func(_ *Layout, e *Event, _ *Window) {
			clicked = true
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	gs.nowFn = fixedClock(100_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 100, 100))

	if !clicked {
		t.Error("expected OnClick from single-touch tap")
	}
}

// --- Pan fallback scroll ---

func TestPanFallbackScroll(t *testing.T) {
	root := scrollLayout()
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)
	guiTheme.ScrollMultiplier = 1

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	// Pan downward — first move crosses threshold (Began).
	gs.nowFn = fixedClock(16_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 100, 80))
	// Second move produces Changed with delta (triggers scroll).
	gs.nowFn = fixedClock(32_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 100, 60))

	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	v, _ := sy.Get(1) // IDScroll = 1
	if v == 0 {
		t.Error("expected scroll offset to change from pan")
	}
}

// --- Touch cancelled ---

func TestTouchCancelled(t *testing.T) {
	t.Parallel()
	var gotPhase GesturePhase
	var gotCancelled bool
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			gotPhase = e.GesturePhase
			if e.GesturePhase == GesturePhaseCancelled {
				gotCancelled = true
			}
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	// Start panning.
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 120, 100))
	// Cancel.
	w.handleTouch(root, touchEvent(EventTouchesCancelled, 1, 120, 100))

	if !gotCancelled {
		t.Errorf("expected cancelled phase, got %d", gotPhase)
	}
	if gs.numTouches != 0 {
		t.Errorf("expected reset, numTouches=%d", gs.numTouches)
	}
}

// --- Pinch to single touch transition ---

func TestPinchToSingleTouchTransition(t *testing.T) {
	t.Parallel()
	var lastType GestureType
	var lastPhase GesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(_ *Layout, e *Event, _ *Window) {
			lastType = e.GestureType
			lastPhase = e.GesturePhase
			e.IsHandled = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	// Two fingers.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 200))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 100, 200, 2, 200, 200))
	// Pinch.
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, 80, 200, 2, 250, 200))

	// Lift second finger — should end pinch.
	endEvt := &Event{
		Type:       EventTouchesEnded,
		NumTouches: 1,
		Touches: [8]TouchPoint{{
			Identifier: 2,
			PosX:       250, PosY: 200,
			ToolType: TouchToolFinger,
			Changed:  true,
		}},
	}
	w.handleTouch(root, endEvt)

	// Should have transitioned to pan.
	if lastType != GesturePan || lastPhase != GesturePhaseBegan {
		t.Errorf("expected Pan/Began after lift, got %d/%d",
			lastType, lastPhase)
	}
}
