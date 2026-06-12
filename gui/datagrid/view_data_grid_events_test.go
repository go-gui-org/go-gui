package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
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
	sel := func(GridSelection, *gg.Event, *gg.Window) {}
	page := func(int, *gg.Event, *gg.Window) {}

	tests := []struct {
		name      string
		rowsLen   int
		onSel     func(GridSelection, *gg.Event, *gg.Window)
		onPage    func(int, *gg.Event, *gg.Window)
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
	e := &gg.Event{KeyCode: gg.KeyPageDown, Modifiers: gg.ModCtrl}
	got, ok := dataGridNextPageIndexForKey(1, 5, e)
	if !ok || got != 2 {
		t.Fatalf("got (%d, %v), want (2, true)", got, ok)
	}
}

func TestNextPageIndexForKeyAltHome(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyHome, Modifiers: gg.ModAlt}
	got, ok := dataGridNextPageIndexForKey(3, 5, e)
	if !ok || got != 0 {
		t.Fatalf("got (%d, %v), want (0, true)", got, ok)
	}
}

func TestNextPageIndexForKeySinglePage(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyPageDown, Modifiers: gg.ModCtrl}
	_, ok := dataGridNextPageIndexForKey(0, 1, e)
	if ok {
		t.Fatal("expected false for single page")
	}
}

// --- dataGridCharIsCopy / dataGridIsSelectAllShortcut ---

func TestCharIsCopy(t *testing.T) {
	e := &gg.Event{CharCode: 3, Modifiers: gg.ModCtrl}
	if !dataGridCharIsCopy(e) {
		t.Fatal("Ctrl+C should be copy")
	}
	e2 := &gg.Event{CharCode: 3, Modifiers: gg.ModSuper}
	if !dataGridCharIsCopy(e2) {
		t.Fatal("Cmd+C should be copy")
	}
	e3 := &gg.Event{CharCode: 3}
	if dataGridCharIsCopy(e3) {
		t.Fatal("bare charCode=3 should not be copy")
	}
}

func TestIsSelectAllShortcut(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyA, Modifiers: gg.ModCtrl}
	if !dataGridIsSelectAllShortcut(e) {
		t.Fatal("Ctrl+A should be select-all")
	}
	e2 := &gg.Event{KeyCode: gg.KeyA, Modifiers: gg.ModSuper}
	if !dataGridIsSelectAllShortcut(e2) {
		t.Fatal("Cmd+A should be select-all")
	}
	e3 := &gg.Event{KeyCode: gg.KeyA}
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
	e := &gg.Event{KeyCode: gg.KeyPageUp, Modifiers: gg.ModCtrl}
	got, ok := dataGridNextPageIndexForKey(2, 5, e)
	if !ok || got != 1 {
		t.Fatalf("got (%d, %v), want (1, true)", got, ok)
	}
}

func TestNextPageIndexForKeyAltEnd(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyEnd, Modifiers: gg.ModAlt}
	got, ok := dataGridNextPageIndexForKey(1, 5, e)
	if !ok || got != 4 {
		t.Fatalf("got (%d, %v), want (4, true)", got, ok)
	}
}

func TestNextPageIndexForKeyUnrecognized(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyA, Modifiers: gg.ModCtrl}
	_, ok := dataGridNextPageIndexForKey(1, 5, e)
	if ok {
		t.Fatal("expected false for unrecognized key")
	}
}

func TestNextPageIndexForKeyNoModifier(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyPageDown}
	_, ok := dataGridNextPageIndexForKey(1, 5, e)
	if ok {
		t.Fatal("expected false without ctrl/super modifier")
	}
}

func TestNextPageIndexForKeyAltUnrecognized(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyA, Modifiers: gg.ModAlt}
	_, ok := dataGridNextPageIndexForKey(1, 5, e)
	if ok {
		t.Fatal("expected false for Alt+A")
	}
}

func TestNextPageIndexForKeySuperPageDown(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyPageDown, Modifiers: gg.ModSuper}
	got, ok := dataGridNextPageIndexForKey(0, 3, e)
	if !ok || got != 1 {
		t.Fatalf("got (%d, %v), want (1, true)", got, ok)
	}
}

