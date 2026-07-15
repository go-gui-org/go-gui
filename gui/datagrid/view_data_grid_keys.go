package datagrid

import (
	"slices"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- Copy ---

func dataGridMakeOnChar(cfg *DataGridCfg, columns []GridColumnCfg) func(*gg.Layout, *gg.Event, *gg.Window) {
	rows := cfg.Rows
	selection := cfg.Selection
	onCopyRows := cfg.OnCopyRows
	return func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
		if !dataGridCharIsCopy(e) {
			return
		}
		selectedRows := dataGridSelectedRows(rows, selection)
		if len(selectedRows) == 0 {
			return
		}
		var payload string
		if onCopyRows != nil {
			text, ok := onCopyRows(selectedRows, e, w)
			if ok {
				payload = text
			} else {
				payload = gridRowsToTSV(columns, selectedRows)
			}
		} else {
			payload = gridRowsToTSV(columns, selectedRows)
		}
		if payload == "" {
			return
		}
		w.SetClipboard(payload)
		e.IsHandled = true
	}
}

func dataGridCharIsCopy(e *gg.Event) bool {
	return (e.Modifiers.Has(gg.ModCtrl) && e.CharCode == 3) ||
		(e.Modifiers.Has(gg.ModSuper) && e.CharCode == 3)
}

func dataGridIsSelectAllShortcut(e *gg.Event) bool {
	return (e.Modifiers.Has(gg.ModCtrl) || e.Modifiers.Has(gg.ModSuper)) && e.KeyCode == gg.KeyA
}

// --- Mouse move tracker ---

func dataGridMakeOnMouseMove(gridID string) func(*gg.Layout, *gg.Event, *gg.Window) {
	return func(layout *gg.Layout, e *gg.Event, w *gg.Window) {
		mouseX := layout.Shape.X + e.MouseX
		mouseY := layout.Shape.Y + e.MouseY
		colID := dataGridHeaderColUnderCursor(layout, gridID, mouseX, mouseY)
		dgHH := gg.StateMap[string, string](w, nsDgHeaderHover, capModerate)
		if colID == "" {
			dgHH.Delete(gridID)
			return
		}
		dgHH.Set(gridID, colID)
	}
}

// --- Header keyboard handler ---

// --- Main grid keyboard handler ---

type dataGridKeydownContext struct {
	selection         GridSelection
	onSelectionChange func(GridSelection, *gg.Event, *gg.Window)
	onRowActivate     func(GridRow, *gg.Event, *gg.Window)
	onPageChange      func(int, *gg.Event, *gg.Window)
	frozenTopIDs      map[string]bool
	dataToDisplay     map[int]int
	gridID            string
	rows              []GridRow
	columns           []GridColumnCfg
	pageIndices       []int
	pageSize          int
	pageIndex         int
	pageRows          int
	firstEditColIdx   int
	colCount          int
	viewportH         float32
	editorFocusBase   string
	rowHeight         float32
	staticTop         float32
	scrollID          string
	multiSelect       bool
	rangeSelect       bool
	editEnabled       bool
	crudEnabled       bool
}

func dataGridMakeOnKeydown(cfg *DataGridCfg, columns []GridColumnCfg, rowHeight, staticTop float32, scrollID string, pageIndices []int, frozenTopIDs map[string]bool, dataToDisplay map[int]int) func(*gg.Layout, *gg.Event, *gg.Window) {
	keyCtx := dataGridKeydownContext{
		gridID:            cfg.ID,
		rows:              cfg.Rows,
		columns:           columns,
		selection:         cfg.Selection,
		multiSelect:       boolDefault(cfg.MultiSelect, true),
		rangeSelect:       boolDefault(cfg.RangeSelect, true),
		onSelectionChange: cfg.OnSelectionChange,
		onRowActivate:     cfg.OnRowActivate,
		onPageChange:      cfg.OnPageChange,
		editEnabled:       dataGridEditingEnabled(cfg),
		crudEnabled:       dataGridCrudEnabled(cfg),
		pageSize:          cfg.PageSize,
		pageIndex:         cfg.PageIndex,
		viewportH:         dataGridHeight(cfg),
		pageRows:          dataGridPageRows(cfg, rowHeight),
		firstEditColIdx:   dataGridFirstEditableColumnIndex(cfg, columns),
		editorFocusBase:   dataGridCellEditorFocusBaseID(cfg, len(columns)),
		colCount:          len(columns),
		rowHeight:         rowHeight,
		staticTop:         staticTop,
		scrollID:          scrollID,
		pageIndices:       pageIndices,
		frozenTopIDs:      frozenTopIDs,
		dataToDisplay:     dataToDisplay,
	}
	return func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
		dataGridOnKeydown(keyCtx, e, w)
	}
}

