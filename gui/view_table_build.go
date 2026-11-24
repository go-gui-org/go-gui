package gui

// tableBuildRow builds a single table row view.
//
// activeRowIdx is the keyboard position (-1 for a table that never
// joins the tab order); the active row is tinted with the hover color
// so arrow-key movement is visible. activeKey is the table's
// effective ID — the key that row clicks sync the keyboard position
// under, so keys continue from where the mouse last went; it is empty
// for a non-focusable table, which skips the sync write entirely.
func tableBuildRow(
	cfg *TableCfg, rowIdx int, columnWidths []float32,
	cellBorder float32, selected map[int]bool,
	multiSelect bool,
	onSelect func(map[int]bool, int, EventCtx),
	activeRowIdx int, activeKey string, rh tableRowHeader,
	fz *tableFreeze,
) View {
	r := cfg.Data[rowIdx]
	isSelected := selected[rowIdx]
	// The keyboard-active row is the Focus state: arrow-key
	// movement stays visible through the theme's focus fill.
	isActive := rowIdx == activeRowIdx && activeRowIdx >= 0
	cells := fz.rowCells(len(r.Cells) + 1)
	windowSlot := -1 // index of the scroll window in a split row

	// The row header leads the row but is not one of its cells, so
	// colIdx below still indexes columnWidths and ColumnAlignments.
	if rh.on {
		cells = append(cells, tableRowHeaderCell(cfg, rowIdx, rh,
			cellBorder, selected, multiSelect, onSelect))
	}

	for colIdx, cell := range r.Cells {
		if fz.on && colIdx == fz.cols {
			cells, windowSlot = fz.split(cells)
		}
		cellTextStyle := cfg.TextStyle
		if cell.TextStyle != nil {
			cellTextStyle = *cell.TextStyle
		} else if cell.HeadCell {
			cellTextStyle = cfg.TextStyleHead
		}
		// The selected row paints the subtle wash, so its cells keep
		// their normal text color — a tint needs no paired foreground
		// (visual-refresh §4.3). A cell's own TextStyle is the
		// caller's deliberate color and stays untouched regardless
		// (issue #373).

		var colWidth float32
		if colIdx < len(columnWidths) {
			colWidth = columnWidths[colIdx]
		}

		hAlign := HAlignStart
		if cell.HAlign != nil {
			hAlign = *cell.HAlign
		} else if cell.HeadCell {
			hAlign = *cfg.AlignHead
		} else if colIdx < len(cfg.ColumnAlignments) {
			hAlign = cfg.ColumnAlignments[colIdx]
		}

		cellContent := rowContent(fz.w, 1)
		if cell.RichText != nil {
			cellContent = append(cellContent,
				RTF(RTFCfg{RichText: *cell.RichText}))
		} else if cell.Content != nil {
			cellContent = append(cellContent, cell.Content)
		} else {
			cellContent = append(cellContent, Text(TextCfg{
				Text:      cell.Value,
				TextStyle: cellTextStyle,
			}))
		}

		cellOnClick := cell.OnClick
		ch := cfg.Colors.Hover
		var cellOnHover func(EventCtx)
		if cellOnClick != nil {
			cellOnHover = func(ctx EventCtx) {
				ctx.Window.SetMouseCursorPointingHand()
				ctx.Layout.Shape.Color = ch
			}
		}

		cells = append(cells, Column(ContainerCfg{
			A11YRole:    AccessRoleGridCell,
			Color:       ColorTransparent,
			ColorBorder: cfg.Colors.Border,
			SizeBorder:  BorderPx(cellBorder),
			Padding:     cfg.CellPadding,
			Radius:      NoRadius,
			HAlign:      hAlign,
			Sizing:      FixedFill,
			Width:       colWidth,
			OnClick:     cellOnClick,
			OnHover:     cellOnHover,
			Content:     cellContent,
		}))
	}

	// The resting fill is transparent, or the alternate color on odd
	// rows; it lands in Base after resolve ran, so it changes only the
	// resting fill.
	base := ColorTransparent
	if cfg.ColorRowAlt != nil && rowIdx%2 == 1 {
		base = *cfg.ColorRowAlt
	}
	bg, bgHover := rowFill(cfg.Colors, base, isSelected, isActive)

	rowOnClick := r.OnClick
	ri := rowIdx

	// A split row fills the viewport, up to its columns, so its window
	// shows what fits beside the frozen cells.
	var rowSizing Sizing
	if fz.on {
		rowSizing = FillFit
	}
	if windowSlot >= 0 {
		cells = fz.closeWindow(cells, windowSlot)
	}

	return Row(ContainerCfg{
		Sizing:     rowSizing,
		MaxWidth:   fz.rowWidth,
		Color:      bg,
		Spacing:    SpacingPx(-cellBorder),
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    cells,
		OnClick: func(ctx EventCtx) {
			// A click is also a keyboard position change: the next
			// arrow key moves from here, not from where keys last
			// went.
			if activeKey != "" {
				StateMap[string, int](
					ctx.Window, nsTableFocus, capModerate,
				).Set(activeKey, ri)
			}
			if rowOnClick != nil {
				rowOnClick(ctx)
			}
			if onSelect != nil {
				onSelect(tableNextSelection(selected, multiSelect, ri),
					ri, ctx)
			}
		},
		OnHover: func(ctx EventCtx) {
			if onSelect != nil {
				ctx.Window.SetMouseCursorPointingHand()
				ctx.Layout.Shape.Color = bgHover
			}
		},
	})
}

