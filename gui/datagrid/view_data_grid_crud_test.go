package datagrid

import (
	"testing"

	. "github.com/go-gui-org/go-gui/gui"
)

// --- dataGridCrudHasUnsaved ---

func TestCrudHasUnsavedEmpty(t *testing.T) {
	state := dataGridCrudState{}
	if dataGridCrudHasUnsaved(state) {
		t.Fatal("empty state should not have unsaved")
	}
}

func TestCrudHasUnsavedDirty(t *testing.T) {
	state := dataGridCrudState{DirtyRowIDs: map[string]bool{"r1": true}}
	if !dataGridCrudHasUnsaved(state) {
		t.Fatal("dirty rows should count as unsaved")
	}
}

func TestCrudHasUnsavedDraft(t *testing.T) {
	state := dataGridCrudState{DraftRowIDs: map[string]bool{"d1": true}}
	if !dataGridCrudHasUnsaved(state) {
		t.Fatal("draft rows should count as unsaved")
	}
}

func TestCrudHasUnsavedDeleted(t *testing.T) {
	state := dataGridCrudState{DeletedRowIDs: map[string]bool{"x1": true}}
	if !dataGridCrudHasUnsaved(state) {
		t.Fatal("deleted rows should count as unsaved")
	}
}

// --- dataGridCrudRowDeleteEnabled ---

func TestCrudRowDeleteEnabledCrudDisabled(t *testing.T) {
	cfg := &DataGridCfg{ShowCRUDToolbar: false}
	if dataGridCrudRowDeleteEnabled(cfg, true, GridDataCapabilities{SupportsDelete: true}) {
		t.Fatal("should be false when CRUD toolbar disabled")
	}
}

func TestCrudRowDeleteEnabledAllowDeleteFalse(t *testing.T) {
	f := false
	cfg := &DataGridCfg{ShowCRUDToolbar: true, AllowDelete: &f}
	if dataGridCrudRowDeleteEnabled(cfg, true, GridDataCapabilities{SupportsDelete: true}) {
		t.Fatal("should be false when AllowDelete is false")
	}
}

func TestCrudRowDeleteEnabledNoSource(t *testing.T) {
	cfg := &DataGridCfg{ShowCRUDToolbar: true}
	if !dataGridCrudRowDeleteEnabled(cfg, false, GridDataCapabilities{}) {
		t.Fatal("should be true when no data source")
	}
}

func TestCrudRowDeleteEnabledSourceNoSupport(t *testing.T) {
	cfg := &DataGridCfg{ShowCRUDToolbar: true}
	if dataGridCrudRowDeleteEnabled(cfg, true, GridDataCapabilities{SupportsDelete: false}) {
		t.Fatal("should be false when source lacks delete support")
	}
}

func TestCrudRowDeleteEnabledSourceWithSupport(t *testing.T) {
	cfg := &DataGridCfg{ShowCRUDToolbar: true}
	if !dataGridCrudRowDeleteEnabled(cfg, true, GridDataCapabilities{SupportsDelete: true}) {
		t.Fatal("should be true when source supports delete")
	}
}

// --- dataGridRowsSignature ---

func TestRowsSignatureEmpty(t *testing.T) {
	if dataGridRowsSignature(nil, nil) != 0 {
		t.Fatal("empty rows should return 0")
	}
}

func TestRowsSignatureStable(t *testing.T) {
	rows := []GridRow{
		{ID: "a", Cells: map[string]string{"x": "1", "y": "2"}},
		{ID: "b", Cells: map[string]string{"x": "3", "y": "4"}},
	}
	h1 := dataGridRowsSignature(rows, []string{"x", "y"})
	h2 := dataGridRowsSignature(rows, []string{"x", "y"})
	if h1 != h2 {
		t.Fatalf("same input should produce same hash: %d vs %d", h1, h2)
	}
}

func TestRowsSignatureDifferentData(t *testing.T) {
	rows1 := []GridRow{{ID: "a", Cells: map[string]string{"x": "1"}}}
	rows2 := []GridRow{{ID: "a", Cells: map[string]string{"x": "2"}}}
	h1 := dataGridRowsSignature(rows1, []string{"x"})
	h2 := dataGridRowsSignature(rows2, []string{"x"})
	if h1 == h2 {
		t.Fatal("different cell values should produce different hashes")
	}
}

func TestRowsSignatureFallbackKeys(t *testing.T) {
	rows := []GridRow{{ID: "r", Cells: map[string]string{"a": "1", "b": "2"}}}
	h1 := dataGridRowsSignature(rows, nil)
	h2 := dataGridRowsSignature(rows, []string{"a", "b"})
	if h1 != h2 {
		t.Fatalf("nil colIDs should use sorted keys from all rows: %d vs %d", h1, h2)
	}
}

