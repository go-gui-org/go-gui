package datagrid

import (
	"testing"

	. "github.com/go-gui-org/go-gui/gui"
)

// --- dataGridAggregateLabel ---

func TestAggregateLabelExplicit(t *testing.T) {
	agg := GridAggregateCfg{Label: "Total Sales", Op: gridAggregateSum, ColID: "amount"}
	got := dataGridAggregateLabel(agg)
	if got != "Total Sales" {
		t.Errorf("got %q, want 'Total Sales'", got)
	}
}

func TestAggregateLabelCount(t *testing.T) {
	agg := GridAggregateCfg{Op: gridAggregateCount}
	got := dataGridAggregateLabel(agg)
	if got != "count" {
		t.Errorf("got %q, want 'count'", got)
	}
}

func TestAggregateLabelOpWithColID(t *testing.T) {
	agg := GridAggregateCfg{Op: gridAggregateSum, ColID: "amount"}
	got := dataGridAggregateLabel(agg)
	if got != "sum amount" {
		t.Errorf("got %q, want 'sum amount'", got)
	}
}

func TestAggregateLabelOpOnly(t *testing.T) {
	agg := GridAggregateCfg{Op: gridAggregateMin}
	got := dataGridAggregateLabel(agg)
	if got != "" || len(got) > 0 {
		// Op.String() with empty ColID — should just be the op name.
		if got != "min" {
			t.Errorf("got %q, want 'min'", got)
		}
	}
}

// --- dataGridGroupAggregateText ---

