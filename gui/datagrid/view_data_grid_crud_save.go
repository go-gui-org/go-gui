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
// server-assigned rows. Returns (idMap, warningMsg). Matching is
// by draft ID, not position, so a draft deleted or reordered
// while saving cannot misalign the server rows.
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
	byDraft := make(map[string]GridRow, min(len(createRows), len(created)))
	for idx := range min(len(createRows), len(created)) {
		draftID := createRows[idx].ID
		if draftID == "" {
			continue
		}
		byDraft[draftID] = created[idx]
	}
	for idx := range rows {
		nextRow, ok := byDraft[rows[idx].ID]
		if !ok {
			continue
		}
		draftID := rows[idx].ID
		rows[idx] = nextRow
		if draftID != "" && nextRow.ID != "" {
			replace[draftID] = nextRow.ID
		}
		delete(byDraft, draftID)
	}
	return replace, warn
}

func dataGridCrudRemapSelection(selection GridSelection, onSelectionChange func(GridSelection, gg.EventCtx), replaceIDs map[string]string, e *gg.Event, w *gg.Window) {
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
	active := selection.activeRowID
	if id, ok := replaceIDs[active]; ok {
		active = id
	}
	anchor := selection.anchorRowID
	if id, ok := replaceIDs[anchor]; ok {
		anchor = id
	}
	onSelectionChange(GridSelection{
		anchorRowID:    anchor,
		activeRowID:    active,
		SelectedRowIDs: selected,
	}, gg.EventCtx{Layout: nil, Event: e, Window: w})
}

// dataGridCrudMutationResult holds the outcome of async
// mutation execution.
type dataGridCrudMutationResult struct {
	errPhase   string    // "create"/"update"/"delete" on error
	errMsg     string    // error message (empty on success)
	createRows []GridRow // input create rows (for replace mapping)
	created    []GridRow // server-returned created rows
	rowCount   int       // -1 when unknown
	requestID  uint64    // dataGridCrudState.RequestID the save ran under
}

