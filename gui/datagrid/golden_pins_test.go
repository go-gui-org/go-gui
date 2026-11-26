package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// TestGoldenDataGridPins records a grid with a column pinned on each side
// (#1364), scrolled so a cell is cut at both edges. Without borders the
// edge lines mark them.
func TestGoldenDataGridPins(t *testing.T) {
	for _, c := range []struct {
		name string
		mut  func(*DataGridCfg)
	}{
		{"data_grid_pins", nil},
		{"data_grid_pins_edge", func(c *DataGridCfg) { c.SizeBorder = gg.NoBorder }},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := gg.NewTestWindow(t, gg.WindowCfg{State: new(int), Width: 420, Height: 240})
			w.SetView(func(w *gg.Window) gg.View {
				w.ScrollX().Set(pinScrollID, -60)
				return pinGrid(w, map[int]GridColumnPin{
					1: GridColumnPinLeft, 2: GridColumnPinRight,
				}, func(cfg *DataGridCfg) {
					cfg.Rows = cfg.Rows[:3]
					cfg.Height = 200
					cfg.ShowFilterRow = true
					cfg.Selection = GridSelection{SelectedRowIDs: map[string]bool{"1": true}}
					if c.mut != nil {
						c.mut(cfg)
					}
				})
			})
			w.TestGolden(t, gg.GoldenCfg{Name: c.name})
		})
	}
}

// TestGoldenDataGridNarrowBars records a grid with the CRUD toolbar and
// the pager in a parent narrower than both bars (#1368). Each bar is
// cut at the parent's edge and scrolls sideways; the grid, its border
// and its vertical scrollbar stay inside the parent.
func TestGoldenDataGridNarrowBars(t *testing.T) {
	w := gg.NewTestWindow(t, gg.WindowCfg{State: new(int), Width: 240, Height: 240})
	w.SetTextMeasurer(dataGridRenderTextMeasurer{})
	w.SetView(func(w *gg.Window) gg.View {
		return gg.Row(gg.ContainerCfg{Sizing: gg.FillFill, Padding: gg.NoPadding,
			Content: []gg.View{
				gg.Column(gg.ContainerCfg{Sizing: gg.FixedFill, Width: 160,
					Padding: gg.NoPadding, SizeBorder: gg.NoBorder,
					Content: []gg.View{New(w, DataGridCfg{
						ID:              "g",
						Columns:         []GridColumnCfg{{ID: "name", Title: "Name", Width: gg.SomeF(80)}},
						Rows:            makeTestRows(40),
						PageSize:        10,
						ShowCRUDToolbar: true,
						Sizing:          gg.FillFixed,
						Height:          200,
					})}}),
			}})
	})
	w.TestGolden(t, gg.GoldenCfg{Name: "data_grid_narrow_bars"})
}
