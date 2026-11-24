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
