package gui

import (
	"strconv"
	"testing"
)

func TestRadioButtonGroupColumnBasic(t *testing.T) {
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		Value: "b",
		Options: []RadioOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
			{Label: "C", Value: "c"},
		},
		OnSelect: func(_ string, _ *Window) {},
	})
	kids := v.Content()
	if len(kids) != 3 {
		t.Fatalf("children = %d, want 3", len(kids))
	}
}

func TestRadioButtonGroupRowBasic(t *testing.T) {
	v := RadioButtonGroupRow(RadioButtonGroupCfg{
		Value: "x",
		Options: []RadioOption{
			{Label: "X", Value: "x"},
			{Label: "Y", Value: "y"},
		},
		OnSelect: func(_ string, _ *Window) {},
	})
	kids := v.Content()
	if len(kids) != 2 {
		t.Fatalf("children = %d, want 2", len(kids))
	}
}

func TestRadioButtonGroupFocusIDs(t *testing.T) {
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		Value:     "a",
		ID:        "rbg",
		Focusable: true,
		Options: []RadioOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
			{Label: "C", Value: "c"},
		},
		OnSelect: func(_ string, _ *Window) {},
	})
	w := newTestWindow()
	kids := v.Content()
	if len(kids) != 3 {
		t.Fatalf("children = %d, want 3", len(kids))
	}
	// Each radio gets a per-index focus ID derived from the group ID.
	for i, child := range kids {
		layout := child.GenerateLayout(w)
		expected := "rbg/" + strconv.Itoa(i)
		if !layout.Shape.Focusable {
			t.Errorf("child[%d] not focusable", i)
		}
		if layout.Shape.ID != expected {
			t.Errorf("child[%d] ID = %q, want %q",
				i, layout.Shape.ID, expected)
		}
	}
}

func TestRadioButtonGroupEmpty(t *testing.T) {
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		OnSelect: func(_ string, _ *Window) {},
	})
	if len(v.Content()) != 0 {
		t.Error("empty options should produce no children")
	}
}

func TestRadioButtonGroupOnSelect(t *testing.T) {
	var selected string
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		Value: "a",
		Options: []RadioOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
		},
		OnSelect: func(val string, _ *Window) {
			selected = val
		},
	})
	w := newTestWindow()
	kids := v.Content()
	// Click second radio.
	layout := kids[1].GenerateLayout(w)
	if layout.Shape.hasEvents() && layout.Shape.events.OnClick != nil {
		layout.Shape.events.OnClick(&layout, &Event{}, w)
	}
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
}

func TestRadioButtonGroupDisabledPropagation(t *testing.T) {
	w := newTestWindow()
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		Value:    "a",
		Disabled: true,
		Options: []RadioOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
		},
		OnSelect: func(_ string, _ *Window) {},
	})
	kids := v.Content()
	for i, child := range kids {
		layout := generateViewLayout(child, w)
		// Circle child should be disabled.
		if len(layout.Children) == 0 {
			t.Fatalf("child[%d] has no children", i)
		}
		if !layout.Children[0].Shape.Disabled {
			t.Errorf("child[%d] circle not disabled", i)
		}
	}
}

func TestRadioButtonGroupItems(t *testing.T) {
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		Value:    "rust",
		Items:    []string{"go", "rust", "zig"},
		OnSelect: func(_ string, _ *Window) {},
	})
	kids := v.Content()
	if len(kids) != 3 {
		t.Fatalf("children = %d, want 3", len(kids))
	}
}

func TestRadioButtonGroupItemsPrecedence(t *testing.T) {
	// Items should take precedence over Options.
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		Value: "b",
		Items: []string{"a", "b"},
		Options: []RadioOption{
			{Label: "Ignored", Value: "ignored"},
		},
		OnSelect: func(_ string, _ *Window) {},
	})
	kids := v.Content()
	if len(kids) != 2 {
		t.Fatalf("children = %d, want 2", len(kids))
	}
}

func TestRadioButtonGroupItemsOnSelect(t *testing.T) {
	var selected string
	v := RadioButtonGroupColumn(RadioButtonGroupCfg{
		Value: "a",
		Items: []string{"a", "b"},
		OnSelect: func(val string, _ *Window) {
			selected = val
		},
	})
	w := newTestWindow()
	kids := v.Content()
	layout := kids[1].GenerateLayout(w)
	if layout.Shape.hasEvents() && layout.Shape.events.OnClick != nil {
		layout.Shape.events.OnClick(&layout, &Event{}, w)
	}
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
}

func TestRadioButtonGroupRowItems(t *testing.T) {
	v := RadioButtonGroupRow(RadioButtonGroupCfg{
		Value:    "go",
		Items:    []string{"go", "rust", "zig"},
		OnSelect: func(_ string, _ *Window) {},
	})
	kids := v.Content()
	if len(kids) != 3 {
		t.Fatalf("children = %d, want 3", len(kids))
	}
}

func TestNewRadioOption(t *testing.T) {
	opt := NewRadioOption("Go", "go")
	if opt.Label != "Go" || opt.Value != "go" {
		t.Errorf("got %+v", opt)
	}
}
