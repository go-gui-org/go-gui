package gui

import "testing"

func requireInspector(t *testing.T) {
	t.Helper()
	if !inspectorSupported {
		t.Skip("inspector disabled in prod build")
	}
}

func TestInspectorBuildTreeNodes(t *testing.T) {
	requireInspector(t)
	root := Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Axis:      AxisTopToBottom,
				Width:     200,
				Height:    100,
				ID:        "root",
				Padding:   PaddingTwoFive,
				Spacing:   8,
				Color:     Red,
				Opacity:   0.5,
				TC: &ShapeTextConfig{
					Text: "hello world",
				},
				events: &eventHandlers{
					OnClick: func(ctx EventCtx) {},
				},
			},
			Children: []Layout{{
				Shape: &Shape{
					shapeType: shapeText,
					Width:     40,
					Height:    12,
					ID:        "child",
				},
			}},
		}},
	}

	props := make(map[string]inspectorNodeProps)
	nodes := inspectorBuildTreeNodes(nil, &root, "0", props)
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if nodes[0].ID != "0" {
		t.Fatalf("nodes[0].ID = %q, want %q", nodes[0].ID, "0")
	}
	if len(nodes[0].Nodes) < 3 {
		t.Fatalf("selected node should include props plus child, got %d children", len(nodes[0].Nodes))
	}
	if got := nodes[0].Text; got != "column 200x100 #root" {
		t.Fatalf("nodes[0].Text = %q, want %q", got, "column 200x100 #root")
	}
	if got := props["0"].TextPreview; got != "hello world" {
		t.Fatalf("props[0].TextPreview = %q, want %q", got, "hello world")
	}
	if got := props["0"].events; got != "click" {
		t.Fatalf("props[0].events = %q, want %q", got, "click")
	}
	if got := props["0"].Children; got != 1 {
		t.Fatalf("props[0].Children = %d, want 1", got)
	}
}

func TestInspectorPickPathDeepestReverseOrder(t *testing.T) {
	requireInspector(t)
	root := Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 100, Height: 100},
			},
			Children: []Layout{
				{
					Shape: &Shape{
						shapeClip: drawClip{X: 0, Y: 0, Width: 80, Height: 80},
					},
				},
				{
					Shape: &Shape{
						shapeClip: drawClip{X: 10, Y: 10, Width: 30, Height: 30},
					},
				},
			},
		}},
	}

	if got := inspectorPickPath(&root, 20, 20); got != "0:1" {
		t.Fatalf("inspectorPickPath() = %q, want %q", got, "0:1")
	}
	if got := inspectorPickPath(&root, 90, 90); got != "0" {
		t.Fatalf("inspectorPickPath() = %q, want %q", got, "0")
	}
}

func TestInspectorSelectExpandsAncestorsAndFocusesTree(t *testing.T) {
	requireInspector(t)
	w := newTestWindow()

	inspectorSelect("0:2:1", w)

	if got := inspectorSelectedPath(w); got != "0:2:1" {
		t.Fatalf("selected path = %q, want %q", got, "0:2:1")
	}
	if got := StateReadOr(w, nsTreeFocus, inspectorTreeID, ""); got != "0:2:1" {
		t.Fatalf("tree focus = %q, want %q", got, "0:2:1")
	}
	expanded := treeExpandedState(w, inspectorTreeID)
	for _, id := range []string{"0", "0:2", "0:2:1"} {
		if !expanded[id] {
			t.Fatalf("expanded[%q] = false, want true", id)
		}
	}
}

func TestInspectorInjectWireframe(t *testing.T) {
	requireInspector(t)
	w := newTestWindow()
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape: &Shape{
				X:       10,
				Y:       20,
				Width:   100,
				Height:  50,
				Radius:  6,
				Padding: NewPadding(1, 2, 3, 4),
			},
		}},
	}
	StateMap[string, string](w, nsInspector, capInspector).
		Set("selected", "0")

	inspectorInjectWireframe(w)

	if len(w.renderers) != 2 {
		t.Fatalf("len(renderers) = %d, want 2", len(w.renderers))
	}
	if w.renderers[0].Kind != RenderStrokeRect {
		t.Fatalf("renderers[0].Kind = %v, want RenderStrokeRect", w.renderers[0].Kind)
	}
	if w.renderers[1].X != 14 || w.renderers[1].Y != 21 {
		t.Fatalf("inner rect pos = (%.0f, %.0f), want (14, 21)", w.renderers[1].X, w.renderers[1].Y)
	}
	if w.renderers[1].W != 94 || w.renderers[1].H != 46 {
		t.Fatalf("inner rect size = (%.0f, %.0f), want (94, 46)", w.renderers[1].W, w.renderers[1].H)
	}
}

