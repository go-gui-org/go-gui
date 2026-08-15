package gui

// tableBuildRow builds a single table row view.
func tableBuildRow(
	cfg *TableCfg, rowIdx int, columnWidths []float32,
	cellBorder float32, selected map[int]bool,
	multiSelect bool, colorHover Color,
	onSelect func(map[int]bool, int, EventCtx),
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
			if rowOnClick != nil {
				rowOnClick(ctx)
			}
			if onSelect != nil {
				var newSel map[int]bool
				if multiSelect {
					newSel = copySelected(selected)
				} else {
					newSel = make(map[int]bool)
				}
				if newSel[ri] {
					delete(newSel, ri)
				} else {
					newSel[ri] = true
				}
				onSelect(newSel, ri, ctx)
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

// tableFreezeLayout builds the split layout: fixed header zone
// above a scrollable body zone.
func tableFreezeLayout(
	cfg *TableCfg, columnWidths []float32, cellBorder float32,
	rowSpacing float32, selected map[int]bool,
	multiSelect bool, colorHover Color,
	onSelect func(map[int]bool, int, EventCtx),
	bodyRows []View,
	scrollID string,
) View {
	// Header zone: row 0 + optional separator.
	headerViews := []View{
		tableBuildRow(cfg, 0, columnWidths, cellBorder,
			selected, multiSelect, colorHover, onSelect),
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

	return Column(ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleGrid,
		A11YCfg:   A11YCfg{A11YLabel: cfg.A11YLabel},
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
	})
}
