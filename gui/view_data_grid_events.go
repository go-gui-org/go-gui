package gui

import (
	"fmt"
	"slices"
	"strconv"
)

// --- Quick filter ---

func dataGridQuickFilterRow(cfg *DataGridCfg) View {
	h := dataGridQuickFilterHeight(cfg)
	queryCallback := cfg.OnQueryChange
	query := cfg.Query
	value := query.QuickFilter
	inputID := cfg.ID + ":quick_filter"
	inputFocusID := fnvSum32(inputID)
	matchesText := dataGridQuickFilterMatchesText(cfg)
	clearDisabled := value == "" || queryCallback == nil
	debounce := cfg.QuickFilterDebounce

	dimColor := cfg.TextStyleFilter.Color
	dimColor.A = 140
	placeholderStyle := cfg.TextStyleFilter
	placeholderStyle.Color = dimColor

	return Row(ContainerCfg{
		Height:      h,
		Sizing:      FillFixed,
		Color:       cfg.ColorQuickFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     SomeP(0, cfg.PaddingCell.Get(Padding{}).Right, 0, cfg.PaddingCell.Get(Padding{}).Left),
		Spacing:     SomeF(6),
		VAlign:      VAlignMiddle,
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if inputFocusID > 0 {
				w.SetIDFocus(inputFocusID)
			}
			e.IsHandled = true
		},
		Content: []View{
			Input(InputCfg{
				ID:               inputID,
				IDFocus:          inputFocusID,
				Text:             value,
				Placeholder:      cfg.QuickFilterPlaceholder,
				Sizing:           FillFill,
				Padding:          NoPadding,
				SizeBorder:       SomeF(0),
				Radius:           SomeF(0),
				Color:            cfg.ColorQuickFilter,
				ColorHover:       cfg.ColorQuickFilter,
				ColorBorder:      cfg.ColorBorder,
				TextStyle:        cfg.TextStyleFilter,
				PlaceholderStyle: placeholderStyle,
				OnTextChanged: func(_ *Layout, text string, w *Window) {
					if queryCallback == nil {
						return
					}
					if debounce <= 0 {
						next := GridQueryState{
							Sorts:       append([]GridSort(nil), query.Sorts...),
							Filters:     append([]GridFilter(nil), query.Filters...),
							QuickFilter: text,
						}
						e := &Event{}
						queryCallback(next, e, w)
						return
					}
					sorts := append([]GridSort(nil), query.Sorts...)
					filters := append([]GridFilter(nil), query.Filters...)
					w.AnimationAdd(&Animate{
						AnimID: inputID + ":debounce",
						Delay:  debounce,
						Callback: func(_ *Animate, w *Window) {
							next := GridQueryState{
								Sorts:       sorts,
								Filters:     filters,
								QuickFilter: text,
							}
							e := &Event{}
							queryCallback(next, e, w)
						},
					})
				},
			}),
			Text(TextCfg{
				Text:      matchesText,
				Mode:      TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
			dataGridIndicatorButton(guiLocale.StrClear, cfg.TextStyleFilter, cfg.ColorHeaderHover,
				clearDisabled, 0, func(_ *Layout, e *Event, w *Window) {
					if queryCallback == nil {
						return
					}
					w.AnimationRemove(inputID + ":debounce")
					next := GridQueryState{
						Sorts:       append([]GridSort(nil), query.Sorts...),
						Filters:     append([]GridFilter(nil), query.Filters...),
						QuickFilter: "",
					}
					queryCallback(next, e, w)
					if inputFocusID > 0 {
						w.SetIDFocus(inputFocusID)
					}
					e.IsHandled = true
				}),
		},
	})
}

func dataGridQuickFilterMatchesText(cfg *DataGridCfg) string {
	if cfg.RowCount != nil {
		return localeMatchesFmt(len(cfg.Rows), strconv.Itoa(*cfg.RowCount))
	}
	if dataGridHasSource(cfg) {
		return localeMatchesFmt(len(cfg.Rows), "?")
	}
	return fmt.Sprintf("%s %d", guiLocale.StrMatches, len(cfg.Rows))
}

// --- Column chooser ---

func dataGridColumnChooserRow(cfg *DataGridCfg, isOpen bool, focusID uint32) View {
	onHiddenColumnsChange := cfg.OnHiddenColumnsChange
	hasVisibilityCallback := onHiddenColumnsChange != nil
	chooserLabel := guiLocale.StrColumns + " \u25B6" // ▶
	if isOpen {
		chooserLabel = guiLocale.StrColumns + " \u25BC" // ▼
	}
	rowH := cfg.RowHeight
	if rowH <= 0 {
		rowH = dataGridHeaderHeight(cfg)
	}
	gridID := cfg.ID
	columns := cfg.Columns

	content := make([]View, 0, 2)
	content = append(content, Row(ContainerCfg{
		Height:  rowH,
		Sizing:  FillFixed,
		Padding: cfg.PaddingFilter,
		Spacing: SomeF(6),
		VAlign:  VAlignMiddle,
		Content: []View{
			dataGridIndicatorButton(chooserLabel, cfg.TextStyleFilter, cfg.ColorHeaderHover,
				false, 0, func(_ *Layout, e *Event, w *Window) {
					dataGridToggleColumnChooserOpen(gridID, w)
					if focusID > 0 {
						w.SetIDFocus(focusID)
					}
					e.IsHandled = true
				}),
		},
	}))
	if isOpen {
		options := make([]View, 0, len(columns))
		for _, col := range columns {
			if col.ID == "" {
				continue
			}
			hidden := cfg.HiddenColumnIDs[col.ID]
			colID := col.ID
			options = append(options, Toggle(ToggleCfg{
				ID:       gridID + ":col-chooser:" + col.ID,
				Label:    col.Title,
				Selected: !hidden,
				Disabled: !hasVisibilityCallback,
				OnClick: dataGridMakeColumnChooserOnClick(onHiddenColumnsChange,
					cfg.HiddenColumnIDs, columns, colID, focusID),
			}))
		}
		content = append(content, Row(ContainerCfg{
			Height:      rowH,
			Sizing:      FillFixed,
			Padding:     cfg.PaddingFilter,
			Spacing:     SomeF(8),
			Color:       ColorTransparent,
			ColorBorder: cfg.ColorBorder,
			SizeBorder:  SomeF(0),
			Content:     options,
		}))
	}
	return Column(ContainerCfg{
		Height:      dataGridColumnChooserHeight(cfg, isOpen),
		Sizing:      FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     NoPadding,
		Spacing:     SomeF(0),
		Content:     content,
	})
}

func dataGridMakeColumnChooserOnClick(onHiddenColumnsChange func(map[string]bool, *Event, *Window), hiddenColumnIDs map[string]bool, columns []GridColumnCfg, colID string, focusID uint32) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		if onHiddenColumnsChange == nil {
			return
		}
		nextHidden := dataGridNextHiddenColumns(hiddenColumnIDs, colID, columns)
		onHiddenColumnsChange(nextHidden, e, w)
		if focusID > 0 {
			w.SetIDFocus(focusID)
		}
		e.IsHandled = true
	}
}

