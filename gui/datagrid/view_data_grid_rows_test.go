package datagrid

import (
	"testing"
	"time"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- dataGridResolveCellFormat ---

func TestResolveCellFormatNoOverride(t *testing.T) {
	base := gg.TextStyle{Color: gg.Color{R: 100}}
	format := GridCellFormat{}
	ts, bg := dataGridResolveCellFormat(base, format)
	if ts.Color.R != 100 {
		t.Error("text color should match base")
	}
	if bg != gg.ColorTransparent {
		t.Error("bg should be transparent")
	}
}

func TestResolveCellFormatTextColor(t *testing.T) {
	base := gg.TextStyle{Color: gg.Color{R: 100}}
	format := GridCellFormat{HasTextColor: true, TextColor: gg.Color{R: 200}}
	ts, _ := dataGridResolveCellFormat(base, format)
	if ts.Color.R != 200 {
		t.Errorf("got R=%d, want 200", ts.Color.R)
	}
}

func TestResolveCellFormatBGColor(t *testing.T) {
	base := gg.TextStyle{}
	format := GridCellFormat{HasBGColor: true, BGColor: gg.Color{G: 150}}
	_, bg := dataGridResolveCellFormat(base, format)
	if bg.G != 150 {
		t.Errorf("got G=%d, want 150", bg.G)
	}
}

// --- dataGridToggleSelectedRowIDs ---

func TestToggleSelectedRowIDsAdd(t *testing.T) {
	sel := map[string]bool{"a": true}
	got := dataGridToggleSelectedRowIDs(sel, "b")
	if !got["a"] || !got["b"] {
		t.Errorf("expected a,b selected: %v", got)
	}
}

func TestToggleSelectedRowIDsRemove(t *testing.T) {
	sel := map[string]bool{"a": true, "b": true}
	got := dataGridToggleSelectedRowIDs(sel, "b")
	if !got["a"] || got["b"] {
		t.Errorf("expected only a: %v", got)
	}
}

func TestToggleSelectedRowIDsEmpty(t *testing.T) {
	got := dataGridToggleSelectedRowIDs(nil, "x")
	if !got["x"] || len(got) != 1 {
		t.Errorf("expected only x: %v", got)
	}
}

// --- dataGridSelectionIsSingleRow ---

func TestSelectionIsSingleRowTrue(t *testing.T) {
	if !dataGridSelectionIsSingleRow(map[string]bool{"r1": true}, "r1") {
		t.Error("expected true")
	}
}

func TestSelectionIsSingleRowFalseMultiple(t *testing.T) {
	if dataGridSelectionIsSingleRow(map[string]bool{"r1": true, "r2": true}, "r1") {
		t.Error("expected false for multiple")
	}
}

func TestSelectionIsSingleRowFalseEmpty(t *testing.T) {
	if dataGridSelectionIsSingleRow(nil, "r1") {
		t.Error("expected false for nil")
	}
}

func TestSelectionIsSingleRowFalseEmptyID(t *testing.T) {
	if dataGridSelectionIsSingleRow(map[string]bool{"": true}, "") {
		t.Error("expected false for empty rowID")
	}
}

// --- dataGridRangeIndices ---

func TestRangeIndicesBothFound(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	s, e := dataGridRangeIndices(rows, "a", "c")
	if s != 0 || e != 2 {
		t.Errorf("got (%d,%d), want (0,2)", s, e)
	}
}

func TestRangeIndicesReversed(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	s, e := dataGridRangeIndices(rows, "c", "a")
	if s != 0 || e != 2 {
		t.Errorf("reversed: got (%d,%d), want (0,2)", s, e)
	}
}

func TestRangeIndicesNotFound(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	s, e := dataGridRangeIndices(rows, "a", "z")
	if s != -1 || e != -1 {
		t.Errorf("got (%d,%d), want (-1,-1)", s, e)
	}
}

func TestRangeIndicesSame(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	s, e := dataGridRangeIndices(rows, "a", "a")
	if s != 0 || e != 0 {
		t.Errorf("same: got (%d,%d), want (0,0)", s, e)
	}
}

// --- dataGridEditorBoolValue ---

func TestEditorBoolValueTrue(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "y", "on", "  True  ", "YES"} {
		if !dataGridEditorBoolValue(v) {
			t.Errorf("expected true for %q", v)
		}
	}
}

