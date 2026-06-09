package datagrid

import (
	"testing"

	. "github.com/go-gui-org/go-gui/gui"
)

// --- dataGridJumpDigits ---

func TestJumpDigitsExtractsOnlyDigits(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123", "123"},
		{"abc", ""},
		{"a1b2c3", "123"},
		{"", ""},
		{"  42  ", "42"},
		{"row#7!", "7"},
	}
	for _, tt := range tests {
		got := dataGridJumpDigits(tt.input)
		if got != tt.want {
			t.Errorf("dataGridJumpDigits(%q) = %q, want %q",
				tt.input, got, tt.want)
		}
	}
}

// --- dataGridParseJumpTarget ---

func TestParseJumpTargetValid(t *testing.T) {
	// "5" with 10 rows -> index 4 (1-based to 0-based)
	got, ok := dataGridParseJumpTarget("5", 10)
	if !ok || got != 4 {
		t.Fatalf("got (%d, %v), want (4, true)", got, ok)
	}
}

func TestParseJumpTargetOutOfRange(t *testing.T) {
	// "20" with 10 rows -> clamped to index 9
	got, ok := dataGridParseJumpTarget("20", 10)
	if !ok || got != 9 {
		t.Fatalf("got (%d, %v), want (9, true)", got, ok)
	}
}

func TestParseJumpTargetEmpty(t *testing.T) {
	_, ok := dataGridParseJumpTarget("", 10)
	if ok {
		t.Fatal("expected false for empty input")
	}
}

func TestParseJumpTargetNonNumeric(t *testing.T) {
	_, ok := dataGridParseJumpTarget("abc", 10)
	if ok {
		t.Fatal("expected false for non-numeric input")
	}
}

func TestParseJumpTargetZeroTotalRows(t *testing.T) {
	_, ok := dataGridParseJumpTarget("1", 0)
	if ok {
		t.Fatal("expected false when totalRows <= 0")
	}
}

func TestParseJumpTargetFirstRow(t *testing.T) {
	got, ok := dataGridParseJumpTarget("1", 5)
	if !ok || got != 0 {
		t.Fatalf("got (%d, %v), want (0, true)", got, ok)
	}
}

// --- dataGridPageBounds (local page count calculation) ---

func TestPageBoundsPageCount(t *testing.T) {
	tests := []struct {
		totalRows int
		pageSize  int
		wantCount int
	}{
		{100, 25, 4},
		{101, 25, 5}, // partial last page
		{0, 25, 1},   // no rows -> 1 page
		{10, 0, 1},   // no paging -> 1 page
		{1, 1, 1},    // single row, single page
		{50, 50, 1},  // exact fit
		{50, 100, 1}, // pageSize > totalRows
	}
	for _, tt := range tests {
		_, _, _, pageCount := dataGridPageBounds(tt.totalRows, tt.pageSize, 0)
		if pageCount != tt.wantCount {
			t.Errorf("dataGridPageBounds(%d, %d, 0) pageCount = %d, want %d",
				tt.totalRows, tt.pageSize, pageCount, tt.wantCount)
		}
	}
}

func TestPageBoundsStartEnd(t *testing.T) {
	// Page 1 of 4 (0-indexed page 1) with 100 rows, pageSize 25
	start, end, pageIdx, _ := dataGridPageBounds(100, 25, 1)
	if start != 25 || end != 50 || pageIdx != 1 {
		t.Fatalf("got start=%d end=%d pageIdx=%d, want 25/50/1",
			start, end, pageIdx)
	}
}

func TestPageBoundsClampsPageIndex(t *testing.T) {
	// Requested page 99 with only 4 pages -> clamped to 3
	_, _, pageIdx, _ := dataGridPageBounds(100, 25, 99)
	if pageIdx != 3 {
		t.Fatalf("got pageIdx=%d, want 3", pageIdx)
	}
}

// --- dataGridPagerRowsText (page bounds text) ---

