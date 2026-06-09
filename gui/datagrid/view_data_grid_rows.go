package datagrid

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	. "github.com/go-gui-org/go-gui/gui"
)

func dataGridGroupHeaderRowView(cfg *DataGridCfg, entry dataGridDisplayRow, rowHeight float32) View {
	depthPad := float32(entry.GroupDepth) * dataGridGroupIndentStep
	label := entry.GroupColTitle + ": " + entry.GroupValue
	if boolDefault(cfg.ShowGroupCounts, true) {
		label += " (" + strconv.Itoa(entry.GroupCount) + ")"
	}
	if entry.AggregateText != "" {
		label += "  " + entry.AggregateText
	}
	pc := cfg.PaddingCell.Get(Padding{})
	return Row(ContainerCfg{
		Height:      rowHeight,
		Sizing:      FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     SomeP(pc.Top, pc.Right, pc.Bottom, pc.Left+depthPad),
		Spacing:     Some(-cfg.SizeBorder.Get(0)),
		Content: []View{
			Text(TextCfg{
				Text:      label,
				Mode:      TextModeSingleLine,
				TextStyle: cfg.TextStyleHeader,
			}),
		},
	})
}

func dataGridDetailRowView(dctx dataGridCtx, rowData GridRow, rowIdx int) View {
	cfg := dctx.cfg
	if cfg.OnDetailRowView == nil {
		return Rectangle(RectangleCfg{
			Height: dctx.rowHeight,
			Sizing: FillFixed,
			Color:  ColorTransparent,
		})
	}
	rowID := dataGridRowID(rowData, rowIdx)
	detailView := cfg.OnDetailRowView(rowData, dctx.w)
	pc := cfg.PaddingCell.Get(Padding{})
	focusID := dctx.focusID
	return Row(ContainerCfg{
		ID:          cfg.ID + ":detail:" + rowID,
		Height:      dctx.rowHeight,
		Sizing:      FillFixed,
		Color:       cfg.ColorBackground,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     SomeP(pc.Top, pc.Right, pc.Bottom, pc.Left+dataGridDetailIndent()),
		Spacing:     Some(-cfg.SizeBorder.Get(0)),
		Content: []View{
			Row(ContainerCfg{
				Width:       dataGridColumnsTotalWidth(dctx.columns, dctx.columnWidths),
				Sizing:      FixedFill,
				Padding:     NoPadding,
				Color:       ColorTransparent,
				ColorBorder: ColorTransparent,
				SizeBorder:  SomeF(0),
				Content:     []View{detailView},
			}),
		},
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if focusID > 0 {
				w.SetIDFocus(focusID)
			}
			e.IsHandled = true
		},
	})
}