func dataGridOnKeydown(kc dataGridKeydownContext, e *gg.Event, w *gg.Window) {
	if dataGridHandleEscapeKey(kc, e, w) {
		return
	}
	if dataGridHandleCrudKeys(kc, e, w) {
		return
	}
	if dataGridHandleEditStartKey(kc, e, w) {
		return
	}
	if dataGridHandlePageShortcut(kc, e, w) {
		return
	}
	if len(kc.rows) == 0 {
		return
	}
	visibleIndices := dataGridVisibleRowIndices(len(kc.rows), kc.pageIndices)
	if len(visibleIndices) == 0 {
		return
	}
	if dataGridHandleSelectAllShortcut(kc, e, w) {
		return
	}
	if dataGridHandleEnterKey(kc, e, w) {
		return
	}
	dataGridHandleRowNavigationKeys(kc, visibleIndices, e, w)
}

func dataGridHandleEscapeKey(kc dataGridKeydownContext, e *gg.Event, w *gg.Window) bool {
	if e.Modifiers != 0 || e.KeyCode != gg.KeyEscape {
		return false
	}
	if dataGridEditingRowID(kc.gridID, w) != "" {
		dataGridClearEditingRow(kc.gridID, w)
		e.IsHandled = true
		return true
	}
	if kc.crudEnabled {
		e.IsHandled = true
		dataGridCrudCancel(kc.gridID, "", e, w)
		return true
	}
	e.IsHandled = true
	return true
}

func dataGridHandleCrudKeys(kc dataGridKeydownContext, e *gg.Event, w *gg.Window) bool {
	if !kc.crudEnabled || e.Modifiers != 0 {
		return false
	}
	switch e.KeyCode {
	case gg.KeyInsert:
		dataGridCrudAddRow(kc.gridID, kc.columns, kc.onSelectionChange, "", kc.scrollID, kc.pageSize, kc.pageIndex, kc.onPageChange, e, w)
		e.IsHandled = true
		return true
	case gg.KeyDelete:
		dataGridCrudDeleteSelected(kc.gridID, kc.selection, kc.onSelectionChange, "", e, w)
		e.IsHandled = true
		return true
	}
	return false
}

func dataGridHandleEditStartKey(kc dataGridKeydownContext, e *gg.Event, w *gg.Window) bool {
	if e.Modifiers != 0 || e.KeyCode != gg.KeyF2 {
		return false
	}
	if kc.editEnabled && len(kc.rows) > 0 && kc.firstEditColIdx >= 0 {
		rowIdx := dataGridActiveRowIndex(kc.rows, kc.selection)
		if rowIdx >= 0 && rowIdx < len(kc.rows) {
			rowID := dataGridRowID(kc.rows[rowIdx], rowIdx)
			dataGridSetEditingRow(kc.gridID, rowID, w)
			editorFocusID := dataGridEditorFocusIDFromBase(kc.editorFocusBase, kc.colCount, kc.firstEditColIdx)
			if editorFocusID != "" {
				w.SetFocus(editorFocusID)
			}
			e.IsHandled = true
		}
	}
	return true
}

