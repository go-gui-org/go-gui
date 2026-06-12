package datagrid

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	gg "github.com/go-gui-org/go-gui/gui"
)

// dataGridCrudClearPendingChanges resets dirty/draft/deleted
// tracking maps to empty.
func dataGridCrudClearPendingChanges(state *dataGridCrudState) {
	state.DirtyRowIDs = map[string]bool{}
	state.DraftRowIDs = map[string]bool{}
	state.DeletedRowIDs = map[string]bool{}
}

func dataGridCrudHasUnsaved(state dataGridCrudState) bool {
	return len(state.DirtyRowIDs) > 0 || len(state.DraftRowIDs) > 0 ||
		len(state.DeletedRowIDs) > 0
}

func dataGridCrudRowDeleteEnabled(cfg *DataGridCfg, hasSource bool, caps GridDataCapabilities) bool {
	if !dataGridCrudEnabled(cfg) || !boolDefault(cfg.AllowDelete, true) {
		return false
	}
	if !hasSource {
		return true
	}
	return caps.SupportsDelete
}

// dataGridRowsSignature computes an FNV-1a hash of all row
// IDs and cell values. colIDs is a pre-sorted column list;
// when empty, keys are extracted from the first row.
func dataGridRowsSignature(rows []GridRow, colIDs []string) uint64 {
	if len(rows) == 0 {
		return 0
	}
	h := uint64(gg.Fnv64Offset)
	fallbackKeys := colIDs
	if len(fallbackKeys) == 0 {
		keySet := map[string]bool{}
		for _, row := range rows {
			for key := range row.Cells {
				keySet[key] = true
			}
		}
		fallbackKeys = sortedMapKeys(keySet)
	}
	for idx, row := range rows {
		if idx > 0 {
			h = gg.Fnv64Str(h, dataGridGroupSep)
		}
		rowID := dataGridRowID(row, idx)
		h = gg.Fnv64Str(h, rowID)
		h = gg.Fnv64Str(h, dataGridRecordSep)
		keys := fallbackKeys
		for j, key := range keys {
			if j > 0 {
				h = gg.Fnv64Str(h, dataGridUnitSep)
			}
			h = gg.Fnv64Str(h, key)
			h = gg.Fnv64Byte(h, '=')
			h = gg.Fnv64Str(h, row.Cells[key])
		}
	}
	return h
}

func dataGridRowsIDSignature(rows []GridRow) uint64 {
	if len(rows) == 0 {
		return 0
	}
	h := uint64(gg.Fnv64Offset)
	for idx, row := range rows {
		if idx > 0 {
			h = gg.Fnv64Str(h, dataGridGroupSep)
		}
		h = gg.Fnv64Str(h, dataGridRowID(row, idx))
	}
	return h
}

// dataGridCrudResolveCfg syncs the CRUD working copy with the
// source data. Returns the effective cfg (with working rows)
// and the current crud state.
func dataGridCrudResolveCfg(cfg DataGridCfg, w *gg.Window) (DataGridCfg, dataGridCrudState) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, _ := dgCrud.Get(cfg.ID)

	// Compute signature.
	var signature uint64
	dgSource := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	if srcState, ok := dgSource.Get(cfg.ID); ok {
		signature = srcState.RowsSignature
		state.LocalRowsSignatureValid = false
		state.LocalRowsLen = -1
		state.LocalRowsIDSignature = 0
	} else {
		localLen := len(cfg.Rows)
		localIDSig := dataGridRowsIDSignature(cfg.Rows)
		if state.LocalRowsSignatureValid && state.LocalRowsLen == localLen &&
			state.LocalRowsIDSignature == localIDSig {
			signature = state.SourceSignature
		} else {
			signature = dataGridRowsSignature(cfg.Rows, nil)
			state.LocalRowsSignatureValid = true
			state.LocalRowsLen = localLen
			state.LocalRowsIDSignature = localIDSig
		}
	}

	hasUnsaved := dataGridCrudHasUnsaved(state)
	sourceChanged := state.SourceSignature != 0 && state.SourceSignature != signature
	if (!hasUnsaved && (sourceChanged ||
		len(state.WorkingRows) != len(cfg.Rows))) ||
		(len(state.WorkingRows) == 0 && len(state.CommittedRows) == 0 && len(cfg.Rows) > 0) {
		state.CommittedRows = cloneRows(cfg.Rows)
		state.WorkingRows = cloneRows(cfg.Rows)
		state.SourceSignature = signature
		state.SourceChanged = false
		dataGridCrudClearPendingChanges(&state)
	} else if hasUnsaved && sourceChanged {
		state.SourceChanged = true
	}
	dgCrud.Set(cfg.ID, state)

	loadError := cfg.LoadError
	if state.SaveError != "" {
		loadError = state.SaveError
	}
	out := cfg
	out.Rows = cloneRows(state.WorkingRows)
	out.LoadError = loadError
	out.Loading = cfg.Loading || state.Saving
	return out, state
}

