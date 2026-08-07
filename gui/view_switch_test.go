package gui

import "testing"

func TestSwitchIDPassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "sw1", OnClick: noop}), w)
	// ID lives on the focusable outer row only; the inner pill must
	// not duplicate it.
	if layout.Shape.ID != "sw1" {
		t.Errorf("outer ID: got %s", layout.Shape.ID)
	}
	if len(layout.Children) == 0 {
		t.Fatal("expected children")
	}
	if layout.Children[0].Shape.ID != "" {
		t.Errorf("pill ID: got %s, want empty",
			layout.Children[0].Shape.ID)
	}
}

func TestSwitchUnselectedState(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "switch_test_test_switch_unselected_state", Selected: false, OnClick: noop}), w)
	if layout.Shape.A11YState != AccessStateNone {
		t.Error("unselected switch should have None state")
	}
}

func TestSwitchOnClickCallback(t *testing.T) {
	fired := false
	w := &Window{}
	v := Switch(SwitchCfg{
		ID: "switch_test_test_switch_on_click_callback",
		OnClick: func(ctx EventCtx) {
			fired = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.OnClick == nil {
		t.Fatal("expected OnClick")
	}
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(EventCtx{&layout, e, w})
	if !fired {
		t.Error("OnClick did not fire")
	}
}

func TestSwitchDisabledFlag(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "switch_test_test_switch_disabled_flag", Disabled: true, OnClick: noop}), w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestSwitchLabelAddsChild(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Switch(SwitchCfg{ID: "switch_test_test_switch_label_adds_child", Label: "Dark Mode", OnClick: noop}), w)
	if len(layout.Children) < 2 {
		t.Errorf("expected >= 2 children with label, got %d",
			len(layout.Children))
	}
}