func TestPagerRowsTextNormal(t *testing.T) {
	got := dataGridPagerRowsText(0, 25, 100)
	want := "Rows 1-25/100"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPagerRowsTextZeroTotal(t *testing.T) {
	got := dataGridPagerRowsText(0, 0, 0)
	want := "Rows 0/0"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- dataGridQuickFilterMatchesText ---

func TestQuickFilterMatchesTextLocalOnly(t *testing.T) {
	cfg := &DataGridCfg{
		Rows: []GridRow{
			{ID: "r1", Cells: map[string]string{"a": "1"}},
			{ID: "r2", Cells: map[string]string{"a": "2"}},
			{ID: "r3", Cells: map[string]string{"a": "3"}},
		},
	}
	got := dataGridQuickFilterMatchesText(cfg)
	want := "Matches 3"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestQuickFilterMatchesTextWithRowCount(t *testing.T) {
	total := 200
	cfg := &DataGridCfg{
		Rows:     []GridRow{{ID: "r1"}, {ID: "r2"}},
		RowCount: &total,
	}
	got := dataGridQuickFilterMatchesText(cfg)
	want := "Matches 2/200"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- dataGridSourceRowMatchesQuery (quick filter matching) ---

func TestSourceRowMatchesQueryCaseInsensitive(t *testing.T) {
	row := GridRow{
		ID:    "r1",
		Cells: map[string]string{"name": "Alice", "city": "Boston"},
	}
	if !dataGridSourceRowMatchesQuery(row, "alice", nil) {
		t.Fatal("expected match for case-insensitive 'alice'")
	}
	if !dataGridSourceRowMatchesQuery(row, "bos", nil) {
		t.Fatal("expected match for partial 'bos'")
	}
	if dataGridSourceRowMatchesQuery(row, "xyz", nil) {
		t.Fatal("expected no match for 'xyz'")
	}
}

func TestSourceRowMatchesQueryEmptyNeedle(t *testing.T) {
	row := GridRow{
		ID:    "r1",
		Cells: map[string]string{"a": "value"},
	}
	if !dataGridSourceRowMatchesQuery(row, "", nil) {
		t.Fatal("empty needle should match all rows")
	}
}

func TestSourceRowMatchesQueryMultipleCells(t *testing.T) {
	row := GridRow{
		ID: "r1",
		Cells: map[string]string{
			"first": "John",
			"last":  "Doe",
			"email": "john@example.com",
		},
	}
	// Match in any cell
	if !dataGridSourceRowMatchesQuery(row, "example", nil) {
		t.Fatal("expected match in email cell")
	}
	if !dataGridSourceRowMatchesQuery(row, "doe", nil) {
		t.Fatal("expected match in last cell")
	}
}

// --- dataGridJumpEnabledLocal ---

func TestJumpEnabledLocal(t *testing.T) {
	sel := func(GridSelection, *Event, *Window) {}
	page := func(int, *Event, *Window) {}

	tests := []struct {
		name      string
		rowsLen   int
		onSel     func(GridSelection, *Event, *Window)
		onPage    func(int, *Event, *Window)
		pageSize  int
		totalRows int
		want      bool
	}{
		{"enabled", 10, sel, page, 5, 10, true},
		{"no rows", 0, sel, page, 5, 10, false},
		{"zero total", 10, sel, page, 5, 0, false},
		{"no selection cb", 10, nil, page, 5, 10, false},
		{"paged no page cb", 10, sel, nil, 5, 10, false},
		{"no paging ok", 10, sel, nil, 0, 10, true},
	}
	for _, tt := range tests {
		got := dataGridJumpEnabledLocal(tt.rowsLen, tt.onSel, tt.onPage,
			tt.pageSize, tt.totalRows)
		if got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

// --- dataGridNextPageIndexForKey ---

func TestNextPageIndexForKeyCtrlPageDown(t *testing.T) {
	e := &Event{KeyCode: KeyPageDown, Modifiers: ModCtrl}
	got, ok := dataGridNextPageIndexForKey(1, 5, e)
	if !ok || got != 2 {
		t.Fatalf("got (%d, %v), want (2, true)", got, ok)
	}
}

func TestNextPageIndexForKeyAltHome(t *testing.T) {
	e := &Event{KeyCode: KeyHome, Modifiers: ModAlt}
	got, ok := dataGridNextPageIndexForKey(3, 5, e)
	if !ok || got != 0 {
		t.Fatalf("got (%d, %v), want (0, true)", got, ok)
	}
}

func TestNextPageIndexForKeySinglePage(t *testing.T) {
	e := &Event{KeyCode: KeyPageDown, Modifiers: ModCtrl}
	_, ok := dataGridNextPageIndexForKey(0, 1, e)
	if ok {
		t.Fatal("expected false for single page")
	}
}

// --- dataGridCharIsCopy / dataGridIsSelectAllShortcut ---

func TestCharIsCopy(t *testing.T) {
	e := &Event{CharCode: 3, Modifiers: ModCtrl}
	if !dataGridCharIsCopy(e) {
		t.Fatal("Ctrl+C should be copy")
	}
	e2 := &Event{CharCode: 3, Modifiers: ModSuper}
	if !dataGridCharIsCopy(e2) {
		t.Fatal("Cmd+C should be copy")
	}
	e3 := &Event{CharCode: 3}
	if dataGridCharIsCopy(e3) {
		t.Fatal("bare charCode=3 should not be copy")
	}
}

func TestIsSelectAllShortcut(t *testing.T) {
	e := &Event{KeyCode: KeyA, Modifiers: ModCtrl}
	if !dataGridIsSelectAllShortcut(e) {
		t.Fatal("Ctrl+A should be select-all")
	}
	e2 := &Event{KeyCode: KeyA, Modifiers: ModSuper}
	if !dataGridIsSelectAllShortcut(e2) {
		t.Fatal("Cmd+A should be select-all")
	}
	e3 := &Event{KeyCode: KeyA}
	if dataGridIsSelectAllShortcut(e3) {
		t.Fatal("bare 'A' should not be select-all")
	}
}

// --- dataGridRangeSelectedRows ---

func TestRangeSelectedRows(t *testing.T) {
	rows := []GridRow{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
	}
	got := dataGridRangeSelectedRows(rows, 1, 2, "b")
	if len(got) != 2 {
		t.Fatalf("got %d selected, want 2", len(got))
	}
	if !got["b"] || !got["c"] {
		t.Fatalf("expected b and c selected, got %v", got)
	}
}

func TestRangeSelectedRowsFallback(t *testing.T) {
	rows := []GridRow{{ID: "a"}}
	got := dataGridRangeSelectedRows(rows, -1, -1, "x")
	if len(got) != 1 || !got["x"] {
		t.Fatalf("expected fallback to target, got %v", got)
	}
}

// --- dataGridNextPageIndexForKey (additional branches) ---

func TestNextPageIndexForKeyCtrlPageUp(t *testing.T) {
	e := &Event{KeyCode: KeyPageUp, Modifiers: ModCtrl}
	got, ok := dataGridNextPageIndexForKey(2, 5, e)
	if !ok || got != 1 {
		t.Fatalf("got (%d, %v), want (1, true)", got, ok)
	}
}

func TestNextPageIndexForKeyAltEnd(t *testing.T) {
	e := &Event{KeyCode: KeyEnd, Modifiers: ModAlt}
	got, ok := dataGridNextPageIndexForKey(1, 5, e)
	if !ok || got != 4 {
		t.Fatalf("got (%d, %v), want (4, true)", got, ok)
	}
}

func TestNextPageIndexForKeyUnrecognized(t *testing.T) {
	e := &Event{KeyCode: KeyA, Modifiers: ModCtrl}
	_, ok := dataGridNextPageIndexForKey(1, 5, e)
	if ok {
		t.Fatal("expected false for unrecognized key")
	}
}

func TestNextPageIndexForKeyNoModifier(t *testing.T) {
	e := &Event{KeyCode: KeyPageDown}
	_, ok := dataGridNextPageIndexForKey(1, 5, e)
	if ok {
		t.Fatal("expected false without ctrl/super modifier")
	}
}

func TestNextPageIndexForKeyAltUnrecognized(t *testing.T) {
	e := &Event{KeyCode: KeyA, Modifiers: ModAlt}
	_, ok := dataGridNextPageIndexForKey(1, 5, e)
	if ok {
		t.Fatal("expected false for Alt+A")
	}
}

func TestNextPageIndexForKeySuperPageDown(t *testing.T) {
	e := &Event{KeyCode: KeyPageDown, Modifiers: ModSuper}
	got, ok := dataGridNextPageIndexForKey(0, 3, e)
	if !ok || got != 1 {
		t.Fatalf("got (%d, %v), want (1, true)", got, ok)
	}
}

func TestNextPageIndexForKeyClampFirst(t *testing.T) {
	e := &Event{KeyCode: KeyPageUp, Modifiers: ModCtrl}
	got, ok := dataGridNextPageIndexForKey(0, 5, e)
	if !ok || got != 0 {
		t.Fatalf("got (%d, %v), want (0, true)", got, ok)
	}
}

func TestNextPageIndexForKeyClampLast(t *testing.T) {
	e := &Event{KeyCode: KeyPageDown, Modifiers: ModCtrl}
	got, ok := dataGridNextPageIndexForKey(4, 5, e)
	if !ok || got != 4 {
		t.Fatalf("got (%d, %v), want (4, true)", got, ok)
	}
}

// --- dataGridHandleEscapeKey ---

func TestHandleEscapeKeyMarksHandled(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &Event{KeyCode: KeyEscape}
	handled := dataGridHandleEscapeKey(kc, e, w)
	if !handled {
		t.Fatal("escape should be handled")
	}
	if !e.IsHandled {
		t.Fatal("event should be marked handled")
	}
}

func TestHandleEscapeKeyIgnoresModifiers(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &Event{KeyCode: KeyEscape, Modifiers: ModShift}
	handled := dataGridHandleEscapeKey(kc, e, w)
	if handled {
		t.Fatal("escape with modifiers should not be handled")
	}
}

func TestHandleEscapeKeyIgnoresNonEscape(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &Event{KeyCode: KeyA}
	handled := dataGridHandleEscapeKey(kc, e, w)
	if handled {
		t.Fatal("non-escape should not be handled")
	}
}

// --- dataGridHandleRowNavigationKeys ---

func TestHandleRowNavigationKeysArrowDown(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	var selected GridSelection
	kc := dataGridKeydownContext{
		gridID:  "g1",
		rows:    rows,
		columns: []GridColumnCfg{{ID: "col1"}},
		selection: GridSelection{
			ActiveRowID:    "a",
			SelectedRowIDs: map[string]bool{"a": true},
		},
		multiSelect:   true,
		rangeSelect:   true,
		pageRows:      10,
		pageIndices:   []int{0, 1, 2},
		colCount:      1,
		frozenTopIDs:  map[string]bool{},
		dataToDisplay: map[int]int{0: 0, 1: 1, 2: 2},
		onSelectionChange: func(sel GridSelection, _ *Event, _ *Window) {
			selected = sel
		},
	}
	e := &Event{KeyCode: KeyDown}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1, 2}, e, w)
	if !e.IsHandled {
		t.Fatal("event should be handled")
	}
	if selected.ActiveRowID != "b" {
		t.Errorf("active row: got %q, want %q", selected.ActiveRowID, "b")
	}
}

func TestHandleRowNavigationKeysArrowUp(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	var selected GridSelection
	kc := dataGridKeydownContext{
		gridID:  "g1",
		rows:    rows,
		columns: []GridColumnCfg{{ID: "col1"}},
		selection: GridSelection{
			ActiveRowID:    "b",
			SelectedRowIDs: map[string]bool{"b": true},
		},
		multiSelect:   true,
		rangeSelect:   true,
		pageRows:      10,
		pageIndices:   []int{0, 1, 2},
		colCount:      1,
		frozenTopIDs:  map[string]bool{},
		dataToDisplay: map[int]int{0: 0, 1: 1, 2: 2},
		onSelectionChange: func(sel GridSelection, _ *Event, _ *Window) {
			selected = sel
		},
	}
	e := &Event{KeyCode: KeyUp}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1, 2}, e, w)
	if selected.ActiveRowID != "a" {
		t.Errorf("active row: got %q, want %q", selected.ActiveRowID, "a")
	}
}

func TestHandleRowNavigationKeysHome(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	var selected GridSelection
	kc := dataGridKeydownContext{
		gridID: "g1",
		rows:   rows,
		selection: GridSelection{
			ActiveRowID:    "c",
			SelectedRowIDs: map[string]bool{"c": true},
		},
		multiSelect:   true,
		rangeSelect:   true,
		pageRows:      10,
		pageIndices:   []int{0, 1, 2},
		frozenTopIDs:  map[string]bool{},
		dataToDisplay: map[int]int{0: 0, 1: 1, 2: 2},
		onSelectionChange: func(sel GridSelection, _ *Event, _ *Window) {
			selected = sel
		},
	}
	e := &Event{KeyCode: KeyHome}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1, 2}, e, w)
	if selected.ActiveRowID != "a" {
		t.Errorf("home: got %q, want %q", selected.ActiveRowID, "a")
	}
}

