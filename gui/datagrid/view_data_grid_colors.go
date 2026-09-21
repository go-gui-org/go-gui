package datagrid

import (
	gg "github.com/go-gui-org/go-gui/gui"
)

// dataGridRowFill returns the fill of one body row. It goes through
// ColorSet.Pick so the grid orders selected and hovered the same way
// the widgets in gui/ do, and does not write the order by hand (#741).
//
// The resting fill is not a ColorsRow slot: it is transparent, or
// ColorRowAlt on odd rows. It is put in Base after Resolved ran, so it
// changes only the resting fill. Hover already holds its own color.
//
// cfg must have been through applyDataGridDefaults.
func dataGridRowFill(cfg *DataGridCfg, rowIdx int, selected, hovered bool) gg.Color {
	cs := cfg.ColorsRow
	cs.Base = gg.ColorTransparent
	if rowIdx%2 == 1 {
		cs.Base = cfg.ColorRowAlt
	}
	fill, _ := cs.Pick(gg.PickState{Selected: selected, Hovered: hovered})
	return fill
}
