package svg

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// --- Stroke attribute extraction ---

func getTransform(elem string) [6]float32 {
	if t, ok := findAttrOrStyle(elem, "transform"); ok {
		return parseTransform(t)
	}
	return identityTransform
}

func getStrokeColor(elem string) gui.SvgColor {
	stroke, ok := findAttrOrStyle(elem, "stroke")
	if !ok {
		return colorInherit
	}
	c, parsed := parseSvgColor(stroke)
	if !parsed {
		return colorInherit
	}
	return c
}

func getStrokeGradientID(elem string) string {
	stroke, ok := findAttrOrStyle(elem, "stroke")
	if !ok {
		return ""
	}
	id, _ := parseFillURL(stroke)
	return id
}

func getStrokeWidth(elem string) float32 {
	ws, ok := findAttrOrStyle(elem, "stroke-width")
	if !ok {
		return -1.0
	}
	return parseLength(ws)
}

func getStrokeLinecap(elem string) gui.SvgStrokeCap {
	lineCap, ok := findAttrOrStyle(elem, "stroke-linecap")
	if !ok {
		return gui.SvgStrokeCap(3) // inherit sentinel
	}
	switch lineCap {
	case "round":
		return gui.SvgRoundCap
	case "square":
		return gui.SvgSquareCap
	default:
		return gui.SvgButtCap
	}
}

func getStrokeLinejoin(elem string) gui.SvgStrokeJoin {
	join, ok := findAttrOrStyle(elem, "stroke-linejoin")
	if !ok {
		return gui.SvgStrokeJoin(3) // inherit sentinel
	}
	switch join {
	case "round":
		return gui.SvgRoundJoin
	case "bevel":
		return gui.SvgBevelJoin
	default:
		return gui.SvgMiterJoin
	}
}

func getStrokeDasharray(elem string) []float32 {
	val, ok := findAttrOrStyle(elem, "stroke-dasharray")
	if !ok {
		return nil
	}
	if strings.TrimSpace(val) == "none" {
		return nil
	}
	result := make([]float32, 0, 4)
	var sum float32
	for i := 0; i < len(val); {
		start := i
		for start < len(val) && isFloatListSep(val[start]) {
			start++
		}
		if start >= len(val) {
			break
		}
		end := start
		for end < len(val) && !isFloatListSep(val[end]) {
			end++
		}
		n := parseFloatTrimmed(val[start:end])
		if n < 0 {
			return nil
		}
		result = append(result, n)
		sum += n
		i = end
	}
	// Zero-sum dasharray = solid line (SVG spec).
	if sum <= 0 {
		return nil
	}
	if len(result) > 0 && len(result)%2 != 0 {
		result = append(result, result...)
	}
	return result
}

func isFloatListSep(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' ||
		b == '\r' || b == ','
}

func parseStrokeCap(v string) gui.SvgStrokeCap {
	switch v {
	case "round":
		return gui.SvgRoundCap
	case "square":
		return gui.SvgSquareCap
	default:
		return gui.SvgButtCap
	}
}

func parseStrokeJoin(v string) gui.SvgStrokeJoin {
	switch v {
	case "round":
		return gui.SvgRoundJoin
	case "bevel":
		return gui.SvgBevelJoin
	default:
		return gui.SvgMiterJoin
	}
}

// sanitizeStrokeWidth maps NaN and negative widths to 0. SVG spec
// treats negative stroke-width as an error; NaN poisons tessellation.
func sanitizeStrokeWidth(v float32) float32 {
	if v != v || v < 0 {
		return 0
	}
	return v
}
