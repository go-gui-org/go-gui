package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
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
	cb := func(s GridSelection, _ *gg.Event, _ *gg.Window) { captured = s }
	dataGridCrudRemapSelection(sel, cb, replaceIDs, &gg.Event{}, nil)

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
	dataGridCrudRemapSelection(GridSelection{}, nil, map[string]string{"a": "b"}, &gg.Event{}, nil)
}

func TestCrudRemapSelectionEmptyReplace(t *testing.T) {
	called := false
	cb := func(_ GridSelection, _ *gg.Event, _ *gg.Window) { called = true }
	dataGridCrudRemapSelection(GridSelection{}, cb, map[string]string{}, &gg.Event{}, nil)
	if called {
		t.Fatal("callback should not fire with empty replaceIDs")
	}
}

// --- dataGridCrudClearPendingChanges ---

func TestCrudClearPendingChanges(t *testing.T) {
	state := &dataGridCrudState{
		DirtyRowIDs:   map[string]bool{"r1": true},
		DraftRowIDs:   map[string]bool{"d1": true},
		DeletedRowIDs: map[string]bool{"x1": true},
	}
	dataGridCrudClearPendingChanges(state)
	if len(state.DirtyRowIDs) != 0 {
		t.Error("dirty should be cleared")
	}
	if len(state.DraftRowIDs) != 0 {
		t.Error("draft should be cleared")
	}
	if len(state.DeletedRowIDs) != 0 {
		t.Error("deleted should be cleared")
	}
}

// --- dataGridCrudToolbarHeight ---

func TestCrudToolbarHeight(t *testing.T) {
	cfg := &DataGridCfg{HeaderHeight: 28}
	got := dataGridCrudToolbarHeight(cfg)
	if got != 28 {
		t.Errorf("got %v, want 28", got)
	}
}

// --- dataGridCrudToolbarRow ---

func TestCrudToolbarRowReturnsView(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleFilter:  gg.DefaultTextStyle,
		ColorFilter:      gg.RGBA(240, 240, 240, 255),
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
		ColorBorder:      gg.RGBA(180, 180, 180, 255),
		Selection:        GridSelection{},
		Columns:          []GridColumnCfg{{ID: "c1"}},
	}
	state := dataGridCrudState{}
	v := dataGridCrudToolbarRow(cfg, state, GridDataCapabilities{}, false, 0)
	if v == nil {
		t.Fatal("toolbar row should return a view")
	}
}

func TestCrudToolbarRowWithUnsaved(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleFilter:  gg.DefaultTextStyle,
		ColorFilter:      gg.RGBA(240, 240, 240, 255),
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
		ColorBorder:      gg.RGBA(180, 180, 180, 255),
		Selection:        GridSelection{},
		Columns:          []GridColumnCfg{{ID: "c1"}},
	}
	state := dataGridCrudState{
		DirtyRowIDs: map[string]bool{"r1": true},
	}
	v := dataGridCrudToolbarRow(cfg, state, GridDataCapabilities{}, false, 0)
	if v == nil {
		t.Fatal("toolbar row with unsaved should return a view")
	}
}

func TestCrudToolbarRowSaving(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleFilter:  gg.DefaultTextStyle,
		ColorFilter:      gg.RGBA(240, 240, 240, 255),
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
		ColorBorder:      gg.RGBA(180, 180, 180, 255),
		Selection:        GridSelection{},
		Columns:          []GridColumnCfg{{ID: "c1"}},
	}
	state := dataGridCrudState{Saving: true}
	v := dataGridCrudToolbarRow(cfg, state, GridDataCapabilities{}, false, 0)
	if v == nil {
		t.Fatal("toolbar row while saving should return a view")
	}
}

// --- dataGridCrudApplyCellEdit ---

func TestCrudApplyCellEdit(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, 4)
	dgCrud.Set("g1", dataGridCrudState{
		WorkingRows: []GridRow{
			{ID: "r1", Cells: map[string]string{"col1": "old"}},
		},
	})
	e := &gg.Event{}
	dataGridCrudApplyCellEdit("g1", true, nil, GridCellEdit{
		RowID: "r1",
		ColID: "col1",
		Value: "new",
	}, e, w)
	state, _ := dgCrud.Get("g1")
	if !state.DirtyRowIDs["r1"] {
		t.Fatal("r1 should be marked dirty")
	}
	if state.WorkingRows[0].Cells["col1"] != "new" {
		t.Errorf("cell: got %q, want new", state.WorkingRows[0].Cells["col1"])
	}
}