func dataGridRowView(dctx dataGridCtx, rowData GridRow, rowIdx int, showDeleteAction bool) View {
	cfg := dctx.cfg
	columns := dctx.columns
	columnWidths := dctx.columnWidths
	rowHeight := dctx.rowHeight
	focusID := dctx.focusID
	w := dctx.w
	rowID := dataGridRowID(rowData, rowIdx)
	isSelected := cfg.Selection.SelectedRowIDs[rowID]
	gridID := cfg.ID
	selection := cfg.Selection
	onSelectionChange := cfg.OnSelectionChange
	rows := cfg.Rows
	multiSelect := boolDefault(cfg.MultiSelect, true)
	rangeSelect := boolDefault(cfg.RangeSelect, true)
	editEnabled := dataGridEditingEnabled(cfg)
	editorFocusBase := dataGridCellEditorFocusBaseID(cfg, len(columns))
	colCount := len(columns)
	detailEnabled := cfg.OnDetailRowView != nil
	detailToggleEnabled := cfg.OnDetailExpandedChange != nil
	detailExpanded := dataGridDetailRowExpanded(cfg, rowID)
	isEditingRow := dctx.editingRowID == rowID && editEnabled

	cells := make([]View, 0, len(columns)+1)
	for colIdx, col := range columns {
		value := rowData.Cells[col.ID]
		baseTextStyle := cfg.TextStyle
		if col.TextStyle != nil {
			baseTextStyle = *col.TextStyle
		}
		textStyle := baseTextStyle
		cellColor := ColorTransparent
		if cfg.OnCellFormat != nil {
			cellFormat := cfg.OnCellFormat(rowData, rowIdx, col, value, w)
			textStyle, cellColor = dataGridResolveCellFormat(baseTextStyle, cellFormat)
		}
		isEditingCell := isEditingRow && col.Editable
		var cellBuf [2]View
		cellContent := cellBuf[:0]
		if colIdx == 0 && detailEnabled {
			cellContent = append(cellContent, dataGridDetailToggleControl(cfg, rowID, detailExpanded, detailToggleEnabled, focusID))
		}
		if isEditingCell {
			editorFocusID := dataGridCellEditorFocusID(cfg, len(columns), rowIdx, colIdx)
			cellContent = append(cellContent, dataGridCellEditorView(cfg, rowID, rowIdx, col, value, editorFocusID, focusID, w))
		} else {
			cellContent = append(cellContent, Text(TextCfg{
				Text:      value,
				Mode:      TextModeSingleLine,
				TextStyle: textStyle,
			}))
		}

		cellPadding := cfg.PaddingCell
		cellSpacing := float32(4)
		cellHAlign := col.Align
		if isEditingCell {
			cellPadding = NoPadding
			cellSpacing = 0
		}
		if colIdx == 0 && detailEnabled {
			cellHAlign = HAlignStart
		}

		cells = append(cells, Row(ContainerCfg{
			ID:          cfg.ID + ":cell:" + rowID + ":" + col.ID,
			A11YRole:    AccessRoleGridCell,
			Width:       dataGridColumnWidthFor(col, columnWidths),
			Sizing:      FixedFill,
			Padding:     cellPadding,
			Color:       cellColor,
			ColorBorder: cfg.ColorBorder,
			SizeBorder:  cfg.SizeBorder,
			HAlign:      cellHAlign,
			VAlign:      VAlignMiddle,
			Spacing:     Some(cellSpacing),
			Content:     cellContent,
		}))
	}

	if showDeleteAction {
		cells = append(cells, Button(ButtonCfg{
			ID:          cfg.ID + ":row-delete:" + rowID,
			Width:       dataGridHeaderControlWidth + 10,
			Sizing:      FixedFill,
			Padding:     NoPadding,
			SizeBorder:  SomeF(0),
			Radius:      SomeF(0),
			Color:       ColorTransparent,
			ColorHover:  cfg.ColorHeaderHover,
			ColorFocus:  ColorTransparent,
			ColorClick:  cfg.ColorHeaderHover,
			ColorBorder: cfg.ColorBorder,
			OnClick: func(_ *Layout, e *Event, w *Window) {
				dataGridCrudDeleteRows(gridID, selection, onSelectionChange, []string{rowID}, focusID, e, w)
			},
			Content: []View{
				Text(TextCfg{
					Text:      "\u00D7", // ×
					Mode:      TextModeSingleLine,
					TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
				}),
			},
		}))
	}

	rowColor := ColorTransparent
	if isSelected {
		rowColor = cfg.ColorRowSelected
	} else if rowIdx%2 == 1 {
		rowColor = cfg.ColorRowAlt
	}
	colorRowHover := cfg.ColorRowHover
	disabled := cfg.Disabled

	return Row(ContainerCfg{
		ID:          cfg.ID + ":row:" + rowID,
		Height:      rowHeight,
		Sizing:      FillFixed,
		Color:       rowColor,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     NoPadding,
		Spacing:     Some(-cfg.SizeBorder.Get(0)),
		OnClick: func(_ *Layout, e *Event, w *Window) {
			dataGridRowClick(rows, selection, gridID, multiSelect, rangeSelect,
				onSelectionChange, editEnabled, editorFocusBase, colCount,
				rowIdx, rowID, focusID, columns, e, w)
		},
		OnHover: func(layout *Layout, _ *Event, win *Window) {
			if disabled {
				return
			}
			win.SetMouseCursorPointingHand()
			if !isSelected {
				layout.Shape.Color = colorRowHover
			}
		},
		Content: cells,
	})
}