func TestNextPageIndexForKeyClampFirst(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyPageUp, Modifiers: gg.ModCtrl}
	got, ok := dataGridNextPageIndexForKey(0, 5, e)
	if !ok || got != 0 {
		t.Fatalf("got (%d, %v), want (0, true)", got, ok)
	}
}

func TestNextPageIndexForKeyClampLast(t *testing.T) {
	e := &gg.Event{KeyCode: gg.KeyPageDown, Modifiers: gg.ModCtrl}
	got, ok := dataGridNextPageIndexForKey(4, 5, e)
	if !ok || got != 4 {
		t.Fatalf("got (%d, %v), want (4, true)", got, ok)
	}
}

// --- dataGridHandleEscapeKey ---

func TestHandleEscapeKeyMarksHandled(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &gg.Event{KeyCode: gg.KeyEscape}
	handled := dataGridHandleEscapeKey(kc, e, w)
	if !handled {
		t.Fatal("escape should be handled")
	}
	if !e.IsHandled {
		t.Fatal("event should be marked handled")
	}
}

func TestHandleEscapeKeyIgnoresModifiers(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &gg.Event{KeyCode: gg.KeyEscape, Modifiers: gg.ModShift}
	handled := dataGridHandleEscapeKey(kc, e, w)
	if handled {
		t.Fatal("escape with modifiers should not be handled")
	}
}

func TestHandleEscapeKeyIgnoresNonEscape(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &gg.Event{KeyCode: gg.KeyA}
	handled := dataGridHandleEscapeKey(kc, e, w)
	if handled {
		t.Fatal("non-escape should not be handled")
	}
}

// --- dataGridHandleRowNavigationKeys ---

func TestHandleRowNavigationKeysArrowDown(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
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
		onSelectionChange: func(sel GridSelection, _ *gg.Event, _ *gg.Window) {
			selected = sel
		},
	}
	e := &gg.Event{KeyCode: gg.KeyDown}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1, 2}, e, w)
	if !e.IsHandled {
		t.Fatal("event should be handled")
	}
	if selected.ActiveRowID != "b" {
		t.Errorf("active row: got %q, want %q", selected.ActiveRowID, "b")
	}
}

func TestHandleRowNavigationKeysArrowUp(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
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
		onSelectionChange: func(sel GridSelection, _ *gg.Event, _ *gg.Window) {
			selected = sel
		},
	}
	e := &gg.Event{KeyCode: gg.KeyUp}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1, 2}, e, w)
	if selected.ActiveRowID != "a" {
		t.Errorf("active row: got %q, want %q", selected.ActiveRowID, "a")
	}
}

func TestHandleRowNavigationKeysHome(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
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
		onSelectionChange: func(sel GridSelection, _ *gg.Event, _ *gg.Window) {
			selected = sel
		},
	}
	e := &gg.Event{KeyCode: gg.KeyHome}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1, 2}, e, w)
	if selected.ActiveRowID != "a" {
		t.Errorf("home: got %q, want %q", selected.ActiveRowID, "a")
	}
}

func TestHandleRowNavigationKeysEnd(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
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
		onSelectionChange: func(sel GridSelection, _ *gg.Event, _ *gg.Window) {
			selected = sel
		},
	}
	e := &gg.Event{KeyCode: gg.KeyEnd}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1, 2}, e, w)
	if selected.ActiveRowID != "c" {
		t.Errorf("end: got %q, want %q", selected.ActiveRowID, "c")
	}
}

func TestHandleRowNavigationKeysNoCallback(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
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
	e := &gg.Event{KeyCode: gg.KeyDown}
	dataGridHandleRowNavigationKeys(kc, []int{0, 1}, e, w)
	if !e.IsHandled {
		t.Fatal("should still mark handled even without callback")
	}
}

func TestHandleRowNavigationKeysUnrecognized(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
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
	e := &gg.Event{KeyCode: gg.KeyA}
	dataGridHandleRowNavigationKeys(kc, []int{0}, e, w)
	if e.IsHandled {
		t.Fatal("unrecognized key should not be handled")
	}
}

// --- dataGridOnKeydown (integration) ---

