package gui

import (
	"fmt"
	"strconv"
)

func inspectorNodeTextStyle() TextStyle {
	return guiTheme.treeStyle.TextStyle
}

func inspectorNodeIconStyle() TextStyle {
	return guiTheme.treeStyle.textStyleIcon
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
	VAlign      verticalAlign
	IsFloat     bool
	Clip        bool
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
	propColor := guiTheme.inspectorStyle.colorTextProp
	ps := TextStyle{
		Size:  guiTheme.SizeTextXSmall,
		Color: propColor,
	}
	// Icon5 is the XSmall themed icon style, so it already carries the
	// theme's icon family (app-overridable via ThemeCfg.IconFontFamily).
	pis := guiTheme.Icon5
	pis.Color = propColor

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
	if p.Sizing.Width != sizingFit || p.Sizing.Height != sizingFit {
		nodes = append(nodes,
			inspectorPropNode(inspectorPropSizingID,
				"sizing: "+inspectorSizingString(p.Sizing), ps, pis))
	}
	if !p.Padding.isNone() {
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
		TypeName: inspectorTypeName(shape),
		// The effective ID, not the leaf: it is the string SetFocus,
		// FindByID and the test helpers take, so it is the one worth
		// copying out of the inspector.
		ID:          shape.idKey(),
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
	if id := shape.idKey(); id != "" {
		label += " #" + id
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
		case axisTopToBottom:
			return "column"
		case axisLeftToRight:
			return "row"
		default:
			return "canvas"
		}
	default:
		return "unknown"
	}
}
