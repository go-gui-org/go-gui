package gui

import (
	"fmt"
	"slices"
)

// dock_layout_tree.go — user-owned, serializable layout tree for
// IDE-style docking panels. Binary tree of splits; leaves are
// panel groups (one or more panels shown as tabs).

// DockSplitDir controls how two panes are arranged in a split.
type DockSplitDir uint8

// DockSplitDir constants.
const (
	DockSplitHorizontal DockSplitDir = iota // left | right
	DockSplitVertical                       // top | bottom
)

var dockSplitDirText = [2][]byte{
	DockSplitHorizontal: []byte("horizontal"),
	DockSplitVertical:   []byte("vertical"),
}

// MarshalText implements encoding.TextMarshaler.
func (d DockSplitDir) MarshalText() ([]byte, error) {
	if int(d) < len(dockSplitDirText) {
		return dockSplitDirText[d], nil
	}
	return nil, fmt.Errorf("unknown DockSplitDir %d", d)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *DockSplitDir) UnmarshalText(text []byte) error {
	switch string(text) {
	case "horizontal":
		*d = DockSplitHorizontal
	case "vertical":
		*d = DockSplitVertical
	default:
		return fmt.Errorf("unknown DockSplitDir %q", text)
	}
	return nil
}

// DockNodeKind distinguishes split nodes from leaf panel groups.
type DockNodeKind uint8

// DockNodeKind constants.
const (
	DockNodeSplit DockNodeKind = iota
	DockNodePanelGroup
)

var dockNodeKindText = [2][]byte{
	DockNodeSplit:      []byte("split"),
	DockNodePanelGroup: []byte("panelGroup"),
}

