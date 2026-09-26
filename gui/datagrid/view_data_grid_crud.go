package datagrid

import (
	"fmt"
	"maps"
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

// dataGridCrudHasUnsavedFor reports unsaved CRUD edits for a grid
// without mutating state. Missing state means nothing to cancel.
func dataGridCrudHasUnsavedFor(gridID string, w *gg.Window) bool {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	state, ok := dgCrud.Get(gridID)
	if !ok {
		return false
	}
	return dataGridCrudHasUnsaved(state)
}

func dataGridCrudRowDeleteEnabled(cfg *DataGridCfg, hasSource bool, caps GridDataCapabilities) bool {
	if !dataGridCrudEnabled(cfg) || !boolDefault(cfg.AllowDelete, true) {
		return false
	}
	if !hasSource {
		return true
	}
	return caps.supportsDelete
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

// dataGridCrudResolveCfg syncs the CRUD working copy with the
// source data. Returns the effective cfg (with working rows)
// and the current crud state.
func dataGridCrudResolveCfg(cfg DataGridCfg, w *gg.Window) (DataGridCfg, dataGridCrudState) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means no CRUD state yet.
	state := dgCrud.GetOr(cfg.ID, dataGridCrudState{})

	// Compute signature. The full cell hash runs every frame on
	// the local-rows path: an ID/length shortcut would miss an
	// app-driven cell edit that keeps IDs and length, leaving
	// the working copy stale.
	var signature uint64
	dgSource := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	if srcState, ok := dgSource.Get(cfg.ID); ok {
		signature = srcState.RowsSignature
	} else {
		signature = dataGridRowsSignature(cfg.Rows, nil)
	}

	hasUnsaved := dataGridCrudHasUnsaved(state)
	sourceChanged := state.SourceSignature != 0 && state.SourceSignature != signature
	if (!hasUnsaved && (sourceChanged ||
		len(state.WorkingRows) != len(cfg.Rows))) ||
		(len(state.WorkingRows) == 0 && len(state.CommittedRows) == 0 && len(cfg.Rows) > 0) {
		state.CommittedRows = cloneRows(cfg.Rows)
		state.WorkingRows = cloneRows(cfg.Rows)
		state.WorkingVersion++
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
	// Publish a memoized copy of the working rows. WorkingVersion
	// bumps on every mutation of the working copy — grid-driven
	// edits and app-input refreshes alike — so a hit means the
	// published rows are still current.
	if state.ClonedRows != nil && state.CloneVersion == state.WorkingVersion {
		out.Rows = state.ClonedRows
	} else {
		out.Rows = cloneRows(state.WorkingRows)
		state.ClonedRows = out.Rows
		state.CloneVersion = state.WorkingVersion
		// Persist the memo: the caller keeps the returned state
		// for the toolbar but never writes it back.
		dgCrud.Set(cfg.ID, state)
	}
	out.LoadError = loadError
	out.Loading = cfg.Loading || state.Saving
	return out, state
}

func dataGridCrudToolbarRow(cfg *DataGridCfg, state dataGridCrudState, caps GridDataCapabilities, hasSource bool, focusID string) gg.View {
	hasUnsaved := dataGridCrudHasUnsaved(state)
	canCreate := boolDefault(cfg.AllowCreate, true) && (!hasSource || caps.supportsCreate)
	canDelete := boolDefault(cfg.AllowDelete, true) && (!hasSource || caps.supportsDelete)
	selectedCount := len(cfg.Selection.SelectedRowIDs)
	gridID := cfg.ID
	columns := cfg.Columns
	selection := cfg.Selection
	onSelectionChange := cfg.OnSelectionChange
	dataSource := cfg.DataSource
	query := cfg.Query
	onCRUDError := cfg.OnCRUDError
	errCue := cfg.sounds.err
	onRowsChange := cfg.OnRowsChange
	onPageChange := cfg.OnPageChange
	pageSize := cfg.PageSize
	pageIndex := cfg.PageIndex
	scrollID := dataGridScrollID(cfg)

	dirtyCount := len(state.DirtyRowIDs)
	draftCount := len(state.DraftRowIDs)
	deleteCount := len(state.DeletedRowIDs)

	// One copy per toolbar: CurrentLocale deep-copies the locale.
	loc := gg.CurrentLocale()
	var status string
	if state.Saving {
		status = loc.StrSaving
	} else if state.SaveError != "" {
		status = loc.StrSaveFailed
	} else if hasUnsaved {
		status = fmt.Sprintf("%s %d %s %d %s %d",
			loc.StrDraft, draftCount,
			loc.StrDirty, dirtyCount,
			loc.StrDelete, deleteCount)
		if state.SourceChanged {
			status += " | " + loc.StrSourceChanged
		}
	} else {
		status = loc.StrClean
	}

	return gg.Row(gg.ContainerCfg{
		Height:      dataGridHeaderHeight(cfg),
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorsRow.Border,
		SizeBorder:  gg.SomeF(0),
		Padding:     dataGridPagerPadding(cfg),
		Spacing:     gg.SomeF(6),
		VAlign:      gg.VAlignMiddle,
		Content: []gg.View{
			dataGridIndicatorButton(gg.ScopeID(gridID, "crud_add"), loc.StrAdd, cfg.TextStyleFilter, cfg.ColorsHeader.Hover,
				!canCreate || state.Saving, 0, cfg.sounds.click, func(ctx gg.EventCtx) {
					dataGridCrudAddRow(gridID, columns, onSelectionChange, focusID,
						scrollID, pageSize, pageIndex, onPageChange, ctx.Event, ctx.Window)
				}),
			dataGridIndicatorButton(gg.ScopeID(gridID, "crud_delete"), loc.StrDelete, cfg.TextStyleFilter, cfg.ColorsHeader.Hover,
				!canDelete || selectedCount == 0 || state.Saving, 0, cfg.sounds.click, func(ctx gg.EventCtx) {
					dataGridCrudDeleteSelected(gridID, selection, onSelectionChange,
						focusID, ctx.Event, ctx.Window)
				}),
			dataGridIndicatorButton(gg.ScopeID(gridID, "crud_save"), loc.StrSave, cfg.TextStyleFilter, cfg.ColorsHeader.Hover,
				!hasUnsaved || state.Saving, 0, cfg.sounds.click, func(ctx gg.EventCtx) {
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
						errCue:            errCue,
					}, ctx.Event, ctx.Window)
				}),
			dataGridIndicatorButton(gg.ScopeID(gridID, "crud_cancel"), loc.StrCancel, cfg.TextStyleFilter, cfg.ColorsHeader.Hover,
				(!hasUnsaved && state.SaveError == "") || state.Saving, 0, cfg.sounds.click, func(ctx gg.EventCtx) {
					dataGridCrudCancel(gridID, focusID, ctx.Event, ctx.Window)
				}),
			gg.Row(gg.ContainerCfg{
				Sizing:  gg.FillFill,
				Padding: gg.NoPadding,
			}),
			gg.Text(gg.TextCfg{
				Text:      fmt.Sprintf("%s %d", loc.StrSelected, selectedCount),
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

func dataGridCrudAddRow(gridID string, columns []GridColumnCfg, onSelectionChange func(GridSelection, gg.EventCtx), focusID string, scrollID string, pageSize, pageIndex int, onPageChange func(int, gg.EventCtx), e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means no rows added yet.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	// The save commits WorkingRows when it finishes. A row added now
	// would be marked saved without ever reaching the source.
	if state.Saving {
		return
	}
	state.NextDraftSeq++
	// A row key, not a scope: it becomes a part of composed row IDs,
	// so the grid ID is flattened first — an effective ID carries
	// ":" separators that a key part must not contain.
	safeGridID := strings.ReplaceAll(gridID, ":", "_")
	draftID := fmt.Sprintf("__draft_%s_%d", safeGridID, state.NextDraftSeq) // ergonomics-audit:id-part
	row := GridRow{
		ID:    draftID,
		Cells: dataGridCrudDefaultCells(columns),
	}
	state.WorkingRows = append([]GridRow{row}, state.WorkingRows...)
	state.WorkingVersion++
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
			anchorRowID:    draftID,
			activeRowID:    draftID,
			SelectedRowIDs: map[string]bool{draftID: true},
		}
		onSelectionChange(next, gg.EventCtx{Layout: nil, Event: e, Window: w})
	}
	if pageSize > 0 && pageIndex > 0 && onPageChange != nil {
		dgPJ := gg.StateMap[string, int](w, nsDgPendingJump, capModerate)
		dgPJ.Set(gridID, 0)
		onPageChange(0, gg.EventCtx{Layout: nil, Event: e, Window: w})
	}
	w.ScrollVerticalTo(scrollID, 0)
	if focusID != "" {
		w.SetFocus(focusID)
	}
	e.IsHandled = true
}

func dataGridCrudDeleteSelected(gridID string, selection GridSelection, onSelectionChange func(GridSelection, gg.EventCtx), focusID string, e *gg.Event, w *gg.Window) {
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

func dataGridCrudDeleteRows(gridID string, selection GridSelection, onSelectionChange func(GridSelection, gg.EventCtx), rowIDs []string, focusID string, e *gg.Event, w *gg.Window) {
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
	// Default zero state: absent entry means nothing to delete.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	// Same as dataGridCrudAddRow: a delete made during a save would
	// be marked saved without ever reaching the source.
	if state.Saving {
		return
	}
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
	state.WorkingVersion++
	state.SaveError = ""
	dgCrud.Set(gridID, state)

	editingRow := dataGridEditingRowID(gridID, w)
	if editingRow != "" && deleteIDs[editingRow] {
		dataGridClearEditingRow(gridID, w)
	}
	if onSelectionChange != nil {
		nextSel := dataGridSelectionRemoveIDs(selection, deleteIDs)
		onSelectionChange(nextSel, gg.EventCtx{Layout: nil, Event: e, Window: w})
	}
	if focusID != "" {
		w.SetFocus(focusID)
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
	active := selection.activeRowID
	anchor := selection.anchorRowID
	if removeIDs[active] {
		active = ""
	}
	if removeIDs[anchor] {
		anchor = ""
	}
	return GridSelection{
		anchorRowID:    anchor,
		activeRowID:    active,
		SelectedRowIDs: selected,
	}
}

func dataGridCrudApplyCellEdit(gridID string, crudEnabled bool, onCellEdit func(GridCellEdit, gg.EventCtx), edit GridCellEdit, e *gg.Event, w *gg.Window) {
	if edit.RowID == "" || edit.ColID == "" {
		return
	}
	// Bound unbounded editor input (e.g. a pasted megabyte) before
	// it fans out into working-copy clones and signature hashes.
	edit.Value = dataGridTruncateRunes(edit.Value, dataGridMaxCellValueLen)
	if crudEnabled {
		dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
		// Default zero state: absent entry means no edit has been applied.
		state := dgCrud.GetOr(gridID, dataGridCrudState{})
		// Refuse the edit while a save runs: the save commits
		// WorkingRows when it finishes and clears the dirty flags, so
		// this edit would be marked saved without reaching the source.
		if state.Saving {
			return
		}
		for idx, row := range state.WorkingRows {
			if dataGridRowID(row, idx) != edit.RowID {
				continue
			}
			cells := make(map[string]string, len(row.Cells))
			maps.Copy(cells, row.Cells)
			cells[edit.ColID] = edit.Value
			// Keep the identity the row was found by. A row without an
			// ID is keyed by a hash of its cells (dataGridRowAutoID), so
			// the edit would give it a new ID: the open editor would
			// lose its row and DirtyRowIDs would name a row that no
			// longer exists. Pinning edit.RowID keeps both valid.
			state.WorkingRows[idx] = GridRow{
				ID:    edit.RowID,
				Cells: cells,
			}
			state.WorkingVersion++
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
		onCellEdit(edit, gg.EventCtx{Layout: nil, Event: e, Window: w})
	}
}

func dataGridCrudCancel(gridID string, focusID string, e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means no pending changes to cancel.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	wasSaving := state.Saving
	if wasSaving {
		// Stop the running save and make its result stale: bumping
		// RequestID makes dataGridCrudApplySaveResult drop it.
		if state.ActiveAbort != nil {
			state.ActiveAbort.Abort()
		}
		state.ActiveAbort = nil
		state.RequestID++
	}
	state.WorkingRows = cloneRows(state.CommittedRows)
	state.WorkingVersion++
	dataGridCrudClearPendingChanges(&state)
	state.SaveError = ""
	state.Saving = false
	state.SourceChanged = false
	dgCrud.Set(gridID, state)
	dataGridClearEditingRow(gridID, w)
	if wasSaving {
		// Part of the save can already be on the server. Fetch again
		// so the grid shows what the source holds.
		dataGridSourceForceRefetch(gridID, w)
	}
	if focusID != "" {
		w.SetFocus(focusID)
	}
	e.IsHandled = true
}
