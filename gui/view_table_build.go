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
	multiSelect bool, colorHover Color,
	onSelect func(map[int]bool, int, EventCtx),
	activeRowIdx int, activeKey string,
) View {
	r := cfg.Data[rowIdx]
	cells := make([]View, 0, len(r.Cells))

	for colIdx, cell := range r.Cells {
		cellTextStyle := cfg.TextStyle
		if cell.TextStyle != nil {
			cellTextStyle = *cell.TextStyle
		} else if cell.HeadCell {
			cellTextStyle = cfg.TextStyleHead
		}

		var colWidth float32
		if colIdx < len(columnWidths) {
			colWidth = columnWidths[colIdx]
		}

		hAlign := HAlignStart
		if cell.HAlign != nil {
			hAlign = *cell.HAlign
		} else if cell.HeadCell {
			hAlign = *cfg.alignHead
		} else if colIdx < len(cfg.columnAlignments) {
			hAlign = cfg.columnAlignments[colIdx]
		}

		var cellContent []View
		if cell.RichText != nil {
			cellContent = []View{
				RTF(RTFCfg{RichText: *cell.RichText}),
			}
		} else if cell.Content != nil {
			cellContent = []View{cell.Content}
		} else {
			cellContent = []View{
				Text(TextCfg{
					Text:      cell.Value,
					TextStyle: cellTextStyle,
				}),
			}
		}

		cellOnClick := cell.OnClick
		ch := colorHover
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
			ColorBorder: cfg.ColorBorder,
			SizeBorder:  Some(cellBorder),
			Padding:     cfg.cellPadding,
			Radius:      SomeF(0),
			HAlign:      hAlign,
			Sizing:      FixedFill,
			Width:       colWidth,
			OnClick:     cellOnClick,
			OnHover:     cellOnHover,
			Content:     cellContent,
		}))
	}

	isSelected := selected[rowIdx]
	rowColor := ColorTransparent
	if isSelected {
		rowColor = cfg.ColorSelect
	} else if rowIdx == activeRowIdx && activeRowIdx >= 0 {
		rowColor = colorHover
	} else if cfg.ColorRowAlt != nil && rowIdx%2 == 1 {
		rowColor = *cfg.ColorRowAlt
	}

	rowOnClick := r.OnClick
	ri := rowIdx

	return Row(ContainerCfg{
		Color:      rowColor,
		Spacing:    Some(-cellBorder),
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
				if !isSelected {
					ctx.Layout.Shape.Color = colorHover
				}
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
	selected map[int]bool, multiSelect bool, colorHover Color,
	onSelect func(map[int]bool, int, EventCtx),
	activeRowIdx int, navKey string,
	first, last, lastRowIdx, dataStart int,
	rowHeight float32, virtualize bool,
) []View {
	capacity := last - first + 3
	if !virtualize {
		capacity = len(cfg.Data) * 2
	}
	rows := make([]View, 0, capacity)

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
			selected, multiSelect, colorHover, onSelect,
			activeRowIdx, navKey))

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
			rows = append(rows, Rectangle(RectangleCfg{
				Color:  cfg.ColorBorder,
				Height: sepHeight,
				Sizing: FillFixed,
			}))
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

// tableFreezeLayout builds the split layout: fixed header zone
// above a scrollable body zone.
func tableFreezeLayout(
	cfg *TableCfg, columnWidths []float32, cellBorder float32,
	rowSpacing float32, selected map[int]bool,
	multiSelect bool, colorHover Color,
	onSelect func(map[int]bool, int, EventCtx),
	bodyRows []View,
	scrollID string,
	activeRowIdx int,
	activeKey string,
	fw tableFocusState,
) View {
	// Header zone: row 0 + optional separator.
	headerViews := []View{
		tableBuildRow(cfg, 0, columnWidths, cellBorder,
			selected, multiSelect, colorHover, onSelect,
			activeRowIdx, activeKey),
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
		headerViews = append(headerViews, Rectangle(RectangleCfg{
			Color:  cfg.ColorBorder,
			Height: sepHeight,
			Sizing: FillFixed,
		}))
	}

	headerZone := Column(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NoPadding,
		Spacing:    Some(rowSpacing),
		SizeBorder: NoBorder,
		Content:    headerViews,
	})

	bodyCfg := ContainerCfg{
		Sizing:     FillFill,
		Padding:    NewPadding(0, DefaultScrollbarStyle.Size+PadXSmall, 0, 0),
		Spacing:    Some(rowSpacing),
		SizeBorder: NoBorder,
		ID:         scrollID,
		Scrollable: true,
		Content:    bodyRows,
		ScrollbarCfgX: &ScrollbarCfg{
			Overflow: ScrollbarHidden,
		},
	}
	if cfg.Scrollbar != ScrollbarAuto {
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
		Spacing:   SomeF(0),
		Radius:    SomeF(0),
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