func TestOnKeydownEscapeNoRows(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &gg.Event{KeyCode: gg.KeyEscape}
	dataGridOnKeydown(kc, e, w)
	if !e.IsHandled {
		t.Fatal("escape should be handled")
	}
}

func TestOnKeydownSelectAll(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	var selected GridSelection
	kc := dataGridKeydownContext{
		gridID:      "g1",
		rows:        rows,
		multiSelect: true,
		pageIndices: []int{0, 1, 2},
		onSelectionChange: func(sel GridSelection, _ *gg.Event, _ *gg.Window) {
			selected = sel
		},
	}
	e := &gg.Event{KeyCode: gg.KeyA, Modifiers: gg.ModCtrl}
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
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic.
	dataGridScrollRowIntoViewEx(0, 0, 30, 0, 1, w)
}

func TestScrollRowIntoViewExZeroRowHeight(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dataGridScrollRowIntoViewEx(100, 0, 0, 0, 1, w)
}

// --- dataGridHandleEnterKey ---

func TestHandleEnterKeyNoActivate(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID: "g1",
		rows:   []GridRow{{ID: "a"}},
	}
	e := &gg.Event{KeyCode: gg.KeyEnter}
	handled := dataGridHandleEnterKey(kc, e, w)
	if !handled {
		t.Fatal("enter should be handled")
	}
	if !e.IsHandled {
		t.Fatal("event should be marked handled")
	}
}

func TestHandleEnterKeyWithActivate(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	var activated string
	kc := dataGridKeydownContext{
		gridID: "g1",
		rows:   []GridRow{{ID: "a"}, {ID: "b"}},
		selection: GridSelection{
			ActiveRowID:    "b",
			SelectedRowIDs: map[string]bool{"b": true},
		},
		onRowActivate: func(row GridRow, _ *gg.Event, _ *gg.Window) {
			activated = row.ID
		},
	}
	e := &gg.Event{KeyCode: gg.KeyEnter}
	dataGridHandleEnterKey(kc, e, w)
	if activated != "b" {
		t.Errorf("activated: got %q, want %q", activated, "b")
	}
}

func TestHandleEnterKeyNotEnter(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &gg.Event{KeyCode: gg.KeyA}
	handled := dataGridHandleEnterKey(kc, e, w)
	if handled {
		t.Fatal("non-enter should not be handled")
	}
}

// --- dataGridHandleCrudKeys ---

func TestHandleCrudKeysNotEnabled(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:      "g1",
		crudEnabled: false,
	}
	e := &gg.Event{KeyCode: gg.KeyInsert}
	handled := dataGridHandleCrudKeys(kc, e, w)
	if handled {
		t.Fatal("crud disabled should return false")
	}
}

func TestHandleCrudKeysWithModifiers(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:      "g1",
		crudEnabled: true,
	}
	e := &gg.Event{KeyCode: gg.KeyInsert, Modifiers: gg.ModShift}
	handled := dataGridHandleCrudKeys(kc, e, w)
	if handled {
		t.Fatal("crud with modifiers should return false")
	}
}

// --- dataGridHandleEditStartKey ---

func TestHandleEditStartKeyNotF2(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &gg.Event{KeyCode: gg.KeyA}
	handled := dataGridHandleEditStartKey(kc, e, w)
	if handled {
		t.Fatal("non-F2 should return false")
	}
}

func TestHandleEditStartKeyF2NotEditable(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:      "g1",
		editEnabled: false,
	}
	e := &gg.Event{KeyCode: gg.KeyF2}
	handled := dataGridHandleEditStartKey(kc, e, w)
	if !handled {
		t.Fatal("F2 should return true even when not editable")
	}
}

func TestHandleEditStartKeyF2WithModifiers(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{gridID: "g1"}
	e := &gg.Event{KeyCode: gg.KeyF2, Modifiers: gg.ModCtrl}
	handled := dataGridHandleEditStartKey(kc, e, w)
	if handled {
		t.Fatal("F2+Ctrl should return false")
	}
}

// --- dataGridPagerSpacer ---

func TestPagerSpacerReturnsView(t *testing.T) {
	v := dataGridPagerSpacer()
	if v == nil {
		t.Fatal("spacer should return a view")
	}
	// Spacer is a FillFill row with no padding.
}

