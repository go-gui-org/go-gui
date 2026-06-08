package gui

import "testing"

func TestButtonGeneratesLayout(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b1"})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	if layout.Shape.ID != "b1" {
		t.Errorf("ID: got %s, want b1", layout.Shape.ID)
	}
	if layout.Shape.A11YRole != AccessRoleButton {
		t.Errorf("a11y role: got %d, want %d",
			layout.Shape.A11YRole, AccessRoleButton)
	}
}

func TestButtonOnClickFires(t *testing.T) {
	fired := false
	w := &Window{}
	v := Button(ButtonCfg{
		ID: "b2",
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			fired = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.OnClick == nil {
		t.Fatal("expected OnClick handler")
	}
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(&layout, e, w)
	if !fired {
		t.Error("OnClick did not fire")
	}
}

func TestButtonDisabledFlag(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b3", Disabled: true})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestButtonIDFocus(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b4", IDFocus: 42})
	layout := generateViewLayout(v, w)
	if layout.Shape.IDFocus != 42 {
		t.Errorf("IDFocus: got %d, want 42", layout.Shape.IDFocus)
	}
}

func TestButtonWithContent(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{
		ID:      "b5",
		Content: []View{Text(TextCfg{Text: "Click"})},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) == 0 {
		t.Error("expected children from content")
	}
}

func TestButtonNoOnClickNoHandler(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b6"})
	layout := generateViewLayout(v, w)
	if layout.Shape.events != nil &&
		layout.Shape.events.OnClick != nil {
		t.Error("expected no OnClick without handler")
	}
}