func TestHandleRowNavigationKeysEnd(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	var selected GridSelection
	kc := dataGridKeydownContext{
		gridID: "g1",
		rows:   rows,
		selection: GridSelection{
			ActiveRowID:    "a",
			SelectedRowIDs: map[string]bool{"a": true},
		},
		multiSelect:   true,
		rangeSelect:   true,
		pageRows:      10,
		pageIndices:   []int{0, 1, 2},
		frozenTopIDs:  map[string]bool{},
		dataToDisplay: map[int]int{0: 0, 1: 1, 2: 2},
		onSelectionChange: func(sel GridSelection, _ *Event, _ *Window) {
			selected = sel
		},
	}
	e := &Event{KeyCode: KeyEnd}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1, 2}, e, w)
	if selected.ActiveRowID != "c" {
		t.Errorf("end: got %q, want %q", selected.ActiveRowID, "c")
	}
}

func TestHandleRowNavigationKeysNoCallback(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	kc := dataGridKeydownContext{
		gridID:            "g1",
		rows:              rows,
		selection:         GridSelection{ActiveRowID: "a"},
		pageRows:          10,
		pageIndices:       []int{0, 1},
		frozenTopIDs:      map[string]bool{},
		dataToDisplay:     map[int]int{0: 0, 1: 1},
		onSelectionChange: nil,
	}
	e := &Event{KeyCode: KeyDown}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1}, e, w)
	if !e.IsHandled {
		t.Fatal("should still mark handled even without callback")
	}
}

