package gui

const treeLoadingSuffix = ".__loading__"

// TreeCfg configures a tree view.
type TreeCfg struct {
	OnSelect   func(string, *Event, *Window)
	OnLazyLoad func(string, string, *Window)

	OnReorder func(movedID, beforeID string, w *Window)

	ID string `gui:"required"`

	A11YLabel       string
	A11YDescription string

	Nodes      []TreeNodeCfg
	Padding    Opt[Padding]
	SizeBorder Opt[float32]
	Radius     Opt[float32]

	Indent  float32
	Spacing float32

	IDFocus  uint32
	IDScroll uint32

	Width     float32
	Height    float32
	MinWidth  float32
	MaxWidth  float32
	MinHeight float32
	MaxHeight float32

	Color       Color
	ColorHover  Color
	ColorFocus  Color
	ColorBorder Color

	Sizing Sizing

	Disabled    bool
	Invisible   bool
	Reorderable bool
}

// TreeNodeCfg configures a single tree node.
type TreeNodeCfg struct {
	TextStyle     TextStyle
	TextStyleIcon TextStyle
	ID            string
	Text          string
	Icon          string
	Nodes         []TreeNodeCfg
	Lazy          bool
}

type treeView struct {
	cfg TreeCfg
}

type treeFlatRow struct {
	TextStyle       TextStyle
	TextStyleIcon   TextStyle
	ID              string
	ParentID        string
	Text            string
	Icon            string
	Depth           int
	HasChildren     bool
	HasRealChildren bool
	IsLazy          bool
	IsExpanded      bool
	IsLoading       bool
}

// Tree creates a tree view with optional virtualization and lazy loading.
func Tree(cfg TreeCfg) View {
	requireID("Tree", cfg.ID)
	applyTreeDefaults(&cfg)
	return &treeView{cfg: cfg}
}

func (tv *treeView) Content() []View { return nil }

func (tv *treeView) GenerateLayout(w *Window) Layout {
	cfg := &tv.cfg

	expanded := treeExpandedState(w, cfg.ID)
	lazyState := StateMap[string, bool](w, nsTreeLazy, capMany)

	flatRows := make([]treeFlatRow, 0, max(8, len(cfg.Nodes)*2))
	treeCollectFlatRows(
		cfg.Nodes, expanded, cfg.ID, lazyState, &flatRows, 0, "")

	visibleIDs, rowByID := treeFlatRowIndex(flatRows)

	listHeight := cfg.Height
	if listHeight <= 0 {
		listHeight = cfg.MaxHeight
	}
	virtualize := cfg.IDScroll > 0 && listHeight > 0 && len(flatRows) > 0
	rowHeight := float32(0)
	first, last := 0, len(flatRows)-1
	if virtualize {
		rowHeight = treeEstimateRowHeight(*cfg, w)
		first, last = treeVisibleRange(
			listHeight, rowHeight, len(flatRows), cfg.IDScroll, w)
	}

	focusedID := StateReadOr(w, nsTreeFocus, cfg.ID, "")
	iconWidth := treeIconWidth(cfg, w)

	canReorder := cfg.Reorderable && cfg.OnReorder != nil
	onReorder := cfg.OnReorder
	idScroll := cfg.IDScroll

	parentOf, siblingsByParent := treeBuildParentMaps(
		flatRows, visibleIDs, canReorder)
	siblingIdx, parentLayoutIDs, parentMidsOff :=
		treeBuildSiblingInfo(cfg.ID, flatRows,
			siblingsByParent, canReorder, first, last)

	var drag dragReorderState
	var dragging bool
	var dragParent string
	if canReorder {
		drag = dragReorderGet(w, cfg.ID)
		dragging = drag.active && !drag.cancelled
		if drag.started || drag.active {
			dragParent = parentOf[drag.itemID]
			dragReorderIDsMetaSet(w, cfg.ID,
				siblingsByParent[dragParent])
		}
	}

	rows, ghostContent := treeBuildRows(
		cfg, flatRows, focusedID, iconWidth,
		parentOf, siblingsByParent, siblingIdx,
		parentLayoutIDs, parentMidsOff, idScroll,
		canReorder, dragging, drag, dragParent,
		virtualize, rowHeight, first, last)

	if dragging {
		dragSibs := siblingsByParent[dragParent]
		if drag.currentIndex >= len(dragSibs) {
			rows = append(rows,
				dragReorderGapView(drag, DragReorderVertical))
		}
	}
	if dragging && ghostContent != nil {
		rows = append(rows,
			dragReorderGhostView(drag, ghostContent))
	}

	sizeBorder := cfg.SizeBorder.Get(DefaultTreeStyle.SizeBorder)
	radius := cfg.Radius.Get(DefaultTreeStyle.Radius)

	return GenerateViewLayout(Column(ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleTree,
		A11YLabel: a11yLabel(cfg.A11YLabel, cfg.ID),
		A11Y:      makeA11YInfo(a11yLabel(cfg.A11YLabel, cfg.ID), cfg.A11YDescription),
		IDFocus:   cfg.IDFocus,
		IDScroll:  cfg.IDScroll,
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			if canReorder {
				if dragReorderEscape(cfg.ID, e.KeyCode, w) {
					e.IsHandled = true
					return
				}
				if e.Modifiers.Has(ModAlt) {
					fid := StateReadOr(
						w, nsTreeFocus, cfg.ID, "")
					if fid != "" {
						fp := parentOf[fid]
						sibs := siblingsByParent[fp]
						si := treeSiblingIndex(sibs, fid)
						if si >= 0 &&
							dragReorderKeyboardMove(
								e.KeyCode, e.Modifiers,
								DragReorderVertical,
								si, sibs, onReorder, w) {
							e.IsHandled = true
							return
						}
					}
				}
			}
			treeOnKeyDown(cfg.ID, visibleIDs, rowByID,
				cfg.OnSelect, cfg.OnLazyLoad,
				idScroll, rowHeight, listHeight, e, w)
		},
		Sizing:      cfg.Sizing,
		Width:       cfg.Width,
		Height:      cfg.Height,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		MinHeight:   cfg.MinHeight,
		MaxHeight:   cfg.MaxHeight,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		Padding:     cfg.Padding,
		Spacing:     Some(cfg.Spacing),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Content:     rows,
	}), w)
}

