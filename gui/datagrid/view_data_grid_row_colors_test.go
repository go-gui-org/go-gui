package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// The selected row paints ColorsRow.Selected, which the theme sets to
// the subtle wash, never the full accent slab (visual-refresh §4.3). A
// caller-set ColorsRow.Selected wins over the theme (#741).
func TestDataGridRowSelectedDefaults(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	applyDataGridDefaults(cfg)
	if !cfg.ColorsRow.Selected.IsSet() {
		t.Error("ColorsRow.Selected unset after defaults, want the theme wash")
	}
	if cfg.ColorsRow.Selected == cfg.ColorsResize.Click {
		t.Errorf("ColorsRow.Selected = %v, the full accent, want the subtle wash",
			cfg.ColorsRow.Selected)
	}

	cfg = &DataGridCfg{ID: "g", ColorsRow: gg.ColorSet{Selected: gg.Blue}}
	applyDataGridDefaults(cfg)
	if cfg.ColorsRow.Selected != gg.Blue {
		t.Errorf("caller Selected = %v, want %v", cfg.ColorsRow.Selected, gg.Blue)
	}
}

// The row fill goes through ColorSet.Pick, so a selected row reacts to
// hover like a selected tab does. Before #741 a selected row kept its
// wash under the pointer and the grid ordered the states by hand.
func TestDataGridRowFill(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", ColorRowAlt: gg.Green}
	applyDataGridDefaults(cfg)
	sel := cfg.ColorsRow.Selected
	selHover := dataGridRowFill(cfg, 0, true, true)

	tests := []struct {
		name              string
		rowIdx            int
		selected, hovered bool
		want              gg.Color
	}{
		{"even resting", 0, false, false, gg.ColorTransparent},
		{"odd resting", 1, false, false, gg.Green},
		{"hovered", 1, false, true, cfg.ColorsRow.Hover},
		{"selected", 1, true, false, sel},
		{"selected hovered", 0, true, true, selHover},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dataGridRowFill(cfg, tc.rowIdx, tc.selected, tc.hovered)
			if got != tc.want {
				t.Errorf("fill = %v, want %v", got, tc.want)
			}
		})
	}
	if selHover == sel {
		t.Error("selected hovered row = the resting wash, want a lightness step")
	}
}
