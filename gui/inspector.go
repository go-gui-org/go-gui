package gui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	capInspector            = 8
	inspectorScrollPanel    = "gui:inspector:panel"
	inspectorTreeID         = "__inspector_tree__"
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

func inspectorNodeTextStyle() TextStyle {
	return guiTheme.TreeStyle.TextStyle
}

func inspectorNodeIconStyle() TextStyle {
	return guiTheme.TreeStyle.TextStyleIcon
}

type inspectorStackFrame struct {
	nodes []TreeNodeCfg
	pos   int
}

type inspectorNodeProps struct {
	TypeName    string
	ID          string
	events      string
	TextPreview string
	Children    int
	Padding     Padding
	X           float32
	Y           float32
	Width       float32
	Height      float32
	Spacing     float32
	Radius      float32
	Focusable   bool
	Scrollable  bool
	Opacity     float32
	Color       Color
	Sizing      Sizing
	HAlign      HorizontalAlign
	VAlign      VerticalAlign
	IsFloat     bool
	Clip        bool
}

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
		ColorThumb: guiTheme.ScrollbarStyle.ColorThumb,
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
		Color:         guiTheme.InspectorStyle.ColorPanel,
		Radius:        SomeF(8),
		Clip:          true,
		ID:            inspectorScrollPanel,
		Scrollable:    true,
		ScrollbarCfgX: scrollbarCfg,
		ScrollbarCfgY: scrollbarCfg,
		Padding:       SomeP(0, scrollbarPad, 0, 0),
		Spacing:       SomeF(0),
		OnClick: func(_ *Layout, e *Event, _ *Window) {
			e.IsHandled = true
		},
		Content: []View{
			inspectorHelpBar(),
			inspectorTreeView(w),
		},
	})
}

func inspectorFloatAttach(left bool) FloatAttach {
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
			Size:  guiTheme.SizeTextXSmall,
			Color: guiTheme.InspectorStyle.ColorTextHelp,
		},
	})
}

func inspectorTreeView(w *Window) View {
	var nodes []TreeNodeCfg
	if w != nil {
		nodes = w.inspectorTreeCache
	}
	return Tree(TreeCfg{
		ID:       inspectorTreeID,
		Indent:   16,
		Spacing:  1,
		Nodes:    nodes,
		OnSelect: func(id string, _ *Event, w *Window) { inspectorSelect(id, w) },
	})
}

func inspectorSelect(path string, w *Window) {
	if w == nil {
		return
	}
	if strings.HasPrefix(path, inspectorPropPrefix) {
		if selected := inspectorSelectedPath(w); selected != "" {
			treeFocusedSet(w, inspectorTreeID, selected)
		}
		return
	}
	sm := StateMap[string, string](w, nsInspector, capInspector)
	// Default "": absent key means nothing is currently selected.
	selected := sm.GetOr("selected", "")
	if selected == path {
		sm.Delete("selected")
		sm.Delete("scroll_to")
		treeFocusedSet(w, inspectorTreeID, "")
		w.UpdateWindow()
		return
	}

	sm.Set("selected", path)
	sm.Set("scroll_to", path)
	treeFocusedSet(w, inspectorTreeID, path)

	expanded := StateReadOr[string, map[string]bool](
		w, nsTreeExpanded, inspectorTreeID, nil)
	if expanded == nil {
		expanded = make(map[string]bool)
	}
	expanded[path] = true
	parts := strings.Split(path, ".")
	if len(parts) > 0 {
		prefix := parts[0]
		expanded[prefix] = true
		for i := 1; i < len(parts); i++ {
			prefix += "." + parts[i]
			expanded[prefix] = true
		}
	}
	StateMap[string, map[string]bool](w, nsTreeExpanded, capModerate).
		Set(inspectorTreeID, expanded)
	w.UpdateWindow()
}

func inspectorPickPath(layout *Layout, x, y float32) string {
	if layout == nil || len(layout.Children) == 0 {
		return ""
	}
	return inspectorPickRecurse(&layout.Children[0], "0", x, y)
}

