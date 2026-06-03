package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestTableBasic(t *testing.T) {
	v := Table(TableCfg{
		ID: "tbl-test",
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("Name"), TH("Age")}),
			TR([]TableCellCfg{TD("Alice"), TD("30")}),
			TR([]TableCellCfg{TD("Bob"), TD("25")}),
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	if len(layout.Children) != 3 {
		t.Fatalf("rows = %d, want 3", len(layout.Children))
	}
}

func TestTableEmpty(t *testing.T) {
	v := Table(TableCfg{ID: "tbl-test"})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	if len(layout.Children) != 0 {
		t.Errorf("children = %d, want 0", len(layout.Children))
	}
}

func TestTableBorderAll(t *testing.T) {
	v := Table(TableCfg{
		ID:          "tbl-test",
		BorderStyle: TableBorderAll,
		SizeBorder:  1,
		Data: []TableRowCfg{
			TR([]TableCellCfg{TD("a"), TD("b")}),
			TR([]TableCellCfg{TD("c"), TD("d")}),
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	// 2 rows (cell borders with negative spacing, no separators).
	if len(layout.Children) != 2 {
		t.Errorf("children = %d, want 2", len(layout.Children))
	}
	// Each cell should have a border.
	row := layout.Children[0]
	cell := row.Children[0]
	if cell.Shape.SizeBorder != 1 {
		t.Errorf("cell border = %v, want 1", cell.Shape.SizeBorder)
	}
}

func TestTableBorderHorizontal(t *testing.T) {
	v := Table(TableCfg{
		ID:          "tbl-test",
		BorderStyle: TableBorderHorizontal,
		SizeBorder:  1,
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("H")}),
			TR([]TableCellCfg{TD("a")}),
			TR([]TableCellCfg{TD("b")}),
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	// 3 rows + 2 separators (between 0-1 and 1-2; not after last).
	if len(layout.Children) != 5 {
		t.Errorf("children = %d, want 5", len(layout.Children))
	}
}

func TestTableBorderHeaderOnly(t *testing.T) {
	v := Table(TableCfg{
		ID:          "tbl-test",
		BorderStyle: TableBorderHeaderOnly,
		SizeBorder:  2,
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("H")}),
			TR([]TableCellCfg{TD("a")}),
			TR([]TableCellCfg{TD("b")}),
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	// 3 rows + 1 separator (after header only).
	if len(layout.Children) != 4 {
		t.Errorf("children = %d, want 4", len(layout.Children))
	}
}

func TestTableCfgFromData(t *testing.T) {
	data := [][]string{
		{"Name", "Age"},
		{"Alice", "30"},
	}
	cfg := TableCfgFromData(data)
	if len(cfg.Data) != 2 {
		t.Fatalf("rows = %d, want 2", len(cfg.Data))
	}
	if !cfg.Data[0].Cells[0].HeadCell {
		t.Error("first row should be header")
	}
	if cfg.Data[1].Cells[0].HeadCell {
		t.Error("second row should not be header")
	}
}

func TestTableCfgError(t *testing.T) {
	cfg := TableCfgError("oops")
	if len(cfg.Data) != 1 {
		t.Fatalf("rows = %d, want 1", len(cfg.Data))
	}
	if cfg.Data[0].Cells[0].Value != "oops" {
		t.Errorf("value = %q", cfg.Data[0].Cells[0].Value)
	}
}

func TestTableHelpers(t *testing.T) {
	row := TR([]TableCellCfg{TH("x"), TD("y")})
	if len(row.Cells) != 2 {
		t.Fatalf("cells = %d", len(row.Cells))
	}
	if !row.Cells[0].HeadCell {
		t.Error("TH should be head")
	}
	if row.Cells[1].HeadCell {
		t.Error("TD should not be head")
	}
}

func TestTableSelection(t *testing.T) {
	var selectedRows map[int]bool
	var clickedRow int
	v := Table(TableCfg{
		ID: "tbl-test",
		Data: []TableRowCfg{
			TR([]TableCellCfg{TD("a")}),
			TR([]TableCellCfg{TD("b")}),
		},
		OnSelect: func(sel map[int]bool, row int, _ *Event, _ *Window) {
			selectedRows = sel
			clickedRow = row
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	// Click second row.
	row := &layout.Children[1]
	if row.Shape.HasEvents() && row.Shape.Events.OnClick != nil {
		row.Shape.Events.OnClick(row, &Event{}, w)
	}
	if clickedRow != 1 {
		t.Errorf("clicked row = %d, want 1", clickedRow)
	}
	if !selectedRows[1] {
		t.Error("row 1 should be selected")
	}
}

type tableTestMeasurer struct{}

func (m *tableTestMeasurer) TextWidth(text string, _ TextStyle) float32 {
	return float32(len(text)) * 8
}
func (m *tableTestMeasurer) TextHeight(_ string, _ TextStyle) float32 { return 16 }
func (m *tableTestMeasurer) FontAscent(s TextStyle) float32           { return s.Size * 0.8 }
func (m *tableTestMeasurer) FontHeight(_ TextStyle) float32           { return 16 }
func (m *tableTestMeasurer) LayoutText(_ string, _ TextStyle, _ float32) (glyph.Layout, error) {
	return glyph.Layout{Height: 16}, nil
}

func TestTableColumnAutoWidth(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}
	v := w.Table(TableCfg{
		ID:          "auto-width-test",
		CellPadding: SomeP(4, 4, 4, 4),
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("Name"), TH("Age")}),
			TR([]TableCellCfg{TD("Alexander"), TD("30")}),
		},
	})
	layout := GenerateViewLayout(v, w)
	// First column cell should be wider than second.
	row0 := layout.Children[0]
	if len(row0.Children) < 2 {
		t.Fatal("expected 2 cells")
	}
	col0W := row0.Children[0].Shape.Width
	col1W := row0.Children[1].Shape.Width
	if col0W <= col1W {
		t.Errorf("col0 width (%.1f) should be > col1 (%.1f)",
			col0W, col1W)
	}
}

func TestTableRowAltColor(t *testing.T) {
	alt := RGB(50, 50, 50)
	v := Table(TableCfg{
		ID:          "tbl-test",
		ColorRowAlt: &alt,
		Data: []TableRowCfg{
			TR([]TableCellCfg{TD("a")}),
			TR([]TableCellCfg{TD("b")}),
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	// Row 0: transparent, Row 1: alt color.
	if layout.Children[1].Shape.Color != alt {
		t.Errorf("row 1 color = %v, want %v",
			layout.Children[1].Shape.Color, alt)
	}
}

func TestWindowTable(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}
	v := w.Table(TableCfg{
		ID: "window-table-test",
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("A"), TH("B")}),
			TR([]TableCellCfg{TD("x"), TD("yy")}),
		},
	})
	layout := GenerateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("rows = %d, want 2", len(layout.Children))
	}
}