func TestCrudApplyCellEditNoCrud(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	var captured GridCellEdit
	cb := func(edit GridCellEdit, _ *gg.Event, _ *gg.Window) {
		captured = edit
	}
	e := &gg.Event{}
	dataGridCrudApplyCellEdit("g1", false, cb, GridCellEdit{
		RowID: "r1",
		ColID: "col1",
		Value: "new",
	}, e, w)
	if captured.Value != "new" {
		t.Errorf("callback: got %q, want new", captured.Value)
	}
}

func TestCrudApplyCellEditEmptyRowID(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic.
	dataGridCrudApplyCellEdit("g1", true, nil, GridCellEdit{
		RowID: "",
		ColID: "col1",
		Value: "val",
	}, &gg.Event{}, w)
}

// --- dataGridCrudCancel ---

func TestCrudCancel(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, 4)
	dgCrud.Set("g1", dataGridCrudState{
		CommittedRows: []GridRow{{ID: "r1", Cells: map[string]string{"x": "original"}}},
		WorkingRows:   []GridRow{{ID: "r1", Cells: map[string]string{"x": "modified"}}},
		DirtyRowIDs:   map[string]bool{"r1": true},
		SaveError:     "some error",
		Saving:        true,
		SourceChanged: true,
	})
	e := &gg.Event{}
	dataGridCrudCancel("g1", 10, e, w)
	state, _ := dgCrud.Get("g1")
	if len(state.DirtyRowIDs) != 0 {
		t.Error("dirty should be cleared")
	}
	if state.Saving {
		t.Error("saving should be false")
	}
	if state.SaveError != "" {
		t.Errorf("save error: got %q, want empty", state.SaveError)
	}
	if state.WorkingRows[0].Cells["x"] != "original" {
		t.Error("working rows should be restored from committed")
	}
	if !e.IsHandled {
		t.Fatal("should be handled")
	}
}

// --- dataGridCrudDeleteRows ---

func TestCrudDeleteRows(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, 4)
	dgCrud.Set("g1", dataGridCrudState{
		CommittedRows: []GridRow{{ID: "r1", Cells: map[string]string{}}, {ID: "r2", Cells: map[string]string{}}},
		WorkingRows: []GridRow{
			{ID: "__draft_1", Cells: map[string]string{}},
			{ID: "r1", Cells: map[string]string{}},
			{ID: "r2", Cells: map[string]string{}},
		},
		DraftRowIDs: map[string]bool{"__draft_1": true},
	})
	var captured GridSelection
	e := &gg.Event{}
	dataGridCrudDeleteRows("g1", GridSelection{}, func(s GridSelection, _ *gg.Event, _ *gg.Window) {
		captured = s
	}, []string{"__draft_1", "r2"}, 10, e, w)
	state, _ := dgCrud.Get("g1")
	if len(state.WorkingRows) != 1 {
		t.Errorf("working rows: got %d, want 1", len(state.WorkingRows))
	}
	if state.WorkingRows[0].ID != "r1" {
		t.Errorf("remaining: got %s, want r1", state.WorkingRows[0].ID)
	}
	if state.DeletedRowIDs["r2"] != true {
		t.Error("r2 should be marked deleted")
	}
	if state.DeletedRowIDs["__draft_1"] {
		t.Error("draft should not be marked deleted")
	}
	if !e.IsHandled {
		t.Fatal("should be handled")
	}
	_ = captured
}

func TestCrudDeleteRowsEmptyList(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic.
	dataGridCrudDeleteRows("g1", GridSelection{}, nil, nil, 0, &gg.Event{}, w)
}

func TestCrudDeleteRowsAllWhitespace(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic with whitespace-only IDs.
	dataGridCrudDeleteRows("g1", GridSelection{}, nil, []string{"  ", ""}, 0, &gg.Event{}, w)
}

// --- dataGridCrudAddRow ---