func TestHandleRowNavigationKeysUnrecognized(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}}
	kc := dataGridKeydownContext{
		gridID:        "g1",
		rows:          rows,
		pageRows:      10,
		pageIndices:   []int{0},
		frozenTopIDs:  map[string]bool{},
		dataToDisplay: map[int]int{0: 0},
	}
	e := &Event{KeyCode: KeyA}
	dataGridHandleRowNavigationKeys(kc, []int{0}, e, w)
	if e.IsHandled {
		t.Fatal("unrecognized key should not be handled")
	}
}

// --- dataGridOnKeydown (integration) ---

func TestOnKeydownEscapeNoRows(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &Event{KeyCode: KeyEscape}
	dataGridOnKeydown(kc, e, w)
	if !e.IsHandled {
		t.Fatal("escape should be handled")
	}
}

func TestOnKeydownSelectAll(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	var selected GridSelection
	kc := dataGridKeydownContext{
		gridID:      "g1",
		rows:        rows,
		multiSelect: true,
		pageIndices: []int{0, 1, 2},
		onSelectionChange: func(sel GridSelection, _ *Event, _ *Window) {
			selected = sel
		},
	}
	e := &Event{KeyCode: KeyA, Modifiers: ModCtrl}
	dataGridOnKeydown(kc, e, w)
	if !e.IsHandled {
		t.Fatal("Ctrl+A should be handled")
	}
	if len(selected.SelectedRowIDs) != 3 {
		t.Errorf("expected 3 selected, got %d", len(selected.SelectedRowIDs))
	}
}

