package datagrid

import (
	"fmt"
	"maps"
	"slices"

	gg "github.com/go-gui-org/go-gui/gui"
)

// dataGridCrudBuildPayload diffs working vs committed rows to
// produce create/update/delete mutation lists.
func dataGridCrudBuildPayload(state dataGridCrudState) (createRows, updateRows []GridRow, updateEdits []GridCellEdit, deleteIDs []string) {
	committedMap := make(map[string]GridRow, len(state.CommittedRows))
	for idx, row := range state.CommittedRows {
		committedMap[dataGridRowID(row, idx)] = row
	}
	for idx, row := range state.WorkingRows {
		rowID := dataGridRowID(row, idx)
		if state.DraftRowIDs[rowID] {
			createRows = append(createRows, row)
			continue
		}
		if !state.DirtyRowIDs[rowID] {
			continue
		}
		updateRows = append(updateRows, row)
		before, ok := committedMap[rowID]
		if !ok {
			before = GridRow{ID: rowID, Cells: map[string]string{}}
		}
		// Collect all keys from both old and new cells.
		keySet := make(map[string]bool, len(row.Cells)+len(before.Cells))
		for k := range row.Cells {
			keySet[k] = true
		}
		for k := range before.Cells {
			keySet[k] = true
		}
		keys := sortedMapKeys(keySet)
		for _, key := range keys {
			nextVal := row.Cells[key]
			prevVal := before.Cells[key]
			if nextVal == prevVal {
				continue
			}
			updateEdits = append(updateEdits, GridCellEdit{
				RowID: rowID,
				ColID: key,
				Value: nextVal,
			})
		}
	}
	for rowID := range state.DeletedRowIDs {
		deleteIDs = append(deleteIDs, rowID)
	}
	slices.Sort(deleteIDs)
	return
}

// dataGridCrudReplaceCreatedRows replaces draft rows with
// server-assigned rows. Returns (idMap, warningMsg).
func dataGridCrudReplaceCreatedRows(rows []GridRow, createRows, created []GridRow) (map[string]string, string) {
	replace := map[string]string{}
	if len(createRows) == 0 || len(created) == 0 {
		if len(createRows) > 0 && len(created) == 0 {
			return replace, fmt.Sprintf("grid: source returned 0 created rows, expected %d", len(createRows))
		}
		return replace, ""
	}
	var warn string
	if len(created) != len(createRows) {
		warn = fmt.Sprintf("grid: source returned %d created rows, expected %d", len(created), len(createRows))
	}
	draftPos := 0
	for idx := range rows {
		if draftPos >= len(createRows) || draftPos >= len(created) {
			break
		}
		draftID := createRows[draftPos].ID
		if rows[idx].ID != draftID {
			continue
		}
		nextRow := created[draftPos]
		rows[idx] = nextRow
		if draftID != "" && nextRow.ID != "" {
			replace[draftID] = nextRow.ID
		}
		draftPos++
	}
	return replace, warn
}

func dataGridCrudRemapSelection(selection GridSelection, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), replaceIDs map[string]string, e *gg.Event, w *gg.Window) {
	if onSelectionChange == nil || len(replaceIDs) == 0 {
		return
	}
	selected := make(map[string]bool, len(selection.SelectedRowIDs))
	for rowID, value := range selection.SelectedRowIDs {
		if !value {
			continue
		}
		if nextID, ok := replaceIDs[rowID]; ok {
			selected[nextID] = true
		} else {
			selected[rowID] = true
		}
	}
	active := selection.ActiveRowID
	if id, ok := replaceIDs[active]; ok {
		active = id
	}
	anchor := selection.AnchorRowID
	if id, ok := replaceIDs[anchor]; ok {
		anchor = id
	}
	onSelectionChange(GridSelection{
		AnchorRowID:    anchor,
		ActiveRowID:    active,
		SelectedRowIDs: selected,
	}, e, w)
}

// dataGridCrudMutationResult holds the outcome of async
// mutation execution.
type dataGridCrudMutationResult struct {
	errPhase   string    // "create"/"update"/"delete" on error
	errMsg     string    // error message (empty on success)
	createRows []GridRow // input create rows (for replace mapping)
	created    []GridRow // server-returned created rows
	rowCount   int       // -1 when unknown
}

