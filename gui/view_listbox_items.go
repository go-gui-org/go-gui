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
//
// rowH is the caller's row-height estimate rather than a second
// computation of it: the spacers, the visible range and the height
// model registered for ScrollToIndex are one arithmetic, and
// recomputing it here is how the three drift apart.
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
	rowH float32,
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
		list = append(list, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(first) * rowH,
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
			ghostContent = listBoxItemContent(cfg.Data[idx], *cfg)
			continue
		}

		if canReorder && isDraggable {
			list = append(list, listBoxReorderItemView(
				cfg.Data[idx], *cfg, selectedSet, focusedID, di,
				itemIDs, itemLayoutIDs, midsOffset, scrollID))
		} else {
			list = append(list,
				listBoxItemView(
					cfg.Data[idx], *cfg, selectedSet,
					focusedID, itemIDs))
		}
	}

	if virtualize && last < len(cfg.Data)-1 {
		remaining := len(cfg.Data) - 1 - last
		list = append(list, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(remaining) * rowH,
			Sizing: FillFixed,
		}))
	}

	return list, ghostContent
}

func listBoxItemView(
	dat ListBoxOption,
	cfg ListBoxCfg,
	selectedSet map[string]struct{},
	focusedID string,
	itemIDs []string,
) View {
	color := ColorTransparent
	// The keyboard-focus row takes a ring, never a fill: a fill here
	// is the same grey the mouse hover paints, so the two cursors
	// were indistinguishable (visual-refresh §4.3).
	isFocusRow := listBoxIsFocusRow(dat, focusedID)
	selected := false
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		// Selection paints the subtle wash, not the full accent
		// slab; focus is the ring, not a second fill
		// (visual-refresh §4.3).
		color = cfg.ColorSelectSubtle
		selected = true
	}
	isSub := dat.isSubheading
	content := listBoxItemContent(dat, cfg)

	datID := dat.ID
	isMultiple := cfg.Multiple
	onSelect := cfg.OnSelect
	hasOnSelect := onSelect != nil
	selectedIDs := cfg.SelectedIDs
	colorHover := cfg.ColorHover
	// Scalars, not cfg: the OnClick closure below would otherwise
	// capture the whole ListBoxCfg and heap-allocate it per row.
	listBoxID := cfg.ID
	focusDisabled := cfg.FocusDisabled

	a11yState := AccessStateNone
	if selected {
		a11yState = AccessStateSelected
	}

	// A subheading is a caption, not a choice, and a list with no
	// OnSelect answers nothing — both stay silent. Otherwise the row
	// picks one of several, which is the selection role (issue #467).
	rowSound := SoundNone
	if hasOnSelect && !isSub {
		rowSound = resolveSoundCue(
			guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled)
	}

	return Row(ContainerCfg{
		A11YRole:  AccessRoleListItem,
		A11YCfg:   A11YCfg{A11YLabel: dat.Name},
		A11YState: a11yState,
		Color:     color,
		Padding:   listBoxItemPad,
		// No border in the Cfg: one there insets content, so every
		// row would grow by the ring's width whether or not it ever
		// paints one. The ring is stroked from AmendLayout instead,
		// which runs after arrange and therefore moves nothing.
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Sound:      rowSound,
		Content:    []View{content},
		// The ring paints only while the list itself holds window
		// focus. Focus is resolved after layout, so it cannot be
		// decided at generation; the row carries no ID of its own,
		// so the hook tests the list's effective ID.
		AmendLayout: listBoxItemRingAmend(
			isFocusRow, cfg.ID, cfg.ColorBorderFocus),
		OnClick: func(ctx EventCtx) {
			if hasOnSelect && !isSub {
				// Keyboard focus follows the click, so the
				// focus ring and the selection agree.
				listBoxFocusOnClick(listBoxID, focusDisabled,
					itemIDs, datID, ctx.Window)
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
	focusedID string,
	dragIdx int,
	itemIDs []string,
	itemLayoutIDs []string,
	midsOffset int,
	scrollID string,
) View {
	color := ColorTransparent
	// A reorderable row takes the same keyboard-focus ring as a plain
	// one, by the same rule.
	isFocusRow := listBoxIsFocusRow(dat, focusedID)
	selected := false
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		// Selection paints the subtle wash, not the full accent
		// slab; focus is the ring, not a second fill
		// (visual-refresh §4.3).
		color = cfg.ColorSelectSubtle
		selected = true
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
	focusDisabled := cfg.FocusDisabled
	onReorder := cfg.OnReorder

	a11yState := AccessStateNone
	if selected {
		a11yState = AccessStateSelected
	}

	// The cue marks the selection the click makes, not the drag it
	// also starts: a drag has no activation moment to sound at, and
	// dragging is phase 3's question (issue #467).
	rowSound := SoundNone
	if hasOnSelect {
		rowSound = resolveSoundCue(
			guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled)
	}

	return Row(ContainerCfg{
		ID:        layoutID,
		A11YRole:  AccessRoleListItem,
		A11YCfg:   A11YCfg{A11YLabel: dat.Name},
		A11YState: a11yState,
		Color:     color,
		Padding:   listBoxItemPad,
		// Stroked from AmendLayout, never reserved in the Cfg — a
		// border here would inset content and grow every row. See
		// listBoxItemRingAmend.
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Sound:      rowSound,
		Content:    []View{content},
		AmendLayout: listBoxItemRingAmend(
			isFocusRow, cfg.ID, cfg.ColorBorderFocus),
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
				// Keyboard focus follows the click, so the
				// focus ring and the selection agree.
				listBoxFocusOnClick(listBoxID, focusDisabled,
					itemIDs, datID, ctx.Window)
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

// listBoxItemContent builds the content view of one row: the
// subheading block, or the label with the style the row's fill
// calls for. The selected fill and the text on it are one decision
// (issue #373) — a selected row draws its label in the paired
// foreground — so the resolution lives here, next to the fill
// callers already compute.
// The wash selection tint needs no paired foreground: body text reads
// on the subtle fill, so the row's text stays its normal style.
func listBoxItemContent(dat ListBoxOption, cfg ListBoxCfg) View {
	if dat.isSubheading {
		return Column(ContainerCfg{
			Spacing: SomeF(1),
			Padding: NoPadding,
			Sizing:  FillFit,
			Content: []View{
				Text(TextCfg{
					Text:      dat.Name,
					TextStyle: cfg.SubheadingStyle,
				}),
				Separator(SeparatorCfg{
					Color: cfg.SubheadingStyle.Color,
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

// listBoxIsFocusRow reports whether this row is the list's keyboard
// focus row. Both row builders ask, so the rule is stated once.
//
// The dat.ID != "" term is load-bearing: focusedID is "" when no row
// holds focus, so without it every option in an ID-less list matches
// the sentinel and the list rings all of them. Subheadings are never
// candidates — keyboard navigation skips them, and the reorder path
// never builds one.
func listBoxIsFocusRow(dat ListBoxOption, focusedID string) bool {
	return dat.ID != "" && dat.ID == focusedID && !dat.isSubheading
}

// listBoxItemRingAmend returns the AmendLayout hook that strokes the
// keyboard-focus ring on the active row — the shared
// focusRingBorderAmend mechanism, keyed on the list's effective ID
// because a row carries no identity of its own.
//
// The stroke is applied from AmendLayout, which runs after arrange: a
// border in the Cfg would inset the row's content and add height to
// every row in the list. Stroked here, the ring traces the row's
// arranged edge and shifts nothing.
//
// It returns nil for every other row, so only the one focused row in
// the list pays for a hook. A disabled row never shows focus — it
// cannot hold it, and the dimmed text already reads as inert.
func listBoxItemRingAmend(
	isFocusRow bool, listID string, colorRing Color,
) func(EventCtx) {
	if !isFocusRow {
		return nil
	}
	return focusRingBorderAmend(listID, focusRingBorderWidth, colorRing)
}
