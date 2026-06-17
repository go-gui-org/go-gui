package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- boolDefault ---

func TestBoolDefaultNilReturnsDefault(t *testing.T) {
	t.Parallel()
	if got := boolDefault(nil, true); !got {
		t.Error("nil pointer should return default true")
	}
	if got := boolDefault(nil, false); got {
		t.Error("nil pointer should return default false")
	}
}

func TestBoolDefaultNonNilReturnsValue(t *testing.T) {
	t.Parallel()
	if got := boolDefault(new(bool), true); got {
		t.Error("non-nil false should return false")
	}
	v := true
	if got := boolDefault(&v, false); !got {
		t.Error("non-nil true should return true")
	}
}

// --- dataGridRowID ---

func TestDataGridRowIDExplicit(t *testing.T) {
	t.Parallel()
	row := GridRow{ID: "abc", Cells: map[string]string{"x": "1"}}
	if got := dataGridRowID(row, 5); got != "abc" {
		t.Errorf("got %q, want %q", got, "abc")
	}
}

func TestDataGridRowIDAutoHash(t *testing.T) {
	t.Parallel()
	row := GridRow{Cells: map[string]string{"col": "val"}}
	got := dataGridRowID(row, 7)
	if got == "7" {
		t.Error("should use auto-hash, not index fallback")
	}
	if len(got) < 10 {
		t.Errorf("auto ID too short: %q", got)
	}
}