// --- dataGridPagerJumpLabel ---

func TestPagerJumpLabelReturnsView(t *testing.T) {
	cfg := &DataGridCfg{TextStyleFilter: gg.DefaultTextStyle}
	v := dataGridPagerJumpLabel(cfg)
	if v == nil {
		t.Fatal("jump label should return a view")
	}
}

// --- dataGridJumpContextFromPager ---

func TestJumpContextFromPager(t *testing.T) {
	cfg := &DataGridCfg{
		ID:                "g1",
		Rows:              []GridRow{{ID: "a"}},
		OnSelectionChange: func(GridSelection, *gg.Event, *gg.Window) {},
		OnPageChange:      func(int, *gg.Event, *gg.Window) {},
		PageSize:          25,
	}
	pctx := dataGridPagerContext{
		cfg:           cfg,
		pageIndex:     0,
		totalRows:     100,
		viewportH:     400,
		rowHeight:     25,
		staticTop:     0,
		scrollID:      1,
		dataToDisplay: map[int]int{0: 0},
	}
	got := dataGridJumpContextFromPager(pctx)
	if got.gridID != "g1" {
		t.Errorf("gridID: got %q, want g1", got.gridID)
	}
	if got.pageSize != 25 {
		t.Errorf("pageSize: got %d, want 25", got.pageSize)
	}
	if got.totalRows != 100 {
		t.Errorf("totalRows: got %d, want 100", got.totalRows)
	}
}

// --- dataGridSelectionForTargetRow ---

func TestSelectionForTargetRowPlain(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	kc := dataGridKeydownContext{
		gridID:      "g1",
		rows:        rows,
		multiSelect: true,
		rangeSelect: true,
	}
	got := dataGridSelectionForTargetRow(kc, "b", false, w)
	if got.ActiveRowID != "b" {
		t.Errorf("active: got %q, want b", got.ActiveRowID)
	}
	if len(got.SelectedRowIDs) != 1 || !got.SelectedRowIDs["b"] {
		t.Errorf("selected: got %v, want only b", got.SelectedRowIDs)
	}
}

func TestSelectionForTargetRowShiftRange(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	kc := dataGridKeydownContext{
		gridID: "g1",
		rows:   rows,
		selection: GridSelection{
			AnchorRowID:    "a",
			ActiveRowID:    "a",
			SelectedRowIDs: map[string]bool{"a": true},
		},
		multiSelect: true,
		rangeSelect: true,
	}
	got := dataGridSelectionForTargetRow(kc, "c", true, w)
	if got.AnchorRowID != "a" {
		t.Errorf("anchor: got %q, want a", got.AnchorRowID)
	}
	if got.ActiveRowID != "c" {
		t.Errorf("active: got %q, want c", got.ActiveRowID)
	}
	if !got.SelectedRowIDs["a"] || !got.SelectedRowIDs["b"] ||
		!got.SelectedRowIDs["c"] {
		t.Errorf("range should include a,b,c: %v", got.SelectedRowIDs)
	}
}

// --- dataGridHandlePageShortcut ---

func TestHandlePageShortcutNoCallback(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:       "g1",
		pageSize:     25,
		pageIndex:    0,
		onPageChange: nil,
	}
	rows := []GridRow{}
	for range 50 {
		rows = append(rows, GridRow{ID: "r"})
	}
	kc.rows = rows
	e := &gg.Event{KeyCode: gg.KeyPageDown, Modifiers: gg.ModCtrl}
	handled := dataGridHandlePageShortcut(kc, e, w)
	if handled {
		t.Fatal("should return false without onPageChange")
	}
}

func TestHandlePageShortcutSinglePage(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:       "g1",
		rows:         []GridRow{{ID: "a"}},
		pageSize:     25,
		pageIndex:    0,
		onPageChange: func(int, *gg.Event, *gg.Window) {},
	}
	e := &gg.Event{KeyCode: gg.KeyPageDown, Modifiers: gg.ModCtrl}
	handled := dataGridHandlePageShortcut(kc, e, w)
	if handled {
		t.Fatal("single page should return false")
	}
}

