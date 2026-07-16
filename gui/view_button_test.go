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

func TestButtonFocusable(t *testing.T) {
	w := &Window{}
	v := Button(ButtonCfg{ID: "b4"})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Focusable {
		t.Error("Focusable: want true")
	}
	if layout.Shape.ID != "b4" {
		t.Errorf("ID: got %q, want b4", layout.Shape.ID)
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

func TestButtonAmendLayoutChains(t *testing.T) {
	w := &Window{}
	called := false
	v := Button(ButtonCfg{
		ID:      "b7",
		OnClick: func(_ *Layout, _ *Event, _ *Window) {},
		AmendLayout: func(_ *Layout, _ *Window) {
			called = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.AmendLayout == nil {
		t.Fatal("expected AmendLayout handler")
	}
	layout.Shape.events.AmendLayout(&layout, w)
	if !called {
		t.Error("AmendLayout did not fire")
	}
}

func TestButtonAmendLayoutNotCalledWhenDisabled(t *testing.T) {
	w := &Window{}
	called := false
	v := Button(ButtonCfg{
		ID:       "b8",
		Disabled: true,
		OnClick:  func(_ *Layout, _ *Event, _ *Window) {},
		AmendLayout: func(_ *Layout, _ *Window) {
			called = true
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.AmendLayout == nil {
		t.Fatal("expected AmendLayout handler")
	}
	layout.Shape.events.AmendLayout(&layout, w)
	if called {
		t.Error("AmendLayout should not fire when disabled")
	}
}

func TestButtonAmendLayoutSuppressedWhenNoOnClick(t *testing.T) {
	// When OnClick is nil, the button creates no event handlers at
	// all — AmendLayout cannot fire because it's never wired.
	w := &Window{}
	v := Button(ButtonCfg{
		ID:          "b9",
		AmendLayout: func(_ *Layout, _ *Window) {},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.events != nil {
		t.Error("expected nil events when OnClick is nil")
	}
}
