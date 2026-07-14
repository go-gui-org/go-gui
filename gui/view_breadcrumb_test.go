package gui

import "testing"

func TestBreadcrumbBasic(t *testing.T) {
	v := Breadcrumb(BreadcrumbCfg{
		ID:       "bc",
		Selected: "b",
		Items: []BreadcrumbItemCfg{
			{ID: "a", Label: "A"},
			{ID: "b", Label: "B"},
			{ID: "c", Label: "C"},
		},
		OnSelect: func(_ string, _ *Event, _ *Window) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	// Outer column has 1 child: trail row (no content views).
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(layout.Children))
	}
}

func TestBreadcrumbWithContent(t *testing.T) {
	v := Breadcrumb(BreadcrumbCfg{
		ID:       "bc",
		Selected: "a",
		Items: []BreadcrumbItemCfg{
			{ID: "a", Label: "A", Content: []View{
				Text(TextCfg{Text: "content A"}),
			}},
			{ID: "b", Label: "B"},
		},
		OnSelect: func(_ string, _ *Event, _ *Window) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	// trail row + content column.
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
}

func TestBcSelectedIndex(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
		{ID: "c", Label: "C"},
	}
	if idx := bcSelectedIndex(items, "b"); idx != 1 {
		t.Errorf("got %d, want 1", idx)
	}
	// Falls back to last enabled.
	if idx := bcSelectedIndex(items, "missing"); idx != 2 {
		t.Errorf("got %d, want 2", idx)
	}
}

func TestBcSelectedIndexDisabled(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B", Disabled: true},
		{ID: "c", Label: "C"},
	}
	// Selecting disabled item falls back.
	if idx := bcSelectedIndex(items, "b"); idx != 2 {
		t.Errorf("got %d, want 2", idx)
	}
}

func TestBcNavigationHelpers(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A", Disabled: true},
		{ID: "b", Label: "B"},
		{ID: "c", Label: "C"},
		{ID: "d", Label: "D", Disabled: true},
	}
	if idx := bcFirstEnabledIndex(items); idx != 1 {
		t.Errorf("first = %d, want 1", idx)
	}
	if idx := bcLastEnabledIndex(items); idx != 2 {
		t.Errorf("last = %d, want 2", idx)
	}
	if idx := bcNextEnabledIndex(items, 1); idx != 2 {
		t.Errorf("next from 1 = %d, want 2", idx)
	}
	// Wrap around.
	if idx := bcNextEnabledIndex(items, 2); idx != 1 {
		t.Errorf("next from 2 = %d, want 1 (wrap)", idx)
	}
	if idx := bcPrevEnabledIndex(items, 2); idx != 1 {
		t.Errorf("prev from 2 = %d, want 1", idx)
	}
	// Wrap around.
	if idx := bcPrevEnabledIndex(items, 1); idx != 2 {
		t.Errorf("prev from 1 = %d, want 2 (wrap)", idx)
	}
}

func TestBcHasAnyContent(t *testing.T) {
	if bcHasAnyContent(nil) {
		t.Error("nil should be false")
	}
	items := []BreadcrumbItemCfg{{ID: "a", Label: "A"}}
	if bcHasAnyContent(items) {
		t.Error("no content should be false")
	}
	items[0].Content = []View{Text(TextCfg{Text: "x"})}
	if !bcHasAnyContent(items) {
		t.Error("has content should be true")
	}
}

func TestBcCrumbID(t *testing.T) {
	id := bcCrumbID("nav", "home")
	if id != "nav:crumb:home" {
		t.Errorf("got %q", id)
	}
}

func TestBcOnKeydown(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
		{ID: "c", Label: "C"},
	}
	var selected string
	onSelect := func(id string, _ *Event, _ *Window) {
		selected = id
	}
	w := &Window{}
	e := &Event{KeyCode: KeyRight}

	bcOnKeydown(false, items, "a", onSelect, "", e, w)
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}
}
