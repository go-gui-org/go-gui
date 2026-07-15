package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- dataGridFilterHeight ---

func TestFilterHeightUsesHeaderHeight(t *testing.T) {
	cfg := &DataGridCfg{HeaderHeight: 28}
	got := dataGridFilterHeight(cfg)
	if got != 28 {
		t.Errorf("got %v, want 28", got)
	}
}

func TestFilterHeightFallsBackToRowHeight(t *testing.T) {
	cfg := &DataGridCfg{RowHeight: 22}
	got := dataGridFilterHeight(cfg)
	if got != 22 {
		t.Errorf("got %v, want 22", got)
	}
}

// --- dataGridQuickFilterHeight ---

func TestQuickFilterHeightUsesHeaderHeight(t *testing.T) {
	cfg := &DataGridCfg{HeaderHeight: 30}
	got := dataGridQuickFilterHeight(cfg)
	if got != 30 {
		t.Errorf("got %v, want 30", got)
	}
}

func TestQuickFilterHeightFallsBackToRowHeight(t *testing.T) {
	cfg := &DataGridCfg{RowHeight: 20}
	got := dataGridQuickFilterHeight(cfg)
	if got != 20 {
		t.Errorf("got %v, want 20", got)
	}
}

// --- dataGridColumnChooserHeight ---

func TestColumnChooserHeightClosed(t *testing.T) {
	cfg := &DataGridCfg{RowHeight: 24}
	got := dataGridColumnChooserHeight(cfg, false)
	if got != 24 {
		t.Errorf("closed: got %v, want 24", got)
	}
}

func TestColumnChooserHeightOpen(t *testing.T) {
	cfg := &DataGridCfg{RowHeight: 24}
	got := dataGridColumnChooserHeight(cfg, true)
	if got != 48 {
		t.Errorf("open: got %v, want 48", got)
	}
}

func TestColumnChooserHeightFallsBackToHeaderHeight(t *testing.T) {
	cfg := &DataGridCfg{HeaderHeight: 32}
	got := dataGridColumnChooserHeight(cfg, true)
	if got != 64 {
		t.Errorf("open fallback: got %v, want 64", got)
	}
}

// --- dataGridSelectedRows ---

func TestSelectedRowsReturnsMatchingRows(t *testing.T) {
	rows := []GridRow{
		{ID: "a", Cells: map[string]string{"x": "1"}},
		{ID: "b", Cells: map[string]string{"x": "2"}},
		{ID: "c", Cells: map[string]string{"x": "3"}},
	}
	sel := GridSelection{
		SelectedRowIDs: map[string]bool{"a": true, "c": true},
	}
	got := dataGridSelectedRows(rows, sel)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("got IDs %q, %q, want a, c", got[0].ID, got[1].ID)
	}
}