func dataGridCrudToolbarRow(cfg *DataGridCfg, state dataGridCrudState, caps GridDataCapabilities, hasSource bool, focusID uint32) gg.View {
	hasUnsaved := dataGridCrudHasUnsaved(state)
	canCreate := boolDefault(cfg.AllowCreate, true) && (!hasSource || caps.SupportsCreate)
	canDelete := boolDefault(cfg.AllowDelete, true) && (!hasSource || caps.SupportsDelete)
	selectedCount := len(cfg.Selection.SelectedRowIDs)
	gridID := cfg.ID
	columns := cfg.Columns
	selection := cfg.Selection
	onSelectionChange := cfg.OnSelectionChange
	dataSource := cfg.DataSource
	query := cfg.Query
	onCRUDError := cfg.OnCRUDError
	onRowsChange := cfg.OnRowsChange
	onPageChange := cfg.OnPageChange
	pageSize := cfg.PageSize
	pageIndex := cfg.PageIndex
	scrollID := dataGridScrollID(cfg)

	dirtyCount := len(state.DirtyRowIDs)
	draftCount := len(state.DraftRowIDs)
	deleteCount := len(state.DeletedRowIDs)

	var status string
	if state.Saving {
		status = gg.ActiveLocale.StrSaving
	} else if state.SaveError != "" {
		status = gg.ActiveLocale.StrSaveFailed
	} else if hasUnsaved {
		status = fmt.Sprintf("%s %d %s %d %s %d",
			gg.ActiveLocale.StrDraft, draftCount,
			gg.ActiveLocale.StrDirty, dirtyCount,
			gg.ActiveLocale.StrDelete, deleteCount)
		if state.SourceChanged {
			status += " | " + gg.ActiveLocale.StrSourceChanged
		}
	} else {
		status = gg.ActiveLocale.StrClean
	}

	return gg.Row(gg.ContainerCfg{
		Height:      dataGridHeaderHeight(cfg),
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  gg.SomeF(0),
		Padding:     gg.Some(dataGridPagerPadding(cfg)),
		Spacing:     gg.SomeF(6),
		VAlign:      gg.VAlignMiddle,
		Content: []gg.View{
			dataGridIndicatorButton(gg.ActiveLocale.StrAdd, cfg.TextStyleFilter, cfg.ColorHeaderHover,
				!canCreate || state.Saving, 0, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
					dataGridCrudAddRow(gridID, columns, onSelectionChange, focusID,
						scrollID, pageSize, pageIndex, onPageChange, e, w)
				}),
			dataGridIndicatorButton(gg.ActiveLocale.StrDelete, cfg.TextStyleFilter, cfg.ColorHeaderHover,
				!canDelete || selectedCount == 0 || state.Saving, 0, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
					dataGridCrudDeleteSelected(gridID, selection, onSelectionChange,
						focusID, e, w)
				}),
			dataGridIndicatorButton(gg.ActiveLocale.StrSave, cfg.TextStyleFilter, cfg.ColorHeaderHover,
				!hasUnsaved || state.Saving, 0, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
					dataGridCrudSave(dataGridCrudSaveContext{
						gridID:            gridID,
						dataSource:        dataSource,
						query:             query,
						onCRUDError:       onCRUDError,
						onRowsChange:      onRowsChange,
						selection:         selection,
						onSelectionChange: onSelectionChange,
						hasSource:         hasSource,
						caps:              caps,
						focusID:           focusID,
					}, e, w)
				}),
			dataGridIndicatorButton(gg.ActiveLocale.StrCancel, cfg.TextStyleFilter, cfg.ColorHeaderHover,
				(!hasUnsaved && state.SaveError == "") || state.Saving, 0, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
					dataGridCrudCancel(gridID, focusID, e, w)
				}),
			gg.Row(gg.ContainerCfg{
				Sizing:  gg.FillFill,
				Padding: gg.NoPadding,
			}),
			gg.Text(gg.TextCfg{
				Text:      fmt.Sprintf("%s %d", gg.ActiveLocale.StrSelected, selectedCount),
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
			gg.Text(gg.TextCfg{
				Text:      status,
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	})
}

func dataGridCrudToolbarHeight(cfg *DataGridCfg) float32 {
	return dataGridHeaderHeight(cfg)
}

func dataGridCrudDefaultCells(columns []GridColumnCfg) map[string]string {
	cells := make(map[string]string, len(columns))
	for _, col := range columns {
		if col.ID == "" {
			continue
		}
		cells[col.ID] = col.DefaultValue
	}
	return cells
}

func dataGridCrudAddRow(gridID string, columns []GridColumnCfg, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), focusID, scrollID uint32, pageSize, pageIndex int, onPageChange func(int, *gg.Event, *gg.Window), e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, _ := dgCrud.Get(gridID)
	state.NextDraftSeq++
	draftID := fmt.Sprintf("__draft_%s_%d", gridID, state.NextDraftSeq)
	row := GridRow{
		ID:    draftID,
		Cells: dataGridCrudDefaultCells(columns),
	}
	state.WorkingRows = append([]GridRow{row}, state.WorkingRows...)
	if state.DraftRowIDs == nil {
		state.DraftRowIDs = map[string]bool{}
	}
	state.DraftRowIDs[draftID] = true
	if state.DirtyRowIDs == nil {
		state.DirtyRowIDs = map[string]bool{}
	}
	state.DirtyRowIDs[draftID] = true
	state.SaveError = ""
	dgCrud.Set(gridID, state)
	dataGridSetEditingRow(gridID, draftID, w)
	if onSelectionChange != nil {
		next := GridSelection{
			AnchorRowID:    draftID,
			ActiveRowID:    draftID,
			SelectedRowIDs: map[string]bool{draftID: true},
		}
		onSelectionChange(next, e, w)
	}
	if pageSize > 0 && pageIndex > 0 && onPageChange != nil {
		dgPJ := gg.StateMap[string, int](w, nsDgPendingJump, capModerate)
		dgPJ.Set(gridID, 0)
		onPageChange(0, e, w)
	}
	w.ScrollVerticalTo(scrollID, 0)
	if focusID > 0 {
		w.SetIDFocus(focusID)
	}
	e.IsHandled = true
}