func TestEditorBoolValueFalse(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "", "n", "off", "maybe"} {
		if dataGridEditorBoolValue(v) {
			t.Errorf("expected false for %q", v)
		}
	}
}

// --- dataGridParseEditorDate ---

func TestParseEditorDateFormats(t *testing.T) {
	tests := []struct {
		input string
		year  int
		month time.Month
		day   int
	}{
		{"1/2/2006", 2006, time.January, 2},
		{"2024-03-15", 2024, time.March, 15},
		{"2024-03-15 10:30:00", 2024, time.March, 15},
	}
	for _, tt := range tests {
		got := dataGridParseEditorDate(tt.input)
		if got.Year() != tt.year || got.Month() != tt.month || got.Day() != tt.day {
			t.Errorf("parse(%q): got %v", tt.input, got)
		}
	}
}

func TestParseEditorDateEmpty(t *testing.T) {
	got := dataGridParseEditorDate("")
	// Empty returns time.Now(); verify it's recent.
	if time.Since(got) > time.Second {
		t.Error("empty should return ~now")
	}
}

func TestParseEditorDateInvalid(t *testing.T) {
	got := dataGridParseEditorDate("not-a-date")
	if time.Since(got) > time.Second {
		t.Error("invalid should return ~now")
	}
}

// --- dataGridNextDetailExpandedMap ---

func TestNextDetailExpandedMapExpand(t *testing.T) {
	got := dataGridNextDetailExpandedMap(nil, "r1")
	if !got["r1"] {
		t.Error("should expand r1")
	}
}

func TestNextDetailExpandedMapCollapse(t *testing.T) {
	got := dataGridNextDetailExpandedMap(map[string]bool{"r1": true}, "r1")
	if got["r1"] {
		t.Error("should collapse r1")
	}
}

func TestNextDetailExpandedMapEmptyRowID(t *testing.T) {
	got := dataGridNextDetailExpandedMap(map[string]bool{"r1": true}, "")
	if !got["r1"] || len(got) != 1 {
		t.Error("empty rowID should not change map")
	}
}

func TestNextDetailExpandedMapDoesNotMutateOriginal(t *testing.T) {
	orig := map[string]bool{"r1": true}
	dataGridNextDetailExpandedMap(orig, "r2")
	if orig["r2"] {
		t.Error("original map should not be mutated")
	}
}

// --- dataGridFrozenTopIDSet ---

func TestFrozenTopIDSetNormal(t *testing.T) {
	cfg := &DataGridCfg{FrozenTopRowIDs: []string{"a", "b"}}
	got := dataGridFrozenTopIDSet(cfg)
	if !got["a"] || !got["b"] || len(got) != 2 {
		t.Errorf("got %v", got)
	}
}

func TestFrozenTopIDSetTrimsWhitespace(t *testing.T) {
	cfg := &DataGridCfg{FrozenTopRowIDs: []string{"  a  ", ""}}
	got := dataGridFrozenTopIDSet(cfg)
	if !got["a"] || len(got) != 1 {
		t.Errorf("got %v", got)
	}
}