func TestSelectedRowsEmptySelection(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{SelectedRowIDs: map[string]bool{}}
	got := dataGridSelectedRows(rows, sel)
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestSelectedRowsNilSelection(t *testing.T) {
	rows := []GridRow{{ID: "a"}}
	sel := GridSelection{}
	got := dataGridSelectedRows(rows, sel)
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestSelectedRowsUsesAutoID(t *testing.T) {
	rows := []GridRow{
		{Cells: map[string]string{"name": "first"}},
		{Cells: map[string]string{"name": "second"}},
	}
	autoID0 := dataGridRowID(rows[0], 0)
	sel := GridSelection{
		SelectedRowIDs: map[string]bool{autoID0: true},
	}
	got := dataGridSelectedRows(rows, sel)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
}

// --- dataGridIndicatorTextStyle ---

func TestIndicatorTextStyleDimsColor(t *testing.T) {
	base := gg.TextStyle{Color: gg.RGBA(255, 255, 255, 255), Size: 14}
	got := dataGridIndicatorTextStyle(base)
	if got.Color.A != dataGridIndicatorAlpha {
		t.Errorf("alpha: got %d, want %d", got.Color.A, dataGridIndicatorAlpha)
	}
	if got.Size != base.Size {
		t.Errorf("size: got %v, want %v", got.Size, base.Size)
	}
}

// --- dataGridDimColor ---

func TestDimColorReducesAlpha(t *testing.T) {
	c := gg.RGBA(100, 150, 200, 255)
	got := dataGridDimColor(c)
	want := gg.RGBA(100, 150, 200, dataGridIndicatorAlpha)
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// --- dataGridHeight ---

func TestDataGridHeightUsesConfigured(t *testing.T) {
	cfg := &DataGridCfg{Height: 400}
	if got := dataGridHeight(cfg); got != 400 {
		t.Errorf("got %v, want 400", got)
	}
}

func TestDataGridHeightFallsBackToMaxHeight(t *testing.T) {
	cfg := &DataGridCfg{MaxHeight: 300}
	if got := dataGridHeight(cfg); got != 300 {
		t.Errorf("got %v, want 300", got)
	}
}

func TestDataGridHeightZeroWhenNone(t *testing.T) {
	cfg := &DataGridCfg{}
	if got := dataGridHeight(cfg); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// --- dataGridPagerEnabled ---

func TestPagerEnabled(t *testing.T) {
	cfg := &DataGridCfg{PageSize: 25}
	if !dataGridPagerEnabled(cfg, 4) {
		t.Fatal("should be enabled with pageSize>0 and pageCount>1")
	}
}

func TestPagerDisabledNoPageSize(t *testing.T) {
	cfg := &DataGridCfg{}
	if dataGridPagerEnabled(cfg, 4) {
		t.Fatal("should be disabled without pageSize")
	}
}

func TestPagerDisabledSinglePage(t *testing.T) {
	cfg := &DataGridCfg{PageSize: 25}
	if dataGridPagerEnabled(cfg, 1) {
		t.Fatal("should be disabled with single page")
	}
}

// --- dataGridPagerHeight ---

func TestPagerHeightUsesRowHeight(t *testing.T) {
	cfg := &DataGridCfg{RowHeight: 26}
	if got := dataGridPagerHeight(cfg); got != 26 {
		t.Errorf("got %v, want 26", got)
	}
}

func TestPagerHeightFallsBackToHeaderHeight(t *testing.T) {
	cfg := &DataGridCfg{HeaderHeight: 30}
	if got := dataGridPagerHeight(cfg); got != 30 {
		t.Errorf("got %v, want 30", got)
	}
}

// --- dataGridPagerPadding ---

func TestPagerPaddingTakesMax(t *testing.T) {
	cfg := &DataGridCfg{
		PaddingCell:   gg.SomeP(1, 2, 3, 4),
		PaddingFilter: gg.SomeP(5, 8, 7, 6),
	}
	got := dataGridPagerPadding(cfg)
	if got.Left != 6 || got.Right != 8 || got.Top != 5 || got.Bottom != 7 {
		t.Errorf("got %+v, want Left=6 Right=8 Top=5 Bottom=7", got)
	}
}

// --- dataGridHeaderHeight ---

func TestHeaderHeightConfigured(t *testing.T) {
	cfg := &DataGridCfg{HeaderHeight: 32}
	if got := dataGridHeaderHeight(cfg); got != 32 {
		t.Errorf("got %v, want 32", got)
	}
}

func TestHeaderHeightFallsBackToRowHeight(t *testing.T) {
	cfg := &DataGridCfg{RowHeight: 22}
	if got := dataGridHeaderHeight(cfg); got != 22 {
		t.Errorf("got %v, want 22", got)
	}
}

// --- dataGridRowHeight ---

func TestRowHeightConfigured(t *testing.T) {
	cfg := &DataGridCfg{RowHeight: 28}
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	if got := dataGridRowHeight(cfg, w); got != 28 {
		t.Errorf("got %v, want 28", got)
	}
}

// --- dataGridStaticTopHeight ---

func TestStaticTopHeightHeaderAndChooser(t *testing.T) {
	cfg := &DataGridCfg{
		RowHeight:         20,
		ShowColumnChooser: true,
	}
	got := dataGridStaticTopHeight(cfg, 0, true, true)
	// open chooser: rowHeight*2 = 40, + header = rowHeight = 20 → 60
	want := float32(60)
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestStaticTopHeightNoHeader(t *testing.T) {
	cfg := &DataGridCfg{
		RowHeight: 20,
	}
	got := dataGridStaticTopHeight(cfg, 0, false, false)
	if got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// --- dataGridFocusID ---

func TestFocusIDIsGridID(t *testing.T) {
	cfg := &DataGridCfg{ID: "mygrid"}
	if got := dataGridFocusID(cfg); got != "mygrid" {
		t.Errorf("got %q, want mygrid", got)
	}
}

// --- dataGridScrollID ---

func TestScrollIDConfigured(t *testing.T) {
	cfg := &DataGridCfg{ID: "99"}
	if got := dataGridScrollID(cfg); got != "99:scroll" {
		t.Errorf("got %q, want 99:scroll", got)
	}
}

func TestScrollIDDerived(t *testing.T) {
	cfg := &DataGridCfg{ID: "mygrid"}
	got := dataGridScrollID(cfg)
	if got == "" {
		t.Fatal("should derive non-empty scroll ID")
	}
}

// --- dataGridVisibleRangeForScroll ---

func TestVisibleRangeForScroll(t *testing.T) {
	gotFirst, gotLast := dataGridVisibleRangeForScroll(50, 200, 25, 100, 0, 2)
	// scrollY=50, viewportH=200, rowH=25 → visible rows 2..11, buffer → 0..13
	if gotFirst != 0 {
		t.Errorf("first: got %d, want 0", gotFirst)
	}
	if gotLast != 13 {
		t.Errorf("last: got %d, want 13", gotLast)
	}
}

func TestVisibleRangeZeroRows(t *testing.T) {
	gotFirst, gotLast := dataGridVisibleRangeForScroll(0, 200, 25, 0, 0, 2)
	if gotFirst != 0 || gotLast != -1 {
		t.Errorf("got (%d, %d), want (0, -1)", gotFirst, gotLast)
	}
}

func TestVisibleRangeZeroViewport(t *testing.T) {
	gotFirst, gotLast := dataGridVisibleRangeForScroll(0, 0, 25, 10, 0, 2)
	if gotFirst != 0 || gotLast != -1 {
		t.Errorf("got (%d, %d), want (0, -1)", gotFirst, gotLast)
	}
}

func TestVisibleRangeWithStaticTop(t *testing.T) {
	gotFirst, _ := dataGridVisibleRangeForScroll(90, 200, 25, 100, 40, 1)
	// scrollY=90, staticTop=40 → bodyScroll=50
	if gotFirst < 0 {
		t.Errorf("first: got %d, want >= 0", gotFirst)
	}
}

// --- dataGridDetailRowExpanded ---

func TestDetailRowExpanded(t *testing.T) {
	cfg := &DataGridCfg{
		DetailExpandedRowIDs: map[string]bool{"r1": true},
	}
	if !dataGridDetailRowExpanded(cfg, "r1") {
		t.Fatal("r1 should be expanded")
	}
	if dataGridDetailRowExpanded(cfg, "r2") {
		t.Fatal("r2 should not be expanded")
	}
	if dataGridDetailRowExpanded(cfg, "") {
		t.Fatal("empty rowID should not be expanded")
	}
}

// --- dataGridHasSource ---

func TestHasSource(t *testing.T) {
	cfg := &DataGridCfg{}
	if dataGridHasSource(cfg) {
		t.Fatal("should be false without DataSource")
	}
}

// --- dataGridPageRowIndices ---

func TestPageRowIndices(t *testing.T) {
	got := dataGridPageRowIndices(2, 5)
	if len(got) != 3 {
		t.Fatalf("got %d indices, want 3", len(got))
	}
	if got[0] != 2 || got[1] != 3 || got[2] != 4 {
		t.Errorf("got %v, want [2 3 4]", got)
	}
}

func TestPageRowIndicesEmpty(t *testing.T) {
	got := dataGridPageRowIndices(0, 0)
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// --- dataGridVisibleRowIndices ---

func TestVisibleRowIndicesUsesPageIndices(t *testing.T) {
	got := dataGridVisibleRowIndices(100, []int{3, 4, 5})
	if len(got) != 3 || got[0] != 3 {
		t.Errorf("got %v, want [3 4 5]", got)
	}
}

func TestVisibleRowIndicesFallsBackToAllRows(t *testing.T) {
	got := dataGridVisibleRowIndices(3, nil)
	if len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Errorf("got %v, want [0 1 2]", got)
	}
}

// --- dataGridHasRowID ---

func TestHasRowID(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	if !dataGridHasRowID(rows, "a") {
		t.Fatal("should find 'a'")
	}
	if dataGridHasRowID(rows, "c") {
		t.Fatal("should not find 'c'")
	}
	if dataGridHasRowID(rows, "") {
		t.Fatal("should not find empty")
	}
}

// --- dataGridActiveRowIndex ---

func TestActiveRowIndex(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	sel := GridSelection{ActiveRowID: "b"}
	got := dataGridActiveRowIndex(rows, sel)
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestActiveRowIndexFallbackToFirst(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{ActiveRowID: "z"}
	got := dataGridActiveRowIndex(rows, sel)
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestActiveRowIndexEmptyRows(t *testing.T) {
	got := dataGridActiveRowIndex(nil, GridSelection{ActiveRowID: "a"})
	if got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

// --- dataGridActiveRowIndexStrict ---

func TestActiveRowIndexStrict(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{ActiveRowID: "b"}
	got := dataGridActiveRowIndexStrict(rows, sel)
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestActiveRowIndexStrictFallbackToSelected(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{
		ActiveRowID:    "",
		SelectedRowIDs: map[string]bool{"b": true},
	}
	got := dataGridActiveRowIndexStrict(rows, sel)
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestActiveRowIndexStrictNotFound(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{}
	got := dataGridActiveRowIndexStrict(rows, sel)
	if got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

// --- dataGridPageRows ---

func TestPageRows(t *testing.T) {
	cfg := &DataGridCfg{Height: 200}
	got := dataGridPageRows(cfg, 25)
	if got != 8 {
		t.Errorf("got %d, want 8", got)
	}
}

func TestPageRowsMinOne(t *testing.T) {
	cfg := &DataGridCfg{Height: 200}
	got := dataGridPageRows(cfg, 0)
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

// --- dataGridEditingEnabled ---

func TestEditingEnabled(t *testing.T) {
	cfg := &DataGridCfg{
		OnCellEdit: func(GridCellEdit, *gg.Event, *gg.Window) {},
		Columns: []GridColumnCfg{
			{ID: "a", Editable: true},
		},
	}
	if !dataGridEditingEnabled(cfg) {
		t.Fatal("should be enabled with editable column and callback")
	}
}

func TestEditingDisabledNoCallback(t *testing.T) {
	cfg := &DataGridCfg{
		Columns: []GridColumnCfg{{ID: "a", Editable: true}},
	}
	if dataGridEditingEnabled(cfg) {
		t.Fatal("should be disabled without OnCellEdit or CRUD toolbar")
	}
}

func TestEditingDisabledNoEditableColumns(t *testing.T) {
	cfg := &DataGridCfg{
		OnCellEdit: func(GridCellEdit, *gg.Event, *gg.Window) {},
		Columns:    []GridColumnCfg{{ID: "a"}},
	}
	if dataGridEditingEnabled(cfg) {
		t.Fatal("should be disabled without editable columns")
	}
}

// --- dataGridCrudEnabled ---

func TestCrudEnabled(t *testing.T) {
	cfg := &DataGridCfg{ShowCRUDToolbar: true}
	if !dataGridCrudEnabled(cfg) {
		t.Fatal("should be enabled")
	}
}

func TestCrudDisabled(t *testing.T) {
	cfg := &DataGridCfg{}
	if dataGridCrudEnabled(cfg) {
		t.Fatal("should be disabled")
	}
}