// tableBuildRows builds the row views for the visible range: the
// data rows themselves, plus the horizontal separators between them
// and the virtualization spacers above and below. The range runs
// first..last, clamped to the data; dataStart is 1 for the freeze
// layout, whose pinned header is row 0 and never part of the body.
func tableBuildRows(
	cfg *TableCfg, columnWidths []float32, cellBorder float32,
	selected map[int]bool, multiSelect bool,
	onSelect func(map[int]bool, int, EventCtx),
	activeRowIdx int, navKey string, rh tableRowHeader,
	fz *tableFreeze,
	first, last, lastRowIdx, dataStart int,
	rowHeight float32, virtualize bool,
) []View {
	capacity := last - first + 3
	if !virtualize {
		capacity = len(cfg.Data) * 2
	}
	rows := make([]View, 0, capacity)
	sepWidth := tableSeparatorWidth(columnWidths, cellBorder, rh, fz)

	// Top spacer for virtualization.
	if virtualize && first > dataStart && rowHeight > 0 {
		rows = append(rows, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(first-dataStart) * rowHeight,
			Sizing: FillFixed,
		}))
	}

	for rowIdx := first; rowIdx <= last; rowIdx++ {
		if rowIdx < 0 || rowIdx > lastRowIdx {
			continue
		}
		rows = append(rows, tableBuildRow(
			cfg, rowIdx, columnWidths, cellBorder,
			selected, multiSelect, onSelect,
			activeRowIdx, navKey, rh, fz))

		// Horizontal separator.
		sepHeight := cfg.SizeBorder
		if rowIdx == 0 && cfg.SizeBorderHeader > 0 {
			sepHeight = cfg.SizeBorderHeader
		}

		needsSep := false
		switch cfg.BorderStyle {
		case TableBorderHorizontal:
			needsSep = rowIdx != lastRowIdx
		case TableBorderHeaderOnly:
			needsSep = rowIdx == 0
		}

		if needsSep {
			rows = append(rows, tableSeparator(cfg, sepHeight, sepWidth))
		}
	}

	// Bottom spacer for virtualization.
	if virtualize && last < lastRowIdx && rowHeight > 0 {
		remaining := lastRowIdx - last
		rows = append(rows, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(remaining) * rowHeight,
			Sizing: FillFixed,
		}))
	}

	return rows
}

// tableRowWidth is the width of a full row: every column plus the row
// header, less the overlap of the collapsed cell borders.
func tableRowWidth(columnWidths []float32, cellBorder float32, rh tableRowHeader) float32 {
	var width float32
	cells := len(columnWidths)
	for _, cw := range columnWidths {
		width += cw
	}
	if rh.on {
		width += rh.width
		cells++
	}
	if cells > 1 {
		width -= float32(cells-1) * cellBorder
	}
	return width
}

// tableSeparatorWidth is the least width of a separator: a full row's,
// or with frozen columns the narrowest row's. Those rows stay in place
// while their cells scroll, and a wider separator would be wider than
// they are.
func tableSeparatorWidth(
	columnWidths []float32, cellBorder float32, rh tableRowHeader,
	fz *tableFreeze,
) float32 {
	if fz.on {
		return fz.rowMin
	}
	return tableRowWidth(columnWidths, cellBorder, rh)
}

