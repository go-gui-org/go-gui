//go:build js && wasm

package web

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The web backend used to build Ended and Cancelled events from
// e.touches, which no longer holds the lifted fingers. That sent an
// empty event that removed nothing, so the recognizer stayed wedged
// with the finger still tracked: the second tap of a double-tap
// arrived as a second finger of a phantom multi-touch, and pinch and
// rotate never got their Ended. These tests pin the fixed mapping:
// Ended and Cancelled carry e.changedTouches.

// Last finger lifting: touches is empty, changed holds the finger.
func TestTouchEndedCarriesLiftedFinger(t *testing.T) {
	t.Parallel()
	lifted := gui.TouchPoint{
		Identifier: 7, PosX: 100, PosY: 200,
		ToolType: gui.TouchToolFinger,
	}
	evt := touchEventFromLists(gui.EventTouchesEnded, nil,
		[]gui.TouchPoint{lifted})

	if evt.Type != gui.EventTouchesEnded {
		t.Fatalf("Type = %v, want EventTouchesEnded", evt.Type)
	}
	if evt.NumTouches != 1 {
		t.Fatalf("NumTouches = %d, want 1", evt.NumTouches)
	}
	got := evt.Touches[0]
	if got.Identifier != 7 || got.PosX != 100 || got.PosY != 200 {
		t.Errorf("touch = %+v, want id 7 at (100,200)", got)
	}
	if !got.Changed {
		t.Error("Changed = false, want true")
	}
}

// One of two fingers lifting: touches holds the finger staying
// down, changed holds the one that lifted. The event must carry
// the lifted one.
func TestTouchEndedTwoToOneCarriesLifted(t *testing.T) {
	t.Parallel()
	staying := gui.TouchPoint{
		Identifier: 2, PosX: 200, PosY: 200,
		ToolType: gui.TouchToolFinger,
	}
	lifted := gui.TouchPoint{
		Identifier: 1, PosX: 100, PosY: 100,
		ToolType: gui.TouchToolFinger,
	}
	evt := touchEventFromLists(gui.EventTouchesEnded,
		[]gui.TouchPoint{staying}, []gui.TouchPoint{lifted})

	if evt.NumTouches != 1 {
		t.Fatalf("NumTouches = %d, want 1", evt.NumTouches)
	}
	if got := evt.Touches[0]; got.Identifier != 1 || !got.Changed {
		t.Errorf("touch = %+v, want lifted id 1 marked Changed",
			got)
	}
}

// Cancelled behaves like Ended: the finger is gone from touches.
func TestTouchCancelledCarriesLiftedFinger(t *testing.T) {
	t.Parallel()
	lifted := gui.TouchPoint{
		Identifier: 3, PosX: 50, PosY: 60,
		ToolType: gui.TouchToolFinger,
	}
	evt := touchEventFromLists(gui.EventTouchesCancelled, nil,
		[]gui.TouchPoint{lifted})

	if evt.NumTouches != 1 {
		t.Fatalf("NumTouches = %d, want 1", evt.NumTouches)
	}
	if got := evt.Touches[0]; got.Identifier != 3 || !got.Changed {
		t.Errorf("touch = %+v, want lifted id 3 marked Changed",
			got)
	}
}

// Began still carries the fingers down, marking only the new one.
func TestTouchBeganMarksOnlyNewFinger(t *testing.T) {
	t.Parallel()
	old := gui.TouchPoint{
		Identifier: 1, PosX: 100, PosY: 200,
		ToolType: gui.TouchToolFinger,
	}
	newFinger := gui.TouchPoint{
		Identifier: 2, PosX: 200, PosY: 200,
		ToolType: gui.TouchToolFinger,
	}
	evt := touchEventFromLists(gui.EventTouchesBegan,
		[]gui.TouchPoint{old, newFinger},
		[]gui.TouchPoint{newFinger})

	if evt.NumTouches != 2 {
		t.Fatalf("NumTouches = %d, want 2", evt.NumTouches)
	}
	if evt.Touches[0].Changed {
		t.Error("old finger marked Changed, want unmarked")
	}
	if !evt.Touches[1].Changed {
		t.Error("new finger unmarked, want Changed")
	}
}

