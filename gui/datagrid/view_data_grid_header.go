package datagrid

import (
	"fmt"
	"math"
	"slices"

	. "github.com/go-gui-org/go-gui/gui"
)

// dataGridHeaderRow builds the header row with all column
// header cells.
func dataGridHeaderRow(cfg *DataGridCfg, columns []GridColumnCfg, columnWidths map[string]float32, focusID uint32, hoveredColID, resizingColID, focusedColID string) View {
	cells := make([]View, 0, len(columns))
	for idx, col := range columns {
		width := dataGridColumnWidthFor(col, columnWidths)
		showControls := dataGridShowHeaderControls(col.ID, hoveredColID, resizingColID, focusedColID)
		cells = append(cells, dataGridHeaderCell(cfg, col, idx, len(columns), width, focusID, showControls))
	}
	return Row(ContainerCfg{
		Height:      dataGridHeaderHeight(cfg),
		Sizing:      FillFixed,
		Color:       ColorTransparent,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     NoPadding,
		Spacing:     Some(-cfg.SizeBorder.Get(0)),
		Content:     cells,
	})
}

func dataGridHeaderCell(cfg *DataGridCfg, col GridColumnCfg, colIdx, colCount int, width float32, focusID uint32, showControls bool) View {
	hasReorder := showControls && cfg.OnColumnOrderChange != nil && col.Reorderable
	hasPin := showControls && cfg.OnColumnPinChange != nil
	headerControls := dataGridHeaderControlState(width, cfg.PaddingHeader.Get(Padding{}), hasReorder, hasPin, showControls && col.Resizable)
	headerFocusID := dataGridHeaderFocusID(cfg, colCount, colIdx)

	content := make([]View, 0, 5)
	indicator := dataGridHeaderIndicator(cfg.Query, col.ID)

	labelContent := make([]View, 0, 2)
	labelContent = append(labelContent, Text(TextCfg{
		Text:      col.Title,
		Mode:      TextModeSingleLine,
		TextStyle: cfg.TextStyleHeader,
	}))
	if indicator != "" {
		labelContent = append(labelContent, Text(TextCfg{
			Text:      indicator,
			Mode:      TextModeSingleLine,
			TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleHeader),
		}))
	}

	if headerControls.showLabel {
		content = append(content, Row(ContainerCfg{
			Sizing:  FillFill,
			Clip:    true,
			Padding: NoPadding,
			HAlign:  col.Align,
			VAlign:  VAlignMiddle,
			Spacing: SomeF(6),
			Content: labelContent,
		}))
	} else {
		content = append(content, Row(ContainerCfg{
			Sizing:  FillFill,
			Padding: NoPadding,
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
	headerA11YState := AccessStateNone
	if headerSorted {
		headerA11YState = AccessStateSelected
	}

	return Row(ContainerCfg{
		ID:          cfg.ID + ":header:" + col.ID,
		A11YRole:    AccessRoleGridCell,
		A11YLabel:   col.Title,
		A11YState:   headerA11YState,
		Width:       width,
		Sizing:      FixedFill,
		Padding:     cfg.PaddingHeader,
		Clip:        true,
		Color:       cfg.ColorHeader,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Spacing:     SomeF(0),
		OnClick: func(_ *Layout, e *Event, w *Window) {
			e.IsHandled = true
			if colSortable && onQueryChange != nil {
				shiftSort := multiSort && e.Modifiers.Has(ModShift)
				next := dataGridToggleSort(query, colID, multiSort, shiftSort)
				onQueryChange(next, e, w)
			}
			if headerFocusID > 0 {
				w.SetIDFocus(headerFocusID)
			} else if focusID > 0 {
				w.SetIDFocus(focusID)
			}
		},
		OnHover: func(layout *Layout, _ *Event, w *Window) {
			if cfg.Disabled {
				return
			}
			if colSortable {
				w.SetMouseCursorPointingHand()
				layout.Shape.Color = colorHeaderHover
			}
		},
		IDFocus: headerFocusID,
		Content: content,
	})
}

func dataGridResizeHandle(cfg *DataGridCfg, col GridColumnCfg, focusID uint32) View {
	gridID := cfg.ID
	columns := cfg.Columns
	rows := cfg.Rows
	textStyleHeader := cfg.TextStyleHeader
	textStyle := cfg.TextStyle
	paddingCell := cfg.PaddingCell.Get(Padding{})
	colorResizeHandle := cfg.ColorResizeHandle
	colorResizeActive := cfg.ColorResizeActive

	disabled := cfg.Disabled

	return Row(ContainerCfg{
		ID:      gridID + ":resize:" + col.ID,
		Width:   dataGridResizeHandleWidth,
		Sizing:  FixedFill,
		Padding: NoPadding,
		Color:   colorResizeHandle,
		OnClick: func(layout *Layout, e *Event, w *Window) {
			if disabled {
				return
			}
			startX := layout.Shape.X + e.MouseX
			dataGridStartResize(gridID, columns, rows, textStyleHeader, textStyle, paddingCell, col, focusID, startX, e, w)
		},
		OnHover: func(layout *Layout, e *Event, w *Window) {
			if disabled {
				return
			}
			w.SetMouseCursorEW()
			if e.MouseButton == MouseLeft {
				layout.Shape.Color = colorResizeActive
			} else {
				layout.Shape.Color = colorResizeHandle
			}
		},
		Content: []View{
			Rectangle(RectangleCfg{
				Width:  1,
				Height: 1,
				Sizing: FillFill,
				Color:  ColorTransparent,
			}),
		},
	})
}

func dataGridReorderControls(cfg *DataGridCfg, col GridColumnCfg) View {
	onColumnOrderChange := cfg.OnColumnOrderChange
	baseOrder, _ := dataGridColumnOrderAndMap(cfg.Columns, cfg.ColumnOrder)
	colID := col.ID
	leftArrow := "\u25C0"  // ◀
	rightArrow := "\u25B6" // ▶
	if ActiveLocale.TextDir == TextDirRTL {
		leftArrow, rightArrow = rightArrow, leftArrow
	}

	reorderCB := func(delta int) func(*Event, *Window) {
		return func(e *Event, w *Window) {
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

	return Row(ContainerCfg{
		Padding: NoPadding,
		Spacing: Some(dataGridHeaderReorderSpacing),
		Width:   dataGridHeaderControlsWidth(true, false, false),
		Sizing:  FixedFill,
		Content: []View{
			dataGridOrderButton(leftArrow, cfg.TextStyleHeader, cfg.ColorHeaderHover, reorderCB(-1)),
			dataGridOrderButton(rightArrow, cfg.TextStyleHeader, cfg.ColorHeaderHover, reorderCB(1)),
		},
	})
}

func dataGridOrderButton(label string, baseStyle TextStyle, hoverColor Color, cb func(*Event, *Window)) View {
	return dataGridIndicatorButton(label, baseStyle, hoverColor, false, dataGridHeaderControlWidth,
		func(_ *Layout, e *Event, w *Window) {
			cb(e, w)
		})
}

func dataGridIndicatorButton(label string, baseStyle TextStyle, hoverColor Color, disabled bool, width float32, onClick func(*Layout, *Event, *Window)) View {
	sizing := FitFill
	if width > 0 {
		sizing = FixedFill
	}
	return Button(ButtonCfg{
		Width:       width,
		Sizing:      sizing,
		Padding:     NoPadding,
		SizeBorder:  SomeF(0),
		Radius:      SomeF(0),
		Color:       ColorTransparent,
		ColorHover:  hoverColor,
		ColorFocus:  ColorTransparent,
		ColorClick:  hoverColor,
		ColorBorder: ColorTransparent,
		Disabled:    disabled,
		OnClick:     onClick,
		Content: []View{
			Text(TextCfg{
				Text:      label,
				Mode:      TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(baseStyle),
			}),
		},
	})
}

func dataGridPinControl(cfg *DataGridCfg, col GridColumnCfg) View {
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
		false, dataGridHeaderControlWidth, func(_ *Layout, e *Event, w *Window) {
			if onColumnPinChange == nil {
				return
			}
			nextPin := dataGridColumnNextPin(colPin)
			onColumnPinChange(colID, nextPin, e, w)
			e.IsHandled = true
		})
}

func dataGridFilterRow(cfg *DataGridCfg, columns []GridColumnCfg, columnWidths map[string]float32) View {
	cells := make([]View, 0, len(columns))
	for _, col := range columns {
		cells = append(cells, dataGridFilterCell(cfg, col, dataGridColumnWidthFor(col, columnWidths)))
	}
	return Row(ContainerCfg{
		Height:      dataGridFilterHeight(cfg),
		Sizing:      FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     cfg.PaddingFilter,
		Spacing:     Some(-cfg.SizeBorder.Get(0)),
		Content:     cells,
	})
}

func dataGridFilterCell(cfg *DataGridCfg, col GridColumnCfg, width float32) View {
	query := cfg.Query
	value := dataGridQueryFilterValue(query, col.ID)
	inputID := cfg.ID + ":filter:" + col.ID
	onQueryChange := cfg.OnQueryChange
	colID := col.ID
	var placeholder string
	if col.Filterable {
		placeholder = ActiveLocale.StrFilter
	}

	return Row(ContainerCfg{
		ID:          cfg.ID + ":filter_cell:" + col.ID,
		Width:       width,
		Sizing:      FixedFill,
		Padding:     cfg.PaddingFilter,
		Color:       ColorTransparent,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Spacing:     SomeF(0),
		Content: []View{
			Input(InputCfg{
				ID:          inputID,
				IDFocus:     FnvSum32(inputID),
				Text:        value,
				Placeholder: placeholder,
				Disabled:    !col.Filterable || onQueryChange == nil,
				Sizing:      FillFill,
				Padding:     NoPadding,
				SizeBorder:  SomeF(0),
				Radius:      SomeF(0),
				Color:       cfg.ColorFilter,
				ColorHover:  cfg.ColorFilter,
				ColorBorder: cfg.ColorBorder,
				TextStyle:   cfg.TextStyleFilter,
				OnTextChanged: func(_ *Layout, text string, w *Window) {
					if onQueryChange == nil {
						return
					}
					next := dataGridQuerySetFilter(query, colID, text)
					e := &Event{}
					onQueryChange(next, e, w)
				},
			}),
		},
	})
}

func dataGridStartResize(gridID string, columns []GridColumnCfg, rows []GridRow, textStyleHeader, textStyle TextStyle, paddingCell Padding, col GridColumnCfg, focusID uint32, startMouseX float32, e *Event, w *Window) {
	if focusID > 0 {
		w.SetIDFocus(focusID)
	}
	dgRS := StateMap[string, dataGridResizeState](w, nsDgResize, capModerate)
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

	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			dataGridResizeDrag(gridID, col, e, w)
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			dataGridEndResize(gridID, w)
			w.MouseUnlock()
			if focusID > 0 {
				w.SetIDFocus(focusID)
			}
		},
	})
	e.IsHandled = true
}

func dataGridResizeDrag(gridID string, col GridColumnCfg, e *Event, w *Window) {
	dgRS := StateMap[string, dataGridResizeState](w, nsDgResize, capModerate)
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

func dataGridEndResize(gridID string, w *Window) {
	dgRS := StateMap[string, dataGridResizeState](w, nsDgResize, capModerate)
	runtime, ok := dgRS.Get(gridID)
	if !ok {
		return
	}
	runtime.Active = false
	dgRS.Set(gridID, runtime)
}

func dataGridAutoFitWidth(rows []GridRow, textStyleHeader, textStyle TextStyle, paddingCell Padding, col GridColumnCfg, w *Window) float32 {
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

func dataGridActiveResizeColID(gridID string, w *Window) string {
	dgRS := StateMap[string, dataGridResizeState](w, nsDgResize, capModerate)
	if runtime, ok := dgRS.Get(gridID); ok && runtime.Active {
		return runtime.ColID
	}
	return ""
}

func dataGridHeaderFocusBaseID(cfg *DataGridCfg, colCount int) uint32 {
	if colCount <= 0 {
		return 0
	}
	span := uint32(colCount)
	body := dataGridFocusID(cfg)
	if body <= math.MaxUint32-span {
		return body + 1
	}
	if body > span {
		return body - span
	}
	return 1
}

func dataGridHeaderFocusID(cfg *DataGridCfg, colCount, colIdx int) uint32 {
	if colCount <= 0 || colIdx < 0 || colIdx >= colCount {
		return 0
	}
	base := dataGridHeaderFocusBaseID(cfg, colCount)
	return base + uint32(colIdx)
}

func dataGridHeaderFocusIndex(cfg *DataGridCfg, colCount int, focusID uint32) int {
	if colCount <= 0 || focusID == 0 {
		return -1
	}
	base := dataGridHeaderFocusBaseID(cfg, colCount)
	if focusID < base {
		return -1
	}
	idx := int(focusID - base)
	if idx < 0 || idx >= colCount {
		return -1
	}
	return idx
}

func dataGridHeaderFocusedColID(cfg *DataGridCfg, columns []GridColumnCfg, focusID uint32) string {
	idx := dataGridHeaderFocusIndex(cfg, len(columns), focusID)
	if idx < 0 || idx >= len(columns) {
		return ""
	}
	return columns[idx].ID
}

func dataGridShowHeaderControls(colID, hoveredColID, resizingColID, focusedColID string) bool {
	return colID != "" &&
		(colID == hoveredColID || colID == resizingColID || colID == focusedColID)
}

func dataGridHeaderColUnderCursor(layout *Layout, gridID string, mouseX, mouseY float32) string {
	prefix := gridID + ":header:"
	cell, ok := layout.FindLayout(func(n Layout) bool {
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
func dataGridHeaderControlState(width float32, padding Padding, hasReorder, hasPin, hasResize bool) dataGridHeaderControlResult {
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
