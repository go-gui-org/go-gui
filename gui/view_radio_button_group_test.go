package gui

import (
	"testing"
)

func TestRadioButtonGroupColumnBasic(t *testing.T) {
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:    "radio_button_group_test_test_radio_button_group_column_basic",
		Value: "b",
		Options: []RadioOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
			{Label: "C", Value: "c"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, newTestWindow())
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(layout.Children))
	}
}

func TestRadioButtonGroupRowBasic(t *testing.T) {
	v := RadioButtonGroupRow(RadioButtonGroupCfg{
		ID:    "radio_button_group_test_test_radio_button_group_row_basic",
		Value: "x",
		Options: []RadioOption{
			{Label: "X", Value: "x"},
			{Label: "Y", Value: "y"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, newTestWindow())
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
}

func TestRadioButtonGroupFocusIDs(t *testing.T) {
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		Value: "a",
		ID:    "rbg",
		Options: []RadioOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
			{Label: "C", Value: "c"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	w := newTestWindow()
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(layout.Children))
	}
	// Each radio gets a per-index focus ID derived from the group ID.
	for i, child := range layout.Children {
		expected := ScopeIDN("rbg", "opt", i)
		if !child.Shape.Focusable {
			t.Errorf("child[%d] not focusable", i)
		}
		if child.Shape.ID != expected {
			t.Errorf("child[%d] ID = %q, want %q",
				i, child.Shape.ID, expected)
		}
	}
}

func TestRadioButtonGroupEmpty(t *testing.T) {
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:       "radio_button_group_test_test_radio_button_group_empty",
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, newTestWindow())
	if len(layout.Children) != 0 {
		t.Error("empty options should produce no children")
	}
}

func TestRadioButtonGroupOnSelect(t *testing.T) {
	var selected string
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:    "radio_button_group_test_test_radio_button_group_on_select",
		Value: "a",
		Options: []RadioOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
		},
		OnSelect: func(val string, ctx EventCtx) {
			selected = val
		},
	})
	w := newTestWindow()
	layout := generateViewLayout(v, w)
	// Click second radio.
	second := &layout.Children[1]
	if second.Shape.hasEvents() && second.Shape.events.OnClick != nil {
		second.Shape.events.OnClick(EventCtx{second, &Event{}, w})
	}
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
}

func TestRadioButtonGroupDisabledPropagation(t *testing.T) {
	w := newTestWindow()
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:       "radio_button_group_test_test_radio_button_group_disabled_propagation",
		Value:    "a",
		Disabled: true,
		Options: []RadioOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	for i, child := range layout.Children {
		// Circle child should be disabled.
		if len(child.Children) == 0 {
			t.Fatalf("child[%d] has no children", i)
		}
		if !child.Children[0].Shape.Disabled {
			t.Errorf("child[%d] circle not disabled", i)
		}
	}
}

func TestRadioButtonGroupItems(t *testing.T) {
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:       "radio_button_group_test_test_radio_button_group_items",
		Value:    "rust",
		Items:    []string{"go", "rust", "zig"},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, newTestWindow())
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(layout.Children))
	}
}

func TestRadioButtonGroupItemsPrecedence(t *testing.T) {
	// Items should take precedence over Options.
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:    "radio_button_group_test_test_radio_button_group_items_precedence",
		Value: "b",
		Items: []string{"a", "b"},
		Options: []RadioOption{
			{Label: "Ignored", Value: "ignored"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, newTestWindow())
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
}

func TestRadioButtonGroupItemsOnSelect(t *testing.T) {
	var selected string
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:    "radio_button_group_test_test_radio_button_group_items_on_select",
		Value: "a",
		Items: []string{"a", "b"},
		OnSelect: func(val string, ctx EventCtx) {
			selected = val
		},
	})
	w := newTestWindow()
	layout := generateViewLayout(v, w)
	second := &layout.Children[1]
	if second.Shape.hasEvents() && second.Shape.events.OnClick != nil {
		second.Shape.events.OnClick(EventCtx{second, &Event{}, w})
	}
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
}

func TestRadioButtonGroupRowItems(t *testing.T) {
	v := RadioButtonGroupRow(RadioButtonGroupCfg{
		ID:       "radio_button_group_test_test_radio_button_group_row_items",
		Value:    "go",
		Items:    []string{"go", "rust", "zig"},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, newTestWindow())
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(layout.Children))
	}
}

func TestNewRadioOption(t *testing.T) {
	opt := NewRadioOption("Go", "go")
	if opt.Label != "Go" || opt.Value != "go" {
		t.Errorf("got %+v", opt)
	}
}

// TestRadioButtonGroupBorderFollowsTheme covers issue #300: the group
// box border used to resolve against a private 1.5px literal that no
// theme could reach. An unset cfg.SizeBorder must fall back to the
// themed container style, and an explicit value must still win.
func TestRadioButtonGroupBorderFollowsTheme(t *testing.T) {
	restoreTheme(t)
	th := ThemeDark.withContainerStyle(containerStyle{SizeBorder: 7})
	SetTheme(th)

	w := &Window{}
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID: "radio-group-theme-border",
		Options: []RadioOption{
			{Label: "A", Value: "a"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 7 {
		t.Errorf("size_border = %v, want 7 (container style)", layout.Shape.SizeBorder)
	}

	v = RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:         "radio-group-theme-border-override",
		SizeBorder: SomeF(3),
		Options: []RadioOption{
			{Label: "A", Value: "a"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	layout = generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 3 {
		t.Errorf("size_border = %v, want 3 (cfg wins)", layout.Shape.SizeBorder)
	}
}

// Sibling options in a radio group are separate controls stacked under
// the group box, so the default gap is the Medium tier — not Small,
// which is for members of one visual group (issue #344, audit §4).
func TestRadioButtonGroupDefaultSpacingMedium(t *testing.T) {
	cfg := RadioButtonGroupCfg{}
	applyRadioGroupDefaults(&cfg)
	if got := cfg.Spacing.Get(0); got != SpacingMedium {
		t.Errorf("default Spacing = %v, want %v (SpacingMedium)", got, SpacingMedium)
	}
}

// A titled radio group is a group box: an unset ColorBorder resolves
// the group-box ink through the container (gui/view_container.go), not
// the hairline wash.
func TestTitledRadioGroupResolvesGroupBoxInk(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)

	w := &Window{}
	layout := generateViewLayout(RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:       "radio_group_test_titled_ink",
		Title:    "Choose",
		Options:  []RadioOption{{Label: "A", Value: "a"}},
		OnSelect: func(_ string, _ EventCtx) {},
	}), w)
	if layout.Shape.ColorBorder != groupBoxInk() {
		t.Errorf("border = %v, want group-box ink %v",
			layout.Shape.ColorBorder, groupBoxInk())
	}
}

// An untitled radio group is not a group box: the border stays unset
// and follows the container defaults like any other container.
func TestUntitledRadioGroupLeavesBorderUnset(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark)

	w := &Window{}
	layout := generateViewLayout(RadioButtonGroupColumn(RadioButtonGroupCfg{
		ID:       "radio_group_test_untitled_unset",
		Options:  []RadioOption{{Label: "A", Value: "a"}},
		OnSelect: func(_ string, _ EventCtx) {},
	}), w)
	if layout.Shape.ColorBorder.IsSet() {
		t.Errorf("border = %v, want unset", layout.Shape.ColorBorder)
	}
}
