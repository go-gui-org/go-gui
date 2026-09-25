package gui

import (
	"math"
	"testing"
)

// dragScrollTestWindow builds a window holding one DragScroll
// scrollable viewport with oversized content. The viewport sits at
// (10, 10) so presses can land inside or outside it. Returns the
// window and the viewport's child layouts for assertions.
func dragScrollTestWindow(
	t *testing.T, mode scrollMode, child OnClickChild,
) *Window {
	t.Helper()
	contentChildren := []Layout{
		{Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     200,
			Height:    300,
			shapeClip: drawClip{Width: 200, Height: 300},
		}},
	}
	if child.onClick != nil {
		contentChildren = append(contentChildren, Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				ID:        "node",
				Focusable: true,
				X:         10,
				Y:         10,
				Width:     60,
				Height:    20,
				shapeClip: drawClip{X: 20, Y: 20, Width: 60, Height: 20},
			},
		})
	}
	w := &Window{}
	w.layout = Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			shapeClip: drawClip{Width: 800, Height: 600},
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType:  shapeRectangle,
				Scrollable: true,
				DragScroll: true,
				ID:         "view",
				X:          10,
				Y:          10,
				Width:      100,
				Height:     100,
				Axis:       axisTopToBottom,
				ScrollMode: mode,
				shapeClip:  drawClip{X: 10, Y: 10, Width: 100, Height: 100},
			},
				Children: contentChildren,
			},
		},
	}
	viewport := &w.layout.Children[0]
	if child.onClick != nil {
		clicks := child.onClick
		viewport.Children[1].Shape.events = w.allocEventHandlers(
			eventHandlers{OnClick: func(ctx EventCtx) {
				*clicks++
				ctx.Consume()
			}},
		)
	}
	return w
}

// OnClickChild carries an optional click counter for the node child.
type OnClickChild struct {
	onClick *int
}

func dragScrollDown(w *Window, x, y float32) {
	mouseDownHandler(&w.layout, false, &Event{
		Type:        EventMouseDown,
		MouseX:      x,
		MouseY:      y,
		MouseButton: MouseLeft,
	}, w)
}

func dragScrollMove(w *Window, x, y float32) {
	mouseMoveHandler(&w.layout, &Event{
		Type:   EventMouseMove,
		MouseX: x,
		MouseY: y,
	}, w)
}

func dragScrollUp(w *Window, x, y float32) {
	mouseUpHandler(&w.layout, &Event{
		Type:        EventMouseUp,
		MouseX:      x,
		MouseY:      y,
		MouseButton: MouseLeft,
	}, w)
}

func dragScrollOffsetY(w *Window) float32 {
	return w.scrollY().GetOr("view", 0)
}

func dragScrollOffsetX(w *Window) float32 {
	return w.scrollX().GetOr("view", 0)
}

func TestDragScrollPanMovesOffset(t *testing.T) {
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
	// Viewport spans (10,10)-(110,110); press mid-viewport.
	dragScrollDown(w, 60, 60)
	if !w.mouseIsLocked() {
		t.Fatal("press did not take the mouse lock")
	}
	// Drag up 30: the content follows the pointer, so the
	// stored offset goes 30 negative.
	dragScrollMove(w, 60, 30)
	if got := dragScrollOffsetY(w); got != -30 {
		t.Errorf("offset Y = %v, want -30", got)
	}
	dragScrollUp(w, 60, 30)
	if w.mouseIsLocked() {
		t.Error("release left the mouse locked")
	}
}

func TestDragScrollBelowThresholdStays(t *testing.T) {
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
	dragScrollDown(w, 60, 60)
	dragScrollMove(w, 61, 62)
	if got := dragScrollOffsetY(w); got != 0 {
		t.Errorf("offset Y = %v, want 0", got)
	}
	if st := w.viewState.dragPan; !st.active || st.panned {
		t.Errorf("state = %+v, want active without pan", st)
	}
	dragScrollUp(w, 61, 62)
}

func TestDragScrollTapReplaysChildClick(t *testing.T) {
	clicks := 0
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{onClick: &clicks})
	// The node child sits at viewport-relative (10,10)-(70,30),
	// which is window-absolute (20,20)-(80,40).
	dragScrollDown(w, 40, 30)
	if clicks != 0 {
		t.Fatalf("press fired the child click, want deferred tap")
	}
	dragScrollUp(w, 40, 30)
	if clicks != 1 {
		t.Errorf("clicks = %d, want 1", clicks)
	}
	if w.mouseIsLocked() {
		t.Error("tap left the mouse locked")
	}
}

