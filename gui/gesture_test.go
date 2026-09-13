package gui

import (
	"math"
	"testing"
)

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
				Scrollable: true,
				ID:         "1",
				Width:      400, Height: 200,
				Axis: axisTopToBottom,
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
		OnGesture: func(ctx EventCtx) {
			got = ctx.Event.GestureType
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
		OnGesture: func(ctx EventCtx) {
			got = ctx.Event.GestureType
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
		OnGesture: func(ctx EventCtx) {
			got = ctx.Event.GestureType
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
	var phases []gesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			if ctx.Event.GestureType == GesturePan {
				phases = append(phases, ctx.Event.GesturePhase)
			}
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
	if phases[0] != gesturePhaseBegan {
		t.Errorf("first phase: expected Began, got %d", phases[0])
	}
	if phases[len(phases)-1] != gesturePhaseEnded {
		t.Errorf("last phase: expected Ended, got %d",
			phases[len(phases)-1])
	}
}

// --- Swipe ---

func TestGestureSwipe(t *testing.T) {
	t.Parallel()
	var got GestureType
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			got = ctx.Event.GestureType
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
	var gotPhase gesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			if ctx.Event.GestureType == GesturePinch {
				gotScale = ctx.Event.PinchScale
				gotPhase = ctx.Event.GesturePhase
			}
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
		OnGesture: func(ctx EventCtx) {
			if ctx.Event.GestureType == GestureRotate {
				gotRotation = ctx.Event.GestureRotation
			}
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
	var gotPhase gesturePhase
	var gotRotation float32
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			if ctx.Event.GestureType == GestureRotate {
				gotPhase = ctx.Event.GesturePhase
				gotRotation = ctx.Event.GestureRotation
			}
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

	if gotPhase != gesturePhaseEnded {
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
	var lastPhase gesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			lastType = ctx.Event.GestureType
			lastPhase = ctx.Event.GesturePhase
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
	if lastType != GesturePan || lastPhase != gesturePhaseBegan {
		t.Errorf("expected Pan/Began after lift, got %d/%d",
			lastType, lastPhase)
	}
}

// --- Single touch mouse compat ---

func TestSingleTouchMouseCompat(t *testing.T) {
	t.Parallel()
	var clicked bool
	root := gestureLayout(&eventHandlers{
		OnClick: func(ctx EventCtx) {
			clicked = true
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
	pinScrollMultiplier(w, 1)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	// Pan downward — first move crosses threshold (Began).
	gs.nowFn = fixedClock(16_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 100, 80))
	// Second move produces Changed with delta (triggers scroll).
	gs.nowFn = fixedClock(32_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 100, 60))

	sy := w.scrollY()
	v, _ := sy.Get("1") // scroll ID = "1"
	if v == 0 {
		t.Error("expected scroll offset to change from pan")
	}
}

// --- Touch cancelled ---

func TestTouchCancelled(t *testing.T) {
	t.Parallel()
	var gotPhase gesturePhase
	var gotCancelled bool
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			gotPhase = ctx.Event.GesturePhase
			if ctx.Event.GesturePhase == gesturePhaseCancelled {
				gotCancelled = true
			}
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
	var lastPhase gesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			lastType = ctx.Event.GestureType
			lastPhase = ctx.Event.GesturePhase
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
	if lastType != GesturePan || lastPhase != gesturePhaseBegan {
		t.Errorf("expected Pan/Began after lift, got %d/%d",
			lastType, lastPhase)
	}
}

// --- Pinch from co-located fingers ---

func TestGesturePinchFromCoLocatedFingers(t *testing.T) {
	t.Parallel()
	var gotScale float32 = 1
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			if ctx.Event.GestureType == GesturePinch {
				gotScale = ctx.Event.PinchScale
			}
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	// Two fingers land on the same point: span baseline is 0.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 200))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 100, 200, 2, 100, 200))

	// Spread apart.
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, 80, 200, 2, 120, 200))
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, 60, 200, 2, 140, 200))

	if math.IsInf(float64(gotScale), 0) ||
		math.IsNaN(float64(gotScale)) {
		t.Fatalf("pinch scale is non-finite: %f", gotScale)
	}
	if gotScale <= 1.0 {
		t.Errorf("expected scale > 1.0 after spread, got %f",
			gotScale)
	}
}

// --- Held pan does not swipe ---

func TestGestureHeldPanDoesNotSwipe(t *testing.T) {
	t.Parallel()
	var got GestureType
	var gotPhase gesturePhase
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			got = ctx.Event.GestureType
			gotPhase = ctx.Event.GesturePhase
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	// Fast moves build a high EMA velocity ...
	gs.nowFn = fixedClock(16_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 150, 100))
	gs.nowFn = fixedClock(32_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 200, 100))
	// ... then the finger rests before lifting: the velocity is
	// stale, so the lift must end the pan, not fling a swipe.
	gs.nowFn = fixedClock(500_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 200, 100))

	if got != GesturePan || gotPhase != gesturePhaseEnded {
		t.Errorf("expected Pan/Ended after hold, got %d/%d",
			got, gotPhase)
	}
}

