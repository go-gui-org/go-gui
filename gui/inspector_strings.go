package gui

import "strings"

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
	if events.OnMouseDown != nil {
		names = append(names, "mouse_down")
	}
	if events.OnMouseUp != nil {
		names = append(names, "mouse_up")
	}
	if events.OnMouseScroll != nil {
		names = append(names, "scroll")
	}
	if events.onScroll != nil {
		names = append(names, "scroll_cb")
	}
	if events.OnHover != nil {
		names = append(names, "hover")
	}
	if events.onIMECommit != nil {
		names = append(names, "ime")
	}
	if events.AmendLayout != nil {
		names = append(names, "amend")
	}
	return strings.Join(names, ", ")
}

func inspectorSizingString(s Sizing) string {
	return inspectorSizingTypeString(s.Width) + ", " +
		inspectorSizingTypeString(s.Height)
}

func inspectorSizingTypeString(s sizingType) string {
	switch s {
	case sizingFill:
		return "fill"
	case sizingFixed:
		return "fixed"
	default:
		return "fit"
	}
}

func inspectorAlignString(h HorizontalAlign, v verticalAlign) string {
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

func inspectorVAlignString(v verticalAlign) string {
	switch v {
	case VAlignMiddle:
		return "middle"
	case VAlignBottom:
		return "bottom"
	default:
		return "top"
	}
}
