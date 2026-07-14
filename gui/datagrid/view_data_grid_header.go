package datagrid

import (
	"fmt"
	"slices"

	gg "github.com/go-gui-org/go-gui/gui"
)

// dataGridHeaderRow builds the header row with all column
// header cells.
func dataGridHeaderRow(cfg *DataGridCfg, columns []GridColumnCfg, columnWidths map[string]float32, focusID string, hoveredColID, resizingColID, focusedColID string) gg.View {
	cells := make([]gg.View, 0, len(columns))
	for idx, col := range columns {
		width := dataGridColumnWidthFor(col, columnWidths)
		showControls := dataGridShowHeaderControls(col.ID, hoveredColID, resizingColID, focusedColID)
		cells = append(cells, dataGridHeaderCell(cfg, col, idx, len(columns), width, focusID, showControls))
	}
	return gg.Row(gg.ContainerCfg{
		Height:      dataGridHeaderHeight(cfg),
		Sizing:      gg.FillFixed,
		Color:       gg.ColorTransparent,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  gg.SomeF(0),
		Padding:     gg.NoPadding,
		Spacing:     gg.Some(-cfg.SizeBorder.Get(0)),
		Content:     cells,
	})
}

func dataGridHeaderCell(cfg *DataGridCfg, col GridColumnCfg, colIdx, colCount int, width float32, focusID string, showControls bool) gg.View {
	hasReorder := showControls && cfg.OnColumnOrderChange != nil && col.Reorderable
	hasPin := showControls && cfg.OnColumnPinChange != nil
	headerControls := dataGridHeaderControlState(width, cfg.PaddingHeader.Get(gg.Padding{}), hasReorder, hasPin, showControls && col.Resizable)
	headerFocusID := cfg.ID + ":header:" + col.ID

	content := make([]gg.View, 0, 5)
	indicator := dataGridHeaderIndicator(cfg.Query, col.ID)

	labelContent := make([]gg.View, 0, 2)
	labelContent = append(labelContent, gg.Text(gg.TextCfg{
		Text:      col.Title,
		Mode:      gg.TextModeSingleLine,
		TextStyle: cfg.TextStyleHeader,
	}))
	if indicator != "" {
		labelContent = append(labelContent, gg.Text(gg.TextCfg{
			Text:      indicator,
			Mode:      gg.TextModeSingleLine,
			TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleHeader),
		}))
	}

	if headerControls.showLabel {
		content = append(content, gg.Row(gg.ContainerCfg{
			Sizing:  gg.FillFill,
			Clip:    true,
			Padding: gg.NoPadding,
			HAlign:  col.Align,
			VAlign:  gg.VAlignMiddle,
			Spacing: gg.SomeF(6),
			Content: labelContent,
		}))
	} else {
		content = append(content, gg.Row(gg.ContainerCfg{
			Sizing:  gg.FillFill,
			Padding: gg.NoPadding,
		}))
	}
	if headerControls.showReorder {
		content = append(content, dataGridReorderControls(cfg, col))
	}
	if headerControls.showPin {
		content = append(content, dataGridPinControl(cfg, col))
	}
	if headerControls.showResize {
		content = append(content, dataGridResizeHandle(cfg, col, headerFocusID))
	}

	onQueryChange := cfg.OnQueryChange
	query := cfg.Query
	multiSort := boolDefault(cfg.MultiSort, true)
	colSortable := col.Sortable
	colID := col.ID
	colorHeaderHover := cfg.ColorHeaderHover
	headerSorted := dataGridSortIndex(query.Sorts, colID) >= 0
	headerA11YState := gg.AccessStateNone
	if headerSorted {
		headerA11YState = gg.AccessStateSelected
	}

	return gg.Row(gg.ContainerCfg{
		ID:          cfg.ID + ":header:" + col.ID,
		A11YRole:    gg.AccessRoleGridCell,
		A11YLabel:   col.Title,
		A11YState:   headerA11YState,
		Width:       width,
		Sizing:      gg.FixedFill,
		Padding:     cfg.PaddingHeader,
		Clip:        true,
		Color:       cfg.ColorHeader,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Spacing:     gg.SomeF(0),
		OnClick: func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
			e.IsHandled = true
			if colSortable && onQueryChange != nil {
				shiftSort := multiSort && e.Modifiers.Has(gg.ModShift)
				next := dataGridToggleSort(query, colID, multiSort, shiftSort)
				onQueryChange(next, e, w)
			}
			if headerFocusID != "" {
				w.SetFocus(headerFocusID)
			} else if focusID != "" {
				w.SetFocus(focusID)
			}
		},
		OnHover: func(layout *gg.Layout, _ *gg.Event, w *gg.Window) {
			if cfg.Disabled {
				return
			}
			if colSortable {
				w.SetMouseCursorPointingHand()
				layout.Shape.Color = colorHeaderHover
			}
		},
		Focusable: true,
		Content:   content,
	})
}

