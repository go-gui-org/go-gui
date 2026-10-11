package gui

import (
	"runtime"
	"strconv"
	"testing"
)

func TestTreeEstimateRowHeightNilWindow(t *testing.T) {
	// Without a text measurer the estimate falls back to the style
	// size, then adds the fixed padding and the configured spacing.
	cfg := TreeCfg{Spacing: SpacingPx(4)}
	got := treeEstimateRowHeight(cfg, nil)
	want := defaultTreeStyle.TextStyle.Size +
		PaddingTwoFive.Height() + 4
	if got != want {
		t.Fatalf("treeEstimateRowHeight() = %v, want %v", got, want)
	}
}

func TestTreeEstimateRowHeightWithMeasurer(t *testing.T) {
	w := newTestWindow()
	w.textMeasurer = &stubTextMeasurer{fontHeight: 22}
	cfg := TreeCfg{Spacing: NoSpacing}
	got := treeEstimateRowHeight(cfg, w)
	want := float32(22) + PaddingTwoFive.Height() + 0
	if got != want {
		t.Fatalf("treeEstimateRowHeight() = %v, want %v", got, want)
	}
}

func TestTreeSiblingIndex(t *testing.T) {
	sibs := []string{"a", "b", "c"}
	if got := treeSiblingIndex(sibs, "b"); got != 1 {
		t.Fatalf("treeSiblingIndex(b) = %d, want 1", got)
	}
	if got := treeSiblingIndex(sibs, "zz"); got != -1 {
		t.Fatalf("treeSiblingIndex(missing) = %d, want -1", got)
	}
	if got := treeSiblingIndex(nil, "a"); got != -1 {
		t.Fatalf("treeSiblingIndex(nil) = %d, want -1", got)
	}
}

func TestTreeDragRowViewLoadingDelegates(t *testing.T) {
	// A loading placeholder row must render through treeRowView with
	// no drag arm, so no layout ID is attached.
	cfg := TreeCfg{ID: "tree", Spacing: NoSpacing}
	view := treeDragRowView(nil,
		cfg,
		treeFlatRow{ID: "load", IsLoading: true, Text: "Loading..."},
		16, "", nil, "",
	)
	w := newTestWindow()
	layout := generateViewLayout(view, w)
	if layout.Shape.ID != "" {
		t.Fatalf("loading row layout ID = %q, want empty", layout.Shape.ID)
	}
	// The loading branch renders through treeRowView's minimal
	// container, which carries no a11y role.
	if layout.Shape.A11YRole != AccessRoleNone {
		t.Fatalf("loading row role = %d, want None", layout.Shape.A11YRole)
	}
}

func TestTreeDragRowViewArmsDrag(t *testing.T) {
	// A reorderable row must carry its layout ID and arm the
	// drag-reorder state when clicked.
	cfg := TreeCfg{ID: "tree", Spacing: NoSpacing}
	row := treeFlatRow{ID: "a", Text: "A"}
	view := treeDragRowView(nil,
		cfg, row, 16, "",
		treeTestDragClick(&cfg, &row, 0,
			[]string{"a"}, []string{"tree:row:a"}),
		"tree:row:a",
	)
	w := newTestWindow()
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	layout := generateViewLayout(view, w)
	if layout.Shape.ID != "tree:row:a" {
		t.Fatalf("row layout ID = %q, want tree:row:a", layout.Shape.ID)
	}
	if layout.Shape.events == nil || layout.Shape.events.OnClick == nil {
		t.Fatal("reorderable row must have an OnClick handler")
	}
	e := &Event{MouseX: 5, MouseY: 5}
	layout.Shape.events.OnClick(EventCtx{&layout, e, w})
	state := dragReorderGet(w, "tree")
	if !state.started {
		t.Fatal("click on reorderable row should arm drag-reorder state")
	}
	if state.itemID != "a" || state.sourceIndex != 0 {
		t.Fatalf("drag state = %+v, want itemID a at index 0", state)
	}
}