func inspectorPickRecurse(layout *Layout, path string, x, y float32) string {
	if layout == nil || layout.Shape == nil {
		return ""
	}
	if !layout.Shape.PointInShape(x, y) {
		return ""
	}
	for i := range slices.Backward(layout.Children) {
		childPath := path + "." + strconv.Itoa(i)
		if picked := inspectorPickRecurse(
			&layout.Children[i], childPath, x, y); picked != "" {
			return picked
		}
	}
	return path
}

func inspectorSelectedPath(w *Window) string {
	if w == nil {
		return ""
	}
	return StateReadOr(w, nsInspector, "selected", "")
}

func inspectorBuildTreeNodes(
	w *Window,
	layout *Layout,
	selected string,
	props map[string]inspectorNodeProps,
) []TreeNodeCfg {
	if layout == nil || len(layout.Children) == 0 {
		return nil
	}
	var expanded map[string]bool
	if w != nil {
		expanded = treeExpandedState(w, inspectorTreeID)
	}
	return inspectorLayoutToTree(
		w, expanded, &layout.Children[0], "0", selected, props)
}

func inspectorLayoutToTree(
	w *Window,
	expanded map[string]bool,
	layout *Layout,
	path string,
	selected string,
	props map[string]inspectorNodeProps,
) []TreeNodeCfg {
	if layout == nil {
		return nil
	}
	propSnapshot := inspectorSnapshotProps(layout)
	if props != nil {
		props[path] = propSnapshot
	}

	childNodes := make([]TreeNodeCfg, 0, 16)
	if path == selected {
		childNodes = append(childNodes, inspectorPropsNodes(propSnapshot)...)
	}

	isExpanded := expanded != nil && expanded[path]
	isAncestor := !isExpanded && selected != "" && strings.HasPrefix(selected, path+".")

	if isExpanded || isAncestor {
		for i := range layout.Children {
			childPath := path + "." + strconv.Itoa(i)
			childNodes = append(
				childNodes,
				inspectorLayoutToTree(
					w, expanded, &layout.Children[i], childPath, selected, props,
				)...,
			)
		}
	} else if len(layout.Children) > 0 {
		// Add a dummy child to show the arrow icon for collapsed nodes.
		childNodes = append(childNodes, TreeNodeCfg{
			ID: path + ".__dummy__",
		})
	}

	return []TreeNodeCfg{{
		ID:            path,
		Text:          inspectorNodeLabel(layout.Shape),
		TextStyle:     inspectorNodeTextStyle(),
		TextStyleIcon: inspectorNodeIconStyle(),
		Nodes:         childNodes,
	}}
}

func inspectorPropNode(
	id, text string, ps, pis TextStyle,
) TreeNodeCfg {
	return TreeNodeCfg{
		ID: id, Text: text,
		TextStyle: ps, TextStyleIcon: pis,
	}
}