// --- Simultaneous pinch and rotate ---

func TestGestureSimultaneousPinchRotateEnd(t *testing.T) {
	t.Parallel()
	type observed struct {
		typ   GestureType
		phase gesturePhase
	}
	var events []observed
	var rotateBegans int
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			events = append(events, observed{
				ctx.Event.GestureType,
				ctx.Event.GesturePhase,
			})
			if ctx.Event.GestureType == GestureRotate &&
				ctx.Event.GesturePhase == gesturePhaseBegan {
				rotateBegans++
			}
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 150, 200))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 150, 200, 2, 250, 200))

	// Spread and twist in one move.
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, 130, 170, 2, 270, 230))

	if rotateBegans != 1 {
		t.Errorf("one move emitted %d Rotate/Began, want 1",
			rotateBegans)
	}

	// Lift all fingers: both gestures must end.
	events = nil
	w.handleTouch(root, twoTouchEvent(EventTouchesEnded,
		1, 130, 170, 2, 270, 230))

	var pinchEnded, rotateEnded bool
	for _, ev := range events {
		if ev.typ == GesturePinch &&
			ev.phase == gesturePhaseEnded {
			pinchEnded = true
		}
		if ev.typ == GestureRotate &&
			ev.phase == gesturePhaseEnded {
			rotateEnded = true
		}
	}
	if !pinchEnded {
		t.Error("expected Pinch/Ended after lift")
	}
	if !rotateEnded {
		t.Error("expected Rotate/Ended after lift")
	}
}

// --- Mouse up at the release point ---

func TestGestureMouseUpAtReleasePoint(t *testing.T) {
	t.Parallel()
	var ux, uy float32
	var ups int
	root := gestureLayout(&eventHandlers{
		OnMouseUp: func(ctx EventCtx) {
			ups++
			ux, uy = ctx.Event.MouseX, ctx.Event.MouseY
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	gs.nowFn = fixedClock(16_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 130, 100))
	gs.nowFn = fixedClock(32_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 130, 100))

	if ups != 1 {
		t.Fatalf("expected 1 mouse up, got %d", ups)
	}
	if ux != 130 || uy != 100 {
		t.Errorf("mouse up at (%f,%f), want release (130,100)",
			ux, uy)
	}
}

// --- Pan over a full container reaches ancestors ---

func TestPanFallbackReachesAncestorAtScrollLimit(t *testing.T) {
	t.Parallel()
	root := &Layout{
		Shape: &Shape{
			Width: 400, Height: 400,
			shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 400},
		},
		Children: []Layout{{
			Shape: &Shape{
				Scrollable: true,
				ID:         "fits",
				Width:      400, Height: 200,
				Axis: axisTopToBottom,
				shapeClip: drawClip{
					X: 0, Y: 0, Width: 400, Height: 200,
				},
			},
			// Content fits: no scroll room, so the
			// fallback must decline and let the pan
			// travel on to the root below.
			Children: []Layout{{
				Shape: &Shape{
					shapeType: shapeRectangle,
					Width:     400, Height: 100,
				},
			}},
		}},
	}
	var rootPhases []gesturePhase
	root.Shape.events = &eventHandlers{
		OnGesture: func(ctx EventCtx) {
			if ctx.Event.GestureType == GesturePan {
				rootPhases = append(rootPhases,
					ctx.Event.GesturePhase)
			}
		},
	}
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)
	pinScrollMultiplier(w, 1)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	gs.nowFn = fixedClock(16_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 100, 80))
	gs.nowFn = fixedClock(32_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 100, 60))

	var changed bool
	for _, ph := range rootPhases {
		if ph == GesturePhaseChanged {
			changed = true
		}
	}
	if !changed {
		t.Error("pan Changed never reached the ancestor " +
			"past a container with no scroll room")
	}
}