func TestTreeRowContent(t *testing.T) {
	// treeRowContent builds the ghost content for a drag — a row
	// container with the tree's icon + text children.
	cfg := TreeCfg{ID: "tree", Spacing: NoSpacing}
	view := treeRowContent(nil,
		cfg,
		treeFlatRow{ID: "a", Text: "Alpha", HasChildren: true},
		16, "a",
	)
	w := newTestWindow()
	layout := generateViewLayout(view, w)
	// treeRowContent is the drag ghost's inner content — no a11y role
	// of its own; it is nested under the floating ghost container.
	if layout.Shape.A11YRole != AccessRoleNone {
		t.Fatalf("ghost row role = %d, want None", layout.Shape.A11YRole)
	}
	// Icon spacer + icon + text.
	if got := len(layout.Children); got != 3 {
		t.Fatalf("ghost row children = %d, want 3", got)
	}
}

func TestTreeScrollTo(t *testing.T) {
	w := newTestWindow()
	treeScrollTo("list", 5, 20, 100, w)
	// Row 5 spans 100..120 in content space; the viewport shows
	// 0..100, so the scroll must bring the row's bottom into view.
	if got := w.scrollY().GetOr("list", 0); got != -20 {
		t.Fatalf("scrollY = %v, want -20", got)
	}

	// Empty scrollID is a no-op, including on an uninitialized window.
	w2 := newTestWindow()
	treeScrollTo("", 5, 20, 100, w2)
	if got := w2.scrollY().GetOr("", 0); got != 0 {
		t.Fatalf("no-op scroll wrote %v", got)
	}

	// Zero row height is a no-op.
	w3 := newTestWindow()
	treeScrollTo("list", 5, 0, 100, w3)
	if got := w3.scrollY().GetOr("list", 0); got != 0 {
		t.Fatalf("zero row height wrote %v", got)
	}
}

// Both row paths paint hover through a HoverStyle, not an OnHover
// closure: the look depends only on the row, so a closure per row per
// frame is waste (#977). The style carries the theme's hover fill and
// the pointing-hand cursor.
func TestTreeRowHoverIsStyleNotClosure(t *testing.T) {
	cfg := TreeCfg{ID: "tree", Spacing: NoSpacing}
	cfg.Colors = cfg.Colors.resolved(cfg.Color, defaultTreeStyle.Colors)
	row := treeFlatRow{ID: "a", Text: "A"}
	views := map[string]View{
		"plain": treeRowView(nil, cfg, row, 16, "",
			&rowClick{owner: newTreeClick(&cfg), id: "a", tree: &row}),
		"drag": treeDragRowView(nil, cfg, row, 16, "",
			treeTestDragClick(&cfg, &row, 0,
				[]string{"a"}, []string{"tree:row:a"}),
			"tree:row:a"),
	}
	for name, view := range views {
		w := newTestWindow()
		w.layout = Layout{Shape: &Shape{ID: "root"}}
		layout := generateViewLayout(view, w)
		ev := layout.Shape.events
		if ev == nil {
			t.Fatalf("%s row has no event handlers", name)
		}
		if ev.OnHover != nil {
			t.Errorf("%s row allocates an OnHover closure", name)
		}
		if ev.hover.Color != cfg.Colors.Hover {
			t.Errorf("%s row hover fill = %v, want %v",
				name, ev.hover.Color, cfg.Colors.Hover)
		}
		if ev.hover.Cursor != CursorPointingHand {
			t.Errorf("%s row hover cursor = %v, want pointing hand",
				name, ev.hover.Cursor)
		}
	}
}

// treeTestDragClick is the click target treeBuildRows gives a
// reorderable root row: siblings and layoutIDs are the root's children.
func treeTestDragClick(
	cfg *TreeCfg, row *treeFlatRow, sibIdx int,
	siblings, layoutIDs []string,
) *rowClick {
	parentOf := make(map[string]string, len(siblings))
	for _, id := range siblings {
		parentOf[id] = ""
	}
	return &rowClick{
		owner: &treeDragClick{
			sel:              *newTreeClick(cfg),
			onReorder:        cfg.OnReorder,
			parentOf:         parentOf,
			siblingsByParent: map[string][]string{"": siblings},
			parentLayoutIDs:  map[string][]string{"": layoutIDs},
			parentMidsOff:    map[string]int{"": 0},
		},
		id:    row.ID,
		index: sibIdx,
		tree:  row,
		cue:   treeRowSound(cfg, *row),
	}
}

