package gui

import "testing"

func TestMenubarLayout(t *testing.T) {
	w := &Window{}
	cfg := MenubarCfg{
		ID: "mb",
		Items: []MenuItemCfg{
			MenuItemText("file", "File"),
			MenuItemText("edit", "Edit"),
		},
	}
	view := Menubar(w, cfg)
	layout := generateViewLayout(view, w)

	if layout.Shape == nil {
		t.Fatal("nil shape")
	}
	if layout.Shape.ID != "mb" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
	if layout.Shape.Axis != AxisLeftToRight {
		t.Errorf("axis = %d, want LeftToRight", layout.Shape.Axis)
	}
	// Should have at least 2 children (one per item).
	if len(layout.Children) < 2 {
		t.Errorf("children = %d, want >= 2",
			len(layout.Children))
	}
}

func TestMenubarKeydownEscape(t *testing.T) {
	w := &Window{}
	w.viewState.focusID = "mb"
	sm := StateMap[string, string](w, nsMenu, capModerate)
	sm.Set("mb", "file")

	cfg := MenubarCfg{
		ID: "mb",
		Items: []MenuItemCfg{
			MenuItemText("file", "File"),
		},
	}

	e := &Event{Type: EventKeyDown, KeyCode: KeyEscape}
	menuOnKeyDown(cfg, menuMapper, e, w)

	if e.IsHandled != true {
		t.Error("escape should be handled")
	}
	if w.viewState.focusID != "" {
		t.Error("focus should be cleared")
	}
	sel, _ := sm.Get("mb")
	if sel != "" {
		t.Errorf("selection = %q, want empty", sel)
	}
}

func TestMenubarKeydownNavigation(t *testing.T) {
	w := &Window{}
	w.viewState.focusID = "mb"
	sm := StateMap[string, string](w, nsMenu, capModerate)
	sm.Set("mb", "file")

	cfg := MenubarCfg{
		ID: "mb",
		Items: []MenuItemCfg{
			MenuItemText("file", "File"),
			MenuItemText("edit", "Edit"),
			MenuItemText("view", "View"),
		},
	}

	// Right arrow: file -> edit.
	e := &Event{Type: EventKeyDown, KeyCode: KeyRight}
	menuOnKeyDown(cfg, menuMapper, e, w)
	sel, _ := sm.Get("mb")
	if sel != "edit" {
		t.Errorf("after Right: sel = %q, want edit", sel)
	}

	// Left arrow: edit -> file.
	e = &Event{Type: EventKeyDown, KeyCode: KeyLeft}
	menuOnKeyDown(cfg, menuMapper, e, w)
	sel, _ = sm.Get("mb")
	if sel != "file" {
		t.Errorf("after Left: sel = %q, want file", sel)
	}
}

func TestMenubarAmendLayoutClearOnDefocus(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, string](w, nsMenu, capModerate)
	sm.Set("mb", "file")

	amend := makeMenuAmendLayout("mb")
	layout := &Layout{Shape: &Shape{}}
	amend(layout, w)

	sel, ok := sm.Get("mb")
	if ok && sel != "" {
		t.Errorf("should clear selection when defocused, got %q", sel)
	}
}

func TestApplyMenubarDefaultsSpacingSubmenu(t *testing.T) {
	cfg := MenubarCfg{}
	applyMenubarDefaults(&cfg)
	if !cfg.SpacingSubmenu.IsSet() {
		t.Fatal("SpacingSubmenu should be set after defaults")
	}
	got := cfg.SpacingSubmenu.Get(0)
	want := DefaultMenubarStyle.SpacingSubmenu
	if got != want {
		t.Errorf("SpacingSubmenu = %v, want %v", got, want)
	}
}

func TestFindMenuByID(t *testing.T) {
	items := []MenuItemCfg{
		MenuSubmenu("a", "A", []MenuItemCfg{
			MenuItemText("b", "B"),
		}),
		MenuItemText("c", "C"),
	}
	item, ok := findMenuItemCfg(items, "b")
	if !ok {
		t.Fatal("should find b")
	}
	if item.Text != "B" {
		t.Errorf("Text = %q", item.Text)
	}
	_, ok = findMenuItemCfg(items, "z")
	if ok {
		t.Error("should not find z")
	}
}