// --- Malformed touch input ---

func TestHandleTouchMalformedInput(t *testing.T) {
	t.Parallel()
	root := gestureLayout(nil)
	w := &Window{}

	// Must not panic.
	w.handleTouch(root, nil)
	w.handleTouch(nil, touchEvent(EventTouchesBegan, 1, 0, 0))
	gestureHandler(nil, &Event{}, &Window{})
	bad := touchEvent(EventTouchesBegan, 1, 0, 0)
	bad.NumTouches = -1
	w.handleTouch(root, bad)
	if w.viewState.gesture.numTouches != 0 {
		t.Errorf("negative count tracked %d touches, want 0",
			w.viewState.gesture.numTouches)
	}
}

// --- Touch tracking order and overflow ---

func TestRemoveTrackedTouchPreservesOrder(t *testing.T) {
	t.Parallel()
	gs := &gestureState{}
	addTrackedTouch(gs, TouchPoint{Identifier: 1, PosX: 10})
	addTrackedTouch(gs, TouchPoint{Identifier: 2, PosX: 20})
	addTrackedTouch(gs, TouchPoint{Identifier: 3, PosX: 30})

	removeTrackedTouch(gs, 1)

	if gs.numTouches != 2 {
		t.Fatalf("numTouches = %d, want 2", gs.numTouches)
	}
	if gs.touches[0].id != 2 || gs.touches[1].id != 3 {
		t.Errorf("order = [%d %d], want [2 3]",
			gs.touches[0].id, gs.touches[1].id)
	}
}

func TestTrackedTouchOverflowBalances(t *testing.T) {
	t.Parallel()
	gs := &gestureState{}
	for id := uint64(1); id <= 8; id++ {
		addTrackedTouch(gs, TouchPoint{Identifier: id})
	}
	// Ninth distinct touch while full: dropped, counted.
	addTrackedTouch(gs, TouchPoint{Identifier: 9})
	if gs.numTouches != 8 || gs.overflowTouches != 1 {
		t.Fatalf("tracked = %d/%d, want 8/1",
			gs.numTouches, gs.overflowTouches)
	}
	// Its release balances the count instead of wedging.
	removeTrackedTouch(gs, 9)
	if gs.overflowTouches != 0 {
		t.Errorf("overflow = %d, want 0", gs.overflowTouches)
	}
}

// --- Remaining finger reseeds after lift ---

func TestRemainingFingerReseedsAfterLift(t *testing.T) {
	t.Parallel()
	var gotX, gotY float32
	var got GestureType
	var clicked bool
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			got = ctx.Event.GestureType
			gotX, gotY = ctx.Event.CentroidX, ctx.Event.CentroidY
		},
		OnClick: func(ctx EventCtx) {
			clicked = true
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	// Finger A down, finger B down without moving, A lifts:
	// B becomes a fresh press at its own position.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 100, 100, 2, 200, 200))
	liftA := &Event{
		Type:       EventTouchesEnded,
		NumTouches: 1,
		Touches: [8]TouchPoint{{
			Identifier: 1, PosX: 100, PosY: 100,
			ToolType: TouchToolFinger, Changed: true,
		}},
	}
	gs.nowFn = fixedClock(50_000_000)
	w.handleTouch(root, liftA)

	gs.nowFn = fixedClock(100_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 2, 200, 200))

	if got != GestureTap {
		t.Errorf("expected GestureTap, got %d", got)
	}
	if gotX != 200 || gotY != 200 {
		t.Errorf("tap at (%f,%f), want B's (200,200)",
			gotX, gotY)
	}
	if !clicked {
		t.Error("expected OnClick from the remaining finger")
	}
}

// --- Pan ends when the original finger lifts ---

