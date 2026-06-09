package datagrid

import (
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	. "github.com/go-gui-org/go-gui/gui"
)

func makeTestRows(n int) []GridRow {
	rows := make([]GridRow, n)
	for i := range rows {
		rows[i] = GridRow{
			ID: string(rune('a' + i)),
			Cells: map[string]string{
				"name": string(rune('A' + i)),
				"val":  string(rune('0' + i)),
			},
		}
	}
	return rows
}

func TestInMemoryFetchAll(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(5))
	res, err := src.FetchData(GridDataRequest{
		Page: gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReceivedCount != 5 {
		t.Fatalf("count = %d, want 5", res.ReceivedCount)
	}
	if res.HasMore {
		t.Fatal("HasMore should be false")
	}
	if res.RowCount != 5 {
		t.Fatalf("RowCount = %d, want 5", res.RowCount)
	}
}

func TestInMemoryFetchPaginationCursor(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(5))
	res, err := src.FetchData(GridDataRequest{
		Page: gridCursorPageReq{Limit: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReceivedCount != 2 {
		t.Fatalf("count = %d, want 2", res.ReceivedCount)
	}
	if !res.HasMore {
		t.Fatal("HasMore should be true")
	}
	if res.NextCursor == "" {
		t.Fatal("NextCursor should be set")
	}
	// Fetch next page.
	res2, err := src.FetchData(GridDataRequest{
		Page: gridCursorPageReq{
			Cursor: res.NextCursor, Limit: 2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res2.ReceivedCount != 2 {
		t.Fatalf("page2 count = %d, want 2", res2.ReceivedCount)
	}
	if res2.Rows[0].ID != "c" {
		t.Fatalf("page2[0].ID = %q, want c", res2.Rows[0].ID)
	}
}

func TestInMemoryFetchPaginationOffset(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(5))
	res, err := src.FetchData(GridDataRequest{
		Page: gridOffsetPageReq{StartIndex: 1, EndIndex: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReceivedCount != 2 {
		t.Fatalf("count = %d, want 2", res.ReceivedCount)
	}
	if res.Rows[0].ID != "b" {
		t.Fatalf("rows[0].ID = %q, want b", res.Rows[0].ID)
	}
}

func TestInMemoryFetchQuickFilter(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(5))
	res, err := src.FetchData(GridDataRequest{
		Query: GridQueryState{QuickFilter: "c"},
		Page:  gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReceivedCount != 1 {
		t.Fatalf("count = %d, want 1", res.ReceivedCount)
	}
}

func TestInMemoryFetchSort(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(5))
	res, err := src.FetchData(GridDataRequest{
		Query: GridQueryState{
			Sorts: []GridSort{{ColID: "name", Dir: GridSortDesc}},
		},
		Page: gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows[0].Cells["name"] != "E" {
		t.Fatalf("first = %q, want E",
			res.Rows[0].Cells["name"])
	}
}

func TestInMemoryFetchMultiSort(t *testing.T) {
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"a": "x", "b": "2"}},
		{ID: "2", Cells: map[string]string{"a": "x", "b": "1"}},
		{ID: "3", Cells: map[string]string{"a": "y", "b": "1"}},
	}
	src := NewInMemoryDataSource(rows)
	res, err := src.FetchData(GridDataRequest{
		Query: GridQueryState{
			Sorts: []GridSort{
				{ColID: "a", Dir: GridSortAsc},
				{ColID: "b", Dir: GridSortAsc},
			},
		},
		Page: gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows[0].ID != "2" {
		t.Fatalf("first = %q, want 2", res.Rows[0].ID)
	}
	if res.Rows[1].ID != "1" {
		t.Fatalf("second = %q, want 1", res.Rows[1].ID)
	}
}

func TestInMemoryFetchSortStableTies(t *testing.T) {
	rows := []GridRow{
		{ID: "2", Cells: map[string]string{"name": "A"}},
		{ID: "1", Cells: map[string]string{"name": "A"}},
		{ID: "3", Cells: map[string]string{"name": "B"}},
	}
	src := NewInMemoryDataSource(rows)
	res, err := src.FetchData(GridDataRequest{
		Query: GridQueryState{
			Sorts: []GridSort{{ColID: "name", Dir: GridSortAsc}},
		},
		Page: gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows[0].ID != "2" || res.Rows[1].ID != "1" {
		t.Fatalf("tie order changed: got [%s %s], want [2 1]",
			res.Rows[0].ID, res.Rows[1].ID)
	}
}

func TestInMemoryFetchFilterEquals(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(5))
	res, err := src.FetchData(GridDataRequest{
		Query: GridQueryState{
			Filters: []GridFilter{
				{ColID: "name", Op: "equals", Value: "c"},
			},
		},
		Page: gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReceivedCount != 1 {
		t.Fatalf("count = %d, want 1", res.ReceivedCount)
	}
	if res.Rows[0].ID != "c" {
		t.Fatalf("ID = %q, want c", res.Rows[0].ID)
	}
}

func TestInMemoryFetchFilterStartsWith(t *testing.T) {
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"name": "Alpha"}},
		{ID: "2", Cells: map[string]string{"name": "Beta"}},
		{ID: "3", Cells: map[string]string{"name": "Apex"}},
	}
	src := NewInMemoryDataSource(rows)
	res, err := src.FetchData(GridDataRequest{
		Query: GridQueryState{
			Filters: []GridFilter{
				{ColID: "name", Op: "starts_with", Value: "al"},
			},
		},
		Page: gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReceivedCount != 1 {
		t.Fatalf("count = %d, want 1", res.ReceivedCount)
	}
}

func TestInMemoryFetchFilterEndsWith(t *testing.T) {
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"name": "Alpha"}},
		{ID: "2", Cells: map[string]string{"name": "Beta"}},
	}
	src := NewInMemoryDataSource(rows)
	res, err := src.FetchData(GridDataRequest{
		Query: GridQueryState{
			Filters: []GridFilter{
				{ColID: "name", Op: "ends_with", Value: "ta"},
			},
		},
		Page: gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ReceivedCount != 1 {
		t.Fatalf("count = %d, want 1", res.ReceivedCount)
	}
}

func TestInMemoryMutateCreate(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(3))
	res, err := src.MutateData(GridMutationRequest{
		Kind: gridMutationCreate,
		Rows: []GridRow{
			{ID: "new1", Cells: map[string]string{"name": "New"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 {
		t.Fatalf("created = %d, want 1", len(res.Created))
	}
	if len(src.Rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(src.Rows))
	}
}

func TestInMemoryMutateCreateAutoID(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(2))
	res, err := src.MutateData(GridMutationRequest{
		Kind: gridMutationCreate,
		Rows: []GridRow{
			{Cells: map[string]string{"name": "Auto"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created[0].ID == "" {
		t.Fatal("auto-generated ID should be non-empty")
	}
}

func TestInMemoryMutateUpdate(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(3))
	_, err := src.MutateData(GridMutationRequest{
		Kind: gridMutationUpdate,
		Rows: []GridRow{
			{ID: "b", Cells: map[string]string{"name": "Updated"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if src.Rows[1].Cells["name"] != "Updated" {
		t.Fatalf("name = %q", src.Rows[1].Cells["name"])
	}
}

func TestInMemoryMutateUpdateWithEdits(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(3))
	_, err := src.MutateData(GridMutationRequest{
		Kind: gridMutationUpdate,
		Edits: []GridCellEdit{
			{RowID: "a", ColID: "name", Value: "Edited"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if src.Rows[0].Cells["name"] != "Edited" {
		t.Fatalf("name = %q", src.Rows[0].Cells["name"])
	}
}

func TestInMemoryMutateUpdateWithEditsDeterministicOrder(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(3))
	res, err := src.MutateData(GridMutationRequest{
		Kind: gridMutationUpdate,
		Edits: []GridCellEdit{
			{RowID: "c", ColID: "name", Value: "Edited-C"},
			{RowID: "a", ColID: "name", Value: "Edited-A"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Updated) != 2 {
		t.Fatalf("updated len = %d, want 2", len(res.Updated))
	}
	if res.Updated[0].ID != "a" || res.Updated[1].ID != "c" {
		t.Fatalf("updated order = [%s %s], want [a c]",
			res.Updated[0].ID, res.Updated[1].ID)
	}
}

func TestInMemoryMutateDelete(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(5))
	res, err := src.MutateData(GridMutationRequest{
		Kind:   gridMutationDelete,
		RowIDs: []string{"b", "d"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.DeletedIDs) != 2 {
		t.Fatalf("deleted = %d, want 2", len(res.DeletedIDs))
	}
	if len(src.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(src.Rows))
	}
}

func TestAbortSignal(t *testing.T) {
	ctrl := NewGridAbortController()
	if ctrl.Signal.IsAborted() {
		t.Fatal("should not be aborted initially")
	}
	ctrl.Abort()
	if !ctrl.Signal.IsAborted() {
		t.Fatal("should be aborted after Abort()")
	}
}

func TestAbortSignalNil(t *testing.T) {
	var s *GridAbortSignal
	if s.IsAborted() {
		t.Fatal("nil signal should not be aborted")
	}
}

func TestAbortCheckFetch(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(3))
	ctrl := NewGridAbortController()
	ctrl.Abort()
	_, err := src.FetchData(GridDataRequest{
		Page:   gridCursorPageReq{Limit: 100},
		Signal: ctrl.Signal,
	})
	if err == nil {
		t.Fatal("expected abort error")
	}
}

func TestGridQuerySignatureStability(t *testing.T) {
	q := GridQueryState{
		QuickFilter: "test",
		Sorts: []GridSort{
			{ColID: "name", Dir: GridSortAsc},
		},
		Filters: []GridFilter{
			{ColID: "status", Op: "equals", Value: "active"},
		},
	}
	h1 := gridQuerySignature(q)
	h2 := gridQuerySignature(q)
	if h1 != h2 {
		t.Fatalf("unstable: %d != %d", h1, h2)
	}
}

func TestGridQuerySignatureFilterOrderIndependent(t *testing.T) {
	q1 := GridQueryState{
		Filters: []GridFilter{
			{ColID: "a", Op: "eq", Value: "1"},
			{ColID: "b", Op: "eq", Value: "2"},
		},
	}
	q2 := GridQueryState{
		Filters: []GridFilter{
			{ColID: "b", Op: "eq", Value: "2"},
			{ColID: "a", Op: "eq", Value: "1"},
		},
	}
	if gridQuerySignature(q1) != gridQuerySignature(q2) {
		t.Fatal("filter order should not affect signature")
	}
}

func TestGridQuerySignatureSortOrderDependent(t *testing.T) {
	q1 := GridQueryState{
		Sorts: []GridSort{
			{ColID: "a"}, {ColID: "b"},
		},
	}
	q2 := GridQueryState{
		Sorts: []GridSort{
			{ColID: "b"}, {ColID: "a"},
		},
	}
	if gridQuerySignature(q1) == gridQuerySignature(q2) {
		t.Fatal("sort order should affect signature")
	}
}

func TestGridContainsLower(t *testing.T) {
	if !gridContainsLower("Hello World", "world") {
		t.Fatal("should match")
	}
	if gridContainsLower("Hello", "xyz") {
		t.Fatal("should not match")
	}
	if !gridContainsLower("anything", "") {
		t.Fatal("empty needle should match")
	}
}

func TestGridEqualsLower(t *testing.T) {
	if !gridEqualsLower("HELLO", "hello") {
		t.Fatal("should match")
	}
	if gridEqualsLower("Hello", "world") {
		t.Fatal("should not match")
	}
}

func TestGridStartsWithLower(t *testing.T) {
	if !gridStartsWithLower("Hello World", "hello") {
		t.Fatal("should match")
	}
}

func TestGridEndsWithLower(t *testing.T) {
	if !gridEndsWithLower("Hello World", "world") {
		t.Fatal("should match")
	}
}

func TestCursorRoundTrip(t *testing.T) {
	cursor := dataGridSourceCursorFromIndex(42)
	idx := dataGridSourceCursorToIndex(cursor)
	if idx != 42 {
		t.Fatalf("got %d, want 42", idx)
	}
}

func TestCursorEmptyString(t *testing.T) {
	idx := dataGridSourceCursorToIndex("")
	if idx != 0 {
		t.Fatalf("got %d, want 0", idx)
	}
}

func TestCursorInvalid(t *testing.T) {
	idx := dataGridSourceCursorToIndex("abc")
	if idx != 0 {
		t.Fatalf("got %d, want 0", idx)
	}
}

func TestDataGridRowID(t *testing.T) {
	r := GridRow{ID: "x"}
	if dataGridRowID(r, 5) != "x" {
		t.Fatal("should use ID")
	}
	r2 := GridRow{}
	if dataGridRowID(r2, 5) != "5" {
		t.Fatal("should use index")
	}
}

func TestRowCountUnknown(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(3))
	src.RowCountKnown = false
	res, err := src.FetchData(GridDataRequest{
		Page: gridCursorPageReq{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowCount != -1 {
		t.Fatalf("RowCount = %d, want -1", res.RowCount)
	}
}

func TestInMemoryConcurrentFetchMutate(t *testing.T) {
	src := NewInMemoryDataSource(makeTestRows(20))
	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Go(func() {
		for !stop.Load() {
			_, err := src.FetchData(GridDataRequest{
				Page: gridCursorPageReq{Limit: 10},
			})
			if err != nil {
				t.Errorf("fetch error: %v", err)
				return
			}
			runtime.Gosched()
		}
	})

	wg.Go(func() {
		for i := range 200 {
			_, err := src.MutateData(GridMutationRequest{
				Kind: gridMutationUpdate,
				Edits: []GridCellEdit{
					{RowID: "a", ColID: "name", Value: strconv.Itoa(i)},
				},
			})
			if err != nil {
				t.Errorf("mutate error: %v", err)
				return
			}
		}
		stop.Store(true)
	})

	wg.Wait()
}