func TestInspectorInjectWireframeDeepPath(t *testing.T) {
	requireInspector(t)
	w := newTestWindow()
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape: &Shape{
				X:      10,
				Y:      20,
				Width:  200,
				Height: 100,
			},
			Children: []Layout{
				{
					Shape: &Shape{
						X:      30,
						Y:      40,
						Width:  60,
						Height: 50,
					},
				},
				{
					Shape: &Shape{
						X:      100,
						Y:      40,
						Width:  80,
						Height: 60,
					},
					Children: []Layout{{
						Shape: &Shape{
							X:       120,
							Y:       60,
							Width:   40,
							Height:  30,
							Padding: NewPadding(1, 2, 3, 4),
						},
					}},
				},
			},
		}},
	}
	StateMap[string, string](w, nsInspector, capInspector).
		Set("selected", "0:1:0")

	inspectorInjectWireframe(w)

	if len(w.renderers) != 2 {
		t.Fatalf("len(renderers) = %d, want 2", len(w.renderers))
	}
	if w.renderers[0].Kind != RenderStrokeRect {
		t.Fatalf("renderers[0].Kind = %v, want RenderStrokeRect", w.renderers[0].Kind)
	}
	if w.renderers[0].X != 120 || w.renderers[0].Y != 60 ||
		w.renderers[0].W != 40 || w.renderers[0].H != 30 {
		t.Fatalf("wireframe rect = (%.0f,%.0f,%.0f,%.0f), want (120,60,40,30)",
			w.renderers[0].X, w.renderers[0].Y,
			w.renderers[0].W, w.renderers[0].H)
	}
	if w.renderers[1].X != 124 || w.renderers[1].Y != 61 ||
		w.renderers[1].W != 34 || w.renderers[1].H != 26 {
		t.Fatalf("padding rect = (%.0f,%.0f,%.0f,%.0f), want (124,61,34,26)",
			w.renderers[1].X, w.renderers[1].Y,
			w.renderers[1].W, w.renderers[1].H)
	}
}

func TestInspectorFindByPathErrors(t *testing.T) {
	requireInspector(t)
	root := Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape:    &Shape{},
			Children: []Layout{{Shape: &Shape{}}, {Shape: &Shape{}}},
		}},
	}

	cases := []struct {
		path string
	}{
		{"0:2"},   // out of range
		{"0:x"},   // non-numeric segment
		{"0:-1"},  // negative index
		{"1:0"},   // out of range at first level
		{"0:0:0"}, // too deep
	}
	for _, tc := range cases {
		if node, ok := inspectorFindByPath(&root, tc.path); ok || node != nil {
			t.Fatalf("inspectorFindByPath(%q) = (%v, %v), want (nil, false)", tc.path, node, ok)
		}
	}
	if node, ok := inspectorFindByPath(nil, "0"); ok || node != nil {
		t.Fatalf("inspectorFindByPath(nil) = (%v, %v), want (nil, false)", node, ok)
	}
	if node, ok := inspectorFindByPath(&root, ""); ok || node != nil {
		t.Fatalf("inspectorFindByPath(\"\") = (%v, %v), want (nil, false)", node, ok)
	}

	node, ok := inspectorFindByPath(&root, "0:1")
	if !ok || node == nil {
		t.Fatalf("inspectorFindByPath(\"0:1\") = (nil, %v), want (node, true)", ok)
	}
}