func inspectorPropsNodes(p inspectorNodeProps) []TreeNodeCfg {
	propColor := guiTheme.InspectorStyle.ColorTextProp
	ps := TextStyle{
		Size:  guiTheme.SizeTextXSmall,
		Color: propColor,
	}
	pis := TextStyle{
		Family: IconFontName,
		Size:   guiTheme.SizeTextXSmall,
		Color:  propColor,
	}

	nodes := make([]TreeNodeCfg, 0, 16)
	if p.TextPreview != "" {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropTextID,
				`text: "`+p.TextPreview+`"`, ps, pis))
	}
	if p.ID != "" {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropIDID,
				"id: "+p.ID, ps, pis))
	}
	nodes = append(nodes,
		inspectorPropNode(inspectorPropPosID,
			"pos: "+strconv.Itoa(int(p.X))+", "+
				strconv.Itoa(int(p.Y)), ps, pis),
		inspectorPropNode(inspectorPropSizeID,
			"size: "+strconv.Itoa(int(p.Width))+" x "+
				strconv.Itoa(int(p.Height)), ps, pis),
	)
	if p.Sizing.Width != SizingFit || p.Sizing.Height != SizingFit {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropSizingID,
				"sizing: "+inspectorSizingString(p.Sizing), ps, pis))
	}
	if !p.Padding.IsNone() {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropPaddingID,
				"pad: "+strconv.Itoa(int(p.Padding.Top))+" "+
					strconv.Itoa(int(p.Padding.Right))+" "+
					strconv.Itoa(int(p.Padding.Bottom))+" "+
					strconv.Itoa(int(p.Padding.Left)), ps, pis))
	}
	if p.Spacing > 0 {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropSpacingID,
				"spacing: "+strconv.Itoa(int(p.Spacing)), ps, pis))
	}
	if p.Color.IsSet() && p.Color.A > 0 {
		nodes = append(nodes, TreeNodeCfg{
			ID:        inspectorPropColorID,
			Text:      "color: " + inspectorColorString(p.Color),
			Icon:      "\u25A0",
			TextStyle: ps,
			TextStyleIcon: TextStyle{
				Size:  guiTheme.SizeTextXSmall,
				Color: p.Color,
			},
		})
	}
	if p.Radius > 0 {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropRadiusID,
				"radius: "+strconv.Itoa(int(p.Radius)), ps, pis))
	}
	if p.Focusable {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropFocusID,
				"focusable: true", ps, pis))
	}
	if p.Scrollable {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropScrollID,
				"scrollable: true", ps, pis))
	}
	if p.HAlign != HAlignStart || p.VAlign != VAlignTop {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropAlignID,
				"align: "+inspectorAlignString(p.HAlign, p.VAlign),
				ps, pis))
	}
	if p.IsFloat {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropFloatID,
				"float: true", ps, pis))
	}
	if p.Clip {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropClipID,
				"clip: true", ps, pis))
	}
	if p.Opacity < 1 {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropOpacityID,
				fmt.Sprintf("opacity: %.2f", p.Opacity), ps, pis))
	}
	if p.events != "" {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropEventsID,
				"events: "+p.events, ps, pis))
	}
	if p.Children > 0 {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropChildrenID,
				"children: "+strconv.Itoa(p.Children), ps, pis))
	}
	return nodes
}

func inspectorSnapshotProps(layout *Layout) inspectorNodeProps {
	if layout == nil || layout.Shape == nil {
		return inspectorNodeProps{}
	}
	shape := layout.Shape
	textPreview := ""
	if shape.TC != nil && shape.TC.Text != "" {
		textPreview = truncatePreview(shape.TC.Text, 30)
	}

	props := inspectorNodeProps{
		TypeName:    inspectorTypeName(shape),
		ID:          shape.ID,
		X:           shape.X,
		Y:           shape.Y,
		Width:       shape.Width,
		Height:      shape.Height,
		Sizing:      shape.Sizing,
		Padding:     shape.Padding,
		Spacing:     shape.Spacing,
		Color:       shape.Color,
		Radius:      shape.Radius,
		Focusable:   shape.Focusable,
		Scrollable:  shape.Scrollable,
		HAlign:      shape.HAlign,
		VAlign:      shape.VAlign,
		IsFloat:     shape.Float,
		Clip:        shape.Clip,
		Opacity:     shape.Opacity,
		TextPreview: textPreview,
		Children:    len(layout.Children),
	}
	if shape.hasEvents() {
		props.events = inspectorEventsString(shape.events)
	}
	return props
}

func inspectorNodeLabel(shape *Shape) string {
	if shape == nil {
		return "(nil)"
	}
	label := inspectorTypeName(shape) + " " +
		strconv.Itoa(int(shape.Width)) + "x" +
		strconv.Itoa(int(shape.Height))
	if shape.ID != "" {
		label += " #" + shape.ID
	}
	return label
}

