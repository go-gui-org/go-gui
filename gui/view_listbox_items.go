package gui

// listBoxItemLayoutIDs builds layout IDs and midsOffset for
// drag-reorder tracking.
func listBoxItemLayoutIDs(
	cfg *ListBoxCfg, canReorder bool, first, last int,
) ([]string, int) {
	if !canReorder {
		return nil, 0
	}
	itemLayoutIDs := make([]string, 0, last-first+1)
	midsOffset := 0
	for idx := range first {
		if idx < len(cfg.Data) &&
			!cfg.Data[idx].isSubheading {
			midsOffset++
		}
	}
	for idx := first; idx <= last; idx++ {
		if idx >= 0 && idx < len(cfg.Data) &&
			!cfg.Data[idx].isSubheading {
			itemLayoutIDs = append(itemLayoutIDs,
				listBoxItemID(cfg.ID, cfg.Data[idx].ID))
		}
	}
	return itemLayoutIDs, midsOffset
}

// listBoxBuildItems builds the list of item views, including
// virtualization spacers and drag-reorder gap/ghost handling.
func listBoxBuildItems(
	cfg *ListBoxCfg,
	selectedSet map[string]struct{},
	focusedID string,
	dragIdxByRow []int,
	itemIDs, itemLayoutIDs []string,
	midsOffset int, scrollID string,
	canReorder, dragging bool,
	drag dragReorderState,
	virtualize bool, first, last int,
) ([]View, View) {
	listCap := len(cfg.Data)
	if virtualize && last >= first {
		listCap = last - first + 3
	}
	if dragging {
		listCap += 3
	}
	list := make([]View, 0, listCap)

	if virtualize && first > 0 {
		rh := listCoreRowHeightEstimate(
			cfg.TextStyle, listBoxItemPad)
		list = append(list, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(first) * rh,
			Sizing: FillFixed,
		}))
	}

	var ghostContent View
	for idx := first; idx <= last; idx++ {
		if idx < 0 || idx >= len(cfg.Data) {
			continue
		}
		di := -1
		isDraggable := false
		if dragIdxByRow != nil && idx < len(dragIdxByRow) {
			di = dragIdxByRow[idx]
			isDraggable = di >= 0
		}

		if dragging && isDraggable && di == drag.currentIndex {
			list = append(list,
				dragReorderGapView(drag, dragReorderVertical))
		}

		if dragging && isDraggable && di == drag.sourceIndex {
			ghostContent = listBoxItemContent(
				cfg.Data[idx], *cfg)
			continue
		}

		if canReorder && isDraggable {
			list = append(list, listBoxReorderItemView(
				cfg.Data[idx], *cfg, selectedSet, di,
				itemIDs, itemLayoutIDs, midsOffset, scrollID))
		} else {
			list = append(list,
				listBoxItemView(
					cfg.Data[idx], *cfg, selectedSet, focusedID))
		}
	}

	if virtualize && last < len(cfg.Data)-1 {
		rh := listCoreRowHeightEstimate(
			cfg.TextStyle, listBoxItemPad)
		remaining := len(cfg.Data) - 1 - last
		list = append(list, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(remaining) * rh,
			Sizing: FillFixed,
		}))
	}

	return list, ghostContent
}

func listBoxItemView(dat ListBoxOption, cfg ListBoxCfg, selectedSet map[string]struct{}, focusedID string) View {
	color := ColorTransparent
	if dat.ID == focusedID && !dat.isSubheading {
		color = cfg.ColorHover
	}
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		color = cfg.ColorSelect
	}
	isSub := dat.isSubheading
	content := listBoxItemContent(dat, cfg)

	datID := dat.ID
	isMultiple := cfg.Multiple
	onSelect := cfg.OnSelect
	hasOnSelect := onSelect != nil
	selectedIDs := cfg.SelectedIDs
	colorHover := cfg.ColorHover

	a11yState := AccessStateNone
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		a11yState = AccessStateSelected
	}

	return Row(ContainerCfg{
		A11YRole:   AccessRoleListItem,
		A11YLabel:  dat.Name,
		A11YState:  a11yState,
		Color:      color,
		Padding:    listBoxItemPad,
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content:    []View{content},
		OnClick: func(ctx EventCtx) {
			if hasOnSelect && !isSub {
				ids := listBoxNextSelectedIDs(
					selectedIDs, datID, isMultiple)
				onSelect(ids, EventCtx{nil, ctx.Event, ctx.Window})
			}
		},
		OnHover: func(ctx EventCtx) {
			if hasOnSelect && !isSub {
				ctx.Window.setMouseCursor(CursorPointingHand)
				if ctx.Layout.Shape.Color == ColorTransparent {
					ctx.Layout.Shape.Color = colorHover
				}
			}
		},
	})
}

