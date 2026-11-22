package gui

import "testing"

// goldenTableScrollCases records a frozen-header table wider than the
// golden window (#1293): it fits the window, shows both scrollbars and
// moves the header with the rows. Run by its own test because
// golden_cases_test.go is past the file-size limit.
func goldenTableScrollCases() []goldenCase {
	wide := func(w *Window, offsetX float32) View {
		if offsetX != 0 {
			// Seeded, not scrolled: the clamp runs before positions, so
			// the recording shows the header following the clamped
			// offset.
			w.scrollX().Set(ScopeID("tbl", "scroll"), offsetX)
		}
		head := make([]TableCellCfg, 0, 8)
		row := make([]TableCellCfg, 0, 8)
		for c := range 8 {
			head = append(head, TH(string(rune('A'+c))))
			row = append(row, TD(string(rune('a'+c))))
		}
		return Table(TableCfg{
			ID:           "tbl",
			FreezeHeader: true,
			BorderStyle:  TableBorderAll,
			SizeBorder:   1,
			Sizing:       FillFill,
			Data:         []TableRowCfg{TR(head), TR(row), TR(row)},
		})
	}
	return []goldenCase{
		{
			name:  "table_freeze_wide",
			build: func(w *Window) View { return wide(w, 0) },
		},
		{
			name:  "table_freeze_wide_scrolled",
			build: func(w *Window) View { return wide(w, -100000) },
		},
	}
}

func TestGoldenTableScroll(t *testing.T) {
	for _, c := range goldenTableScrollCases() {
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