func TestRowsSignatureFallbackIncludesKeysFromAllRows(t *testing.T) {
	rows1 := []GridRow{
		{ID: "r1", Cells: map[string]string{"a": "1"}},
		{ID: "r2", Cells: map[string]string{"a": "2", "b": "x"}},
	}
	rows2 := []GridRow{
		{ID: "r1", Cells: map[string]string{"a": "1"}},
		{ID: "r2", Cells: map[string]string{"a": "2", "b": "y"}},
	}
	h1 := dataGridRowsSignature(rows1, nil)
	h2 := dataGridRowsSignature(rows2, nil)
	if h1 == h2 {
		t.Fatal("fallback signature should include keys introduced in later rows")
	}
}

// --- dataGridRowsIDSignature ---

func TestRowsIDSignatureEmpty(t *testing.T) {
	if dataGridRowsIDSignature(nil) != 0 {
		t.Fatal("empty rows should return 0")
	}
}

func TestRowsIDSignatureStable(t *testing.T) {
	rows := []GridRow{{ID: "a"}, {ID: "b"}}
	h1 := dataGridRowsIDSignature(rows)
	h2 := dataGridRowsIDSignature(rows)
	if h1 != h2 {
		t.Fatal("same IDs should produce same hash")
	}
}

func TestRowsIDSignatureDifferentIDs(t *testing.T) {
	r1 := []GridRow{{ID: "a"}, {ID: "b"}}
	r2 := []GridRow{{ID: "a"}, {ID: "c"}}
	if dataGridRowsIDSignature(r1) == dataGridRowsIDSignature(r2) {
		t.Fatal("different IDs should produce different hashes")
	}
}

// --- dataGridCrudBuildPayload ---

func TestCrudBuildPayloadCreates(t *testing.T) {
	state := dataGridCrudState{
		CommittedRows: []GridRow{{ID: "r1", Cells: map[string]string{"a": "1"}}},
		WorkingRows: []GridRow{
			{ID: "__draft_g_1", Cells: map[string]string{"a": "new"}},
			{ID: "r1", Cells: map[string]string{"a": "1"}},
		},
		DraftRowIDs:   map[string]bool{"__draft_g_1": true},
		DirtyRowIDs:   map[string]bool{"__draft_g_1": true},
		DeletedRowIDs: map[string]bool{},
	}
	creates, updates, edits, deletes := dataGridCrudBuildPayload(state)
	if len(creates) != 1 || creates[0].ID != "__draft_g_1" {
		t.Fatalf("expected 1 create, got %d", len(creates))
	}
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates, got %d", len(updates))
	}
	if len(edits) != 0 {
		t.Fatalf("expected 0 edits, got %d", len(edits))
	}
	if len(deletes) != 0 {
		t.Fatalf("expected 0 deletes, got %d", len(deletes))
	}
}

func TestCrudBuildPayloadUpdates(t *testing.T) {
	state := dataGridCrudState{
		CommittedRows: []GridRow{{ID: "r1", Cells: map[string]string{"a": "old", "b": "same"}}},
		WorkingRows:   []GridRow{{ID: "r1", Cells: map[string]string{"a": "new", "b": "same"}}},
		DirtyRowIDs:   map[string]bool{"r1": true},
		DraftRowIDs:   map[string]bool{},
		DeletedRowIDs: map[string]bool{},
	}
	creates, updates, edits, deletes := dataGridCrudBuildPayload(state)
	if len(creates) != 0 {
		t.Fatalf("expected 0 creates, got %d", len(creates))
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].ColID != "a" || edits[0].Value != "new" {
		t.Fatalf("edit mismatch: %+v", edits[0])
	}
	if len(deletes) != 0 {
		t.Fatalf("expected 0 deletes, got %d", len(deletes))
	}
}

func TestCrudBuildPayloadDeletes(t *testing.T) {
	state := dataGridCrudState{
		CommittedRows: []GridRow{
			{ID: "r1", Cells: map[string]string{}},
			{ID: "r2", Cells: map[string]string{}},
		},
		WorkingRows:   []GridRow{{ID: "r1", Cells: map[string]string{}}},
		DirtyRowIDs:   map[string]bool{},
		DraftRowIDs:   map[string]bool{},
		DeletedRowIDs: map[string]bool{"r2": true},
	}
	_, _, _, deletes := dataGridCrudBuildPayload(state)
	if len(deletes) != 1 || deletes[0] != "r2" {
		t.Fatalf("expected delete of r2, got %v", deletes)
	}
}