func dataGridResizeHandle(cfg *DataGridCfg, col GridColumnCfg, focusID string) gg.View {
	gridID := cfg.ID
	columns := cfg.Columns
	rows := cfg.Rows
	textStyleHeader := cfg.TextStyleHeader
	textStyle := cfg.TextStyle
	paddingCell := cfg.PaddingCell.Get(gg.Padding{})
	colorResizeHandle := cfg.ColorResizeHandle
	colorResizeActive := cfg.ColorResizeActive

	disabled := cfg.Disabled

	return gg.Row(gg.ContainerCfg{
		ID:      gridID + ":resize:" + col.ID,
		Width:   dataGridResizeHandleWidth,
		Sizing:  gg.FixedFill,
		Padding: gg.NoPadding,
		Color:   colorResizeHandle,
		OnClick: func(layout *gg.Layout, e *gg.Event, w *gg.Window) {
			if disabled {
				return
			}
			startX := layout.Shape.X + e.MouseX
			dataGridStartResize(gridID, columns, rows, textStyleHeader, textStyle, paddingCell, col, focusID, startX, e, w)
		},
		OnHover: func(layout *gg.Layout, e *gg.Event, w *gg.Window) {
			if disabled {
				return
			}
			w.SetMouseCursorEW()
			if e.MouseButton == gg.MouseLeft {
				layout.Shape.Color = colorResizeActive
			} else {
				layout.Shape.Color = colorResizeHandle
			}
		},
		Content: []gg.View{
			gg.Rectangle(gg.RectangleCfg{
				Width:  1,
				Height: 1,
				Sizing: gg.FillFill,
				Color:  gg.ColorTransparent,
			}),
		},
	})
}

func dataGridReorderControls(cfg *DataGridCfg, col GridColumnCfg) gg.View {
	onColumnOrderChange := cfg.OnColumnOrderChange
	baseOrder, _ := dataGridColumnOrderAndMap(cfg.Columns, cfg.ColumnOrder)
	colID := col.ID
	leftArrow := "\u25C0"  // ◀
	rightArrow := "\u25B6" // ▶
	if gg.ActiveLocale.TextDir == gg.TextDirRTL {
		leftArrow, rightArrow = rightArrow, leftArrow
	}

	reorderCB := func(delta int) func(*gg.Event, *gg.Window) {
		return func(e *gg.Event, w *gg.Window) {
			if onColumnOrderChange == nil {
				e.IsHandled = true
				return
			}
			nextOrder := dataGridColumnOrderMove(baseOrder, colID, delta)
			if len(nextOrder) == len(baseOrder) && slices.Equal(nextOrder, baseOrder) {
				e.IsHandled = true
				return
			}
			onColumnOrderChange(nextOrder, e, w)
			e.IsHandled = true
		}
	}

	return gg.Row(gg.ContainerCfg{
		Padding: gg.NoPadding,
		Spacing: gg.Some(dataGridHeaderReorderSpacing),
		Width:   dataGridHeaderControlsWidth(true, false, false),
		Sizing:  gg.FixedFill,
		Content: []gg.View{
			dataGridOrderButton(leftArrow, cfg.TextStyleHeader, cfg.ColorHeaderHover, reorderCB(-1)),
			dataGridOrderButton(rightArrow, cfg.TextStyleHeader, cfg.ColorHeaderHover, reorderCB(1)),
		},
	})
}