func TestDragScrollPanSuppressesChildClick(t *testing.T) {
	clicks := 0
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{onClick: &clicks})
	// Seed mid-content: dragging down from the top would clamp
	// at zero and move nothing.
	w.scrollY().Set("view", -100)
	dragScrollDown(w, 40, 30)
	dragScrollMove(w, 40, 90)
	if clicks != 0 {
		t.Fatalf("pan fired the child click, want suppression")
	}
	if got := dragScrollOffsetY(w); got != -40 {
		t.Errorf("offset Y = %v, want -40", got)
	}
	dragScrollUp(w, 40, 90)
	if clicks != 0 {
		t.Errorf("release after pan fired the child click, want 0")
	}
}

func TestDragScrollClampsAtContentEnd(t *testing.T) {
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
	dragScrollDown(w, 60, 60)
	// Content is 300 tall in a 100 viewport: -200 is the end.
	dragScrollMove(w, 60, -500)
	if got := dragScrollOffsetY(w); got != -200 {
		t.Errorf("offset Y = %v, want -200", got)
	}
	dragScrollUp(w, 60, -500)
}

func TestDragScrollHonorsScrollMode(t *testing.T) {
	w := dragScrollTestWindow(t, ScrollHorizontalOnly, OnClickChild{})
	dragScrollDown(w, 60, 60)
	// Diagonal drag: only the X axis may move.
	dragScrollMove(w, 30, 30)
	if got := dragScrollOffsetY(w); got != 0 {
		t.Errorf("offset Y = %v, want 0", got)
	}
	// Content is 200 wide in a 100 viewport: dragging left 30
	// pulls the content 30 negative.
	if got := dragScrollOffsetX(w); got != -30 {
		t.Errorf("offset X = %v, want -30", got)
	}
	dragScrollUp(w, 30, 30)
}

func TestDragScrollCancelKeepsOffset(t *testing.T) {
	clicks := 0
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{onClick: &clicks})
	// Seed mid-content: dragging down from the top would clamp
	// at zero and move nothing.
	w.scrollY().Set("view", -100)
	dragScrollDown(w, 40, 30)
	dragScrollMove(w, 40, 90)
	const want = float32(-40)
	if got := dragScrollOffsetY(w); got != want {
		t.Fatalf("offset Y = %v, want %v", got, want)
	}
	w.MouseCancel()
	if got := dragScrollOffsetY(w); got != want {
		t.Errorf("offset Y = %v after cancel, want %v", got, want)
	}
	if clicks != 0 {
		t.Errorf("cancel fired the child click, want 0")
	}
	// The revoked release unwinds silently: no replay, no panic.
	dragScrollUp(w, 40, 90)
	if clicks != 0 {
		t.Errorf("up after cancel fired the child click, want 0")
	}
}

func TestDragScrollGestureFallbackSkips(t *testing.T) {
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
	// A single-finger pan over the viewport reaches the gesture
	// fallback, which must decline: the same finger already pans
	// through the mouse-lock path, and scrolling here too would
	// move twice for one finger.
	e := &Event{
		GestureType:  GesturePan,
		GesturePhase: GesturePhaseChanged,
		CentroidX:    60,
		CentroidY:    60,
		GestureDX:    0,
		GestureDY:    -20,
	}
	gestureHandler(&w.layout, e, w)
	if e.IsHandled {
		t.Error("gesture fallback handled a DragScroll pan")
	}
	if got := dragScrollOffsetY(w); got != 0 {
		t.Errorf("offset Y = %v, want 0", got)
	}
}

func TestDragScrollRequiresScrollableID(t *testing.T) {
	w := &Window{}
	defer func() {
		if recover() == nil {
			t.Error("DragScroll without Scrollable did not panic")
		}
	}()
	buildContainerShape(&ContainerCfg{DragScroll: true, ID: "x"}, w)
}

func TestDragScrollRequiresID(t *testing.T) {
	w := &Window{}
	defer func() {
		if recover() == nil {
			t.Error("DragScroll without ID did not panic")
		}
	}()
	buildContainerShape(
		&ContainerCfg{DragScroll: true, Scrollable: true}, w)
}