func TestDataGridRowIDIndexFallback(t *testing.T) {
	t.Parallel()
	row := GridRow{}
	if got := dataGridRowID(row, 42); got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

// --- dataGridRowAutoID ---

func TestDataGridRowAutoIDStable(t *testing.T) {
	t.Parallel()
	row := GridRow{Cells: map[string]string{"a": "1", "b": "2"}}
	id1 := dataGridRowAutoID(row)
	id2 := dataGridRowAutoID(row)
	if id1 != id2 {
		t.Errorf("unstable: %q != %q", id1, id2)
	}
	if id1 == "" {
		t.Error("should not be empty")
	}
}

func TestDataGridRowAutoIDEmpty(t *testing.T) {
	t.Parallel()
	row := GridRow{Cells: map[string]string{}}
	if got := dataGridRowAutoID(row); got != "" {
		t.Errorf("empty cells should return empty, got %q", got)
	}
}

func TestDataGridRowAutoIDDifferentData(t *testing.T) {
	t.Parallel()
	r1 := GridRow{Cells: map[string]string{"a": "1"}}
	r2 := GridRow{Cells: map[string]string{"a": "2"}}
	if dataGridRowAutoID(r1) == dataGridRowAutoID(r2) {
		t.Error("different data should produce different IDs")
	}
}

// --- dataGridPageBounds ---

func TestDataGridPageBoundsNoPagination(t *testing.T) {
	t.Parallel()
	start, end, pageIdx, pageCount := dataGridPageBounds(10, 0, 0)
	if start != 0 || end != 10 {
		t.Errorf("range: got [%d,%d), want [0,10)", start, end)
	}
	if pageIdx != 0 || pageCount != 1 {
		t.Errorf("page: got idx=%d count=%d, want 0,1",
			pageIdx, pageCount)
	}
}

func TestDataGridPageBoundsFirstPage(t *testing.T) {
	t.Parallel()
	start, end, pageIdx, _ := dataGridPageBounds(25, 10, 0)
	if start != 0 || end != 10 || pageIdx != 0 {
		t.Errorf("first page: got start=%d end=%d idx=%d",
			start, end, pageIdx)
	}
}

func TestDataGridPageBoundsLastPage(t *testing.T) {
	t.Parallel()
	start, end, pageIdx, pageCount := dataGridPageBounds(25, 10, 2)
	if start != 20 || end != 25 {
		t.Errorf("last page range: got [%d,%d), want [20,25)",
			start, end)
	}
	if pageIdx != 2 || pageCount != 3 {
		t.Errorf("page: got idx=%d count=%d, want 2,3",
			pageIdx, pageCount)
	}
}

func TestDataGridPageBoundsClampsBeyondLast(t *testing.T) {
	t.Parallel()
	_, _, pageIdx, pageCount := dataGridPageBounds(25, 10, 99)
	if pageIdx != pageCount-1 {
		t.Errorf("should clamp to last page: got %d, want %d",
			pageIdx, pageCount-1)
	}
}

func TestDataGridPageBoundsZeroRows(t *testing.T) {
	t.Parallel()
	start, end, _, pageCount := dataGridPageBounds(0, 10, 0)
	if start != 0 || end != 0 || pageCount != 1 {
		t.Errorf("zero rows: got start=%d end=%d pages=%d",
			start, end, pageCount)
	}
}

// --- dataGridVisibleRangeForScroll ---

func TestDataGridVisibleRangeTop(t *testing.T) {
	t.Parallel()
	first, last := dataGridVisibleRangeForScroll(
		0, 100, 20, 50, 0, 0)
	if first != 0 {
		t.Errorf("first: got %d, want 0", first)
	}
	// visibleRows = int(100/20)+1 = 6, last = 0+6 = 6
	if last != 6 {
		t.Errorf("last: got %d, want 6", last)
	}
}

func TestDataGridVisibleRangeMiddle(t *testing.T) {
	t.Parallel()
	// scrollY=200, rowHeight=20 → first row index = 10
	first, last := dataGridVisibleRangeForScroll(
		200, 100, 20, 50, 0, 2)
	if first != 8 { // 10-2 buffer
		t.Errorf("first: got %d, want 8", first)
	}
	// last = 10+6+2 = 18
	if last != 18 {
		t.Errorf("last: got %d, want 18", last)
	}
}

func TestDataGridVisibleRangeBottom(t *testing.T) {
	t.Parallel()
	// scrollY large enough to push past end; 20 rows total
	first, last := dataGridVisibleRangeForScroll(
		1000, 100, 20, 20, 0, 0)
	if last != 19 {
		t.Errorf("last should clamp to 19, got %d", last)
	}
	if first > last {
		t.Error("first should not exceed last")
	}
}

func TestDataGridVisibleRangeEmpty(t *testing.T) {
	t.Parallel()
	first, last := dataGridVisibleRangeForScroll(
		0, 100, 20, 0, 0, 0)
	if first != 0 || last != -1 {
		t.Errorf("empty: got [%d,%d], want [0,-1]", first, last)
	}
}

// --- dataGridPresentationSignature ---

func TestDataGridPresentationSignatureStable(t *testing.T) {
	t.Parallel()
	cfg := &DataGridCfg{
		ID:   "g1",
		Rows: []GridRow{{Cells: map[string]string{"a": "1"}}},
	}
	cols := []GridColumnCfg{{ID: "a", Title: "A"}}
	idx := []int{0}
	s1 := dataGridPresentationSignature(cfg, cols, idx, nil, nil, nil)
	s2 := dataGridPresentationSignature(cfg, cols, idx, nil, nil, nil)
	if s1 != s2 {
		t.Errorf("unstable: %d != %d", s1, s2)
	}
}

func TestDataGridPresentationSignatureDiffers(t *testing.T) {
	t.Parallel()
	cfg1 := &DataGridCfg{
		ID:      "g1",
		GroupBy: []string{"a"},
		Rows:    []GridRow{{Cells: map[string]string{"a": "1"}}},
	}
	cfg2 := &DataGridCfg{
		ID:      "g1",
		GroupBy: []string{"a"},
		Rows:    []GridRow{{Cells: map[string]string{"a": "2"}}},
	}
	cols := []GridColumnCfg{{ID: "a", Title: "A"}}
	idx := []int{0}
	groupCols := []string{"a"}
	valueCols := []string{"a"}
	groupTitles := map[string]string{"a": "A"}
	s1 := dataGridPresentationSignature(cfg1, cols, idx, groupCols, valueCols, groupTitles)
	s2 := dataGridPresentationSignature(cfg2, cols, idx, groupCols, valueCols, groupTitles)
	if s1 == s2 {
		t.Error("different data should produce different signature")
	}
}

func TestDataGridPresentationSignatureFlatIgnoresCellValues(t *testing.T) {
	t.Parallel()
	cfg1 := &DataGridCfg{
		ID:   "g1",
		Rows: []GridRow{{ID: "r1", Cells: map[string]string{"a": "1"}}},
	}
	cfg2 := &DataGridCfg{
		ID:   "g1",
		Rows: []GridRow{{ID: "r1", Cells: map[string]string{"a": "2"}}},
	}
	cols := []GridColumnCfg{{ID: "a", Title: "A"}}
	idx := []int{0}
	s1 := dataGridPresentationSignature(cfg1, cols, idx, nil, nil, nil)
	s2 := dataGridPresentationSignature(cfg2, cols, idx, nil, nil, nil)
	if s1 != s2 {
		t.Error("flat presentation signature should ignore cell value changes")
	}
}

// --- dataGridBuildPresentation (via dataGridPresentationRows) ---

func TestDataGridBuildPresentationFlat(t *testing.T) {
	t.Parallel()
	cfg := &DataGridCfg{
		Rows: []GridRow{
			{ID: "r0", Cells: map[string]string{"a": "x"}},
			{ID: "r1", Cells: map[string]string{"a": "y"}},
		},
	}
	cols := []GridColumnCfg{{ID: "a", Title: "A"}}
	pres := dataGridPresentationRows(cfg, cols, []int{0, 1})
	if len(pres.Rows) != 2 {
		t.Fatalf("rows: got %d, want 2", len(pres.Rows))
	}
	for i, r := range pres.Rows {
		if r.Kind != dataGridDisplayRowData {
			t.Errorf("row %d: kind %d, want data", i, r.Kind)
		}
	}
}

func TestDataGridBuildPresentationGrouped(t *testing.T) {
	t.Parallel()
	cfg := &DataGridCfg{
		GroupBy: []string{"dept"},
		Rows: []GridRow{
			{ID: "r0", Cells: map[string]string{"dept": "eng"}},
			{ID: "r1", Cells: map[string]string{"dept": "eng"}},
			{ID: "r2", Cells: map[string]string{"dept": "sales"}},
		},
	}
	cols := []GridColumnCfg{{ID: "dept", Title: "Dept"}}
	pres := dataGridPresentationRows(cfg, cols, []int{0, 1, 2})

	headers := 0
	data := 0
	for _, r := range pres.Rows {
		switch r.Kind {
		case dataGridDisplayRowGroupHeader:
			headers++
		case dataGridDisplayRowData:
			data++
		}
	}
	if headers != 2 {
		t.Errorf("group headers: got %d, want 2", headers)
	}
	if data != 3 {
		t.Errorf("data rows: got %d, want 3", data)
	}
}

func TestDataGridBuildPresentationDetail(t *testing.T) {
	t.Parallel()
	cfg := &DataGridCfg{
		Rows: []GridRow{
			{ID: "r0", Cells: map[string]string{"a": "1"}},
			{ID: "r1", Cells: map[string]string{"a": "2"}},
		},
		DetailExpandedRowIDs: map[string]bool{"r0": true},
		OnDetailRowView: func(GridRow, *gg.Window) gg.View {
			return nil
		},
	}
	cols := []GridColumnCfg{{ID: "a", Title: "A"}}
	pres := dataGridPresentationRows(cfg, cols, []int{0, 1})

	details := 0
	for _, r := range pres.Rows {
		if r.Kind == dataGridDisplayRowDetail {
			details++
		}
	}
	if details != 1 {
		t.Errorf("detail rows: got %d, want 1", details)
	}
	if len(pres.Rows) != 3 {
		t.Errorf("total rows: got %d, want 3 (2 data + 1 detail)",
			len(pres.Rows))
	}
}

// --- dataGridGroupRanges ---

func TestDataGridGroupRanges(t *testing.T) {
	t.Parallel()
	rows := []GridRow{
		{Cells: map[string]string{"g": "A"}},
		{Cells: map[string]string{"g": "A"}},
		{Cells: map[string]string{"g": "B"}},
		{Cells: map[string]string{"g": "B"}},
		{Cells: map[string]string{"g": "B"}},
	}
	indices := []int{0, 1, 2, 3, 4}
	groupCols := []string{"g"}
	ranges := dataGridGroupRanges(rows, indices, groupCols)

	// Group "A" starts at local 0, ends at local 1.
	if end, ok := ranges["0:0"]; !ok || end != 1 {
		t.Errorf("group A end: got %d ok=%v, want 1", end, ok)
	}
	// Group "B" starts at local 2, ends at local 4.
	if end, ok := ranges["0:2"]; !ok || end != 4 {
		t.Errorf("group B end: got %d ok=%v, want 4", end, ok)
	}
}

func TestDataGridGroupRangesEmpty(t *testing.T) {
	t.Parallel()
	ranges := dataGridGroupRanges(nil, nil, []string{"g"})
	if len(ranges) != 0 {
		t.Errorf("should be empty, got %d entries", len(ranges))
	}
}

// --- dataGridAggregateValue (compute aggregates) ---

func TestDataGridAggregateCount(t *testing.T) {
	t.Parallel()
	rows := []GridRow{
		{Cells: map[string]string{"v": "10"}},
		{Cells: map[string]string{"v": "20"}},
		{Cells: map[string]string{"v": "30"}},
	}
	agg := GridAggregateCfg{Op: GridAggregateCount}
	val, ok := dataGridAggregateValue(rows, 0, 2, agg)
	if !ok || val != "3" {
		t.Errorf("count: got %q ok=%v, want '3'", val, ok)
	}
}

func TestDataGridAggregateSum(t *testing.T) {
	t.Parallel()
	rows := []GridRow{
		{Cells: map[string]string{"v": "10"}},
		{Cells: map[string]string{"v": "20"}},
		{Cells: map[string]string{"v": "30"}},
	}
	agg := GridAggregateCfg{ColID: "v", Op: GridAggregateSum}
	val, ok := dataGridAggregateValue(rows, 0, 2, agg)
	if !ok || val != "60" {
		t.Errorf("sum: got %q ok=%v, want '60'", val, ok)
	}
}

func TestDataGridAggregateAvg(t *testing.T) {
	t.Parallel()
	rows := []GridRow{
		{Cells: map[string]string{"v": "10"}},
		{Cells: map[string]string{"v": "20"}},
		{Cells: map[string]string{"v": "30"}},
	}
	agg := GridAggregateCfg{ColID: "v", Op: GridAggregateAvg}
	val, ok := dataGridAggregateValue(rows, 0, 2, agg)
	if !ok || val != "20" {
		t.Errorf("avg: got %q ok=%v, want '20'", val, ok)
	}
}

func TestDataGridAggregateMin(t *testing.T) {
	t.Parallel()
	rows := []GridRow{
		{Cells: map[string]string{"v": "30"}},
		{Cells: map[string]string{"v": "10"}},
		{Cells: map[string]string{"v": "20"}},
	}
	agg := GridAggregateCfg{ColID: "v", Op: GridAggregateMin}
	val, ok := dataGridAggregateValue(rows, 0, 2, agg)
	if !ok || val != "10" {
		t.Errorf("min: got %q ok=%v, want '10'", val, ok)
	}
}

func TestDataGridAggregateMax(t *testing.T) {
	t.Parallel()
	rows := []GridRow{
		{Cells: map[string]string{"v": "10"}},
		{Cells: map[string]string{"v": "30"}},
		{Cells: map[string]string{"v": "20"}},
	}
	agg := GridAggregateCfg{ColID: "v", Op: GridAggregateMax}
	val, ok := dataGridAggregateValue(rows, 0, 2, agg)
	if !ok || val != "30" {
		t.Errorf("max: got %q ok=%v, want '30'", val, ok)
	}
}

func TestDataGridAggregateNonNumeric(t *testing.T) {
	t.Parallel()
	rows := []GridRow{
		{Cells: map[string]string{"v": "abc"}},
	}
	agg := GridAggregateCfg{ColID: "v", Op: GridAggregateSum}
	_, ok := dataGridAggregateValue(rows, 0, 0, agg)
	if ok {
		t.Error("non-numeric should return ok=false")
	}
}

// --- dataGridActiveRowIndex ---

func TestDataGridActiveRowIndexFound(t *testing.T) {
	t.Parallel()
	rows := []GridRow{
		{ID: "a"}, {ID: "b"}, {ID: "c"},
	}
	sel := GridSelection{ActiveRowID: "b"}
	if got := dataGridActiveRowIndex(rows, sel); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestDataGridActiveRowIndexMissing(t *testing.T) {
	t.Parallel()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{ActiveRowID: "z"}
	// Falls back to 0 when rows exist.
	if got := dataGridActiveRowIndex(rows, sel); got != 0 {
		t.Errorf("got %d, want 0 (fallback)", got)
	}
}

func TestDataGridActiveRowIndexEmpty(t *testing.T) {
	t.Parallel()
	sel := GridSelection{ActiveRowID: "x"}
	if got := dataGridActiveRowIndex(nil, sel); got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

func TestDataGridActiveRowIndexStrict(t *testing.T) {
	t.Parallel()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{ActiveRowID: "z"}
	if got := dataGridActiveRowIndexStrict(rows, sel); got != -1 {
		t.Errorf("strict missing: got %d, want -1", got)
	}
}

func TestDataGridActiveRowIndexSelectedFallback(t *testing.T) {
	t.Parallel()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	sel := GridSelection{
		SelectedRowIDs: map[string]bool{"b": true, "c": true},
	}
	if got := dataGridActiveRowIndex(rows, sel); got != 1 {
		t.Errorf("got %d, want 1 (first selected)", got)
	}
}

// --- dataGridHasRowID ---

func TestDataGridHasRowIDFound(t *testing.T) {
	t.Parallel()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	if !dataGridHasRowID(rows, "b") {
		t.Error("should find 'b'")
	}
}

func TestDataGridHasRowIDNotFound(t *testing.T) {
	t.Parallel()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	if dataGridHasRowID(rows, "z") {
		t.Error("should not find 'z'")
	}
}

func TestDataGridHasRowIDEmpty(t *testing.T) {
	t.Parallel()
	rows := []GridRow{{ID: "a"}}
	if dataGridHasRowID(rows, "") {
		t.Error("empty rowID should return false")
	}
}

// --- dataGridPagerEnabled ---

func TestDataGridPagerEnabledTrue(t *testing.T) {
	t.Parallel()
	cfg := &DataGridCfg{PageSize: 10}
	if !dataGridPagerEnabled(cfg, 3) {
		t.Error("should be enabled with pageCount>1 and pageSize>0")
	}
}

func TestDataGridPagerEnabledSinglePage(t *testing.T) {
	t.Parallel()
	cfg := &DataGridCfg{PageSize: 10}
	if dataGridPagerEnabled(cfg, 1) {
		t.Error("should not be enabled with pageCount=1")
	}
}

func TestDataGridPagerEnabledZeroPageSize(t *testing.T) {
	t.Parallel()
	cfg := &DataGridCfg{PageSize: 0}
	if dataGridPagerEnabled(cfg, 5) {
		t.Error("should not be enabled with pageSize=0")
	}
}

// --- dataGridIndicatorTextStyle ---

func TestDataGridIndicatorTextStyleDimsAlpha(t *testing.T) {
	t.Parallel()
	base := gg.TextStyle{
		Color: gg.RGBA(200, 100, 50, 255),
		Size:  14,
	}
	got := dataGridIndicatorTextStyle(base)
	if got.Color.A != dataGridIndicatorAlpha {
		t.Errorf("alpha: got %d, want %d",
			got.Color.A, dataGridIndicatorAlpha)
	}
	if got.Color.R != 200 || got.Color.G != 100 || got.Color.B != 50 {
		t.Error("RGB channels should be preserved")
	}
	if got.Size != 14 {
		t.Errorf("size: got %f, want 14", got.Size)
	}
}

// --- DataGrid Disabled/Invisible propagation ---

func TestDataGridDisabledPropagates(t *testing.T) {
	t.Parallel()
	w := &gg.Window{}
	v := New(w, DataGridCfg{
		ID:       "dg1",
		Disabled: true,
		Columns:  []GridColumnCfg{{ID: "a", Title: "A"}},
		Rows:     []GridRow{{ID: "r0", Cells: map[string]string{"a": "1"}}},
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck
	if !layout.Shape.Disabled {
		t.Error("outer container should be disabled")
	}
}

func TestDataGridInvisiblePropagates(t *testing.T) {
	t.Parallel()
	w := &gg.Window{}
	v := New(w, DataGridCfg{
		ID:        "dg2",
		Invisible: true,
		Columns:   []GridColumnCfg{{ID: "a", Title: "A"}},
		Rows:      []GridRow{{ID: "r0", Cells: map[string]string{"a": "1"}}},
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck
	if !layout.Shape.Disabled {
		t.Error("invisible should be disabled")
	}
	if !layout.Shape.OverDraw {
		t.Error("invisible should be overdraw")
	}
}

func TestDataGridRowsData(t *testing.T) {
	t.Parallel()
	w := &gg.Window{}
	v := New(w, DataGridCfg{
		ID: "dg-rowsdata",
		RowsData: []map[string]string{
			{"name": "Alice", "age": "30"},
			{"name": "Bob", "age": "25"},
		},
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck
	if len(layout.Children) == 0 {
		t.Fatal("expected children")
	}
}

func TestDataGridRowsDataAutoColumns(t *testing.T) {
	t.Parallel()
	w := &gg.Window{}
	v := New(w, DataGridCfg{
		ID:       "dg-autocol",
		PageSize: 10,
		RowsData: []map[string]string{
			{"name": "Alice", "age": "30"},
		},
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck
	if len(layout.Children) == 0 {
		t.Fatal("expected children")
	}
}

func TestDataGridRowsDataPrecedence(t *testing.T) {
	t.Parallel()
	// RowsData takes precedence over Rows. DataSource still
	// wins over both.
	w := &gg.Window{}
	v := New(w, DataGridCfg{
		ID: "dg-prec",
		RowsData: []map[string]string{
			{"col": "value"},
		},
		Rows: []GridRow{{ID: "ignored", Cells: map[string]string{"x": "y"}}},
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck
	if len(layout.Children) == 0 {
		t.Fatal("expected children")
	}
}

func TestDataGridRowsDataDataSourceWins(t *testing.T) {
	t.Parallel()
	// DataSource takes precedence over RowsData.
	ds := NewInMemoryDataSource([]GridRow{
		{ID: "ds1", Cells: map[string]string{"name": "FromDS"}},
	})
	w := &gg.Window{}
	v := New(w, DataGridCfg{
		ID:         "dg-ds-wins",
		Columns:    []GridColumnCfg{{ID: "name", Title: "Name"}},
		DataSource: ds,
		RowsData: []map[string]string{
			{"name": "FromRowsData"},
		},
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck
	if len(layout.Children) == 0 {
		t.Fatal("expected children from DataSource")
	}
}

func TestDataGridRowsDataEmptyFirstRow(t *testing.T) {
	t.Parallel()
	w := &gg.Window{}
	v := New(w, DataGridCfg{
		ID: "dg-empty-first",
		RowsData: []map[string]string{
			{},
		},
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck
	// Empty first-row map means no auto-generated columns,
	// but a row should still be created.
	if len(layout.Children) == 0 {
		t.Fatal("expected children (empty-first-row)")
	}
}
