package datagrid

import (
	"testing"

	. "github.com/go-gui-org/go-gui/gui"
)

// --- helpers ---

func cols(ids ...string) []GridColumnCfg {
	out := make([]GridColumnCfg, len(ids))
	for i, id := range ids {
		out[i] = GridColumnCfg{ID: id, Title: id}
	}
	return out
}

func colsWithPin(specs ...struct {
	id  string
	pin GridColumnPin
}) []GridColumnCfg {
	out := make([]GridColumnCfg, len(specs))
	for i, s := range specs {
		out[i] = GridColumnCfg{ID: s.id, Title: s.id, Pin: s.pin}
	}
	return out
}

func colIDs(columns []GridColumnCfg) []string {
	out := make([]string, len(columns))
	for i, c := range columns {
		out[i] = c.ID
	}
	return out
}

func strSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- dataGridEffectiveColumns ---

func TestEffectiveColumnsDeclarationOrder(t *testing.T) {
	c := cols("a", "b", "c")
	got := colIDs(dataGridEffectiveColumns(c, nil, nil))
	want := []string{"a", "b", "c"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestEffectiveColumnsCustomOrder(t *testing.T) {
	c := cols("a", "b", "c")
	got := colIDs(dataGridEffectiveColumns(c, []string{"c", "a", "b"}, nil))
	want := []string{"c", "a", "b"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestEffectiveColumnsHidden(t *testing.T) {
	c := cols("a", "b", "c")
	hidden := map[string]bool{"b": true}
	got := colIDs(dataGridEffectiveColumns(c, nil, hidden))
	want := []string{"a", "c"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestEffectiveColumnsAllHiddenFallback(t *testing.T) {
	c := cols("a", "b")
	hidden := map[string]bool{"a": true, "b": true}
	got := dataGridEffectiveColumns(c, nil, hidden)
	if len(got) != 1 {
		t.Fatalf("expected 1 fallback column, got %d", len(got))
	}
	if got[0].ID != "a" {
		t.Fatalf("expected fallback to first column 'a', got %q", got[0].ID)
	}
}

func TestEffectiveColumnsEmptyInput(t *testing.T) {
	got := dataGridEffectiveColumns(nil, nil, nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestEffectiveColumnsPinPartitioning(t *testing.T) {
	c := colsWithPin(
		struct {
			id  string
			pin GridColumnPin
		}{"r1", GridColumnPinRight},
		struct {
			id  string
			pin GridColumnPin
		}{"c1", GridColumnPinNone},
		struct {
			id  string
			pin GridColumnPin
		}{"l1", GridColumnPinLeft},
		struct {
			id  string
			pin GridColumnPin
		}{"c2", GridColumnPinNone},
	)
	got := colIDs(dataGridEffectiveColumns(c, nil, nil))
	want := []string{"l1", "c1", "c2", "r1"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// --- dataGridColumnOrderAndMap (order normalization) ---

func TestNormalizedColumnOrderRespectsOrder(t *testing.T) {
	c := cols("a", "b", "c")
	got, _ := dataGridColumnOrderAndMap(c, []string{"c", "a"})
	want := []string{"c", "a", "b"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormalizedColumnOrderSkipsUnknown(t *testing.T) {
	c := cols("a", "b")
	got, _ := dataGridColumnOrderAndMap(c, []string{"z", "b", "a"})
	want := []string{"b", "a"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormalizedColumnOrderNilColumns(t *testing.T) {
	got, _ := dataGridColumnOrderAndMap(nil, []string{"a"})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestNormalizedColumnOrderDeduplicates(t *testing.T) {
	c := cols("a", "b")
	got, _ := dataGridColumnOrderAndMap(c, []string{"a", "a", "b"})
	want := []string{"a", "b"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// --- dataGridColumnOrderMove ---

func TestColumnOrderMoveRight(t *testing.T) {
	order := []string{"a", "b", "c"}
	got := dataGridColumnOrderMove(order, "a", 1)
	want := []string{"b", "a", "c"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestColumnOrderMoveLeft(t *testing.T) {
	order := []string{"a", "b", "c"}
	got := dataGridColumnOrderMove(order, "c", -1)
	want := []string{"a", "c", "b"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestColumnOrderMoveClampLeft(t *testing.T) {
	order := []string{"a", "b", "c"}
	got := dataGridColumnOrderMove(order, "a", -5)
	// Already at index 0, clamped target = 0, no change.
	want := []string{"a", "b", "c"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestColumnOrderMoveClampRight(t *testing.T) {
	order := []string{"a", "b", "c"}
	got := dataGridColumnOrderMove(order, "c", 10)
	// Already at last index, clamped target = 2, no change.
	want := []string{"a", "b", "c"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestColumnOrderMoveMissingID(t *testing.T) {
	order := []string{"a", "b", "c"}
	got := dataGridColumnOrderMove(order, "z", 1)
	want := []string{"a", "b", "c"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestColumnOrderMoveZeroDelta(t *testing.T) {
	order := []string{"a", "b"}
	got := dataGridColumnOrderMove(order, "a", 0)
	want := []string{"a", "b"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// --- dataGridPartitionPins ---

func TestPartitionPinsLeftCenterRight(t *testing.T) {
	input := []GridColumnCfg{
		{ID: "c1", Pin: GridColumnPinNone},
		{ID: "r1", Pin: GridColumnPinRight},
		{ID: "l1", Pin: GridColumnPinLeft},
		{ID: "l2", Pin: GridColumnPinLeft},
		{ID: "c2", Pin: GridColumnPinNone},
		{ID: "r2", Pin: GridColumnPinRight},
	}
	got := colIDs(dataGridPartitionPins(input))
	want := []string{"l1", "l2", "c1", "c2", "r1", "r2"}
	if !strSliceEq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPartitionPinsEmpty(t *testing.T) {
	got := dataGridPartitionPins(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

// --- dataGridColumnNextPin ---

func TestColumnNextPinCycle(t *testing.T) {
	tests := []struct {
		in   GridColumnPin
		want GridColumnPin
	}{
		{GridColumnPinNone, GridColumnPinLeft},
		{GridColumnPinLeft, GridColumnPinRight},
		{GridColumnPinRight, GridColumnPinNone},
	}
	for _, tt := range tests {
		got := dataGridColumnNextPin(tt.in)
		if got != tt.want {
			t.Errorf("nextPin(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// --- dataGridToggleSort ---

func TestToggleSortSingleAscDescNone(t *testing.T) {
	q := GridQueryState{}

	// First toggle: asc
	q = dataGridToggleSort(q, "name", false, false)
	if len(q.Sorts) != 1 || q.Sorts[0].Dir != GridSortAsc {
		t.Fatalf("expected asc, got %+v", q.Sorts)
	}

	// Second toggle: desc
	q = dataGridToggleSort(q, "name", false, false)
	if len(q.Sorts) != 1 || q.Sorts[0].Dir != GridSortDesc {
		t.Fatalf("expected desc, got %+v", q.Sorts)
	}

	// Third toggle: remove
	q = dataGridToggleSort(q, "name", false, false)
	if len(q.Sorts) != 0 {
		t.Fatalf("expected no sorts, got %+v", q.Sorts)
	}
}

func TestToggleSortMultiAppend(t *testing.T) {
	q := GridQueryState{}

	// Add first sort (multi + append)
	q = dataGridToggleSort(q, "name", true, true)
	if len(q.Sorts) != 1 || q.Sorts[0].ColID != "name" {
		t.Fatalf("expected [name asc], got %+v", q.Sorts)
	}

	// Append second sort
	q = dataGridToggleSort(q, "age", true, true)
	if len(q.Sorts) != 2 {
		t.Fatalf("expected 2 sorts, got %+v", q.Sorts)
	}
	if q.Sorts[0].ColID != "name" || q.Sorts[1].ColID != "age" {
		t.Fatalf("unexpected order %+v", q.Sorts)
	}

	// Toggle first to desc
	q = dataGridToggleSort(q, "name", true, true)
	if q.Sorts[0].Dir != GridSortDesc {
		t.Fatalf("expected name desc, got %+v", q.Sorts[0])
	}
	if len(q.Sorts) != 2 {
		t.Fatalf("expected 2 sorts preserved, got %d", len(q.Sorts))
	}
}

func TestToggleSortPreservesQuickFilter(t *testing.T) {
	q := GridQueryState{QuickFilter: "hello"}
	q = dataGridToggleSort(q, "x", false, false)
	if q.QuickFilter != "hello" {
		t.Fatalf("QuickFilter lost: %q", q.QuickFilter)
	}
}

// --- dataGridQuerySetFilter ---

func TestQuerySetFilterAdd(t *testing.T) {
	q := GridQueryState{}
	q = dataGridQuerySetFilter(q, "name", "alice")
	if len(q.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(q.Filters))
	}
	if q.Filters[0].ColID != "name" || q.Filters[0].Value != "alice" ||
		q.Filters[0].Op != "contains" {
		t.Fatalf("unexpected filter %+v", q.Filters[0])
	}
}

func TestQuerySetFilterUpdate(t *testing.T) {
	q := GridQueryState{
		Filters: []GridFilter{
			{ColID: "name", Op: "equals", Value: "old"},
		},
	}
	q = dataGridQuerySetFilter(q, "name", "new")
	if len(q.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(q.Filters))
	}
	if q.Filters[0].Value != "new" {
		t.Fatalf("expected updated value 'new', got %q", q.Filters[0].Value)
	}
	// Op should be preserved.
	if q.Filters[0].Op != "equals" {
		t.Fatalf("expected preserved op 'equals', got %q", q.Filters[0].Op)
	}
}

func TestQuerySetFilterRemoveEmpty(t *testing.T) {
	q := GridQueryState{
		Filters: []GridFilter{
			{ColID: "name", Op: "contains", Value: "alice"},
		},
	}
	q = dataGridQuerySetFilter(q, "name", "  ")
	if len(q.Filters) != 0 {
		t.Fatalf("expected 0 filters, got %d", len(q.Filters))
	}
}

func TestQuerySetFilterRemoveNonExistent(t *testing.T) {
	q := GridQueryState{}
	q = dataGridQuerySetFilter(q, "name", "")
	if len(q.Filters) != 0 {
		t.Fatalf("expected 0 filters, got %d", len(q.Filters))
	}
}

// --- dataGridClampWidth ---

func TestClampWidthDefaults(t *testing.T) {
	col := GridColumnCfg{ID: "a"}
	// Default min=60, max=600. Width 100 is in range.
	got := dataGridClampWidth(col, 100)
	if got != 100 {
		t.Fatalf("expected 100, got %f", got)
	}
}

func TestClampWidthBelowMin(t *testing.T) {
	col := GridColumnCfg{ID: "a", MinWidth: SomeF(80)}
	got := dataGridClampWidth(col, 50)
	if got != 80 {
		t.Fatalf("expected 80, got %f", got)
	}
}

func TestClampWidthAboveMax(t *testing.T) {
	col := GridColumnCfg{ID: "a", MaxWidth: SomeF(200)}
	got := dataGridClampWidth(col, 300)
	if got != 200 {
		t.Fatalf("expected 200, got %f", got)
	}
}

func TestClampWidthMaxBelowMin(t *testing.T) {
	// When MaxWidth < MinWidth, max is raised to min.
	col := GridColumnCfg{ID: "a", MinWidth: SomeF(100), MaxWidth: SomeF(50)}
	got := dataGridClampWidth(col, 80)
	// maxW becomes 100 (=minW), so clamped to [100,100].
	if got != 100 {
		t.Fatalf("expected 100, got %f", got)
	}
}

func TestClampWidthDefaultMinMax(t *testing.T) {
	col := GridColumnCfg{ID: "a"}
	// Below default min=60.
	if got := dataGridClampWidth(col, 10); got != 60 {
		t.Fatalf("expected 60, got %f", got)
	}
	// Above default max=600.
	if got := dataGridClampWidth(col, 999); got != 600 {
		t.Fatalf("expected 600, got %f", got)
	}
}

// --- dataGridVisibleColumnCount ---

func TestVisibleColumnCount(t *testing.T) {
	c := cols("a", "b", "c", "d")
	hidden := map[string]bool{"b": true, "d": true}
	got := dataGridVisibleColumnCount(c, hidden)
	if got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
}

func TestVisibleColumnCountNoneHidden(t *testing.T) {
	c := cols("a", "b", "c")
	got := dataGridVisibleColumnCount(c, nil)
	if got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}

func TestVisibleColumnCountSkipsEmptyID(t *testing.T) {
	c := []GridColumnCfg{{ID: ""}, {ID: "a"}, {ID: "b"}}
	got := dataGridVisibleColumnCount(c, nil)
	if got != 2 {
		t.Fatalf("expected 2 (skip empty ID), got %d", got)
	}
}

// --- dataGridNextHiddenColumns ---

func TestNextHiddenColumnsHide(t *testing.T) {
	c := cols("a", "b", "c")
	hidden := map[string]bool{}
	got := dataGridNextHiddenColumns(hidden, "b", c)
	if !got["b"] {
		t.Fatalf("expected b hidden, got %v", got)
	}
}

func TestNextHiddenColumnsUnhide(t *testing.T) {
	c := cols("a", "b", "c")
	hidden := map[string]bool{"b": true}
	got := dataGridNextHiddenColumns(hidden, "b", c)
	if got["b"] {
		t.Fatalf("expected b visible, got %v", got)
	}
}

func TestNextHiddenColumnsMin1Guard(t *testing.T) {
	c := cols("a", "b")
	hidden := map[string]bool{"a": true}
	// Only "b" visible. Hiding "b" should be blocked.
	got := dataGridNextHiddenColumns(hidden, "b", c)
	if got["b"] {
		t.Fatalf("should not hide last visible column")
	}
}

func TestNextHiddenColumnsEmptyID(t *testing.T) {
	c := cols("a", "b")
	hidden := map[string]bool{}
	got := dataGridNextHiddenColumns(hidden, "", c)
	if len(got) != 0 {
		t.Fatalf("expected empty map for empty colID, got %v", got)
	}
}

func TestNextHiddenColumnsDoesNotMutateInput(t *testing.T) {
	c := cols("a", "b", "c")
	hidden := map[string]bool{"a": true}
	_ = dataGridNextHiddenColumns(hidden, "b", c)
	if hidden["b"] {
		t.Fatalf("original map mutated")
	}
}