// Every reorderable tree row carries the layout ID treeRowID composes
// for it, with nested expanded nodes and with the list scrolled so the
// window starts mid-tree. The rows reuse the IDs treeBuildSiblingInfo
// already composed for drag tracking, indexed by sibling index minus
// the parent's midsOffset; this pins that the two agree.
func TestTreeReorderRowLayoutIDs(t *testing.T) {
	nodes := make([]TreeNodeCfg, 0, 20)
	for i := range 20 {
		s := strconv.Itoa(i)
		kids := make([]TreeNodeCfg, 0, 4)
		for j := range 4 {
			k := s + "-" + strconv.Itoa(j)
			kids = append(kids, TreeNodeCfg{ID: "n-" + k, Text: "Node " + k})
		}
		nodes = append(nodes, TreeNodeCfg{
			ID: "n-" + s, Text: "Node " + s, Nodes: kids})
	}
	for _, scroll := range []float32{0, -900} {
		w := newTestWindow()
		for i := range 20 {
			if i%2 == 0 {
				treeExpandedSet(w, "tree-ids", "n-"+strconv.Itoa(i), true)
			}
		}
		w.scrollY().Set("tree-ids", scroll)
		layout := generateViewLayout(Tree(TreeCfg{
			ID:          "tree-ids",
			MaxHeight:   220,
			Nodes:       nodes,
			Reorderable: true,
			OnReorder:   func(string, string, EventCtx) {},
		}), w)
		got := 0
		firstName := ""
		for _, row := range treeRows(&layout) {
			if row.Shape.A11YRole != AccessRoleTreeItem {
				continue
			}
			name := row.Shape.a11Y.Label
			if firstName == "" {
				firstName = name
			}
			// The label "Node 3-1" belongs to ID "n-3-1".
			want := treeRowID("tree-ids", "n-"+name[len("Node "):])
			if row.Shape.ID != want {
				t.Errorf("scroll %v: row %q ID = %q, want %q",
					scroll, name, row.Shape.ID, want)
			}
			got++
		}
		if got == 0 {
			t.Fatalf("scroll %v: no reorderable rows built", scroll)
		}
		// The scrolled window must start past the first rows, or the
		// midsOffset path goes untested.
		if scroll != 0 && firstName == "Node 0" {
			t.Fatalf("scroll %v: window starts at the top", scroll)
		}
	}
}

// A virtualized tree builds rows only for its visible window, so the
// memory one frame's row build takes must follow the window, not the
// number of expanded rows. Sizing the views or click-target slices by
// every row allocated about 90 bytes per row of data per frame.
func TestTreeBuildRowsMemoryFollowsWindow(t *testing.T) {
	const n = 20000
	flatRows := make([]treeFlatRow, n)
	for i := range flatRows {
		flatRows[i] = treeFlatRow{ID: "n" + strconv.Itoa(i), Text: "x"}
	}
	cfg := &TreeCfg{ID: "tree-mem", Colors: defaultTreeStyle.Colors}
	build := func() {
		_, _ = treeBuildRows(nil, cfg, flatRows, "", 16,
			nil, nil, nil, nil, nil, "",
			false, false, dragReorderState{}, "",
			true, 20, 100, 109)
	}
	build() // warm up lazily built state
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	build()
	runtime.ReadMemStats(&after)
	// A 10-row window needs a few KB. 8 bytes per row of data is far
	// below what a slice sized by the data costs (16 per View alone).
	if got := after.TotalAlloc - before.TotalAlloc; got > n*8 {
		t.Errorf("row build allocated %d bytes for a 10-row window "+
			"over %d rows, want <= %d", got, n, n*8)
	}
}