func dataGridHandlePageShortcut(kc dataGridKeydownContext, e *gg.Event, w *gg.Window) bool {
	if kc.onPageChange == nil || kc.pageSize <= 0 {
		return false
	}
	_, _, pageIdx, pageCount := dataGridPageBounds(len(kc.rows), kc.pageSize, kc.pageIndex)
	if pageCount <= 1 {
		return false
	}
	nextPageIdx, ok := dataGridNextPageIndexForKey(pageIdx, pageCount, e)
	if !ok {
		return false
	}
	if nextPageIdx != pageIdx {
		kc.onPageChange(nextPageIdx, e, w)
	}
	e.IsHandled = true
	return true
}

func dataGridHandleSelectAllShortcut(kc dataGridKeydownContext, e *gg.Event, w *gg.Window) bool {
	if !dataGridIsSelectAllShortcut(e) || !kc.multiSelect {
		return false
	}
	if len(kc.rows) == 0 {
		return false
	}
	selected := make(map[string]bool, len(kc.rows))
	for rowIdx, rowData := range kc.rows {
		selected[dataGridRowID(rowData, rowIdx)] = true
	}
	nextSelection := GridSelection{
		AnchorRowID:    dataGridRowID(kc.rows[0], 0),
		ActiveRowID:    dataGridRowID(kc.rows[len(kc.rows)-1], len(kc.rows)-1),
		SelectedRowIDs: selected,
	}
	dataGridSetAnchor(kc.gridID, nextSelection.AnchorRowID, w)
	if kc.onSelectionChange != nil {
		kc.onSelectionChange(nextSelection, e, w)
	}
	e.IsHandled = true
	return true
}

func dataGridHandleEnterKey(kc dataGridKeydownContext, e *gg.Event, w *gg.Window) bool {
	if e.KeyCode != gg.KeyEnter {
		return false
	}
	if dataGridEditingRowID(kc.gridID, w) != "" {
		dataGridClearEditingRow(kc.gridID, w)
		e.IsHandled = true
		return true
	}
	if kc.onRowActivate == nil {
		e.IsHandled = true
		return true
	}
	rowIdx := dataGridActiveRowIndex(kc.rows, kc.selection)
	if rowIdx >= 0 && rowIdx < len(kc.rows) {
		kc.onRowActivate(kc.rows[rowIdx], e, w)
		e.IsHandled = true
	}
	return true
}

func dataGridHandleRowNavigationKeys(kc dataGridKeydownContext, visibleIndices []int, e *gg.Event, w *gg.Window) {
	isShift := e.Modifiers.Has(gg.ModShift)
	if e.Modifiers != 0 && !isShift {
		return
	}
	currentIdx := dataGridActiveRowIndex(kc.rows, kc.selection)
	currentPos := slices.Index(visibleIndices, currentIdx)
	targetPos := max(currentPos, 0)

	switch e.KeyCode {
	case gg.KeyUp:
		targetPos--
	case gg.KeyDown:
		targetPos++
	case gg.KeyHome:
		targetPos = 0
	case gg.KeyEnd:
		targetPos = len(visibleIndices) - 1
	case gg.KeyPageUp:
		targetPos -= kc.pageRows
	case gg.KeyPageDown:
		targetPos += kc.pageRows
	default:
		return
	}
	if kc.onSelectionChange == nil {
		e.IsHandled = true
		return
	}
	targetPos = max(0, min(len(visibleIndices)-1, targetPos))
	targetIdx := visibleIndices[targetPos]
	targetRowID := dataGridRowID(kc.rows[targetIdx], targetIdx)
	nextSelection := dataGridSelectionForTargetRow(kc, targetRowID, isShift, w)
	kc.onSelectionChange(nextSelection, e, w)
	if kc.frozenTopIDs[targetRowID] {
		e.IsHandled = true
		return
	}
	displayIdx, ok := kc.dataToDisplay[targetIdx]
	if !ok || displayIdx < 0 {
		e.IsHandled = true
		return
	}
	dataGridScrollRowIntoViewEx(kc.viewportH, displayIdx, kc.rowHeight, kc.staticTop, kc.scrollID, w)
	e.IsHandled = true
}