func dataGridCrudDeleteSelected(gridID string, selection GridSelection, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), focusID uint32, e *gg.Event, w *gg.Window) {
	if len(selection.SelectedRowIDs) == 0 {
		return
	}
	ids := make([]string, 0, len(selection.SelectedRowIDs))
	for rowID, selected := range selection.SelectedRowIDs {
		if selected && rowID != "" {
			ids = append(ids, rowID)
		}
	}
	dataGridCrudDeleteRows(gridID, selection, onSelectionChange, ids, focusID, e, w)
}

func dataGridCrudDeleteRows(gridID string, selection GridSelection, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), rowIDs []string, focusID uint32, e *gg.Event, w *gg.Window) {
	if len(rowIDs) == 0 {
		return
	}
	deleteIDs := make(map[string]bool, len(rowIDs))
	for _, rowID := range rowIDs {
		id := strings.TrimSpace(rowID)
		if id != "" {
			deleteIDs[id] = true
		}
	}
	if len(deleteIDs) == 0 {
		return
	}
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, _ := dgCrud.Get(gridID)
	kept := make([]GridRow, 0, len(state.WorkingRows))
	for idx, row := range state.WorkingRows {
		rowID := dataGridRowID(row, idx)
		if deleteIDs[rowID] {
			if state.DraftRowIDs[rowID] {
				delete(state.DraftRowIDs, rowID)
			} else {
				if state.DeletedRowIDs == nil {
					state.DeletedRowIDs = map[string]bool{}
				}
				state.DeletedRowIDs[rowID] = true
			}
			delete(state.DirtyRowIDs, rowID)
			continue
		}
		kept = append(kept, row)
	}
	state.WorkingRows = kept
	state.SaveError = ""
	dgCrud.Set(gridID, state)

	editingRow := dataGridEditingRowID(gridID, w)
	if editingRow != "" && deleteIDs[editingRow] {
		dataGridClearEditingRow(gridID, w)
	}
	if onSelectionChange != nil {
		nextSel := dataGridSelectionRemoveIDs(selection, deleteIDs)
		onSelectionChange(nextSel, e, w)
	}
	if focusID > 0 {
		w.SetIDFocus(focusID)
	}
	e.IsHandled = true
}

