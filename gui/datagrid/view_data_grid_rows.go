package datagrid

import (
	"maps"
	"strconv"
	"strings"

	gg "github.com/go-gui-org/go-gui/gui"
)

func dataGridGroupHeaderRowView(cfg *DataGridCfg, entry dataGridDisplayRow, rowHeight float32) gg.View {
	depthPad := float32(entry.GroupDepth) * dataGridGroupIndentStep
	label := entry.GroupColTitle + ": " + entry.GroupValue
	if boolDefault(cfg.ShowGroupCounts, true) {
		label += " (" + strconv.Itoa(entry.GroupCount) + ")"
	}
	if entry.AggregateText != "" {
		label += "  " + entry.AggregateText
	}
	pc := cfg.PaddingCell.Or(gg.PaddingNone)
	return gg.Row(gg.ContainerCfg{
		Height:      rowHeight,
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorsRow.Border,
		SizeBorder:  gg.NoBorder,
		Padding:     gg.NewPadding(pc.Top, pc.Right, pc.Bottom, pc.Left+depthPad),
		Spacing:     gg.SpacingPx(-cfg.SizeBorder.Or(0)),
		Content: []gg.View{
			gg.Text(gg.TextCfg{
				Text:      label,
				Mode:      gg.TextModeSingleLine,
				TextStyle: cfg.TextStyleHeader,
			}),
		},
	})
}

func dataGridDetailRowView(dctx dataGridCtx, rowData GridRow, rowIdx int) gg.View {
	cfg := dctx.cfg
	if cfg.DetailRowView == nil {
		return gg.Rectangle(gg.RectangleCfg{
			Height: dctx.RowHeight,
			Sizing: gg.FillFixed,
			Color:  gg.ColorTransparent,
		})
	}
	rowID := dataGridRowID(rowData, rowIdx)
	detailView := cfg.DetailRowView(rowData, dctx.w)
	pc := cfg.PaddingCell.Or(gg.PaddingNone)
	focusID := dctx.focusID
	return gg.Row(gg.ContainerCfg{
		ID:          gg.ScopeID(cfg.ID, "detail", rowID),
		Height:      dctx.RowHeight,
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorBackground,
		ColorBorder: cfg.ColorsRow.Border,
		SizeBorder:  gg.NoBorder,
		Padding:     gg.NewPadding(pc.Top, pc.Right, pc.Bottom, pc.Left+dataGridDetailIndent()),
		Spacing:     gg.SpacingPx(-cfg.SizeBorder.Or(0)),
		Content: []gg.View{
			gg.Row(gg.ContainerCfg{
				Width:       dataGridColumnsTotalWidth(dctx.columns, dctx.columnWidths),
				Sizing:      gg.FixedFill,
				Padding:     gg.NoPadding,
				Color:       gg.ColorTransparent,
				ColorBorder: gg.ColorTransparent,
				SizeBorder:  gg.NoBorder,
				Content:     []gg.View{detailView},
			}),
		},
		OnClick: func(ctx gg.EventCtx) {
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
			ctx.Consume()
		},
	})
}

// dataGridRowClickCtx is the part of a row's OnClick that is the same
// for every row of a frame. dataGridBuild makes it once, so a row's
// closure captures one pointer, the row index and the row ID instead of
// a dozen values, and the editor focus base ID is built once per frame
// instead of once per row.
type dataGridRowClickCtx struct {
	rows              []GridRow
	selection         GridSelection
	gridID            string
	onSelectionChange func(GridSelection, gg.EventCtx)
	editorFocusBase   string
	focusID           string
	columns           []GridColumnCfg
	multiSelect       bool
	rangeSelect       bool
	editEnabled       bool
}

func newDataGridRowClickCtx(dctx dataGridCtx) *dataGridRowClickCtx {
	cfg := dctx.cfg
	return &dataGridRowClickCtx{
		rows:              cfg.Rows,
		selection:         cfg.Selection,
		gridID:            cfg.ID,
		onSelectionChange: cfg.OnSelectionChange,
		editorFocusBase:   dataGridCellEditorFocusBaseID(cfg, len(dctx.columns)),
		focusID:           dctx.focusID,
		columns:           dctx.columns,
		multiSelect:       boolDefault(cfg.MultiSelect, true),
		rangeSelect:       boolDefault(cfg.RangeSelect, true),
		editEnabled:       dataGridEditingEnabled(cfg),
	}
}

