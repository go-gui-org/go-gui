package datagrid

import (
	"testing"
	"time"

	. "github.com/go-gui-org/go-gui/gui"
)

// --- dataGridResolveCellFormat ---

func TestResolveCellFormatNoOverride(t *testing.T) {
	base := TextStyle{Color: Color{R: 100}}
	format := GridCellFormat{}
	ts, bg := dataGridResolveCellFormat(base, format)
	if ts.Color.R != 100 {
		t.Error("text color should match base")
	}
	if bg != ColorTransparent {
		t.Error("bg should be transparent")
	}
}

func TestResolveCellFormatTextColor(t *testing.T) {
	base := TextStyle{Color: Color{R: 100}}
	format := GridCellFormat{HasTextColor: true, TextColor: Color{R: 200}}
	ts, _ := dataGridResolveCellFormat(base, format)
	if ts.Color.R != 200 {
		t.Errorf("got R=%d, want 200", ts.Color.R)
	}
}

func TestResolveCellFormatBGColor(t *testing.T) {
	base := TextStyle{}
	format := GridCellFormat{HasBGColor: true, BGColor: Color{G: 150}}
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
	if dataGridHasKeyboardModifiers(&Event{}) {
		t.Error("no modifiers should return false")
	}
	if !dataGridHasKeyboardModifiers(&Event{Modifiers: ModShift}) {
		t.Error("Shift should return true")
	}
	if !dataGridHasKeyboardModifiers(&Event{Modifiers: ModCtrl}) {
		t.Error("Ctrl should return true")
	}
}

// --- dataGridEditorFocusIDFromBase ---

func TestEditorFocusIDFromBase(t *testing.T) {
	if got := dataGridEditorFocusIDFromBase(100, 3, 0); got != 100 {
		t.Errorf("got %d, want 100", got)
	}
	if got := dataGridEditorFocusIDFromBase(100, 3, 2); got != 102 {
		t.Errorf("got %d, want 102", got)
	}
	if got := dataGridEditorFocusIDFromBase(0, 3, 0); got != 0 {
		t.Errorf("zero base: got %d, want 0", got)
	}
	if got := dataGridEditorFocusIDFromBase(100, 3, 3); got != 0 {
		t.Errorf("out of range: got %d, want 0", got)
	}
	if got := dataGridEditorFocusIDFromBase(100, 3, -1); got != 0 {
		t.Errorf("negative: got %d, want 0", got)
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
	cfg := &DataGridCfg{Scrollbar: ScrollbarHidden}
	got := dataGridScrollPadding(cfg)
	if got != PaddingNone {
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
	w := NewWindow(WindowCfg{})
	defer w.Close()
	cols := []GridColumnCfg{{ID: "name", Editable: true}}

	// First click at frame 5.
	e1 := &Event{FrameCount: 5}
	dataGridTrackRowEditClick("g1", true, 100, 1, cols, 0, "row1", 1, e1, w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("after first click: editing=%q, want empty", got)
	}

	// Second click within threshold → double-click.
	e2 := &Event{FrameCount: 10}
	dataGridTrackRowEditClick("g1", true, 100, 1, cols, 0, "row1", 1, e2, w)
	if got := dataGridEditingRowID("g1", w); got != "row1" {
		t.Fatalf("after double click: editing=%q, want row1", got)
	}
}

func TestTrackRowEditClickTooSlow(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	cols := []GridColumnCfg{{ID: "name", Editable: true}}

	e1 := &Event{FrameCount: 5}
	dataGridTrackRowEditClick("g1", true, 100, 1, cols, 0, "row1", 1, e1, w)

	// Second click beyond threshold.
	e2 := &Event{FrameCount: 5 + dataGridEditDoubleClickFrames + 1}
	dataGridTrackRowEditClick("g1", true, 100, 1, cols, 0, "row1", 1, e2, w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("slow double click: editing=%q, want empty", got)
	}
}

func TestTrackRowEditClickDifferentRow(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	cols := []GridColumnCfg{{ID: "name", Editable: true}}

	e1 := &Event{FrameCount: 5}
	dataGridTrackRowEditClick("g1", true, 100, 1, cols, 0, "row1", 1, e1, w)

	// Second click on different row — not a double click.
	e2 := &Event{FrameCount: 10}
	dataGridTrackRowEditClick("g1", true, 100, 1, cols, 0, "row2", 1, e2, w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("different row: editing=%q, want empty", got)
	}
}

func TestTrackRowEditClickDisabled(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	cols := []GridColumnCfg{{ID: "name", Editable: true}}

	e1 := &Event{FrameCount: 5}
	dataGridTrackRowEditClick("g1", false, 100, 1, cols, 0, "row1", 1, e1, w)
	e2 := &Event{FrameCount: 10}
	dataGridTrackRowEditClick("g1", false, 100, 1, cols, 0, "row1", 1, e2, w)
	if got := dataGridEditingRowID("g1", w); got != "" {
		t.Fatalf("edit disabled: editing=%q, want empty", got)
	}
}
