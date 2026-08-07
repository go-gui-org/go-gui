package gui

import "testing"

func TestIsFocusedTargetNilShape(t *testing.T) {
	w := &Window{}
	l := &Layout{}
	if isFocusedTarget(l, w) {
		t.Error("nil Shape should return false")
	}
}

func TestIsFocusedTargetReservedDialog(t *testing.T) {
	w := &Window{}
	l := &Layout{Shape: &Shape{ID: reservedDialogID}}
	if !isFocusedTarget(l, w) {
		t.Error("reservedDialogID should return true")
	}
}

func TestIsFocusedTargetZeroIDFocus(t *testing.T) {
	w := &Window{}
	l := &Layout{Shape: &Shape{Focusable: true, ID: "f0"}}
	if isFocusedTarget(l, w) {
		t.Error("IDFocus 0 should return false")
	}
}

func TestIsFocusedTargetMatches(t *testing.T) {
	w := &Window{}
	w.viewState.focusID = "f42"
	l := &Layout{Shape: &Shape{Focusable: true, ID: "f42"}}
	if !isFocusedTarget(l, w) {
		t.Error("matching IDFocus should return true")
	}
}

func TestExecuteFocusCallbackNil(t *testing.T) {
	w := &Window{}
	w.viewState.focusID = "f1"
	l := &Layout{Shape: &Shape{ID: reservedDialogID}}
	e := &Event{}
	if executeFocusCallback(l, e, w, nil, evNotify) {
		t.Error("nil callback should return false")
	}
}

func TestExecuteMouseCallbackOutside(t *testing.T) {
	w := &Window{}
	l := &Layout{Shape: &Shape{
		shapeClip: drawClip{X: 10, Y: 10, Width: 50, Height: 50},
	}}
	e := &Event{MouseX: 0, MouseY: 0}
	called := false
	cb := func(ctx EventCtx) { called = true }
	if executeMouseCallback(l, e, w, cb, evNotify) {
		t.Error("outside mouse should return false")
	}
	if called {
		t.Error("callback should not fire")
	}
}

func TestExecuteMouseCallbackInside(t *testing.T) {
	w := &Window{}
	s := &Shape{
		shapeClip: drawClip{X: 10, Y: 20, Width: 50, Height: 50},
	}
	s.X = 10
	s.Y = 20
	l := &Layout{Shape: s}
	e := &Event{MouseX: 30, MouseY: 40}
	var relX, relY float32
	cb := func(ctx EventCtx) {
		relX = ctx.Event.MouseX
		relY = ctx.Event.MouseY
		ctx.Consume()
	}
	if !executeMouseCallback(l, e, w, cb, evNotify) {
		t.Error("inside mouse should return true")
	}
	if relX != 20 || relY != 20 {
		t.Errorf("relative coords = (%f,%f), want (20,20)", relX, relY)
	}
}

func TestExecuteMouseCallbackRestoresEvent(t *testing.T) {
	w := &Window{}
	s := &Shape{
		shapeClip: drawClip{X: 10, Y: 20, Width: 50, Height: 50},
	}
	s.X = 10
	s.Y = 20
	l := &Layout{Shape: s}
	e := &Event{MouseX: 30, MouseY: 40}
	cb := func(ctx EventCtx) {
		ctx.Consume()
	}
	executeMouseCallback(l, e, w, cb, evNotify)
	if e.MouseX != 30 || e.MouseY != 40 {
		t.Errorf("event coords = (%f,%f), want (30,40)", e.MouseX, e.MouseY)
	}
}

func TestIsChildEnabledTraversal(t *testing.T) {
	if isChildEnabled(&Layout{}) {
		t.Error("nil Shape should return false")
	}
	if isChildEnabled(&Layout{Shape: &Shape{Disabled: true}}) {
		t.Error("Disabled should return false")
	}
	if !isChildEnabled(&Layout{Shape: &Shape{}}) {
		t.Error("normal shape should return true")
	}
}
