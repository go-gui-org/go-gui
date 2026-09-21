package gui

import "testing"

// Tests for ListBox rows routed through ColorSet.pick (#744).

// The selected row paints Colors.Selected, which the theme sets to
// the subtle wash, never the full accent slab (visual-refresh §4.3).
// A caller-set Colors.Selected wins over the theme. A caller-set
// ColorSelect keeps its old meaning: it becomes the selected fill
// when Colors.Selected is unset.
func TestListBoxSelectedDefaults(t *testing.T) {
	cfg := ListBoxCfg{ID: "lb-sel-def"}
	applyListBoxDefaults(&cfg)
	if !cfg.Colors.Selected.IsSet() {
		t.Fatal("Colors.Selected unset after defaults, want the theme wash")
	}
	if cfg.Colors.Selected != defaultListBoxStyle.Colors.Selected {
		t.Errorf("Colors.Selected = %v, want theme %v",
			cfg.Colors.Selected, defaultListBoxStyle.Colors.Selected)
	}
	if cfg.Colors.Selected == cfg.Colors.Hover && cfg.Colors.Hover.IsSet() {
		t.Errorf("Colors.Selected = %v, the hover fill, want the subtle wash",
			cfg.Colors.Selected)
	}

	cfg = ListBoxCfg{ID: "lb-sel-caller", Colors: ColorSet{Selected: Blue}}
	applyListBoxDefaults(&cfg)
	if cfg.Colors.Selected != Blue {
		t.Errorf("caller Selected = %v, want %v", cfg.Colors.Selected, Blue)
	}

	cfg = ListBoxCfg{ID: "lb-sel-alias", ColorSelect: Red}
	applyListBoxDefaults(&cfg)
	if cfg.Colors.Selected != Red {
		t.Errorf("ColorSelect alias = %v, want %v", cfg.Colors.Selected, Red)
	}
}

// A selected row reacts to hover: the resting wash steps one OKLCH
// notch lighter under the pointer, the same as data grid rows. Before
// #744 the row kept its wash — OnHover only painted transparent rows,
// so selection never hovered.
func TestListBoxSelectedRowHovers(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID: "lb-sel-hover",
		Data: []ListBoxOption{
			{ID: "a", Name: "Alpha"},
			{ID: "b", Name: "Beta"},
		},
		SelectedIDs: []string{"b"},
		OnSelect:    func([]string, EventCtx) {},
	}), w)
	rows := listBoxRows(&layout)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	row := &rows[1]
	resting := row.Shape.Color
	if row.Shape.events == nil || row.Shape.events.OnHover == nil {
		t.Fatal("selected row has no hover handler")
	}
	row.Shape.events.OnHover(EventCtx{row, &Event{}, w})
	got := row.Shape.Color
	if got == resting {
		t.Fatalf("selected hovered fill = %v, the resting wash, want a lightness step",
			got)
	}
	if want := accentShift(resting, oklchRampDelta); got != want {
		t.Errorf("selected hovered fill = %v, want %v", got, want)
	}
}

// The reorderable path builds its rows through
// listBoxReorderItemView, so it needs the same hover step as the
// plain row above.
func TestListBoxReorderSelectedRowHovers(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID:          "lb-reorder-sel-hover",
		Data:        listBoxTestData(3),
		SelectedIDs: []string{"id-1"},
		Reorderable: true,
		OnSelect:    func([]string, EventCtx) {},
		OnReorder:   func(string, string, EventCtx) {},
	}), w)
	rows := listBoxRows(&layout)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	row := &rows[1]
	resting := row.Shape.Color
	if row.Shape.events == nil || row.Shape.events.OnHover == nil {
		t.Fatal("selected reorder row has no hover handler")
	}
	row.Shape.events.OnHover(EventCtx{row, &Event{}, w})
	got := row.Shape.Color
	if got == resting {
		t.Fatalf("selected hovered fill = %v, the resting wash, want a lightness step",
			got)
	}
	if want := accentShift(resting, oklchRampDelta); got != want {
		t.Errorf("selected hovered fill = %v, want %v", got, want)
	}
}