func TestTableColumnWidthCaching(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}
	cfg := TableCfg{
		ID: "cache-test",
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("Name")}),
			TR([]TableCellCfg{TD("Alice")}),
		},
	}
	// First call measures.
	v1 := w.Table(cfg)
	_ = GenerateViewLayout(v1, w)

	// Second call should hit cache.
	v2 := w.Table(cfg)
	layout2 := GenerateViewLayout(v2, w)
	if len(layout2.Children) != 2 {
		t.Fatalf("rows = %d, want 2", len(layout2.Children))
	}
}

func TestClearTableCache(_ *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}
	cfg := TableCfg{
		ID: "clear-cache-test",
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("Col")}),
			TR([]TableCellCfg{TD("val")}),
		},
	}
	v := w.Table(cfg)
	_ = GenerateViewLayout(v, w)

	// Clear specific.
	w.ClearTableCache("clear-cache-test")

	// Clear all.
	v2 := w.Table(cfg)
	_ = GenerateViewLayout(v2, w)
	w.ClearAllTableCaches()
}

func TestTableVirtualization(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}

	data := make([]TableRowCfg, 0, 101)
	data = append(data, TR([]TableCellCfg{TH("Col")}))
	for range 100 {
		data = append(data, TR([]TableCellCfg{TD("row")}))
	}

	v := w.Table(TableCfg{
		ID:        "virtual-test",
		IDScroll:  9999,
		MaxHeight: 200,
		Data:      data,
	})
	layout := GenerateViewLayout(v, w)
	// Should have fewer children than total rows due to
	// virtualization (visible rows + spacers).
	if len(layout.Children) >= 101 {
		t.Errorf("expected virtualized children < 101, got %d",
			len(layout.Children))
	}
}

func TestTableCfgFromCSV(t *testing.T) {
	csv := "Name,Age\nAlice,30\nBob,25\n"
	cfg, err := TableCfgFromCSV(csv)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Data) != 3 {
		t.Fatalf("rows = %d, want 3", len(cfg.Data))
	}
	if !cfg.Data[0].Cells[0].HeadCell {
		t.Error("first row should be header")
	}
	if cfg.Data[0].Cells[0].Value != "Name" {
		t.Errorf("header = %q, want Name", cfg.Data[0].Cells[0].Value)
	}
}

