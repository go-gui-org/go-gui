package datagrid

import (
	"strconv"
	"testing"
)

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
