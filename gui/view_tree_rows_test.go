package gui

import "testing"

func TestTreeEstimateRowHeightNilWindow(t *testing.T) {
	// Without a text measurer the estimate falls back to the style
	// size, then adds the fixed padding and the configured spacing.
	cfg := TreeCfg{Spacing: Some(float32(4))}
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
	cfg := TreeCfg{Spacing: Some(float32(0))}
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
	cfg := TreeCfg{ID: "tree", Spacing: Some(float32(0))}
	view := treeDragRowView(
		cfg,
		treeFlatRow{ID: "load", IsLoading: true, Text: "Loading..."},
		16, "",
		0, nil, nil, 0, "",
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
	cfg := TreeCfg{ID: "tree", Spacing: Some(float32(0))}
	view := treeDragRowView(
		cfg,
		treeFlatRow{ID: "a", Text: "A"},
		16, "",
		0, []string{"a"}, []string{"tree:row:a"}, 0, "",
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
	cfg := TreeCfg{ID: "tree", Spacing: Some(float32(0))}
	view := treeRowContent(
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