func TestFrozenTopIDSetEmpty(t *testing.T) {
	cfg := &DataGridCfg{}
	got := dataGridFrozenTopIDSet(cfg)
	if len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

// --- dataGridDetailIndent ---

func TestDetailIndent(t *testing.T) {
	got := dataGridDetailIndent()
	want := dataGridHeaderControlWidth + dataGridDetailIndentGap
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// --- dataGridFirstEditableColumnIndexEx ---

func TestFirstEditableColumnIndexExFound(t *testing.T) {
	cols := []GridColumnCfg{
		{ID: "a", Editable: false},
		{ID: "b", Editable: true},
		{ID: "c", Editable: true},
	}
	if got := dataGridFirstEditableColumnIndexEx(cols); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestFirstEditableColumnIndexExNone(t *testing.T) {
	cols := []GridColumnCfg{
		{ID: "a", Editable: false},
	}
	if got := dataGridFirstEditableColumnIndexEx(cols); got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

// --- dataGridHasKeyboardModifiers ---

func TestHasKeyboardModifiers(t *testing.T) {
	if dataGridHasKeyboardModifiers(&gg.Event{}) {
		t.Error("no modifiers should return false")
	}
	if !dataGridHasKeyboardModifiers(&gg.Event{Modifiers: gg.ModShift}) {
		t.Error("Shift should return true")
	}
	if !dataGridHasKeyboardModifiers(&gg.Event{Modifiers: gg.ModCtrl}) {
		t.Error("Ctrl should return true")
	}
}

// --- dataGridEditorFocusIDFromBase ---

func TestEditorFocusIDFromBase(t *testing.T) {
	if got := dataGridEditorFocusIDFromBase("g:efocus:", 3, 0); got != "g:efocus:0" {
		t.Errorf("got %q, want g:efocus:0", got)
	}
	if got := dataGridEditorFocusIDFromBase("g:efocus:", 3, 2); got != "g:efocus:2" {
		t.Errorf("got %q, want g:efocus:2", got)
	}
	if got := dataGridEditorFocusIDFromBase("", 3, 0); got != "" {
		t.Errorf("empty base: got %q, want empty", got)
	}
	if got := dataGridEditorFocusIDFromBase("g:efocus:", 3, 3); got != "" {
		t.Errorf("out of range: got %q, want empty", got)
	}
	if got := dataGridEditorFocusIDFromBase("g:efocus:", 3, -1); got != "" {
		t.Errorf("negative: got %q, want empty", got)
	}
}

// --- dataGridSplitFrozenTopIndices ---

func TestSplitFrozenTopIndicesNoFrozen(t *testing.T) {
	cfg := &DataGridCfg{
		Rows: []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}},
	}
	frozen, body := dataGridSplitFrozenTopIndices(cfg, nil)
	if len(frozen) != 0 {
		t.Errorf("frozen: %v", frozen)
	}
	if len(body) != 3 {
		t.Errorf("body len: %d, want 3", len(body))
	}
}

func TestSplitFrozenTopIndicesWithFrozen(t *testing.T) {
	cfg := &DataGridCfg{
		Rows:            []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		FrozenTopRowIDs: []string{"b"},
	}
	frozen, body := dataGridSplitFrozenTopIndices(cfg, nil)
	if len(frozen) != 1 {
		t.Fatalf("frozen len: %d, want 1", len(frozen))
	}
	if frozen[0] != 1 {
		t.Errorf("frozen[0] = %d, want 1", frozen[0])
	}
	if len(body) != 2 {
		t.Errorf("body len: %d, want 2", len(body))
	}
}

// --- dataGridScrollPadding ---

func TestScrollPaddingHidden(t *testing.T) {
	cfg := &DataGridCfg{Scrollbar: gg.ScrollbarHidden}
	got := dataGridScrollPadding(cfg)
	if got != gg.PaddingNone {
		t.Errorf("hidden scrollbar should return PaddingNone: %v", got)
	}
}

func TestScrollPaddingVisible(t *testing.T) {
	cfg := &DataGridCfg{}
	got := dataGridScrollPadding(cfg)
	// Default scrollbar should have right padding > 0.
	if got.Right <= 0 {
		t.Errorf("visible scrollbar should have right padding > 0: %v", got)
	}
}

// --- dataGridTrackRowEditClick (double-click to edit) ---

func TestTrackRowEditClickDoubleClick(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cols := []GridColumnCfg{{ID: "name", Editable: true}}

	// First click at frame 5.
	e1 := &gg.Event{FrameCount: 5}
	dataGridTrackRowEditClick("g1", true, "g1:efocus:", 1, cols, 0, "row1", "g1", e1, w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("after first click: editing=%q, want empty", got)
	}

	// Second click within threshold → double-click.
	e2 := &gg.Event{FrameCount: 10}
	dataGridTrackRowEditClick("g1", true, "g1:efocus:", 1, cols, 0, "row1", "g1", e2, w)
	if got := dataGridEditingRowID("g1", w); got != "row1" {
		t.Fatalf("after double click: editing=%q, want row1", got)
	}
}

func TestTrackRowEditClickTooSlow(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cols := []GridColumnCfg{{ID: "name", Editable: true}}

	e1 := &gg.Event{FrameCount: 5}
	dataGridTrackRowEditClick("g1", true, "g1:efocus:", 1, cols, 0, "row1", "g1", e1, w)

	// Second click beyond threshold.
	e2 := &gg.Event{FrameCount: 5 + dataGridEditDoubleClickFrames + 1}
	dataGridTrackRowEditClick("g1", true, "g1:efocus:", 1, cols, 0, "row1", "g1", e2, w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("slow double click: editing=%q, want empty", got)
	}
}

func TestTrackRowEditClickDifferentRow(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cols := []GridColumnCfg{{ID: "name", Editable: true}}

	e1 := &gg.Event{FrameCount: 5}
	dataGridTrackRowEditClick("g1", true, "g1:efocus:", 1, cols, 0, "row1", "g1", e1, w)

	// Second click on different row — not a double click.
	e2 := &gg.Event{FrameCount: 10}
	dataGridTrackRowEditClick("g1", true, "g1:efocus:", 1, cols, 0, "row2", "g1", e2, w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("different row: editing=%q, want empty", got)
	}
}

// --- dataGridCellEditorFocusBaseID ---

func TestCellEditorFocusBaseID(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	got := dataGridCellEditorFocusBaseID(cfg, 3)
	if got != "g:efocus:" {
		t.Errorf("got %q, want g:efocus:", got)
	}
}

func TestCellEditorFocusBaseIDZeroCols(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	got := dataGridCellEditorFocusBaseID(cfg, 0)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- dataGridCellEditorFocusID ---

func TestCellEditorFocusID(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	got := dataGridCellEditorFocusID(cfg, 3, 0, 0)
	if got != "g:efocus:0" {
		t.Errorf("got %q, want g:efocus:0", got)
	}
}

func TestCellEditorFocusIDOutOfRange(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	if got := dataGridCellEditorFocusID(cfg, 3, -1, 0); got != "" {
		t.Errorf("negative row: got %q, want empty", got)
	}
	if got := dataGridCellEditorFocusID(cfg, 3, 0, 3); got != "" {
		t.Errorf("col out of range: got %q, want empty", got)
	}
}

// --- dataGridFirstEditableColumnIndex ---

func TestFirstEditableColumnIndex(t *testing.T) {
	cfg := &DataGridCfg{
		OnCellEdit: func(GridCellEdit, *gg.Event, *gg.Window) {},
		Columns: []GridColumnCfg{
			{ID: "a", Editable: false},
			{ID: "b", Editable: true},
		},
	}
	got := dataGridFirstEditableColumnIndex(cfg, cfg.Columns)
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestFirstEditableColumnIndexDisabled(t *testing.T) {
	cfg := &DataGridCfg{
		Columns: []GridColumnCfg{{ID: "a", Editable: true}},
	}
	got := dataGridFirstEditableColumnIndex(cfg, cfg.Columns)
	if got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

// --- dataGridMakeEditorOnKeydown ---

func TestMakeEditorOnKeydownEscape(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dataGridSetEditingRow("g1", "r1", w)
	fn := dataGridMakeEditorOnKeydown("g1", "g1")
	e := &gg.Event{KeyCode: gg.KeyEscape}
	fn(nil, e, w)
	if !e.IsHandled {
		t.Fatal("escape should be handled")
	}
	if dataGridEditingRowID("g1", w) != "" {
		t.Fatal("editing row should be cleared")
	}
}

func TestMakeEditorOnKeydownNonEscape(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dataGridSetEditingRow("g1", "r1", w)
	fn := dataGridMakeEditorOnKeydown("g1", "g1")
	e := &gg.Event{KeyCode: gg.KeyEnter}
	fn(nil, e, w)
	if e.IsHandled {
		t.Fatal("non-escape should not be handled")
	}
	if dataGridEditingRowID("g1", w) != "r1" {
		t.Fatal("editing row should not be cleared on non-escape")
	}
}

func TestMakeEditorOnKeydownEscapeWithModifiers(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dataGridSetEditingRow("g1", "r1", w)
	fn := dataGridMakeEditorOnKeydown("g1", "g1")
	e := &gg.Event{KeyCode: gg.KeyEscape, Modifiers: gg.ModShift}
	fn(nil, e, w)
	if e.IsHandled {
		t.Fatal("escape+shift should not be handled by editor")
	}
}

// --- dataGridSetEditingRow / dataGridClearEditingRow / dataGridEditingRowID ---

func TestSetAndClearEditingRow(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("initially empty: got %q", got)
	}
	dataGridSetEditingRow("g1", "row1", w)
	if got := dataGridEditingRowID("g1", w); got != "row1" {
		t.Fatalf("after set: got %q, want row1", got)
	}
	dataGridClearEditingRow("g1", w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("after clear: got %q, want empty", got)
	}
}

// --- dataGridAnchorRowIDEx ---

func TestAnchorRowIDExFromStateMap(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	dgRange := gg.StateMap[string, dataGridRangeState](w, nsDgRange, 4)
	dgRange.Set("g1", dataGridRangeState{AnchorRowID: "b"})
	sel := GridSelection{AnchorRowID: "a"}
	got := dataGridAnchorRowIDEx(sel, "g1", rows, w, "fallback")
	if got != "b" {
		t.Errorf("got %q, want b", got)
	}
}

func TestAnchorRowIDExFallbackToSelection(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{AnchorRowID: "b"}
	got := dataGridAnchorRowIDEx(sel, "g1", rows, w, "fallback")
	if got != "b" {
		t.Errorf("got %q, want b", got)
	}
}

func TestAnchorRowIDExFallbackToArg(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}}
	sel := GridSelection{}
	got := dataGridAnchorRowIDEx(sel, "g1", rows, w, "fallback")
	if got != "fallback" {
		t.Errorf("got %q, want fallback", got)
	}
}

// --- dataGridSetAnchor ---

func TestSetAnchor(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dataGridSetAnchor("g1", "r1", w)
	dgRange := gg.StateMap[string, dataGridRangeState](w, nsDgRange, 4)
	st, ok := dgRange.Get("g1")
	if !ok || st.AnchorRowID != "r1" {
		t.Errorf("got anchor=%q, want r1", st.AnchorRowID)
	}
}

// --- dataGridComputeRowSelection ---

func TestComputeRowSelectionPlain(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	sel := GridSelection{}
	got := dataGridComputeRowSelection(rows, sel, "g1", true, true, "b",
		&gg.Event{}, w)
	if got.ActiveRowID != "b" {
		t.Errorf("active: got %q, want b", got.ActiveRowID)
	}
	if len(got.SelectedRowIDs) != 1 || !got.SelectedRowIDs["b"] {
		t.Errorf("selected: %v", got.SelectedRowIDs)
	}
}

func TestComputeRowSelectionToggle(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{
		SelectedRowIDs: map[string]bool{"a": true},
	}
	got := dataGridComputeRowSelection(rows, sel, "g1", true, true, "b",
		&gg.Event{Modifiers: gg.ModCtrl}, w)
	if !got.SelectedRowIDs["a"] || !got.SelectedRowIDs["b"] {
		t.Errorf("toggle should keep a and add b: %v", got.SelectedRowIDs)
	}
}

func TestComputeRowSelectionShiftRange(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	sel := GridSelection{
		AnchorRowID:    "a",
		SelectedRowIDs: map[string]bool{"a": true},
	}
	got := dataGridComputeRowSelection(rows, sel, "g1", true, true, "c",
		&gg.Event{Modifiers: gg.ModShift}, w)
	if len(got.SelectedRowIDs) != 3 {
		t.Errorf("shift should select 3 rows: %v", got.SelectedRowIDs)
	}
}

func TestComputeRowSelectionSingleRowReselection(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	sel := GridSelection{
		SelectedRowIDs: map[string]bool{"a": true},
	}
	got := dataGridComputeRowSelection(rows, sel, "g1", true, true, "a",
		&gg.Event{}, w)
	// Reselection of the only selected row preserves the selection map.
	if len(got.SelectedRowIDs) != 1 || !got.SelectedRowIDs["a"] {
		t.Errorf("expected a still selected: %v", got.SelectedRowIDs)
	}
}

// --- dataGridRowClick ---

func TestRowClickWithCallback(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	var selected GridSelection
	sel := GridSelection{}
	e := &gg.Event{FrameCount: 5}
	dataGridRowClick(rows, sel, "g1", true, true,
		func(s GridSelection, _ *gg.Event, _ *gg.Window) { selected = s },
		false, "", 0, 0, "a", "g1",
		[]GridColumnCfg{}, e, w)
	if selected.ActiveRowID != "a" {
		t.Errorf("active: got %q, want a", selected.ActiveRowID)
	}
	if !e.IsHandled {
		t.Fatal("should be handled")
	}
}

func TestRowClickOutOfRange(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	rows := []GridRow{{ID: "a"}}
	e := &gg.Event{}
	// Should not panic for out-of-range index.
	dataGridRowClick(rows, GridSelection{}, "g1", true, true, nil,
		false, "", 0, -1, "x", "", nil, e, w)
}

// --- dataGridDetailToggleControl ---

func TestDetailToggleControlCollapsed(t *testing.T) {
	cfg := &DataGridCfg{
		ID:            "g1",
		TextStyle:     gg.DefaultTextStyle,
		ColorRowHover: gg.RGBA(220, 220, 220, 255),
	}
	v := dataGridDetailToggleControl(cfg, "r1", false, true, "g1")
	if v == nil {
		t.Fatal("detail toggle should return a view")
	}
}

func TestDetailToggleControlExpanded(t *testing.T) {
	cfg := &DataGridCfg{
		ID:            "g1",
		TextStyle:     gg.DefaultTextStyle,
		ColorRowHover: gg.RGBA(220, 220, 220, 255),
	}
	v := dataGridDetailToggleControl(cfg, "r1", true, true, "g1")
	if v == nil {
		t.Fatal("expanded toggle should return a view")
	}
}

func TestDetailToggleControlDisabled(t *testing.T) {
	cfg := &DataGridCfg{
		ID:        "g1",
		TextStyle: gg.DefaultTextStyle,
	}
	v := dataGridDetailToggleControl(cfg, "r1", false, false, "")
	if v == nil {
		t.Fatal("disabled toggle should return a view")
	}
}

// --- dataGridGroupHeaderRowView ---

func TestGroupHeaderRowView(t *testing.T) {
	trueVal := true
	cfg := &DataGridCfg{
		ColorBorder:     gg.RGBA(180, 180, 180, 255),
		SizeBorder:      gg.SomeF(1),
		PaddingCell:     gg.SomeP(2, 4, 2, 4),
		TextStyleHeader: gg.DefaultTextStyle,
		ColorFilter:     gg.RGBA(240, 240, 240, 255),
		ShowGroupCounts: &trueVal,
	}
	entry := dataGridDisplayRow{
		Kind:          dataGridDisplayRowGroupHeader,
		GroupColTitle: "Status",
		GroupValue:    "Active",
		GroupDepth:    0,
		GroupCount:    5,
	}
	v := dataGridGroupHeaderRowView(cfg, entry, 25)
	if v == nil {
		t.Fatal("group header should return a view")
	}
}

func TestGroupHeaderRowViewWithAggregate(t *testing.T) {
	cfg := &DataGridCfg{
		ColorBorder:     gg.RGBA(180, 180, 180, 255),
		SizeBorder:      gg.SomeF(1),
		PaddingCell:     gg.SomeP(2, 4, 2, 4),
		TextStyleHeader: gg.DefaultTextStyle,
		ColorFilter:     gg.RGBA(240, 240, 240, 255),
	}
	entry := dataGridDisplayRow{
		Kind:          dataGridDisplayRowGroupHeader,
		GroupColTitle: "Category",
		GroupValue:    "Electronics",
		GroupDepth:    1,
		GroupCount:    3,
		AggregateText: "sum: 500",
	}
	v := dataGridGroupHeaderRowView(cfg, entry, 25)
	if v == nil {
		t.Fatal("group header with aggregate should return a view")
	}
}

// --- dataGridFrozenTopZone ---

func TestFrozenTopZone(t *testing.T) {
	cfg := &DataGridCfg{
		ColorBackground: gg.RGBA(255, 255, 255, 255),
		ColorBorder:     gg.RGBA(180, 180, 180, 255),
	}
	v := dataGridFrozenTopZone(cfg, nil, 50, 200, 0)
	if v == nil {
		t.Fatal("frozen top zone should return a view")
	}
}

// --- dataGridScrollGutter ---

func TestScrollGutter(t *testing.T) {
	got := dataGridScrollGutter()
	if got <= 0 {
		t.Errorf("got %v, want > 0", got)
	}
}

func TestTrackRowEditClickDisabled(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	cols := []GridColumnCfg{{ID: "name", Editable: true}}

	e1 := &gg.Event{FrameCount: 5}
	dataGridTrackRowEditClick("g1", false, "g1:efocus:", 1, cols, 0, "row1", "g1", e1, w)
	e2 := &gg.Event{FrameCount: 10}
	dataGridTrackRowEditClick("g1", false, "g1:efocus:", 1, cols, 0, "row1", "g1", e2, w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("edit disabled: editing=%q, want empty", got)
	}
}