func dataGridResolveCellFormat(base TextStyle, format GridCellFormat) (TextStyle, Color) {
	textStyle := base
	if format.HasTextColor {
		textStyle.Color = format.TextColor
	}
	bgColor := ColorTransparent
	if format.HasBGColor {
		bgColor = format.BGColor
	}
	return textStyle, bgColor
}

func dataGridRowClick(rows []GridRow, selection GridSelection, gridID string, multiSelect, rangeSelect bool, onSelectionChange func(GridSelection, *Event, *Window), editEnabled bool, editorFocusBase uint32, colCount, rowIdx int, rowID string, focusID uint32, columns []GridColumnCfg, e *Event, w *Window) {
	if focusID > 0 {
		w.SetIDFocus(focusID)
	}
	if rowIdx < 0 || rowIdx >= len(rows) {
		return
	}
	if onSelectionChange != nil {
		next := dataGridComputeRowSelection(rows, selection, gridID, multiSelect, rangeSelect, rowID, e, w)
		onSelectionChange(next, e, w)
	}
	dataGridTrackRowEditClick(gridID, editEnabled, editorFocusBase, colCount, columns, rowIdx, rowID, focusID, e, w)
	e.IsHandled = true
}

func dataGridToggleSelectedRowIDs(selectedRowIDs map[string]bool, rowID string) map[string]bool {
	next := make(map[string]bool, len(selectedRowIDs)+1)
	if selectedRowIDs[rowID] {
		for id, enabled := range selectedRowIDs {
			if id != rowID && enabled {
				next[id] = true
			}
		}
		return next
	}
	for id, enabled := range selectedRowIDs {
		if enabled {
			next[id] = true
		}
	}
	next[rowID] = true
	return next
}

func dataGridSelectionIsSingleRow(selectedRowIDs map[string]bool, rowID string) bool {
	return rowID != "" && len(selectedRowIDs) == 1 && selectedRowIDs[rowID]
}

func dataGridComputeRowSelection(rows []GridRow, selection GridSelection, gridID string, multiSelect, rangeSelect bool, rowID string, e *Event, w *Window) GridSelection {
	isShift := e.Modifiers.Has(ModShift)
	isToggle := e.Modifiers.Has(ModCtrl) || e.Modifiers.Has(ModSuper)

	if multiSelect && rangeSelect && isShift {
		anchor := dataGridAnchorRowIDEx(selection, gridID, rows, w, rowID)
		start, end := dataGridRangeIndices(rows, anchor, rowID)
		selected := make(map[string]bool, max(end-start+1, 1))
		if start >= 0 && end >= start {
			for idx := start; idx <= end; idx++ {
				selected[dataGridRowID(rows[idx], idx)] = true
			}
		} else {
			selected[rowID] = true
		}
		dataGridSetAnchor(gridID, anchor, w)
		return GridSelection{
			AnchorRowID:    anchor,
			ActiveRowID:    rowID,
			SelectedRowIDs: selected,
		}
	} else if multiSelect && isToggle {
		selected := dataGridToggleSelectedRowIDs(selection.SelectedRowIDs, rowID)
		dataGridSetAnchor(gridID, rowID, w)
		return GridSelection{
			AnchorRowID:    rowID,
			ActiveRowID:    rowID,
			SelectedRowIDs: selected,
		}
	}
	dataGridSetAnchor(gridID, rowID, w)
	if dataGridSelectionIsSingleRow(selection.SelectedRowIDs, rowID) {
		return GridSelection{
			AnchorRowID:    rowID,
			ActiveRowID:    rowID,
			SelectedRowIDs: selection.SelectedRowIDs,
		}
	}
	return GridSelection{
		AnchorRowID:    rowID,
		ActiveRowID:    rowID,
		SelectedRowIDs: map[string]bool{rowID: true},
	}
}

