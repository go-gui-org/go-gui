package datagrid

import (
	"strconv"

	gg "github.com/go-gui-org/go-gui/gui"
)

type dataGridJumpContext struct {
	onSelectionChange func(GridSelection, *gg.Event, *gg.Window)
	onPageChange      func(int, *gg.Event, *gg.Window)
	dataToDisplay     map[int]int
	gridID            string
	rows              []GridRow
	pageSize          int
	totalRows         int
	pageIndex         int
	viewportH         float32
	rowHeight         float32
	staticTop         float32
	scrollID          string
	focusID           string
}

// --- Jump ---

func dataGridJumpEnabledLocal(rowsLen int, onSelectionChange func(GridSelection, *gg.Event, *gg.Window), onPageChange func(int, *gg.Event, *gg.Window), pageSize, totalRows int) bool {
	if totalRows <= 0 || rowsLen == 0 {
		return false
	}
	if onSelectionChange == nil {
		return false
	}
	if pageSize > 0 && onPageChange == nil {
		return false
	}
	return true
}

func dataGridJumpDigits(text string) string {
	buf := make([]byte, 0, len(text))
	for i := range len(text) {
		if text[i] >= '0' && text[i] <= '9' {
			buf = append(buf, text[i])
		}
	}
	return string(buf)
}

func dataGridParseJumpTarget(text string, totalRows int) (int, bool) {
	if totalRows <= 0 {
		return 0, false
	}
	digits := dataGridJumpDigits(text)
	if digits == "" {
		return 0, false
	}
	target, err := strconv.Atoi(digits)
	if err != nil || target <= 0 {
		return 0, false
	}
	return max(1, min(totalRows, target)) - 1, true
}

func dataGridSubmitLocalJump(ctx dataGridJumpContext, e *gg.Event, w *gg.Window) {
	if !dataGridJumpEnabledLocal(len(ctx.rows), ctx.onSelectionChange, ctx.onPageChange, ctx.pageSize, ctx.totalRows) {
		return
	}
	dgJI := gg.StateMap[string, string](w, nsDgJump, capModerate)
	// Default "": absent entry means no jump text typed yet.
	jumpText := dgJI.GetOr(ctx.gridID, "")
	targetIdx, ok := dataGridParseJumpTarget(jumpText, ctx.totalRows)
	if !ok {
		return
	}
	dgJI.Set(ctx.gridID, strconv.Itoa(targetIdx+1))
	dataGridJumpToLocalRow(ctx, targetIdx, e, w)
	if ctx.focusID != "" {
		w.SetFocus(ctx.focusID)
	}
	e.IsHandled = true
}

func dataGridJumpToLocalRow(ctx dataGridJumpContext, targetIdx int, e *gg.Event, w *gg.Window) {
	if targetIdx < 0 || targetIdx >= len(ctx.rows) {
		return
	}
	targetRowID := dataGridRowID(ctx.rows[targetIdx], targetIdx)
	if ctx.onSelectionChange != nil {
		next := GridSelection{
			AnchorRowID:    targetRowID,
			ActiveRowID:    targetRowID,
			SelectedRowIDs: map[string]bool{targetRowID: true},
		}
		ctx.onSelectionChange(next, e, w)
		dataGridSetAnchor(ctx.gridID, targetRowID, w)
	}
	if ctx.pageSize > 0 {
		if ctx.onPageChange == nil {
			return
		}
		targetPage := targetIdx / ctx.pageSize
		if targetPage != ctx.pageIndex {
			dgPJ := gg.StateMap[string, int](w, nsDgPendingJump, capModerate)
			dgPJ.Set(ctx.gridID, targetIdx)
			ctx.onPageChange(targetPage, e, w)
			return
		}
	}
	dgPJ := gg.StateMap[string, int](w, nsDgPendingJump, capModerate)
	dgPJ.Delete(ctx.gridID)
	displayIdx, ok := ctx.dataToDisplay[targetIdx]
	if !ok || displayIdx < 0 {
		return
	}
	dataGridScrollRowIntoViewEx(ctx.viewportH, displayIdx, ctx.rowHeight, ctx.staticTop, ctx.scrollID, w)
}