type dataGridCrudSaveContext struct {
	selection         GridSelection
	dataSource        DataGridDataSource
	onCRUDError       func(string, *gg.Event, *gg.Window)
	onRowsChange      func([]GridRow, *gg.Event, *gg.Window)
	onSelectionChange func(GridSelection, *gg.Event, *gg.Window)
	query             GridQueryState
	gridID            string
	focusID           string
	caps              GridDataCapabilities
	hasSource         bool
}

func dataGridCrudSave(ctx dataGridCrudSaveContext, e *gg.Event, w *gg.Window) {
	gridID := ctx.gridID
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means nothing to save.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	if state.Saving || !dataGridCrudHasUnsaved(state) {
		return
	}
	createRows, updateRows, updateEdits, deleteIDs := dataGridCrudBuildPayload(state)
	snapshotRows := cloneRows(state.CommittedRows)
	state.Saving = true
	state.SaveError = ""
	dgCrud.Set(gridID, state)

	if ctx.hasSource {
		source := ctx.dataSource
		if source == nil {
			state.Saving = false
			state.SaveError = "grid: data source unavailable"
			dgCrud.Set(gridID, state)
			return
		}
		// Pre-validate capabilities.
		if len(createRows) > 0 && !ctx.caps.SupportsCreate {
			dataGridCrudRestoreOnError(gridID, "create", ctx.onCRUDError,
				e, w, snapshotRows, "grid: create not supported")
			return
		}
		if len(updateEdits) > 0 && !ctx.caps.SupportsUpdate {
			dataGridCrudRestoreOnError(gridID, "update", ctx.onCRUDError,
				e, w, snapshotRows, "grid: update not supported")
			return
		}
		if len(deleteIDs) > 0 && !ctx.caps.SupportsDelete {
			dataGridCrudRestoreOnError(gridID, "delete", ctx.onCRUDError,
				e, w, snapshotRows, "grid: delete not supported")
			return
		}
		query := ctx.query
		onCRUDError := ctx.onCRUDError
		onRowsChange := ctx.onRowsChange
		selection := ctx.selection
		onSelectionChange := ctx.onSelectionChange
		focusID := ctx.focusID
		wCtx := w.Ctx()
		// Cancel any prior in-flight save and set up abort for
		// this save so slow mutations can observe cancellation.
		if state.ActiveAbort != nil {
			state.ActiveAbort.Abort()
		}
		ctrl := gg.NewGridAbortController()
		state.ActiveAbort = ctrl
		state.RequestID++
		nextRequestID := state.RequestID
		dgCrud.Set(gridID, state)
		go func() {
			result := dataGridCrudExecMutations(source, gridID, query,
				createRows, updateRows, updateEdits, deleteIDs,
				ctrl.Signal, nextRequestID)
			if wCtx.Err() != nil {
				return
			}
			w.QueueCommand(func(w *gg.Window) {
				dataGridCrudApplySaveResult(gridID, result, snapshotRows,
					onCRUDError, onRowsChange, selection, onSelectionChange,
					focusID, w)
			})
		}()
	} else {
		// Local-rows mode: no I/O, apply immediately.
		dataGridCrudFinishSave(gridID, nil, -1, ctx.onRowsChange,
			false, ctx.focusID, e, w)
	}
	e.IsHandled = true
}

func dataGridCrudExecMutations(source DataGridDataSource, gridID string, query GridQueryState, createRows, updateRows []GridRow, updateEdits []GridCellEdit, deleteIDs []string, signal *gg.GridAbortSignal, requestID uint64) dataGridCrudMutationResult {
	rowCount := -1
	var created []GridRow
	if len(createRows) > 0 {
		res, err := source.MutateData(GridMutationRequest{
			GridID:    gridID,
			Kind:      GridMutationCreate,
			Query:     query,
			Rows:      createRows,
			Signal:    signal,
			RequestID: requestID,
		})
		if err != nil {
			return dataGridCrudMutationResult{errPhase: "create", errMsg: err.Error()}
		}
		created = append([]GridRow(nil), res.Created...)
		if res.RowCount >= 0 {
			rowCount = res.RowCount
		}
	}
	if len(updateEdits) > 0 {
		res, err := source.MutateData(GridMutationRequest{
			GridID:    gridID,
			Kind:      GridMutationUpdate,
			Query:     query,
			Rows:      updateRows,
			Edits:     updateEdits,
			Signal:    signal,
			RequestID: requestID,
		})
		if err != nil {
			return dataGridCrudMutationResult{
				createRows: createRows, created: created,
				errPhase: "update", errMsg: err.Error(),
			}
		}
		if res.RowCount >= 0 {
			rowCount = res.RowCount
		}
	}
	if len(deleteIDs) > 0 {
		res, err := source.MutateData(GridMutationRequest{
			GridID:    gridID,
			Kind:      GridMutationDelete,
			Query:     query,
			RowIDs:    deleteIDs,
			Signal:    signal,
			RequestID: requestID,
		})
		if err != nil {
			return dataGridCrudMutationResult{
				createRows: createRows, created: created,
				errPhase: "delete", errMsg: err.Error(),
			}
		}
		if res.RowCount >= 0 {
			rowCount = res.RowCount
		}
	}
	return dataGridCrudMutationResult{
		createRows: createRows,
		created:    created,
		rowCount:   rowCount,
	}
}

