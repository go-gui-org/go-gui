package gui

func treeCollectFlatRows(
	nodes []TreeNodeCfg,
	expanded map[string]bool,
	treeID string,
	lazyState *BoundedMap[string, bool],
	out *[]treeFlatRow,
	depth int,
	parentID string,
) {
	for i := range nodes {
		node := nodes[i]
		nodeID := treeNodeID(node)
		isExpanded := expanded[nodeID]
		hasRealChildren := len(node.Nodes) > 0
		hasChildren := hasRealChildren || node.Lazy
		lazyKey := treeLazyKey(treeID, nodeID)
		// Default false: absent entry means node is not loading.
		isLoading := lazyState.GetOr(lazyKey, false)

		if node.Lazy && hasRealChildren && isLoading {
			lazyState.Delete(lazyKey)
			isLoading = false
		}

		*out = append(*out, treeFlatRow{
			ID:              nodeID,
			ParentID:        parentID,
			Depth:           depth,
			Text:            node.Text,
			Icon:            node.Icon,
			TextStyle:       treeNodeTextStyle(node),
			textStyleIcon:   treeNodeTextStyleIcon(node),
			HasChildren:     hasChildren,
			HasRealChildren: hasRealChildren,
			IsLazy:          node.Lazy,
			IsExpanded:      isExpanded,
			IsLoading:       false,
		})

		if !isExpanded {
			continue
		}
		if hasRealChildren {
			treeCollectFlatRows(
				node.Nodes, expanded, treeID, lazyState, out, depth+1, nodeID)
			continue
		}
		if node.Lazy && isLoading {
			*out = append(*out, treeFlatRow{
				ID:            nodeID + treeLoadingSuffix,
				ParentID:      nodeID,
				Depth:         depth + 1,
				Text:          activeLocaleShared().StrLoading,
				TextStyle:     defaultTreeStyle.TextStyle,
				textStyleIcon: defaultTreeStyle.textStyleIcon,
				IsLoading:     true,
			})
		}
	}
}

func treeEstimateRowHeight(cfg TreeCfg, w *Window) float32 {
	style := defaultTreeStyle.TextStyle
	if len(cfg.Nodes) > 0 {
		style = treeNodeTextStyle(cfg.Nodes[0])
	}
	height := style.Size
	if w != nil && w.textMeasurer != nil {
		height = w.textMeasurer.FontHeight(style)
	}
	return height + PaddingTwoFive.Height() + cfg.Spacing.Or(defaultTreeStyle.Spacing)
}

func treeVisibleRange(
	treeHeight, rowHeight float32,
	totalRows int,
	scrollID string,
	w *Window,
) (int, int) {
	if w == nil {
		return 0, totalRows - 1
	}
	// Default 0: unscrolled position when no offset recorded yet.
	scrollY := w.scrollY().GetOr(scrollID, 0)
	// Index space: the *flat* row index, not the node index. Re-
	// registered every frame because flatRows is rebuilt whenever a
	// node expands or collapses.
	listHeightRegisterUniform(w, scrollID, totalRows, rowHeight, 0, 0)
	return listCoreVisibleRange(totalRows, rowHeight, treeHeight, scrollY)
}

func treeNodeID(node TreeNodeCfg) string {
	if node.ID != "" {
		return node.ID
	}
	return node.Text
}

func treeNodeTextStyle(node TreeNodeCfg) TextStyle {
	if node.TextStyle != (TextStyle{}) {
		return node.TextStyle
	}
	return defaultTreeStyle.TextStyle
}

func treeNodeTextStyleIcon(node TreeNodeCfg) TextStyle {
	if node.TextStyleIcon != (TextStyle{}) {
		return node.TextStyleIcon
	}
	return defaultTreeStyle.textStyleIcon
}

func treeIconWidth(cfg *TreeCfg, w *Window) float32 {
	style := defaultTreeStyle.textStyleIcon
	if len(cfg.Nodes) > 0 {
		style = treeNodeTextStyleIcon(cfg.Nodes[0])
	}
	if w != nil && w.textMeasurer != nil {
		return max(
			w.textMeasurer.TextWidth(IconDropDown+" ", style),
			style.Size+4,
		)
	}
	return style.Size + 4
}

// treeArrowIcon is the row's expand arrow, with the trailing space
// that separates it from the icon. The space is part of each constant,
// not appended per row: a concatenation allocated a string per
// expandable row per frame.
func treeArrowIcon(row treeFlatRow) string {
	if !row.HasChildren {
		return "  "
	}
	if row.IsExpanded {
		return IconDropDown + " "
	}
	if activeLocaleShared().TextDir == TextDirRTL {
		return IconDropLeft + " "
	}
	return IconDropRight + " "
}