func dataGridAnchorRowIDEx(selection GridSelection, gridID string, rows []GridRow, w *Window, fallback string) string {
	dgRange := StateMap[string, dataGridRangeState](w, nsDgRange, capModerate)
	if st, ok := dgRange.Get(gridID); ok && st.AnchorRowID != "" && dataGridHasRowID(rows, st.AnchorRowID) {
		return st.AnchorRowID
	}
	if selection.AnchorRowID != "" && dataGridHasRowID(rows, selection.AnchorRowID) {
		return selection.AnchorRowID
	}
	return fallback
}

func dataGridSetAnchor(gridID, rowID string, w *Window) {
	dgRange := StateMap[string, dataGridRangeState](w, nsDgRange, capModerate)
	dgRange.Set(gridID, dataGridRangeState{AnchorRowID: rowID})
}

func dataGridRangeIndices(rows []GridRow, anchorID, targetID string) (int, int) {
	anchorIdx := -1
	targetIdx := -1
	for idx, row := range rows {
		id := dataGridRowID(row, idx)
		if id == anchorID {
			anchorIdx = idx
		}
		if id == targetID {
			targetIdx = idx
		}
		if anchorIdx >= 0 && targetIdx >= 0 {
			break
		}
	}
	if anchorIdx < 0 || targetIdx < 0 {
		return -1, -1
	}
	if anchorIdx <= targetIdx {
		return anchorIdx, targetIdx
	}
	return targetIdx, anchorIdx
}

// --- Cell editors ---

func dataGridCellEditorView(cfg *DataGridCfg, rowID string, rowIdx int, col GridColumnCfg, value string, editorFocusID, gridFocusID uint32, _ *Window) View {
	editorID := cfg.ID + ":editor:" + rowID + ":" + col.ID
	colID := col.ID
	gridID := cfg.ID
	crudEnabled := dataGridCrudEnabled(cfg)
	onCellEdit := cfg.OnCellEdit

	var editor View
	switch col.Editor {
	case GridCellEditorSelect:
		options := make([]string, len(col.EditorOptions))
		copy(options, col.EditorOptions)
		if len(options) == 0 && value != "" {
			options = []string{value}
		}
		var selectVal []string
		if value != "" {
			selectVal = []string{value}
		}
		editor = Select(SelectCfg{
			ID:         editorID,
			IDFocus:    editorFocusID,
			Selected:   selectVal,
			Options:    options,
			Sizing:     FillFill,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Radius:     SomeF(0),
			OnSelect: func(selected []string, e *Event, w *Window) {
				nextValue := ""
				if len(selected) > 0 {
					nextValue = selected[0]
				}
				if rowID != "" && colID != "" {
					dataGridCrudApplyCellEdit(gridID, crudEnabled, onCellEdit, GridCellEdit{
						RowID:  rowID,
						RowIdx: rowIdx,
						ColID:  colID,
						Value:  nextValue,
					}, e, w)
				}
			},
		})
	case GridCellEditorDate:
		date := dataGridParseEditorDate(value)
		editor = InputDate(InputDateCfg{
			ID:      editorID,
			IDFocus: editorFocusID,
			Date:    date,
			Sizing:  FillFill,
			Padding: NoPadding,
			OnSelect: func(dates []time.Time, e *Event, w *Window) {
				if len(dates) == 0 {
					return
				}
				nextValue := dates[0].Format("1/2/2006")
				if rowID != "" && colID != "" {
					dataGridCrudApplyCellEdit(gridID, crudEnabled, onCellEdit, GridCellEdit{
						RowID:  rowID,
						RowIdx: rowIdx,
						ColID:  colID,
						Value:  nextValue,
					}, e, w)
				}
			},
		})
	case GridCellEditorCheckbox:
		checked := dataGridEditorBoolValue(value)
		editorTrueValue := col.EditorTrueValue
		editorFalseValue := col.EditorFalseValue
		editor = Toggle(ToggleCfg{
			ID:       editorID,
			IDFocus:  editorFocusID,
			Selected: checked,
			Padding:  NoPadding,
			OnClick: func(_ *Layout, e *Event, w *Window) {
				nextValue := editorFalseValue
				if !checked {
					nextValue = editorTrueValue
				}
				if rowID != "" && colID != "" {
					dataGridCrudApplyCellEdit(gridID, crudEnabled, onCellEdit, GridCellEdit{
						RowID:  rowID,
						RowIdx: rowIdx,
						ColID:  colID,
						Value:  nextValue,
					}, e, w)
				}
				e.IsHandled = true
			},
		})
	default: // GridCellEditorText
		editor = Input(InputCfg{
			ID:         editorID,
			IDFocus:    editorFocusID,
			Text:       value,
			Sizing:     FillFill,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Radius:     SomeF(0),
			OnTextChanged: func(_ *Layout, text string, w *Window) {
				if rowID != "" && colID != "" {
					e := &Event{}
					dataGridCrudApplyCellEdit(gridID, crudEnabled, onCellEdit, GridCellEdit{
						RowID:  rowID,
						RowIdx: rowIdx,
						ColID:  colID,
						Value:  text,
					}, e, w)
				}
			},
			OnEnter: func(_ *Layout, e *Event, w *Window) {
				dataGridClearEditingRow(gridID, w)
				if gridFocusID > 0 {
					w.SetIDFocus(gridFocusID)
				}
				e.IsHandled = true
			},
		})
	}

	return Row(ContainerCfg{
		ID:        editorID + ":wrap",
		IDFocus:   editorFocusID,
		FocusSkip: true,
		Sizing:    FillFill,
		Padding:   NoPadding,
		Spacing:   SomeF(0),
		OnKeyDown: dataGridMakeEditorOnKeydown(cfg.ID, gridFocusID),
		Content:   []View{editor},
	})
}