func dataGridOrderButton(label string, baseStyle gg.TextStyle, hoverColor gg.Color, cb func(*gg.Event, *gg.Window)) gg.View {
	return dataGridIndicatorButton(label, baseStyle, hoverColor, false, dataGridHeaderControlWidth,
		func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
			cb(e, w)
		})
}

func dataGridIndicatorButton(label string, baseStyle gg.TextStyle, hoverColor gg.Color, disabled bool, width float32, onClick func(*gg.Layout, *gg.Event, *gg.Window)) gg.View {
	sizing := gg.FitFill
	if width > 0 {
		sizing = gg.FixedFill
	}
	return gg.Button(gg.ButtonCfg{
		Width:       width,
		Sizing:      sizing,
		Padding:     gg.NoPadding,
		SizeBorder:  gg.SomeF(0),
		Radius:      gg.SomeF(0),
		Color:       gg.ColorTransparent,
		ColorHover:  hoverColor,
		ColorFocus:  gg.ColorTransparent,
		ColorClick:  hoverColor,
		ColorBorder: gg.ColorTransparent,
		Disabled:    disabled,
		OnClick:     onClick,
		Content: []gg.View{
			gg.Text(gg.TextCfg{
				Text:      label,
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(baseStyle),
			}),
		},
	})
}

func dataGridPinControl(cfg *DataGridCfg, col GridColumnCfg) gg.View {
	var label string
	switch col.Pin {
	case GridColumnPinNone:
		label = "\u2022" // •
	case GridColumnPinLeft:
		label = "\u21A4" // ↤
	case GridColumnPinRight:
		label = "\u21A6" // ↦
	}
	onColumnPinChange := cfg.OnColumnPinChange
	colID := col.ID
	colPin := col.Pin

	return dataGridIndicatorButton(label, cfg.TextStyleHeader, cfg.ColorHeaderHover,
		false, dataGridHeaderControlWidth, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
			if onColumnPinChange == nil {
				return
			}
			nextPin := dataGridColumnNextPin(colPin)
			onColumnPinChange(colID, nextPin, e, w)
			e.IsHandled = true
		})
}

func dataGridFilterRow(cfg *DataGridCfg, columns []GridColumnCfg, columnWidths map[string]float32) gg.View {
	cells := make([]gg.View, 0, len(columns))
	for _, col := range columns {
		cells = append(cells, dataGridFilterCell(cfg, col, dataGridColumnWidthFor(col, columnWidths)))
	}
	return gg.Row(gg.ContainerCfg{
		Height:      dataGridFilterHeight(cfg),
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  gg.SomeF(0),
		Padding:     cfg.PaddingFilter,
		Spacing:     gg.Some(-cfg.SizeBorder.Get(0)),
		Content:     cells,
	})
}

func dataGridFilterCell(cfg *DataGridCfg, col GridColumnCfg, width float32) gg.View {
	query := cfg.Query
	value := dataGridQueryFilterValue(query, col.ID)
	inputID := cfg.ID + ":filter:" + col.ID
	onQueryChange := cfg.OnQueryChange
	colID := col.ID
	var placeholder string
	if col.Filterable {
		placeholder = gg.ActiveLocale.StrFilter
	}

	return gg.Row(gg.ContainerCfg{
		ID:          cfg.ID + ":filter_cell:" + col.ID,
		Width:       width,
		Sizing:      gg.FixedFill,
		Padding:     cfg.PaddingFilter,
		Color:       gg.ColorTransparent,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Spacing:     gg.SomeF(0),
		Content: []gg.View{
			gg.Input(gg.InputCfg{
				ID:          inputID,
				Focusable:   true,
				Text:        value,
				Placeholder: placeholder,
				Disabled:    !col.Filterable || onQueryChange == nil,
				Sizing:      gg.FillFill,
				Padding:     gg.NoPadding,
				SizeBorder:  gg.SomeF(0),
				Radius:      gg.SomeF(0),
				Color:       cfg.ColorFilter,
				ColorHover:  cfg.ColorFilter,
				ColorBorder: cfg.ColorBorder,
				TextStyle:   cfg.TextStyleFilter,
				OnTextChanged: func(_ *gg.Layout, text string, w *gg.Window) {
					if onQueryChange == nil {
						return
					}
					next := dataGridQuerySetFilter(query, colID, text)
					e := &gg.Event{}
					onQueryChange(next, e, w)
				},
			}),
		},
	})
}