func inspectorTypeName(shape *Shape) string {
	if shape == nil {
		return "(nil)"
	}
	switch shape.shapeType {
	case shapeText:
		return "text"
	case shapeImage:
		return "image"
	case shapeCircle:
		return "circle"
	case shapeRTF:
		return "rtf"
	case shapeSVG:
		return "svg"
	case shapeDrawCanvas:
		return "draw_canvas"
	case shapeTermGrid:
		return "termgrid"
	case shapeNone, shapeRectangle:
		switch shape.Axis {
		case AxisTopToBottom:
			return "column"
		case AxisLeftToRight:
			return "row"
		default:
			return "canvas"
		}
	default:
		return "unknown"
	}
}

func inspectorFindByPath(layout *Layout, path string) (*Layout, bool) {
	if layout == nil || path == "" {
		return nil, false
	}
	node := layout
	for part := range strings.SplitSeq(path, ".") {
		idx, err := strconv.Atoi(part)
		if err != nil || idx < 0 || idx >= len(node.Children) {
			return nil, false
		}
		node = &node.Children[idx]
	}
	return node, true
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
	emitRenderer(RenderCmd{
		Kind:      RenderStrokeRect,
		X:         shape.X,
		Y:         shape.Y,
		W:         shape.Width,
		H:         shape.Height,
		Radius:    shape.Radius,
		Color:     guiTheme.InspectorStyle.ColorWireframe,
		Thickness: 2,
	}, w)

	if shape.Padding.IsNone() {
		return
	}
	emitRenderer(RenderCmd{
		Kind:      RenderStrokeRect,
		X:         shape.X + shape.Padding.Left,
		Y:         shape.Y + shape.Padding.Top,
		W:         f32Max(0, shape.Width-shape.Padding.Left-shape.Padding.Right),
		H:         f32Max(0, shape.Height-shape.Padding.Top-shape.Padding.Bottom),
		Color:     guiTheme.InspectorStyle.ColorPadding,
		Thickness: 1,
	}, w)
}

func inspectorEventsString(events *eventHandlers) string {
	if events == nil {
		return ""
	}
	names := make([]string, 0, 10)
	if events.OnClick != nil {
		names = append(names, "click")
	}
	if events.OnChar != nil {
		names = append(names, "char")
	}
	if events.OnKeyDown != nil {
		names = append(names, "keydown")
	}
	if events.OnMouseMove != nil {
		names = append(names, "mouse_move")
	}
	if events.OnMouseUp != nil {
		names = append(names, "mouse_up")
	}
	if events.OnMouseScroll != nil {
		names = append(names, "scroll")
	}
	if events.OnScroll != nil {
		names = append(names, "scroll_cb")
	}
	if events.OnHover != nil {
		names = append(names, "hover")
	}
	if events.OnIMECommit != nil {
		names = append(names, "ime")
	}
	if events.AmendLayout != nil {
		names = append(names, "amend")
	}
	return strings.Join(names, ", ")
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
		Spacing: 1,
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

func inspectorSizingString(s Sizing) string {
	return inspectorSizingTypeString(s.Width) + ", " +
		inspectorSizingTypeString(s.Height)
}

func inspectorSizingTypeString(s SizingType) string {
	switch s {
	case SizingFill:
		return "fill"
	case SizingFixed:
		return "fixed"
	default:
		return "fit"
	}
}

func inspectorAlignString(h HorizontalAlign, v VerticalAlign) string {
	return inspectorHAlignString(h) + ", " + inspectorVAlignString(v)
}

func inspectorHAlignString(h HorizontalAlign) string {
	switch h {
	case HAlignEnd:
		return "end"
	case HAlignCenter:
		return "center"
	case HAlignLeft:
		return "left"
	case HAlignRight:
		return "right"
	default:
		return "start"
	}
}

func inspectorVAlignString(v VerticalAlign) string {
	switch v {
	case VAlignMiddle:
		return "middle"
	case VAlignBottom:
		return "bottom"
	default:
		return "top"
	}
}
