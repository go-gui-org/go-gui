package gui

import "testing"

func TestTabControlBasic(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:       "tabs",
		Selected: "b",
		Items: []TabItemCfg{
			{ID: "a", Label: "A"},
			{ID: "b", Label: "B"},
			{ID: "c", Label: "C"},
		},
		OnSelect: func(_ string, _ *Event, _ *Window) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	// Outer column: header row + content column.
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
	// Header row has 3 buttons.
	header := layout.Children[0]
	if len(header.Children) != 3 {
		t.Errorf("header children = %d, want 3",
			len(header.Children))
	}
}

func TestTabsAlias(t *testing.T) {
	v := Tabs(TabControlCfg{
		ID:       "tabs",
		Items:    []TabItemCfg{{ID: "a", Label: "A"}},
		OnSelect: func(_ string, _ *Event, _ *Window) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
}

func TestTabSelectedIndex(t *testing.T) {
	ids := []string{"a", "b", "c"}
	disabled := []bool{false, false, false}
	if idx := tabSelectedIndex(ids, disabled, "b"); idx != 1 {
		t.Errorf("got %d, want 1", idx)
	}
	// Missing falls back to first.
	if idx := tabSelectedIndex(ids, disabled, "z"); idx != 0 {
		t.Errorf("got %d, want 0", idx)
	}
}

func TestTabNavigationHelpers(t *testing.T) {
	disabled := []bool{true, false, false, true}
	if idx := tabFirstEnabledIndex(disabled); idx != 1 {
		t.Errorf("first = %d, want 1", idx)
	}
	if idx := tabLastEnabledIndex(disabled); idx != 2 {
		t.Errorf("last = %d, want 2", idx)
	}
	if idx := tabNextEnabledIndex(disabled, 1); idx != 2 {
		t.Errorf("next from 1 = %d, want 2", idx)
	}
	if idx := tabNextEnabledIndex(disabled, 2); idx != 1 {
		t.Errorf("next from 2 = %d, want 1 (wrap)", idx)
	}
	if idx := tabPrevEnabledIndex(disabled, 2); idx != 1 {
		t.Errorf("prev from 2 = %d, want 1", idx)
	}
	if idx := tabPrevEnabledIndex(disabled, 1); idx != 2 {
		t.Errorf("prev from 1 = %d, want 2 (wrap)", idx)
	}
}

func TestTabControlOnKeydown(t *testing.T) {
	ids := []string{"a", "b", "c"}
	disabled := []bool{false, false, false}
	var selected string
	onSelect := func(id string, _ *Event, _ *Window) {
		selected = id
	}
	w := &Window{}
	e := &Event{KeyCode: KeyRight}
	tabControlOnKeydown(false, ids, disabled, "a", onSelect, 0, e, w)
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}
}

func TestTabButtonID(t *testing.T) {
	id := tabButtonID("main", "settings")
	if id != "tc_main_settings" {
		t.Errorf("got %q", id)
	}
}

func TestNewTabItem(t *testing.T) {
	item := NewTabItem("t1", "Tab 1", nil)
	if item.ID != "t1" || item.Label != "Tab 1" {
		t.Errorf("got %+v", item)
	}
}

func TestTabControlDisabled(t *testing.T) {
	ids := []string{"a", "b", "c"}
	disabled := []bool{false, false, false}
	var selected string
	onSelect := func(id string, _ *Event, _ *Window) {
		selected = id
	}
	w := &Window{}
	e := &Event{KeyCode: KeyRight}
	tabControlOnKeydown(true, ids, disabled, "a", onSelect, 0, e, w)
	if selected != "" {
		t.Errorf("expected no selection, got %q", selected)
	}
	if e.IsHandled {
		t.Error("event should not be handled when disabled")
	}
}

func TestTabControlEmptyItems(t *testing.T) {
	v := TabControl(TabControlCfg{ID: "tabs"})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
	header := layout.Children[0]
	if len(header.Children) != 0 {
		t.Errorf("header children = %d, want 0",
			len(header.Children))
	}
}

func TestTabControlAllDisabledItems(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID: "tabs",
		Items: []TabItemCfg{
			{ID: "a", Label: "A", Disabled: true},
			{ID: "b", Label: "B", Disabled: true},
		},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	content := layout.Children[1]
	if len(content.Children) != 0 {
		t.Errorf("content children = %d, want 0",
			len(content.Children))
	}
}

func TestTabNavEmptySlice(t *testing.T) {
	if idx := tabNextEnabledIndex(nil, 0); idx != -1 {
		t.Errorf("next on empty = %d, want -1", idx)
	}
	if idx := tabPrevEnabledIndex(nil, 0); idx != -1 {
		t.Errorf("prev on empty = %d, want -1", idx)
	}
	if idx := tabFirstEnabledIndex(nil); idx != -1 {
		t.Errorf("first on empty = %d, want -1", idx)
	}
	if idx := tabLastEnabledIndex(nil); idx != -1 {
		t.Errorf("last on empty = %d, want -1", idx)
	}
}

func TestTabNavAllDisabled(t *testing.T) {
	disabled := []bool{true, true, true}
	if idx := tabNextEnabledIndex(disabled, 0); idx != -1 {
		t.Errorf("next = %d, want -1", idx)
	}
	if idx := tabPrevEnabledIndex(disabled, 0); idx != -1 {
		t.Errorf("prev = %d, want -1", idx)
	}
}

func TestTabNavOutOfBounds(t *testing.T) {
	disabled := []bool{false, false}
	if idx := tabNextEnabledIndex(disabled, 99); idx != 0 {
		t.Errorf("next from 99 = %d, want 0", idx)
	}
	if idx := tabPrevEnabledIndex(disabled, -5); idx != 1 {
		t.Errorf("prev from -5 = %d, want 1", idx)
	}
}

func TestTabSelectedContent(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:       "tabs",
		Selected: "b",
		Items: []TabItemCfg{
			{ID: "a", Label: "A", Content: []View{
				Text(TextCfg{Text: "A"}),
			}},
			{ID: "b", Label: "B", Content: []View{
				Text(TextCfg{Text: "B1"}),
				Text(TextCfg{Text: "B2"}),
			}},
		},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	content := layout.Children[1]
	if len(content.Children) != 2 {
		t.Errorf("content children = %d, want 2",
			len(content.Children))
	}
}

func TestTabControlReorderable(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:          "tabs",
		Selected:    "a",
		Reorderable: true,
		Items: []TabItemCfg{
			{ID: "a", Label: "A"},
			{ID: "b", Label: "B"},
		},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
	header := layout.Children[0]
	if len(header.Children) != 2 {
		t.Errorf("header children = %d, want 2",
			len(header.Children))
	}
}

func TestTabOptZeroOverride(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:         "tabs",
		Selected:   "a",
		SizeBorder: NoBorder,
		Items: []TabItemCfg{
			{ID: "a", Label: "A"},
		},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("SizeBorder = %v, want 0",
			layout.Shape.SizeBorder)
	}
}
