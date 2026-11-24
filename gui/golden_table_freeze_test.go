package gui

import "testing"

// goldenTableFreezeCases records frozen columns in a table wider than the
// golden window (#1363), scrolled so a cell is cut at the frozen edge.
func goldenTableFreezeCases() []goldenCase {
	table := func(w *Window, cfg TableCfg) View {
		w.scrollX().Set(ScopeID("tbl", "scroll"), -70)
		head := make([]TableCellCfg, 0, 8)
		row := make([]TableCellCfg, 0, 8)
		for c := range 8 {
			head = append(head, TH(string(rune('A'+c))))
			row = append(row, TD(string(rune('a'+c))))
		}
		cfg.ID = "tbl"
		cfg.FreezeHeader = true
		cfg.SizeBorder = 1
		cfg.Sizing = FillFill
		cfg.Data = []TableRowCfg{TR(head), TR(row), TR(row)}
		return Table(cfg)
	}
	return []goldenCase{
		{
			name: "table_freeze_columns_grid",
			build: func(w *Window) View {
				return table(w, TableCfg{
					BorderStyle: TableBorderAll, RowHeader: true, FreezeColumns: 1,
				})
			},
		},
		{
			// No cell borders: the line marks the frozen edge.
			name: "table_freeze_columns_edge",
			build: func(w *Window) View {
				return table(w, TableCfg{
					BorderStyle: TableBorderHorizontal, FreezeColumns: 1,
					Selected: map[int]bool{1: true},
				})
			},
		},
		{
			// The line follows the row header when only that is frozen.
			name: "table_freeze_row_header",
			build: func(w *Window) View {
				return table(w, TableCfg{
					BorderStyle: TableBorderHeaderOnly, RowHeader: true,
					FreezeRowHeader: true, CurrentRow: Some(1),
				})
			},
		},
	}
}

func TestGoldenTableFreeze(t *testing.T) {
	for _, c := range goldenTableFreezeCases() {
		t.Run(c.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{
				State:  new(int),
				Width:  goldenWidth,
				Height: goldenHeight,
			})
			w.SetView(wrapGoldenRoot(c.build))
			w.TestGolden(t, GoldenCfg{Dir: "testdata", Name: c.name})
		})
	}
}
