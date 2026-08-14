package gui

import (
	"slices"
	"strconv"
	"strings"
)

func inspectorTreeView(w *Window) View {
	var nodes []TreeNodeCfg
	if w != nil {
		nodes = w.inspectorTreeCache
	}
	return Tree(TreeCfg{
		ID:       inspectorTreeID,
		indent:   16,
		Spacing:  SomeF(1),
		Nodes:    nodes,
		OnSelect: func(id string, ctx EventCtx) { inspectorSelect(id, ctx.Window) },
	})
}

func inspectorSelect(path string, w *Window) {
	if w == nil {
		return
	}
	if strings.HasPrefix(path, inspectorPropPrefix) {
		if selected := inspectorSelectedPath(w); selected != "" {
			treeFocusedSet(w, inspectorTreeID, selected)
		}
		return
	}
	sm := StateMap[string, string](w, nsInspector, capInspector)
	// Default "": absent key means nothing is currently selected.
	selected := sm.GetOr("selected", "")
	if selected == path {
		sm.Delete("selected")
		sm.Delete("scroll_to")
		treeFocusedSet(w, inspectorTreeID, "")
		w.UpdateWindow()
		return
	}

	sm.Set("selected", path)
	sm.Set("scroll_to", path)
	treeFocusedSet(w, inspectorTreeID, path)

	expanded := StateReadOr[string, map[string]bool](
		w, nsTreeExpanded, inspectorTreeID, nil)
	if expanded == nil {
		expanded = make(map[string]bool)
	}
	// Mark every ancestor prefix expanded so the tree widget
	// flattens the path down to the selected node. Paths are
	// colon-composed (ScopeIDN), so split on IDSep.
	parts := strings.Split(path, IDSep)
	prefix := parts[0]
	expanded[prefix] = true
	for i := 1; i < len(parts); i++ {
		prefix = ScopeID(prefix, parts[i])
		expanded[prefix] = true
	}
	StateMap[string, map[string]bool](w, nsTreeExpanded, capModerate).
		Set(inspectorTreeID, expanded)
	w.UpdateWindow()
}

func inspectorPickPath(layout *Layout, x, y float32) string {
	if layout == nil || len(layout.Children) == 0 {
		return ""
	}
	return inspectorPickRecurse(&layout.Children[0], "0", x, y)
}

func inspectorPickRecurse(layout *Layout, path string, x, y float32) string {
	if layout == nil || layout.Shape == nil {
		return ""
	}
	if !layout.Shape.PointInShape(x, y) {
		return ""
	}
	for i := range slices.Backward(layout.Children) {
		childPath := ScopeIDN(path, "", i)
		if picked := inspectorPickRecurse(
			&layout.Children[i], childPath, x, y); picked != "" {
			return picked
		}
	}
	return path
}

func inspectorSelectedPath(w *Window) string {
	if w == nil {
		return ""
	}
	return StateReadOr(w, nsInspector, "selected", "")
}

func inspectorBuildTreeNodes(
	w *Window,
	layout *Layout,
	selected string,
	props map[string]inspectorNodeProps,
) []TreeNodeCfg {
	if layout == nil || len(layout.Children) == 0 {
		return nil
	}
	var expanded map[string]bool
	if w != nil {
		expanded = treeExpandedState(w, inspectorTreeID)
	}
	return inspectorLayoutToTree(
		w, expanded, &layout.Children[0], "0", selected, props)
}

func inspectorLayoutToTree(
	w *Window,
	expanded map[string]bool,
	layout *Layout,
	path string,
	selected string,
	props map[string]inspectorNodeProps,
) []TreeNodeCfg {
	if layout == nil {
		return nil
	}
	propSnapshot := inspectorSnapshotProps(layout)
	if props != nil {
		props[path] = propSnapshot
	}

	childNodes := make([]TreeNodeCfg, 0, 16)
	if path == selected {
		childNodes = append(childNodes, inspectorPropsNodes(propSnapshot)...)
	}

	isExpanded := expanded != nil && expanded[path]
	isAncestor := !isExpanded && selected != "" && strings.HasPrefix(selected, path+IDSep)

	if isExpanded || isAncestor {
		for i := range layout.Children {
			childPath := ScopeIDN(path, "", i)
			childNodes = append(
				childNodes,
				inspectorLayoutToTree(
					w, expanded, &layout.Children[i], childPath, selected, props,
				)...,
			)
		}
	} else if len(layout.Children) > 0 {
		// Add a dummy child to show the arrow icon for collapsed nodes.
		childNodes = append(childNodes, TreeNodeCfg{
			ID: ScopeID(path, "__dummy__"),
		})
	}

	return []TreeNodeCfg{{
		ID:            path,
		Text:          inspectorNodeLabel(layout.Shape),
		TextStyle:     inspectorNodeTextStyle(),
		textStyleIcon: inspectorNodeIconStyle(),
		Nodes:         childNodes,
	}}
}

func inspectorFindByPath(layout *Layout, path string) (*Layout, bool) {
	if layout == nil || path == "" {
		return nil, false
	}
	node := layout
	for part := range strings.SplitSeq(path, IDSep) {
		idx, err := strconv.Atoi(part)
		if err != nil || idx < 0 || idx >= len(node.Children) {
			return nil, false
		}
		node = &node.Children[idx]
	}
	return node, true
}
