package gui

import (
	"strings"
	"testing"
)

// dock_layout_identity_test.go — issue #389: a dock group's container
// ID must be absolute and independent of the group's position in the
// tree, so panel content keeps its identity (and its ID-keyed state)
// across a rearrangement.

// dockIdentityFixture renders a two-group dock whose first panel holds
// a scrollable container. root is re-read on every frame, so a test can
// swap in a rearranged tree and re-render.
type dockIdentityFixture struct {
	w    *Window
	root *DockNode
}

func newDockIdentityFixture(t *testing.T) *dockIdentityFixture {
	t.Helper()
	fix := &dockIdentityFixture{
		root: DockSplit("s1", DockSplitVertical, 0.5,
			DockPanelGroup("g1", []string{"a"}, "a"),
			DockPanelGroup("g2", []string{"b"}, "b")),
	}
	rows := func(n int) []View {
		out := make([]View, 0, n)
		for range n {
			out = append(out, Column(ContainerCfg{Sizing: FillFixed, Height: 20}))
		}
		return out
	}
	fix.w = NewWindow(WindowCfg{State: new(int), Width: 800, Height: 600})
	fix.w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			DockLayout(DockLayoutCfg{
				ID:   "dock",
				Root: fix.root,
				Panels: []DockPanelDef{
					{ID: "a", Label: "A", Content: []View{
						Column(ContainerCfg{
							ID:         "list",
							Scrollable: true,
							Sizing:     FixedFixed,
							Width:      200,
							Height:     100,
							Content:    rows(20),
						}),
					}},
					{ID: "b", Label: "B"},
				},
			}),
		}})
	}
	fix.w.refreshLayout = true
	fix.w.FrameFn()
	return fix
}

// rearrange drops panel b on group g1's bottom edge — the same tree
// edit a completed drag performs — and re-renders.
func (f *dockIdentityFixture) rearrange() {
	f.root = dockTreeMovePanel(f.root, "b", "g1", dockDropBottom)
	f.w.refreshLayout = true
	f.w.FrameFn()
}

// TestDockPanelContentIDSurvivesRearrange is the acceptance test for
// issue #389: the group scopes its panel content, so a position-derived
// group ID re-keyed every widget inside the panel whenever anything
// else in the dock moved.
func TestDockPanelContentIDSurvivesRearrange(t *testing.T) {
	fix := newDockIdentityFixture(t)
	const wantID = "dock:g1:list"

	if _, ok := fix.w.layout.FindByID(wantID); !ok {
		t.Fatalf("%s not found before rearrange", wantID)
	}
	fix.rearrange()
	if _, ok := fix.w.layout.FindByID(wantID); !ok {
		t.Fatalf("%s not found after rearrange — panel content was re-keyed", wantID)
	}
}

// TestDockPanelScrollSurvivesRearrange covers the user-visible half:
// state keyed on the panel-content ID is still addressed by the same
// key after a drop elsewhere in the dock.
func TestDockPanelScrollSurvivesRearrange(t *testing.T) {
	fix := newDockIdentityFixture(t)
	const listID = "dock:g1:list"

	fix.w.ScrollVerticalTo(listID, -60)
	before := fix.w.ScrollVerticalOffset(listID)
	if before == 0 {
		t.Fatalf("scroll offset did not take: %g", before)
	}
	fix.rearrange()
	if got := fix.w.ScrollVerticalOffset(listID); got != before {
		t.Errorf("scroll offset = %g, want %g (orphaned by the rearrangement)",
			got, before)
	}
}

// TestDockGroupAndSplitIDsIndependentOfPosition pins the shape of the
// two container IDs: dock ID + node ID, never the splitter path.
func TestDockGroupAndSplitIDsIndependentOfPosition(t *testing.T) {
	fix := newDockIdentityFixture(t)
	for _, id := range []string{"dock:g1", "dock:g2", "dock:split:s1"} {
		if _, ok := fix.w.layout.FindByID(id); !ok {
			t.Errorf("%s not found in rendered dock", id)
		}
	}
}

// TestDockMintedNodeIDsHaveNoIDSep guards the other half of the rule:
// a minted node ID is a *part* fed into ScopeID(dockID, node.ID), so an
// IDSep in it would make the composed leaf absolute and put the group
// back outside the dock scope.
func TestDockMintedNodeIDsHaveNoIDSep(t *testing.T) {
	root := DockSplit("s1", DockSplitVertical, 0.5,
		DockPanelGroup("g1", []string{"a"}, "a"),
		DockPanelGroup("g2", []string{"b"}, "b"))
	trees := map[string]*DockNode{
		"split at group": dockTreeMovePanel(root, "b", "g1", dockDropBottom),
		"window edge":    dockTreeMovePanel(root, "b", "", dockDropWindowLeft),
	}
	for name, tree := range trees {
		dockWalkNodes(tree, func(nd *DockNode) {
			if strings.Contains(nd.ID, IDSep) {
				t.Errorf("%s: node ID %q contains %q", name, nd.ID, IDSep)
			}
		})
	}
}

func dockWalkNodes(nd *DockNode, fn func(*DockNode)) {
	if nd == nil {
		return
	}
	fn(nd)
	dockWalkNodes(nd.First, fn)
	dockWalkNodes(nd.Second, fn)
}