func listBoxReorderItemView(
	dat ListBoxOption,
	cfg ListBoxCfg,
	selectedSet map[string]struct{},
	dragIdx int,
	itemIDs []string,
	itemLayoutIDs []string,
	midsOffset int,
	scrollID string,
) View {
	color := ColorTransparent
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		color = cfg.ColorSelect
	}
	content := listBoxItemContent(dat, cfg)
	layoutID := listBoxItemID(cfg.ID, dat.ID)

	datID := dat.ID
	isMultiple := cfg.Multiple
	onSelect := cfg.OnSelect
	hasOnSelect := onSelect != nil
	selectedIDs := cfg.SelectedIDs
	colorHover := cfg.ColorHover
	listBoxID := cfg.ID
	onReorder := cfg.OnReorder

	a11yState := AccessStateNone
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		a11yState = AccessStateSelected
	}

	return Row(ContainerCfg{
		ID:         layoutID,
		A11YRole:   AccessRoleListItem,
		A11YLabel:  dat.Name,
		A11YState:  a11yState,
		Color:      color,
		Padding:    listBoxItemPad,
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content:    []View{content},
		OnClick: func(ctx EventCtx) {
			dragReorderStart(dragReorderStartCfg{
				DragKey:       listBoxID,
				Index:         dragIdx,
				ItemID:        datID,
				Axis:          dragReorderVertical,
				ItemIDs:       itemIDs,
				OnReorder:     onReorder,
				ItemLayoutIDs: itemLayoutIDs,
				MidsOffset:    midsOffset,
				scrollID:      scrollID,
				Layout:        ctx.Layout,
				Event:         ctx.Event,
			}, ctx.Window)
			if hasOnSelect {
				ids := listBoxNextSelectedIDs(
					selectedIDs, datID, isMultiple)
				onSelect(ids, EventCtx{nil, ctx.Event, ctx.Window})
			}
		},
		OnHover: func(ctx EventCtx) {
			ctx.Window.setMouseCursor(CursorPointingHand)
			if ctx.Layout.Shape.Color == ColorTransparent {
				ctx.Layout.Shape.Color = colorHover
			}
		},
	})
}

// listBoxItemID is the layout ID of one option row. Both the row
// itself and the scroll-into-view lookup compose it, so they share
// this helper rather than each spelling out the scope.
func listBoxItemID(listID, optionID string) string {
	return ScopeID(listID, "item", optionID)
}

func listBoxItemContent(dat ListBoxOption, cfg ListBoxCfg) View {
	if dat.isSubheading {
		return Column(ContainerCfg{
			Spacing: SomeF(1),
			Padding: NoPadding,
			Sizing:  FillFit,
			Content: []View{
				Text(TextCfg{
					Text:      dat.Name,
					TextStyle: cfg.subheadingStyle,
				}),
				Row(ContainerCfg{
					Padding: NoPadding,
					Sizing:  FillFit,
					Content: []View{
						Rectangle(RectangleCfg{
							Width:  1,
							Height: 1,
							Sizing: FillFit,
							Color:  cfg.subheadingStyle.Color,
						}),
					},
				}),
			},
		})
	}
	return Text(TextCfg{
		Text:      dat.Name,
		Mode:      TextModeMultiline,
		TextStyle: cfg.TextStyle,
	})
}

// listBoxDataIndex maps an itemIDs index to the full data index
// (including subheading rows). Returns dataIdx unchanged if no
// mapping is provided.
func listBoxDataIndex(itemDataIndices []int, idx int) int {
	if idx >= 0 && idx < len(itemDataIndices) {
		return itemDataIndices[idx]
	}
	return idx
}
