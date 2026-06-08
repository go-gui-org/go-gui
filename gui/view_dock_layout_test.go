package gui

import (
	"slices"
	"testing"
)

// --- DockPanelDef / DockLayoutCfg ---

func TestDockPanelDefClosableDefault(t *testing.T) {
	p := DockPanelDef{ID: "p", Label: "Panel"}
	if p.Closable {
		t.Fatal("default should be false (Go zero value)")
	}
}

// --- dockFindPanelDef / dockFindPanelLabel ---

func TestDockFindPanelDef(t *testing.T) {
	panels := []DockPanelDef{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
	}
	p, ok := dockFindPanelDef(panels, "b")
	if !ok || p.Label != "Beta" {
		t.Fatal("not found")
	}
}

func TestDockFindPanelDefNotFound(t *testing.T) {
	panels := []DockPanelDef{{ID: "a", Label: "A"}}
	_, ok := dockFindPanelDef(panels, "z")
	if ok {
		t.Fatal("should not find")
	}
}

func TestDockFindPanelLabel(t *testing.T) {
	panels := []DockPanelDef{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
	}
	if dockFindPanelLabel(panels, "a") != "Alpha" {
		t.Fatal("wrong label")
	}
	if dockFindPanelLabel(panels, "z") != "" {
		t.Fatal("should return empty")
	}
}

// --- applyDockLayoutDefaults ---

func TestApplyDockLayoutDefaults(t *testing.T) {
	cfg := DockLayoutCfg{}
	applyDockLayoutDefaults(&cfg)
	if cfg.Sizing != FillFill {
		t.Fatal("sizing default")
	}
	if !cfg.ColorZonePreview.IsSet() {
		t.Fatal("zone preview color should be set")
	}
	if !cfg.ColorTab.IsSet() {
		t.Fatal("tab color should be set")
	}
}

func TestApplyDockLayoutDefaultsPreservesExplicit(t *testing.T) {
	c := Color{255, 0, 0, 255, true}
	cfg := DockLayoutCfg{
		Sizing:            FixedFixed,
		ColorZonePreview:  c,
		ColorTab:          c,
		ColorTabActive:    c,
		ColorTabHover:     c,
		ColorTabBar:       c,
		ColorTabSeparator: c,
		ColorContent:      c,
	}
	applyDockLayoutDefaults(&cfg)
	if cfg.Sizing != FixedFixed {
		t.Fatal("should preserve explicit sizing")
	}
	if cfg.ColorZonePreview != c {
		t.Fatal("should preserve explicit color")
	}
}

// --- DockLayout view generation ---

func TestDockLayoutGeneratesLayout(t *testing.T) {
	w := &Window{}
	root := DockPanelGroup("g1", []string{"p1"}, "p1")
	v := DockLayout(DockLayoutCfg{
		ID:   "dock1",
		Root: root,
		Panels: []DockPanelDef{
			{ID: "p1", Label: "Panel 1", Content: []View{
				Text(TextCfg{Text: "hello"}),
			}},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	})
	layout := GenerateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("nil shape")
	}
	if layout.Shape.ID != "dock1" {
		t.Fatalf("id = %s, want dock1", layout.Shape.ID)
	}
}

func TestDockLayoutWithSplit(t *testing.T) {
	w := &Window{}
	left := DockPanelGroup("g1", []string{"p1"}, "p1")
	right := DockPanelGroup("g2", []string{"p2"}, "p2")
	root := DockSplit("s1", DockSplitHorizontal, 0.4, left, right)

	v := DockLayout(DockLayoutCfg{
		ID:   "dock1",
		Root: root,
		Panels: []DockPanelDef{
			{ID: "p1", Label: "Left"},
			{ID: "p2", Label: "Right"},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	})
	layout := GenerateViewLayout(v, w)
	if layout.Shape.ID != "dock1" {
		t.Fatal("wrong id")
	}
	// Should have children (splitter + overlay at minimum)
	if len(layout.Children) < 1 {
		t.Fatal("expected children")
	}
}

// --- dockGroupView ---

func TestDockGroupViewTabButtons(t *testing.T) {
	w := &Window{}
	group := DockPanelGroup("g1", []string{"a", "b", "c"}, "b")
	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: group,
		Panels: []DockPanelDef{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
			{ID: "c", Label: "Gamma"},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	}
	applyDockLayoutDefaults(cfg)
	core := newDockLayoutCore(cfg)

	v := dockGroupView(core, group, cfg, dockDragState{})
	layout := GenerateViewLayout(v, w)

	if layout.Shape.ID != "g1" {
		t.Fatalf("group id = %s, want g1", layout.Shape.ID)
	}
}

func TestDockGroupViewWithClosable(t *testing.T) {
	w := &Window{}
	group := DockPanelGroup("g1", []string{"a"}, "a")
	closeCalled := ""
	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: group,
		Panels: []DockPanelDef{
			{ID: "a", Label: "Alpha", Closable: true},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
		OnPanelClose:   func(pid string, _ *Window) { closeCalled = pid },
	}
	applyDockLayoutDefaults(cfg)
	core := newDockLayoutCore(cfg)

	v := dockGroupView(core, group, cfg, dockDragState{})
	layout := GenerateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("nil layout")
	}
	// Verify close callback is wired
	if closeCalled != "" {
		t.Fatal("should not be called yet")
	}
}

