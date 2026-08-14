package gui

import (
	"fmt"
)

const (
	capInspector         = 8
	inspectorScrollPanel = "gui:inspector:panel"
	// Absolute, and it has to be. The tree is a child of the ID-bearing
	// panel above, so a plain leaf would resolve to
	// "gui:inspector:panel:<leaf>" — while inspectorSelect and the
	// expansion helpers, which run from event handlers rather than from
	// generation, would keep writing the bare constant. The wireframe
	// would still follow the pick (it keys on nsInspector) and the tree
	// would never show the row. Containing IDSep makes this the identity
	// the Tree widget stores under, so both halves agree.
	inspectorTreeID         = "gui:inspector:tree"
	inspectorPanelMinWidth  = float32(300)
	inspectorResizeStep     = float32(50)
	inspectorMargin         = float32(10)
	inspectorPropPrefix     = "__prop_"
	inspectorPropTextID     = "__prop_text"
	inspectorPropIDID       = "__prop_id"
	inspectorPropPosID      = "__prop_pos"
	inspectorPropSizeID     = "__prop_size"
	inspectorPropSizingID   = "__prop_sizing"
	inspectorPropPaddingID  = "__prop_pad"
	inspectorPropSpacingID  = "__prop_spacing"
	inspectorPropColorID    = "__prop_color"
	inspectorPropRadiusID   = "__prop_radius"
	inspectorPropFocusID    = "__prop_focus"
	inspectorPropScrollID   = "__prop_scroll"
	inspectorPropAlignID    = "__prop_align"
	inspectorPropFloatID    = "__prop_float"
	inspectorPropClipID     = "__prop_clip"
	inspectorPropOpacityID  = "__prop_opacity"
	inspectorPropEventsID   = "__prop_events"
	inspectorPropChildrenID = "__prop_children"
)

func inspectorToggle(w *Window) {
	if w == nil {
		return
	}
	w.inspectorEnabled = !w.inspectorEnabled
	w.UpdateWindow()
}

func inspectorIsLeft(w *Window) bool {
	if w == nil {
		return false
	}
	side := StateReadOr(w, nsInspector, "side", "")
	return side == "left"
}

func inspectorToggleSide(w *Window) {
	if w == nil {
		return
	}
	sm := StateMap[string, string](w, nsInspector, capInspector)
	// Default "": absent key means inspector is on the right side.
	side := sm.GetOr("side", "")
	if side == "left" {
		sm.Delete("side")
	} else {
		sm.Set("side", "left")
	}
	w.UpdateWindow()
}

func inspectorPanelWidth(w *Window) float32 {
	if w == nil {
		return inspectorPanelMinWidth
	}
	width := StateReadOr(
		w, nsInspectorWidth, "width", inspectorPanelMinWidth)
	return f32Max(width, inspectorPanelMinWidth)
}

func inspectorResize(delta float32, w *Window) {
	if w == nil {
		return
	}
	maxWidth := float32(w.windowWidth) * 0.8
	maxWidth = max(maxWidth, inspectorPanelMinWidth)
	width := f32Clamp(
		inspectorPanelWidth(w)+delta,
		inspectorPanelMinWidth,
		maxWidth,
	)
	StateMap[string, float32](w, nsInspectorWidth, capInspector).
		Set("width", width)
	w.UpdateWindow()
}

func inspectorFloatingPanel(w *Window) View {
	if w == nil {
		return nil
	}
	panelHeight := f32Max(0, float32(w.windowHeight)-inspectorMargin*2)
	panelWidth := inspectorPanelWidth(w)
	inspectorApplyScrollTo(panelHeight, w)

	left := inspectorIsLeft(w)
	scrollbarPad := guiTheme.ScrollbarStyle.Size +
		guiTheme.ScrollbarStyle.GapEdge*2
	scrollbarCfg := &ScrollbarCfg{
		colorThumb: guiTheme.ScrollbarStyle.colorThumb,
	}

	return Column(ContainerCfg{
		Float:         true,
		FloatAnchor:   inspectorFloatAttach(left),
		FloatTieOff:   inspectorFloatAttach(left),
		FloatOffsetX:  inspectorFloatOffsetX(left),
		FloatOffsetY:  inspectorMargin,
		Sizing:        FixedFixed,
		Width:         panelWidth,
		Height:        panelHeight,
		Color:         guiTheme.inspectorStyle.ColorPanel,
		Radius:        SomeF(8),
		Clip:          true,
		ID:            inspectorScrollPanel,
		Scrollable:    true,
		ScrollbarCfgX: scrollbarCfg,
		ScrollbarCfgY: scrollbarCfg,
		Padding:       NewPadding(0, scrollbarPad, 0, 0),
		Spacing:       SomeF(0),
		// The inspector panel overlays the app being inspected; clicks
		// on it must not reach through and mutate what is under study.
		OnClick: func(ctx EventCtx) {
			ctx.Consume()
		},
		Content: []View{
			inspectorHelpBar(),
			inspectorTreeView(w),
		},
	})
}