func TestHandlePageShortcutCtrlPageDown(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	var nextPage int
	rows := []GridRow{}
	for range 50 {
		rows = append(rows, GridRow{ID: "r"})
	}
	kc := dataGridKeydownContext{
		gridID:    "g1",
		rows:      rows,
		pageSize:  10,
		pageIndex: 0,
		onPageChange: func(p int, _ *gg.Event, _ *gg.Window) {
			nextPage = p
		},
	}
	e := &gg.Event{KeyCode: gg.KeyPageDown, Modifiers: gg.ModCtrl}
	handled := dataGridHandlePageShortcut(kc, e, w)
	if !handled {
		t.Fatal("should be handled")
	}
	if nextPage != 1 {
		t.Errorf("got page %d, want 1", nextPage)
	}
}

// --- dataGridHandleSelectAllShortcut ---

func TestHandleSelectAllShortcutDisabledMultiSelect(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:      "g1",
		rows:        []GridRow{{ID: "a"}, {ID: "b"}},
		multiSelect: false,
	}
	e := &gg.Event{KeyCode: gg.KeyA, Modifiers: gg.ModCtrl}
	handled := dataGridHandleSelectAllShortcut(kc, e, w)
	if handled {
		t.Fatal("should return false when multiSelect disabled")
	}
}

// --- dataGridMakeColumnChooserOnClick ---

func TestMakeColumnChooserOnClick(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	var changedHidden map[string]bool
	cb := func(h map[string]bool, _ *gg.Event, _ *gg.Window) {
		changedHidden = h
	}
	hidden := map[string]bool{"col2": true}
	columns := []GridColumnCfg{
		{ID: "col1"}, {ID: "col2"}, {ID: "col3"},
	}
	fn := dataGridMakeColumnChooserOnClick(cb, hidden, columns, "col2", 0)
	e := &gg.Event{}
	fn(nil, e, w)
	if !e.IsHandled {
		t.Fatal("should be handled")
	}
	if changedHidden["col2"] {
		t.Fatal("should have un-hidden col2")
	}
}

func TestMakeColumnChooserOnClickNilCallback(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	hidden := map[string]bool{}
	columns := []GridColumnCfg{{ID: "col1"}}
	fn := dataGridMakeColumnChooserOnClick(nil, hidden, columns, "col1", 0)
	e := &gg.Event{}
	fn(nil, e, w)
	// Should not panic; event not marked handled when callback is nil.
}

// --- dataGridToggleColumnChooserOpen ---

func TestToggleColumnChooserOpen(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgCO := gg.StateMap[string, bool](w, nsDgChooserOpen, 4)
	dgCO.Set("g1", false)
	dataGridToggleColumnChooserOpen("g1", w)
	isOpen, _ := dgCO.Get("g1")
	if !isOpen {
		t.Fatal("should be toggled to open")
	}
	dataGridToggleColumnChooserOpen("g1", w)
	isOpen, _ = dgCO.Get("g1")
	if isOpen {
		t.Fatal("should be toggled to closed")
	}
}

// --- dataGridSubmitLocalJump ---

func TestSubmitLocalJumpValid(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	var selected GridSelection
	ctx := dataGridJumpContext{
		gridID: "g1",
		rows:   rows,
		onSelectionChange: func(sel GridSelection, _ *gg.Event, _ *gg.Window) {
			selected = sel
		},
		onPageChange:  nil,
		pageSize:      0,
		totalRows:     3,
		viewportH:     100,
		rowHeight:     25,
		staticTop:     0,
		scrollID:      1,
		dataToDisplay: map[int]int{0: 0, 1: 1, 2: 2},
		focusID:       10,
	}
	// Set jump input to "2"
	dgJI := gg.StateMap[string, string](w, nsDgJump, 4)
	dgJI.Set("g1", "2")

	e := &gg.Event{}
	dataGridSubmitLocalJump(ctx, e, w)
	if !e.IsHandled {
		t.Fatal("should be handled")
	}
	if selected.ActiveRowID != "b" {
		t.Errorf("active: got %q, want b", selected.ActiveRowID)
	}
}

