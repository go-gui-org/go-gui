package gui

import "testing"

// A plain tree row's click goes through the shared rowOnClick and does
// what the per-row closure did: focus the row, toggle a row with
// children, and report the selection.
func TestRowOnClickTreeRow(t *testing.T) {
	w := newTestWindow()
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	selected := ""
	layout := generateViewLayout(Tree(TreeCfg{
		ID: "tree-click",
		Nodes: []TreeNodeCfg{
			{ID: "a", Text: "A", Nodes: []TreeNodeCfg{{ID: "a1", Text: "A1"}}},
			{ID: "b", Text: "B"},
		},
		OnSelect: func(id string, _ EventCtx) { selected = id },
	}), w)
	rows := treeRows(&layout)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	row := &rows[0]
	if row.Shape.events == nil || row.Shape.events.rowClick == nil {
		t.Fatal("tree row has no click target")
	}
	row.Shape.events.OnClick(EventCtx{row, &Event{}, w})
	if selected != "a" {
		t.Errorf("OnSelect id = %q, want a", selected)
	}
	if !treeExpandedState(w, "tree-click")["a"] {
		t.Error("click did not expand the row")
	}
	if got := StateReadOr(w, nsTreeFocus, "tree-click", ""); got != "a" {
		t.Errorf("tree focus = %q, want a", got)
	}
}

// A listCore row's click reports its ID and filtered index. A disabled
// row answers nothing: no click target, no handler, no hover look.
func TestRowOnClickListCoreRow(t *testing.T) {
	items := []listCoreItem{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta", Disabled: true},
		{ID: "c", Label: "Gamma"},
	}
	gotID, gotIdx := "", -1
	cfg := listCoreCfg{
		Colors: ColorSet{Hover: Blue},
		OnItemClick: func(id string, idx int, _ EventCtx) {
			gotID, gotIdx = id, idx
		},
	}
	w := newTestWindow()
	layout := generateViewLayout(Column(ContainerCfg{
		Content: listCoreViews(items, cfg, 0, 2, -1, nil, 0),
	}), w)
	if len(layout.Children) != 3 {
		t.Fatalf("rows = %d, want 3", len(layout.Children))
	}

	row := &layout.Children[2]
	row.Shape.events.OnClick(EventCtx{row, &Event{}, w})
	if gotID != "c" || gotIdx != 2 {
		t.Errorf("OnItemClick = (%q, %d), want (c, 2)", gotID, gotIdx)
	}
	if !row.Shape.events.hover.isSet() {
		t.Error("enabled row has no hover style")
	}

	disabled := layout.Children[1].Shape.events
	if disabled != nil && (disabled.OnClick != nil ||
		disabled.rowClick != nil || disabled.hover.isSet()) {
		t.Error("disabled row answers a click or takes a hover look")
	}
}

// rowOnClick tolerates the contexts a click slot can be reached with
// that carry no target.
func TestRowOnClickNoTarget(t *testing.T) {
	rowOnClick(EventCtx{})
	rowOnClick(EventCtx{Layout: &Layout{Shape: &Shape{}}})
	rowOnClick(EventCtx{Layout: &Layout{Shape: &Shape{
		events: &eventHandlers{}}}})
	rowOnClick(EventCtx{Layout: &Layout{Shape: &Shape{
		events: &eventHandlers{rowClick: &rowClick{}}}}})
}
