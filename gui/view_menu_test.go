package gui

import "testing"

func TestMenuItemTextFactory(t *testing.T) {
	item := MenuItemText("file", "File")
	if item.ID != "file" {
		t.Errorf("ID = %q, want file", item.ID)
	}
	if item.Text != "File" {
		t.Errorf("Text = %q, want File", item.Text)
	}
	if item.Separator {
		t.Error("should not be separator")
	}
}

func TestMenuSeparatorFactory(t *testing.T) {
	item := MenuSeparator()
	if item.ID != menuSeparatorID {
		t.Errorf("ID = %q, want %q", item.ID, menuSeparatorID)
	}
	if !item.Separator {
		t.Error("should be separator")
	}
}

func TestMenuSubtitleFactory(t *testing.T) {
	item := MenuSubtitle("Section")
	if item.ID != menuSubtitleID {
		t.Errorf("ID = %q, want %q", item.ID, menuSubtitleID)
	}
	if item.Text != "Section" {
		t.Errorf("Text = %q", item.Text)
	}
	if !item.disabled {
		t.Error("subtitle should be disabled")
	}
}

func TestMenuSubmenuFactory(t *testing.T) {
	sub := []MenuItemCfg{MenuItemText("a", "A")}
	item := MenuSubmenu("parent", "Parent", sub)
	if item.ID != "parent" {
		t.Errorf("ID = %q", item.ID)
	}
	if len(item.Submenu) != 1 {
		t.Fatalf("Submenu len = %d", len(item.Submenu))
	}
	if item.Submenu[0].ID != "a" {
		t.Error("submenu child ID mismatch")
	}
}

func TestDuplicateMenuIDPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate menu item ID")
		}
	}()
	checkForDuplicateMenuIDs([]MenuItemCfg{
		{ID: "dup"},
		{ID: "dup"},
	})
}

func TestIsSelectableMenuID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"file", true},
		{menuSeparatorID, false},
		{menuSubtitleID, false},
		{"", true},
	}
	for _, tc := range tests {
		got := isSelectableMenuID(tc.id)
		if got != tc.want {
			t.Errorf("isSelectableMenuID(%q) = %v, want %v",
				tc.id, got, tc.want)
		}
	}
}

func TestMenuMapper(t *testing.T) {
	items := []MenuItemCfg{
		MenuSubmenu("file", "File", []MenuItemCfg{
			MenuItemText("new", "New"),
			MenuSeparator(),
			MenuItemText("open", "Open"),
		}),
		MenuItemText("edit", "Edit"),
		MenuItemText("view", "View"),
	}
	m := menuMapper(items)

	// Root-level left/right wrap.
	if m["file"].Right != "edit" {
		t.Errorf("file.Right = %q", m["file"].Right)
	}
	if m["view"].Right != "file" {
		t.Errorf("view.Right = %q (wrap)", m["view"].Right)
	}
	if m["file"].Left != "view" {
		t.Errorf("file.Left = %q (wrap)", m["file"].Left)
	}

	// Root down enters submenu.
	if m["file"].Down != "new" {
		t.Errorf("file.Down = %q, want new",
			m["file"].Down)
	}

	// Submenu vertical nav.
	if m["new"].Down != "open" {
		t.Errorf("new.Down = %q, want open",
			m["new"].Down)
	}
	if m["open"].Up != "new" {
		t.Errorf("open.Up = %q, want new",
			m["open"].Up)
	}

	// Submenu left goes to parent.
	if m["new"].Left != "file" {
		t.Errorf("new.Left = %q, want file",
			m["new"].Left)
	}
}

func TestFindMenuItemCfg(t *testing.T) {
	items := []MenuItemCfg{
		MenuSubmenu("file", "File", []MenuItemCfg{
			MenuItemText("new", "New"),
		}),
		MenuItemText("edit", "Edit"),
	}

	if _, ok := findMenuItemCfg(items, "new"); !ok {
		t.Error("should find nested item")
	}
	if _, ok := findMenuItemCfg(items, "edit"); !ok {
		t.Error("should find top-level item")
	}
	if _, ok := findMenuItemCfg(items, "missing"); ok {
		t.Error("should not find missing item")
	}
}

func TestMenuItemHasID(t *testing.T) {
	cfg := MenubarCfg{
		ID: "mb",
		Items: []MenuItemCfg{
			MenuItemText("file", "File"),
			MenuItemText("edit", "Edit"),
		},
	}
	applyMenubarDefaults(&cfg)
	views := menuBuild(cfg, 0, cfg.Items, &Window{})
	for _, v := range views {
		layout := generateViewLayout(v, &Window{})
		if layout.Shape.ID == "" {
			t.Error("menu item should have ID set")
		}
	}
}

func TestIsMenuIDInTree(t *testing.T) {
	items := []MenuItemCfg{
		MenuSubmenu("a", "A", []MenuItemCfg{
			MenuItemText("b", "B"),
		}),
	}
	if !isMenuIDInTree(items, "b") {
		t.Error("should find b in tree")
	}
	if isMenuIDInTree(items, "c") {
		t.Error("should not find c in tree")
	}
}