func TestDragScrollHoverGrabOnlyOnOverflow(t *testing.T) {
	// Overflowing viewport: hover shows the grab hand.
	w := &Window{}
	view := container(ContainerCfg{
		ID:         "view",
		Scrollable: true,
		DragScroll: true,
		Sizing:     FixedFixed,
		Width:      100,
		Height:     100,
		Content: []View{container(ContainerCfg{
			Sizing: FixedFixed,
			Width:  100,
			Height: 300,
		})},
	}).GenerateLayout(w)
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{view},
	}
	w.setMouseCursor(CursorArrow)
	view.Shape.events.OnHover(EventCtx{&view, &Event{}, w})
	if w.viewState.mouseCursor != CursorGrab {
		t.Errorf("cursor = %v, want grab on overflow", w.viewState.mouseCursor)
	}
	// Fitting viewport: the same hover leaves the arrow alone.
	w2 := &Window{}
	fit := container(ContainerCfg{
		ID:         "view",
		Scrollable: true,
		DragScroll: true,
		Sizing:     FixedFixed,
		Width:      100,
		Height:     100,
		Content: []View{container(ContainerCfg{
			Sizing: FixedFixed,
			Width:  50,
			Height: 50,
		})},
	}).GenerateLayout(w2)
	w2.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{fit},
	}
	w2.setMouseCursor(CursorArrow)
	fit.Shape.events.OnHover(EventCtx{&fit, &Event{}, w2})
	if w2.viewState.mouseCursor != CursorArrow {
		t.Errorf("cursor = %v, want arrow when content fits",
			w2.viewState.mouseCursor)
	}
}

func TestDragScrollGrabbingWhilePanning(t *testing.T) {
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
	dragScrollDown(w, 60, 60)
	dragScrollMove(w, 60, 30)
	if w.viewState.mouseCursor != CursorGrabbing {
		t.Errorf("cursor = %v, want grabbing mid-pan",
			w.viewState.mouseCursor)
	}
	dragScrollUp(w, 60, 30)
}

func TestDragScrollDebugSweepTerminates(t *testing.T) {
	// The unconsumed-event sweep dispatches synthetic presses
	// through the real walk. A DragScroll claim mid-sweep must
	// unwind through the tap replay, never wedge the window.
	clicks := 0
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{onClick: &clicks})
	w.TestUnconsumedEvents()
	w.MouseCancel()
	if w.mouseIsLocked() {
		t.Error("sweep left the mouse locked")
	}
}

func TestDragScrollNestedInnerClaims(t *testing.T) {
	w := &Window{}
	innerContent := Layout{Shape: &Shape{
		shapeType: shapeRectangle,
		Width:     60,
		Height:    200,
		shapeClip: drawClip{Width: 60, Height: 200},
	}}
	inner := Layout{Shape: &Shape{
		shapeType:  shapeRectangle,
		Scrollable: true,
		DragScroll: true,
		ID:         "inner",
		X:          20,
		Y:          20,
		Width:      60,
		Height:     60,
		ScrollMode: scrollBoth,
		shapeClip:  drawClip{X: 20, Y: 20, Width: 60, Height: 60},
	}, Children: []Layout{innerContent}}
	outerContent := Layout{Shape: &Shape{
		shapeType: shapeRectangle,
		Width:     100,
		Height:    300,
		shapeClip: drawClip{Width: 100, Height: 300},
	}}
	w.layout = Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			shapeClip: drawClip{Width: 800, Height: 600},
		},
		Children: []Layout{{Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			DragScroll: true,
			ID:         "outer",
			X:          10,
			Y:          10,
			Width:      100,
			Height:     100,
			ScrollMode: scrollBoth,
			shapeClip:  drawClip{X: 10, Y: 10, Width: 100, Height: 100},
		}, Children: []Layout{outerContent, inner}}},
	}
	// (50,50) sits inside both viewports: the inner one owns it.
	dragScrollDown(w, 50, 50)
	if got := w.viewState.dragPan.scrollID; got != "inner" {
		t.Fatalf("claim = %q, want inner", got)
	}
	dragScrollMove(w, 50, 30)
	if got := w.scrollY().GetOr("inner", 0); got != -20 {
		t.Errorf("inner offset = %v, want -20", got)
	}
	if got := w.scrollY().GetOr("outer", 0); got != 0 {
		t.Errorf("outer offset = %v, want 0", got)
	}
	dragScrollUp(w, 50, 30)
}