// MarshalText implements encoding.TextMarshaler.
func (k DockNodeKind) MarshalText() ([]byte, error) {
	if int(k) < len(dockNodeKindText) {
		return dockNodeKindText[k], nil
	}
	return nil, fmt.Errorf("unknown DockNodeKind %d", k)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (k *DockNodeKind) UnmarshalText(text []byte) error {
	switch string(text) {
	case "split":
		*k = DockNodeSplit
	case "panelGroup":
		*k = DockNodePanelGroup
	default:
		return fmt.Errorf("unknown DockNodeKind %q", text)
	}
	return nil
}

// DockNode is a single node in the dock layout tree: either a
// split (with two children) or a leaf panel group.
type DockNode struct {
	First      *DockNode `json:"first,omitempty"`
	Second     *DockNode `json:"second,omitempty"`
	ID         string    `json:"id"`
	SelectedID string    `json:"selectedID,omitempty"`
	// Panel group fields (used when Kind == DockNodePanelGroup).
	PanelIDs []string     `json:"panelIDs,omitempty"`
	Ratio    float32      `json:"ratio"`
	Kind     DockNodeKind `json:"kind"`
	// Split fields (used when Kind == DockNodeSplit).
	Dir DockSplitDir `json:"dir"`
}

// DockSplit creates a split node.
func DockSplit(id string, dir DockSplitDir, ratio float32, first, second *DockNode) *DockNode {
	return &DockNode{
		Kind:   DockNodeSplit,
		ID:     id,
		Dir:    dir,
		Ratio:  ratio,
		First:  first,
		Second: second,
	}
}

// DockPanelGroup creates a panel group node.
func DockPanelGroup(id string, panelIDs []string, selectedID string) *DockNode {
	return &DockNode{
		Kind:       DockNodePanelGroup,
		ID:         id,
		PanelIDs:   panelIDs,
		SelectedID: selectedID,
	}
}

// dockNodeMaxDepth caps recursion when sanitizing deserialized trees.
const dockNodeMaxDepth = 32

// DockNodeSanitize clamps ratio to [0,1], replaces NaN/Inf with
// 0.5, and truncates trees deeper than dockNodeMaxDepth. Call
// after json.Unmarshal to harden against malformed input.
func DockNodeSanitize(node *DockNode) {
	dockNodeSanitizeRec(node, 0)
}

func dockNodeSanitizeRec(node *DockNode, depth int) {
	if node == nil {
		return
	}
	if node.Kind == DockNodeSplit {
		if !f32IsFinite(node.Ratio) {
			node.Ratio = 0.5
		}
		node.Ratio = max(0, min(1, node.Ratio))
		if depth >= dockNodeMaxDepth {
			node.First = nil
			node.Second = nil
			return
		}
		dockNodeSanitizeRec(node.First, depth+1)
		dockNodeSanitizeRec(node.Second, depth+1)
	}
}

// DockTreeCollectPanelNodes returns all panel group nodes in the
// tree. Used for zone detection during drag.
func DockTreeCollectPanelNodes(node *DockNode) []*DockNode {
	var result []*DockNode
	dockTreeCollectPanelNodesRec(node, &result)
	return result
}

func dockTreeCollectPanelNodesRec(node *DockNode, result *[]*DockNode) {
	if node.Kind == DockNodeSplit {
		if node.First != nil {
			dockTreeCollectPanelNodesRec(node.First, result)
		}
		if node.Second != nil {
			dockTreeCollectPanelNodesRec(node.Second, result)
		}
	} else {
		*result = append(*result, node)
	}
}

// DockTreeFindGroupByPanel returns the panel group node containing
// the given panelID, or nil if not found.
func DockTreeFindGroupByPanel(node *DockNode, panelID string) (*DockNode, bool) {
	if node.Kind == DockNodeSplit {
		if node.First != nil {
			if g, ok := DockTreeFindGroupByPanel(node.First, panelID); ok {
				return g, true
			}
		}
		if node.Second != nil {
			if g, ok := DockTreeFindGroupByPanel(node.Second, panelID); ok {
				return g, true
			}
		}
	} else if slices.Contains(node.PanelIDs, panelID) {
		return node, true
	}
	return nil, false
}

// DockTreeFindGroupByID returns the panel group node with the
// given group id, or nil if not found.
func DockTreeFindGroupByID(node *DockNode, groupID string) (*DockNode, bool) {
	if node.Kind == DockNodeSplit {
		if node.First != nil {
			if g, ok := DockTreeFindGroupByID(node.First, groupID); ok {
				return g, true
			}
		}
		if node.Second != nil {
			if g, ok := DockTreeFindGroupByID(node.Second, groupID); ok {
				return g, true
			}
		}
	} else if node.ID == groupID {
		return node, true
	}
	return nil, false
}

// DockTreeRemovePanel removes a panel from the tree. If the group
// becomes empty, collapses the parent split (replaces it with the
// remaining sibling). Returns the new root.
func DockTreeRemovePanel(root *DockNode, panelID string) *DockNode {
	return dockTreeRemovePanelRec(root, panelID)
}

func dockTreeRemovePanelRec(nd *DockNode, panelID string) *DockNode {
	if nd.Kind == DockNodeSplit {
		if nd.First == nil || nd.Second == nil {
			return nd
		}
		newFirst := dockTreeRemovePanelRec(nd.First, panelID)
		newSecond := dockTreeRemovePanelRec(nd.Second, panelID)
		if dockTreeIsEmpty(newFirst) {
			return newSecond
		}
		if dockTreeIsEmpty(newSecond) {
			return newFirst
		}
		if newFirst != nd.First || newSecond != nd.Second {
			return DockSplit(nd.ID, nd.Dir, nd.Ratio, newFirst, newSecond)
		}
		return nd
	}

	if !slices.Contains(nd.PanelIDs, panelID) {
		return nd
	}
	newIDs := make([]string, 0, max(len(nd.PanelIDs)-1, 0))
	for _, id := range nd.PanelIDs {
		if id != panelID {
			newIDs = append(newIDs, id)
		}
	}
	if len(newIDs) == 0 {
		return DockPanelGroup("__dock_empty__", nil, "")
	}
	newSelected := nd.SelectedID
	if newSelected == panelID {
		newSelected = newIDs[0]
	}
	return DockPanelGroup(nd.ID, newIDs, newSelected)
}

func dockTreeIsEmpty(node *DockNode) bool {
	return node.Kind == DockNodePanelGroup && len(node.PanelIDs) == 0
}

// DockTreeAddTab adds a panel to an existing group (by groupID).
// Returns the new root.
func DockTreeAddTab(root *DockNode, groupID, panelID string) *DockNode {
	return dockTreeAddTabRec(root, groupID, panelID)
}

func dockTreeAddTabRec(nd *DockNode, groupID, panelID string) *DockNode {
	if nd.Kind == DockNodeSplit {
		if nd.First == nil || nd.Second == nil {
			return nd
		}
		newFirst := dockTreeAddTabRec(nd.First, groupID, panelID)
		newSecond := dockTreeAddTabRec(nd.Second, groupID, panelID)
		if newFirst != nd.First || newSecond != nd.Second {
			return DockSplit(nd.ID, nd.Dir, nd.Ratio, newFirst, newSecond)
		}
		return nd
	}
	if nd.ID != groupID {
		return nd
	}
	newIDs := make([]string, len(nd.PanelIDs), len(nd.PanelIDs)+1)
	copy(newIDs, nd.PanelIDs)
	newIDs = append(newIDs, panelID)
	return DockPanelGroup(nd.ID, newIDs, panelID)
}

// DockTreeSplitAt replaces a group (by groupID) with a new split
// containing the original group and a new single-panel group.
// The new panel goes into the position indicated by zone.
func DockTreeSplitAt(root *DockNode, groupID, panelID string, zone DockDropZone) *DockNode {
	return dockTreeSplitAtRec(root, groupID, panelID, zone)
}

func dockTreeSplitAtRec(nd *DockNode, groupID, panelID string, zone DockDropZone) *DockNode {
	if nd.Kind == DockNodeSplit {
		if nd.First == nil || nd.Second == nil {
			return nd
		}
		newFirst := dockTreeSplitAtRec(nd.First, groupID, panelID, zone)
		newSecond := dockTreeSplitAtRec(nd.Second, groupID, panelID, zone)
		if newFirst != nd.First || newSecond != nd.Second {
			return DockSplit(nd.ID, nd.Dir, nd.Ratio, newFirst, newSecond)
		}
		return nd
	}
	if nd.ID != groupID {
		return nd
	}
	newGroup := DockPanelGroup(groupID+"_new_"+panelID, []string{panelID}, panelID)
	existing := DockPanelGroup(nd.ID, nd.PanelIDs, nd.SelectedID)
	dir := dockZoneToSplitDir(zone)
	splitID := groupID + "_split_" + panelID
	firstIsNew := zone == DockDropLeft || zone == DockDropTop
	if firstIsNew {
		return DockSplit(splitID, dir, 0.5, newGroup, existing)
	}
	return DockSplit(splitID, dir, 0.5, existing, newGroup)
}

// DockTreeWrapRoot wraps the current root in a new split for
// window-edge docking. The new panel goes at the indicated edge.
func DockTreeWrapRoot(root *DockNode, panelID string, zone DockDropZone) *DockNode {
	newGroup := DockPanelGroup("dock_edge_"+panelID, []string{panelID}, panelID)
	dir := dockZoneToSplitDir(zone)
	splitID := "dock_root_split_" + panelID
	firstIsNew := zone == DockDropWindowLeft || zone == DockDropWindowTop
	ratio := float32(0.8)
	if firstIsNew {
		ratio = 0.2
	}
	if firstIsNew {
		return DockSplit(splitID, dir, ratio, newGroup, root)
	}
	return DockSplit(splitID, dir, ratio, root, newGroup)
}

// DockTreeMovePanel removes a panel from its source group and
// inserts it at the target: either as a tab (center zone) or as a
// new split (edge zones). Returns the new root.
func DockTreeMovePanel(root *DockNode, panelID, targetGroupID string, zone DockDropZone) *DockNode {
	newRoot := DockTreeRemovePanel(root, panelID)
	switch zone {
	case DockDropCenter:
		return DockTreeAddTab(newRoot, targetGroupID, panelID)
	case DockDropWindowTop, DockDropWindowBottom,
		DockDropWindowLeft, DockDropWindowRight:
		return DockTreeWrapRoot(newRoot, panelID, zone)
	default:
		return DockTreeSplitAt(newRoot, targetGroupID, panelID, zone)
	}
}

// DockTreeSelectPanel sets the selected panel in the group with
// the given groupID. Returns the new root.
func DockTreeSelectPanel(nd *DockNode, groupID, panelID string) *DockNode {
	if nd.Kind == DockNodeSplit {
		if nd.First == nil || nd.Second == nil {
			return nd
		}
		newFirst := DockTreeSelectPanel(nd.First, groupID, panelID)
		newSecond := DockTreeSelectPanel(nd.Second, groupID, panelID)
		if newFirst != nd.First || newSecond != nd.Second {
			return DockSplit(nd.ID, nd.Dir, nd.Ratio, newFirst, newSecond)
		}
	} else if nd.ID == groupID && nd.SelectedID != panelID {
		return DockPanelGroup(nd.ID, nd.PanelIDs, panelID)
	}
	return nd
}

// dockZoneToSplitDir maps a drop zone to a split direction.
func dockZoneToSplitDir(zone DockDropZone) DockSplitDir {
	switch zone {
	case DockDropLeft, DockDropRight, DockDropWindowLeft, DockDropWindowRight:
		return DockSplitHorizontal
	default:
		return DockSplitVertical
	}
}

// dockTreeUpdateRatio returns a new tree with the ratio of the
// given split updated.
func dockTreeUpdateRatio(root *DockNode, splitID string, ratio float32) *DockNode {
	return dockTreeUpdateRatioRec(root, splitID, ratio)
}

func dockTreeUpdateRatioRec(nd *DockNode, splitID string, ratio float32) *DockNode {
	if nd.Kind == DockNodeSplit {
		if nd.ID == splitID {
			return DockSplit(nd.ID, nd.Dir, ratio, nd.First, nd.Second)
		}
		if nd.First == nil || nd.Second == nil {
			return nd
		}
		newFirst := dockTreeUpdateRatioRec(nd.First, splitID, ratio)
		newSecond := dockTreeUpdateRatioRec(nd.Second, splitID, ratio)
		if newFirst != nd.First || newSecond != nd.Second {
			return DockSplit(nd.ID, nd.Dir, nd.Ratio, newFirst, newSecond)
		}
	}
	return nd
}
