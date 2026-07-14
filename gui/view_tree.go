package gui

import (
	"sort"
	"strings"
)

const treeLoadingSuffix = ".__loading__"

// TreeCfg configures a tree view.
type TreeCfg struct {
	OnSelect   func(string, *Event, *Window)
	OnLazyLoad func(string, string, *Window)

	OnReorder func(movedID, beforeID string, w *Window)

	ID string `gui:"required"`

	A11YLabel       string
	A11YDescription string

	// ItemPaths is a convenience field for flat path strings. Each
	// string is slash-separated ("a/b/c") and auto-expanded into
	// nested TreeNodeCfg nodes. Duplicate path prefixes are merged.
	// When set, ItemPaths takes precedence over Nodes.
	ItemPaths  []string
	Nodes      []TreeNodeCfg
	Padding    Opt[Padding]
	SizeBorder Opt[float32]
	Radius     Opt[float32]

	Indent  float32
	Spacing float32

	Focusable bool
	IDScroll  uint32

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

// itemPathsToNodes converts slash-separated path strings
// ("a/b/c") into nested TreeNodeCfg nodes. Duplicate path
// prefixes are merged. ID is the full path, Text is the
// last segment.
func itemPathsToNodes(paths []string) []TreeNodeCfg {
	const maxDepth = 100 // safety cap on path nesting

	type nodeEntry struct {
		node     *TreeNodeCfg
		children map[string]*nodeEntry
	}
	root := &nodeEntry{children: make(map[string]*nodeEntry)}
	n := min(len(paths), maxDataConvLen)
	for _, p := range paths[:n] {
		parts := strings.Split(p, "/")
		cur := root
		depth := 0
		for _, part := range parts {
			if part == "" {
				continue
			}
			if depth >= maxDepth {
				break
			}
			depth++
			child, ok := cur.children[part]
			if !ok {
				child = &nodeEntry{
					node:     &TreeNodeCfg{Text: part},
					children: make(map[string]*nodeEntry),
				}
				cur.children[part] = child
			}
			cur = child
		}
	}
	// Materialize into slice, assigning full-path IDs.
	var build func(entry *nodeEntry, prefix string) []TreeNodeCfg
	build = func(entry *nodeEntry, prefix string) []TreeNodeCfg {
		var nodes []TreeNodeCfg
		keys := make([]string, 0, len(entry.children))
		for k := range entry.children {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := entry.children[k]
			fullPath := k
			if prefix != "" {
				fullPath = prefix + "/" + k
			}
			child.node.ID = fullPath
			child.node.Nodes = build(child, fullPath)
			nodes = append(nodes, *child.node)
		}
		return nodes
	}
	return build(root, "")
}

// Tree creates a tree view with optional virtualization and lazy loading.
func Tree(cfg TreeCfg) View {
	RequireID("Tree", cfg.ID)
	applyTreeDefaults(&cfg)
	if len(cfg.ItemPaths) > 0 {
		cfg.Nodes = itemPathsToNodes(cfg.ItemPaths)
	}
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

	return generateViewLayout(Column(ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleTree,
		A11YLabel: a11yLabel(cfg.A11YLabel, cfg.ID),
		A11Y:      makeA11YInfo(a11yLabel(cfg.A11YLabel, cfg.ID), cfg.A11YDescription),
		Focusable: cfg.Focusable,
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