// treeRowSound resolves the cue one row's activation emits. A row with
// children opens or closes, so the cue names what the click will do; a
// leaf row picks one node out of the tree, which is a selection
// (issue #467).
func treeRowSound(cfg *TreeCfg, row treeFlatRow) SoundCue {
	themeCue := guiTheme.Sounds.Selection
	if row.HasChildren {
		themeCue = guiTheme.Sounds.ToggleOn
		if row.IsExpanded {
			themeCue = guiTheme.Sounds.ToggleOff
		}
	}
	return resolveSoundCue(themeCue, cfg.Sound, cfg.SoundDisabled)
}

// treeClick is the click state every plain row of one Tree shares:
// what the per-row OnClick closure used to capture.
type treeClick struct {
	treeID     string
	onSelect   func(string, EventCtx)
	onLazyLoad func(string, string, *Window)
}

// newTreeClick builds the click state of one tree. cfg.ID is the
// tree's effective ID.
func newTreeClick(cfg *TreeCfg) *treeClick {
	return &treeClick{
		treeID:     cfg.ID,
		onSelect:   cfg.OnSelect,
		onLazyLoad: cfg.OnLazyLoad,
	}
}

// rowClick focuses the row, toggles it if it has children, and
// reports the selection.
func (t *treeClick) rowClick(rc *rowClick, ctx EventCtx) {
	treeRowClick(t.treeID, *rc.tree, t.treeID, t.onSelect,
		t.onLazyLoad, ctx.Event, ctx.Window)
}

// treeDragClick is the click state every reorderable row of one Tree
// shares. A row's drag data depends on its parent, so the per-parent
// maps live here and are read at click time. treeBuild makes them
// fresh each frame and changes nothing after the rows are built, so a
// click reads what the row was built with. rowClick.index is the
// row's sibling index; rowClick.cue its drop cue.
type treeDragClick struct {
	sel              treeClick
	onReorder        func(string, string, EventCtx)
	parentOf         map[string]string
	siblingsByParent map[string][]string
	parentLayoutIDs  map[string][]string
	parentMidsOff    map[string]int
	scrollID         string
}

// rowClick arms the drag among the row's siblings, then does what a
// plain row's click does.
func (t *treeDragClick) rowClick(rc *rowClick, ctx EventCtx) {
	parent := t.parentOf[rc.id]
	dragReorderStart(dragReorderStartCfg{
		DragKey:       t.sel.treeID,
		DropCue:       rc.cue,
		Index:         rc.index,
		ItemID:        rc.id,
		Axis:          dragReorderVertical,
		ItemIDs:       t.siblingsByParent[parent],
		OnReorder:     t.onReorder,
		ItemLayoutIDs: t.parentLayoutIDs[parent],
		MidsOffset:    t.parentMidsOff[parent],
		ScrollID:      t.scrollID,
		Layout:        ctx.Layout,
		Event:         ctx.Event,
	}, ctx.Window)
	t.sel.rowClick(rc, ctx)
}

// treeRowView builds one plain row. click is the row's target in the
// tree's treeClick; nil only for a loading placeholder, which answers
// no click.
func treeRowView(
	w *Window,
	cfg TreeCfg,
	row treeFlatRow,
	iconWidth float32,
	focusedID string,
	click *rowClick,
) View {
	if row.IsLoading {
		return Row(ContainerCfg{
			Padding: NewPadding(
				2, 5, 2,
				float32(row.Depth)*cfg.Indent+5,
			),
			Sizing: FillFit,
			Content: []View{
				Text(TextCfg{
					Text:      row.Text,
					TextStyle: row.TextStyle,
				}),
			},
		})
	}

	isFocused := focusedID == row.ID
	rowColor := ColorTransparent
	if isFocused {
		rowColor = cfg.Colors.Focus
	}
	a11yState := AccessStateNone
	if row.IsExpanded && row.HasChildren {
		a11yState = AccessStateExpanded
	}

	return Row(ContainerCfg{
		A11YRole:  AccessRoleTreeItem,
		A11YCfg:   A11YCfg{A11YLabel: row.Text},
		A11YState: a11yState,
		Color:     rowColor,
		Radius:    RadiusPx(cfg.Radius.Or(defaultTreeStyle.Radius)),
		Padding: NewPadding(
			2, 5, 2,
			float32(row.Depth)*cfg.Indent+5,
		),
		Sizing:  FillFit,
		Spacing: NoSpacing,
		Sound:   treeRowSound(&cfg, row),
		Content: treeRowContentViews(w, row, iconWidth),
		// The shared handler reads the row back through click
		// (treeClick.rowClick).
		OnClick:  rowOnClick,
		rowClick: click,
		// The fill follows the pointer even on the focused row,
		// matching ColorSet.pick's rule (#690). gui paints the style, so
		// no OnHover closure is allocated per row per frame (#977).
		Hover: HoverStyle{Color: cfg.Colors.Hover, Cursor: CursorPointingHand},
	})
}

