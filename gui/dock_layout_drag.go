package gui

// dock_layout_drag.go — drag lifecycle, zone detection, ghost
// rendering, and overlay drawing for docking panel drag operations.

const (
	dockDragThreshold      = float32(5.0)
	dockDragWindowEdgeZone = float32(20.0)
	dockDragEdgeRatio      = float32(0.25)
)

// DockDropZone identifies where a panel will be inserted on drop.
type DockDropZone uint8

// DockDropZone constants.
const (
	DockDropNone   DockDropZone = iota
	DockDropCenter              // add as tab
	DockDropTop                 // split above
	DockDropBottom              // split below
	DockDropLeft                // split left
	DockDropRight               // split right
	DockDropWindowTop
	DockDropWindowBottom
	DockDropWindowLeft
	DockDropWindowRight
)

// dockDragState tracks an in-progress dock panel drag.
type dockDragState struct {
	panelID      string
	sourceGroup  string
	hoverGroupID string
	panelNodes   []*DockNode // cached at drag activation
	mouseX       float32
	mouseY       float32
	startMouseX  float32
	startMouseY  float32
	ghostW       float32
	ghostH       float32
	parentX      float32
	parentY      float32
	active       bool
	hoverZone    DockDropZone
}

// dockDragGet retrieves the current drag state.
func dockDragGet(w *Window, dockID string) dockDragState {
	sm := StateMap[string, dockDragState](w, nsDockDrag, capFew)
	v, ok := sm.Get(dockID)
	if !ok {
		return dockDragState{}
	}
	return v
}

// dockDragSet stores drag state.
func dockDragSet(w *Window, dockID string, state dockDragState) {
	sm := StateMap[string, dockDragState](w, nsDockDrag, capFew)
	sm.Set(dockID, state)
}

// dockDragClear removes drag state.
func dockDragClear(w *Window, dockID string) {
	sm := StateMap[string, dockDragState](w, nsDockDrag, capFew)
	sm.Delete(dockID)
}

// dockDragStart initiates a dock panel drag from a tab header
// click.
func dockDragStart(
	dockID, panelID, sourceGroup string,
	root *DockNode,
	onLayoutChange func(*DockNode, *Window),
	layout *Layout, e *Event, w *Window,
) {
	// Ghost base offset: tab position relative to dock container.
	ghostBaseX := layout.Shape.X
	ghostBaseY := layout.Shape.Y
	if w.layout.Shape != nil {
		if dockLayout, ok := w.layout.FindByID(dockID); ok {
			ghostBaseX -= dockLayout.Shape.X
			ghostBaseY -= dockLayout.Shape.Y
		}
	}
	absMouseX := layout.Shape.X + e.MouseX
	absMouseY := layout.Shape.Y + e.MouseY
	state := dockDragState{
		active:      false,
		panelID:     panelID,
		sourceGroup: sourceGroup,
		mouseX:      absMouseX,
		mouseY:      absMouseY,
		startMouseX: absMouseX,
		startMouseY: absMouseY,
		ghostW:      layout.Shape.Width,
		ghostH:      layout.Shape.Height,
		parentX:     ghostBaseX,
		parentY:     ghostBaseY,
	}
	dockDragSet(w, dockID, state)
	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			dockDragOnMouseMove(dockID, root, e.MouseX, e.MouseY, w)
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			dockDragOnMouseUp(dockID, root, onLayoutChange, w)
		},
	})
}

// dockDragOnMouseMove handles threshold detection and zone
// tracking.
func dockDragOnMouseMove(
	dockID string, root *DockNode,
	mouseX, mouseY float32, w *Window,
) {
	state := dockDragGet(w, dockID)
	state.mouseX = mouseX
	state.mouseY = mouseY

	if !state.active {
		dx := mouseX - state.startMouseX
		dy := mouseY - state.startMouseY
		dist := max(f32Abs(dx), f32Abs(dy))
		if dist < dockDragThreshold {
			dockDragSet(w, dockID, state)
			return
		}
		state.active = true
		state.panelNodes = DockTreeCollectPanelNodes(root)
	}

	zone, groupID := dockDragDetectZone(
		dockID, state.panelNodes, mouseX, mouseY,
		state.sourceGroup, w)
	state.hoverZone = zone
	state.hoverGroupID = groupID
	dockDragSet(w, dockID, state)
	w.UpdateWindow()
}