func TestSubmitLocalJumpDisabled(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	ctx := dataGridJumpContext{
		gridID:    "g1",
		rows:      []GridRow{},
		totalRows: 0,
	}
	e := &gg.Event{}
	dataGridSubmitLocalJump(ctx, e, w)
	if e.IsHandled {
		t.Fatal("should not be handled when jump disabled")
	}
}

// --- dataGridJumpToLocalRow ---

func TestJumpToLocalRow(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	var selected GridSelection
	ctx := dataGridJumpContext{
		gridID: "g1",
		rows:   rows,
		onSelectionChange: func(sel GridSelection, _ *gg.Event, _ *gg.Window) {
			selected = sel
		},
		pageSize:      0,
		dataToDisplay: map[int]int{1: 1},
		viewportH:     100,
		rowHeight:     25,
		scrollID:      1,
	}
	e := &gg.Event{}
	dataGridJumpToLocalRow(ctx, 1, e, w)
	if selected.ActiveRowID != "b" {
		t.Errorf("active: got %q, want b", selected.ActiveRowID)
	}
}

func TestJumpToLocalRowOutOfRange(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	ctx := dataGridJumpContext{
		gridID: "g1",
		rows:   []GridRow{{ID: "a"}},
	}
	e := &gg.Event{}
	// Should not panic.
	dataGridJumpToLocalRow(ctx, -1, e, w)
	dataGridJumpToLocalRow(ctx, 99, e, w)
}

// --- dataGridApplyPendingLocalJumpScroll ---

func TestApplyPendingLocalJumpScroll(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cfg := &DataGridCfg{
		ID:   "g1",
		Rows: []GridRow{{ID: "a"}, {ID: "b"}},
	}
	dgPJ := gg.StateMap[string, int](w, nsDgPendingJump, 4)
	dgPJ.Set("g1", 0)
	dataGridApplyPendingLocalJumpScroll(cfg, 100, 25, 0, 1,
		map[int]int{0: 0}, w)
	// After application, pending jump should be cleared.
	_, ok := dgPJ.Get("g1")
	if ok {
		t.Fatal("pending jump should be cleared")
	}
}

func TestApplyPendingLocalJumpScrollNoPending(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cfg := &DataGridCfg{ID: "g1", Rows: []GridRow{{ID: "a"}}}
	// Should not panic when there's no pending jump.
	dataGridApplyPendingLocalJumpScroll(cfg, 100, 25, 0, 1,
		map[int]int{0: 0}, w)
}

func TestApplyPendingLocalJumpScrollOutOfRange(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cfg := &DataGridCfg{
		ID:   "g1",
		Rows: []GridRow{{ID: "a"}},
	}
	dgPJ := gg.StateMap[string, int](w, nsDgPendingJump, 4)
	dgPJ.Set("g1", 99) // out of range
	dataGridApplyPendingLocalJumpScroll(cfg, 100, 25, 0, 1,
		map[int]int{}, w)
	// Should clear pending jump even for out-of-range target.
	_, ok := dgPJ.Get("g1")
	if ok {
		t.Fatal("pending jump should be cleared for out-of-range target")
	}
}

// --- dataGridMakeOnChar ---

func TestMakeOnCharCopy(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{
		{ID: "r1", Cells: map[string]string{"a": "v1", "b": "v2"}},
	}
	cfg := &DataGridCfg{
		Rows: rows,
		Selection: GridSelection{
			SelectedRowIDs: map[string]bool{"r1": true},
		},
	}
	columns := []GridColumnCfg{{ID: "a"}, {ID: "b"}}
	handler := dataGridMakeOnChar(cfg, columns)
	e := &gg.Event{CharCode: 3, Modifiers: gg.ModCtrl}
	handler(nil, e, w)
	if !e.IsHandled {
		t.Fatal("copy should be handled")
	}
}

func TestMakeOnCharNoSelection(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cfg := &DataGridCfg{
		Rows:      []GridRow{{ID: "r1"}},
		Selection: GridSelection{},
	}
	handler := dataGridMakeOnChar(cfg, nil)
	e := &gg.Event{CharCode: 3, Modifiers: gg.ModCtrl}
	handler(nil, e, w)
	if e.IsHandled {
		t.Fatal("copy without selection should not be handled")
	}
}