// treeDragRowView builds one reorderable row. click is the row's
// target in the tree's treeDragClick; a loading placeholder builds as
// a plain one and ignores it. layoutID is the row's layout ID
// (treeDragLayoutID).
func treeDragRowView(
	w *Window,
	cfg TreeCfg,
	row treeFlatRow,
	iconWidth float32,
	focusedID string,
	click *rowClick,
	layoutID string,
) View {
	if row.IsLoading {
		return treeRowView(w, cfg, row, iconWidth, focusedID, nil)
	}

	isFocused := focusedID == row.ID
	rowColor := ColorTransparent
	if isFocused {
		rowColor = cfg.Colors.Focus
	}
	a11yState := AccessStateNone
	if row.IsExpanded && row.HasChildren {
		a11yState = AccessStateExpanded
	}

	return Row(ContainerCfg{
		ID:        layoutID,
		A11YRole:  AccessRoleTreeItem,
		A11YCfg:   A11YCfg{A11YLabel: row.Text},
		A11YState: a11yState,
		Color:     rowColor,
		Radius:    RadiusPx(cfg.Radius.Or(defaultTreeStyle.Radius)),
		Padding: NewPadding(
			2, 5, 2,
			float32(row.Depth)*cfg.Indent+5,
		),
		Sizing:  FillFit,
		Spacing: NoSpacing,
		// Same cue as the plain row: the drag this click also starts
		// has no activation moment to sound at (issue #467). The drop
		// cue is the same sound, carried on click.
		Sound:   click.cue,
		Content: treeRowContentViews(w, row, iconWidth),
		// Arms the drag, then acts as a plain row
		// (treeDragClick.rowClick).
		OnClick:  rowOnClick,
		rowClick: click,
		// The fill follows the pointer even on the focused row,
		// matching ColorSet.pick's rule (#690). gui paints the style, so
		// no OnHover closure is allocated per row per frame (#977).
		Hover: HoverStyle{Color: cfg.Colors.Hover, Cursor: CursorPointingHand},
	})
}

// treeRowContent returns the inner content of a tree row without
// the Row container — used for the drag ghost.
func treeRowContent(
	w *Window,
	cfg TreeCfg,
	row treeFlatRow,
	iconWidth float32,
	focusedID string,
) View {
	rowColor := ColorTransparent
	if focusedID == row.ID {
		rowColor = cfg.Colors.Focus
	}
	return Row(ContainerCfg{
		Color:  rowColor,
		Radius: RadiusPx(cfg.Radius.Or(defaultTreeStyle.Radius)),
		Padding: NewPadding(
			2, 5, 2,
			float32(row.Depth)*cfg.Indent+5,
		),
		Sizing:  FillFit,
		Spacing: NoSpacing,
		Content: treeRowContentViews(w, row, iconWidth),
	})
}

// treeRowContentViews is a row's arrow, icon and label. The slice comes
// from the frame arena (rowContent).
func treeRowContentViews(w *Window, row treeFlatRow, iconWidth float32) []View {
	return append(rowContent(w, 3),
		Text(TextCfg{
			Text:      treeArrowIcon(row),
			MinWidth:  iconWidth,
			TextStyle: row.textStyleIcon,
		}),
		Text(TextCfg{
			Text:      treeIconText(row.Icon),
			MinWidth:  iconWidth,
			TextStyle: row.textStyleIcon,
		}),
		Text(TextCfg{
			Text:      row.Text,
			TextStyle: row.TextStyle,
		}),
	)
}

func treeSiblingIndex(siblings []string, id string) int {
	for i, s := range siblings {
		if s == id {
			return i
		}
	}
	return -1
}

func treeIconText(icon string) string {
	if icon == "" {
		return " "
	}
	return icon + " "
}

func treeRowClick(
	treeID string,
	row treeFlatRow,
	rootFocusID string,
	onSelect func(string, EventCtx),
	onLazyLoad func(string, string, *Window),
	e *Event,
	w *Window,
) {
	treeFocusedSet(w, treeID, row.ID)
	if rootFocusID != "" {
		w.SetFocus(rootFocusID)
	}
	if row.HasChildren {
		nextExpanded := !row.IsExpanded
		treeExpandedSet(w, treeID, row.ID, nextExpanded)
		if nextExpanded && row.IsLazy && !row.HasRealChildren {
			treeTryLazyLoad(treeID, row.ID, onLazyLoad, w)
		}
		if !nextExpanded {
			treeClearLoading(treeID, row.ID, w)
		}
	}
	if onSelect != nil {
		onSelect(row.ID, EventCtx{nil, e, w})
	}
	e.IsHandled = true
}

