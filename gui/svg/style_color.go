package svg

import (
	"math"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// parseSvgColor converts SVG color strings to SvgColor. ok=false
// lets the cascade drop unparseable declarations per CSS
// "invalid → ignore" rather than clobbering inherited paint.
// Empty input returns colorInherit so callers can distinguish
// "no value" from a recognized keyword. Case-insensitive.
func parseSvgColor(s string) (gui.SvgColor, bool) {
	str := strings.TrimSpace(s)
	if len(str) == 0 {
		return colorInherit, false
	}
	if str[0] == '#' {
		return parseHexColor(str)
	}
	if hasASCIIPrefixFold(str, "url(") {
		return colorTransparent, true
	}
	if hasASCIIPrefixFold(str, "rgb") {
		return parseRGBColor(str)
	}
	if strings.EqualFold(str, "none") {
		return colorTransparent, true
	}
	if strings.EqualFold(str, "currentColor") || strings.EqualFold(str, "inherit") {
		return colorCurrent, true
	}
	if c, ok := stringColors[strings.ToLower(str)]; ok {
		return c, true
	}
	return gui.SvgColor{}, false
}

// hasASCIIPrefixFold reports whether s starts with prefix using
// case-insensitive ASCII comparison without lowercasing the entire
// string. prefix must already be lowercase ASCII.
func hasASCIIPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

// parseHexColor parses #RGB, #RRGGBB, #RGBA, #RRGGBBAA. Returns
// ok=false on wrong length or non-hex digit so the cascade ignores
// the declaration per CSS "invalid → ignore".
func parseHexColor(s string) (gui.SvgColor, bool) {
	hex := s[1:]
	for i := 0; i < len(hex); i++ {
		if !isHexDigit(hex[i]) {
			return colorBlack, false
		}
	}
	switch len(hex) {
	case 3:
		r := hexDigit(hex[0]) * 17
		g := hexDigit(hex[1]) * 17
		b := hexDigit(hex[2]) * 17
		return gui.SvgColor{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, true
	case 4:
		r := hexDigit(hex[0]) * 17
		g := hexDigit(hex[1]) * 17
		b := hexDigit(hex[2]) * 17
		a := hexDigit(hex[3]) * 17
		return gui.SvgColor{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, true
	case 6:
		r := hexDigit(hex[0])*16 + hexDigit(hex[1])
		g := hexDigit(hex[2])*16 + hexDigit(hex[3])
		b := hexDigit(hex[4])*16 + hexDigit(hex[5])
		return gui.SvgColor{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, true
	case 8:
		r := hexDigit(hex[0])*16 + hexDigit(hex[1])
		g := hexDigit(hex[2])*16 + hexDigit(hex[3])
		b := hexDigit(hex[4])*16 + hexDigit(hex[5])
		a := hexDigit(hex[6])*16 + hexDigit(hex[7])
		return gui.SvgColor{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, true
	}
	return colorBlack, false
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f') ||
		(c >= 'A' && c <= 'F')
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return 0
}

// parseRGBColor parses rgb(r,g,b) or rgba(r,g,b,a). Slash-alpha and
// percent channel forms are not yet supported and return false.
func parseRGBColor(s string) (gui.SvgColor, bool) {
	start := strings.IndexByte(s, '(')
	end := strings.IndexByte(s, ')')
	if start < 0 || end < 0 || end <= start+1 {
		return colorBlack, false
	}
	body := s[start+1 : end]
	rv, next, ok := nextCommaValue(body, 0)
	if !ok {
		return colorBlack, false
	}
	gv, next, ok := nextCommaValue(body, next)
	if !ok {
		return colorBlack, false
	}
	bv, next, ok := nextCommaValue(body, next)
	if !ok {
		return colorBlack, false
	}
	rn, ok := parseIntStrict(rv)
	if !ok {
		return colorBlack, false
	}
	gn, ok := parseIntStrict(gv)
	if !ok {
		return colorBlack, false
	}
	bn, ok := parseIntStrict(bv)
	if !ok {
		return colorBlack, false
	}
	r := clampByte(rn)
	g := clampByte(gn)
	b := clampByte(bn)
	a := 255
	if av, _, ok := nextCommaValue(body, next); ok {
		alpha, aok := parseFloatStrict(av)
		if !aok {
			return colorBlack, false
		}
		if alpha <= 1.0 {
			a = clampByte(int(alpha * 255))
		} else {
			a = clampByte(int(alpha))
		}
	}
	return gui.SvgColor{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, true
}

// parseIntStrict parses a base-10 integer, rejecting empty or
// non-numeric input rather than silently returning 0.
func parseIntStrict(s string) (int, bool) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, false
	}
	v, err := strconv.Atoi(t)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseFloatStrict parses a float32, rejecting empty, non-numeric,
// NaN, and Inf. Non-finite values poison downstream byte clamping.
func parseFloatStrict(s string) (float32, bool) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(t, 32)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return float32(v), true
}

// parseFloatTrimmed parses s as a float32. NaN and ±Inf collapse to 0
// so non-finite tokens (e.g. "NaN%", "1e500s") cannot poison downstream
// arithmetic — uint8/uint16 casts of NaN are implementation-defined,
// and Inf coords break tessellation, animation timing, and bbox math.
func parseFloatTrimmed(s string) float32 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return float32(v)
}

func clampByte(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func nextCommaValue(s string, start int) (string, int, bool) {
	i := start
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' ||
		s[i] == '\n' || s[i] == '\r' || s[i] == ',') {
		i++
	}
	if i >= len(s) {
		return "", len(s), false
	}
	j := i
	for j < len(s) && s[j] != ',' {
		j++
	}
	return strings.TrimSpace(s[i:j]), j + 1, true
}

// clampOpacity01 maps NaN, ±Inf, and out-of-range values to a
// safe [0,1] range. Guards applyOpacity's uint8 cast, whose
// result is implementation-defined for NaN / negative inputs.
func clampOpacity01(v float32) float32 {
	if v != v {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
