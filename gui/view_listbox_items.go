package gui

import "slices"

// listBoxItemLayoutIDs builds layout IDs and midsOffset for
// drag-reorder tracking. itemDataIndices is the cache's ascending
// data indices of the draggable rows, so the count of rows above
// the window is a binary search, not a scan of every row before it.
func listBoxItemLayoutIDs(
	cfg *ListBoxCfg, canReorder bool, first, last int,
	itemDataIndices []int,
) ([]string, int) {
	if !canReorder {
		return nil, 0
	}
	itemLayoutIDs := make([]string, 0, last-first+1)
	midsOffset, _ := slices.BinarySearch(itemDataIndices, first)
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
	w *Window,
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

	// The rows' click targets (row_click.go), one slice for the whole
	// list. listCap bounds the rows, so the slice never grows. A plain
	// row answers a click only when the list has an OnSelect; a
	// reorderable row always does, because the click also starts the
	// drag.
	var clicks []rowClick
	var plain *listBoxClick
	var reorder *listBoxReorderClick
	if cfg.OnSelect != nil || canReorder {
		clicks = rowClicks(listCap)
	}
	if cfg.OnSelect != nil {
		plain = newListBoxClick(cfg, itemIDs)
	}
	if canReorder {
		reorder = newListBoxReorderClick(cfg, itemIDs, itemLayoutIDs,
			midsOffset, scrollID)
	}

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
			click := addRowClick(&clicks, rowClick{
				owner: reorder, id: cfg.Data[idx].ID, index: di})
			list = append(list, listBoxReorderItemView(w,
				cfg.Data[idx], *cfg, selectedSet, focusedID, click,
				listBoxReorderLayoutID(cfg, itemLayoutIDs, midsOffset,
					di, cfg.Data[idx].ID)))
		} else {
			var click *rowClick
			if plain != nil && !cfg.Data[idx].isSubheading {
				click = addRowClick(&clicks, rowClick{
					owner: plain, id: cfg.Data[idx].ID})
			}
			list = append(list,
				listBoxItemView(w,
					cfg.Data[idx], *cfg, selectedSet,
					focusedID, click))
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

// listBoxClick is the click state every plain row of one ListBox
// shares: what the per-row OnClick closure used to capture.
type listBoxClick struct {
	listBoxID     string
	focusDisabled bool
	isMultiple    bool
	onSelect      func([]string, EventCtx)
	selectedIDs   []string
	itemIDs       []string
}

// newListBoxClick builds the select state of one list. itemIDs are the
// list's selectable IDs, in order.
func newListBoxClick(cfg *ListBoxCfg, itemIDs []string) *listBoxClick {
	return &listBoxClick{
		listBoxID:     cfg.ID,
		focusDisabled: cfg.FocusDisabled,
		isMultiple:    cfg.Multiple,
		onSelect:      cfg.OnSelect,
		selectedIDs:   cfg.SelectedIDs,
		itemIDs:       itemIDs,
	}
}

// rowClick selects the row. Keyboard focus follows the click, so the
// focus ring and the selection agree.
func (l *listBoxClick) rowClick(rc *rowClick, ctx EventCtx) {
	listBoxFocusOnClick(l.listBoxID, l.focusDisabled,
		l.itemIDs, rc.id, ctx.Window)
	ids := listBoxNextSelectedIDs(l.selectedIDs, rc.id, l.isMultiple)
	l.onSelect(ids, EventCtx{nil, ctx.Event, ctx.Window})
}

// listBoxReorderClick is the click state every reorderable row of one
// ListBox shares. A row's rowClick.index is its drag index.
type listBoxReorderClick struct {
	sel           listBoxClick
	dropCue       SoundCue
	onReorder     func(string, string, EventCtx)
	itemLayoutIDs []string
	midsOffset    int
	scrollID      string
}

// newListBoxReorderClick builds the drag and select state of one
// reorderable list.
func newListBoxReorderClick(
	cfg *ListBoxCfg, itemIDs, itemLayoutIDs []string,
	midsOffset int, scrollID string,
) *listBoxReorderClick {
	return &listBoxReorderClick{
		sel: *newListBoxClick(cfg, itemIDs),
		// The drop cue is resolved unconditionally: a reorder is a
		// change whether or not the list also reports selection
		// (issue #468).
		dropCue: resolveSoundCue(
			guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled),
		onReorder:     cfg.OnReorder,
		itemLayoutIDs: itemLayoutIDs,
		midsOffset:    midsOffset,
		scrollID:      scrollID,
	}
}

// rowClick arms the drag, then selects the row when the list has an
// OnSelect.
func (l *listBoxReorderClick) rowClick(rc *rowClick, ctx EventCtx) {
	dragReorderStart(dragReorderStartCfg{
		DragKey:       l.sel.listBoxID,
		DropCue:       l.dropCue,
		Index:         rc.index,
		ItemID:        rc.id,
		Axis:          dragReorderVertical,
		ItemIDs:       l.sel.itemIDs,
		OnReorder:     l.onReorder,
		ItemLayoutIDs: l.itemLayoutIDs,
		MidsOffset:    l.midsOffset,
		ScrollID:      l.scrollID,
		Layout:        ctx.Layout,
		Event:         ctx.Event,
	}, ctx.Window)
	if l.sel.onSelect != nil {
		l.sel.rowClick(rc, ctx)
	}
}

// listBoxItemView builds one plain row. click is nil for a row that
// answers no click: a subheading, or any row of a list with no
// OnSelect.
func listBoxItemView(
	w *Window,
	dat ListBoxOption,
	cfg ListBoxCfg,
	selectedSet map[string]struct{},
	focusedID string,
	click *rowClick,
) View {
	isFocusRow := listBoxIsFocusRow(dat, focusedID)
	selected := listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID)
	// The row fill goes through pick, so a selected row hovers one
	// OKLCH step lighter instead of ignoring the pointer (#744).
	// The keyboard-focus row still takes a ring, never a fill: a
	// fill here is the same grey the mouse hover paints, so the two
	// cursors were indistinguishable (visual-refresh §4.3).
	bg, bgHover := rowFill(cfg.Colors, ColorTransparent, selected,
		false)
	content := listBoxItemContent(dat, cfg)

	// A row answers a click only when it has a click target: a
	// subheading is a caption, not a choice, and a list with no
	// OnSelect answers nothing (see listBoxBuildItems).
	answers := click != nil

	a11yState := AccessStateNone
	if selected {
		a11yState = AccessStateSelected
	}

	// A row that answers nothing stays silent. Otherwise the row picks
	// one of several, which is the selection role (issue #467).
	rowSound := SoundNone
	if answers {
		rowSound = resolveSoundCue(
			guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled)
	}

	// The same rule decides the hover look. A HoverStyle, not an
	// OnHover closure: the look depends only on the row, so gui paints
	// it and no closure is allocated per row per frame (#977).
	var hover HoverStyle
	var onClick func(EventCtx)
	if answers {
		hover = HoverStyle{Color: bgHover, Cursor: CursorPointingHand}
		onClick = rowOnClick
	}

	return Row(ContainerCfg{
		A11YRole:  AccessRoleListItem,
		A11YCfg:   A11YCfg{A11YLabel: dat.Name},
		A11YState: a11yState,
		Color:     bg,
		Padding:   listBoxItemPad,
		// No border in the Cfg: one there insets content, so every
		// row would grow by the ring's width whether or not it ever
		// paints one. The ring is stroked from AmendLayout instead,
		// which runs after arrange and therefore moves nothing.
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Sound:      rowSound,
		Content:    append(rowContent(w, 1), content),
		// The ring paints only while the list itself holds window
		// focus. Focus is resolved after layout, so it cannot be
		// decided at generation; the row carries no ID of its own,
		// so the hook tests the list's effective ID.
		AmendLayout: listBoxItemRingAmend(
			isFocusRow, cfg.ID, cfg.Colors.BorderFocus),
		OnClick:  onClick,
		rowClick: click,
		Hover:    hover,
	})
}

