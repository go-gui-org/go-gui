package gui

import "testing"

func TestSwitchIDPassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "sw1", OnClick: noop}), w)
	// ID is on the inner pill, not outer row.
	if len(layout.Children) == 0 {
		t.Fatal("expected children")
	}
	if layout.Children[0].Shape.ID != "sw1" {
		t.Errorf("ID: got %s", layout.Children[0].Shape.ID)
	}
}

func TestSwitchUnselectedState(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{Selected: false, OnClick: noop}), w)
	if layout.Shape.A11YState != AccessStateNone {
		t.Error("unselected switch should have None state")
	}
}

func TestSwitchOnClickCallback(t *testing.T) {
	fired := false
	w := &Window{}
	v := Switch(SwitchCfg{
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

func TestSwitchDisabledFlag(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{Disabled: true, OnClick: noop}), w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestSwitchLabelAddsChild(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{Label: "Dark Mode", OnClick: noop}), w)
	if len(layout.Children) < 2 {
		t.Errorf("expected >= 2 children with label, got %d",
			len(layout.Children))
	}
}