// dockDragOnMouseUp handles the drop or cancel.
func dockDragOnMouseUp(
	dockID string, root *DockNode,
	onLayoutChange func(*DockNode, *Window), w *Window,
) {
	state := dockDragGet(w, dockID)
	w.MouseUnlock()

	if state.active && state.hoverZone != DockDropNone && onLayoutChange != nil {
		newRoot := DockTreeMovePanel(
			root, state.panelID, state.hoverGroupID,
			state.hoverZone)
		onLayoutChange(newRoot, w)
	}

	dockDragClear(w, dockID)
	w.UpdateWindow()
}

// dockDragCancel cancels the drag in progress.
func dockDragCancel(dockID string, w *Window) {
	w.MouseUnlock()
	dockDragClear(w, dockID)
	w.UpdateWindow()
}

// dockDragDetectZone determines which drop zone the cursor is
// over. panelNodes is pre-collected at drag activation to avoid
// per-move allocations.
//
//nolint:gocyclo // zone hit-testing
func dockDragDetectZone(
	dockID string, panelNodes []*DockNode,
	mouseX, mouseY float32,
	sourceGroup string,
	w *Window,
) (DockDropZone, string) {
	dockLayout, ok := w.layout.FindByID(dockID)
	if !ok {
		return DockDropNone, ""
	}
	clip := dockLayout.Shape.ShapeClip
	if clip.Width <= 0 || clip.Height <= 0 {
		return DockDropNone, ""
	}

	// Check window-edge zones first.
	edge := dockDragWindowEdgeZone
	if mouseX >= clip.X && mouseX < clip.X+edge &&
		mouseY >= clip.Y && mouseY < clip.Y+clip.Height {
		return DockDropWindowLeft, ""
	}
	if mouseX >= clip.X+clip.Width-edge && mouseX < clip.X+clip.Width &&
		mouseY >= clip.Y && mouseY < clip.Y+clip.Height {
		return DockDropWindowRight, ""
	}
	if mouseY >= clip.Y && mouseY < clip.Y+edge &&
		mouseX >= clip.X && mouseX < clip.X+clip.Width {
		return DockDropWindowTop, ""
	}
	if mouseY >= clip.Y+clip.Height-edge && mouseY < clip.Y+clip.Height &&
		mouseX >= clip.X && mouseX < clip.X+clip.Width {
		return DockDropWindowBottom, ""
	}

	// Check each panel group's zone.
	for _, group := range panelNodes {
		groupLayout, ok := w.layout.FindByID(group.ID)
		if !ok {
			continue
		}
		gc := groupLayout.Shape.ShapeClip
		if gc.Width <= 0 || gc.Height <= 0 {
			continue
		}
		if mouseX < gc.X || mouseX >= gc.X+gc.Width ||
			mouseY < gc.Y || mouseY >= gc.Y+gc.Height {
			continue
		}
		// Skip dropping onto source group if it only has one panel.
		if group.ID == sourceGroup && len(group.PanelIDs) <= 1 {
			continue
		}
		relX := (mouseX - gc.X) / gc.Width
		relY := (mouseY - gc.Y) / gc.Height
		zone := dockClassifyZone(relX, relY)
		// Skip center drop on same group (already a tab there).
		if zone == DockDropCenter && group.ID == sourceGroup {
			continue
		}
		return zone, group.ID
	}

	return DockDropNone, ""
}