func TestCrudBuildPayloadDeletesSorted(t *testing.T) {
	state := dataGridCrudState{
		CommittedRows: nil,
		WorkingRows:   nil,
		DirtyRowIDs:   map[string]bool{},
		DraftRowIDs:   map[string]bool{},
		DeletedRowIDs: map[string]bool{"z": true, "a": true, "m": true},
	}
	_, _, _, deletes := dataGridCrudBuildPayload(state)
	if len(deletes) != 3 {
		t.Fatalf("expected 3 deletes, got %d", len(deletes))
	}
	if deletes[0] != "a" || deletes[1] != "m" || deletes[2] != "z" {
		t.Fatalf("deletes not sorted: %v", deletes)
	}
}

// --- dataGridCrudReplaceCreatedRows ---

func TestCrudReplaceCreatedRows(t *testing.T) {
	rows := []GridRow{
		{ID: "__draft_1", Cells: map[string]string{"a": "x"}},
		{ID: "existing", Cells: map[string]string{"a": "y"}},
	}
	createRows := []GridRow{{ID: "__draft_1", Cells: map[string]string{"a": "x"}}}
	created := []GridRow{{ID: "server_1", Cells: map[string]string{"a": "x"}}}

	idMap, warn := dataGridCrudReplaceCreatedRows(rows, createRows, created)
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if rows[0].ID != "server_1" {
		t.Fatalf("draft row not replaced: %s", rows[0].ID)
	}
	if rows[1].ID != "existing" {
		t.Fatalf("existing row changed: %s", rows[1].ID)
	}
	if idMap["__draft_1"] != "server_1" {
		t.Fatalf("idMap wrong: %v", idMap)
	}
}

func TestCrudReplaceCreatedRowsMismatchCount(t *testing.T) {
	rows := []GridRow{
		{ID: "__d1", Cells: map[string]string{}},
		{ID: "__d2", Cells: map[string]string{}},
	}
	createRows := []GridRow{
		{ID: "__d1", Cells: map[string]string{}},
		{ID: "__d2", Cells: map[string]string{}},
	}
	created := []GridRow{{ID: "s1", Cells: map[string]string{}}}
	_, warn := dataGridCrudReplaceCreatedRows(rows, createRows, created)
	if warn == "" {
		t.Fatal("expected warning for mismatched count")
	}
}

func TestCrudReplaceCreatedRowsNoCreates(t *testing.T) {
	rows := []GridRow{{ID: "r1", Cells: map[string]string{}}}
	idMap, warn := dataGridCrudReplaceCreatedRows(rows, nil, nil)
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if len(idMap) != 0 {
		t.Fatalf("expected empty idMap, got %v", idMap)
	}
}

func TestCrudReplaceCreatedRowsZeroReturned(t *testing.T) {
	createRows := []GridRow{{ID: "__d1", Cells: map[string]string{}}}
	_, warn := dataGridCrudReplaceCreatedRows(nil, createRows, nil)
	if warn == "" {
		t.Fatal("expected warning when source returned 0 rows")
	}
}

// --- dataGridCrudDefaultCells ---

func TestCrudDefaultCells(t *testing.T) {
	cols := []GridColumnCfg{
		{ID: "name", DefaultValue: "unknown"},
		{ID: "age", DefaultValue: "0"},
		{ID: "", DefaultValue: "skip"},
	}
	cells := dataGridCrudDefaultCells(cols)
	if cells["name"] != "unknown" {
		t.Fatalf("name: got %q", cells["name"])
	}
	if cells["age"] != "0" {
		t.Fatalf("age: got %q", cells["age"])
	}
	if _, ok := cells[""]; ok {
		t.Fatal("empty-ID column should be skipped")
	}
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}
}

func TestCrudDefaultCellsEmpty(t *testing.T) {
	cells := dataGridCrudDefaultCells(nil)
	if len(cells) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(cells))
	}
}

// --- dataGridSelectionRemoveIDs ---

func TestSelectionRemoveIDs(t *testing.T) {
	sel := GridSelection{
		AnchorRowID: "a",
		ActiveRowID: "b",
		SelectedRowIDs: map[string]bool{
			"a": true, "b": true, "c": true,
		},
	}
	remove := map[string]bool{"a": true, "c": true}
	result := dataGridSelectionRemoveIDs(sel, remove)
	if result.AnchorRowID != "" {
		t.Fatalf("anchor should be cleared, got %q", result.AnchorRowID)
	}
	if result.ActiveRowID != "b" {
		t.Fatalf("active should remain b, got %q", result.ActiveRowID)
	}
	if len(result.SelectedRowIDs) != 1 || !result.SelectedRowIDs["b"] {
		t.Fatalf("selected should be {b}, got %v", result.SelectedRowIDs)
	}
}