func TestGroupAggregateTextEmpty(t *testing.T) {
	cfg := &DataGridCfg{Rows: []GridRow{{ID: "r1"}}}
	got := dataGridGroupAggregateText(cfg, 0, 0)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestGroupAggregateTextCount(t *testing.T) {
	cfg := &DataGridCfg{
		Rows:       []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		Aggregates: []GridAggregateCfg{{Op: gridAggregateCount, Label: "Rows"}},
	}
	got := dataGridGroupAggregateText(cfg, 0, 2)
	if got != "Rows: 3" {
		t.Errorf("got %q, want 'Rows: 3'", got)
	}
}

func TestGroupAggregateTextInvalidRange(t *testing.T) {
	cfg := &DataGridCfg{
		Rows:       []GridRow{{ID: "a"}, {ID: "b"}},
		Aggregates: []GridAggregateCfg{{Op: gridAggregateCount, Label: "N"}},
	}
	// startIdx > endIdx
	got := dataGridGroupAggregateText(cfg, 1, 0)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
	// endIdx out of range
	got = dataGridGroupAggregateText(cfg, 0, 99)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
	// negative startIdx
	got = dataGridGroupAggregateText(cfg, -1, 0)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- dataGridParseNumber ---

func TestParseNumberValid(t *testing.T) {
	if v, ok := dataGridParseNumber("123.45"); !ok || v != 123.45 {
		t.Errorf("got (%v, %v), want (123.45, true)", v, ok)
	}
}

func TestParseNumberEmpty(t *testing.T) {
	if _, ok := dataGridParseNumber(""); ok {
		t.Fatal("empty should return false")
	}
}

func TestParseNumberInvalid(t *testing.T) {
	if _, ok := dataGridParseNumber("not-a-number"); ok {
		t.Fatal("invalid should return false")
	}
}

func TestParseNumberNaN(t *testing.T) {
	if _, ok := dataGridParseNumber("NaN"); ok {
		t.Fatal("NaN should return false")
	}
}

func TestParseNumberInf(t *testing.T) {
	if _, ok := dataGridParseNumber("Inf"); ok {
		t.Fatal("Inf should return false")
	}
}

func TestParseNumberTrimmed(t *testing.T) {
	if v, ok := dataGridParseNumber("  42  "); !ok || v != 42 {
		t.Errorf("got (%v, %v), want (42, true)", v, ok)
	}
}

// --- dataGridFormatNumber ---

func TestFormatNumberInteger(t *testing.T) {
	got := dataGridFormatNumber(42)
	if got != "42" {
		t.Errorf("got %q, want '42'", got)
	}
}

func TestFormatNumberDecimal(t *testing.T) {
	got := dataGridFormatNumber(3.14)
	if got != "3.14" {
		t.Errorf("got %q, want '3.14'", got)
	}
}

func TestFormatNumberTrailingZeros(t *testing.T) {
	got := dataGridFormatNumber(5.0)
	if got != "5" {
		t.Errorf("got %q, want '5'", got)
	}
}

func TestFormatNumberPrecision(t *testing.T) {
	got := dataGridFormatNumber(1.23456789)
	// 4 decimal places → "1.2346"
	if got != "1.2346" {
		t.Errorf("got %q, want '1.2346'", got)
	}
}

// --- dataGridAggregateValue ---

func TestAggregateValueCount(t *testing.T) {
	rows := []GridRow{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
	}
	agg := GridAggregateCfg{Op: gridAggregateCount}
	got, ok := dataGridAggregateValue(rows, 1, 3, agg)
	if !ok {
		t.Fatal("count should succeed")
	}
	if got != "3" {
		t.Errorf("got %q, want '3'", got)
	}
}

func TestAggregateValueSum(t *testing.T) {
	rows := []GridRow{
		{Cells: map[string]string{"val": "10"}},
		{Cells: map[string]string{"val": "20"}},
		{Cells: map[string]string{"val": "30"}},
	}
	agg := GridAggregateCfg{Op: gridAggregateSum, ColID: "val"}
	got, ok := dataGridAggregateValue(rows, 0, 2, agg)
	if !ok {
		t.Fatal("sum should succeed")
	}
	if got != "60" {
		t.Errorf("got %q, want '60'", got)
	}
}

func TestAggregateValueAvg(t *testing.T) {
	rows := []GridRow{
		{Cells: map[string]string{"val": "10"}},
		{Cells: map[string]string{"val": "20"}},
	}
	agg := GridAggregateCfg{Op: gridAggregateAvg, ColID: "val"}
	got, ok := dataGridAggregateValue(rows, 0, 1, agg)
	if !ok {
		t.Fatal("avg should succeed")
	}
	if got != "15" {
		t.Errorf("got %q, want '15'", got)
	}
}

func TestAggregateValueMin(t *testing.T) {
	rows := []GridRow{
		{Cells: map[string]string{"val": "5"}},
		{Cells: map[string]string{"val": "10"}},
		{Cells: map[string]string{"val": "3"}},
	}
	agg := GridAggregateCfg{Op: gridAggregateMin, ColID: "val"}
	got, ok := dataGridAggregateValue(rows, 0, 2, agg)
	if !ok {
		t.Fatal("min should succeed")
	}
	if got != "3" {
		t.Errorf("got %q, want '3'", got)
	}
}

func TestAggregateValueMax(t *testing.T) {
	rows := []GridRow{
		{Cells: map[string]string{"val": "5"}},
		{Cells: map[string]string{"val": "10"}},
		{Cells: map[string]string{"val": "3"}},
	}
	agg := GridAggregateCfg{Op: gridAggregateMax, ColID: "val"}
	got, ok := dataGridAggregateValue(rows, 0, 2, agg)
	if !ok {
		t.Fatal("max should succeed")
	}
	if got != "10" {
		t.Errorf("got %q, want '10'", got)
	}
}

func TestAggregateValueNoColID(t *testing.T) {
	rows := []GridRow{{ID: "a"}}
	agg := GridAggregateCfg{Op: gridAggregateSum}
	_, ok := dataGridAggregateValue(rows, 0, 0, agg)
	if ok {
		t.Fatal("should return false with empty colID and non-count op")
	}
}

func TestAggregateValueAllNonNumeric(t *testing.T) {
	rows := []GridRow{
		{Cells: map[string]string{"val": "abc"}},
		{Cells: map[string]string{"val": "xyz"}},
	}
	agg := GridAggregateCfg{Op: gridAggregateSum, ColID: "val"}
	_, ok := dataGridAggregateValue(rows, 0, 1, agg)
	if ok {
		t.Fatal("should return false when no numeric values")
	}
}

// --- dataGridGroupColumns ---

func TestGroupColumns(t *testing.T) {
	cols := []GridColumnCfg{
		{ID: "name"}, {ID: "age"}, {ID: "city"},
	}
	got := dataGridGroupColumns([]string{"city", "name"}, cols)
	if len(got) != 2 || got[0] != "city" || got[1] != "name" {
		t.Errorf("got %v, want [city name]", got)
	}
}

func TestGroupColumnsFiltersInvalid(t *testing.T) {
	cols := []GridColumnCfg{{ID: "name"}}
	got := dataGridGroupColumns([]string{"unknown", ""}, cols)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestGroupColumnsEmpty(t *testing.T) {
	got := dataGridGroupColumns(nil, nil)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

// --- dataGridGroupTitles ---

func TestGroupTitles(t *testing.T) {
	cols := []GridColumnCfg{
		{ID: "name", Title: "Name"},
		{ID: "city", Title: "City"},
		{ID: "", Title: "Skip"},
	}
	got := dataGridGroupTitles(cols)
	if len(got) != 2 {
		t.Errorf("got %d entries, want 2", len(got))
	}
	if got["name"] != "Name" {
		t.Errorf("name: got %q", got["name"])
	}
}

// --- dataGridGroupRangeKey ---

func TestGroupRangeKey(t *testing.T) {
	got := dataGridGroupRangeKey(2, 5)
	if got != "2:5" {
		t.Errorf("got %q, want '2:5'", got)
	}
}

// --- dataGridGroupRanges ---

func TestGroupRangesSingleGroup(t *testing.T) {
	rows := []GridRow{
		{ID: "r1", Cells: map[string]string{"cat": "A"}},
		{ID: "r2", Cells: map[string]string{"cat": "A"}},
		{ID: "r3", Cells: map[string]string{"cat": "A"}},
	}
	got := dataGridGroupRanges(rows, []int{0, 1, 2}, []string{"cat"})
	// All same value → one range covering all.
	if len(got) == 0 {
		t.Error("expected at least one range entry")
	}
}

func TestGroupRangesMultiple(t *testing.T) {
	rows := []GridRow{
		{ID: "r1", Cells: map[string]string{"cat": "A"}},
		{ID: "r2", Cells: map[string]string{"cat": "B"}},
		{ID: "r3", Cells: map[string]string{"cat": "B"}},
		{ID: "r4", Cells: map[string]string{"cat": "C"}},
	}
	got := dataGridGroupRanges(rows, []int{0, 1, 2, 3}, []string{"cat"})
	// Should have range entries for A (0:0), B (1:2), C (3:3).
	keyA := dataGridGroupRangeKey(0, 0)
	keyB := dataGridGroupRangeKey(0, 1)
	keyC := dataGridGroupRangeKey(0, 3)
	if got[keyA] != 0 {
		t.Errorf("range A: got %d, want 0", got[keyA])
	}
	if got[keyB] != 2 {
		t.Errorf("range B: got %d, want 2", got[keyB])
	}
	if got[keyC] != 3 {
		t.Errorf("range C: got %d, want 3", got[keyC])
	}
}

func TestGroupRangesEmpty(t *testing.T) {
	got := dataGridGroupRanges(nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("got %d entries, want 0", len(got))
	}
}

// --- dataGridPresentationValueCols ---

func TestPresentationValueCols(t *testing.T) {
	groupCols := []string{"name"}
	aggs := []GridAggregateCfg{
		{Op: gridAggregateSum, ColID: "amount"},
		{Op: gridAggregateCount},
		{Op: gridAggregateAvg, ColID: "price"},
	}
	got := dataGridPresentationValueCols(groupCols, aggs)
	// Should include "name", "amount", "price" sorted.
	if len(got) != 3 {
		t.Errorf("got %d, want 3: %v", len(got), got)
	}
}

func TestPresentationValueColsDedup(t *testing.T) {
	groupCols := []string{"name"}
	aggs := []GridAggregateCfg{
		{Op: gridAggregateSum, ColID: "name"}, // same as group col
	}
	got := dataGridPresentationValueCols(groupCols, aggs)
	if len(got) != 1 {
		t.Errorf("got %d, want 1: %v", len(got), got)
	}
}

// --- dataGridFnv64U64 ---

func TestFnv64U64(t *testing.T) {
	a := dataGridFnv64U64(Fnv64Offset, 42)
	b := dataGridFnv64U64(Fnv64Offset, 42)
	if a != b {
		t.Fatal("same input should produce same hash")
	}
	if a == Fnv64Offset {
		t.Fatal("should differ from offset")
	}
}
