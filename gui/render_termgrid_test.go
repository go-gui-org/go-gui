package gui

import (
	"math"
	"testing"
)

func termCells(cols, rows int) []TermCell {
	cells := make([]TermCell, cols*rows)
	for i := range cells {
		cells[i] = TermCell{Ch: 'x', FG: White, BG: Black, Width: 1}
	}
	return cells
}

func TestTermGridFactoryLayout(t *testing.T) {
	w := makeWindowWithScratch()
	v := TermGrid(TermGridCfg{
		Cols: 80, Rows: 24,
		CellW: 8, CellH: 16,
		Cells: termCells(80, 24),
	})
	l := v.GenerateLayout(w)
	s := l.Shape
	if s.shapeType != shapeTermGrid {
		t.Fatalf("shapeType = %d, want shapeTermGrid", s.shapeType)
	}
	if s.Width != 640 || s.Height != 384 {
		t.Errorf("size = (%v,%v), want (640,384)", s.Width, s.Height)
	}
	if s.Sizing != FixedFixed {
		t.Errorf("Sizing = %v, want FixedFixed", s.Sizing)
	}
	if s.tg == nil {
		t.Fatal("shape.tg is nil")
	}
	if s.tg.Cols != 80 || s.tg.Rows != 24 {
		t.Errorf("tg dims = (%d,%d), want (80,24)", s.tg.Cols, s.tg.Rows)
	}
}

func TestRenderTermGridEmitsOneCommand(t *testing.T) {
	w := makeWindowWithScratch()
	tg := &TermGridData{
		Cells: termCells(4, 2),
		Cols:  4, Rows: 2,
		CellW: 8, CellH: 16,
	}
	shape := &Shape{
		shapeType: shapeTermGrid,
		X:         10, Y: 20,
		Width: 32, Height: 32,
		tg: tg,
	}
	renderTermGrid(shape, makeClip(0, 0, 200, 200), w)

	var n int
	var cmd *RenderCmd
	for i := range w.renderers {
		if w.renderers[i].Kind == RenderTermGrid {
			n++
			cmd = &w.renderers[i]
		}
	}
	if n != 1 {
		t.Fatalf("RenderTermGrid count = %d, want 1 (single-pass, no per-cell)", n)
	}
	if cmd.TermGrid != tg {
		t.Error("RenderCmd.TermGrid does not point at the grid buffer")
	}
	if cmd.X != 10 || cmd.Y != 20 || cmd.W != 32 || cmd.H != 32 {
		t.Errorf("cmd geom = (%v,%v,%v,%v), want (10,20,32,32)",
			cmd.X, cmd.Y, cmd.W, cmd.H)
	}
}

func TestRenderTermGridNoPerCellNodes(t *testing.T) {
	w := makeWindowWithScratch()
	tg := &TermGridData{
		Cells: termCells(40, 10), // 400 cells
		Cols:  40, Rows: 10,
		CellW: 8, CellH: 16,
	}
	shape := &Shape{
		shapeType: shapeTermGrid,
		Width:     320, Height: 160,
		tg: tg,
	}
	renderTermGrid(shape, makeClip(0, 0, 400, 400), w)

	if len(w.renderers) != 1 {
		t.Errorf("emitted %d renderers for 400 cells, want 1", len(w.renderers))
	}
}

func TestRenderTermGridOutsideClipSkips(t *testing.T) {
	w := makeWindowWithScratch()
	tg := &TermGridData{
		Cells: termCells(4, 2),
		Cols:  4, Rows: 2,
		CellW: 8, CellH: 16,
	}
	shape := &Shape{
		shapeType: shapeTermGrid,
		X:         500, Y: 500,
		Width: 32, Height: 32,
		tg: tg,
	}
	renderTermGrid(shape, makeClip(0, 0, 100, 100), w)

	if len(w.renderers) != 0 {
		t.Errorf("got %d renderers, want 0 for out-of-clip grid", len(w.renderers))
	}
}

func TestRenderTermGridDegenerateSkips(t *testing.T) {
	w := makeWindowWithScratch()
	cases := []struct {
		name string
		tg   *TermGridData
	}{
		{"nil", nil},
		{"zero cols", &TermGridData{Rows: 2, CellW: 8, CellH: 16}},
		{"zero cellW", &TermGridData{Cols: 4, Rows: 2, CellH: 16}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w.renderers = w.renderers[:0]
			shape := &Shape{
				shapeType: shapeTermGrid,
				Width:     32, Height: 32,
				tg: tc.tg,
			}
			renderTermGrid(shape, makeClip(0, 0, 200, 200), w)
			if len(w.renderers) != 0 {
				t.Errorf("got %d renderers, want 0", len(w.renderers))
			}
		})
	}
}

func TestRenderTermGridSkipsOnNaNCellW(t *testing.T) {
	w := makeWindowWithScratch()
	tg := &TermGridData{
		Cells: termCells(4, 2),
		Cols:  4, Rows: 2,
		CellW: float32(math.NaN()), CellH: 16,
	}
	shape := &Shape{
		shapeType: shapeTermGrid,
		Width:     32, Height: 32,
		tg: tg,
	}
	renderTermGrid(shape, makeClip(0, 0, 200, 200), w)
	if len(w.renderers) != 0 {
		t.Errorf("got %d renderers, want 0 for NaN CellW", len(w.renderers))
	}
}

func TestRenderTermGridSkipsOnInfCellW(t *testing.T) {
	w := makeWindowWithScratch()
	tg := &TermGridData{
		Cells: termCells(4, 2),
		Cols:  4, Rows: 2,
		CellW: float32(math.Inf(1)), CellH: 16,
	}
	shape := &Shape{
		shapeType: shapeTermGrid,
		Width:     32, Height: 32,
		tg: tg,
	}
	renderTermGrid(shape, makeClip(0, 0, 200, 200), w)
	if len(w.renderers) != 0 {
		t.Errorf("got %d renderers, want 0 for Inf CellW", len(w.renderers))
	}
}

func TestRenderTermGridSkipsOnZeroRows(t *testing.T) {
	w := makeWindowWithScratch()
	tg := &TermGridData{
		Cells: termCells(4, 2),
		Cols:  4, Rows: 0,
		CellW: 8, CellH: 16,
	}
	shape := &Shape{
		shapeType: shapeTermGrid,
		Width:     32, Height: 32,
		tg: tg,
	}
	renderTermGrid(shape, makeClip(0, 0, 200, 200), w)
	if len(w.renderers) != 0 {
		t.Errorf("got %d renderers, want 0 for zero rows", len(w.renderers))
	}
}
