package gui

import "testing"

func TestToggleIDPassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Toggle(ToggleCfg{ID: "tg1", OnClick: noop}), w)
	if layout.Shape.ID != "tg1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
}

func TestCheckboxAliasRole(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Toggle(ToggleCfg{ID: "cb1", OnClick: noop}), w)
	if layout.Shape.A11YRole != AccessRoleCheckbox {
		t.Errorf("a11y role: got %d", layout.Shape.A11YRole)
	}
}

func TestToggleSelectedTextContent(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Toggle(ToggleCfg{
		OnClick:      noop,
		Selected:     true,
		TextSelect:   "YES",
		TextUnselect: "NO",
	}), w)
	if len(layout.Children) == 0 {
		t.Fatal("expected children")
	}
	box := layout.Children[0]
	if len(box.Children) == 0 {
		t.Fatal("expected text in box")
	}
	tc := box.Children[0].Shape.TC
	if tc == nil || tc.Text != "YES" {
		got := ""
		if tc != nil {
			got = tc.Text
		}
		t.Errorf("text: got %q, want YES", got)
	}
}

func TestToggleUnselectedTextContent(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Toggle(ToggleCfg{
		OnClick:      noop,
		Selected:     false,
		TextSelect:   "YES",
		TextUnselect: "NO",
	}), w)
	box := layout.Children[0]
	tc := box.Children[0].Shape.TC
	if tc == nil || tc.Text != "NO" {
		got := ""
		if tc != nil {
			got = tc.Text
		}
		t.Errorf("text: got %q, want NO", got)
	}
}

func TestToggleDisabledFlag(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Toggle(ToggleCfg{OnClick: noop, Disabled: true}), w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestToggleLabelAddsChild(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(
		Toggle(ToggleCfg{OnClick: noop, Label: "Accept"}), w)
	if len(layout.Children) < 2 {
		t.Errorf("expected >= 2 children, got %d",
			len(layout.Children))
	}
}