func dataGridCrudApplySaveResult(gridID string, result dataGridCrudMutationResult, snapshotRows []GridRow, onCRUDError func(string, *gg.Event, *gg.Window), onRowsChange func([]GridRow, *gg.Event, *gg.Window), selection GridSelection, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), focusID string, w *gg.Window) {
	e := &gg.Event{}
	if result.errMsg != "" {
		dataGridCrudRestoreOnError(gridID, result.errPhase, onCRUDError,
			e, w, snapshotRows, result.errMsg)
		return
	}
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means save was not in CRUD mode.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	replaceIDs, createWarn := dataGridCrudReplaceCreatedRows(
		state.WorkingRows, result.createRows, result.created)
	if createWarn != "" {
		dataGridCrudRestoreOnError(gridID, "create", onCRUDError,
			e, w, snapshotRows, createWarn)
		return
	}
	dgCrud.Set(gridID, state)
	dataGridCrudRemapSelection(selection, onSelectionChange, replaceIDs, e, w)
	dataGridCrudFinishSave(gridID, replaceIDs, result.rowCount,
		onRowsChange, true, focusID, e, w)
}

func dataGridCrudFinishSave(gridID string, _ map[string]string, rowCount int, onRowsChange func([]GridRow, *gg.Event, *gg.Window), hasSource bool, focusID string, e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means no CRUD state to finalize.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	state.CommittedRows = cloneRows(state.WorkingRows)
	dataGridCrudClearPendingChanges(&state)
	state.Saving = false
	state.ActiveAbort = nil
	state.SaveError = ""
	state.SourceChanged = false
	state.SourceSignature = dataGridRowsSignature(state.CommittedRows, nil)
	dgCrud.Set(gridID, state)
	dataGridClearEditingRow(gridID, w)
	rowsCopy := cloneRows(state.WorkingRows)
	if onRowsChange != nil {
		onRowsChange(rowsCopy, e, w)
	}
	if hasSource {
		rc := -1
		if rowCount >= 0 {
			rc = rowCount
		}
		dataGridSourceApplyLocalMutation(gridID, rowsCopy, rc, w)
		dataGridSourceForceRefetch(gridID, w)
	}
	if focusID != "" {
		w.SetFocus(focusID)
	}
}

func dataGridCrudRestoreOnError(gridID, phase string, onCRUDError func(string, *gg.Event, *gg.Window), e *gg.Event, w *gg.Window, snapshotRows []GridRow, errMsg string) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means nothing to restore on error.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	state.CommittedRows = snapshotRows
	state.WorkingRows = cloneRows(snapshotRows)
	dataGridCrudClearPendingChanges(&state)
	state.Saving = false
	state.ActiveAbort = nil
	state.SourceChanged = false
	if phase != "" {
		state.SaveError = phase + ": " + errMsg
	} else {
		state.SaveError = errMsg
	}
	state.SourceSignature = dataGridRowsSignature(state.CommittedRows, nil)
	dgCrud.Set(gridID, state)
	dataGridClearEditingRow(gridID, w)
	dataGridSourceForceRefetch(gridID, w)
	if onCRUDError != nil {
		onCRUDError(errMsg, e, w)
	}
}

// --- helpers ---

func cloneRows(rows []GridRow) []GridRow {
	if rows == nil {
		return nil
	}
	out := make([]GridRow, len(rows))
	for i, row := range rows {
		cells := make(map[string]string, len(row.Cells))
		maps.Copy(cells, row.Cells)
		out[i] = GridRow{ID: row.ID, Cells: cells}
	}
	return out
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