// listBoxReorderLayoutID is the layout ID of the reorderable row with
// drag index di. listBoxItemLayoutIDs already composed it for drag
// tracking: entry k is the k-th draggable row of the window, and
// midsOffset counts the draggable rows above the window, so the row's
// entry is di - midsOffset. Reusing it saves a ScopeID string per row
// per frame. Out of range falls back to composing it.
func listBoxReorderLayoutID(
	cfg *ListBoxCfg, itemLayoutIDs []string, midsOffset, di int,
	id string,
) string {
	if k := di - midsOffset; k >= 0 && k < len(itemLayoutIDs) {
		return itemLayoutIDs[k]
	}
	return listBoxItemID(cfg.ID, id)
}

// listBoxReorderItemView builds one reorderable row. click is its
// target in the list's listBoxReorderClick; never nil. layoutID is
// the row's layout ID (listBoxReorderLayoutID).
func listBoxReorderItemView(
	w *Window,
	dat ListBoxOption,
	cfg ListBoxCfg,
	selectedSet map[string]struct{},
	focusedID string,
	click *rowClick,
	layoutID string,
) View {
	// A reorderable row takes the same keyboard-focus ring as a plain
	// one, by the same rule.
	isFocusRow := listBoxIsFocusRow(dat, focusedID)
	selected := listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID)
	// Same pick routing as the plain row above: a selected row
	// hovers one OKLCH step lighter (#744).
	bg, bgHover := rowFill(cfg.Colors, ColorTransparent, selected,
		false)
	content := listBoxItemContent(dat, cfg)

	a11yState := AccessStateNone
	if selected {
		a11yState = AccessStateSelected
	}

	// The cue marks the selection the click makes, not the drag it
	// also starts: a drag has no activation moment to sound at, and
	// dragging is phase 3's question (issue #467).
	rowSound := SoundNone
	// Matches the plain row: a list with no OnSelect answers nothing,
	// so the row stays silent and takes no hover look. Subheadings
	// never reach this view — without a drag index they build as
	// plain rows.
	var hover HoverStyle
	if cfg.OnSelect != nil {
		rowSound = resolveSoundCue(
			guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled)
		hover = HoverStyle{Color: bgHover, Cursor: CursorPointingHand}
	}

	return Row(ContainerCfg{
		ID:        layoutID,
		A11YRole:  AccessRoleListItem,
		A11YCfg:   A11YCfg{A11YLabel: dat.Name},
		A11YState: a11yState,
		Color:     bg,
		Padding:   listBoxItemPad,
		// Stroked from AmendLayout, never reserved in the Cfg — a
		// border here would inset content and grow every row. See
		// listBoxItemRingAmend.
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Sound:      rowSound,
		Content:    append(rowContent(w, 1), content),
		AmendLayout: listBoxItemRingAmend(
			isFocusRow, cfg.ID, cfg.Colors.BorderFocus),
		// Arms the drag, then selects (listBoxReorderClick.rowClick).
		OnClick:  rowOnClick,
		rowClick: click,
		Hover:    hover,
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
			Spacing: SpacingPx(1), // ergonomics-audit:spacing — 1px hairline, not a gap
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
		Text: dat.Name,
		// Single line, like every other uniform-height row
		// (combobox, select, menu): a wrapped row would arrange
		// taller than the rowH the spacers and the scroll model
		// assume, drifting every position below it.
		Mode:      TextModeSingleLine,
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