func TestDragScrollScrollbarPassThrough(t *testing.T) {
	thumbClicks := 0
	w := &Window{}
	thumb := Layout{Shape: &Shape{
		shapeType:            shapeRectangle,
		X:                    90,
		Y:                    10,
		Width:                10,
		Height:               40,
		shapeClip:            drawClip{X: 90, Y: 10, Width: 10, Height: 40},
		scrollbarOrientation: scrollbarVertical,
	}}
	thumb.Shape.events = w.allocEventHandlers(
		eventHandlers{OnClick: func(ctx EventCtx) {
			thumbClicks++
			ctx.Consume()
		}},
	)
	w.layout = Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			shapeClip: drawClip{Width: 800, Height: 600},
		},
		Children: []Layout{{Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			DragScroll: true,
			ID:         "view",
			X:          10,
			Y:          10,
			Width:      100,
			Height:     100,
			ScrollMode: scrollBoth,
			shapeClip:  drawClip{X: 10, Y: 10, Width: 100, Height: 100},
		}, Children: []Layout{
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Width:     100,
				Height:    300,
				shapeClip: drawClip{Width: 100, Height: 300},
			}},
			thumb,
		}}},
	}
	// Press on the scrollbar thumb: the thumb owns the press,
	// so no pan starts and its click fires on press as usual.
	dragScrollDown(w, 95, 30)
	if w.mouseIsLocked() {
		t.Fatal("scrollbar press took the pan lock")
	}
	if thumbClicks != 1 {
		t.Errorf("thumb clicks = %d, want 1", thumbClicks)
	}
}

func TestDragScrollStaleTreeNoReplay(t *testing.T) {
	clicks := 0
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{onClick: &clicks})
	dragScrollDown(w, 40, 30)
	// The view changes under the press: the container is gone.
	w.layout.Children = nil
	dragScrollUp(w, 40, 30)
	if clicks != 0 {
		t.Errorf("stale tap replayed a click, want 0")
	}
	if w.mouseIsLocked() {
		t.Error("stale release left the mouse locked")
	}
}

func TestDragScrollTapSetsFocus(t *testing.T) {
	clicks := 0
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{onClick: &clicks})
	dragScrollDown(w, 40, 30)
	dragScrollUp(w, 40, 30)
	if got := w.FocusID(); got != "node" {
		t.Errorf("focus = %q, want node", got)
	}
}

func TestDragScrollNonFiniteMoveIgnored(t *testing.T) {
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
	dragScrollDown(w, 60, 60)
	dragScrollMove(w, float32(math.NaN()), float32(math.NaN()))
	if got := dragScrollOffsetY(w); got != 0 {
		t.Errorf("offset Y = %v after NaN move, want 0", got)
	}
	if st := w.viewState.dragPan; !st.active {
		t.Fatal("NaN move ended the press")
	}
	// The press deltas from the last good position, so the next
	// valid move still pans the full distance.
	dragScrollMove(w, 60, 30)
	if got := dragScrollOffsetY(w); got != -30 {
		t.Errorf("offset Y = %v, want -30", got)
	}
	dragScrollUp(w, 60, 30)
}

func TestDragScrollDisabledNoClaim(t *testing.T) {
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
	w.layout.Children[0].Shape.Disabled = true
	dragScrollDown(w, 60, 60)
	if w.mouseIsLocked() {
		t.Error("disabled container claimed the press")
	}
}

func TestDragScrollRightButtonNoClaim(t *testing.T) {
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
	mouseDownHandler(&w.layout, false, &Event{
		Type:        EventMouseDown,
		MouseX:      60,
		MouseY:      60,
		MouseButton: MouseRight,
	}, w)
	if w.mouseIsLocked() {
		t.Error("right press claimed the press")
	}
}

// A drag-to-scroll claim consumes every press inside its container,
// empty space included, so the press puts off its blur decision. A tap
// on empty space blurs on its replay, and a pan blurs when it starts
// (issue #770).
func TestDragScrollPressOnEmptySpaceBlurs(t *testing.T) {
	for _, pan := range []bool{false, true} {
		w := dragScrollTestWindow(t, scrollBoth, OnClickChild{})
		w.focused = true
		w.SetFocus("elsewhere")
		w.EventFn(&Event{
			Type: EventMouseDown, MouseButton: MouseLeft, MouseX: 60, MouseY: 60,
		})
		if !w.mouseIsLocked() {
			t.Fatal("press did not start a pan claim")
		}
		if got := w.FocusID(); got != "elsewhere" {
			t.Fatalf("pan=%v: focus at press = %q, want it kept until "+
				"tap or pan", pan, got)
		}
		upY := float32(60)
		if pan {
			upY = 30
			w.EventFn(&Event{
				Type: EventMouseMove, MouseButton: MouseLeft,
				MouseX: 60, MouseY: upY,
			})
			if got := w.FocusID(); got != "" {
				t.Fatalf("focus at pan start = %q, want empty", got)
			}
		}
		w.EventFn(&Event{
			Type: EventMouseUp, MouseButton: MouseLeft, MouseX: 60, MouseY: upY,
		})
		if got := w.FocusID(); got != "" {
			t.Errorf("pan=%v: focus = %q, want empty", pan, got)
		}
	}
}
