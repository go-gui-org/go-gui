package gui

import "testing"

func TestRadioIDPassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r1", Label: "A"}), w)
	if layout.Shape.ID != "r1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
}

func TestRadioUnselectedStateNone(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r2", Selected: false, OnClick: noop}), w)
	if layout.Shape.A11YState != AccessStateNone {
		t.Error("unselected radio should have None state")
	}
}

func TestRadioOnClickCallback(t *testing.T) {
	fired := false
	w := &Window{}
	v := Radio(RadioCfg{
		ID: "r3",
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			fired = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.OnClick == nil {
		t.Fatal("expected OnClick")
	}
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(&layout, e, w)
	if !fired {
		t.Error("OnClick did not fire")
	}
}

func TestRadioFocusablePassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Radio(RadioCfg{ID: "r4", Focusable: true, OnClick: noop}), w)
	if !layout.Shape.Focusable {
		t.Error("Focusable: want true")
	}
	if layout.Shape.ID != "r4" {
		t.Errorf("ID: got %q, want r4", layout.Shape.ID)
	}
}