func treeOnKeyDown(
	treeID string,
	visibleIDs []string,
	rowByID map[string]*treeFlatRow,
	onSelect func(string, EventCtx),
	onLazyLoad func(string, string, *Window),
	scrollID string,
	rowHeight float32,
	listHeight float32,
	cues soundCues,
	e *Event,
	w *Window,
) {
	if e.Modifiers != ModNone || len(visibleIDs) == 0 {
		return
	}
	focusedID := StateReadOr(w, nsTreeFocus, treeID, "")
	cur := treeFocusedIndex(visibleIDs, focusedID)
	focusMap := StateMap[string, string](w, nsTreeFocus, capModerate)

	switch e.KeyCode {
	case KeyUp:
		next := 0
		if cur > 0 {
			next = cur - 1
		}
		if next == cur {
			// Already on the first row. Movement is silent by
			// decision; only the refusal sounds (issue #468).
			playSoundCue(cues.reject, w)
		}
		focusMap.Set(treeID, visibleIDs[next])
		treeScrollTo(scrollID, next, rowHeight, listHeight, w)
		e.IsHandled = true
	case KeyDown:
		next := 0
		if cur >= 0 {
			next = min(cur+1, len(visibleIDs)-1)
		}
		if next == cur {
			playSoundCue(cues.reject, w)
		}
		focusMap.Set(treeID, visibleIDs[next])
		treeScrollTo(scrollID, next, rowHeight, listHeight, w)
		e.IsHandled = true
	case KeyHome:
		focusMap.Set(treeID, visibleIDs[0])
		treeScrollTo(scrollID, 0, rowHeight, listHeight, w)
		e.IsHandled = true
	case KeyEnd:
		last := len(visibleIDs) - 1
		focusMap.Set(treeID, visibleIDs[last])
		treeScrollTo(scrollID, last, rowHeight, listHeight, w)
		e.IsHandled = true
	case KeyLeft:
		if cur < 0 {
			return
		}
		row, ok := rowByID[focusedID]
		if !ok {
			return
		}
		if row.HasChildren && row.IsExpanded {
			treeExpandedSet(w, treeID, focusedID, false)
			treeClearLoading(treeID, focusedID, w)
			e.IsHandled = true
			return
		}
		if row.ParentID != "" {
			pi := treeFocusedIndex(visibleIDs, row.ParentID)
			if pi >= 0 {
				focusMap.Set(treeID, row.ParentID)
				treeScrollTo(scrollID, pi, rowHeight, listHeight, w)
				e.IsHandled = true
			}
		}
	case KeyRight:
		if cur < 0 {
			return
		}
		row, ok := rowByID[focusedID]
		if !ok || !row.HasChildren || row.IsExpanded {
			return
		}
		treeExpandedSet(w, treeID, focusedID, true)
		if row.IsLazy && !row.HasRealChildren {
			treeTryLazyLoad(treeID, focusedID, onLazyLoad, w)
		}
		e.IsHandled = true
	case KeyEnter, KeySpace:
		if cur < 0 {
			return
		}
		// Keyboard activation, with a nil Layout: dispatch never sees
		// it, so the row's own cue cannot fire (issue #468).
		playSoundCue(cues.act, w)
		if onSelect != nil {
			onSelect(focusedID, EventCtx{nil, e, w})
		}
		e.IsHandled = true
	}
}

func treeFocusedIndex(visibleIDs []string, focusedID string) int {
	for i := range visibleIDs {
		if visibleIDs[i] == focusedID {
			return i
		}
	}
	return -1
}

func treeTryLazyLoad(
	treeID, nodeID string,
	onLazyLoad func(string, string, *Window),
	w *Window,
) {
	if onLazyLoad == nil {
		return
	}
	key := treeLazyKey(treeID, nodeID)
	lazyState := StateMap[string, bool](w, nsTreeLazy, capMany)
	if loading, ok := lazyState.Get(key); ok && loading {
		return
	}
	lazyState.Set(key, true)
	onLazyLoad(treeID, nodeID, w)
}

func treeClearLoading(treeID, nodeID string, w *Window) {
	key := treeLazyKey(treeID, nodeID)
	lazyState := StateMap[string, bool](w, nsTreeLazy, capMany)
	lazyState.Delete(key)
}

func treeScrollTo(
	scrollID string, idx int, rowH, listH float32, w *Window,
) {
	if scrollID != "" && rowH > 0 {
		scrollEnsureVisible(scrollID, idx, rowH, listH, w)
	}
}