func dataGridToggleColumnChooserOpen(gridID string, w *Window) {
	dgCO := StateMap[string, bool](w, nsDgChooserOpen, capModerate)
	isOpen, _ := dgCO.Get(gridID) // ok ignored: false means "not open", toggle correct
	dgCO.Set(gridID, !isOpen)
}

// --- Pager ---

type dataGridPagerContext struct {
	cfg           *DataGridCfg
	dataToDisplay map[int]int
	jumpText      string
	pageIndex     int
	pageCount     int
	pageStart     int
	pageEnd       int
	totalRows     int
	focusID       uint32
	viewportH     float32
	rowHeight     float32
	staticTop     float32
	scrollID      uint32
}

func dataGridPagerRow(cfg *DataGridCfg, focusID uint32, pageIndex, pageCount, pageStart, pageEnd, totalRows int, viewportH, rowHeight, staticTop float32, scrollID uint32, dataToDisplay map[int]int, jumpText string) View {
	return dataGridBuildPagerRow(dataGridPagerContext{
		cfg: cfg, focusID: focusID, pageIndex: pageIndex, pageCount: pageCount,
		pageStart: pageStart, pageEnd: pageEnd, totalRows: totalRows,
		viewportH: viewportH, rowHeight: rowHeight, staticTop: staticTop,
		scrollID: scrollID, dataToDisplay: dataToDisplay, jumpText: jumpText,
	})
}