func TestPanEndsWhenOriginalFingerLifts(t *testing.T) {
	t.Parallel()
	type observed struct {
		typ   GestureType
		phase gesturePhase
	}
	var events []observed
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			events = append(events, observed{
				ctx.Event.GestureType,
				ctx.Event.GesturePhase,
			})
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)

	// Finger A pans, B taps without moving, A lifts: the pan
	// must end and B must reseed as a fresh press.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	gs.nowFn = fixedClock(16_000_000)
	w.handleTouch(root, touchEvent(EventTouchesMoved, 1, 130, 100))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, 130, 100, 2, 200, 200))
	liftA := &Event{
		Type:       EventTouchesEnded,
		NumTouches: 1,
		Touches: [8]TouchPoint{{
			Identifier: 1, PosX: 130, PosY: 100,
			ToolType: TouchToolFinger, Changed: true,
		}},
	}
	gs.nowFn = fixedClock(32_000_000)
	w.handleTouch(root, liftA)

	var panEnded, panBeganAfter bool
	for _, ev := range events {
		if ev.typ == GesturePan &&
			ev.phase == gesturePhaseEnded {
			panEnded = true
		}
		if panEnded && ev.typ == GesturePan &&
			ev.phase == gesturePhaseBegan {
			panBeganAfter = true
		}
	}
	if !panEnded {
		t.Error("expected Pan/Ended when the panning finger lifts")
	}
	if panBeganAfter {
		t.Error("reseeded finger must not resume the old pan")
	}

	// B lifts promptly: a tap at B's position.
	events = nil
	gs.nowFn = fixedClock(60_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 2, 200, 200))
	if len(events) != 1 || events[0].typ != GestureTap {
		t.Errorf("expected one tap, got %v", events)
	}
}

// --- Gesture dispatch depth cap ---

// gestureDeepChain builds a single-child chain n levels deep
// with a sized OnGesture leaf, so dispatch can reach it.
func gestureDeepChain(n int, eh *eventHandlers) *Layout {
	root := &Layout{Shape: &Shape{
		Width: 400, Height: 400,
		shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 400},
	}}
	cur := root
	for range n {
		cur.Children = []Layout{{Shape: &Shape{
			Width: 400, Height: 400,
			shapeClip: drawClip{
				X: 0, Y: 0, Width: 400, Height: 400,
			},
		}}}
		cur = &cur.Children[0]
	}
	cur.Shape.events = eh
	return root
}

func TestGestureHandlerDeepChainTerminates(t *testing.T) {
	t.Parallel()
	root := gestureDeepChain(maxEventDepth+50, &eventHandlers{
		OnGesture: func(ctx EventCtx) {
			ctx.Consume()
		},
	})
	e := &Event{CentroidX: 100, CentroidY: 100}
	gestureHandler(root, e, newTestWindow())
	// Past the budget the leaf is never reached: the event
	// travels on unhandled instead of recursing without bound.
	if e.IsHandled {
		t.Error("gestureHandler reached past maxEventDepth")
	}
}

func TestGestureHandlerReachesOrdinaryDepth(t *testing.T) {
	t.Parallel()
	root := gestureDeepChain(100, &eventHandlers{
		OnGesture: func(ctx EventCtx) {
			ctx.Consume()
		},
	})
	e := &Event{CentroidX: 100, CentroidY: 100}
	gestureHandler(root, e, newTestWindow())
	if !e.IsHandled {
		t.Error("gestureHandler missed a leaf at depth 100")
	}
}

// --- Non-finite touch input ---

func TestNonFiniteTouchDropped(t *testing.T) {
	t.Parallel()
	var gestures int
	var downs int
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			gestures++
		},
		OnMouseDown: func(ctx EventCtx) {
			downs++
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)
	nan := float32(math.NaN())

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, nan, nan))
	if gs.numTouches != 0 {
		t.Fatalf("numTouches = %d, want 0 for NaN coords",
			gs.numTouches)
	}
	if gestures != 0 || downs != 0 {
		t.Fatalf("gestures=%d downs=%d, want 0/0 for NaN began",
			gestures, downs)
	}

	// A NaN move over a tracked touch leaves it in place.
	w.handleTouch(root, touchEvent(EventTouchesBegan, 7, 100, 100))
	w.handleTouch(root, touchEvent(EventTouchesMoved, 7, nan, 100))
	if gs.touches[0].x != 100 || gs.touches[0].y != 100 {
		t.Errorf("tracked moved to (%f,%f), want (100,100)",
			gs.touches[0].x, gs.touches[0].y)
	}

	// The tracked finger still taps: no wedge, no NaN stored.
	gs.nowFn = fixedClock(50_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 7, 100, 100))
	if gestures != 1 {
		t.Errorf("gestures = %d, want 1 tap after NaN move",
			gestures)
	}
}

