package datagrid

import (
	"strconv"
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// dataGridFrameCfg is a grid of 2000 rows by 8 columns whose scroll
// body shows about visibleRows rows. Virtualization builds only those,
// so the frame cost follows visibleRows, not the row count.
func dataGridFrameCfg(visibleRows int) DataGridCfg {
	cols := make([]GridColumnCfg, 8)
	for i := range cols {
		cols[i] = GridColumnCfg{ID: "c" + strconv.Itoa(i), Title: "C" + strconv.Itoa(i)}
	}
	rows := make([]GridRow, 2000)
	for r := range rows {
		cells := make(map[string]string, len(cols))
		for _, c := range cols {
			cells[c.ID] = c.ID + "-" + strconv.Itoa(r)
		}
		rows[r] = GridRow{ID: "r" + strconv.Itoa(r), Cells: cells}
	}
	return DataGridCfg{
		ID:      "grid",
		Columns: cols,
		Rows:    rows,
		Height:  dataGridDefaultHeaderHeight + float32(visibleRows)*dataGridDefaultRowHeight,
		// A selection handler makes the rows clickable, as in an app.
		OnSelectionChange: func(GridSelection, gg.EventCtx) {},
	}
}

// dataGridFrameAllocs is the allocation count of one full frame of a
// grid showing visibleRows rows. TestRender runs a real frame, which
// resets gui's view arenas, so the count is the steady-state cost.
func dataGridFrameAllocs(visibleRows int) float64 {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cfg := dataGridFrameCfg(visibleRows)
	view := func(w *gg.Window) gg.View { return New(w, cfg) }
	w.TestRender(view)
	return testing.AllocsPerRun(20, func() { w.TestRender(nil) })
}

// TestDataGridRowAllocsPerRow gates the cost of one visible row: a
// grid 16 rows tall minus one 8 rows tall, over 8. Everything a frame
// pays once (header, scroll body, scrollbars) cancels out.
//
// On 8 columns a row cost 38 allocations before the content backing,
// cell IDs and editor focus base were shared per row.
func TestDataGridRowAllocsPerRow(t *testing.T) {
	perRow := (dataGridFrameAllocs(16) - dataGridFrameAllocs(8)) / 8
	t.Logf("allocs per row = %.2f", perRow)
	if perRow > 22 {
		t.Errorf("allocs per row = %.2f, want <= 22", perRow)
	}
}

// BenchmarkDataGridFrame is one full frame of a 2000 x 8 grid showing
// about 30 rows.
func BenchmarkDataGridFrame(b *testing.B) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cfg := dataGridFrameCfg(30)
	w.TestRender(func(w *gg.Window) gg.View { return New(w, cfg) })
	b.ReportAllocs()
	for b.Loop() {
		w.TestRender(nil)
	}
}

// BenchmarkDataGridPresentationRows benchmarks the row-preparation hot
// path that maps visible indices to display rows for virtual scrolling.
//
//	flat: rows without grouping
//	grouped: rows with GroupBy triggering group header generation
func BenchmarkDataGridPresentationRows(b *testing.B) {
	makeRows := func(n int) []GridRow {
		out := make([]GridRow, n)
		for i := range n {
			out[i] = GridRow{
				ID:    "row-" + strconv.Itoa(i),
				Cells: map[string]string{"colA": "val_" + strconv.Itoa(i)},
			}
		}
		return out
	}
	makeCols := func() []GridColumnCfg {
		return []GridColumnCfg{
			{ID: "colA", Title: "Column A"},
		}
	}
	makeIndices := func(n int) []int {
		idx := make([]int, n)
		for i := range n {
			idx[i] = i
		}
		return idx
	}

	b.Run("flat_100", func(b *testing.B) {
		cfg := &DataGridCfg{Rows: makeRows(100)}
		cols := makeCols()
		indices := makeIndices(50) // 50 visible of 100
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = dataGridPresentationRows(cfg, cols, indices)
		}
	})

	b.Run("flat_1000", func(b *testing.B) {
		cfg := &DataGridCfg{Rows: makeRows(1000)}
		cols := makeCols()
		indices := makeIndices(200)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = dataGridPresentationRows(cfg, cols, indices)
		}
	})

	// Grouped: 10 groups of 10 rows each.
	b.Run("grouped_100", func(b *testing.B) {
		rows := makeRows(100)
		for i := range 100 {
			rows[i].Cells["colA"] = "group_" + strconv.Itoa(i/10)
		}
		cfg := &DataGridCfg{
			Rows:    rows,
			GroupBy: []string{"colA"},
		}
		cols := []GridColumnCfg{
			{ID: "colA", Title: "Column A"},
		}
		indices := makeIndices(100)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = dataGridPresentationRows(cfg, cols, indices)
		}
	})

	b.Run("grouped_500", func(b *testing.B) {
		rows := makeRows(500)
		for i := range 500 {
			rows[i].Cells["colA"] = "group_" + strconv.Itoa(i/25)
		}
		cfg := &DataGridCfg{
			Rows:    rows,
			GroupBy: []string{"colA"},
		}
		cols := []GridColumnCfg{
			{ID: "colA", Title: "Column A"},
		}
		indices := makeIndices(200)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = dataGridPresentationRows(cfg, cols, indices)
		}
	})
}