func dataGridSelectionRemoveIDs(selection GridSelection, removeIDs map[string]bool) GridSelection {
	selected := make(map[string]bool, len(selection.SelectedRowIDs))
	for rowID, value := range selection.SelectedRowIDs {
		if value && !removeIDs[rowID] {
			selected[rowID] = true
		}
	}
	active := selection.ActiveRowID
	anchor := selection.AnchorRowID
	if removeIDs[active] {
		active = ""
	}
	if removeIDs[anchor] {
		anchor = ""
	}
	return GridSelection{
		AnchorRowID:    anchor,
		ActiveRowID:    active,
		SelectedRowIDs: selected,
	}
}

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

func dataGridCrudApplyCellEdit(gridID string, crudEnabled bool, onCellEdit func(GridCellEdit, *gg.Event, *gg.Window), edit GridCellEdit, e *gg.Event, w *gg.Window) {
	if edit.RowID == "" || edit.ColID == "" {
		return
	}
	if crudEnabled {
		dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
		state, _ := dgCrud.Get(gridID)
		for idx, row := range state.WorkingRows {
			if dataGridRowID(row, idx) != edit.RowID {
				continue
			}
			cells := make(map[string]string, len(row.Cells))
			maps.Copy(cells, row.Cells)
			cells[edit.ColID] = edit.Value
			state.WorkingRows[idx] = GridRow{
				ID:    row.ID,
				Cells: cells,
			}
			if state.DirtyRowIDs == nil {
				state.DirtyRowIDs = map[string]bool{}
			}
			state.DirtyRowIDs[edit.RowID] = true
			state.SaveError = ""
			break
		}
		dgCrud.Set(gridID, state)
	}
	if onCellEdit != nil {
		onCellEdit(edit, e, w)
	}
}

func dataGridCrudCancel(gridID string, focusID uint32, e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, _ := dgCrud.Get(gridID)
	state.WorkingRows = cloneRows(state.CommittedRows)
	dataGridCrudClearPendingChanges(&state)
	state.SaveError = ""
	state.Saving = false
	state.SourceChanged = false
	dgCrud.Set(gridID, state)
	dataGridClearEditingRow(gridID, w)
	if focusID > 0 {
		w.SetIDFocus(focusID)
	}
	e.IsHandled = true
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
	focusID           uint32
	caps              GridDataCapabilities
	hasSource         bool
}

func dataGridCrudSave(ctx dataGridCrudSaveContext, e *gg.Event, w *gg.Window) {
	gridID := ctx.gridID
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, _ := dgCrud.Get(gridID)
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

func dataGridCrudApplySaveResult(gridID string, result dataGridCrudMutationResult, snapshotRows []GridRow, onCRUDError func(string, *gg.Event, *gg.Window), onRowsChange func([]GridRow, *gg.Event, *gg.Window), selection GridSelection, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), focusID uint32, w *gg.Window) {
	e := &gg.Event{}
	if result.errMsg != "" {
		dataGridCrudRestoreOnError(gridID, result.errPhase, onCRUDError,
			e, w, snapshotRows, result.errMsg)
		return
	}
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, _ := dgCrud.Get(gridID)
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

func dataGridCrudFinishSave(gridID string, _ map[string]string, rowCount int, onRowsChange func([]GridRow, *gg.Event, *gg.Window), hasSource bool, focusID uint32, e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, _ := dgCrud.Get(gridID)
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
	if focusID > 0 {
		w.SetIDFocus(focusID)
	}
}

func dataGridCrudRestoreOnError(gridID, phase string, onCRUDError func(string, *gg.Event, *gg.Window), e *gg.Event, w *gg.Window, snapshotRows []GridRow, errMsg string) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, _ := dgCrud.Get(gridID)
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