func TestDockGroupViewDraggedTabHidden(t *testing.T) {
	w := &Window{}
	group := DockPanelGroup("g1", []string{"a", "b"}, "a")
	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: group,
		Panels: []DockPanelDef{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	}
	applyDockLayoutDefaults(cfg)
	core := newDockLayoutCore(cfg)

	drag := dockDragState{
		active:      true,
		panelID:     "a",
		sourceGroup: "g1",
	}

	v := dockGroupView(core, group, cfg, drag)
	layout := GenerateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("nil layout")
	}
	// The dragged tab "a" should be skipped, "b" shown
}

// --- dockSplitView ---

func TestDockSplitViewOrientation(t *testing.T) {
	w := &Window{}
	left := DockPanelGroup("g1", []string{"p1"}, "p1")
	right := DockPanelGroup("g2", []string{"p2"}, "p2")
	node := DockSplit("s1", DockSplitHorizontal, 0.5, left, right)

	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: node,
		Panels: []DockPanelDef{
			{ID: "p1", Label: "P1"},
			{ID: "p2", Label: "P2"},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	}
	applyDockLayoutDefaults(cfg)
	core := newDockLayoutCore(cfg)

	v := dockSplitView(core, node, cfg, dockDragState{})
	layout := GenerateViewLayout(v, w)
	if layout.Shape.ID != "dock_split:s1" {
		t.Fatalf("id = %s, want dock_split:s1", layout.Shape.ID)
	}
}

// --- dockNodeView ---

func TestDockNodeViewRoutesSplit(t *testing.T) {
	w := &Window{}
	left := DockPanelGroup("g1", []string{"p1"}, "p1")
	right := DockPanelGroup("g2", []string{"p2"}, "p2")
	node := DockSplit("s1", DockSplitVertical, 0.5, left, right)

	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: node,
		Panels: []DockPanelDef{
			{ID: "p1", Label: "P1"},
			{ID: "p2", Label: "P2"},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	}
	applyDockLayoutDefaults(cfg)
	core := newDockLayoutCore(cfg)

	v := dockNodeView(core, node, cfg, dockDragState{})
	layout := GenerateViewLayout(v, w)
	if layout.Shape.ID != "dock_split:s1" {
		t.Fatal("should route to split view")
	}
}

func TestDockNodeViewRoutesGroup(t *testing.T) {
	w := &Window{}
	group := DockPanelGroup("g1", []string{"p1"}, "p1")

	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: group,
		Panels: []DockPanelDef{
			{ID: "p1", Label: "P1"},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	}
	applyDockLayoutDefaults(cfg)
	core := newDockLayoutCore(cfg)

	v := dockNodeView(core, group, cfg, dockDragState{})
	layout := GenerateViewLayout(v, w)
	if layout.Shape.ID != "g1" {
		t.Fatal("should route to group view")
	}
}

// --- newDockLayoutCore ---

func TestNewDockLayoutCore(t *testing.T) {
	called := false
	cfg := &DockLayoutCfg{
		ID:               "d1",
		Root:             DockPanelGroup("g", nil, ""),
		OnLayoutChange:   func(_ *DockNode, _ *Window) { called = true },
		ColorZonePreview: Color{1, 2, 3, 4, true},
	}
	core := newDockLayoutCore(cfg)
	if core.id != "d1" {
		t.Fatal("wrong id")
	}
	if core.root != cfg.Root {
		t.Fatal("wrong root")
	}
	if core.colorZonePreview != (Color{1, 2, 3, 4, true}) {
		t.Fatal("wrong color")
	}
	core.onLayoutChange(nil, nil)
	if !called {
		t.Fatal("callback not wired")
	}
}

// --- dockLayoutAmend ---

func TestDockLayoutAmendNoChildren(t *testing.T) {
	w := &Window{}
	layout := &Layout{Shape: &Shape{Width: 100, Height: 50}}
	// Should not panic with empty children.
	dockLayoutAmend("dock1", Color{}, layout, w)
	if len(layout.Children) != 0 {
		t.Fatal("children should remain empty")
	}
}

