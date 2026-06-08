package gui

import "testing"

// --- dockClassifyZone ---

func TestDockClassifyZoneCenter(t *testing.T) {
	z := dockClassifyZone(0.5, 0.5)
	if z != DockDropCenter {
		t.Fatalf("got %d, want center", z)
	}
}

func TestDockClassifyZoneTop(t *testing.T) {
	z := dockClassifyZone(0.5, 0.1)
	if z != DockDropTop {
		t.Fatalf("got %d, want top", z)
	}
}

func TestDockClassifyZoneBottom(t *testing.T) {
	z := dockClassifyZone(0.5, 0.9)
	if z != DockDropBottom {
		t.Fatalf("got %d, want bottom", z)
	}
}

func TestDockClassifyZoneLeft(t *testing.T) {
	z := dockClassifyZone(0.1, 0.5)
	if z != DockDropLeft {
		t.Fatalf("got %d, want left", z)
	}
}

func TestDockClassifyZoneRight(t *testing.T) {
	z := dockClassifyZone(0.9, 0.5)
	if z != DockDropRight {
		t.Fatalf("got %d, want right", z)
	}
}

func TestDockClassifyZoneEdgeBoundary(t *testing.T) {
	// Exactly at edge ratio boundary
	edge := dockDragEdgeRatio
	z := dockClassifyZone(edge, edge)
	if z != DockDropCenter {
		t.Fatalf("at boundary should be center, got %d", z)
	}
}

func TestDockClassifyZoneTopLeftCorner(t *testing.T) {
	// Top takes priority over left when both in edge
	z := dockClassifyZone(0.1, 0.1)
	if z != DockDropTop {
		t.Fatalf("top-left corner should be top, got %d", z)
	}
}

// --- DockDragState get/set/clear ---

func TestDockDragGetSetClear(t *testing.T) {
	w := &Window{}

	// Initially empty.
	state := dockDragGet(w, "d1")
	if state.active {
		t.Fatal("should not be active")
	}

	// Set.
	dockDragSet(w, "d1", dockDragState{
		active:  true,
		panelID: "p1",
		mouseX:  100,
		mouseY:  200,
	})
	state = dockDragGet(w, "d1")
	if !state.active || state.panelID != "p1" {
		t.Fatal("state not stored")
	}
	if state.mouseX != 100 || state.mouseY != 200 {
		t.Fatal("position not stored")
	}

	// Clear.
	dockDragClear(w, "d1")
	state = dockDragGet(w, "d1")
	if state.active {
		t.Fatal("should be cleared")
	}
}

func TestDockDragMultipleDocks(t *testing.T) {
	w := &Window{}
	dockDragSet(w, "d1", dockDragState{panelID: "p1"})
	dockDragSet(w, "d2", dockDragState{panelID: "p2"})

	s1 := dockDragGet(w, "d1")
	s2 := dockDragGet(w, "d2")
	if s1.panelID != "p1" || s2.panelID != "p2" {
		t.Fatal("separate docks should be independent")
	}
}

// --- dockDragDetectZone ---

func TestDockDragDetectZoneNoLayout(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "other"}}
	zone, _ := dockDragDetectZone("dock1", nil, 50, 50, "", w)
	if zone != DockDropNone {
		t.Fatal("should be none with no matching layout")
	}
}

func TestDockDragDetectZoneWindowEdges(t *testing.T) {
	w := &Window{}
	// Set up a layout with known clip.
	w.layout = Layout{
		Shape: &Shape{
			ID:        "dock1",
			shapeClip: drawClip{X: 0, Y: 0, Width: 800, Height: 600},
		},
	}

	tests := []struct {
		x, y float32
		want DockDropZone
	}{
		{5, 300, DockDropWindowLeft},
		{795, 300, DockDropWindowRight},
		{400, 5, DockDropWindowTop},
		{400, 595, DockDropWindowBottom},
	}
	for _, tc := range tests {
		zone, _ := dockDragDetectZone("dock1", nil, tc.x, tc.y, "", w)
		if zone != tc.want {
			t.Errorf("(%g,%g): got %d, want %d", tc.x, tc.y, zone, tc.want)
		}
	}
}

func TestDockDragDetectZoneGroupZone(t *testing.T) {
	w := &Window{}

	groupNode := DockPanelGroup("g1", []string{"A", "B"}, "A")

	// Layout tree: dock1 -> g1
	groupLayout := Layout{
		Shape: &Shape{
			ID:        "g1",
			shapeClip: drawClip{X: 100, Y: 100, Width: 400, Height: 300},
		},
	}
	w.layout = Layout{
		Shape: &Shape{
			ID:        "dock1",
			shapeClip: drawClip{X: 0, Y: 0, Width: 800, Height: 600},
		},
		Children: []Layout{groupLayout},
	}

	// Center of group
	zone, gid := dockDragDetectZone(
		"dock1", []*DockNode{groupNode},
		300, 250, "other", w)
	if zone != DockDropCenter || gid != "g1" {
		t.Fatalf("center: zone=%d, gid=%s", zone, gid)
	}

	// Top of group
	zone, gid = dockDragDetectZone(
		"dock1", []*DockNode{groupNode},
		300, 110, "other", w)
	if zone != DockDropTop || gid != "g1" {
		t.Fatalf("top: zone=%d, gid=%s", zone, gid)
	}
}