// treeFlatRowIndex builds the visible ID list and a lookup map
// from flat rows, skipping loading placeholders.
func treeFlatRowIndex(
	flatRows []treeFlatRow,
) ([]string, map[string]treeFlatRow) {
	visibleIDs := make([]string, 0, len(flatRows))
	rowByID := make(map[string]treeFlatRow, len(flatRows))
	for i := range flatRows {
		row := flatRows[i]
		if row.IsLoading {
			continue
		}
		visibleIDs = append(visibleIDs, row.ID)
		rowByID[row.ID] = row
	}
	return visibleIDs, rowByID
}

// treeBuildParentMaps builds parentOf and siblingsByParent maps
// for drag-reorder scoping.
func treeBuildParentMaps(
	flatRows []treeFlatRow, visibleIDs []string, canReorder bool,
) (map[string]string, map[string][]string) {
	if !canReorder {
		return nil, nil
	}
	parentOf := make(map[string]string, len(visibleIDs))
	siblingsByParent := make(map[string][]string)
	for i := range flatRows {
		row := flatRows[i]
		if row.IsLoading {
			continue
		}
		parentOf[row.ID] = row.ParentID
		siblingsByParent[row.ParentID] = append(
			siblingsByParent[row.ParentID], row.ID)
	}
	return parentOf, siblingsByParent
}

// treeBuildSiblingInfo builds sibling index, per-parent layout
// IDs, and per-parent midsOffset for drag-reorder tracking.
func treeBuildSiblingInfo(
	treeID string, flatRows []treeFlatRow,
	siblingsByParent map[string][]string,
	canReorder bool, first, last int,
) (map[string]int, map[string][]string, map[string]int) {
	if !canReorder {
		return nil, nil, nil
	}
	siblingIdx := make(map[string]int)
	parentLayoutIDs := make(map[string][]string)
	parentMidsOff := make(map[string]int)

	flatIdxOf := make(map[string]int, len(flatRows))
	for i := range flatRows {
		if !flatRows[i].IsLoading {
			flatIdxOf[flatRows[i].ID] = i
		}
	}
	for pid, sibs := range siblingsByParent {
		moff := 0
		var lids []string
		for si, sid := range sibs {
			siblingIdx[sid] = si
			fi, ok := flatIdxOf[sid]
			if !ok {
				continue
			}
			if fi < first {
				moff++
			} else if fi <= last {
				lids = append(lids,
					"tr_"+treeID+"_"+sid)
			}
		}
		parentLayoutIDs[pid] = lids
		parentMidsOff[pid] = moff
	}
	return siblingIdx, parentLayoutIDs, parentMidsOff
}

