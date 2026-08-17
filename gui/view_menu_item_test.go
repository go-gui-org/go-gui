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

// menuItemTextYs renders one menu item through the real frame pipeline
// and returns the arranged Y of every text shape under it, in order. It
// is the number optical centring moves, and the only way to see it: the
// hook runs in AmendLayout, after arrange, so a bare GenerateLayout
// shows nothing.
func menuItemTextYs(t *testing.T, itemCfg MenuItemCfg) []float32 {
	t.Helper()
	itemCfg.textStyle = DefaultTextStyle
	itemCfg.sizing = FitFit
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return menuItem(MenubarCfg{}, itemCfg)
	})
	item, ok := w.layout.FindByID(itemCfg.ID)
	if !ok {
		t.Fatal("no menu item in the rendered window")
	}
	var ys []float32
	var walk func(l *Layout)
	walk = func(l *Layout) {
		if l.Shape != nil && l.Shape.shapeType == shapeText {
			ys = append(ys, l.Shape.Y)
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	walk(item)
	return ys
}

// A menu is a list at a regular pitch, so its items are corrected on the
// face's cap band rather than each label's own ink: a descending label
// and a cap-only one must take the same offset, or the baselines step
// down the menu (issue #346).
func TestMenuItemOpticalCenterDoesNotFollowContent(t *testing.T) {
	base := menuItemTextYs(t, MenuItemText("m", "PICK"))
	if len(base) != 1 {
		t.Fatalf("want one text shape, got %d", len(base))
	}
	for _, label := range []string{
		"Copy",  // descends
		"gypsy", // descends, no caps
		"Paste",
	} {
		got := menuItemTextYs(t, MenuItemText("m", label))
		if len(got) != 1 || got[0] != base[0] {
			t.Errorf("label %q: y=%v, want %v", label, got, base[0])
		}
	}
}

// A CustomView item is the uncorrected reference: its content is the
// app's to place, so the hook is not attached at all.
func TestMenuItemOpticalCenterIsApplied(t *testing.T) {
	uncorrected := menuItemTextYs(t, MenuItemCfg{
		ID: "m",
		CustomView: Text(TextCfg{
			Text: "PICK", TextStyle: DefaultTextStyle,
		}),
	})
	corrected := menuItemTextYs(t, MenuItemText("m", "PICK"))
	if len(uncorrected) != 1 || len(corrected) != 1 {
		t.Fatalf("want one text shape each, got %d and %d",
			len(uncorrected), len(corrected))
	}
	if corrected[0] <= uncorrected[0] {
		t.Errorf("corrected y %v not below uncorrected %v",
			corrected[0], uncorrected[0])
	}
}

// The label and its shortcut hint sit in one row and must move together;
// the correction is attached to that row rather than to the item, or the
// pair would read skewed.
func TestMenuItemShortcutHintMovesWithLabel(t *testing.T) {
	item := MenuItemText("m", "Copy")
	item.level = 1
	item.shortcutText = "Ctrl+C"
	ys := menuItemTextYs(t, item)
	if len(ys) != 2 {
		t.Fatalf("want label and hint, got %d text shapes", len(ys))
	}
	if ys[0] != ys[1] {
		t.Errorf("label y %v, hint y %v: must match", ys[0], ys[1])
	}
	plain := menuItemTextYs(t, MenuItemText("m", "Copy"))
	if ys[0] <= plain[0]-1 || ys[0] >= plain[0]+1 {
		t.Errorf("row-corrected y %v differs from item-corrected %v",
			ys[0], plain[0])
	}
}