// Moved still carries the fingers down, marking only moved ones.
func TestTouchMovedMarksOnlyMovedFinger(t *testing.T) {
	t.Parallel()
	still := gui.TouchPoint{
		Identifier: 1, PosX: 100, PosY: 200,
		ToolType: gui.TouchToolFinger,
	}
	moved := gui.TouchPoint{
		Identifier: 2, PosX: 210, PosY: 200,
		ToolType: gui.TouchToolFinger,
	}
	evt := touchEventFromLists(gui.EventTouchesMoved,
		[]gui.TouchPoint{still, moved},
		[]gui.TouchPoint{moved})

	if evt.NumTouches != 2 {
		t.Fatalf("NumTouches = %d, want 2", evt.NumTouches)
	}
	if evt.Touches[0].Changed {
		t.Error("still finger marked Changed, want unmarked")
	}
	if !evt.Touches[1].Changed {
		t.Error("moved finger unmarked, want Changed")
	}
}

// More fingers than the fixed array holds are capped, never
// overflowing the event.
func TestTouchEventCapsAtEight(t *testing.T) {
	t.Parallel()
	var all, changed []gui.TouchPoint
	for id := uint64(1); id <= 10; id++ {
		all = append(all, gui.TouchPoint{
			Identifier: id, PosX: float32(id * 10), PosY: 0,
			ToolType: gui.TouchToolFinger,
		})
		changed = append(changed, gui.TouchPoint{
			Identifier: id, PosX: float32(id * 10), PosY: 0,
			ToolType: gui.TouchToolFinger,
		})
	}
	evt := touchEventFromLists(gui.EventTouchesMoved, all, changed)

	if evt.NumTouches != 8 {
		t.Fatalf("NumTouches = %d, want 8", evt.NumTouches)
	}
}

// --- Pipeline: helper-shaped events through the recognizer ---

// gesturePad opens a window with one DrawCanvas recording gesture
// types, and reports the pad center in window coordinates.
func gesturePad(t *testing.T) (*gui.Window, float32, float32,
	*[]gui.GestureType) {
	t.Helper()
	var got []gui.GestureType
	w := gui.NewTestWindow(gui.WindowCfg{})
	w.SetView(func(*gui.Window) gui.View {
		return gui.DrawCanvas(gui.DrawCanvasCfg{
			ID:     "pad",
			Width:  400,
			Height: 400,
			OnGesture: func(ctx gui.EventCtx) {
				got = append(got, ctx.Event.GestureType)
			},
		})
	})
	root := w.TestRender(nil)
	ly, ok := root.FindByID("pad")
	if !ok {
		t.Fatal("pad not found")
	}
	cx := ly.Shape.X + ly.Shape.Width/2
	cy := ly.Shape.Y + ly.Shape.Height/2
	return w, cx, cy, &got
}

// webTouch models one DOM touch event's two lists and sends the
// mapped framework event, settling a frame the way the backend's
// event loop does between DOM events.
func webTouch(w *gui.Window, typ gui.EventType,
	all, changed []gui.TouchPoint) {
	evt := touchEventFromLists(typ, all, changed)
	w.EventFn(&evt)
	w.TestRender(nil)
}

func webFinger(id uint64, x, y float32) gui.TouchPoint {
	return gui.TouchPoint{
		Identifier: id, PosX: x, PosY: y,
		ToolType: gui.TouchToolFinger,
	}
}

// Two rapid taps shaped like the DOM sends them (Ended carries
// only changedTouches) must end in a double-tap. Before the fix
// the Ended events were empty, the first finger stayed tracked,
// and the second tap arrived as a second finger: no double-tap.
func TestWebDoubleTapReachesGesture(t *testing.T) {
	w, cx, cy, got := gesturePad(t)

	webTouch(w, gui.EventTouchesBegan,
		[]gui.TouchPoint{webFinger(1, cx, cy)},
		[]gui.TouchPoint{webFinger(1, cx, cy)})
	webTouch(w, gui.EventTouchesEnded, nil,
		[]gui.TouchPoint{webFinger(1, cx, cy)})
	webTouch(w, gui.EventTouchesBegan,
		[]gui.TouchPoint{webFinger(2, cx+2, cy+2)},
		[]gui.TouchPoint{webFinger(2, cx+2, cy+2)})
	webTouch(w, gui.EventTouchesEnded, nil,
		[]gui.TouchPoint{webFinger(2, cx+2, cy+2)})

	if len(*got) == 0 || (*got)[len(*got)-1] != gui.GestureDoubleTap {
		t.Fatalf("gestures = %v, want last GestureDoubleTap",
			*got)
	}
}