// treeBuildRows builds the list of row views, including
// virtualization spacers and drag-reorder gap/ghost handling.
func treeBuildRows(
	cfg *TreeCfg,
	flatRows []treeFlatRow,
	focusedID string, iconWidth float32,
	parentOf map[string]string,
	siblingsByParent map[string][]string,
	siblingIdx map[string]int,
	parentLayoutIDs map[string][]string,
	parentMidsOff map[string]int,
	idScroll uint32,
	canReorder, dragging bool,
	drag dragReorderState, dragParent string,
	virtualize bool, rowHeight float32,
	first, last int,
) ([]View, View) {
	rowsCap := len(flatRows) + 2
	if dragging {
		rowsCap += 3
	}
	rows := make([]View, 0, rowsCap)
	if virtualize && first > 0 {
		rows = append(rows, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(first) * rowHeight,
			Sizing: FillFixed,
		}))
	}

	var ghostContent View
	for i := first; i <= last; i++ {
		if i < 0 || i >= len(flatRows) {
			continue
		}
		row := flatRows[i]
		var rowParent string
		if canReorder {
			rowParent = parentOf[row.ID]
		}
		isDragSibling := dragging && rowParent == dragParent

		if isDragSibling {
			si := siblingIdx[row.ID]
			if si == drag.currentIndex {
				rows = append(rows,
					dragReorderGapView(drag, DragReorderVertical))
			}
			if si == drag.sourceIndex {
				ghostContent = treeRowContent(
					*cfg, row, iconWidth, focusedID)
				continue
			}
		}

		if canReorder {
			rows = append(rows, treeDragRowView(
				*cfg, row, iconWidth, focusedID,
				siblingIdx[row.ID],
				siblingsByParent[rowParent],
				parentLayoutIDs[rowParent],
				parentMidsOff[rowParent],
				idScroll))
		} else {
			rows = append(rows, treeRowView(
				*cfg, row, iconWidth, focusedID))
		}
	}

	if virtualize && last < len(flatRows)-1 {
		remaining := len(flatRows) - 1 - last
		rows = append(rows, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(remaining) * rowHeight,
			Sizing: FillFixed,
		}))
	}

	return rows, ghostContent
}

func applyTreeDefaults(cfg *TreeCfg) {
	d := &DefaultTreeStyle
	if cfg.Indent == 0 {
		cfg.Indent = d.Indent
	}
	if cfg.Spacing == 0 {
		cfg.Spacing = d.Spacing
	}
	if cfg.Color == (Color{}) {
		cfg.Color = d.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = d.ColorFocus
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
}

func treeExpandedState(w *Window, treeID string) map[string]bool {
	if treeID == "" {
		return nil
	}
	return StateReadOr[string, map[string]bool](
		w, nsTreeExpanded, treeID, nil)
}

func treeExpandedSet(w *Window, treeID, nodeID string, expanded bool) {
	if treeID == "" || nodeID == "" {
		return
	}
	sm := StateMap[string, map[string]bool](w, nsTreeExpanded, capModerate)
	nodes, _ := sm.Get(treeID)
	if nodes == nil {
		nodes = make(map[string]bool)
	}
	if expanded {
		nodes[nodeID] = true
		sm.Set(treeID, nodes)
		return
	}
	delete(nodes, nodeID)
	if len(nodes) == 0 {
		sm.Delete(treeID)
		return
	}
	sm.Set(treeID, nodes)
}

func treeFocusedSet(w *Window, treeID, nodeID string) {
	if treeID == "" {
		return
	}
	sm := StateMap[string, string](w, nsTreeFocus, capModerate)
	if nodeID == "" {
		sm.Delete(treeID)
		return
	}
	sm.Set(treeID, nodeID)
}

func treeLazyKey(treeID, nodeID string) string {
	return treeID + "\x1e" + nodeID
}

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
		isLoading, _ := lazyState.Get(lazyKey)

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
			TextStyleIcon:   treeNodeTextStyleIcon(node),
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
				Text:          guiLocale.StrLoading,
				TextStyle:     DefaultTreeStyle.TextStyle,
				TextStyleIcon: DefaultTreeStyle.TextStyleIcon,
				IsLoading:     true,
			})
		}
	}
}

