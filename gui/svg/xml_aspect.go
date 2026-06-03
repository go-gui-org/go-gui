package svg

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// parsePreserveAspectRatio decodes the SVG preserveAspectRatio
// attribute. Format: <align> [meet|slice]; align is one of "none" or
// "xMin|xMid|xMax" + "YMin|YMid|YMax". Empty / malformed input
// returns the spec default (xMidYMid meet).
func parsePreserveAspectRatio(s string) (gui.SvgAlign, bool) {
	// Cap input length — well-formed values are <=20 chars
	// ("xMidYMid slice"). A pathological attribute would otherwise
	// pump strings.Fields with a huge allocation.
	const maxAspectAttrLen = 64
	if len(s) > maxAspectAttrLen {
		s = s[:maxAspectAttrLen]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return gui.SvgAlignXMidYMid, false
	}
	tokens := strings.Fields(s)
	align := gui.SvgAlignXMidYMid
	slice := false
	for _, tok := range tokens {
		switch tok {
		case "none":
			align = gui.SvgAlignNone
		case "xMinYMin":
			align = gui.SvgAlignXMinYMin
		case "xMidYMin":
			align = gui.SvgAlignXMidYMin
		case "xMaxYMin":
			align = gui.SvgAlignXMaxYMin
		case "xMinYMid":
			align = gui.SvgAlignXMinYMid
		case "xMidYMid":
			align = gui.SvgAlignXMidYMid
		case "xMaxYMid":
			align = gui.SvgAlignXMaxYMid
		case "xMinYMax":
			align = gui.SvgAlignXMinYMax
		case "xMidYMax":
			align = gui.SvgAlignXMidYMax
		case "xMaxYMax":
			align = gui.SvgAlignXMaxYMax
		case "meet":
			slice = false
		case "slice":
			slice = true
		}
	}
	return align, slice
}