func TestSelectionRemoveIDsNoneRemoved(t *testing.T) {
	sel := GridSelection{
		AnchorRowID:    "x",
		ActiveRowID:    "y",
		SelectedRowIDs: map[string]bool{"x": true, "y": true},
	}
	result := dataGridSelectionRemoveIDs(sel, map[string]bool{})
	if len(result.SelectedRowIDs) != 2 {
		t.Fatalf("expected 2 selected, got %d", len(result.SelectedRowIDs))
	}
}

// --- cloneRows ---

func TestCloneRowsNil(t *testing.T) {
	if cloneRows(nil) != nil {
		t.Fatal("nil input should return nil")
	}
}

func TestCloneRowsDeepCopy(t *testing.T) {
	orig := []GridRow{
		{ID: "r1", Cells: map[string]string{"a": "1", "b": "2"}},
		{ID: "r2", Cells: map[string]string{"x": "9"}},
	}
	clone := cloneRows(orig)
	if len(clone) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(clone))
	}
	// Mutate clone; original must be unaffected.
	clone[0].Cells["a"] = "changed"
	clone[0].ID = "modified"
	if orig[0].Cells["a"] != "1" {
		t.Fatal("original cell mutated via clone")
	}
	if orig[0].ID != "r1" {
		t.Fatal("original ID mutated via clone")
	}
}

func TestCloneRowsEmptySlice(t *testing.T) {
	clone := cloneRows([]GridRow{})
	if clone == nil {
		t.Fatal("empty slice should return non-nil empty slice")
	}
	if len(clone) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(clone))
	}
}

// --- sortedMapKeys (generic) ---

func TestSortedMapKeys(t *testing.T) {
	m := map[string]string{"z": "1", "a": "2", "m": "3"}
	keys := sortedMapKeys(m)
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "m" || keys[2] != "z" {
		t.Fatalf("expected [a m z], got %v", keys)
	}
}

func TestSortedMapKeysEmpty(t *testing.T) {
	keys := sortedMapKeys(map[string]string{})
	if len(keys) != 0 {
		t.Fatalf("expected empty, got %v", keys)
	}
}

func TestSortedMapKeysBoolMap(t *testing.T) {
	m := map[string]bool{"z": true, "a": true, "m": true}
	keys := sortedMapKeys(m)
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "m" || keys[2] != "z" {
		t.Fatalf("expected [a m z], got %v", keys)
	}
}

func TestSortedMapKeysBoolMapEmpty(t *testing.T) {
	keys := sortedMapKeys(map[string]bool{})
	if len(keys) != 0 {
		t.Fatalf("expected empty, got %v", keys)
	}
}

// --- dataGridCrudRemapSelection ---

func TestCrudRemapSelection(t *testing.T) {
	sel := GridSelection{
		AnchorRowID:    "__draft_1",
		ActiveRowID:    "__draft_2",
		SelectedRowIDs: map[string]bool{"__draft_1": true, "__draft_2": true, "keep": true},
	}
	replaceIDs := map[string]string{
		"__draft_1": "server_1",
		"__draft_2": "server_2",
	}
	var captured GridSelection
	cb := func(s GridSelection, _ *Event, _ *Window) { captured = s }
	dataGridCrudRemapSelection(sel, cb, replaceIDs, &Event{}, nil)

	if captured.AnchorRowID != "server_1" {
		t.Fatalf("anchor: got %q", captured.AnchorRowID)
	}
	if captured.ActiveRowID != "server_2" {
		t.Fatalf("active: got %q", captured.ActiveRowID)
	}
	if !captured.SelectedRowIDs["server_1"] || !captured.SelectedRowIDs["server_2"] || !captured.SelectedRowIDs["keep"] {
		t.Fatalf("selected: %v", captured.SelectedRowIDs)
	}
	if len(captured.SelectedRowIDs) != 3 {
		t.Fatalf("expected 3 selected, got %d", len(captured.SelectedRowIDs))
	}
}

func TestCrudRemapSelectionNilCallback(_ *testing.T) {
	// Should not panic.
	dataGridCrudRemapSelection(GridSelection{}, nil, map[string]string{"a": "b"}, &Event{}, nil)
}

func TestCrudRemapSelectionEmptyReplace(t *testing.T) {
	called := false
	cb := func(_ GridSelection, _ *Event, _ *Window) { called = true }
	dataGridCrudRemapSelection(GridSelection{}, cb, map[string]string{}, &Event{}, nil)
	if called {
		t.Fatal("callback should not fire with empty replaceIDs")
	}
}