func treeEstimateRowHeight(cfg TreeCfg, w *Window) float32 {
	style := DefaultTreeStyle.TextStyle
	if len(cfg.Nodes) > 0 {
		style = treeNodeTextStyle(cfg.Nodes[0])
	}
	height := style.Size
	if w != nil && w.textMeasurer != nil {
		height = w.textMeasurer.FontHeight(style)
	}
	return height + PaddingTwoFive.Height() + cfg.Spacing
}

func treeVisibleRange(
	treeHeight, rowHeight float32,
	totalRows int,
	idScroll uint32,
	w *Window,
) (int, int) {
	if w == nil {
		return 0, totalRows - 1
	}
	scrollY := StateReadOr(
		w, nsScrollY, idScroll, float32(0))
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
	return DefaultTreeStyle.TextStyle
}

func treeNodeTextStyleIcon(node TreeNodeCfg) TextStyle {
	if node.TextStyleIcon != (TextStyle{}) {
		return node.TextStyleIcon
	}
	return DefaultTreeStyle.TextStyleIcon
}

func treeIconWidth(cfg *TreeCfg, w *Window) float32 {
	style := DefaultTreeStyle.TextStyleIcon
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

func treeArrowIcon(row treeFlatRow) string {
	if !row.HasChildren {
		return " "
	}
	if row.IsExpanded {
		return IconDropDown
	}
	if guiLocale.TextDir == TextDirRTL {
		return IconDropLeft
	}
	return IconDropRight
}

func treeRowView(
	cfg TreeCfg,
	row treeFlatRow,
	iconWidth float32,
	focusedID string,
) View {
	if row.IsLoading {
		return Row(ContainerCfg{
			Padding: SomeP(
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

	rowID := row.ID
	isFocused := focusedID == rowID
	rowColor := ColorTransparent
	if isFocused {
		rowColor = cfg.ColorFocus
	}
	a11yState := AccessStateNone
	if row.IsExpanded && row.HasChildren {
		a11yState = AccessStateExpanded
	}
	rootFocusID := cfg.IDFocus
	onSelect := cfg.OnSelect
	onLazyLoad := cfg.OnLazyLoad

	return Row(ContainerCfg{
		A11YRole:  AccessRoleTreeItem,
		A11YLabel: row.Text,
		A11YState: a11yState,
		Color:     rowColor,
		Radius:    Some(cfg.Radius.Get(DefaultTreeStyle.Radius)),
		Padding: SomeP(
			2, 5, 2,
			float32(row.Depth)*cfg.Indent+5,
		),
		Sizing:  FillFit,
		Spacing: NoSpacing,
		Content: treeRowContentViews(row, iconWidth),
		OnClick: func(_ *Layout, e *Event, w *Window) {
			treeRowClick(
				cfg.ID, row, rootFocusID, onSelect, onLazyLoad, e, w)
		},
		OnHover: func(layout *Layout, _ *Event, w *Window) {
			w.SetMouseCursorPointingHand()
			if !isFocused {
				layout.Shape.Color = cfg.ColorHover
			}
		},
	})
}

func treeDragRowView(
	cfg TreeCfg,
	row treeFlatRow,
	iconWidth float32,
	focusedID string,
	sibIdx int,
	siblingIDs []string,
	itemLayoutIDs []string,
	midsOffset int,
	idScroll uint32,
) View {
	if row.IsLoading {
		return treeRowView(cfg, row, iconWidth, focusedID)
	}

	rowID := row.ID
	isFocused := focusedID == rowID
	rowColor := ColorTransparent
	if isFocused {
		rowColor = cfg.ColorFocus
	}
	a11yState := AccessStateNone
	if row.IsExpanded && row.HasChildren {
		a11yState = AccessStateExpanded
	}
	rootFocusID := cfg.IDFocus
	onSelect := cfg.OnSelect
	onLazyLoad := cfg.OnLazyLoad
	onReorder := cfg.OnReorder
	treeID := cfg.ID
	layoutID := "tr_" + cfg.ID + "_" + row.ID

	return Row(ContainerCfg{
		ID:        layoutID,
		A11YRole:  AccessRoleTreeItem,
		A11YLabel: row.Text,
		A11YState: a11yState,
		Color:     rowColor,
		Radius:    Some(cfg.Radius.Get(DefaultTreeStyle.Radius)),
		Padding: SomeP(
			2, 5, 2,
			float32(row.Depth)*cfg.Indent+5,
		),
		Sizing:  FillFit,
		Spacing: NoSpacing,
		Content: treeRowContentViews(row, iconWidth),
		OnClick: func(layout *Layout, e *Event, w *Window) {
			dragReorderStart(dragReorderStartCfg{
				DragKey:       treeID,
				Index:         sibIdx,
				ItemID:        rowID,
				Axis:          DragReorderVertical,
				ItemIDs:       siblingIDs,
				OnReorder:     onReorder,
				ItemLayoutIDs: itemLayoutIDs,
				MidsOffset:    midsOffset,
				IDScroll:      idScroll,
				Layout:        layout,
				Event:         e,
			}, w)
			treeRowClick(
				treeID, row, rootFocusID, onSelect, onLazyLoad, e, w)
		},
		OnHover: func(layout *Layout, _ *Event, w *Window) {
			w.SetMouseCursorPointingHand()
			if !isFocused {
				layout.Shape.Color = cfg.ColorHover
			}
		},
	})
}

// treeRowContent returns the inner content of a tree row without
// the Row container — used for the drag ghost.
func treeRowContent(
	cfg TreeCfg,
	row treeFlatRow,
	iconWidth float32,
	focusedID string,
) View {
	rowColor := ColorTransparent
	if focusedID == row.ID {
		rowColor = cfg.ColorFocus
	}
	return Row(ContainerCfg{
		Color:  rowColor,
		Radius: Some(cfg.Radius.Get(DefaultTreeStyle.Radius)),
		Padding: SomeP(
			2, 5, 2,
			float32(row.Depth)*cfg.Indent+5,
		),
		Sizing:  FillFit,
		Spacing: NoSpacing,
		Content: treeRowContentViews(row, iconWidth),
	})
}

func treeRowContentViews(row treeFlatRow, iconWidth float32) []View {
	return []View{
		Text(TextCfg{
			Text:      treeArrowIcon(row) + " ",
			MinWidth:  iconWidth,
			TextStyle: row.TextStyleIcon,
		}),
		Text(TextCfg{
			Text:      treeIconText(row.Icon),
			MinWidth:  iconWidth,
			TextStyle: row.TextStyleIcon,
		}),
		Text(TextCfg{
			Text:      row.Text,
			TextStyle: row.TextStyle,
		}),
	}
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
	rootFocusID uint32,
	onSelect func(string, *Event, *Window),
	onLazyLoad func(string, string, *Window),
	e *Event,
	w *Window,
) {
	treeFocusedSet(w, treeID, row.ID)
	if rootFocusID > 0 {
		w.SetIDFocus(rootFocusID)
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
		onSelect(row.ID, e, w)
	}
	e.IsHandled = true
}

func treeOnKeyDown(
	treeID string,
	visibleIDs []string,
	rowByID map[string]treeFlatRow,
	onSelect func(string, *Event, *Window),
	onLazyLoad func(string, string, *Window),
	idScroll uint32,
	rowHeight float32,
	listHeight float32,
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
		focusMap.Set(treeID, visibleIDs[next])
		treeScrollTo(idScroll, next, rowHeight, listHeight, w)
		e.IsHandled = true
	case KeyDown:
		next := 0
		if cur >= 0 {
			next = min(cur+1, len(visibleIDs)-1)
		}
		focusMap.Set(treeID, visibleIDs[next])
		treeScrollTo(idScroll, next, rowHeight, listHeight, w)
		e.IsHandled = true
	case KeyHome:
		focusMap.Set(treeID, visibleIDs[0])
		treeScrollTo(idScroll, 0, rowHeight, listHeight, w)
		e.IsHandled = true
	case KeyEnd:
		last := len(visibleIDs) - 1
		focusMap.Set(treeID, visibleIDs[last])
		treeScrollTo(idScroll, last, rowHeight, listHeight, w)
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
				treeScrollTo(idScroll, pi, rowHeight, listHeight, w)
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
		if onSelect != nil {
			onSelect(focusedID, e, w)
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
	idScroll uint32, idx int, rowH, listH float32, w *Window,
) {
	if idScroll > 0 && rowH > 0 {
		scrollEnsureVisible(idScroll, idx, rowH, listH, w)
	}
}