func dataGridStartResize(gridID string, columns []GridColumnCfg, rows []GridRow, textStyleHeader, textStyle gg.TextStyle, paddingCell gg.Padding, col GridColumnCfg, focusID string, startMouseX float32, e *gg.Event, w *gg.Window) {
	if focusID != "" {
		w.SetFocus(focusID)
	}
	dgRS := gg.StateMap[string, dataGridResizeState](w, nsDgResize, capModerate)
	runtime, _ := dgRS.Get(gridID)

	if runtime.LastClickColID == col.ID && runtime.LastClickFrame > 0 &&
		e.FrameCount-runtime.LastClickFrame <= dataGridResizeDoubleClickFrames {
		fitWidth := dataGridAutoFitWidth(rows, textStyleHeader, textStyle, paddingCell, col, w)
		dataGridSetColumnWidth(gridID, col, fitWidth, w)
		runtime.Active = false
		runtime.LastClickFrame = 0
		runtime.LastClickColID = ""
		dgRS.Set(gridID, runtime)
		e.IsHandled = true
		return
	}

	runtime.Active = true
	runtime.ColID = col.ID
	runtime.StartMouseX = startMouseX
	runtime.StartWidth = dataGridColumnWidth(gridID, columns, col, w)
	runtime.LastClickFrame = e.FrameCount
	runtime.LastClickColID = col.ID
	dgRS.Set(gridID, runtime)

	w.MouseLock(gg.MouseLockCfg{
		MouseMove: func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
			dataGridResizeDrag(gridID, col, e, w)
		},
		MouseUp: func(_ *gg.Layout, _ *gg.Event, w *gg.Window) {
			dataGridEndResize(gridID, w)
			w.MouseUnlock()
			if focusID != "" {
				w.SetFocus(focusID)
			}
		},
	})
	e.IsHandled = true
}

func dataGridResizeDrag(gridID string, col GridColumnCfg, e *gg.Event, w *gg.Window) {
	dgRS := gg.StateMap[string, dataGridResizeState](w, nsDgResize, capModerate)
	runtime, ok := dgRS.Get(gridID)
	if !ok || !runtime.Active || runtime.ColID != col.ID {
		return
	}
	delta := e.MouseX - runtime.StartMouseX
	nextWidth := runtime.StartWidth + delta
	dataGridSetColumnWidth(gridID, col, nextWidth, w)
	w.SetMouseCursorEW()
	e.IsHandled = true
}

func dataGridEndResize(gridID string, w *gg.Window) {
	dgRS := gg.StateMap[string, dataGridResizeState](w, nsDgResize, capModerate)
	runtime, ok := dgRS.Get(gridID)
	if !ok {
		return
	}
	runtime.Active = false
	dgRS.Set(gridID, runtime)
}

func dataGridAutoFitWidth(rows []GridRow, textStyleHeader, textStyle gg.TextStyle, paddingCell gg.Padding, col GridColumnCfg, w *gg.Window) float32 {
	if w.TextMeasurer() == nil {
		return dataGridColumnWidthFor(col, nil)
	}
	longest := w.TextMeasurer().TextWidth(col.Title, textStyleHeader)
	style := textStyle
	if col.TextStyle != nil {
		style = *col.TextStyle
	}
	sample := rows
	if len(rows) > dataGridAutofitMaxRows {
		sample = rows[:dataGridAutofitMaxRows]
	}
	for _, row := range sample {
		value := row.Cells[col.ID]
		width := w.TextMeasurer().TextWidth(value, style)
		if width > longest {
			longest = width
		}
	}
	return dataGridClampWidth(col, longest+paddingCell.Width()+dataGridAutofitPadding)
}