func inspectorFloatAttach(left bool) floatAttach {
	if left {
		return FloatTopLeft
	}
	return FloatTopRight
}

func inspectorFloatOffsetX(left bool) float32 {
	if left {
		return inspectorMargin
	}
	return -inspectorMargin
}

func inspectorHelpBar() View {
	return Text(TextCfg{
		Text: "  F12 toggle  Alt+Left/Right resize  Alt+Up side",
		TextStyle: TextStyle{
			Size:  guiTheme.sizeTextXSmall,
			Color: guiTheme.inspectorStyle.colorTextHelp,
		},
	})
}

func inspectorInjectWireframe(w *Window) {
	if w == nil {
		return
	}
	selected := inspectorSelectedPath(w)
	if selected == "" {
		return
	}
	node, ok := inspectorFindByPath(&w.layout, selected)
	if !ok || node == nil || node.Shape == nil {
		return
	}
	shape := node.Shape
	// Runs from updateLayout after arrange, outside generation: name
	// the window rather than the installed frame cache.
	insp := w.Theme().inspectorStyle
	emitRenderer(RenderCmd{
		Kind:      RenderStrokeRect,
		X:         shape.X,
		Y:         shape.Y,
		W:         shape.Width,
		H:         shape.Height,
		Radius:    shape.Radius,
		Color:     insp.colorWireframe,
		Thickness: 2,
	}, w)

	if shape.Padding.isNone() {
		return
	}
	emitRenderer(RenderCmd{
		Kind:      RenderStrokeRect,
		X:         shape.X + shape.Padding.Left,
		Y:         shape.Y + shape.Padding.Top,
		W:         f32Max(0, shape.Width-shape.Padding.Left-shape.Padding.Right),
		H:         f32Max(0, shape.Height-shape.Padding.Top-shape.Padding.Bottom),
		Color:     insp.colorPadding,
		Thickness: 1,
	}, w)
}

func inspectorColorString(c Color) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

func inspectorApplyScrollTo(panelHeight float32, w *Window) {
	if w == nil || panelHeight <= 0 {
		return
	}
	sm := StateMap[string, string](w, nsInspector, capInspector)
	target, ok := sm.Get("scroll_to")
	if !ok || target == "" {
		return
	}
	sm.Delete("scroll_to")

	expanded := treeExpandedState(w, inspectorTreeID)
	rowIdx := inspectorFlatRowIndex(w.inspectorTreeCache, expanded, target)
	if rowIdx < 0 {
		return
	}
	rowHeight := treeEstimateRowHeight(TreeCfg{
		Nodes:   w.inspectorTreeCache,
		Spacing: SomeF(1),
	}, w)
	targetY := float32(rowIdx) * rowHeight
	newScroll := -(targetY - rowHeight*2)
	newScroll = min(newScroll, 0)
	w.scrollY().Set(inspectorScrollPanel, newScroll)
}

func inspectorFlatRowIndex(
	nodes []TreeNodeCfg,
	expanded map[string]bool,
	target string,
) int {
	stack := []inspectorStackFrame{{nodes: nodes}}
	idx := 0
	for len(stack) > 0 {
		last := len(stack) - 1
		if stack[last].pos >= len(stack[last].nodes) {
			stack = stack[:last]
			continue
		}
		node := stack[last].nodes[stack[last].pos]
		stack[last].pos++
		id := treeNodeID(node)
		if id == target {
			return idx
		}
		idx++
		if expanded[id] && len(node.Nodes) > 0 {
			stack = append(stack, inspectorStackFrame{
				nodes: node.Nodes,
			})
		}
	}
	return -1
}