func TestLayoutArrangeWithInspector(t *testing.T) {
	requireInspector(t)
	w := newTestWindow()
	w.windowWidth = 400
	w.windowHeight = 300
	w.inspectorEnabled = true

	layout := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     400,
			Height:    300,
			Sizing:    FillFill,
			Opacity:   1,
		},
	}
	layouts := layoutArrange(&layout, w)
	if len(layouts) < 2 {
		t.Fatalf("expected inspector floating layer, got %d layouts", len(layouts))
	}
}

func TestUpdateCachesInspectorTreeFromPreviousLayout(t *testing.T) {
	requireInspector(t)
	w := &Window{
		windowWidth:   400,
		windowHeight:  300,
		refreshLayout: true,
		windowInspector: windowInspector{
			inspectorEnabled: true,
		},
		viewGenerator: func(*Window) View {
			return Rectangle(RectangleCfg{Width: 10, Height: 10, Color: Blue})
		},
	}
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Axis:      AxisTopToBottom,
				Width:     50,
				Height:    20,
				ID:        "old",
			},
		}},
	}
	StateMap[string, string](w, nsInspector, capInspector).
		Set("selected", "0")

	w.Update()

	if len(w.inspectorTreeCache) != 1 {
		t.Fatalf("len(inspectorTreeCache) = %d, want 1", len(w.inspectorTreeCache))
	}
	if got := w.inspectorTreeCache[0].Text; got != "column 50x20 #old" {
		t.Fatalf("inspectorTreeCache[0].Text = %q, want %q", got, "column 50x20 #old")
	}
	if _, ok := w.inspectorPropsCache["0"]; !ok {
		t.Fatal("inspectorPropsCache should contain previous root path")
	}
}

func TestEventFnInspectorHotkeysAndPick(t *testing.T) {
	requireInspector(t)
	w := newEventTestWindow()
	w.layout = Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape: &Shape{
				shapeClip: drawClip{X: 0, Y: 0, Width: 800, Height: 600},
			},
			Children: []Layout{{
				Shape: &Shape{
					shapeClip: drawClip{X: 360, Y: 20, Width: 40, Height: 40},
				},
			}},
		}},
	}

	w.EventFn(&Event{Type: EventKeyDown, KeyCode: KeyF12})
	if !w.inspectorEnabled {
		t.Fatal("F12 should enable inspector")
	}

	w.EventFn(&Event{Type: EventKeyDown, KeyCode: KeyLeft, Modifiers: ModAlt})
	if got := inspectorPanelWidth(w); got != inspectorPanelMinWidth+inspectorResizeStep {
		t.Fatalf("panel width = %.0f, want %.0f", got, inspectorPanelMinWidth+inspectorResizeStep)
	}

	w.EventFn(&Event{Type: EventKeyDown, KeyCode: KeyUp, Modifiers: ModAlt})
	if !inspectorIsLeft(w) {
		t.Fatal("Alt+Up should move inspector to the left")
	}

	w.EventFn(&Event{Type: EventMouseDown, MouseX: 361, MouseY: 30})
	if got := inspectorSelectedPath(w); got != "0:0" {
		t.Fatalf("selected path = %q, want %q", got, "0:0")
	}
}

func TestInspectorLazyBuilding(t *testing.T) {
	requireInspector(t)
	root := Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape: &Shape{ID: "root"},
			Children: []Layout{
				{Shape: &Shape{ID: "child1"}},
				{Shape: &Shape{ID: "child2"}},
			},
		}},
	}

	// Case 1: Collapsed root. Should only see root and a dummy child.
	props := make(map[string]inspectorNodeProps)
	nodes := inspectorBuildTreeNodes(nil, &root, "", props)
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if len(nodes[0].Nodes) != 1 {
		t.Fatalf("collapsed node should have 1 dummy child, got %d", len(nodes[0].Nodes))
	}
	if nodes[0].Nodes[0].ID != "0:__dummy__" {
		t.Fatalf("dummy child ID = %q, want %q", nodes[0].Nodes[0].ID, "0:__dummy__")
	}

	// Case 2: Expanded root via window state.
	w := newTestWindow()
	treeExpandedSet(w, inspectorTreeID, "0", true)
	nodes = inspectorBuildTreeNodes(w, &root, "", props)
	if len(nodes[0].Nodes) != 2 {
		t.Fatalf("expanded node should have 2 children, got %d", len(nodes[0].Nodes))
	}
}