func dataGridMakeEditorOnKeydown(gridID string, gridFocusID uint32) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		if e.Modifiers != 0 || e.KeyCode != KeyEscape {
			return
		}
		dataGridClearEditingRow(gridID, w)
		if gridFocusID > 0 {
			w.SetIDFocus(gridFocusID)
		}
		e.IsHandled = true
	}
}

func dataGridTrackRowEditClick(gridID string, editEnabled bool, editorFocusBase uint32, colCount int, columns []GridColumnCfg, _ int, rowID string, gridFocusID uint32, e *Event, w *Window) {
	if !editEnabled || dataGridHasKeyboardModifiers(e) {
		return
	}
	firstColIdx := dataGridFirstEditableColumnIndexEx(columns)
	if firstColIdx < 0 {
		return
	}
	dgES := StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	state, _ := dgES.Get(gridID)

	isDoubleClick := state.LastClickRowID == rowID && state.LastClickFrame > 0 &&
		e.FrameCount-state.LastClickFrame <= dataGridEditDoubleClickFrames
	if isDoubleClick {
		state.EditingRowID = rowID
		state.LastClickRowID = ""
		state.LastClickFrame = 0
		dgES.Set(gridID, state)
		editorFocusID := dataGridEditorFocusIDFromBase(editorFocusBase, colCount, firstColIdx)
		if editorFocusID > 0 {
			w.SetIDFocus(editorFocusID)
		} else if gridFocusID > 0 {
			w.SetIDFocus(gridFocusID)
		}
		return
	}
	if state.EditingRowID != "" && state.EditingRowID != rowID {
		state.EditingRowID = ""
	}
	state.LastClickRowID = rowID
	state.LastClickFrame = e.FrameCount
	dgES.Set(gridID, state)
}

// hasAnyModifiers checks if the event has any of the specified keyboard modifiers
func hasAnyModifiers(e *Event, modifiers ...Modifier) bool {
	return slices.ContainsFunc(modifiers, e.Modifiers.Has)
}

func dataGridHasKeyboardModifiers(e *Event) bool {
	return hasAnyModifiers(e, ModShift, ModCtrl, ModAlt, ModSuper)
}

func dataGridFirstEditableColumnIndex(cfg *DataGridCfg, columns []GridColumnCfg) int {
	if !dataGridEditingEnabled(cfg) {
		return -1
	}
	return dataGridFirstEditableColumnIndexEx(columns)
}

func dataGridFirstEditableColumnIndexEx(columns []GridColumnCfg) int {
	for idx, col := range columns {
		if col.Editable {
			return idx
		}
	}
	return -1
}

