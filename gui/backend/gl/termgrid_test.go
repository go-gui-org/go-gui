//go:build !js && !darwin && !android

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// drawTermGrid must degrade to a no-op without a text system —
// headless-safe, no GL context needed.
func TestDrawTermGridNoTextSys(t *testing.T) {
	b := &Backend{}
	cells := make([]gui.TermCell, 4)
	b.drawTermGrid(&gui.RenderCmd{
		Kind: gui.RenderTermGrid, X: 0, Y: 0,
		TermGrid: &gui.TermGridData{
			Cells: cells, Cols: 2, Rows: 2, CellW: 8, CellH: 16,
		},
	})
}

// drawTermGrid must drop malformed grids without looping or
// allocating — mirrors the soft backend's validTermGrid contract.
func TestDrawTermGridMalformed(t *testing.T) {
	b := &Backend{}
	bad := []*gui.RenderCmd{
		{Kind: gui.RenderTermGrid},
		{Kind: gui.RenderTermGrid,
			TermGrid: &gui.TermGridData{}},
		{Kind: gui.RenderTermGrid,
			TermGrid: &gui.TermGridData{
				Cells: make([]gui.TermCell, 2),
				Cols:  2, Rows: 2, CellW: 8, CellH: 16,
			}},
	}
	for _, r := range bad {
		b.drawTermGrid(r)
	}
}