func TestDockDragDetectZoneSkipSinglePanelSource(t *testing.T) {
	w := &Window{}

	groupNode := DockPanelGroup("g1", []string{"A"}, "A")

	groupLayout := Layout{
		Shape: &Shape{
			ID:        "g1",
			shapeClip: drawClip{X: 100, Y: 100, Width: 400, Height: 300},
		},
	}
	w.layout = Layout{
		Shape: &Shape{
			ID:        "dock1",
			shapeClip: drawClip{X: 0, Y: 0, Width: 800, Height: 600},
		},
		Children: []Layout{groupLayout},
	}

	// Dragging from g1 which only has 1 panel — skip
	zone, _ := dockDragDetectZone(
		"dock1", []*DockNode{groupNode},
		300, 250, "g1", w)
	if zone != DockDropNone {
		t.Fatal("should skip single-panel source group")
	}
}

func TestDockDragDetectZoneSkipCenterSameGroup(t *testing.T) {
	w := &Window{}

	groupNode := DockPanelGroup("g1", []string{"A", "B"}, "A")

	groupLayout := Layout{
		Shape: &Shape{
			ID:        "g1",
			shapeClip: drawClip{X: 100, Y: 100, Width: 400, Height: 300},
		},
	}
	w.layout = Layout{
		Shape: &Shape{
			ID:        "dock1",
			shapeClip: drawClip{X: 0, Y: 0, Width: 800, Height: 600},
		},
		Children: []Layout{groupLayout},
	}

	// Center of same group — skip (already a tab)
	zone, _ := dockDragDetectZone(
		"dock1", []*DockNode{groupNode},
		300, 250, "g1", w)
	if zone != DockDropNone {
		t.Fatal("should skip center drop on same group")
	}
}

// --- dockDragAmendOverlay ---

func TestDockDragAmendOverlayInactive(t *testing.T) {
	w := &Window{}
	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 800, Height: 600},
		Children: []Layout{
			{Shape: &Shape{ID: "dock_zone_overlay", Width: 0, Height: 0}},
		},
	}
	// No active drag — overlay should stay at zero size.
	dockDragAmendOverlay("dock1", Color{70, 130, 220, 80, true}, layout, w)
	if layout.Children[0].Shape.Width != 0 {
		t.Fatal("overlay should not change when inactive")
	}
}

func TestDockDragAmendOverlayWindowTop(t *testing.T) {
	w := &Window{}
	dockDragSet(w, "dock1", dockDragState{
		active:    true,
		hoverZone: DockDropWindowTop,
	})

	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 800, Height: 600},
		Children: []Layout{
			{Shape: &Shape{ID: "dock_zone_overlay"}},
		},
	}
	colorZone := Color{70, 130, 220, 80, true}
	dockDragAmendOverlay("dock1", colorZone, layout, w)

	ov := layout.Children[0].Shape
	if ov.Width != 800 {
		t.Fatalf("overlay width = %f, want 800", ov.Width)
	}
	if ov.Height != 300 {
		t.Fatalf("overlay height = %f, want 300", ov.Height)
	}
	if ov.Y != 0 {
		t.Fatalf("overlay y = %f, want 0", ov.Y)
	}
}

func TestDockDragAmendOverlayGroupRight(t *testing.T) {
	w := &Window{}

	groupLayout := Layout{
		Shape: &Shape{
			ID:        "g1",
			X:         100,
			Y:         50,
			Width:     400,
			Height:    300,
			shapeClip: drawClip{X: 100, Y: 50, Width: 400, Height: 300},
		},
	}

	dockDragSet(w, "dock1", dockDragState{
		active:       true,
		hoverZone:    DockDropRight,
		hoverGroupID: "g1",
	})

	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 800, Height: 600},
		Children: []Layout{
			groupLayout,
			{Shape: &Shape{ID: "dock_zone_overlay"}},
		},
	}
	dockDragAmendOverlay("dock1", Color{70, 130, 220, 80, true}, layout, w)

	ov := layout.Children[1].Shape
	if ov.X != 300 {
		t.Fatalf("overlay x = %f, want 300", ov.X)
	}
	if ov.Width != 200 {
		t.Fatalf("overlay width = %f, want 200", ov.Width)
	}
}

// --- Ghost view ---

func TestDockDragGhostView(t *testing.T) {
	state := dockDragState{
		mouseX:      150,
		mouseY:      200,
		startMouseX: 100,
		startMouseY: 100,
		parentX:     50,
		parentY:     50,
		ghostW:      120,
		ghostH:      30,
	}
	v := dockDragGhostView(state, "Test Panel")
	if v == nil {
		t.Fatal("nil view")
	}
}

// --- Zone overlay view ---

func TestDockDragZoneOverlayView(t *testing.T) {
	v := dockDragZoneOverlayView(Color{70, 130, 220, 80, true})
	if v == nil {
		t.Fatal("nil view")
	}
}

// --- DockDropZone constants ---

func TestDockDropZoneValues(t *testing.T) {
	if DockDropNone != 0 {
		t.Fatal("DockDropNone should be 0")
	}
	// Verify all zones have distinct values
	zones := []DockDropZone{
		DockDropNone, DockDropCenter, DockDropTop,
		DockDropBottom, DockDropLeft, DockDropRight,
		DockDropWindowTop, DockDropWindowBottom,
		DockDropWindowLeft, DockDropWindowRight,
	}
	seen := make(map[DockDropZone]bool)
	for _, z := range zones {
		if seen[z] {
			t.Fatalf("duplicate zone value: %d", z)
		}
		seen[z] = true
	}
}