func dataGridBuildPagerRow(pctx dataGridPagerContext) View {
	cfg := pctx.cfg
	content := dataGridPagerContent(pctx)
	return Row(ContainerCfg{
		Height:      dataGridPagerHeight(cfg),
		Sizing:      FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     Some(dataGridPagerPadding(cfg)),
		Spacing:     SomeF(6),
		VAlign:      VAlignMiddle,
		Content:     content,
	})
}

func dataGridPagerContent(pctx dataGridPagerContext) []View {
	cfg := pctx.cfg
	onPageChange := cfg.OnPageChange
	isFirst := pctx.pageIndex <= 0
	isLast := pctx.pageIndex >= pctx.pageCount-1
	pageText := localePageFmt(pctx.pageIndex+1, pctx.pageCount)
	rowsText := dataGridPagerRowsText(pctx.pageStart, pctx.pageEnd, pctx.totalRows)
	jumpCtx := dataGridJumpContextFromPager(pctx)
	jumpEnabled := dataGridJumpEnabledLocal(len(cfg.Rows), cfg.OnSelectionChange, cfg.OnPageChange, cfg.PageSize, pctx.totalRows)
	jumpInputID := cfg.ID + ":jump"
	jumpFocusID := fnvSum32(jumpInputID)
	prevArrow, nextArrow := dataGridPagerArrows()

	content := make([]View, 0, 9)
	content = append(content, dataGridPagerPrevButton(cfg, onPageChange, pctx.pageIndex, pctx.focusID, isFirst, prevArrow))
	content = append(content, Text(TextCfg{
		Text:      pageText,
		Mode:      TextModeSingleLine,
		TextStyle: cfg.TextStyleFilter,
	}))
	content = append(content, dataGridPagerNextButton(cfg, onPageChange, pctx.pageIndex, pctx.pageCount, pctx.focusID, isLast, nextArrow))
	content = append(content, dataGridPagerSpacer())
	content = append(content, dataGridPagerRowsStatus(cfg, rowsText))
	content = append(content, dataGridPagerJumpLabel(cfg))
	content = append(content, dataGridPagerJumpInput(cfg, jumpInputID, jumpFocusID, pctx.jumpText, jumpEnabled, jumpCtx, pctx.focusID))
	return content
}

func dataGridPagerRowsText(pageStart, pageEnd, totalRows int) string {
	if totalRows == 0 || pageEnd <= pageStart {
		return guiLocale.StrRows + " 0/0"
	}
	return localeRowsFmt(pageStart+1, pageEnd, totalRows)
}

func dataGridPagerArrows() (string, string) {
	prev := "\u25C0" // ◀
	next := "\u25B6" // ▶
	if guiLocale.TextDir == TextDirRTL {
		prev, next = next, prev
	}
	return prev, next
}

func dataGridPagerPrevButton(cfg *DataGridCfg, onPageChange func(int, *Event, *Window), pageIndex int, focusID uint32, isFirst bool, prevArrow string) View {
	return dataGridIndicatorButton(prevArrow, cfg.TextStyleHeader, cfg.ColorHeaderHover,
		onPageChange == nil || isFirst, dataGridHeaderControlWidth+10,
		func(_ *Layout, e *Event, w *Window) {
			if onPageChange == nil {
				return
			}
			next := max(0, pageIndex-1)
			onPageChange(next, e, w)
			if focusID > 0 {
				w.SetIDFocus(focusID)
			}
			e.IsHandled = true
		})
}

func dataGridPagerNextButton(cfg *DataGridCfg, onPageChange func(int, *Event, *Window), pageIndex, pageCount int, focusID uint32, isLast bool, nextArrow string) View {
	return dataGridIndicatorButton(nextArrow, cfg.TextStyleHeader, cfg.ColorHeaderHover,
		onPageChange == nil || isLast, dataGridHeaderControlWidth+10,
		func(_ *Layout, e *Event, w *Window) {
			if onPageChange == nil {
				return
			}
			next := min(pageCount-1, pageIndex+1)
			onPageChange(next, e, w)
			if focusID > 0 {
				w.SetIDFocus(focusID)
			}
			e.IsHandled = true
		})
}

