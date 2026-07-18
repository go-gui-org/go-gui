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
	// Default zero state: absent entry means no CRUD state yet.
	state := dgCrud.GetOr(cfg.ID, dataGridCrudState{})

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

func dataGridCrudToolbarRow(cfg *DataGridCfg, state dataGridCrudState, caps GridDataCapabilities, hasSource bool, focusID string) gg.View {
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

func dataGridCrudAddRow(gridID string, columns []GridColumnCfg, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), focusID string, scrollID string, pageSize, pageIndex int, onPageChange func(int, *gg.Event, *gg.Window), e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means no rows added yet.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
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
	if focusID != "" {
		w.SetFocus(focusID)
	}
	e.IsHandled = true
}

func dataGridCrudDeleteSelected(gridID string, selection GridSelection, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), focusID string, e *gg.Event, w *gg.Window) {
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

func dataGridCrudDeleteRows(gridID string, selection GridSelection, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), rowIDs []string, focusID string, e *gg.Event, w *gg.Window) {
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

func dataGridCrudApplyCellEdit(gridID string, crudEnabled bool, onCellEdit func(GridCellEdit, *gg.Event, *gg.Window), edit GridCellEdit, e *gg.Event, w *gg.Window) {
	if edit.RowID == "" || edit.ColID == "" {
		return
	}
	if crudEnabled {
		dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
		// Default zero state: absent entry means no edit has been applied.
		state := dgCrud.GetOr(gridID, dataGridCrudState{})
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

func dataGridCrudCancel(gridID string, focusID string, e *gg.Event, w *gg.Window) {
	dgCrud := gg.StateMap[string, dataGridCrudState](w, nsDgCrud, capModerate)
	// Default zero state: absent entry means no pending changes to cancel.
	state := dgCrud.GetOr(gridID, dataGridCrudState{})
	state.WorkingRows = cloneRows(state.CommittedRows)
	dataGridCrudClearPendingChanges(&state)
	state.SaveError = ""
	state.Saving = false
	state.SourceChanged = false
	dgCrud.Set(gridID, state)
	dataGridClearEditingRow(gridID, w)
	if focusID != "" {
		w.SetFocus(focusID)
	}
	e.IsHandled = true
}