// dataGridCellEditorFocusBaseID returns the first focus ID for
// editor cells. Header cells occupy [base+1 .. base+col_count];
// editor cells start at base+col_count+1.
func dataGridCellEditorFocusBaseID(cfg *DataGridCfg, colCount int) uint32 {
	if colCount <= 0 {
		return 0
	}
	headerBase := dataGridHeaderFocusBaseID(cfg, colCount)
	if headerBase == 0 {
		return 0
	}
	if headerBase > math.MaxUint32-uint32(colCount) {
		return 0
	}
	return headerBase + uint32(colCount)
}

func dataGridCellEditorFocusID(cfg *DataGridCfg, colCount, rowIdx, colIdx int) uint32 {
	if colCount <= 0 || rowIdx < 0 || colIdx < 0 || colIdx >= colCount {
		return 0
	}
	base := dataGridCellEditorFocusBaseID(cfg, colCount)
	if base == 0 {
		return 0
	}
	cellOffset := uint64(colIdx)
	if cellOffset > uint64(math.MaxUint32-base) {
		return 0
	}
	return base + uint32(cellOffset)
}

func dataGridEditorFocusIDFromBase(base uint32, colCount, colIdx int) uint32 {
	if base == 0 || colCount <= 0 || colIdx < 0 || colIdx >= colCount {
		return 0
	}
	cellOffset := uint64(colIdx)
	if cellOffset > uint64(math.MaxUint32-base) {
		return 0
	}
	return base + uint32(cellOffset)
}

func dataGridEditingRowID(gridID string, w *Window) string {
	dgES := StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	if state, ok := dgES.Get(gridID); ok {
		return state.EditingRowID
	}
	return ""
}

func dataGridSetEditingRow(gridID, rowID string, w *Window) {
	dgES := StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	state, _ := dgES.Get(gridID)
	state.EditingRowID = rowID
	dgES.Set(gridID, state)
}

func dataGridClearEditingRow(gridID string, w *Window) {
	dgES := StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	state, _ := dgES.Get(gridID)
	state.EditingRowID = ""
	dgES.Set(gridID, state)
}

func dataGridEditorBoolValue(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

func dataGridParseEditorDate(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Now()
	}
	for _, layout := range []string{
		"1/2/2006",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC3339,
	} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed
		}
	}
	return time.Now()
}

// --- Detail expansion ---

func dataGridDetailToggleControl(cfg *DataGridCfg, rowID string, expanded, enabled bool, focusID uint32) View {
	label := "\u25B6" // ▶
	if expanded {
		label = "\u25BC" // ▼
	}
	style := dataGridIndicatorTextStyle(cfg.TextStyle)
	if !enabled {
		return Row(ContainerCfg{
			Width:   dataGridHeaderControlWidth,
			Sizing:  FixedFill,
			Padding: NoPadding,
			Content: []View{
				Text(TextCfg{
					Text:      label,
					Mode:      TextModeSingleLine,
					TextStyle: style,
				}),
			},
		})
	}
	onDetailExpandedChange := cfg.OnDetailExpandedChange
	detailExpandedRowIDs := cfg.DetailExpandedRowIDs
	return Button(ButtonCfg{
		ID:          cfg.ID + ":detail_toggle:" + rowID,
		Width:       dataGridHeaderControlWidth,
		Sizing:      FixedFill,
		Padding:     NoPadding,
		SizeBorder:  SomeF(0),
		Radius:      SomeF(0),
		Color:       ColorTransparent,
		ColorHover:  cfg.ColorRowHover,
		ColorFocus:  ColorTransparent,
		ColorClick:  cfg.ColorRowHover,
		ColorBorder: ColorTransparent,
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if rowID == "" || onDetailExpandedChange == nil {
				return
			}
			next := dataGridNextDetailExpandedMap(detailExpandedRowIDs, rowID)
			onDetailExpandedChange(next, e, w)
			if focusID > 0 {
				w.SetIDFocus(focusID)
			}
			e.IsHandled = true
		},
		Content: []View{
			Text(TextCfg{
				Text:      label,
				Mode:      TextModeSingleLine,
				TextStyle: style,
			}),
		},
	})
}