func dataGridRowView(dctx dataGridCtx, rowData GridRow, rowIdx int, showDeleteAction bool) gg.View {
	cfg := dctx.cfg
	rc := dctx.rowClick
	if rc == nil {
		rc = newDataGridRowClickCtx(dctx)
	}
	rowID := dataGridRowID(rowData, rowIdx)
	isSelected := cfg.Selection.SelectedRowIDs[rowID]
	isEditingRow := dctx.editingRowID == rowID && rc.editEnabled

	cells := dataGridBuildRowCells(dctx, rowData, rowIdx, isEditingRow)
	if showDeleteAction {
		cells = append(cells, dataGridRowDeleteButton(dctx, rowID))
	}

	// Selection paints the subtle wash, not the full accent slab;
	// focus is the ring, not a second fill (visual-refresh §4.3).
	rowColor := dataGridRowFill(cfg, rowIdx, isSelected, false)
	rowHoverColor := dataGridRowFill(cfg, rowIdx, isSelected, true)

	return gg.Row(gg.ContainerCfg{
		ID:          gg.ScopeID(cfg.ID, "row", rowID),
		Height:      dctx.RowHeight,
		Sizing:      gg.FillFixed,
		Color:       rowColor,
		ColorBorder: cfg.ColorsRow.Border,
		SizeBorder:  gg.NoBorder,
		Padding:     gg.NoPadding,
		Spacing:     gg.SpacingPx(-cfg.SizeBorder.Or(0)),
		// Clicking a row selects it, which is the same activation a
		// button makes — the grid's own click role (issue #467).
		Sound: cfg.sounds.click,
		OnClick: func(ctx gg.EventCtx) {
			dataGridRowClick(rc.rows, rc.selection, rc.gridID, rc.multiSelect,
				rc.rangeSelect, rc.onSelectionChange, rc.editEnabled,
				rc.editorFocusBase, len(rc.columns), rowIdx, rowID, rc.focusID,
				rc.columns, ctx.Event, ctx.Window)
			ctx.Consume()
		},
		// Painted by gui, not an OnHover closure: a pointer move inside
		// the row then needs no rebuild, and the row allocates no hover
		// closure (#977). A selected row reacts to hover too: Pick
		// moves the wash one lightness step (#741). A disabled grid
		// disables the row too, and gui skips hover on it.
		Hover: gg.HoverStyle{
			Color:  rowHoverColor,
			Cursor: gg.CursorPointingHand,
		},
		Content: cells,
	})
}

// dataGridBuildRowCells builds one display cell per column, swapping
// in the editor for editable cells of the editing row.
func dataGridBuildRowCells(dctx dataGridCtx, rowData GridRow, rowIdx int, isEditingRow bool) []gg.View {
	cfg := dctx.cfg
	columns := dctx.columns
	columnWidths := dctx.columnWidths
	focusID := dctx.focusID
	w := dctx.w
	rowID := dataGridRowID(rowData, rowIdx)
	detailEnabled := cfg.DetailRowView != nil
	detailToggleEnabled := cfg.OnDetailExpandedChange != nil
	detailExpanded := dataGridDetailRowExpanded(cfg, rowID)

	cells := make([]gg.View, 0, len(columns)+1)
	// One backing array holds every cell's content (at most a detail
	// toggle and the value), and one string holds every cell's ID, so
	// a row pays two allocations for these instead of two per cell.
	content := make([]gg.View, 2*len(columns))
	cellIDs := newDataGridCellIDs(cfg.ID, rowID, columns)
	for colIdx, col := range columns {
		value := rowData.Cells[col.ID]
		baseTextStyle := cfg.TextStyle
		if col.TextStyle != nil {
			baseTextStyle = *col.TextStyle
		}
		textStyle := baseTextStyle
		cellColor := gg.ColorTransparent
		if cfg.CellFormat != nil {
			cellFormat := cfg.CellFormat(rowData, rowIdx, col, value, w)
			textStyle, cellColor = dataGridResolveCellFormat(baseTextStyle, cellFormat)
		}
		isEditingCell := isEditingRow && col.Editable
		// The full slice expression caps the cell at its own two
		// slots, so an append can never reach the next cell's.
		cellContent := content[2*colIdx : 2*colIdx : 2*colIdx+2]
		if colIdx == 0 && detailEnabled {
			cellContent = append(cellContent, dataGridDetailToggleControl(cfg, rowID, detailExpanded, detailToggleEnabled, focusID))
		}
		if isEditingCell {
			editorFocusID := dataGridCellEditorFocusID(cfg, len(columns), rowIdx, colIdx)
			cellContent = append(cellContent, dataGridCellEditorView(cfg, rowID, rowIdx, col, value, editorFocusID, focusID, w))
		} else {
			cellContent = append(cellContent, gg.Text(gg.TextCfg{
				Text:      value,
				Mode:      gg.TextModeSingleLine,
				TextStyle: textStyle,
			}))
		}

		cellPadding := cfg.PaddingCell
		cellSpacing := float32(4)
		cellHAlign := col.Align
		if isEditingCell {
			cellPadding = gg.NoPadding
			cellSpacing = 0
		}
		if colIdx == 0 && detailEnabled {
			cellHAlign = gg.HAlignStart
		}

		cells = append(cells, gg.Row(gg.ContainerCfg{
			ID:       cellIDs.next(col.ID),
			A11YRole: gg.AccessRoleGridCell,
			Width:    dataGridColumnWidthFor(col, columnWidths),
			Sizing:   gg.FixedFill,
			// A committed value wider than its column would paint over
			// the next cell, so a display cell clips its contents. An
			// editing cell must not: the editor fills the cell exactly
			// (NoPadding above) and the theme draws focus as a glow
			// *outside* the control (applyFocusRingShadow), so a clip
			// here cuts the focus ring off the cell being edited. The
			// editor constrains its own text already.
			Clip:        !isEditingCell,
			Padding:     cellPadding,
			Color:       cellColor,
			ColorBorder: cfg.ColorsRow.Border,
			SizeBorder:  cfg.SizeBorder,
			HAlign:      cellHAlign,
			VAlign:      gg.VAlignMiddle,
			Spacing:     gg.SpacingPx(cellSpacing),
			Content:     cellContent,
		}))
	}
	return cells
}