// dockClassifyZone classifies a relative position (0..1) within
// a group rectangle into a drop zone.
func dockClassifyZone(relX, relY float32) DockDropZone {
	edge := dockDragEdgeRatio
	if relY < edge {
		return DockDropTop
	}
	if relY > 1.0-edge {
		return DockDropBottom
	}
	if relX < edge {
		return DockDropLeft
	}
	if relX > 1.0-edge {
		return DockDropRight
	}
	return DockDropCenter
}

// dockDragGhostView returns a floating ghost of the dragged tab.
func dockDragGhostView(state dockDragState, label string) View {
	return Column(ContainerCfg{
		Float:        true,
		FloatOffsetX: state.parentX + (state.mouseX - state.startMouseX),
		FloatOffsetY: state.parentY + (state.mouseY - state.startMouseY),
		Width:        state.ghostW,
		Height:       state.ghostH,
		Opacity:      SomeF(dragGhostOpacity),
		Sizing:       FixedFixed,
		Clip:         true,
		Padding:      SomeP(2, 6, 2, 6),
		Color:        guiTheme.ColorPanel,
		Shadow: &BoxShadow{
			Color:      dragGhostShadowColor,
			OffsetY:    dragGhostShadowOffY,
			BlurRadius: dragGhostShadowBlur,
		},
		Content: []View{Text(TextCfg{Text: label})},
	})
}

// dockDragZoneOverlayView returns a semi-transparent overlay
// showing the drop zone preview. Positioned via AmendLayout.
func dockDragZoneOverlayView(colorZone Color) View {
	return Column(ContainerCfg{
		ID:      "dock_zone_overlay",
		Sizing:  FixedFixed,
		Width:   0,
		Height:  0,
		Padding: NoPadding,
		Color:   colorZone,
	})
}

// dockDragAmendOverlay positions the zone overlay based on the
// current drag state and target group layout.
func dockDragAmendOverlay(
	dockID string, colorZone Color,
	layout *Layout, w *Window,
) {
	state := dockDragGet(w, dockID)
	if !state.active || state.hoverZone == DockDropNone {
		return
	}

	// Find the overlay child by id.
	overlayIdx := -1
	for i := range layout.Children {
		if layout.Children[i].Shape.ID == "dock_zone_overlay" {
			overlayIdx = i
			break
		}
	}
	if overlayIdx < 0 {
		return
	}

	// Determine target rect.
	var tx, ty, tw, th float32

	switch {
	case state.hoverZone == DockDropWindowTop ||
		state.hoverZone == DockDropWindowBottom ||
		state.hoverZone == DockDropWindowLeft ||
		state.hoverZone == DockDropWindowRight:
		tx = layout.Shape.X
		ty = layout.Shape.Y
		tw = layout.Shape.Width
		th = layout.Shape.Height
	case len(state.hoverGroupID) > 0:
		groupLayout, ok := layout.FindByID(state.hoverGroupID)
		if !ok {
			return
		}
		tx = groupLayout.Shape.X
		ty = groupLayout.Shape.Y
		tw = groupLayout.Shape.Width
		th = groupLayout.Shape.Height
	default:
		return
	}

	// Subdivide based on zone.
	switch state.hoverZone {
	case DockDropTop, DockDropWindowTop:
		th *= 0.5
	case DockDropBottom, DockDropWindowBottom:
		ty += th * 0.5
		th *= 0.5
	case DockDropLeft, DockDropWindowLeft:
		tw *= 0.5
	case DockDropRight, DockDropWindowRight:
		tx += tw * 0.5
		tw *= 0.5
	case DockDropCenter, DockDropNone:
		// full rect
	}

	layout.Children[overlayIdx].Shape.X = tx
	layout.Children[overlayIdx].Shape.Y = ty
	layout.Children[overlayIdx].Shape.Width = tw
	layout.Children[overlayIdx].Shape.Height = th
	layout.Children[overlayIdx].Shape.Color = colorZone
}