func dataGridNextDetailExpandedMap(expanded map[string]bool, rowID string) map[string]bool {
	next := make(map[string]bool, len(expanded))
	maps.Copy(next, expanded)
	if rowID == "" {
		return next
	}
	if next[rowID] {
		delete(next, rowID)
	} else {
		next[rowID] = true
	}
	return next
}

func dataGridDetailIndent() float32 {
	return dataGridHeaderControlWidth + dataGridDetailIndentGap
}

// --- Scrollbar helpers ---

func dataGridScrollPadding(cfg *DataGridCfg) Padding {
	if cfg.Scrollbar == ScrollbarHidden {
		return PaddingNone
	}
	return NewPadding(0, dataGridScrollGutter(), 0, 0)
}

func dataGridScrollGutter() float32 {
	style := DefaultScrollbarStyle
	return style.Size + style.GapEdge + style.GapEnd
}

// --- Frozen top rows ---

func dataGridFrozenTopZone(cfg *DataGridCfg, rowViews []View, zoneHeight, totalWidth, scrollX float32) View {
	return Row(ContainerCfg{
		Height:      zoneHeight,
		Sizing:      FillFixed,
		Clip:        true,
		Color:       cfg.ColorBackground,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     Some(dataGridScrollPadding(cfg)),
		Spacing:     SomeF(0),
		Content: []View{
			Column(ContainerCfg{
				X:           scrollX,
				Width:       totalWidth,
				Sizing:      FixedFill,
				Color:       ColorTransparent,
				ColorBorder: ColorTransparent,
				SizeBorder:  SomeF(0),
				Padding:     NoPadding,
				Spacing:     SomeF(0),
				Content:     rowViews,
			}),
		},
	})
}

func dataGridFrozenTopViews(dctx dataGridCtx, frozenTopIndices []int, showDeleteAction bool) ([]View, int) {
	cfg := dctx.cfg
	if len(frozenTopIndices) == 0 {
		return nil, 0
	}
	views := make([]View, 0, len(frozenTopIndices)*2)
	displayRows := 0
	for _, rowIdx := range frozenTopIndices {
		if rowIdx < 0 || rowIdx >= len(cfg.Rows) {
			continue
		}
		rowData := cfg.Rows[rowIdx]
		rowID := dataGridRowID(rowData, rowIdx)
		views = append(views, dataGridRowView(dctx, rowData, rowIdx, showDeleteAction))
		displayRows++
		if cfg.OnDetailRowView != nil && dataGridDetailRowExpanded(cfg, rowID) {
			views = append(views, dataGridDetailRowView(dctx, rowData, rowIdx))
			displayRows++
		}
	}
	return views, displayRows
}

func dataGridFrozenTopIDSet(cfg *DataGridCfg) map[string]bool {
	if len(cfg.FrozenTopRowIDs) == 0 {
		return nil
	}
	out := make(map[string]bool, len(cfg.FrozenTopRowIDs))
	for _, rowID := range cfg.FrozenTopRowIDs {
		trimmed := strings.TrimSpace(rowID)
		if trimmed != "" {
			out[trimmed] = true
		}
	}
	return out
}

func dataGridSplitFrozenTopIndices(cfg *DataGridCfg, rowIndices []int) (frozenTop, body []int) {
	visibleIndices := dataGridVisibleRowIndices(len(cfg.Rows), rowIndices)
	frozenIDs := dataGridFrozenTopIDSet(cfg)
	if len(visibleIndices) == 0 || len(frozenIDs) == 0 {
		return nil, visibleIndices
	}
	frozenTop = make([]int, 0, len(visibleIndices))
	body = make([]int, 0, len(visibleIndices))
	seen := map[string]bool{}
	for _, rowIdx := range visibleIndices {
		if rowIdx < 0 || rowIdx >= len(cfg.Rows) {
			continue
		}
		rowID := dataGridRowID(cfg.Rows[rowIdx], rowIdx)
		if rowID != "" && frozenIDs[rowID] && !seen[rowID] {
			seen[rowID] = true
			frozenTop = append(frozenTop, rowIdx)
			continue
		}
		body = append(body, rowIdx)
	}
	return
}