func TestTableFromCSV(t *testing.T) {
	w := &Window{}
	v := w.TableFromCSV("A,B\n1,2\n")
	layout := GenerateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("rows = %d, want 2", len(layout.Children))
	}
}

func TestTableFromCSVError(t *testing.T) {
	w := &Window{}
	v := w.TableFromCSV("\"unclosed")
	layout := GenerateViewLayout(v, w)
	// Should produce error table with 1 row.
	if len(layout.Children) != 1 {
		t.Fatalf("rows = %d, want 1", len(layout.Children))
	}
}

func TestTableFreezeHeader(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}
	v := w.Table(TableCfg{
		ID:           "freeze-test",
		IDScroll:     8800,
		MaxHeight:    200,
		FreezeHeader: true,
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("Name"), TH("Age")}),
			TR([]TableCellCfg{TD("Alice"), TD("30")}),
			TR([]TableCellCfg{TD("Bob"), TD("25")}),
		},
	})
	layout := GenerateViewLayout(v, w)
	// Outer has 2 children: header zone, body zone.
	if len(layout.Children) != 2 {
		t.Fatalf("outer children = %d, want 2", len(layout.Children))
	}
	headerZone := layout.Children[0]
	bodyZone := layout.Children[1]
	// Header zone: 1 row (no separator for BorderNone default).
	if len(headerZone.Children) != 1 {
		t.Errorf("header zone children = %d, want 1",
			len(headerZone.Children))
	}
	// Body zone: 2 data rows + scrollbar.
	if len(bodyZone.Children) != 3 {
		t.Errorf("body zone children = %d, want 3",
			len(bodyZone.Children))
	}
}

func TestTableFreezeHeaderWithSeparator(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}
	v := w.Table(TableCfg{
		ID:           "freeze-sep-test",
		IDScroll:     8801,
		MaxHeight:    200,
		FreezeHeader: true,
		BorderStyle:  TableBorderHeaderOnly,
		SizeBorder:   2,
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("H")}),
			TR([]TableCellCfg{TD("a")}),
			TR([]TableCellCfg{TD("b")}),
		},
	})
	layout := GenerateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("outer children = %d, want 2", len(layout.Children))
	}
	headerZone := layout.Children[0]
	bodyZone := layout.Children[1]
	// Header zone: 1 row + 1 separator.
	if len(headerZone.Children) != 2 {
		t.Errorf("header zone children = %d, want 2",
			len(headerZone.Children))
	}
	// Body zone: 2 body rows + scrollbar.
	if len(bodyZone.Children) != 3 {
		t.Errorf("body zone children = %d, want 3",
			len(bodyZone.Children))
	}
}

func TestTableFreezeHeaderNoScroll(t *testing.T) {
	// FreezeHeader=true but IDScroll=0 → falls back to single Column.
	v := Table(TableCfg{
		ID:           "tbl-test",
		FreezeHeader: true,
		Data: []TableRowCfg{
			TR([]TableCellCfg{TH("H")}),
			TR([]TableCellCfg{TD("a")}),
			TR([]TableCellCfg{TD("b")}),
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	// Same as non-frozen: 3 rows, single Column.
	if len(layout.Children) != 3 {
		t.Errorf("children = %d, want 3", len(layout.Children))
	}
}

func TestTableFreezeHeaderVirtualization(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}

	data := make([]TableRowCfg, 0, 101)
	data = append(data, TR([]TableCellCfg{TH("Col")}))
	for range 100 {
		data = append(data, TR([]TableCellCfg{TD("row")}))
	}

	v := w.Table(TableCfg{
		ID:           "freeze-virtual-test",
		IDScroll:     8802,
		MaxHeight:    200,
		FreezeHeader: true,
		Data:         data,
	})
	layout := GenerateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("outer children = %d, want 2", len(layout.Children))
	}
	bodyZone := layout.Children[1]
	// Body should have fewer than 100 children due to virtualization.
	if len(bodyZone.Children) >= 100 {
		t.Errorf("body children = %d, want < 100",
			len(bodyZone.Children))
	}
}

func TestTableRichTextCell(t *testing.T) {
	v := Table(TableCfg{
		ID: "tbl-test",
		Data: []TableRowCfg{
			TR([]TableCellCfg{{
				RichText: &RichText{
					Runs: []RichTextRun{
						{Text: "bold", Style: TextStyle{Size: 14}},
						{Text: " normal"},
					},
				},
			}}),
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	if len(layout.Children) != 1 {
		t.Fatalf("rows = %d, want 1", len(layout.Children))
	}
}