// --- dataGridScrollRowIntoViewEx ---

func TestScrollRowIntoViewExZeroViewport(_ *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	// Should not panic.
	dataGridScrollRowIntoViewEx(0, 0, 30, 0, 1, w)
}

func TestScrollRowIntoViewExZeroRowHeight(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	dataGridScrollRowIntoViewEx(100, 0, 0, 0, 1, w)
}

// --- dataGridHandleEnterKey ---

func TestHandleEnterKeyNoActivate(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID: "g1",
		rows:   []GridRow{{ID: "a"}},
	}
	e := &Event{KeyCode: KeyEnter}
	handled := dataGridHandleEnterKey(kc, e, w)
	if !handled {
		t.Fatal("enter should be handled")
	}
	if !e.IsHandled {
		t.Fatal("event should be marked handled")
	}
}

func TestHandleEnterKeyWithActivate(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	var activated string
	kc := dataGridKeydownContext{
		gridID: "g1",
		rows:   []GridRow{{ID: "a"}, {ID: "b"}},
		selection: GridSelection{
			ActiveRowID:    "b",
			SelectedRowIDs: map[string]bool{"b": true},
		},
		onRowActivate: func(row GridRow, _ *Event, _ *Window) {
			activated = row.ID
		},
	}
	e := &Event{KeyCode: KeyEnter}
	dataGridHandleEnterKey(kc, e, w)
	if activated != "b" {
		t.Errorf("activated: got %q, want %q", activated, "b")
	}
}