func dataGridHeaderIndicator(query GridQueryState, colID string) string {
	idx := dataGridSortIndex(query.Sorts, colID)
	if idx < 0 {
		return ""
	}
	sort := query.Sorts[idx]
	dir := "\u25B2" // ▲
	if sort.Dir != GridSortAsc {
		dir = "\u25BC" // ▼
	}
	if len(query.Sorts) > 1 {
		return fmt.Sprintf("%d%s", idx+1, dir)
	}
	return dir
}

func dataGridActiveResizeColID(gridID string, w *gg.Window) string {
	dgRS := gg.StateMap[string, dataGridResizeState](w, nsDgResize, capModerate)
	if runtime, ok := dgRS.Get(gridID); ok && runtime.Active {
		return runtime.ColID
	}
	return ""
}

func dataGridHeaderFocusedColID(cfg *DataGridCfg, columns []GridColumnCfg, focusID string) string {
	colID := dataGridHeaderColIDFromLayoutID(cfg.ID, focusID)
	if colID == "" {
		return ""
	}
	for _, c := range columns {
		if c.ID == colID {
			return colID
		}
	}
	return ""
}

func dataGridShowHeaderControls(colID, hoveredColID, resizingColID, focusedColID string) bool {
	return colID != "" &&
		(colID == hoveredColID || colID == resizingColID || colID == focusedColID)
}

func dataGridHeaderColUnderCursor(layout *gg.Layout, gridID string, mouseX, mouseY float32) string {
	prefix := gridID + ":header:"
	cell, ok := layout.FindLayout(func(n gg.Layout) bool {
		return len(n.Shape.ID) > len(prefix) &&
			n.Shape.ID[:len(prefix)] == prefix &&
			n.Shape.PointInShape(mouseX, mouseY)
	})
	if ok {
		return dataGridHeaderColIDFromLayoutID(gridID, cell.Shape.ID)
	}
	return ""
}

func dataGridHeaderColIDFromLayoutID(gridID, layoutID string) string {
	prefix := gridID + ":header:"
	if len(layoutID) <= len(prefix) || layoutID[:len(prefix)] != prefix {
		return ""
	}
	return layoutID[len(prefix):]
}

type dataGridHeaderControlResult struct {
	showLabel   bool
	showReorder bool
	showPin     bool
	showResize  bool
}

// dataGridHeaderControlState progressive disclosure:
// controls shown only if they fit. Dropped in priority
// order (pin, reorder, resize). Label hidden if controls
// alone exceed width.
func dataGridHeaderControlState(width float32, padding gg.Padding, hasReorder, hasPin, hasResize bool) dataGridHeaderControlResult {
	available := f32Max(0, width-padding.Width())
	var reorderW, pinW, resizeW float32
	if hasReorder {
		reorderW = dataGridHeaderControlWidth*2 + dataGridHeaderReorderSpacing
	}
	if hasPin {
		pinW = dataGridHeaderControlWidth
	}
	if hasResize {
		resizeW = dataGridResizeHandleWidth
	}
	state := dataGridHeaderControlResult{
		showLabel:   true,
		showReorder: hasReorder,
		showPin:     hasPin,
		showResize:  hasResize,
	}
	controlsWidth := reorderW + pinW + resizeW
	if available < controlsWidth+dataGridHeaderLabelMinWidth {
		state.showLabel = false
	}
	if state.showPin && available < controlsWidth {
		state.showPin = false
		controlsWidth -= pinW
	}
	if state.showReorder && available < controlsWidth {
		state.showReorder = false
		controlsWidth -= reorderW
	}
	if state.showResize && available < controlsWidth {
		state.showResize = false
		controlsWidth -= resizeW
	}
	if available >= controlsWidth+dataGridHeaderLabelMinWidth {
		state.showLabel = true
	}
	return state
}

func dataGridHeaderControlsWidth(showReorder, showPin, showResize bool) float32 {
	width := float32(0)
	if showReorder {
		width += dataGridHeaderControlWidth*2 + dataGridHeaderReorderSpacing
	}
	if showPin {
		width += dataGridHeaderControlWidth
	}
	if showResize {
		width += dataGridResizeHandleWidth
	}
	return width
}