// dataGridCellIDs hands out the ID of each cell in a row, in column
// order. Each is the string gg.ScopeID(gridID, "cell", rowID, col.ID)
// returns, but all of them are substrings of one string built for the
// row: one allocation per row for the IDs instead of one per cell. The
// substrings keep that string alive only as long as the row's shapes.
type dataGridCellIDs struct {
	all       string
	prefixLen int
	off       int
}

func newDataGridCellIDs(gridID, rowID string, columns []GridColumnCfg) dataGridCellIDs {
	// The prefix is gridID:cell:rowID with ScopeID's rule that an
	// empty part is dropped. It holds at least "cell", so it is never
	// empty, and an empty column ID leaves the cell ID equal to it.
	prefixLen := len("cell")
	for _, part := range [2]string{gridID, rowID} {
		if part != "" {
			prefixLen += len(gg.IDSep) + len(part)
		}
	}
	size := 0
	for _, col := range columns {
		size += prefixLen
		if col.ID != "" {
			size += len(gg.IDSep) + len(col.ID)
		}
	}
	var b strings.Builder
	b.Grow(size)
	for _, col := range columns {
		if gridID != "" {
			b.WriteString(gridID)
			b.WriteString(gg.IDSep)
		}
		b.WriteString("cell")
		if rowID != "" {
			b.WriteString(gg.IDSep)
			b.WriteString(rowID)
		}
		if col.ID != "" {
			b.WriteString(gg.IDSep)
			b.WriteString(col.ID)
		}
	}
	return dataGridCellIDs{all: b.String(), prefixLen: prefixLen}
}

// next returns the ID of the next cell, whose column ID is colID. The
// caller asks for every column of the row, in order.
func (c *dataGridCellIDs) next(colID string) string {
	n := c.prefixLen
	if colID != "" {
		n += len(gg.IDSep) + len(colID)
	}
	id := c.all[c.off : c.off+n]
	c.off += n
	return id
}