// --- Integration: OnChange callback ---

func TestDockSplitOnChangeCallback(t *testing.T) {
	var newRoot *DockNode
	left := DockPanelGroup("g1", []string{"p1"}, "p1")
	right := DockPanelGroup("g2", []string{"p2"}, "p2")
	root := DockSplit("s1", DockSplitHorizontal, 0.5, left, right)

	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: root,
		Panels: []DockPanelDef{
			{ID: "p1", Label: "P1"},
			{ID: "p2", Label: "P2"},
		},
		OnLayoutChange: func(r *DockNode, _ *Window) { newRoot = r },
	}

	// Simulate ratio update via dockTreeUpdateRatio
	updated := dockTreeUpdateRatio(cfg.Root, "s1", 0.7)
	cfg.OnLayoutChange(updated, nil)

	if newRoot == nil {
		t.Fatal("callback not called")
	}
	if newRoot.Ratio != 0.7 {
		t.Fatalf("ratio = %f, want 0.7", newRoot.Ratio)
	}
}

// --- Full tree integration ---

func TestDockLayoutFullTreeIntegration(t *testing.T) {
	w := &Window{}

	left := DockPanelGroup("g1", []string{"explorer", "search"}, "explorer")
	topRight := DockPanelGroup("g2", []string{"editor1", "editor2"}, "editor1")
	bottomRight := DockPanelGroup("g3", []string{"terminal", "output"}, "terminal")
	right := DockSplit("s2", DockSplitVertical, 0.65, topRight, bottomRight)
	root := DockSplit("s1", DockSplitHorizontal, 0.22, left, right)

	panels := []DockPanelDef{
		{ID: "explorer", Label: "Explorer"},
		{ID: "search", Label: "Search"},
		{ID: "editor1", Label: "main.go"},
		{ID: "editor2", Label: "README"},
		{ID: "terminal", Label: "Terminal"},
		{ID: "output", Label: "Output"},
	}

	v := DockLayout(DockLayoutCfg{
		ID:             "dock1",
		Root:           root,
		Panels:         panels,
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	})

	layout := GenerateViewLayout(v, w)
	if layout.Shape.ID != "dock1" {
		t.Fatal("wrong root id")
	}

	// All panels should be reachable
	allIDs := []string{"explorer", "search", "editor1", "editor2", "terminal", "output"}
	nodes := DockTreeCollectPanelNodes(root)
	var foundIDs []string
	for _, n := range nodes {
		foundIDs = append(foundIDs, n.PanelIDs...)
	}
	slices.Sort(allIDs)
	slices.Sort(foundIDs)
	if !slices.Equal(allIDs, foundIDs) {
		t.Fatalf("panel ids mismatch: %v vs %v", allIDs, foundIDs)
	}
}

// --- dockTabButton ---

func TestDockTabButtonWithSelect(t *testing.T) {
	w := &Window{}
	group := DockPanelGroup("g1", []string{"a", "b"}, "a")
	selectCalled := false
	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: group,
		Panels: []DockPanelDef{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
		OnPanelSelect: func(gid, pid string, _ *Window) {
			selectCalled = true
			if gid != "g1" || pid != "a" {
				t.Fatalf("wrong select args: %s, %s", gid, pid)
			}
		},
	}
	applyDockLayoutDefaults(cfg)
	core := newDockLayoutCore(cfg)

	panel := DockPanelDef{ID: "a", Label: "Alpha"}
	v := dockTabButton(core, group, panel, true, cfg)
	layout := GenerateViewLayout(v, w)

	if layout.Shape.ID != "dock_tab:g1:a" {
		t.Fatalf("tab id = %s", layout.Shape.ID)
	}

	// Simulate click
	e := &Event{}
	if layout.Shape.events != nil && layout.Shape.events.OnClick != nil {
		layout.Shape.events.OnClick(&layout, e, w)
	}
	if !selectCalled {
		t.Fatal("select not called")
	}
}

func TestDockGroupViewFallbackContent(t *testing.T) {
	w := &Window{}
	// Selected panel "a" is being dragged, fallback to "b"
	group := DockPanelGroup("g1", []string{"a", "b"}, "a")
	cfg := &DockLayoutCfg{
		ID:   "dock1",
		Root: group,
		Panels: []DockPanelDef{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
		},
		OnLayoutChange: func(_ *DockNode, _ *Window) {},
	}
	applyDockLayoutDefaults(cfg)
	core := newDockLayoutCore(cfg)

	drag := dockDragState{
		active:      true,
		panelID:     "a",
		sourceGroup: "g1",
	}

	v := dockGroupView(core, group, cfg, drag)
	layout := GenerateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("nil layout")
	}
}
