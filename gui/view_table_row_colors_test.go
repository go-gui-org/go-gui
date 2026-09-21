package gui

import "testing"

// Tests for Table rows routed through ColorSet.pick (#744).

// The selected row paints Colors.Selected, which the theme sets to
// the subtle wash, never the full accent slab (visual-refresh §4.3).
// The keyboard-active row paints Colors.Focus, which the theme sets
// to the hover fill. A caller-set Colors.Selected wins over the
// theme. A caller-set ColorSelect keeps its old meaning: it becomes
// the selected fill when Colors.Selected is unset.
func TestTableSelectedDefaults(t *testing.T) {
	cfg := TableCfg{ID: "tbl-sel-def"}
	applyTableDefaults(&cfg)
	if !cfg.Colors.Selected.IsSet() {
		t.Fatal("Colors.Selected unset after defaults, want the theme wash")
	}
	if cfg.Colors.Selected != defaultTableStyle.Colors.Selected {
		t.Errorf("Colors.Selected = %v, want theme %v",
			cfg.Colors.Selected, defaultTableStyle.Colors.Selected)
	}
	if cfg.Colors.Focus != cfg.Colors.Hover {
		t.Errorf("Colors.Focus = %v, want the hover fill %v",
			cfg.Colors.Focus, cfg.Colors.Hover)
	}

	cfg = TableCfg{ID: "tbl-sel-caller", Colors: ColorSet{Selected: Blue}}
	applyTableDefaults(&cfg)
	if cfg.Colors.Selected != Blue {
		t.Errorf("caller Selected = %v, want %v", cfg.Colors.Selected, Blue)
	}

	cfg = TableCfg{ID: "tbl-sel-alias", ColorSelect: Red}
	applyTableDefaults(&cfg)
	if cfg.Colors.Selected != Red {
		t.Errorf("ColorSelect alias = %v, want %v", cfg.Colors.Selected, Red)
	}
}

// A selected row reacts to hover: the resting wash steps one OKLCH
// notch lighter under the pointer, the same as data grid rows. Before
// #744 the OnHover guard skipped selected rows, so selection never
// hovered.
func TestTableSelectedRowHovers(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Table(TableCfg{
		ID:       "tbl-sel-hover",
		OnSelect: func(map[int]bool, int, EventCtx) {},
		Data: []TableRowCfg{
			TR([]TableCellCfg{tD("a")}),
			TR([]TableCellCfg{tD("b")}),
		},
		Selected: map[int]bool{1: true},
	}), w)
	rows := tableRows(&layout)
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

// The keyboard-active row keeps its hover tint: under pick it is the
// Focus state, and the theme sets Focus to the hover fill.
func TestTableActiveRowPaintsFocus(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Table(TableCfg{
		ID:        "tbl-active-focus",
		Focusable: true,
		Data: []TableRowCfg{
			TR([]TableCellCfg{tD("a")}),
			TR([]TableCellCfg{tD("b")}),
		},
	}), w)
	rows := tableRows(&layout)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	cfg := TableCfg{ID: "tbl-active-focus"}
	applyTableDefaults(&cfg)
	if got := rows[0].Shape.Color; got != cfg.Colors.Hover {
		t.Errorf("active row fill = %v, want hover %v", got, cfg.Colors.Hover)
	}
}