// dataGridRowDeleteButton builds the CRUD row-delete button.
func dataGridRowDeleteButton(dctx dataGridCtx, rowID string) gg.View {
	cfg := dctx.cfg
	// Copy what the closure needs: retaining the whole frame
	// context per row would pin the config, columns, widths and
	// window for the button lifetime.
	gridID := cfg.ID
	selection := cfg.Selection
	onSelectionChange := cfg.OnSelectionChange
	focusID := dctx.focusID
	return gg.Button(gg.ButtonCfg{
		ID:         gg.ScopeID(gridID, "row-delete", rowID),
		Width:      dataGridHeaderControlWidth + 10,
		Sizing:     gg.FixedFill,
		Padding:    gg.NoPadding,
		SizeBorder: gg.NoBorder,
		Radius:     gg.NoRadius,
		Color:      gg.ColorTransparent,
		Colors:     gg.ColorSet{Base: gg.ColorTransparent, Hover: cfg.ColorsHeader.Hover, Click: cfg.ColorsHeader.Hover, Focus: gg.ColorTransparent, Border: cfg.ColorsRow.Border, BorderFocus: cfg.ColorsRow.Border},
		// SoundDisabled as well as Sound: a resolved gg.SoundNone
		// reads as "unset" inside gg.ButtonCfg (issue #467).
		Sound:         cfg.sounds.click,
		SoundDisabled: cfg.sounds.click == gg.SoundNone,
		OnClick: func(ctx gg.EventCtx) {
			dataGridCrudDeleteRows(gridID, selection, onSelectionChange, []string{rowID}, focusID, ctx.Event, ctx.Window)
			ctx.Consume()
		},
		Content: []gg.View{
			gg.Text(gg.TextCfg{
				Text:      "\u00D7", // ×
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	})
}

func dataGridResolveCellFormat(base gg.TextStyle, format GridCellFormat) (gg.TextStyle, gg.Color) {
	textStyle := base
	if format.hasTextColor {
		textStyle.Color = format.textColor
	}
	bgColor := gg.ColorTransparent
	if format.hasBGColor {
		bgColor = format.bGColor
	}
	return textStyle, bgColor
}

func dataGridRowClick(rows []GridRow, selection GridSelection, gridID string, multiSelect, rangeSelect bool, onSelectionChange func(GridSelection, gg.EventCtx), editEnabled bool, editorFocusBase string, colCount, rowIdx int, rowID string, focusID string, columns []GridColumnCfg, e *gg.Event, w *gg.Window) {
	if focusID != "" {
		w.SetFocus(focusID)
	}
	if rowIdx < 0 || rowIdx >= len(rows) {
		return
	}
	if onSelectionChange != nil {
		next := dataGridComputeRowSelection(rows, selection, gridID, multiSelect, rangeSelect, rowID, e, w)
		onSelectionChange(next, gg.EventCtx{Layout: nil, Event: e, Window: w})
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

func dataGridComputeRowSelection(rows []GridRow, selection GridSelection, gridID string, multiSelect, rangeSelect bool, rowID string, e *gg.Event, w *gg.Window) GridSelection {
	isShift := e.Modifiers.Has(gg.ModShift)
	isToggle := e.Modifiers.Has(gg.ShortcutModifier())

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
			anchorRowID:    anchor,
			activeRowID:    rowID,
			SelectedRowIDs: selected,
		}
	} else if multiSelect && isToggle {
		selected := dataGridToggleSelectedRowIDs(selection.SelectedRowIDs, rowID)
		dataGridSetAnchor(gridID, rowID, w)
		return GridSelection{
			anchorRowID:    rowID,
			activeRowID:    rowID,
			SelectedRowIDs: selected,
		}
	}
	dataGridSetAnchor(gridID, rowID, w)
	if dataGridSelectionIsSingleRow(selection.SelectedRowIDs, rowID) {
		return GridSelection{
			anchorRowID:    rowID,
			activeRowID:    rowID,
			SelectedRowIDs: selection.SelectedRowIDs,
		}
	}
	return GridSelection{
		anchorRowID:    rowID,
		activeRowID:    rowID,
		SelectedRowIDs: map[string]bool{rowID: true},
	}
}

func dataGridAnchorRowIDEx(selection GridSelection, gridID string, rows []GridRow, w *gg.Window, fallback string) string {
	dgRange := gg.StateMap[string, dataGridRangeState](w, nsDgRange, capModerate)
	if st, ok := dgRange.Get(gridID); ok && st.anchorRowID != "" && dataGridHasRowID(rows, st.anchorRowID) {
		return st.anchorRowID
	}
	if selection.anchorRowID != "" && dataGridHasRowID(rows, selection.anchorRowID) {
		return selection.anchorRowID
	}
	return fallback
}

func dataGridSetAnchor(gridID, rowID string, w *gg.Window) {
	dgRange := gg.StateMap[string, dataGridRangeState](w, nsDgRange, capModerate)
	dgRange.Set(gridID, dataGridRangeState{anchorRowID: rowID})
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

// --- Detail expansion ---

func dataGridDetailToggleControl(cfg *DataGridCfg, rowID string, expanded, enabled bool, focusID string) gg.View {
	label := "\u25B6" // ▶
	if expanded {
		label = "\u25BC" // ▼
	}
	style := dataGridIndicatorTextStyle(cfg.TextStyle)
	if !enabled {
		return gg.Row(gg.ContainerCfg{
			Width:   dataGridHeaderControlWidth,
			Sizing:  gg.FixedFill,
			Padding: gg.NoPadding,
			Content: []gg.View{
				gg.Text(gg.TextCfg{
					Text:      label,
					Mode:      gg.TextModeSingleLine,
					TextStyle: style,
				}),
			},
		})
	}
	onDetailExpandedChange := cfg.OnDetailExpandedChange
	detailExpandedRowIDs := cfg.DetailExpandedRowIDs
	return gg.Button(gg.ButtonCfg{
		ID:            gg.ScopeID(cfg.ID, "detail_toggle", rowID),
		Width:         dataGridHeaderControlWidth,
		Sizing:        gg.FixedFill,
		Padding:       gg.NoPadding,
		SizeBorder:    gg.NoBorder,
		Radius:        gg.NoRadius,
		Color:         gg.ColorTransparent,
		Colors:        gg.ColorSet{Base: gg.ColorTransparent, Hover: cfg.ColorsRow.Hover, Click: cfg.ColorsRow.Hover, Focus: gg.ColorTransparent, Border: gg.ColorTransparent, BorderFocus: gg.ColorTransparent},
		Sound:         cfg.sounds.click,
		SoundDisabled: cfg.sounds.click == gg.SoundNone,
		OnClick: func(ctx gg.EventCtx) {
			if rowID == "" || onDetailExpandedChange == nil {
				// Nothing to toggle: pass the click on
				return
			}
			next := dataGridNextDetailExpandedMap(detailExpandedRowIDs, rowID)
			onDetailExpandedChange(next, gg.EventCtx{Layout: nil, Event: ctx.Event, Window: ctx.Window})
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
			// Expanding the detail is the click; the row beneath must
			// not also select.
			ctx.Consume()
		},
		Content: []gg.View{
			gg.Text(gg.TextCfg{
				Text:      label,
				Mode:      gg.TextModeSingleLine,
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

func dataGridScrollPadding(cfg *DataGridCfg) gg.Padding {
	if cfg.Scrollbar == gg.ScrollbarHidden {
		return gg.PaddingNone
	}
	return gg.NewPadding(0, dataGridScrollGutter(), 0, 0)
}

func dataGridScrollGutter() float32 {
	style := gg.DefaultScrollbarStyle
	return style.Size + style.GapEdge + style.GapEnd
}

// --- Frozen top rows ---

func dataGridFrozenTopZone(cfg *DataGridCfg, rowViews []gg.View, zoneHeight, totalWidth, scrollX float32) gg.View {
	return gg.Row(gg.ContainerCfg{
		Height:      zoneHeight,
		Sizing:      gg.FillFixed,
		Clip:        true,
		Color:       cfg.ColorBackground,
		ColorBorder: cfg.ColorsRow.Border,
		SizeBorder:  gg.NoBorder,
		Padding:     dataGridScrollPadding(cfg),
		Spacing:     gg.NoSpacing,
		Content: []gg.View{
			gg.Column(gg.ContainerCfg{
				X:           scrollX,
				Width:       totalWidth,
				Sizing:      gg.FixedFill,
				Color:       gg.ColorTransparent,
				ColorBorder: gg.ColorTransparent,
				SizeBorder:  gg.NoBorder,
				Padding:     gg.NoPadding,
				Spacing:     gg.NoSpacing,
				Content:     rowViews,
			}),
		},
	})
}

func dataGridFrozenTopViews(dctx dataGridCtx, frozenTopIndices []int, showDeleteAction bool) ([]gg.View, int) {
	cfg := dctx.cfg
	if len(frozenTopIndices) == 0 {
		return nil, 0
	}
	views := make([]gg.View, 0, len(frozenTopIndices)*2)
	displayRows := 0
	for _, rowIdx := range frozenTopIndices {
		if rowIdx < 0 || rowIdx >= len(cfg.Rows) {
			continue
		}
		rowData := cfg.Rows[rowIdx]
		rowID := dataGridRowID(rowData, rowIdx)
		views = append(views, dataGridRowView(dctx, rowData, rowIdx, showDeleteAction))
		displayRows++
		if cfg.DetailRowView != nil && dataGridDetailRowExpanded(cfg, rowID) {
			views = append(views, dataGridDetailRowView(dctx, rowData, rowIdx))
			displayRows++
		}
	}
	return views, displayRows
}

// dataGridFrozenTopDisplayRowCount counts the rows dataGridFrozenTopViews
// emits for frozenTopIndices, detail rows included, without building
// the views.
func dataGridFrozenTopDisplayRowCount(cfg *DataGridCfg, frozenTopIndices []int) int {
	n := 0
	for _, rowIdx := range frozenTopIndices {
		if rowIdx < 0 || rowIdx >= len(cfg.Rows) {
			continue
		}
		n++
		if cfg.DetailRowView != nil &&
			dataGridDetailRowExpanded(cfg, dataGridRowID(cfg.Rows[rowIdx], rowIdx)) {
			n++
		}
	}
	return n
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