func TestHandleEnterKeyNotEnter(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &Event{KeyCode: KeyA}
	handled := dataGridHandleEnterKey(kc, e, w)
	if handled {
		t.Fatal("non-enter should not be handled")
	}
}

// --- dataGridHandleCrudKeys ---

func TestHandleCrudKeysNotEnabled(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:      "g1",
		crudEnabled: false,
	}
	e := &Event{KeyCode: KeyInsert}
	handled := dataGridHandleCrudKeys(kc, e, w)
	if handled {
		t.Fatal("crud disabled should return false")
	}
}

func TestHandleCrudKeysWithModifiers(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:      "g1",
		crudEnabled: true,
	}
	e := &Event{KeyCode: KeyInsert, Modifiers: ModShift}
	handled := dataGridHandleCrudKeys(kc, e, w)
	if handled {
		t.Fatal("crud with modifiers should return false")
	}
}

// --- dataGridHandleEditStartKey ---

func TestHandleEditStartKeyNotF2(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &Event{KeyCode: KeyA}
	handled := dataGridHandleEditStartKey(kc, e, w)
	if handled {
		t.Fatal("non-F2 should return false")
	}
}

func TestHandleEditStartKeyF2NotEditable(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:      "g1",
		editEnabled: false,
	}
	e := &Event{KeyCode: KeyF2}
	handled := dataGridHandleEditStartKey(kc, e, w)
	if !handled {
		t.Fatal("F2 should return true even when not editable")
	}
}

func TestHandleEditStartKeyF2WithModifiers(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &Event{KeyCode: KeyF2, Modifiers: ModCtrl}
	handled := dataGridHandleEditStartKey(kc, e, w)
	if handled {
		t.Fatal("F2+Ctrl should return false")
	}
}