type dataGridCrudSaveContext struct {
	selection         GridSelection
	dataSource        DataGridDataSource
	onCRUDError       func(string, gg.EventCtx)
	onRowsChange      func([]GridRow, gg.EventCtx)
	onSelectionChange func(GridSelection, gg.EventCtx)
	query             GridQueryState
	gridID            string
	focusID           string
	errCue            gg.SoundCue
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
	// Fail fast on unbounded batches instead of fanning one save
	// into an unbounded mutation storm that holds locks for ages.
	// The working copy is kept: nothing was attempted, so there is
	// nothing to roll back, and the user can shrink the batch and
	// retry.
	if len(createRows) > dataGridMaxMutationBatch ||
		len(updateRows) > dataGridMaxMutationBatch ||
		len(deleteIDs) > dataGridMaxMutationBatch {
		msg := fmt.Sprintf("grid %s: mutation batch exceeds %d per kind", gridID, dataGridMaxMutationBatch)
		state.SaveError = msg
		dgCrud.Set(gridID, state)
		w.PlaySoundCue(ctx.errCue)
		if ctx.onCRUDError != nil {
			ctx.onCRUDError(msg, gg.EventCtx{Layout: nil, Event: e, Window: w})
		}
		return
	}
	state.Saving = true
	state.SaveError = ""
	dgCrud.Set(gridID, state)

	if ctx.hasSource {
		source := ctx.dataSource
		if source == nil {
			state.Saving = false
			state.SaveError = fmt.Sprintf("grid %s: data source unavailable", gridID)
			dgCrud.Set(gridID, state)
			return
		}
		// Pre-validate capabilities.
		if len(createRows) > 0 && !ctx.caps.supportsCreate {
			dataGridCrudRestoreOnError(gridID, "create", ctx.onCRUDError,
				e, w, snapshotRows, "grid: create not supported", ctx.errCue)
			return
		}
		if len(updateEdits) > 0 && !ctx.caps.supportsUpdate {
			dataGridCrudRestoreOnError(gridID, "update", ctx.onCRUDError,
				e, w, snapshotRows, "grid: update not supported", ctx.errCue)
			return
		}
		if len(deleteIDs) > 0 && !ctx.caps.supportsDelete {
			dataGridCrudRestoreOnError(gridID, "delete", ctx.onCRUDError,
				e, w, snapshotRows, "grid: delete not supported", ctx.errCue)
			return
		}
		query := ctx.query
		onCRUDError := ctx.onCRUDError
		onRowsChange := ctx.onRowsChange
		selection := ctx.selection
		onSelectionChange := ctx.onSelectionChange
		focusID := ctx.focusID
		errCue := ctx.errCue
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
		// Deep-copy the payload: WorkingRows cell maps stay live
		// under further keystrokes while the save runs, so the
		// goroutine must not share them.
		saveCreates := cloneRows(createRows)
		saveUpdates := cloneRows(updateRows)
		saveEdits := append([]GridCellEdit(nil), updateEdits...)
		saveDeletes := append([]string(nil), deleteIDs...)
		// Close any open cell editor. Edits are refused while Saving
		// (see dataGridCrudApplyCellEdit), so an editor left open would
		// take keystrokes that go nowhere.
		dataGridClearEditingRow(gridID, w)
		go func() {
			result := dataGridCrudExecMutations(source, gridID, query,
				saveCreates, saveUpdates, saveEdits, saveDeletes,
				ctrl.Signal, nextRequestID)
			result.requestID = nextRequestID
			if wCtx.Err() != nil {
				return
			}
			w.QueueCommand(func(w *gg.Window) {
				dataGridCrudApplySaveResult(gridID, result, snapshotRows,
					onCRUDError, onRowsChange, selection, onSelectionChange,
					focusID, errCue, w)
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
			gridID:    gridID,
			Kind:      gridMutationCreate,
			Query:     query,
			Rows:      createRows,
			Signal:    signal,
			RequestID: requestID,
		})
		if err != nil {
			return dataGridCrudMutationResult{errPhase: "create", errMsg: err.Error()}
		}
		created = append([]GridRow(nil), res.created...)
		if res.RowCount >= 0 {
			rowCount = res.RowCount
		}
	}
	if len(updateEdits) > 0 {
		res, err := source.MutateData(GridMutationRequest{
			gridID:    gridID,
			Kind:      gridMutationUpdate,
			Query:     query,
			Rows:      updateRows,
			edits:     updateEdits,
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
			gridID:    gridID,
			Kind:      gridMutationDelete,
			Query:     query,
			rowIDs:    deleteIDs,
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

func dataGridCrudApplySaveResult(gridID string, result dataGridCrudMutationResult, snapshotRows []GridRow, onCRUDError func(string, gg.EventCtx), onRowsChange func([]GridRow, gg.EventCtx), selection GridSelection, onSelectionChange func(GridSelection, gg.EventCtx), focusID string, errCue gg.SoundCue, w *gg.Window) {
	e := &gg.Event{}
	// A result from a save that was cancelled (Escape) or replaced by
	// a newer save is stale. Applying it would restore or commit rows
	// the user has since discarded or changed, and clear the flags of
	// the save that now runs. dataGridCrudCancel and dataGridCrudSave
	// both bump RequestID, so a mismatch means the result is stale.
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	if cur, ok := dgCrud.Get(gridID); !ok || cur.RequestID != result.requestID {
		return
	}
	if result.errMsg != "" {
		// The creates already committed server-side before the
		// update/delete failed. Restoring the snapshot would drop
		// them, so a retry would duplicate them: commit the
		// creates, keep the failed phase dirty for retry.
		if (result.errPhase == "update" || result.errPhase == "delete") && len(result.created) > 0 {
			dataGridCrudCommitCreatesKeepDirty(gridID, result, snapshotRows,
				onCRUDError, selection, onSelectionChange, focusID, errCue, e, w)
			return
		}
		dataGridCrudRestoreOnError(gridID, result.errPhase, onCRUDError,
			e, w, snapshotRows, result.errMsg, errCue)
		return
	}
	// Default zero state: absent entry means save was not in CRUD mode.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	replaceIDs, createWarn := dataGridCrudReplaceCreatedRows(
		state.WorkingRows, result.createRows, result.created)
	// A count mismatch no longer discards the good updates and
	// deletes with a full rollback: map what the server returned,
	// finish the save, then surface the warning.
	if createWarn != "" && len(result.created) == 0 {
		dataGridCrudRestoreOnError(gridID, "create", onCRUDError,
			e, w, snapshotRows, createWarn, errCue)
		return
	}
	state.WorkingVersion++
	dgCrud.Set(gridID, state)
	dataGridCrudRemapSelection(selection, onSelectionChange, replaceIDs, e, w)
	dataGridCrudFinishSave(gridID, replaceIDs, result.rowCount,
		onRowsChange, true, focusID, e, w)
	if createWarn != "" {
		stuck := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
		warnState := stuck.GetOr(gridID, dataGridCrudState{})
		warnState.SaveError = createWarn
		stuck.Set(gridID, warnState)
		if onCRUDError != nil {
			onCRUDError(createWarn, gg.EventCtx{Layout: nil, Event: e, Window: w})
		}
	}
}

// dataGridCrudCommitCreatesKeepDirty commits server-created rows
// after a later phase failed, keeping the failed updates/deletes
// dirty so a retry sends only those.
func dataGridCrudCommitCreatesKeepDirty(gridID string, result dataGridCrudMutationResult, snapshotRows []GridRow, onCRUDError func(string, gg.EventCtx), selection GridSelection, onSelectionChange func(GridSelection, gg.EventCtx), focusID string, errCue gg.SoundCue, e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means nothing to preserve.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	replaceIDs, _ := dataGridCrudReplaceCreatedRows(
		state.WorkingRows, result.createRows, result.created)
	for draftID := range replaceIDs {
		delete(state.DraftRowIDs, draftID)
		delete(state.DirtyRowIDs, draftID)
	}
	committed := cloneRows(snapshotRows)
	seen := make(map[string]bool, len(committed)+len(result.created))
	for idx, row := range committed {
		seen[dataGridRowID(row, idx)] = true
	}
	for _, created := range result.created {
		rowID := created.ID
		if rowID == "" || seen[rowID] {
			continue
		}
		seen[rowID] = true
		committed = append(committed, created)
	}
	state.CommittedRows = committed
	state.WorkingVersion++
	state.Saving = false
	state.ActiveAbort = nil
	state.SourceChanged = false
	state.SaveError = result.errPhase + ": " + result.errMsg
	state.SourceSignature = dataGridRowsSignature(state.CommittedRows, nil)
	dgCrud.Set(gridID, state)
	dataGridClearEditingRow(gridID, w)
	dataGridSourceForceRefetch(gridID, w)
	w.PlaySoundCue(errCue)
	dataGridCrudRemapSelection(selection, onSelectionChange, replaceIDs, e, w)
	if focusID != "" {
		w.SetFocus(focusID)
	}
	if onCRUDError != nil {
		onCRUDError(result.errMsg, gg.EventCtx{Layout: nil, Event: e, Window: w})
	}
}

func dataGridCrudFinishSave(gridID string, _ map[string]string, rowCount int, onRowsChange func([]GridRow, gg.EventCtx), hasSource bool, focusID string, e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means no CRUD state to finalize.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	state.CommittedRows = cloneRows(state.WorkingRows)
	dataGridCrudClearPendingChanges(&state)
	state.WorkingVersion++
	state.Saving = false
	state.ActiveAbort = nil
	state.SaveError = ""
	state.SourceChanged = false
	state.SourceSignature = dataGridRowsSignature(state.CommittedRows, nil)
	dgCrud.Set(gridID, state)
	dataGridClearEditingRow(gridID, w)
	rowsCopy := cloneRows(state.WorkingRows)
	if onRowsChange != nil {
		onRowsChange(rowsCopy, gg.EventCtx{Layout: nil, Event: e, Window: w})
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

func dataGridCrudRestoreOnError(gridID, phase string, onCRUDError func(string, gg.EventCtx), e *gg.Event, w *gg.Window, snapshotRows []GridRow, errMsg string, errCue gg.SoundCue) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means nothing to restore on error.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	state.CommittedRows = snapshotRows
	state.WorkingRows = cloneRows(snapshotRows)
	dataGridCrudClearPendingChanges(&state)
	state.WorkingVersion++
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
	// Every CRUD failure funnels through here, so this is the one emit
	// site. Before the callback, like every other cue. datagrid is
	// outside gui/, so it goes through the exported seam rather than
	// the unexported playSoundCue (issue #469).
	w.PlaySoundCue(errCue)
	if onCRUDError != nil {
		onCRUDError(errMsg, gg.EventCtx{Layout: nil, Event: e, Window: w})
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
