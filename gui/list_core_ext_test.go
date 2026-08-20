package gui

import "testing"

// firstTextColor renders a view and returns the color of its first
// text shape.
func firstTextColor(t *testing.T, v View) Color {
	t.Helper()
	layout := generateViewLayout(v, &Window{})
	var color Color
	var walk func(Layout)
	walk = func(l Layout) {
		if l.Shape != nil && l.Shape.shapeType == shapeText &&
			l.Shape.TC != nil && l.Shape.TC.TextStyle != nil {
			color = l.Shape.TC.TextStyle.Color
			return
		}
		for i := range l.Children {
			walk(l.Children[i])
		}
	}
	walk(layout)
	return color
}

// textColorOf renders v with w and returns the color of the text
// shape whose text equals label. The zero Color is returned when
// the label is not found. The window is passed through because
// widget state (open dropdowns, selections) lives on it.
func textColorOf(t *testing.T, w *Window, v View, label string) Color {
	t.Helper()
	layout := generateViewLayout(v, w)
	var color Color
	var walk func(Layout)
	walk = func(l Layout) {
		if color.IsSet() {
			return
		}
		if l.Shape != nil && l.Shape.shapeType == shapeText &&
			l.Shape.TC != nil && l.Shape.TC.TextStyle != nil &&
			l.Shape.TC.Text == label {
			color = l.Shape.TC.TextStyle.Color
			return
		}
		for i := range l.Children {
			walk(l.Children[i])
		}
	}
	walk(layout)
	return color
}

func TestListCoreFilterEmpty(t *testing.T) {
	items := []listCoreItem{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
	}
	result := listCoreFilter(items, "")
	if len(result) != 2 {
		t.Errorf("empty query: got %d indices, want 2", len(result))
	}
}

func TestListCoreFilterMatch(t *testing.T) {
	items := []listCoreItem{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
		{ID: "c", Label: "Gamma"},
	}
	result := listCoreFilter(items, "al")
	if len(result) != 1 || result[0] != 0 {
		t.Errorf("filter 'al': got %v", result)
	}
}

func TestListCoreFilterSkipsSubheadings(t *testing.T) {
	items := []listCoreItem{
		{ID: "h", Label: "Header", isSubheading: true},
		{ID: "a", Label: "Alpha"},
	}
	result := listCoreFilter(items, "hea")
	// Subheading should be skipped; "Alpha" doesn't match "hea".
	if len(result) != 0 {
		t.Errorf("expected 0 matches, got %v", result)
	}
}

func TestListCorePrepare(t *testing.T) {
	items := []listCoreItem{
		{ID: "x", Label: "Exit"},
		{ID: "s", Label: "Save"},
	}
	p := listCorePrepare(items, "", 5)
	if len(p.Items) != 2 {
		t.Errorf("items = %d", len(p.Items))
	}
	if p.HL != 1 {
		t.Errorf("hl = %d, want 1 (clamped from 5)", p.HL)
	}
	if len(p.IDs) != 2 {
		t.Errorf("ids = %d", len(p.IDs))
	}
}

func TestListCorePrepareFiltered(t *testing.T) {
	items := []listCoreItem{
		{ID: "a", Label: "Apple"},
		{ID: "b", Label: "Banana"},
		{ID: "c", Label: "Cherry"},
	}
	p := listCorePrepare(items, "an", 0)
	if len(p.Items) != 1 || p.Items[0].ID != "b" {
		t.Errorf("filtered items = %v", p.Items)
	}
}

func TestListCoreViews(t *testing.T) {
	items := []listCoreItem{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
		{ID: "c", Label: "Gamma"},
	}
	cfg := listCoreCfg{
		TextStyle:      DefaultTextStyle,
		ColorHighlight: Red,
		ColorHover:     Blue,
		PaddingItem:    PaddingSmall,
	}
	views := listCoreViews(items, cfg, 0, 2, 1, nil, 20)
	if len(views) != 3 {
		t.Errorf("views = %d, want 3", len(views))
	}
}

func TestListCoreViewsWithSpacers(t *testing.T) {
	items := make([]listCoreItem, 20)
	for i := range items {
		items[i] = listCoreItem{ID: string(rune('a' + i)), Label: "Item"}
	}
	cfg := listCoreCfg{
		TextStyle:   DefaultTextStyle,
		PaddingItem: PaddingSmall,
	}
	views := listCoreViews(items, cfg, 5, 10, 5, nil, 20)
	// Should have top spacer + 6 items + bottom spacer.
	if len(views) != 8 {
		t.Errorf("views = %d, want 8", len(views))
	}
}

func TestListCoreSubheadingView(t *testing.T) {
	item := listCoreItem{ID: "h", Label: "Group A", isSubheading: true}
	cfg := listCoreCfg{
		TextStyle:       DefaultTextStyle,
		subheadingStyle: DefaultTextStyle,
		PaddingItem:     PaddingSmall,
	}
	views := listCoreViews([]listCoreItem{item}, cfg, 0, 0, -1, nil, 20)
	if len(views) != 1 {
		t.Fatalf("views = %d", len(views))
	}
}

// Highlighted and selected rows paint the subtle wash, never the
// full accent slab (visual-refresh §4.3), so their labels stay in
// the body color — a tint needs no paired foreground. The
// accent/text pairing (issue #373) survives only where the full
// accent fill still happens: menus and the focused widget's own
// chrome.
func TestListCoreItemViewTextOnSelectFill(t *testing.T) {
	item := listCoreItem{ID: "a", Label: "Alpha"}
	cfg := listCoreCfg{
		TextStyle:      DefaultTextStyle,
		ColorHighlight: Blue,
		ColorSelected:  Blue,
		PaddingItem:    PaddingSmall,
	}

	plain := firstTextColor(t, listCoreItemView(item, 0, false, false, cfg))
	if !plain.eq(DefaultTextStyle.Color) {
		t.Errorf("plain row color = %v, want body %v",
			plain, DefaultTextStyle.Color)
	}
	highlighted := firstTextColor(t, listCoreItemView(item, 0, true, false, cfg))
	if !highlighted.eq(DefaultTextStyle.Color) {
		t.Errorf("highlighted row color = %v, want body %v",
			highlighted, DefaultTextStyle.Color)
	}
	selected := firstTextColor(t, listCoreItemView(item, 0, false, true, cfg))
	if !selected.eq(DefaultTextStyle.Color) {
		t.Errorf("selected row color = %v, want body %v",
			selected, DefaultTextStyle.Color)
	}
}
