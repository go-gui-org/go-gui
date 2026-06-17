package datagrid

import (
	"strconv"
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

func TestEffectivePaginationKindCursorPreferred(t *testing.T) {
	// Cursor preferred + both supported → cursor.
	caps := GridDataCapabilities{
		SupportsCursorPagination: true,
		SupportsOffsetPagination: true,
	}
	got := dataGridSourceEffectivePaginationKind(GridPaginationCursor, caps)
	if got != GridPaginationCursor {
		t.Fatalf("got %d, want GridPaginationCursor", got)
	}
}

func TestEffectivePaginationKindCursorFallbackToOffset(t *testing.T) {
	// Cursor preferred but only offset supported → offset.
	caps := GridDataCapabilities{
		SupportsOffsetPagination: true,
	}
	got := dataGridSourceEffectivePaginationKind(GridPaginationCursor, caps)
	if got != GridPaginationOffset {
		t.Fatalf("got %d, want GridPaginationOffset", got)
	}
}

func TestEffectivePaginationKindCursorNeitherSupported(t *testing.T) {
	// Cursor preferred, nothing supported → none.
	caps := GridDataCapabilities{}
	got := dataGridSourceEffectivePaginationKind(GridPaginationCursor, caps)
	if got != GridPaginationNone {
		t.Fatalf("got %d, want GridPaginationNone", got)
	}
}

func TestEffectivePaginationKindOffsetPreferred(t *testing.T) {
	// Offset preferred + both supported → offset.
	caps := GridDataCapabilities{
		SupportsCursorPagination: true,
		SupportsOffsetPagination: true,
	}
	got := dataGridSourceEffectivePaginationKind(GridPaginationOffset, caps)
	if got != GridPaginationOffset {
		t.Fatalf("got %d, want GridPaginationOffset", got)
	}
}

func TestEffectivePaginationKindOffsetFallbackToCursor(t *testing.T) {
	// Offset preferred but only cursor supported → cursor.
	caps := GridDataCapabilities{
		SupportsCursorPagination: true,
	}
	got := dataGridSourceEffectivePaginationKind(GridPaginationOffset, caps)
	if got != GridPaginationCursor {
		t.Fatalf("got %d, want GridPaginationCursor", got)
	}
}

func TestDataGridPageLimit(t *testing.T) {
	// PageLimit set → use it.
	cfg := &DataGridCfg{PageLimit: 50}
	if got := dataGridPageLimit(cfg); got != 50 {
		t.Fatalf("got %d, want 50", got)
	}
	// PageLimit zero, PageSize set → use PageSize.
	cfg = &DataGridCfg{PageSize: 25}
	if got := dataGridPageLimit(cfg); got != 25 {
		t.Fatalf("got %d, want 25", got)
	}
	// Both zero → default (100).
	cfg = &DataGridCfg{}
	if got := dataGridPageLimit(cfg); got != dataGridDefaultPageLimit {
		t.Fatalf("got %d, want %d", got, dataGridDefaultPageLimit)
	}
}

func TestDataGridSourceRequestKeyCursor(t *testing.T) {
	cfg := &DataGridCfg{PageLimit: 20}
	state := dataGridSourceState{CurrentCursor: "i:10"}
	sig := uint64(42)
	got := dataGridSourceRequestKey(cfg, state, GridPaginationCursor, sig)
	want := "k:cursor|cursor:i:10|limit:20|q:42"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataGridSourceRequestKeyOffset(t *testing.T) {
	cfg := &DataGridCfg{PageLimit: 30}
	state := dataGridSourceState{OffsetStart: 60}
	sig := uint64(7)
	got := dataGridSourceRequestKey(cfg, state, GridPaginationOffset, sig)
	want := "k:offset|start:60|end:90|q:7"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataGridSourceRowsWithStableIDsOffset(t *testing.T) {
	rows := []GridRow{
		{Cells: map[string]string{"a": "x"}},
		{ID: "keep", Cells: map[string]string{"a": "y"}},
		{Cells: map[string]string{"a": "z"}},
	}
	state := dataGridSourceState{OffsetStart: 20}
	got := dataGridSourceRowsWithStableIDs(rows, GridPaginationOffset, state)
	if got[0].ID != "__src_o_20" {
		t.Fatalf("row 0 ID = %q, want %q", got[0].ID, "__src_o_20")
	}
	if got[1].ID != "keep" {
		t.Fatalf("row 1 ID = %q, want %q", got[1].ID, "keep")
	}
	if got[2].ID != "__src_o_22" {
		t.Fatalf("row 2 ID = %q, want %q", got[2].ID, "__src_o_22")
	}
	if rows[0].ID != "" {
		t.Fatal("input rows should not be mutated")
	}
}

func TestDataGridSourceRowsWithStableIDsCursor(t *testing.T) {
	rows := []GridRow{
		{Cells: map[string]string{"a": "x"}},
		{Cells: map[string]string{"a": "y"}},
	}
	state := dataGridSourceState{CurrentCursor: "i:40"}
	got := dataGridSourceRowsWithStableIDs(rows, GridPaginationCursor, state)
	if got[0].ID != "__src_c_40" || got[1].ID != "__src_c_41" {
		t.Fatalf("got IDs [%q,%q], want [__src_c_40,__src_c_41]", got[0].ID, got[1].ID)
	}
}

func TestDataGridSourceRowsWithStableIDsCursorOpaque(t *testing.T) {
	rows := []GridRow{{Cells: map[string]string{"a": "x"}}}
	state := dataGridSourceState{CurrentCursor: "opaque:token"}
	got := dataGridSourceRowsWithStableIDs(rows, GridPaginationCursor, state)
	if got[0].ID == "" {
		t.Fatal("expected non-empty synthetic ID for opaque cursor")
	}
}

func TestDataGridSourceCanPrevCursor(t *testing.T) {
	// Has prev cursor → true.
	state := dataGridSourceState{PrevCursor: "i:0"}
	if !dataGridSourceCanPrev(GridPaginationCursor, state, 10) {
		t.Fatal("expected true with PrevCursor set")
	}
	// Empty prev cursor → false.
	state.PrevCursor = ""
	if dataGridSourceCanPrev(GridPaginationCursor, state, 10) {
		t.Fatal("expected false with empty PrevCursor")
	}
}

func TestDataGridSourceCanPrevOffset(t *testing.T) {
	state := dataGridSourceState{OffsetStart: 20}
	if !dataGridSourceCanPrev(GridPaginationOffset, state, 10) {
		t.Fatal("expected true with OffsetStart > 0")
	}
	state.OffsetStart = 0
	if dataGridSourceCanPrev(GridPaginationOffset, state, 10) {
		t.Fatal("expected false with OffsetStart == 0")
	}
	// Zero pageLimit → false even with positive offset.
	state.OffsetStart = 20
	if dataGridSourceCanPrev(GridPaginationOffset, state, 0) {
		t.Fatal("expected false with pageLimit == 0")
	}
}

func TestDataGridSourceCanNextCursor(t *testing.T) {
	state := dataGridSourceState{NextCursor: "i:50"}
	if !dataGridSourceCanNext(GridPaginationCursor, state, 10) {
		t.Fatal("expected true with NextCursor set")
	}
	state.NextCursor = ""
	if dataGridSourceCanNext(GridPaginationCursor, state, 10) {
		t.Fatal("expected false with empty NextCursor")
	}
}

func TestDataGridSourceCanNextOffset(t *testing.T) {
	// Known row count, more data ahead.
	rc := 100
	state := dataGridSourceState{
		OffsetStart:   0,
		ReceivedCount: 20,
		RowCount:      &rc,
	}
	if !dataGridSourceCanNext(GridPaginationOffset, state, 20) {
		t.Fatal("expected true when more rows remain")
	}
	// At end of known data.
	state.OffsetStart = 80
	state.ReceivedCount = 20
	if dataGridSourceCanNext(GridPaginationOffset, state, 20) {
		t.Fatal("expected false at end of data")
	}
	// Unknown row count but HasMore.
	state = dataGridSourceState{HasMore: true, ReceivedCount: 10}
	if !dataGridSourceCanNext(GridPaginationOffset, state, 10) {
		t.Fatal("expected true with HasMore")
	}
	// Unknown row count, no HasMore, received < pageLimit.
	state = dataGridSourceState{ReceivedCount: 5}
	if dataGridSourceCanNext(GridPaginationOffset, state, 10) {
		t.Fatal("expected false with received < pageLimit")
	}
	// Unknown row count, no HasMore, received >= pageLimit.
	state = dataGridSourceState{ReceivedCount: 10}
	if !dataGridSourceCanNext(GridPaginationOffset, state, 10) {
		t.Fatal("expected true with received >= pageLimit")
	}
}

func TestDataGridSourceRowsTextOffset(t *testing.T) {
	rc := 200
	state := dataGridSourceState{
		OffsetStart:   20,
		ReceivedCount: 50,
		RowCount:      &rc,
	}
	got := dataGridSourceRowsText(GridPaginationOffset, state)
	want := gg.ActiveLocale.StrRows + " 21-70/200"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataGridSourceRowsTextCursorWithIndex(t *testing.T) {
	rc := 100
	state := dataGridSourceState{
		CurrentCursor: "i:10",
		ReceivedCount: 20,
		RowCount:      &rc,
	}
	got := dataGridSourceRowsText(GridPaginationCursor, state)
	want := gg.ActiveLocale.StrRows + " 11-30/100"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataGridSourceRowsTextCursorNoIndex(t *testing.T) {
	// Opaque cursor that doesn't parse as index.
	state := dataGridSourceState{
		CurrentCursor: "abc-opaque",
		ReceivedCount: 15,
	}
	got := dataGridSourceRowsText(GridPaginationCursor, state)
	want := gg.ActiveLocale.StrRows + " 15/?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataGridSourceFormatRows(t *testing.T) {
	rc := 500
	// Normal range.
	got := dataGridSourceFormatRows(10, 25, &rc)
	want := gg.ActiveLocale.StrRows + " 11-35/500"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// End exceeds total → clamped.
	got = dataGridSourceFormatRows(490, 20, &rc)
	want = gg.ActiveLocale.StrRows + " 491-500/500"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// Zero count.
	got = dataGridSourceFormatRows(0, 0, &rc)
	want = gg.ActiveLocale.StrRows + " 0/500"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// Nil total.
	got = dataGridSourceFormatRows(5, 10, nil)
	want = gg.ActiveLocale.StrRows + " 6-15/?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataGridSourceJumpEnabled(t *testing.T) {
	sel := func(GridSelection, *gg.Event, *gg.Window) {}
	rc := 50

	// Happy path: all conditions met.
	if !dataGridSourceJumpEnabled(sel, &rc, false, "", GridPaginationOffset, 10) {
		t.Fatal("expected true")
	}
	// Nil onSelectionChange → false.
	if dataGridSourceJumpEnabled(nil, &rc, false, "", GridPaginationOffset, 10) {
		t.Fatal("expected false with nil onSelectionChange")
	}
	// PageLimit zero → false.
	if dataGridSourceJumpEnabled(sel, &rc, false, "", GridPaginationOffset, 0) {
		t.Fatal("expected false with pageLimit 0")
	}
	// Cursor mode → false.
	if dataGridSourceJumpEnabled(sel, &rc, false, "", GridPaginationCursor, 10) {
		t.Fatal("expected false in cursor mode")
	}
	// Loading → false.
	if dataGridSourceJumpEnabled(sel, &rc, true, "", GridPaginationOffset, 10) {
		t.Fatal("expected false when loading")
	}
	// Load error → false.
	if dataGridSourceJumpEnabled(sel, &rc, false, "err", GridPaginationOffset, 10) {
		t.Fatal("expected false with load error")
	}
	// Nil rowCount → false.
	if dataGridSourceJumpEnabled(sel, nil, false, "", GridPaginationOffset, 10) {
		t.Fatal("expected false with nil rowCount")
	}
	// Zero rowCount → false.
	zero := 0
	if dataGridSourceJumpEnabled(sel, &zero, false, "", GridPaginationOffset, 10) {
		t.Fatal("expected false with zero rowCount")
	}
}

func TestDataGridSourceRowPositionText(t *testing.T) {
	rc := 100
	cfg := &DataGridCfg{
		Rows: []GridRow{
			{ID: "r0"}, {ID: "r1"}, {ID: "r2"},
		},
		Selection: GridSelection{ActiveRowID: "r1"},
	}
	state := dataGridSourceState{
		OffsetStart: 20,
		RowCount:    &rc,
	}
	got := dataGridSourceRowPositionText(cfg, state, GridPaginationOffset)
	// localIdx=1 (r1 at index 1), current=20+1+1=22
	if got != "Row 22 of 100" {
		t.Fatalf("got %q, want %q", got, "Row 22 of 100")
	}
	// Unknown total.
	state.RowCount = nil
	got = dataGridSourceRowPositionText(cfg, state, GridPaginationOffset)
	if got != "Row 22 of ?" {
		t.Fatalf("got %q, want %q", got, "Row 22 of ?")
	}
	// Empty rows.
	cfg.Rows = nil
	got = dataGridSourceRowPositionText(cfg, state, GridPaginationOffset)
	if got != "Row 0 of ?" {
		t.Fatalf("got %q, want %q", got, "Row 0 of ?")
	}
}

func TestDataGridSourceCancelActive(t *testing.T) {
	ctrl := gg.NewGridAbortController()
	state := dataGridSourceState{
		Loading:        true,
		ActiveAbort:    ctrl,
		CancelledCount: 0,
	}
	dataGridSourceCancelActive(&state)
	if !ctrl.Signal.IsAborted() {
		t.Fatal("expected signal to be aborted")
	}
	if state.CancelledCount != 1 {
		t.Fatalf("CancelledCount = %d, want 1", state.CancelledCount)
	}
	// Second call while still loading increments again.
	ctrl2 := gg.NewGridAbortController()
	state.ActiveAbort = ctrl2
	dataGridSourceCancelActive(&state)
	if state.CancelledCount != 2 {
		t.Fatalf("CancelledCount = %d, want 2", state.CancelledCount)
	}
}

func TestDataGridSourceCancelActiveNotLoading(t *testing.T) {
	ctrl := gg.NewGridAbortController()
	state := dataGridSourceState{
		Loading:     false,
		ActiveAbort: ctrl,
	}
	dataGridSourceCancelActive(&state)
	if ctrl.Signal.IsAborted() {
		t.Fatal("should not abort when not loading")
	}
	if state.CancelledCount != 0 {
		t.Fatalf("CancelledCount = %d, want 0", state.CancelledCount)
	}
}

func TestDataGridSourceCancelActiveNilAbort(t *testing.T) {
	state := dataGridSourceState{
		Loading:     true,
		ActiveAbort: nil,
	}
	dataGridSourceCancelActive(&state)
	if state.CancelledCount != 0 {
		t.Fatalf("CancelledCount = %d, want 0", state.CancelledCount)
	}
}

func TestDataGridSourceDropIfStaleMatching(t *testing.T) {
	state := dataGridSourceState{
		RequestID:      5,
		StaleDropCount: 0,
	}
	// Matching request ID → not stale.
	dropped := dataGridSourceDropIfStale(5, &state, nil, "grid1")
	if dropped {
		t.Fatal("should not drop matching request ID")
	}
	if state.StaleDropCount != 0 {
		t.Fatalf("StaleDropCount = %d, want 0", state.StaleDropCount)
	}
}

// --- dataGridSourceForceRefetch ---

func TestSourceForceRefetch(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic — force refetch even without existing state.
	dataGridSourceForceRefetch("g1", w)
}

// --- dataGridSourceRetry ---

func TestSourceRetry(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic — retry does nothing when no state exists.
	dataGridSourceRetry("g1", w)
}

// --- dataGridSourcePrevPage / dataGridSourceNextPage ---

func TestSourcePrevPage(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgSource := gg.StateMap[string, dataGridSourceState](w, nsDgSource, 4)
	dgSource.Set("g1", dataGridSourceState{
		PrevCursor:     "prev-cursor",
		PaginationKind: GridPaginationCursor,
	})
	dataGridSourcePrevPage("g1", GridPaginationCursor, 50, w)
	state, _ := dgSource.Get("g1")
	if state.CurrentCursor != "prev-cursor" {
		t.Errorf("CurrentCursor: got %q, want prev-cursor",
			state.CurrentCursor)
	}
}

func TestSourceNextPage(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgSource := gg.StateMap[string, dataGridSourceState](w, nsDgSource, 4)
	dgSource.Set("g1", dataGridSourceState{
		NextCursor:     "next-cursor",
		PaginationKind: GridPaginationCursor,
	})
	dataGridSourceNextPage("g1", GridPaginationCursor, 50, w)
	state, _ := dgSource.Get("g1")
	if state.CurrentCursor != "next-cursor" {
		t.Errorf("CurrentCursor: got %q, want next-cursor",
			state.CurrentCursor)
	}
}

// --- dataGridSourceJumpToRow ---

func TestSourceJumpToRow(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic even without existing state.
	dataGridSourceJumpToRow("g1", 10, 0, w)
}

// --- dataGridSourceApplyLocalMutation ---

func TestSourceApplyLocalMutation(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	// Creates state when none exists.
	dataGridSourceApplyLocalMutation("g1", rows, 2, w)
	state, _ := gg.StateMap[string, dataGridSourceState](w, nsDgSource, 4).Get("g1")
	if len(state.Rows) != 2 {
		t.Errorf("rows: got %d, want 2", len(state.Rows))
	}
}

func TestSourceApplyLocalMutationNilRows(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dataGridSourceApplyLocalMutation("g1", nil, -1, w)
}

// --- GetSourceStats ---

func TestGetSourceStatsReturnsValue(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgSource := gg.StateMap[string, dataGridSourceState](w, nsDgSource, 4)
	dgSource.Set("g1", dataGridSourceState{
		Rows:          []GridRow{{ID: "a"}},
		ReceivedCount: 1,
		RequestCount:  1,
	})
	stats := GetSourceStats(w, "g1")
	_ = stats
}

// --- dataGridSourceApplyPendingJumpSelection ---

// --- ScrollY / ScrollX map integration ---

func TestWindowScrollYMapRoundTrips(t *testing.T) {
	// Regression: the datagrid must read scroll position from the
	// same map the scroll system writes to (w.ScrollY()). A prior
	// bug used gg.StateMap with a separate namespace, so scroll
	// position was never shared and virtualization always saw zero.
	t.Parallel()
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()

	scrollID := gg.FnvSum32("dg:scroll")
	const offset = float32(-350)

	// Write.
	w.ScrollY().Set(scrollID, offset)

	// Read back through the same map.
	v, ok := w.ScrollY().Get(scrollID)
	if !ok {
		t.Fatal("ScrollY key not found after Set")
	}
	if v != offset {
		t.Fatalf("got %f, want %f", v, offset)
	}
}

func TestWindowScrollXMapRoundTrips(t *testing.T) {
	t.Parallel()
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()

	scrollID := gg.FnvSum32("dg:hscroll")
	const offset = float32(-120)

	w.ScrollX().Set(scrollID, offset)

	v, ok := w.ScrollX().Get(scrollID)
	if !ok {
		t.Fatal("ScrollX key not found after Set")
	}
	if v != offset {
		t.Fatalf("got %f, want %f", v, offset)
	}
}

func TestWindowScrollYMapsAreIndependent(t *testing.T) {
	// Verify that ScrollX and ScrollY maps are separate, keyed
	// by the same uint32 IDs but stored independently.
	t.Parallel()
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()

	id := gg.FnvSum32("dg:both")
	w.ScrollY().Set(id, -100)
	w.ScrollX().Set(id, -200)

	vy, ok := w.ScrollY().Get(id)
	if !ok || vy != -100 {
		t.Fatalf("ScrollY: got (%f,%v), want (-100,true)", vy, ok)
	}
	vx, ok := w.ScrollX().Get(id)
	if !ok || vx != -200 {
		t.Fatalf("ScrollX: got (%f,%v), want (-200,true)", vx, ok)
	}
}

// --- Scroll position affects visible rows (regression for
//     datagrid reading scroll from wrong state map) ---

func TestGridScrollShiftsVisibleRows(t *testing.T) {
	// When the user scrolls down, w.ScrollY() holds the scroll
	// offset.  The datagrid must read that offset to decide which
	// rows to render.  Before the fix it read from a separate
	// StateMap that was never written, so every frame looked like
	// scrollY=0 and only the first ~20 rows ever rendered.
	t.Parallel()

	const (
		rowHeight = 30
		maxHeight = 300
		numRows   = 100
		// Scroll down enough that row 0 is well above the
		// viewport and row 30 is visible.
		scrollDown = float32(-750)
	)

	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()

	rows := make([]GridRow, numRows)
	for i := range numRows {
		rows[i] = GridRow{
			ID:    "r" + strconv.Itoa(i),
			Cells: map[string]string{"a": strconv.Itoa(i)},
		}
	}

	gridID := "dg-scroll-test"
	scrollID := gg.FnvSum32(gridID + ":scroll")

	// Simulate the user having scrolled down before this frame.
	w.ScrollY().Set(scrollID, scrollDown)

	v := New(w, DataGridCfg{
		ID:        gridID,
		MaxHeight: maxHeight,
		RowHeight: rowHeight,
		Columns:   []GridColumnCfg{{ID: "a", Title: "A"}},
		Rows:      rows,
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck

	// Row 0 must NOT be in the layout — it is above the visible
	// range and the virtualizer replaces it with a spacer.
	row0ID := gridID + ":row:r0"
	if _, ok := layout.FindByID(row0ID); ok {
		t.Error("row 0 should be scrolled off-screen and absent from layout")
	}

	// Row 30 should be visible (scrollDown / rowHeight ≈ 25,
	// plus buffer = 27, so row 30 is inside the range).
	row30ID := gridID + ":row:r30"
	if _, ok := layout.FindByID(row30ID); !ok {
		t.Error("row 30 should be visible when scrolled to offset -750")
	}
}

func TestGridScrollTopShowsFirstRows(t *testing.T) {
	// At scroll position 0 the first rows must be rendered.
	t.Parallel()

	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()

	rows := make([]GridRow, 50)
	for i := range 50 {
		rows[i] = GridRow{
			ID:    "r" + strconv.Itoa(i),
			Cells: map[string]string{"a": strconv.Itoa(i)},
		}
	}

	gridID := "dg-scroll-top"
	v := New(w, DataGridCfg{
		ID:        gridID,
		MaxHeight: 200,
		RowHeight: 30,
		Columns:   []GridColumnCfg{{ID: "a", Title: "A"}},
		Rows:      rows,
	})
	layout := gg.GenerateViewLayout(v, w) //nolint:staticcheck

	// Row 0 and row 5 must be present.
	for _, idx := range []int{0, 5} {
		rowID := gridID + ":row:r" + strconv.Itoa(idx)
		if _, ok := layout.FindByID(rowID); !ok {
			t.Errorf("row %d should be visible at scroll=0", idx)
		}
	}
}

func TestSourceApplyPendingJumpSelection(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	state := dataGridSourceState{
		Rows:           rows,
		PendingJumpRow: 1,
	}
	cfg := &DataGridCfg{
		ID:                "g1",
		Rows:              rows,
		OnSelectionChange: func(s GridSelection, _ *gg.Event, _ *gg.Window) {},
	}
	// Should not panic.
	dataGridSourceApplyPendingJumpSelection(cfg, state, w)
}
