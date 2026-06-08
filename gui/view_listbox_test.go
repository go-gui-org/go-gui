package gui

import "testing"

func TestListBoxIDPassthrough(t *testing.T) {
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID:   "lb1",
		Data: []ListBoxOption{{ID: "a", Name: "A"}},
	}), w)
	if layout.Shape.ID != "lb1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
}

func TestListBoxChildCount(t *testing.T) {
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID: "lb2",
		Data: []ListBoxOption{
			{ID: "a", Name: "Alpha"},
			{ID: "b", Name: "Beta"},
			{ID: "c", Name: "Gamma"},
		},
	}), w)
	if len(layout.Children) != 3 {
		t.Errorf("children: got %d, want 3", len(layout.Children))
	}
}

func TestListBoxSingleSelectClick(t *testing.T) {
	var selected []string
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID: "lb3",
		Data: []ListBoxOption{
			{ID: "a", Name: "Alpha"},
			{ID: "b", Name: "Beta"},
		},
		OnSelect: func(ids []string, _ *Event, _ *Window) {
			selected = ids
		},
	}), w)
	if len(layout.Children) < 1 {
		t.Fatal("expected children")
	}
	item := &layout.Children[0]
	if item.Shape.events != nil && item.Shape.events.OnClick != nil {
		e := &Event{MouseButton: MouseLeft}
		item.Shape.events.OnClick(item, e, w)
		if len(selected) != 1 || selected[0] != "a" {
			t.Errorf("expected [a], got %v", selected)
		}
	}
}

func TestListBoxDisabledFlag(t *testing.T) {
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID:       "lb4",
		Disabled: true,
		Data:     []ListBoxOption{{ID: "a", Name: "A"}},
	}), w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestListBoxItems(t *testing.T) {
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID:    "lb-items",
		Items: []string{"Go", "Rust", "Zig"},
	}), w)
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(layout.Children))
	}
}

func TestListBoxItemsPrecedence(t *testing.T) {
	// Items should take precedence over Data.
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID:    "lb-prec",
		Items: []string{"Alpha", "Beta"},
		Data:  []ListBoxOption{{ID: "ignored", Name: "Ignored"}},
	}), w)
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
}

func TestListBoxItemsSelect(t *testing.T) {
	var selected []string
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID:    "lb-items-sel",
		Items: []string{"Alpha", "Beta"},
		OnSelect: func(ids []string, _ *Event, _ *Window) {
			selected = ids
		},
	}), w)
	if len(layout.Children) < 1 {
		t.Fatal("expected children")
	}
	item := &layout.Children[0]
	if item.Shape.events != nil && item.Shape.events.OnClick != nil {
		e := &Event{MouseButton: MouseLeft}
		item.Shape.events.OnClick(item, e, w)
		if len(selected) != 1 || selected[0] != "Alpha" {
			t.Errorf("expected [Alpha], got %v", selected)
		}
	}
}

func TestListBoxItemsEmptyKeepsData(t *testing.T) {
	// Empty Items (non-nil, zero-length) should not overwrite Data.
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID:    "lb-empty-items",
		Items: []string{},
		Data:  []ListBoxOption{{ID: "a", Name: "Alpha"}},
	}), w)
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1 (Data preserved)", len(layout.Children))
	}
}

func TestListBoxSubheadingCount(t *testing.T) {
	w := &Window{}
	layout := GenerateViewLayout(ListBox(ListBoxCfg{
		ID: "lb5",
		Data: []ListBoxOption{
			{ID: "h1", Name: "Section", IsSubheading: true},
			{ID: "a", Name: "Alpha"},
		},
	}), w)
	if len(layout.Children) != 2 {
		t.Errorf("children: got %d, want 2", len(layout.Children))
	}
}