func dataGridPagerSpacer() View {
	return Row(ContainerCfg{Sizing: FillFill, Padding: NoPadding})
}

func dataGridPagerRowsStatus(cfg *DataGridCfg, rowsText string) View {
	return Row(ContainerCfg{
		Sizing:  FitFill,
		Padding: SomeP(0, 6, 0, 0),
		VAlign:  VAlignMiddle,
		Content: []View{
			Text(TextCfg{
				Text:      rowsText,
				Mode:      TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	})
}

func dataGridPagerJumpLabel(cfg *DataGridCfg) View {
	return Text(TextCfg{
		Text:      guiLocale.StrJump,
		Mode:      TextModeSingleLine,
		TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
	})
}

type dataGridJumpContext struct {
	onSelectionChange func(GridSelection, *Event, *Window)
	onPageChange      func(int, *Event, *Window)
	dataToDisplay     map[int]int
	gridID            string
	rows              []GridRow
	pageSize          int
	totalRows         int
	pageIndex         int
	viewportH         float32
	rowHeight         float32
	staticTop         float32
	scrollID          uint32
	focusID           uint32
}

func dataGridJumpContextFromPager(pctx dataGridPagerContext) dataGridJumpContext {
	cfg := pctx.cfg
	return dataGridJumpContext{
		rows:              cfg.Rows,
		onSelectionChange: cfg.OnSelectionChange,
		onPageChange:      cfg.OnPageChange,
		pageSize:          cfg.PageSize,
		totalRows:         pctx.totalRows,
		pageIndex:         pctx.pageIndex,
		viewportH:         pctx.viewportH,
		rowHeight:         pctx.rowHeight,
		staticTop:         pctx.staticTop,
		scrollID:          pctx.scrollID,
		dataToDisplay:     pctx.dataToDisplay,
		gridID:            cfg.ID,
	}
}

func dataGridPagerJumpInput(cfg *DataGridCfg, inputID string, focusID uint32, jumpText string, jumpEnabled bool, jumpCtx dataGridJumpContext, gridFocusID uint32) View {
	return Input(InputCfg{
		ID:          inputID,
		IDFocus:     focusID,
		Text:        jumpText,
		Placeholder: "#",
		Disabled:    !jumpEnabled,
		Width:       dataGridJumpInputWidth,
		Sizing:      FixedFill,
		Padding:     NoPadding,
		SizeBorder:  SomeF(0),
		Radius:      SomeF(0),
		Color:       cfg.ColorFilter,
		ColorHover:  cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		TextStyle:   cfg.TextStyleFilter,
		OnTextChanged: func(_ *Layout, inputText string, w *Window) {
			digits := dataGridJumpDigits(inputText)
			dgJI := StateMap[string, string](w, nsDgJump, capModerate)
			dgJI.Set(jumpCtx.gridID, digits)
			e := &Event{}
			dataGridSubmitLocalJump(jumpCtx, e, w)
		},
		OnEnter: func(_ *Layout, e *Event, w *Window) {
			ctx := jumpCtx
			ctx.focusID = gridFocusID
			dataGridSubmitLocalJump(ctx, e, w)
		},
	})
}

// --- Copy ---

func dataGridMakeOnChar(cfg *DataGridCfg, columns []GridColumnCfg) func(*Layout, *Event, *Window) {
	rows := cfg.Rows
	selection := cfg.Selection
	onCopyRows := cfg.OnCopyRows
	return func(_ *Layout, e *Event, w *Window) {
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
				payload = GridRowsToTSV(columns, selectedRows)
			}
		} else {
			payload = GridRowsToTSV(columns, selectedRows)
		}
		if payload == "" {
			return
		}
		w.SetClipboard(payload)
		e.IsHandled = true
	}
}

func dataGridCharIsCopy(e *Event) bool {
	return (e.Modifiers.Has(ModCtrl) && e.CharCode == 3) ||
		(e.Modifiers.Has(ModSuper) && e.CharCode == 3)
}

func dataGridIsSelectAllShortcut(e *Event) bool {
	return (e.Modifiers.Has(ModCtrl) || e.Modifiers.Has(ModSuper)) && e.KeyCode == KeyA
}

// --- Mouse move tracker ---

func dataGridMakeOnMouseMove(gridID string) func(*Layout, *Event, *Window) {
	return func(layout *Layout, e *Event, w *Window) {
		mouseX := layout.Shape.X + e.MouseX
		mouseY := layout.Shape.Y + e.MouseY
		colID := dataGridHeaderColUnderCursor(layout, gridID, mouseX, mouseY)
		dgHH := StateMap[string, string](w, nsDgHeaderHover, capModerate)
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
	onSelectionChange func(GridSelection, *Event, *Window)
	onRowActivate     func(GridRow, *Event, *Window)
	onPageChange      func(int, *Event, *Window)
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
	editorFocusBase   uint32
	rowHeight         float32
	staticTop         float32
	scrollID          uint32
	multiSelect       bool
	rangeSelect       bool
	editEnabled       bool
	crudEnabled       bool
}

func dataGridMakeOnKeydown(cfg *DataGridCfg, columns []GridColumnCfg, rowHeight, staticTop float32, scrollID uint32, pageIndices []int, frozenTopIDs map[string]bool, dataToDisplay map[int]int) func(*Layout, *Event, *Window) {
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
	return func(_ *Layout, e *Event, w *Window) {
		dataGridOnKeydown(keyCtx, e, w)
	}
}

func dataGridOnKeydown(kc dataGridKeydownContext, e *Event, w *Window) {
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

func dataGridHandleEscapeKey(kc dataGridKeydownContext, e *Event, w *Window) bool {
	if e.Modifiers != 0 || e.KeyCode != KeyEscape {
		return false
	}
	if dataGridEditingRowID(kc.gridID, w) != "" {
		dataGridClearEditingRow(kc.gridID, w)
		e.IsHandled = true
		return true
	}
	if kc.crudEnabled {
		e.IsHandled = true
		dataGridCrudCancel(kc.gridID, 0, e, w)
		return true
	}
	e.IsHandled = true
	return true
}

func dataGridHandleCrudKeys(kc dataGridKeydownContext, e *Event, w *Window) bool {
	if !kc.crudEnabled || e.Modifiers != 0 {
		return false
	}
	switch e.KeyCode {
	case KeyInsert:
		dataGridCrudAddRow(kc.gridID, kc.columns, kc.onSelectionChange, 0, kc.scrollID, kc.pageSize, kc.pageIndex, kc.onPageChange, e, w)
		e.IsHandled = true
		return true
	case KeyDelete:
		dataGridCrudDeleteSelected(kc.gridID, kc.selection, kc.onSelectionChange, 0, e, w)
		e.IsHandled = true
		return true
	}
	return false
}

func dataGridHandleEditStartKey(kc dataGridKeydownContext, e *Event, w *Window) bool {
	if e.Modifiers != 0 || e.KeyCode != KeyF2 {
		return false
	}
	if kc.editEnabled && len(kc.rows) > 0 && kc.firstEditColIdx >= 0 {
		rowIdx := dataGridActiveRowIndex(kc.rows, kc.selection)
		if rowIdx >= 0 && rowIdx < len(kc.rows) {
			rowID := dataGridRowID(kc.rows[rowIdx], rowIdx)
			dataGridSetEditingRow(kc.gridID, rowID, w)
			editorFocusID := dataGridEditorFocusIDFromBase(kc.editorFocusBase, kc.colCount, kc.firstEditColIdx)
			if editorFocusID > 0 {
				w.SetIDFocus(editorFocusID)
			}
			e.IsHandled = true
		}
	}
	return true
}

func dataGridHandlePageShortcut(kc dataGridKeydownContext, e *Event, w *Window) bool {
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

func dataGridHandleSelectAllShortcut(kc dataGridKeydownContext, e *Event, w *Window) bool {
	if !dataGridIsSelectAllShortcut(e) || !kc.multiSelect {
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

func dataGridHandleEnterKey(kc dataGridKeydownContext, e *Event, w *Window) bool {
	if e.KeyCode != KeyEnter {
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

func dataGridHandleRowNavigationKeys(kc dataGridKeydownContext, visibleIndices []int, e *Event, w *Window) {
	isShift := e.Modifiers.Has(ModShift)
	if e.Modifiers != 0 && !isShift {
		return
	}
	currentIdx := dataGridActiveRowIndex(kc.rows, kc.selection)
	currentPos := slices.Index(visibleIndices, currentIdx)
	targetPos := max(currentPos, 0)

	switch e.KeyCode {
	case KeyUp:
		targetPos--
	case KeyDown:
		targetPos++
	case KeyHome:
		targetPos = 0
	case KeyEnd:
		targetPos = len(visibleIndices) - 1
	case KeyPageUp:
		targetPos -= kc.pageRows
	case KeyPageDown:
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

func dataGridSelectionForTargetRow(kc dataGridKeydownContext, targetRowID string, isShift bool, w *Window) GridSelection {
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
	if start >= 0 && end >= start {
		for rowIdx := start; rowIdx <= end; rowIdx++ {
			selected[dataGridRowID(rows[rowIdx], rowIdx)] = true
		}
		return selected
	}
	selected[targetRowID] = true
	return selected
}

// --- Scroll ---

func dataGridScrollRowIntoViewEx(viewportH float32, rowIdx int, rowHeight, staticTop float32, scrollID uint32, w *Window) {
	if viewportH <= 0 || rowHeight <= 0 {
		return
	}
	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	currentNeg, _ := sy.Get(scrollID) // ok ignored: zero offset is correct initial scroll
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

func dataGridNextPageIndexForKey(pageIndex, pageCount int, e *Event) (int, bool) {
	if pageCount <= 1 || pageIndex < 0 || pageIndex >= pageCount {
		return 0, false
	}
	if e.Modifiers == ModAlt {
		switch e.KeyCode {
		case KeyHome:
			return 0, true
		case KeyEnd:
			return pageCount - 1, true
		}
		return 0, false
	}
	if !e.Modifiers.HasAny(ModCtrl, ModSuper) || e.Modifiers.Has(ModAlt) {
		return 0, false
	}
	switch e.KeyCode {
	case KeyPageUp:
		return max(0, pageIndex-1), true
	case KeyPageDown:
		return min(pageCount-1, pageIndex+1), true
	}
	return 0, false
}

// --- Jump ---

func dataGridJumpEnabledLocal(rowsLen int, onSelectionChange func(GridSelection, *Event, *Window), onPageChange func(int, *Event, *Window), pageSize, totalRows int) bool {
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

func dataGridSubmitLocalJump(ctx dataGridJumpContext, e *Event, w *Window) {
	if !dataGridJumpEnabledLocal(len(ctx.rows), ctx.onSelectionChange, ctx.onPageChange, ctx.pageSize, ctx.totalRows) {
		return
	}
	dgJI := StateMap[string, string](w, nsDgJump, capModerate)
	// ok ignored: empty string → parseJumpTarget returns (0, false).
	jumpText, _ := dgJI.Get(ctx.gridID)
	targetIdx, ok := dataGridParseJumpTarget(jumpText, ctx.totalRows)
	if !ok {
		return
	}
	dgJI.Set(ctx.gridID, strconv.Itoa(targetIdx+1))
	dataGridJumpToLocalRow(ctx, targetIdx, e, w)
	if ctx.focusID > 0 {
		w.SetIDFocus(ctx.focusID)
	}
	e.IsHandled = true
}

func dataGridJumpToLocalRow(ctx dataGridJumpContext, targetIdx int, e *Event, w *Window) {
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
			dgPJ := StateMap[string, int](w, nsDgPendingJump, capModerate)
			dgPJ.Set(ctx.gridID, targetIdx)
			ctx.onPageChange(targetPage, e, w)
			return
		}
	}
	dgPJ := StateMap[string, int](w, nsDgPendingJump, capModerate)
	dgPJ.Delete(ctx.gridID)
	displayIdx, ok := ctx.dataToDisplay[targetIdx]
	if !ok || displayIdx < 0 {
		return
	}
	dataGridScrollRowIntoViewEx(ctx.viewportH, displayIdx, ctx.rowHeight, ctx.staticTop, ctx.scrollID, w)
}

func dataGridApplyPendingLocalJumpScroll(cfg *DataGridCfg, viewportH, rowHeight, staticTop float32, scrollID uint32, dataToDisplay map[int]int, w *Window) {
	dgPJ := StateMap[string, int](w, nsDgPendingJump, capModerate)
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