func dataGridApplyPendingLocalJumpScroll(cfg *DataGridCfg, viewportH, rowHeight, staticTop float32, scrollID string, dataToDisplay map[int]int, w *gg.Window) {
	dgPJ := gg.StateMap[string, int](w, nsDgPendingJump, capModerate)
	targetIdx, ok := dgPJ.Get(cfg.ID)
	if !ok {
		return
	}
	if targetIdx < 0 || targetIdx >= len(cfg.Rows) {
		dgPJ.Delete(cfg.ID)
		return
	}
	displayIdx, ok2 := dataToDisplay[targetIdx]
	if !ok2 || displayIdx < 0 {
		dgPJ.Delete(cfg.ID)
		return
	}
	dataGridScrollRowIntoViewEx(viewportH, displayIdx, rowHeight, staticTop, scrollID, w)
	dgPJ.Delete(cfg.ID)
}

// --- Scroll ---

func dataGridScrollRowIntoViewEx(viewportH float32, rowIdx int, rowHeight, staticTop float32, scrollID string, w *gg.Window) {
	if viewportH <= 0 || rowHeight <= 0 {
		return
	}
	// Default 0: absent entry means scroll is at origin.
	currentNeg := w.ScrollY().GetOr(scrollID, 0)
	current := -currentNeg
	rowTop := staticTop + float32(rowIdx)*rowHeight
	rowBottom := rowTop + rowHeight
	next := current
	if rowTop < current {
		next = rowTop
	} else if rowBottom > current+viewportH {
		next = rowBottom - viewportH
	}
	next = max(next, 0)
	w.ScrollVerticalTo(scrollID, -next)
}

// --- Page shortcuts ---

func dataGridNextPageIndexForKey(pageIndex, pageCount int, e *gg.Event) (int, bool) {
	if pageCount <= 1 || pageIndex < 0 || pageIndex >= pageCount {
		return 0, false
	}
	if e.Modifiers == gg.ModAlt {
		switch e.KeyCode {
		case gg.KeyHome:
			return 0, true
		case gg.KeyEnd:
			return pageCount - 1, true
		}
		return 0, false
	}
	if !e.Modifiers.HasAny(gg.ModCtrl, gg.ModSuper) || e.Modifiers.Has(gg.ModAlt) {
		return 0, false
	}
	switch e.KeyCode {
	case gg.KeyPageUp:
		return max(0, pageIndex-1), true
	case gg.KeyPageDown:
		return min(pageCount-1, pageIndex+1), true
	}
	return 0, false
}

func dataGridSelectionForTargetRow(kc dataGridKeydownContext, targetRowID string, isShift bool, w *gg.Window) GridSelection {
	if isShift && kc.multiSelect && kc.rangeSelect {
		anchorRowID := dataGridAnchorRowIDEx(kc.selection, kc.gridID, kc.rows, w, targetRowID)
		start, end := dataGridRangeIndices(kc.rows, anchorRowID, targetRowID)
		selectedRows := dataGridRangeSelectedRows(kc.rows, start, end, targetRowID)
		dataGridSetAnchor(kc.gridID, anchorRowID, w)
		return GridSelection{
			AnchorRowID:    anchorRowID,
			ActiveRowID:    targetRowID,
			SelectedRowIDs: selectedRows,
		}
	}
	dataGridSetAnchor(kc.gridID, targetRowID, w)
	return GridSelection{
		AnchorRowID:    targetRowID,
		ActiveRowID:    targetRowID,
		SelectedRowIDs: map[string]bool{targetRowID: true},
	}
}

func dataGridRangeSelectedRows(rows []GridRow, start, end int, targetRowID string) map[string]bool {
	selected := make(map[string]bool, max(end-start+1, 1))
	if start >= 0 && end >= start && end < len(rows) {
		for rowIdx := start; rowIdx <= end; rowIdx++ {
			selected[dataGridRowID(rows[rowIdx], rowIdx)] = true
		}
		return selected
	}
	selected[targetRowID] = true
	return selected
}
