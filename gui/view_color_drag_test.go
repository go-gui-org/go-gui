package gui

import "testing"

// clickShape dispatches a left-click at window coordinates (x, y) into
// a one-shape tree covering box, and returns the event so the caller
// can check whether the handler consumed it.
//
// The shape is placed at box's origin: dispatch hit-tests against
// shapeClip while the handler reads Shape.X/Y, and a control that
// works only at the window origin is exactly the bug this catches.
func clickShape(
	t *testing.T, s *Shape, x, y float32,
) *Event {
	t.Helper()
	root := &Layout{
		Shape:    &Shape{shapeClip: drawClip{Width: 1000, Height: 1000}},
		Children: []Layout{{Shape: s}},
	}
	layoutParents(root, nil)
	e := &Event{MouseX: x, MouseY: y}
	mouseDownHandler(root, false, e, &Window{})
	return e
}

// dragShape builds a shape carrying a colorDragTrack handler over the
// given box.
func dragShape(
	id string, x, y, w, h float32,
	onPoint func(nx, ny float32, ctx EventCtx),
) *Shape {
	return &Shape{
		ID:     id,
		X:      x,
		Y:      y,
		Width:  w,
		Height: h,
		events: &eventHandlers{
			OnClick: colorDragTrack(id, onPoint),
		},
		shapeClip: drawClip{X: x, Y: y, Width: w, Height: h},
	}
}

// A click must report where in the control it landed, as a fraction of
// the control's own box.
func TestColorDragTrackReportsNormalizedPoint(t *testing.T) {
	var gotX, gotY float32
	got := false
	s := dragShape("d", 0, 0, 100, 200,
		func(nx, ny float32, _ EventCtx) {
			gotX, gotY, got = nx, ny, true
		})

	clickShape(t, s, 25, 150)
	if !got {
		t.Fatal("handler never fired")
	}
	if f32Abs(gotX-0.25) > 0.001 || f32Abs(gotY-0.75) > 0.001 {
		t.Errorf("point = %v,%v want 0.25,0.75", gotX, gotY)
	}
}

// The coordinate conversion is the subtle part: OnClick arrives
// shape-relative, so a control away from the window origin must still
// report the same fraction.
func TestColorDragTrackHandlesOffsetShape(t *testing.T) {
	var gotX, gotY float32
	s := dragShape("d", 300, 400, 100, 100,
		func(nx, ny float32, _ EventCtx) { gotX, gotY = nx, ny })

	// Window (350, 450) is the centre of a control at (300, 400).
	clickShape(t, s, 350, 450)
	if f32Abs(gotX-0.5) > 0.001 || f32Abs(gotY-0.5) > 0.001 {
		t.Errorf("point = %v,%v want 0.5,0.5", gotX, gotY)
	}
}

// A drag that runs past the edge must pin to the edge rather than
// report a fraction outside the box.
func TestColorDragTrackClamps(t *testing.T) {
	var gotX, gotY float32
	s := dragShape("d", 0, 0, 100, 100,
		func(nx, ny float32, _ EventCtx) { gotX, gotY = nx, ny })
	s.shapeClip = drawClip{Width: 1000, Height: 1000} // let the click land

	clickShape(t, s, 500, 500)
	if gotX != 1 || gotY != 1 {
		t.Errorf("point = %v,%v want clamped to 1,1", gotX, gotY)
	}
}

// The control sits inside a focusable container in the composed
// picker; not consuming would move focus off it mid-drag.
func TestColorDragTrackConsumes(t *testing.T) {
	s := dragShape("d", 0, 0, 100, 100, func(float32, float32, EventCtx) {})
	e := clickShape(t, s, 50, 50)
	if !e.IsHandled {
		t.Error("drag handler did not consume the click")
	}
}

// A zero-sized shape would divide by zero; it must report nothing.
func TestColorDragTrackIgnoresZeroSize(t *testing.T) {
	called := false
	s := dragShape("d", 0, 0, 0, 0,
		func(float32, float32, EventCtx) { called = true })
	s.shapeClip = drawClip{Width: 100, Height: 100}
	clickShape(t, s, 5, 5)
	if called {
		t.Error("handler fired for a zero-sized control")
	}
}

// End to end through the real component: a click in the plane's
// lower-right quadrant must raise saturation and lower lightness.
func TestColorPlaneClickSetsValue(t *testing.T) {
	w := &Window{}
	var got HSLA
	fired := false
	l := generateViewLayout(ColorPlane(ColorPlaneCfg{
		ID:    "plane",
		Value: HSLA{H: 210, S: 0.5, L: 0.5, A: 1},
		Size:  100,
		OnChange: func(v HSLA, _ EventCtx) {
			got, fired = v, true
		},
	}), w)

	// Give the shape a box; layout normally does this in arrange.
	l.Shape.X, l.Shape.Y = 0, 0
	l.Shape.Width, l.Shape.Height = 100, 100
	l.Shape.shapeClip = drawClip{Width: 100, Height: 100}

	root := &Layout{
		Shape:    &Shape{shapeClip: drawClip{Width: 500, Height: 500}},
		Children: []Layout{l},
	}
	layoutParents(root, nil)
	mouseDownHandler(root, false,
		&Event{MouseX: 75, MouseY: 80}, w)

	if !fired {
		t.Fatal("plane did not report a change")
	}
	if f32Abs(got.S-0.75) > 0.01 {
		t.Errorf("S = %v, want 0.75", got.S)
	}
	if f32Abs(got.L-0.2) > 0.01 {
		t.Errorf("L = %v, want 0.2", got.L)
	}
	// Hue and alpha are not this control's business.
	if got.H != 210 || got.A != 1 {
		t.Errorf("H/A = %v/%v, want 210/1 passed through", got.H, got.A)
	}
}
