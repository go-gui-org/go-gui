package datagrid

import (
	"strings"
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// Grid cells draw their borders collapsed into one set of grid lines:
// each cell overlaps its neighbor by one border width. A rounded cell
// corner cuts a notch into the line at every intersection, and the
// grid reads as a sheet of separate boxes. So every header, filter and
// data cell is square, whatever the theme's container radius is. Only
// the grid's outer frame takes DataGridCfg.Radius.
func TestDataGridCellsAreSquare(t *testing.T) {
	cfg := DataGridCfg{
		ID: "g1",
		Columns: []GridColumnCfg{
			{ID: "c1", Title: "One", Filterable: true},
			{ID: "c2", Title: "Two", Filterable: true},
		},
		ShowFilterRow: true,
		Rows: []GridRow{
			{ID: "r1", Cells: map[string]string{"c1": "a", "c2": "b"}},
			{ID: "r2", Cells: map[string]string{"c1": "c", "c2": "d"}},
		},
	}
	w := gg.NewTestWindow(t, gg.WindowCfg{})
	t.Cleanup(w.Close)
	root := w.TestRender(func(win *gg.Window) gg.View { return New(win, cfg) })

	var cells int
	var walk func(*gg.Layout)
	walk = func(ly *gg.Layout) {
		id := ly.Shape.ID
		isCell := strings.Contains(id, ":cell:") ||
			strings.Contains(id, ":filter_cell:") ||
			strings.Contains(id, ":header:")
		if isCell && ly.Shape.SizeBorder > 0 {
			cells++
			if ly.Shape.Radius != 0 {
				t.Errorf("cell %q radius = %v, want 0", id, ly.Shape.Radius)
			}
		}
		for i := range ly.Children {
			walk(&ly.Children[i])
		}
	}
	walk(root)
	if cells == 0 {
		t.Fatal("found no bordered grid cells")
	}
}