// tableSeparator is the line between two rows. It fills the table's
// width, but is never narrower than a row: in a table scrolled sideways
// a line only as wide as the table stopped short of the last columns
// (#1293).
func tableSeparator(cfg *TableCfg, height, rowWidth float32) View {
	return Rectangle(RectangleCfg{
		Color:  cfg.Colors.Border,
		Width:  rowWidth,
		Height: height,
		Sizing: FillFixed,
	})
}

// tableFreezeLayout builds the split layout: fixed header zone
// above a scrollable body zone.
func tableFreezeLayout(
	cfg *TableCfg, columnWidths []float32, cellBorder float32,
	rowSpacing float32, selected map[int]bool,
	multiSelect bool,
	onSelect func(map[int]bool, int, EventCtx),
	bodyRows []View,
	scrollID string,
	activeRowIdx int,
	activeKey string,
	rh tableRowHeader,
	fz *tableFreeze,
	fw tableFocusState,
) View {
	// Header zone: row 0 + optional separator.
	headerViews := []View{
		tableBuildRow(cfg, 0, columnWidths, cellBorder,
			selected, multiSelect, onSelect,
			activeRowIdx, activeKey, rh, fz),
	}

	sepHeight := cfg.SizeBorder
	if cfg.SizeBorderHeader > 0 {
		sepHeight = cfg.SizeBorderHeader
	}
	needsSep := false
	switch cfg.BorderStyle {
	case TableBorderHorizontal, TableBorderHeaderOnly:
		needsSep = true
	}
	if needsSep {
		headerViews = append(headerViews, tableSeparator(cfg, sepHeight,
			tableSeparatorWidth(columnWidths, cellBorder, rh, fz)))
	}

	// The body keeps room on the right for the vertical scrollbar and
	// at the bottom for the horizontal one, so neither covers a cell.
	gutter := DefaultScrollbarStyle.Size + PadXSmall

	// The header zone clips, so its row of columns no longer sets the
	// table's minimum width: the table fits its parent and the body
	// scrolls sideways. The zone follows the body's X offset
	// (scroll_link.go), so the header moves with the columns but stays
	// frozen above the rows (#1293). It keeps the body's side gutter,
	// so both start and clip at the same x; RTL moves the gutter to the
	// left in both.
	headerZone := Column(ContainerCfg{
		Sizing:      FillFit,
		Padding:     NewPadding(0, gutter, 0, 0),
		Spacing:     SpacingPx(rowSpacing),
		SizeBorder:  NoBorder,
		Clip:        true,
		ScrollLinkX: scrollID,
		Content:     headerViews,
	})

	bodyCfg := ContainerCfg{
		Sizing:     FillFill,
		Padding:    NewPadding(0, gutter, gutter, 0),
		Spacing:    SpacingPx(rowSpacing),
		SizeBorder: NoBorder,
		ID:         scrollID,
		Scrollable: true,
		Content:    bodyRows,
	}
	if fz.on {
		bodyCfg.scrollExtentX = fz.rowWidth
		bodyCfg.scrollExtentMin = fz.rowMin
	}
	if cfg.Scrollbar != ScrollbarAuto {
		bodyCfg.ScrollbarCfgX = &ScrollbarCfg{
			Overflow: cfg.Scrollbar,
		}
		bodyCfg.ScrollbarCfgY = &ScrollbarCfg{
			Overflow: cfg.Scrollbar,
		}
	}
	bodyZone := Column(bodyCfg)

	outerCfg := ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleGrid,
		A11YCfg:   A11YCfg{A11YLabel: cfg.A11YLabel, A11YDescription: cfg.A11YDescription},
		Color:     ColorTransparent,
		Padding:   NoPadding,
		Spacing:   NoSpacing,
		Radius:    NoRadius,
		Sizing:    cfg.Sizing,
		Width:     cfg.Width,
		Height:    cfg.Height,
		MinWidth:  cfg.MinWidth,
		MaxWidth:  cfg.MaxWidth,
		MinHeight: cfg.MinHeight,
		MaxHeight: cfg.MaxHeight,
		Content: []View{
			headerZone,
			bodyZone,
		},
	}
	tableWireFocus(&outerCfg, fw)
	return Column(outerCfg)
}