// --- Huge pinch span stays finite ---

func TestHugePinchSpanEmitsFiniteScale(t *testing.T) {
	t.Parallel()
	var scales []float32
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			if ctx.Event.GestureType == GesturePinch {
				scales = append(scales, ctx.Event.PinchScale)
			}
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture
	gs.nowFn = fixedClock(0)
	big := float32(1e20)

	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, -big, 0))
	w.handleTouch(root, twoTouchEvent(EventTouchesBegan,
		1, -big, 0, 2, big, 0))
	// The span overflows float32: whatever emits must be finite.
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, -big, 0, 2, big+float32(1e19), 0))
	for _, s := range scales {
		if math.IsInf(float64(s), 0) ||
			math.IsNaN(float64(s)) {
			t.Fatalf("pinch scale non-finite: %f", s)
		}
	}

	// Back to finite coords: the baseline reseeds and the
	// pinch recovers instead of sticking at the overflow.
	scales = nil
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, -100, 0, 2, 100, 0))
	w.handleTouch(root, twoTouchEvent(EventTouchesMoved,
		1, -150, 0, 2, 150, 0))
	if len(scales) == 0 {
		t.Fatal("pinch never recovered after overflow span")
	}
	last := scales[len(scales)-1]
	if math.IsInf(float64(last), 0) ||
		math.IsNaN(float64(last)) || last <= 1.0 {
		t.Errorf("recovered scale = %f, want finite > 1.0", last)
	}
}

// --- Dialog routing ---

func TestDialogRoute(t *testing.T) {
	t.Parallel()
	if dialogRoute(nil) != nil {
		t.Error("dialogRoute(nil) != nil")
	}
	w := &Window{}
	w.layout = Layout{Children: []Layout{{}, {}}}
	if got := dialogRoute(w); got != &w.layout {
		t.Error("hidden dialog routes past the root layout")
	}
	w.dialogCfg.visible = true
	if got := dialogRoute(w); got != &w.layout.Children[1] {
		t.Error("visible dialog does not route to the last child")
	}
}

// --- Tap timing boundaries ---

func TestDoubleTapGapExpiryIsSingleTap(t *testing.T) {
	t.Parallel()
	var seq []GestureType
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			seq = append(seq, ctx.Event.GestureType)
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture

	gs.nowFn = fixedClock(0)
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	gs.nowFn = fixedClock(50_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 100, 100))

	// Past the 300ms double-tap gap: a second tap, not a double.
	gs.nowFn = fixedClock(500_000_000)
	w.handleTouch(root, touchEvent(EventTouchesBegan, 2, 102, 102))
	gs.nowFn = fixedClock(550_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 2, 102, 102))

	if len(seq) != 2 || seq[0] != GestureTap ||
		seq[1] != GestureTap {
		t.Errorf("sequence = %v, want [Tap Tap]", seq)
	}
}

func TestTapAtTimeoutBoundaryEmitsNothing(t *testing.T) {
	t.Parallel()
	var gestures int
	root := gestureLayout(&eventHandlers{
		OnGesture: func(ctx EventCtx) {
			gestures++
		},
	})
	w := &Window{}
	w.animations = make(map[string]Animation)
	gs := &w.viewState.gesture

	gs.nowFn = fixedClock(0)
	w.handleTouch(root, touchEvent(EventTouchesBegan, 1, 100, 100))
	// Exactly the 300ms tap timeout: held too long for a tap.
	gs.nowFn = fixedClock(300_000_000)
	w.handleTouch(root, touchEvent(EventTouchesEnded, 1, 100, 100))

	if gestures != 0 {
		t.Errorf("gestures = %d, want 0 at the tap timeout",
			gestures)
	}
}