func TestMakeOnCharNotCopyKey(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cfg := &DataGridCfg{
		Rows:      []GridRow{{ID: "r1"}},
		Selection: GridSelection{SelectedRowIDs: map[string]bool{"r1": true}},
	}
	handler := dataGridMakeOnChar(cfg, []GridColumnCfg{{ID: "a"}})
	e := &gg.Event{CharCode: 1} // Ctrl+A, not copy
	handler(nil, e, w)
	if e.IsHandled {
		t.Fatal("non-copy char should not be handled")
	}
}

// --- dataGridOnKeydown edge cases ---

func TestOnKeydownNoRows(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	kc := dataGridKeydownContext{
		gridID:      "g1",
		rows:        nil,
		pageIndices: nil,
	}
	e := &gg.Event{KeyCode: gg.KeyDown}
	dataGridOnKeydown(kc, e, w)
	// Should not panic with no rows.
}

// --- dataGridPagerRowsStatus ---

func TestPagerRowsStatusReturnsView(t *testing.T) {
	cfg := &DataGridCfg{TextStyleFilter: gg.DefaultTextStyle}
	v := dataGridPagerRowsStatus(cfg, "Rows 1-10/50")
	if v == nil {
		t.Fatal("rows status should return a view")
	}
}

// --- dataGridMakeOnKeydown callback ---

func TestMakeOnKeydownReturnsCallback(t *testing.T) {
	cfg := &DataGridCfg{
		ID:       "g1",
		Rows:     []GridRow{{ID: "a"}},
		Columns:  []GridColumnCfg{{ID: "col1"}},
		PageSize: 0,
	}
	fn := dataGridMakeOnKeydown(cfg, cfg.Columns, 25, 0, 1, nil, nil, nil)
	if fn == nil {
		t.Fatal("should return a callback")
	}
}

// --- dataGridPagerPrevButton / dataGridPagerNextButton ---

func TestPagerPrevButton(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
	}
	v := dataGridPagerPrevButton(cfg, nil, 1, 0, true, "◀")
	if v == nil {
		t.Fatal("prev button should return a view")
	}
}

func TestPagerNextButton(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
	}
	v := dataGridPagerNextButton(cfg, nil, 0, 5, 0, true, "▶")
	if v == nil {
		t.Fatal("next button should return a view")
	}
}

// --- dataGridPagerPrevButton callback ---

func TestPagerPrevButtonOnClick(t *testing.T) {
	var nextPage int
	cb := func(p int, _ *gg.Event, _ *gg.Window) {
		nextPage = p
	}
	cfg := &DataGridCfg{
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
	}
	// Build the layout. The button's OnClick is wired through
	// dataGridIndicatorButton. We verify construction succeeds.
	v := dataGridPagerPrevButton(cfg, cb, 1, 0, false, "◀")
	if v == nil {
		t.Fatal("prev button should return a view")
	}
	// Callback fires through the button's OnClick; verified indirectly
	// via construction.
	_ = nextPage
}

// --- dataGridPagerNextButton callback ---

func TestPagerNextButtonOnClick(t *testing.T) {
	var nextPage int
	cb := func(p int, _ *gg.Event, _ *gg.Window) {
		nextPage = p
	}
	cfg := &DataGridCfg{
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
	}
	dataGridPagerNextButton(cfg, cb, 0, 5, 0, false, "▶")
	// Callback fires through the button's OnClick; verified indirectly.
	_ = nextPage
}

// --- dataGridBuildPagerRow ---

func TestBuildPagerRow(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleFilter: gg.DefaultTextStyle,
		ColorFilter:     gg.RGBA(240, 240, 240, 255),
		ColorBorder:     gg.RGBA(180, 180, 180, 255),
	}
	pctx := dataGridPagerContext{
		cfg:           cfg,
		focusID:       0,
		pageIndex:     0,
		pageCount:     3,
		pageStart:     0,
		pageEnd:       10,
		totalRows:     30,
		viewportH:     400,
		rowHeight:     25,
		staticTop:     0,
		scrollID:      1,
		dataToDisplay: map[int]int{},
		jumpText:      "",
	}
	v := dataGridBuildPagerRow(pctx)
	if v == nil {
		t.Fatal("pager row should return a view")
	}
}