func TestCrudAddRow(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	columns := []GridColumnCfg{{ID: "name", DefaultValue: ""}, {ID: "age", DefaultValue: "0"}}
	var captured GridSelection
	e := &gg.Event{}
	dataGridCrudAddRow("g1", columns, func(s GridSelection, _ *gg.Event, _ *gg.Window) {
		captured = s
	}, 10, 1, 0, 0, func(int, *gg.Event, *gg.Window) {}, e, w)
	if captured.ActiveRowID == "" {
		t.Fatal("should have an active row")
	}
	if len(captured.SelectedRowIDs) != 1 {
		t.Errorf("expected 1 selected, got %d", len(captured.SelectedRowIDs))
	}
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, 4)
	state, _ := dgCrud.Get("g1")
	if len(state.WorkingRows) != 1 {
		t.Errorf("working rows: got %d, want 1", len(state.WorkingRows))
	}
	if !e.IsHandled {
		t.Fatal("should be handled")
	}
}

// --- dataGridCrudDeleteSelected ---

func TestCrudDeleteSelected(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, 4)
	dgCrud.Set("g1", dataGridCrudState{
		WorkingRows: []GridRow{
			{ID: "r1", Cells: map[string]string{}},
			{ID: "r2", Cells: map[string]string{}},
		},
	})
	sel := GridSelection{
		SelectedRowIDs: map[string]bool{"r1": true, "r2": true},
	}
	e := &gg.Event{}
	dataGridCrudDeleteSelected("g1", sel, nil, 0, e, w)
	state, _ := dgCrud.Get("g1")
	if len(state.WorkingRows) != 0 {
		t.Errorf("working rows: got %d, want 0", len(state.WorkingRows))
	}
}

func TestCrudDeleteSelectedEmptySelection(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic.
	dataGridCrudDeleteSelected("g1", GridSelection{}, nil, 0, &gg.Event{}, w)
}

// --- dataGridCrudBuildPayload additional ---

func TestCrudBuildPayloadDirtyNotCommitted(t *testing.T) {
	state := dataGridCrudState{
		CommittedRows: []GridRow{},
		WorkingRows: []GridRow{
			{ID: "r1", Cells: map[string]string{"a": "val"}},
		},
		DirtyRowIDs:   map[string]bool{"r1": true},
		DraftRowIDs:   map[string]bool{},
		DeletedRowIDs: map[string]bool{},
	}
	creates, updates, edits, _ := dataGridCrudBuildPayload(state)
	if len(creates) != 0 {
		t.Errorf("creates: got %d, want 0", len(creates))
	}
	if len(updates) != 1 {
		t.Errorf("updates: got %d, want 1", len(updates))
	}
	if len(edits) != 1 {
		t.Errorf("edits: got %d, want 1", len(edits))
	}
}

// --- dataGridCrudRestoreOnError ---

func TestCrudRestoreOnError(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dataGridSetEditingRow("g1", "r1", w)
	snapshot := []GridRow{
		{ID: "r1", Cells: map[string]string{"a": "orig"}},
	}
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, 4)
	dgCrud.Set("g1", dataGridCrudState{
		CommittedRows: []GridRow{{ID: "r1", Cells: map[string]string{"a": "modified"}}},
		WorkingRows:   []GridRow{{ID: "r1", Cells: map[string]string{"a": "modified"}}},
		DirtyRowIDs:   map[string]bool{"r1": true},
	})
	var errMsg string
	cb := func(msg string, _ *gg.Event, _ *gg.Window) {
		errMsg = msg
	}
	e := &gg.Event{}
	dataGridCrudRestoreOnError("g1", "save", cb, e, w, snapshot, "save failed")
	if errMsg != "save failed" {
		t.Errorf("error callback: got %q, want 'save failed'", errMsg)
	}
	state, _ := dgCrud.Get("g1")
	if state.Saving {
		t.Error("saving should be false")
	}
	if state.SaveError != "save: save failed" {
		t.Errorf("save error: got %q", state.SaveError)
	}
	if state.WorkingRows[0].Cells["a"] != "orig" {
		t.Error("working rows should be restored from snapshot")
	}
	if dataGridEditingRowID("g1", w) != "" {
		t.Error("editing row should be cleared")
	}
}

func TestCrudRestoreOnErrorNoPhase(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	snapshot := []GridRow{{ID: "r1", Cells: map[string]string{}}}
	dataGridCrudRestoreOnError("g1", "", nil, &gg.Event{}, w, snapshot, "generic error")
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, 4)
	state, _ := dgCrud.Get("g1")
	if state.SaveError != "generic error" {
		t.Errorf("save error: got %q", state.SaveError)
	}
}
