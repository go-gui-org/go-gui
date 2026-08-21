package gui

import (
	"fmt"
	"slices"
	"strings"
)

// dock_layout_tree.go — user-owned, serializable layout tree for
// IDE-style docking panels. Binary tree of splits; leaves are
// panel groups (one or more panels shown as tabs).

// DockSplitDir controls how two panes are arranged in a split.
// exportaudit:keep — reachable from an exported signature
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

// MarshalText implements encoding.TextMarshaler. // exportaudit:keep — stdlib interface method
func (d DockSplitDir) MarshalText() ([]byte, error) {
	if int(d) < len(dockSplitDirText) {
		return dockSplitDirText[d], nil
	}
	return nil, fmt.Errorf("unknown DockSplitDir %d", d)
}

// UnmarshalText implements encoding.TextUnmarshaler. // exportaudit:keep — stdlib interface method
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
type dockNodeKind uint8

// DockNodeKind constants.
const (
	dockNodeSplit dockNodeKind = iota
	dockNodePanelGroup
)

var dockNodeKindText = [2][]byte{
	dockNodeSplit:      []byte("split"),
	dockNodePanelGroup: []byte("panelGroup"),
}

// MarshalText implements encoding.TextMarshaler. // exportaudit:keep — stdlib interface method
func (k dockNodeKind) MarshalText() ([]byte, error) {
	if int(k) < len(dockNodeKindText) {
		return dockNodeKindText[k], nil
	}
	return nil, fmt.Errorf("unknown DockNodeKind %d", k)
}

// UnmarshalText implements encoding.TextUnmarshaler. // exportaudit:keep — stdlib interface method
func (k *dockNodeKind) UnmarshalText(text []byte) error {
	switch string(text) {
	case "split":
		*k = dockNodeSplit
	case "panelGroup":
		*k = dockNodePanelGroup
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
	// exportaudit:keep — json-tagged or same-named member
	PanelIDs []string     `json:"panelIDs,omitempty"`
	Ratio    float32      `json:"ratio"`
	Kind     dockNodeKind `json:"kind"`
	// Split fields (used when Kind == DockNodeSplit).
	Dir DockSplitDir `json:"dir"`
}

// DockSplit creates a split node.
func DockSplit(id string, dir DockSplitDir, ratio float32, first, second *DockNode) *DockNode {
	return &DockNode{
		Kind:   dockNodeSplit,
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
		Kind:       dockNodePanelGroup,
		ID:         id,
		PanelIDs:   panelIDs,
		SelectedID: selectedID,
	}
}

// dockNodeIDSep joins the parts of a minted node ID.
//
// A node ID is tree data, not a layout ID: dockGroupView and
// dockSplitView feed it into ScopeID(dockID, node.ID) as a *part*. A
// part containing IDSep would make the composed leaf absolute, so it
// would skip the dock scope and land window-global (issue #389).
const dockNodeIDSep = "-"

// dockNodeID composes a minted node ID from parts. Separate from
// ScopeID on purpose — see dockNodeIDSep.
func dockNodeID(parts ...string) string {
	return strings.Join(parts, dockNodeIDSep)
}

// dockNodeMaxDepth caps recursion when sanitizing deserialized trees.
const dockNodeMaxDepth = 32

// DockNodeSanitize clamps ratio to [0,1], replaces NaN/Inf with
// 0.5, and truncates trees deeper than dockNodeMaxDepth. Call
// after json.Unmarshal to harden against malformed input.
func dockNodeSanitize(node *DockNode) {
	dockNodeSanitizeRec(node, 0)
}

func dockNodeSanitizeRec(node *DockNode, depth int) {
	if node == nil {
		return
	}
	if node.Kind == dockNodeSplit {
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
func dockTreeCollectPanelNodes(node *DockNode) []*DockNode {
	var result []*DockNode
	dockTreeCollectPanelNodesRec(node, &result)
	return result
}

func dockTreeCollectPanelNodesRec(node *DockNode, result *[]*DockNode) {
	if node.Kind == dockNodeSplit {
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
	if node.Kind == dockNodeSplit {
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
func dockTreeFindGroupByID(node *DockNode, groupID string) (*DockNode, bool) {
	if node.Kind == dockNodeSplit {
		if node.First != nil {
			if g, ok := dockTreeFindGroupByID(node.First, groupID); ok {
				return g, true
			}
		}
		if node.Second != nil {
			if g, ok := dockTreeFindGroupByID(node.Second, groupID); ok {
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
	if nd.Kind == dockNodeSplit {
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
	return node.Kind == dockNodePanelGroup && len(node.PanelIDs) == 0
}

// DockTreeAddTab adds a panel to an existing group (by groupID).
// Returns the new root.
func DockTreeAddTab(root *DockNode, groupID, panelID string) *DockNode {
	return dockTreeAddTabRec(root, groupID, panelID)
}

func dockTreeAddTabRec(nd *DockNode, groupID, panelID string) *DockNode {
	if nd.Kind == dockNodeSplit {
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
func dockTreeSplitAt(root *DockNode, groupID, panelID string, zone DockDropZone) *DockNode {
	return dockTreeSplitAtRec(root, groupID, panelID, zone)
}

func dockTreeSplitAtRec(nd *DockNode, groupID, panelID string, zone DockDropZone) *DockNode {
	if nd.Kind == dockNodeSplit {
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
	newGroup := DockPanelGroup(dockNodeID(groupID, "new", panelID), []string{panelID}, panelID)
	existing := DockPanelGroup(nd.ID, nd.PanelIDs, nd.SelectedID)
	dir := dockZoneToSplitDir(zone)
	splitID := dockNodeID(groupID, "split", panelID)
	firstIsNew := zone == dockDropLeft || zone == dockDropTop
	if firstIsNew {
		return DockSplit(splitID, dir, 0.5, newGroup, existing)
	}
	return DockSplit(splitID, dir, 0.5, existing, newGroup)
}

// DockTreeWrapRoot wraps the current root in a new split for
// window-edge docking. The new panel goes at the indicated edge.
func dockTreeWrapRoot(root *DockNode, panelID string, zone DockDropZone) *DockNode {
	newGroup := DockPanelGroup(dockNodeID("dock_edge", panelID), []string{panelID}, panelID)
	dir := dockZoneToSplitDir(zone)
	splitID := dockNodeID("dock_root_split", panelID)
	firstIsNew := zone == dockDropWindowLeft || zone == dockDropWindowTop
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
func dockTreeMovePanel(root *DockNode, panelID, targetGroupID string, zone DockDropZone) *DockNode {
	newRoot := DockTreeRemovePanel(root, panelID)
	switch zone {
	case dockDropCenter:
		return DockTreeAddTab(newRoot, targetGroupID, panelID)
	case dockDropWindowTop, dockDropWindowBottom,
		dockDropWindowLeft, dockDropWindowRight:
		return dockTreeWrapRoot(newRoot, panelID, zone)
	default:
		return dockTreeSplitAt(newRoot, targetGroupID, panelID, zone)
	}
}

// DockTreeSelectPanel sets the selected panel in the group with
// the given groupID. Returns the new root.
func DockTreeSelectPanel(nd *DockNode, groupID, panelID string) *DockNode {
	if nd.Kind == dockNodeSplit {
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
	case dockDropLeft, dockDropRight, dockDropWindowLeft, dockDropWindowRight:
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
	if nd.Kind == dockNodeSplit {
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
