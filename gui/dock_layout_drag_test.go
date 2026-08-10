package gui

import "testing"

// --- dockClassifyZone ---

func TestDockClassifyZoneCenter(t *testing.T) {
	z := dockClassifyZone(0.5, 0.5)
	if z != dockDropCenter {
		t.Fatalf("got %d, want center", z)
	}
}

func TestDockClassifyZoneTop(t *testing.T) {
	z := dockClassifyZone(0.5, 0.1)
	if z != dockDropTop {
		t.Fatalf("got %d, want top", z)
	}
}

func TestDockClassifyZoneBottom(t *testing.T) {
	z := dockClassifyZone(0.5, 0.9)
	if z != dockDropBottom {
		t.Fatalf("got %d, want bottom", z)
	}
}

func TestDockClassifyZoneLeft(t *testing.T) {
	z := dockClassifyZone(0.1, 0.5)
	if z != dockDropLeft {
		t.Fatalf("got %d, want left", z)
	}
}

func TestDockClassifyZoneRight(t *testing.T) {
	z := dockClassifyZone(0.9, 0.5)
	if z != dockDropRight {
		t.Fatalf("got %d, want right", z)
	}
}

func TestDockClassifyZoneEdgeBoundary(t *testing.T) {
	// Exactly at edge ratio boundary
	edge := dockDragEdgeRatio
	z := dockClassifyZone(edge, edge)
	if z != dockDropCenter {
		t.Fatalf("at boundary should be center, got %d", z)
	}
}

func TestDockClassifyZoneTopLeftCorner(t *testing.T) {
	// Top takes priority over left when both in edge
	z := dockClassifyZone(0.1, 0.1)
	if z != dockDropTop {
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

// Issue #110: losing mouse capture over a drop zone must not dock the
// panel. Only releasing the button is a drop. Run the same state both
// ways so the contrast, not just the silence, is asserted.
func TestDockDragCaptureLossDoesNotDrop(t *testing.T) {
	setup := func(dropped *bool) *Window {
		w := &Window{}
		w.layout = Layout{Shape: &Shape{ID: "dock"}}
		root := DockPanelGroup("g1", []string{"p1", "p2"}, "p1")
		dockDragStart("dock", "p1", "g1", root,
			func(*DockNode, EventCtx) { *dropped = true },
			&Layout{Shape: &Shape{ID: "tab"}}, &Event{}, w)
		// Hovering a drop zone: a release here would dock the panel.
		state := dockDragGet(w, "dock")
		state.active = true
		state.hoverZone = dockDropRight
		state.hoverGroupID = "g1"
		dockDragSet(w, "dock", state)
		return w
	}

	upDropped := false
	wUp := setup(&upDropped)
	mouseUpHandler(&Layout{Shape: &Shape{}}, &Event{}, wUp)
	if !upDropped {
		t.Fatal("setup does not drop on release; the cancel case proves nothing")
	}

	dropped := false
	w := setup(&dropped)
	w.MouseCancel()
	if dropped {
		t.Error("panel docked by a drag the user never released")
	}
	if w.mouseIsLocked() {
		t.Error("mouse still locked after capture loss")
	}
	if dockDragGet(w, "dock").active {
		t.Error("drag state left active; the drop-zone overlay would persist")
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
	if zone != dockDropNone {
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
		{5, 300, dockDropWindowLeft},
		{795, 300, dockDropWindowRight},
		{400, 5, dockDropWindowTop},
		{400, 595, dockDropWindowBottom},
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
	if zone != dockDropCenter || gid != "g1" {
		t.Fatalf("center: zone=%d, gid=%s", zone, gid)
	}

	// Top of group
	zone, gid = dockDragDetectZone(
		"dock1", []*DockNode{groupNode},
		300, 110, "other", w)
	if zone != dockDropTop || gid != "g1" {
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
	if zone != dockDropNone {
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
	if zone != dockDropNone {
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
	dockDragAmendOverlay("dock1", RGBA(70, 130, 220, 80), layout, w)
	if layout.Children[0].Shape.Width != 0 {
		t.Fatal("overlay should not change when inactive")
	}
}

func TestDockDragAmendOverlayWindowTop(t *testing.T) {
	w := &Window{}
	dockDragSet(w, "dock1", dockDragState{
		active:    true,
		hoverZone: dockDropWindowTop,
	})

	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 800, Height: 600},
		Children: []Layout{
			{Shape: &Shape{ID: "dock_zone_overlay"}},
		},
	}
	colorZone := RGBA(70, 130, 220, 80)
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
		hoverZone:    dockDropRight,
		hoverGroupID: "g1",
	})

	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 800, Height: 600},
		Children: []Layout{
			groupLayout,
			{Shape: &Shape{ID: "dock_zone_overlay"}},
		},
	}
	dockDragAmendOverlay("dock1", RGBA(70, 130, 220, 80), layout, w)

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
	v := dockDragZoneOverlayView(RGBA(70, 130, 220, 80))
	if v == nil {
		t.Fatal("nil view")
	}
}

// --- DockDropZone constants ---

func TestDockDropZoneValues(t *testing.T) {
	if dockDropNone != 0 {
		t.Fatal("DockDropNone should be 0")
	}
	// Verify all zones have distinct values
	zones := []DockDropZone{
		dockDropNone, dockDropCenter, dockDropTop,
		dockDropBottom, dockDropLeft, dockDropRight,
		dockDropWindowTop, dockDropWindowBottom,
		dockDropWindowLeft, dockDropWindowRight,
	}
	seen := make(map[DockDropZone]bool)
	for _, z := range zones {
		if seen[z] {
			t.Fatalf("duplicate zone value: %d", z)
		}
		seen[z] = true
	}
}
