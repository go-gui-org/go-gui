package gui

import "testing"

func TestModifierHas(t *testing.T) {
	// none
	if !ModNone.Has(ModNone) {
		t.Error("none should have none")
	}
	if ModNone.Has(ModShift) {
		t.Error("none should not have shift")
	}

	// single modifier
	if !ModShift.Has(ModShift) {
		t.Error("shift should have shift")
	}
	if ModShift.Has(ModCtrl) {
		t.Error("shift should not have ctrl")
	}

	// combined bitmask
	combined := ModShift | ModCtrl
	if !combined.Has(ModShift) {
		t.Error("combined should have shift")
	}
	if !combined.Has(ModCtrl) {
		t.Error("combined should have ctrl")
	}
	if combined.Has(ModAlt) {
		t.Error("combined should not have alt")
	}
}

func TestModifierHasAny(t *testing.T) {
	if !ModNone.HasAny(ModNone) {
		t.Error("none should match none")
	}
	if ModNone.HasAny(ModShift) {
		t.Error("none should not match shift")
	}
	if !ModShift.HasAny(ModShift) {
		t.Error("shift should match shift")
	}
	if ModShift.HasAny(ModCtrl, ModAlt) {
		t.Error("shift should not match ctrl|alt")
	}

	combined := ModShift | ModCtrl
	if !combined.HasAny(ModShift) {
		t.Error("combined should match shift")
	}
	if !combined.HasAny(ModCtrl) {
		t.Error("combined should match ctrl")
	}
	if !combined.HasAny(ModShift, ModAlt) {
		t.Error("combined should match shift in {shift,alt}")
	}
}

func TestModifierCombinations(t *testing.T) {
	combined := ModCtrl | ModShift
	if !combined.Has(ModCtrl) {
		t.Error("should have ctrl")
	}
	if !combined.Has(ModShift) {
		t.Error("should have shift")
	}
	if combined.Has(ModAlt) {
		t.Error("should not have alt")
	}

	all := ModCtrl | ModShift | ModAlt | ModSuper
	if !all.Has(ModCtrl) {
		t.Error("all should have ctrl")
	}
	if !all.Has(ModShift) {
		t.Error("all should have shift")
	}
	if !all.Has(ModAlt) {
		t.Error("all should have alt")
	}
	if !all.Has(ModSuper) {
		t.Error("all should have super")
	}
}

func TestEventRelativeCoordinates(t *testing.T) {
	shape := &Shape{
		X: 100, Y: 50,
		Width: 200, Height: 100,
	}
	e := &Event{MouseX: 150, MouseY: 75}
	rel := eventRelativeTo(shape, e)

	if !f32AreClose(rel.MouseX, 50) {
		t.Errorf("mouseX: got %f, want 50", rel.MouseX)
	}
	if !f32AreClose(rel.MouseY, 25) {
		t.Errorf("mouseY: got %f, want 25", rel.MouseY)
	}
}

func TestEventTypeTracking(t *testing.T) {
	e := Event{
		Type:        EventMouseDown,
		MouseButton: MouseLeft,
		MouseX:      100,
		MouseY:      200,
	}
	if e.Type != EventMouseDown {
		t.Error("type should be mouse_down")
	}
	if e.MouseButton != MouseLeft {
		t.Error("button should be left")
	}
	if !f32AreClose(e.MouseX, 100) {
		t.Error("mouseX should be 100")
	}
	if !f32AreClose(e.MouseY, 200) {
		t.Error("mouseY should be 200")
	}
	if e.IsHandled {
		t.Error("should start unhandled")
	}
}

func TestEventHandledFlag(t *testing.T) {
	e := Event{Type: EventMouseDown}
	if e.IsHandled {
		t.Error("should start unhandled")
	}
	e.IsHandled = true
	if !e.IsHandled {
		t.Error("should be handled")
	}
}

func TestSpacebarToClick(t *testing.T) {
	// nil input → nil output
	if spacebarToClick(nil) != nil {
		t.Error("nil should return nil")
	}

	// spacebar fires handler
	called := false
	handler := spacebarToClick(
		func(_ *Layout, _ *Event, _ *Window) {
			called = true
		},
	)
	e := &Event{CharCode: CharSpace}
	handler(nil, e, nil)
	if !called {
		t.Error("spacebar should fire handler")
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}

	// non-spacebar does not fire
	called = false
	e2 := &Event{CharCode: 'a'}
	handler(nil, e2, nil)
	if called {
		t.Error("non-spacebar should not fire handler")
	}
}

func TestLeftClickOnly(t *testing.T) {
	if leftClickOnly(nil) != nil {
		t.Error("nil should return nil")
	}

	called := false
	handler := leftClickOnly(
		func(_ *Layout, _ *Event, _ *Window) {
			called = true
		},
	)

	// left click fires
	e := &Event{MouseButton: MouseLeft}
	handler(nil, e, nil)
	if !called {
		t.Error("left click should fire")
	}

	// right click does not fire
	called = false
	e2 := &Event{MouseButton: MouseRight}
	handler(nil, e2, nil)
	if called {
		t.Error("right click should not fire")
	}
}

func TestEnterToClick(t *testing.T) {
	if enterToClick(nil) != nil {
		t.Error("nil should return nil")
	}

	called := false
	handler := enterToClick(
		func(_ *Layout, _ *Event, _ *Window) {
			called = true
		},
	)

	// enter key fires
	e := &Event{KeyCode: KeyEnter}
	handler(nil, e, nil)
	if !called {
		t.Error("enter should fire handler")
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}

	// non-enter does not fire
	called = false
	e2 := &Event{KeyCode: KeyA}
	handler(nil, e2, nil)
	if called {
		t.Error("non-enter should not fire handler")
	}
}