// A two-finger spread shaped like the DOM sends it must report a
// scale above 1 and finish cleanly, leaving the recognizer able to
// tap again afterwards.
func TestWebPinchReachesGesture(t *testing.T) {
	var got []gui.GestureType
	var scales []float32
	w := gui.NewTestWindow(gui.WindowCfg{})
	w.SetView(func(*gui.Window) gui.View {
		return gui.DrawCanvas(gui.DrawCanvasCfg{
			ID:     "pad",
			Width:  400,
			Height: 400,
			OnGesture: func(ctx gui.EventCtx) {
				got = append(got, ctx.Event.GestureType)
				if ctx.Event.GestureType == gui.GesturePinch {
					scales = append(scales,
						ctx.Event.PinchScale)
				}
			},
		})
	})
	root := w.TestRender(nil)
	ly, ok := root.FindByID("pad")
	if !ok {
		t.Fatal("pad not found")
	}
	cx := ly.Shape.X + ly.Shape.Width/2
	cy := ly.Shape.Y + ly.Shape.Height/2

	ax, ay := cx-50, cy
	bx, by := cx+50, cy
	webTouch(w, gui.EventTouchesBegan,
		[]gui.TouchPoint{webFinger(1, ax, ay)},
		[]gui.TouchPoint{webFinger(1, ax, ay)})
	webTouch(w, gui.EventTouchesBegan,
		[]gui.TouchPoint{webFinger(1, ax, ay), webFinger(2, bx, by)},
		[]gui.TouchPoint{webFinger(2, bx, by)})
	nax, nay := ax-20, ay
	nbx, nby := bx+30, by
	webTouch(w, gui.EventTouchesMoved,
		[]gui.TouchPoint{webFinger(1, nax, nay), webFinger(2, nbx, nby)},
		[]gui.TouchPoint{webFinger(1, nax, nay), webFinger(2, nbx, nby)})

	if len(scales) == 0 || scales[len(scales)-1] <= 1 {
		t.Fatalf("scales = %v, want last > 1", scales)
	}

	// Both fingers lift at once, DOM-shaped.
	webTouch(w, gui.EventTouchesEnded, nil,
		[]gui.TouchPoint{webFinger(1, nax, nay), webFinger(2, nbx, nby)})

	// The recognizer must be idle again: a tap lands as a tap.
	got = nil
	webTouch(w, gui.EventTouchesBegan,
		[]gui.TouchPoint{webFinger(3, cx, cy)},
		[]gui.TouchPoint{webFinger(3, cx, cy)})
	webTouch(w, gui.EventTouchesEnded, nil,
		[]gui.TouchPoint{webFinger(3, cx, cy)})
	if len(got) == 0 || got[len(got)-1] != gui.GestureTap {
		t.Fatalf("after pinch, gestures = %v, want last GestureTap",
			got)
	}
}

// A two-finger twist shaped like the DOM sends it must report a
// non-zero rotation.
func TestWebRotateReachesGesture(t *testing.T) {
	var got []gui.GestureType
	var rotations []float32
	w := gui.NewTestWindow(gui.WindowCfg{})
	w.SetView(func(*gui.Window) gui.View {
		return gui.DrawCanvas(gui.DrawCanvasCfg{
			ID:     "pad",
			Width:  400,
			Height: 400,
			OnGesture: func(ctx gui.EventCtx) {
				got = append(got, ctx.Event.GestureType)
				if ctx.Event.GestureType == gui.GestureRotate {
					rotations = append(rotations,
						ctx.Event.GestureRotation)
				}
			},
		})
	})
	root := w.TestRender(nil)
	ly, ok := root.FindByID("pad")
	if !ok {
		t.Fatal("pad not found")
	}
	cx := ly.Shape.X + ly.Shape.Width/2
	cy := ly.Shape.Y + ly.Shape.Height/2

	ax, ay := cx-50, cy
	bx, by := cx+50, cy
	webTouch(w, gui.EventTouchesBegan,
		[]gui.TouchPoint{webFinger(1, ax, ay)},
		[]gui.TouchPoint{webFinger(1, ax, ay)})
	webTouch(w, gui.EventTouchesBegan,
		[]gui.TouchPoint{webFinger(1, ax, ay), webFinger(2, bx, by)},
		[]gui.TouchPoint{webFinger(2, bx, by)})
	// Twist ~45 degrees around the centroid.
	nax, nay := cx-35, cy-35
	nbx, nby := cx+35, cy+35
	webTouch(w, gui.EventTouchesMoved,
		[]gui.TouchPoint{webFinger(1, nax, nay), webFinger(2, nbx, nby)},
		[]gui.TouchPoint{webFinger(1, nax, nay), webFinger(2, nbx, nby)})

	if len(rotations) == 0 || rotations[len(rotations)-1] == 0 {
		t.Fatalf("rotations = %v, want last non-zero",
			rotations)
	}
}
