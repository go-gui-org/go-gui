package datagrid

import (
	"fmt"
	"testing"

	. "github.com/go-gui-org/go-gui/gui"
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
	want := fmt.Sprintf("%s 21-70/200", ActiveLocale.StrRows)
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
	want := fmt.Sprintf("%s 11-30/100", ActiveLocale.StrRows)
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
	want := fmt.Sprintf("%s 15/?", ActiveLocale.StrRows)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataGridSourceFormatRows(t *testing.T) {
	rc := 500
	// Normal range.
	got := dataGridSourceFormatRows(10, 25, &rc)
	want := fmt.Sprintf("%s 11-35/500", ActiveLocale.StrRows)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// End exceeds total → clamped.
	got = dataGridSourceFormatRows(490, 20, &rc)
	want = fmt.Sprintf("%s 491-500/500", ActiveLocale.StrRows)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// Zero count.
	got = dataGridSourceFormatRows(0, 0, &rc)
	want = fmt.Sprintf("%s 0/500", ActiveLocale.StrRows)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// Nil total.
	got = dataGridSourceFormatRows(5, 10, nil)
	want = fmt.Sprintf("%s 6-15/?", ActiveLocale.StrRows)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataGridSourceJumpEnabled(t *testing.T) {
	sel := func(GridSelection, *Event, *Window) {}
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
	ctrl := NewGridAbortController()
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
	ctrl2 := NewGridAbortController()
	state.ActiveAbort = ctrl2
	dataGridSourceCancelActive(&state)
	if state.CancelledCount != 2 {
		t.Fatalf("CancelledCount = %d, want 2", state.CancelledCount)
	}
}

func TestDataGridSourceCancelActiveNotLoading(t *testing.T) {
	ctrl := NewGridAbortController()
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
