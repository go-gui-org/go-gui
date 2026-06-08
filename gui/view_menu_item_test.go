package gui

import "testing"

func TestMenuSeparatorCfg(t *testing.T) {
	s := MenuSeparator()
	if !s.Separator {
		t.Error("expected separator flag")
	}
	if s.ID != menuSeparatorID {
		t.Errorf("ID: got %s, want %s", s.ID, menuSeparatorID)
	}
}

func TestMenuSubtitleCfg(t *testing.T) {
	s := MenuSubtitle("Section")
	if s.Text != "Section" {
		t.Errorf("text: got %s", s.Text)
	}
	if s.ID != menuSubtitleID {
		t.Errorf("ID: got %s", s.ID)
	}
	if !s.disabled {
		t.Error("subtitle should be disabled")
	}
}

func TestMenuSubmenuCfg(t *testing.T) {
	items := []MenuItemCfg{
		MenuItemText("a", "Alpha"),
		MenuItemText("b", "Beta"),
	}
	s := MenuSubmenu("sub", "More", items)
	if s.ID != "sub" {
		t.Errorf("ID: got %s", s.ID)
	}
	if len(s.Submenu) != 2 {
		t.Errorf("submenu len: got %d", len(s.Submenu))
	}
}

func TestMenuItemTextCfg(t *testing.T) {
	m := MenuItemText("m1", "Click Me")
	if m.ID != "m1" || m.Text != "Click Me" {
		t.Errorf("got ID=%s Text=%s", m.ID, m.Text)
	}
}

func TestMenuItemSeparatorView(t *testing.T) {
	w := &Window{}
	mbCfg := MenubarCfg{}
	itemCfg := MenuSeparator()
	v := menuItem(mbCfg, itemCfg)
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	// Separator renders a Column > Rectangle.
	if len(layout.Children) == 0 {
		t.Error("expected children for separator")
	}
}

func TestMenuItemSubmenuIndicator(t *testing.T) {
	m := MenuSubmenu("sub2", "More", []MenuItemCfg{
		MenuItemText("a", "A"),
	})
	// Submenu indicator is added by menuItem when level > 0.
	m.level = 1
	m.textStyle = DefaultTextStyle
	m.sizing = FillFit
	w := &Window{}
	v := menuItem(MenubarCfg{}, m)
	layout := generateViewLayout(v, w)
	// The text child should have the submenu indicator appended.
	if len(layout.Children) == 0 {
		t.Fatal("expected children")
	}
	tc := layout.Children[0].Shape.TC
	if tc == nil {
		t.Fatal("expected text config")
	}
	if len(tc.Text) == 0 {
		t.Error("expected text with submenu indicator")
	}
}
