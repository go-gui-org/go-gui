package svg

import (
	"maps"
	"math"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// Named SVG colors lookup table.
var stringColors = map[string]gui.SvgColor{
	"blue":                 {R: 0, G: 0, B: 255, A: 255},
	"red":                  {R: 255, G: 0, B: 0, A: 255},
	"green":                {R: 0, G: 128, B: 0, A: 255},
	"yellow":               {R: 255, G: 255, B: 0, A: 255},
	"orange":               {R: 255, G: 165, B: 0, A: 255},
	"purple":               {R: 128, G: 0, B: 128, A: 255},
	"black":                {R: 0, G: 0, B: 0, A: 255},
	"gray":                 {R: 128, G: 128, B: 128, A: 255},
	"grey":                 {R: 128, G: 128, B: 128, A: 255},
	"indigo":               {R: 75, G: 0, B: 130, A: 255},
	"pink":                 {R: 255, G: 192, B: 203, A: 255},
	"violet":               {R: 238, G: 130, B: 238, A: 255},
	"white":                {R: 255, G: 255, B: 255, A: 255},
	"cyan":                 {R: 0, G: 255, B: 255, A: 255},
	"magenta":              {R: 255, G: 0, B: 255, A: 255},
	"aliceblue":            {R: 240, G: 248, B: 255, A: 255},
	"antiquewhite":         {R: 250, G: 235, B: 215, A: 255},
	"aqua":                 {R: 0, G: 255, B: 255, A: 255},
	"aquamarine":           {R: 127, G: 255, B: 212, A: 255},
	"azure":                {R: 240, G: 255, B: 255, A: 255},
	"beige":                {R: 245, G: 245, B: 220, A: 255},
	"bisque":               {R: 255, G: 228, B: 196, A: 255},
	"blanchedalmond":       {R: 255, G: 235, B: 205, A: 255},
	"blueviolet":           {R: 138, G: 43, B: 226, A: 255},
	"brown":                {R: 165, G: 42, B: 42, A: 255},
	"burlywood":            {R: 222, G: 184, B: 135, A: 255},
	"cadetblue":            {R: 95, G: 158, B: 160, A: 255},
	"chartreuse":           {R: 127, G: 255, B: 0, A: 255},
	"chocolate":            {R: 210, G: 105, B: 30, A: 255},
	"coral":                {R: 255, G: 127, B: 80, A: 255},
	"cornflowerblue":       {R: 100, G: 149, B: 237, A: 255},
	"cornsilk":             {R: 255, G: 248, B: 220, A: 255},
	"crimson":              {R: 220, G: 20, B: 60, A: 255},
	"darkblue":             {R: 0, G: 0, B: 139, A: 255},
	"darkcyan":             {R: 0, G: 139, B: 139, A: 255},
	"darkgoldenrod":        {R: 184, G: 134, B: 11, A: 255},
	"darkgray":             {R: 169, G: 169, B: 169, A: 255},
	"darkgreen":            {R: 0, G: 100, B: 0, A: 255},
	"darkgrey":             {R: 169, G: 169, B: 169, A: 255},
	"darkkhaki":            {R: 189, G: 183, B: 107, A: 255},
	"darkmagenta":          {R: 139, G: 0, B: 139, A: 255},
	"darkolivegreen":       {R: 85, G: 107, B: 47, A: 255},
	"darkorange":           {R: 255, G: 140, B: 0, A: 255},
	"darkorchid":           {R: 153, G: 50, B: 204, A: 255},
	"darkred":              {R: 139, G: 0, B: 0, A: 255},
	"darksalmon":           {R: 233, G: 150, B: 122, A: 255},
	"darkseagreen":         {R: 143, G: 188, B: 143, A: 255},
	"darkslateblue":        {R: 72, G: 61, B: 139, A: 255},
	"darkslategray":        {R: 47, G: 79, B: 79, A: 255},
	"darkslategrey":        {R: 47, G: 79, B: 79, A: 255},
	"darkturquoise":        {R: 0, G: 206, B: 209, A: 255},
	"darkviolet":           {R: 148, G: 0, B: 211, A: 255},
	"deeppink":             {R: 255, G: 20, B: 147, A: 255},
	"deepskyblue":          {R: 0, G: 191, B: 255, A: 255},
	"dimgray":              {R: 105, G: 105, B: 105, A: 255},
	"dimgrey":              {R: 105, G: 105, B: 105, A: 255},
	"dodgerblue":           {R: 30, G: 144, B: 255, A: 255},
	"firebrick":            {R: 178, G: 34, B: 34, A: 255},
	"floralwhite":          {R: 255, G: 250, B: 240, A: 255},
	"forestgreen":          {R: 34, G: 139, B: 34, A: 255},
	"fuchsia":              {R: 255, G: 0, B: 255, A: 255},
	"gainsboro":            {R: 220, G: 220, B: 220, A: 255},
	"ghostwhite":           {R: 248, G: 248, B: 255, A: 255},
	"gold":                 {R: 255, G: 215, B: 0, A: 255},
	"goldenrod":            {R: 218, G: 165, B: 32, A: 255},
	"greenyellow":          {R: 173, G: 255, B: 47, A: 255},
	"honeydew":             {R: 240, G: 255, B: 240, A: 255},
	"hotpink":              {R: 255, G: 105, B: 180, A: 255},
	"indianred":            {R: 205, G: 92, B: 92, A: 255},
	"ivory":                {R: 255, G: 255, B: 240, A: 255},
	"khaki":                {R: 240, G: 230, B: 140, A: 255},
	"lavender":             {R: 230, G: 230, B: 250, A: 255},
	"lavenderblush":        {R: 255, G: 240, B: 245, A: 255},
	"lawngreen":            {R: 124, G: 252, B: 0, A: 255},
	"lemonchiffon":         {R: 255, G: 250, B: 205, A: 255},
	"lightblue":            {R: 173, G: 216, B: 230, A: 255},
	"lightcoral":           {R: 240, G: 128, B: 128, A: 255},
	"lightcyan":            {R: 224, G: 255, B: 255, A: 255},
	"lightgoldenrodyellow": {R: 250, G: 250, B: 210, A: 255},
	"lightgray":            {R: 211, G: 211, B: 211, A: 255},
	"lightgreen":           {R: 144, G: 238, B: 144, A: 255},
	"lightgrey":            {R: 211, G: 211, B: 211, A: 255},
	"lightpink":            {R: 255, G: 182, B: 193, A: 255},
	"lightsalmon":          {R: 255, G: 160, B: 122, A: 255},
	"lightseagreen":        {R: 32, G: 178, B: 170, A: 255},
	"lightskyblue":         {R: 135, G: 206, B: 250, A: 255},
	"lightslategray":       {R: 119, G: 136, B: 153, A: 255},
	"lightslategrey":       {R: 119, G: 136, B: 153, A: 255},
	"lightsteelblue":       {R: 176, G: 196, B: 222, A: 255},
	"lightyellow":          {R: 255, G: 255, B: 224, A: 255},
	"lime":                 {R: 0, G: 255, B: 0, A: 255},
	"limegreen":            {R: 50, G: 205, B: 50, A: 255},
	"linen":                {R: 250, G: 240, B: 230, A: 255},
	"maroon":               {R: 128, G: 0, B: 0, A: 255},
	"mediumaquamarine":     {R: 102, G: 205, B: 170, A: 255},
	"mediumblue":           {R: 0, G: 0, B: 205, A: 255},
	"mediumorchid":         {R: 186, G: 85, B: 211, A: 255},
	"mediumpurple":         {R: 147, G: 111, B: 219, A: 255},
	"mediumseagreen":       {R: 60, G: 179, B: 113, A: 255},
	"mediumslateblue":      {R: 123, G: 104, B: 238, A: 255},
	"mediumspringgreen":    {R: 0, G: 250, B: 154, A: 255},
	"mediumturquoise":      {R: 72, G: 209, B: 204, A: 255},
	"mediumvioletred":      {R: 199, G: 21, B: 133, A: 255},
	"midnightblue":         {R: 25, G: 25, B: 112, A: 255},
	"mintcream":            {R: 245, G: 255, B: 250, A: 255},
	"mistyrose":            {R: 255, G: 228, B: 225, A: 255},
	"moccasin":             {R: 255, G: 228, B: 181, A: 255},
	"navajowhite":          {R: 255, G: 222, B: 173, A: 255},
	"navy":                 {R: 0, G: 0, B: 128, A: 255},
	"oldlace":              {R: 253, G: 245, B: 230, A: 255},
	"olive":                {R: 128, G: 128, B: 0, A: 255},
	"olivedrab":            {R: 107, G: 142, B: 35, A: 255},
	"orangered":            {R: 255, G: 69, B: 0, A: 255},
	"orchid":               {R: 218, G: 112, B: 214, A: 255},
	"palegoldenrod":        {R: 238, G: 232, B: 170, A: 255},
	"palegreen":            {R: 152, G: 251, B: 152, A: 255},
	"paleturquoise":        {R: 175, G: 238, B: 238, A: 255},
	"palevioletred":        {R: 219, G: 112, B: 147, A: 255},
	"papayawhip":           {R: 255, G: 239, B: 213, A: 255},
	"peachpuff":            {R: 255, G: 218, B: 185, A: 255},
	"peru":                 {R: 205, G: 133, B: 63, A: 255},
	"plum":                 {R: 221, G: 160, B: 221, A: 255},
	"powderblue":           {R: 176, G: 224, B: 230, A: 255},
	"rebeccapurple":        {R: 102, G: 51, B: 153, A: 255},
	"rosybrown":            {R: 188, G: 143, B: 143, A: 255},
	"royalblue":            {R: 65, G: 105, B: 225, A: 255},
	"saddlebrown":          {R: 139, G: 69, B: 19, A: 255},
	"salmon":               {R: 250, G: 128, B: 114, A: 255},
	"sandybrown":           {R: 244, G: 164, B: 96, A: 255},
	"seagreen":             {R: 46, G: 139, B: 87, A: 255},
	"seashell":             {R: 255, G: 245, B: 238, A: 255},
	"sienna":               {R: 160, G: 82, B: 45, A: 255},
	"silver":               {R: 192, G: 192, B: 192, A: 255},
	"skyblue":              {R: 135, G: 206, B: 235, A: 255},
	"slateblue":            {R: 106, G: 90, B: 205, A: 255},
	"slategray":            {R: 112, G: 128, B: 144, A: 255},
	"slategrey":            {R: 112, G: 128, B: 144, A: 255},
	"snow":                 {R: 255, G: 250, B: 250, A: 255},
	"springgreen":          {R: 0, G: 255, B: 127, A: 255},
	"steelblue":            {R: 70, G: 130, B: 180, A: 255},
	"tan":                  {R: 210, G: 180, B: 140, A: 255},
	"teal":                 {R: 0, G: 128, B: 128, A: 255},
	"thistle":              {R: 216, G: 191, B: 216, A: 255},
	"tomato":               {R: 255, G: 99, B: 71, A: 255},
	"turquoise":            {R: 64, G: 224, B: 208, A: 255},
	"wheat":                {R: 245, G: 222, B: 179, A: 255},
	"whitesmoke":           {R: 245, G: 245, B: 245, A: 255},
	"yellowgreen":          {R: 154, G: 205, B: 50, A: 255},
}

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

// applyOpacity multiplies opacity into color alpha channel.
func applyOpacity(c gui.SvgColor, opacity float32) gui.SvgColor {
	if opacity >= 1.0 {
		return c
	}
	return gui.SvgColor{R: c.R, G: c.G, B: c.B, A: uint8(float32(c.A) * opacity)}
}

// parseOpacityAttr extracts an opacity value from element attrs.
func parseOpacityAttr(elem, name string, fallback float32) float32 {
	val, ok := findAttrOrStyle(elem, name)
	if !ok {
		return fallback
	}
	o := parseFloatTrimmed(val)
	if o < 0 {
		return 0
	}
	if o > 1.0 {
		return 1.0
	}
	return o
}

// parseLength parses a CSS length value (ignores units).
func parseLength(s string) float32 {
	end := 0
	for end < len(s) {
		c := s[end]
		if (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '+' {
			end++
		} else if end > 0 && (c == 'e' || c == 'E') {
			// Scientific notation: accept 'e'/'E' optionally
			// followed by '+'/'-'.
			end++
			if end < len(s) && (s[end] == '+' || s[end] == '-') {
				end++
			}
		} else {
			break
		}
	}
	if end == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(s[:end], 32)
	value := float32(v)
	if value > maxCoordinate {
		return maxCoordinate
	}
	if value < -maxCoordinate {
		return -maxCoordinate
	}
	return value
}

func clampViewBoxDim(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > maxViewBoxDim {
		return maxViewBoxDim
	}
	return v
}

// --- Transform parsing ---

// matrixMultiply composes two affine transforms: result = m1 * m2.
func matrixMultiply(m1, m2 [6]float32) [6]float32 {
	return [6]float32{
		m1[0]*m2[0] + m1[2]*m2[1],
		m1[1]*m2[0] + m1[3]*m2[1],
		m1[0]*m2[2] + m1[2]*m2[3],
		m1[1]*m2[2] + m1[3]*m2[3],
		m1[0]*m2[4] + m1[2]*m2[5] + m1[4],
		m1[1]*m2[4] + m1[3]*m2[5] + m1[5],
	}
}

// parseTransform parses SVG transform attribute.
func parseTransform(s string) [6]float32 {
	result := identityTransform
	str := strings.TrimSpace(s)
	pos := 0
	count := 0

	for pos < len(str) {
		count++
		if count > 100 {
			break
		}
		// Skip whitespace and commas
		for pos < len(str) && (str[pos] == ' ' || str[pos] == ',' || str[pos] == '\t') {
			pos++
		}
		if pos >= len(str) {
			break
		}
		// Find transform name
		nameEnd := pos
		for nameEnd < len(str) && str[nameEnd] != '(' && str[nameEnd] != ' ' {
			nameEnd++
		}
		name := str[pos:nameEnd]

		parenStart := strings.IndexByte(str[nameEnd:], '(')
		if parenStart < 0 {
			break
		}
		parenStart += nameEnd
		parenEnd := strings.IndexByte(str[parenStart:], ')')
		if parenEnd < 0 {
			break
		}
		parenEnd += parenStart

		args := parseNumberList(str[parenStart+1 : parenEnd])
		m := parseSingleTransform(name, args)
		result = matrixMultiply(result, m)
		pos = parenEnd + 1
	}
	return result
}

func parseSingleTransform(name string, args []float32) [6]float32 {
	switch name {
	case "matrix":
		if len(args) >= 6 {
			return [6]float32{args[0], args[1], args[2], args[3], args[4], args[5]}
		}
	case "translate":
		tx := float32(0)
		ty := float32(0)
		if len(args) >= 1 {
			tx = args[0]
		}
		if len(args) >= 2 {
			ty = args[1]
		}
		return [6]float32{1, 0, 0, 1, tx, ty}
	case "scale":
		sx := float32(1)
		sy := sx
		if len(args) >= 1 {
			sx = args[0]
			sy = sx
		}
		if len(args) >= 2 {
			sy = args[1]
		}
		return [6]float32{sx, 0, 0, sy, 0, 0}
	case "rotate":
		return parseRotateTransform(args)
	case "skewX":
		if len(args) >= 1 {
			angle := args[0] * math.Pi / 180.0
			return [6]float32{1, 0, float32(math.Tan(float64(angle))), 1, 0, 0}
		}
	case "skewY":
		if len(args) >= 1 {
			angle := args[0] * math.Pi / 180.0
			return [6]float32{1, float32(math.Tan(float64(angle))), 0, 1, 0, 0}
		}
	}
	return identityTransform
}

func parseRotateTransform(args []float32) [6]float32 {
	if len(args) < 1 {
		return identityTransform
	}
	angle := float64(args[0]) * math.Pi / 180.0
	cosA := float32(math.Cos(angle))
	sinA := float32(math.Sin(angle))
	if len(args) >= 3 {
		cx := args[1]
		cy := args[2]
		return [6]float32{
			cosA, sinA, -sinA, cosA,
			cx - cosA*cx + sinA*cy,
			cy - sinA*cx - cosA*cy,
		}
	}
	return [6]float32{cosA, sinA, -sinA, cosA, 0, 0}
}

// applyTransformPt transforms a point by affine matrix.
func applyTransformPt(x, y float32, m [6]float32) (float32, float32) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// applyInheritedTransformPt transforms (x, y) by m when m is an
// "active" matrix. Both the SVG identity matrix and a fully-zero
// matrix are treated as no-ops: identity because the transform is
// a no-op by definition, zero because tests construct ComputedStyle{}
// directly and the resulting zero matrix would otherwise collapse
// every point to the origin.
func applyInheritedTransformPt(x, y float32, m [6]float32) (float32, float32) {
	if m == identityTransform {
		return x, y
	}
	if m[0] == 0 && m[1] == 0 && m[2] == 0 && m[3] == 0 && m[4] == 0 && m[5] == 0 {
		return x, y
	}
	return applyTransformPt(x, y, m)
}

func isIdentityTransform(m [6]float32) bool {
	return m[0] == 1 && m[1] == 0 && m[2] == 0 && m[3] == 1 && m[4] == 0 && m[5] == 0
}

// --- Attribute extraction ---

// unescapeAttrEntities reverses the five entity escapes emitted by
// buildOpenTag's writeAttrEscaped. encoding/xml hands attribute values
// back already entity-decoded, so a legitimate `&` (written as `&amp;`
// in source) round-trips through buildOpenTag as `&amp;`. Downstream
// parsers (color, url, id, transform, …) expect the decoded form, so
// findAttr restores it before returning. Only the five escapes the
// re-encoder produces are reversed; unknown entities pass through
// unchanged. Allocates only when at least one `&` is present.
func unescapeAttrEntities(s string) string {
	if !strings.ContainsRune(s, '&') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '&' {
			b.WriteByte(s[i])
			i++
			continue
		}
		switch {
		case strings.HasPrefix(s[i:], "&amp;"):
			b.WriteByte('&')
			i += 5
		case strings.HasPrefix(s[i:], "&lt;"):
			b.WriteByte('<')
			i += 4
		case strings.HasPrefix(s[i:], "&gt;"):
			b.WriteByte('>')
			i += 4
		case strings.HasPrefix(s[i:], "&quot;"):
			b.WriteByte('"')
			i += 6
		case strings.HasPrefix(s[i:], "&#39;"):
			b.WriteByte('\'')
			i += 5
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// findAttr extracts an attribute value from raw element text.
func findAttr(elem, name string) (string, bool) {
	pos := 0
	for pos < len(elem) {
		idx := strings.Index(elem[pos:], name)
		if idx < 0 {
			return "", false
		}
		idx += pos
		// Verify preceded by whitespace
		if idx == 0 || (elem[idx-1] != ' ' && elem[idx-1] != '\t' &&
			elem[idx-1] != '\n' && elem[idx-1] != '\r') {
			pos = idx + len(name)
			continue
		}
		// Check for '=' after name
		eq := idx + len(name)
		if eq >= len(elem) || elem[eq] != '=' {
			pos = eq
			continue
		}
		q := eq + 1
		if q >= len(elem) {
			return "", false
		}
		quote := elem[q]
		if quote != '"' && quote != '\'' {
			pos = q
			continue
		}
		start := q + 1
		endIdx := strings.IndexByte(elem[start:], quote)
		if endIdx < 0 {
			return "", false
		}
		attrLen := endIdx
		if attrLen > maxAttrLen {
			return "", false
		}
		if attrLen > 0 {
			return unescapeAttrEntities(elem[start : start+endIdx]), true
		}
		return "", false
	}
	return "", false
}

// findStyleProperty extracts a CSS property from a style string.
func findStyleProperty(style, name string) (string, bool) {
	pos := 0
	for pos < len(style) {
		idx := strings.Index(style[pos:], name)
		if idx < 0 {
			return "", false
		}
		idx += pos
		validStart := idx == 0 || style[idx-1] == ';' ||
			style[idx-1] == ' ' || style[idx-1] == '\t'
		if !validStart {
			pos = idx + len(name)
			continue
		}
		colon := idx + len(name)
		for colon < len(style) && (style[colon] == ' ' || style[colon] == '\t') {
			colon++
		}
		if colon >= len(style) || style[colon] != ':' {
			pos = colon
			continue
		}
		valStart := colon + 1
		valEnd := strings.IndexByte(style[valStart:], ';')
		if valEnd < 0 {
			valEnd = len(style) - valStart
		}
		if valEnd > 0 {
			return strings.TrimSpace(style[valStart : valStart+valEnd]), true
		}
		return "", false
	}
	return "", false
}

// findAttrOrStyle checks inline style first, then presentation attribute.
func findAttrOrStyle(elem, name string) (string, bool) {
	if style, ok := findAttr(elem, "style"); ok {
		if val, ok2 := findStyleProperty(style, name); ok2 {
			return val, true
		}
	}
	return findAttr(elem, name)
}

// isValidClipOrFilterValue reports whether v is a parseable
// clip-path/filter declaration value that the cascade should treat
// as authored. Accepts url(#id) references and the "none" keyword;
// rejects bogus tokens so an invalid declaration is ignored rather
// than promoted to authored state.
func isValidClipOrFilterValue(v string) bool {
	t := strings.TrimSpace(v)
	if strings.EqualFold(t, "none") {
		return true
	}
	_, ok := parseFillURL(t)
	return ok
}

// parseFillURL extracts gradient ID from fill="url(#id)".
func parseFillURL(fill string) (string, bool) {
	str := strings.TrimSpace(fill)
	if !strings.HasPrefix(str, "url(") {
		return "", false
	}
	hashPos := strings.IndexByte(str, '#')
	if hashPos < 0 {
		return "", false
	}
	endPos := strings.IndexByte(str[hashPos:], ')')
	if endPos < 0 {
		return "", false
	}
	endPos += hashPos
	if endPos > hashPos+1 {
		return str[hashPos+1 : endPos], true
	}
	return "", false
}

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

func parseElementStyle(elem string) elementStyle {
	return elementStyle{
		Transform:        getTransform(elem),
		StrokeColor:      getStrokeColor(elem),
		StrokeWidth:      getStrokeWidth(elem),
		StrokeCap:        getStrokeLinecap(elem),
		StrokeJoin:       getStrokeLinejoin(elem),
		Opacity:          parseOpacityAttr(elem, "opacity", 1.0),
		FillOpacity:      parseOpacityAttr(elem, "fill-opacity", 1.0),
		StrokeOpacity:    parseOpacityAttr(elem, "stroke-opacity", 1.0),
		StrokeGradientID: getStrokeGradientID(elem),
		StrokeDasharray:  getStrokeDasharray(elem),
	}
}

const strokeCapInherit = gui.SvgStrokeCap(3)
const strokeJoinInherit = gui.SvgStrokeJoin(3)

// presAttrCascadeNames lists the SVG presentation attributes that
// participate as cascade declarations (paint, stroke, opacity,
// fill-rule, font, text-anchor). transform / clip-path / filter are
// handled outside the decl loop because they compose or carry url()
// references rather than being last-write-wins values.
var presAttrCascadeNames = []string{
	"fill", "stroke",
	"stroke-width", "stroke-linecap", "stroke-linejoin",
	"stroke-dasharray", "stroke-dashoffset",
	"opacity", "fill-opacity", "stroke-opacity",
	"fill-rule",
	"font-family", "font-size", "font-weight", "font-style",
	"text-anchor",
	"transform-origin",
	"display", "visibility",
	"clip-path", "filter",
}

// computeStyle walks one element under parent, returning the
// resolved ComputedStyle. Cascade order:
//
//  1. Pres-attr decls (Origin=Pres, spec=0)
//  2. Author CSS rule decls (Origin=Rule, spec from selector)
//  3. Inline style="" decls (Origin=Inline, spec=0)
//  4. !important promotes any layer above all normal layers
//
// Custom properties (--name) are gathered first and var(--x)
// substitution happens at apply time. Transform composes parent ×
// own; clip-path / filter are inherited unless the element overrides.
func computeStyle(
	elem string,
	parent ComputedStyle,
	state *parseState,
	info css.ElementInfo,
	ancestors []css.ElementInfo,
	siblings []css.ElementInfo,
) ComputedStyle {
	out := parent
	out.Transform = matrixMultiply(parent.Transform, getTransform(elem))
	out.Vars = parent.Vars
	// Reset per-element opacity scalar; combine with parent at the
	// end. FillOpacity / StrokeOpacity inherit values directly.
	out.Opacity = 1
	out.FillOpacity = parent.FillOpacity
	out.StrokeOpacity = parent.StrokeOpacity
	// CSS animations are not inherited.
	out.Animation = cssAnimSpec{}
	// transform-origin is not inherited per CSS Transforms 1; reset.
	out.TransformOrigin = ""
	// display is not inherited; visibility is. Reset display so
	// descendants of a non-skipped element start "rendered". Skip
	// logic in parseSvgContent / appendShape filters elements whose
	// own cascade resolves to display:none.
	out.Display = DisplayInline

	// clip-path / filter participate in the cascade so CSS rules and
	// inline style="" can set them, not only the bare attribute. Seed
	// from parent (inherited); the cascade fold below overwrites when
	// the element declares its own value via any origin. After the
	// fold, a fresh FilterID allocates a new per-occurrence FilterGroupKey
	// so two siblings sharing one filter render to two offscreen
	// buffers (composite-z correctness).
	out.ClipPathID = parent.ClipPathID
	out.FilterID = parent.FilterID
	out.FilterGroupKey = parent.FilterGroupKey
	out.AuthoredClipPath = false
	if gid, ok := findAttr(elem, "id"); ok {
		out.GroupID = gid
	} else {
		out.GroupID = parent.GroupID
	}

	var decls []css.MatchedDecl
	for _, name := range presAttrCascadeNames {
		if v, ok := findAttr(elem, name); ok {
			decls = append(decls, css.MatchedDecl{
				Decl:   css.Decl{Name: name, Value: v},
				Origin: css.OriginPresAttr,
			})
		}
	}
	if state != nil && len(state.cssRules) > 0 {
		decls = append(decls,
			css.Match(state.cssRules, info, ancestors, siblings)...)
	}
	if styleAttr, ok := findAttr(elem, "style"); ok {
		for _, d := range parseInlineStyle(styleAttr) {
			decls = append(decls, css.MatchedDecl{
				Decl:   d,
				Origin: css.OriginInline,
			})
		}
	}
	if len(decls) == 0 {
		out.Opacity = parent.Opacity
		return out
	}
	css.SortCascade(decls)

	out.Vars = collectVars(decls, parent.Vars)
	var authoredFilter bool
	for _, d := range decls {
		if d.CustomProp {
			continue
		}
		v := resolveVarRefs(d.Value, out.Vars)
		if v == "" {
			continue
		}
		v = resolveCalcRefs(v)
		if v == "" {
			continue
		}
		if applyCSSAnimProp(d.Name, v, &out.Animation) {
			continue
		}
		// Mark authored only when the declaration parses as a usable
		// value (url(#id) or the "none" keyword). A bogus value like
		// `clip-path: bogus` must be ignored per CSS, otherwise it
		// would either suppress the synthesized nested-svg viewport
		// clip (clip-path) or allocate a fresh per-occurrence filter
		// group buffer (filter) without any actual filter applying.
		switch d.Name {
		case "clip-path":
			if isValidClipOrFilterValue(v) {
				out.AuthoredClipPath = true
			}
		case "filter":
			if isValidClipOrFilterValue(v) {
				authoredFilter = true
			}
		}
		applyCSSProp(d.Name, v, &out)
	}
	out.Opacity = parent.Opacity * out.Opacity
	// Filter group key is per-occurrence: any element that declares
	// filter via its own cascade origin gets a fresh key so two
	// siblings allocate distinct offscreen buffers — even when the
	// declared id matches the inherited one. Pure inheritance (no
	// authored decl) shares the parent's group.
	if authoredFilter {
		if out.FilterID != "" {
			state.nextFilterGroup++
			out.FilterGroupKey = state.nextFilterGroup
		} else {
			out.FilterGroupKey = 0
		}
	}
	return out
}

// collectVars folds custom-property declarations from a sorted
// cascade into a vars map. When the element introduces no new vars,
// the parent's map is shared (no allocation).
func collectVars(decls []css.MatchedDecl,
	parentVars map[string]string,
) map[string]string {
	var out map[string]string
	for _, d := range decls {
		if !d.CustomProp {
			continue
		}
		if out == nil {
			out = make(map[string]string, len(parentVars)+4)
			maps.Copy(out, parentVars)
		}
		out[strings.ToLower(d.Name)] = d.Value
	}
	if out == nil {
		return parentVars
	}
	return out
}

// makeElementInfo builds a css.ElementInfo from the element tag,
// raw open-tag text, and tree-walk metadata (1-based child index
// in parent, isRoot for the root <svg>). attrs is the parsed
// attribute map (nil to disable attribute-selector matching for
// this element). The map is aliased, not copied: callers must treat
// it as read-only — `matchAttr` honors that contract today.
func makeElementInfo(
	tag, openTag string, index int, isRoot bool,
	attrs map[string]string,
) css.ElementInfo {
	info := css.ElementInfo{Tag: tag, Index: index, IsRoot: isRoot}
	if id, ok := findAttr(openTag, "id"); ok {
		info.ID = id
	}
	if cls, ok := findAttr(openTag, "class"); ok {
		info.Classes = splitClassAttr(cls)
	}
	info.Attrs = attrs
	return info
}

// applyPseudoState toggles ElementInfo.State.Hover / Focus when the
// element's id matches parseState.hoveredID / focusedID. Empty IDs
// disable the corresponding state.
func applyPseudoState(info *css.ElementInfo, state *parseState) {
	if state == nil {
		return
	}
	if state.hoveredID != "" && info.ID == state.hoveredID {
		info.State.Hover = true
	}
	if state.focusedID != "" && info.ID == state.focusedID {
		info.State.Focus = true
	}
}

// resolveFillRule reads fill-rule from elem, falling back to the
// inherited value. "evenodd" maps to FillRuleEvenOdd; any other
// token (including the empty string) maps to FillRuleNonzero,
// which is the SVG default. Case-sensitive per SVG spec.
func resolveFillRule(elem string, parent ComputedStyle) FillRule {
	val, ok := findAttrOrStyle(elem, "fill-rule")
	if !ok {
		return parent.FillRule
	}
	if strings.TrimSpace(val) == "evenodd" {
		return FillRuleEvenOdd
	}
	return FillRuleNonzero
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

// applyComputedStyle folds the cascade-resolved style into a
// shape's VectorPath. inh is authoritative for paint properties:
// the cascade has already merged pres-attrs, author CSS, and inline
// style with proper precedence, so any duplicate value the shape
// parser stashed onto path is overwritten. Geometry (Segments,
// Primitive, FillRule) and shape-owned IDs (ClipPathID, GroupID
// when the shape has inline animations) survive.
func applyComputedStyle(path *VectorPath, inh ComputedStyle) {
	// inh.Transform = parent × own (composed in computeStyle). The
	// shape parser already stashed `own` onto path.Transform via
	// parseElementStyle; assigning inh.Transform replaces that with
	// the fully composed matrix and avoids double-applying `own`.
	path.Transform = inh.Transform

	if path.ClipPathID == "" && inh.ClipPathID != "" {
		path.ClipPathID = inh.ClipPathID
	}
	if path.FilterID == "" && inh.FilterID != "" {
		path.FilterID = inh.FilterID
	}
	if path.FilterGroupKey == 0 {
		path.FilterGroupKey = inh.FilterGroupKey
	}

	// Fill — gradient takes precedence over color. Honor cascade
	// winner over the shape's pres-attr-derived value.
	switch {
	case inh.FillGradient != "":
		path.FillGradientID = inh.FillGradient
		path.FillColor = colorTransparent
	case inh.FillSet:
		path.FillColor = inh.Fill
		path.FillGradientID = ""
	case path.FillGradientID == "" && path.FillColor == colorInherit:
		path.FillColor = colorBlack
	}

	switch {
	case inh.StrokeGradient != "":
		path.StrokeGradientID = inh.StrokeGradient
		path.StrokeColor = colorTransparent
	case inh.StrokeSet:
		path.StrokeColor = inh.Stroke
		path.StrokeGradientID = ""
	case path.StrokeColor == colorInherit:
		path.StrokeColor = colorTransparent
	}
	if inh.StrokeWidth >= 0 {
		path.StrokeWidth = inh.StrokeWidth
	}
	if path.StrokeWidth < 0 {
		path.StrokeWidth = 1.0
	}
	if inh.StrokeCap != strokeCapInherit {
		path.StrokeCap = inh.StrokeCap
	}
	if path.StrokeCap == strokeCapInherit {
		path.StrokeCap = gui.SvgButtCap
	}
	if inh.StrokeJoin != strokeJoinInherit {
		path.StrokeJoin = inh.StrokeJoin
	}
	if path.StrokeJoin == strokeJoinInherit {
		path.StrokeJoin = gui.SvgMiterJoin
	}
	if inh.StrokeDasharray != nil {
		path.StrokeDasharray = inh.StrokeDasharray
	}
	if inh.StrokeDashOffsetSet {
		path.StrokeDashOffset = inh.StrokeDashOffset
	}

	if path.GroupID == "" && inh.GroupID != "" {
		path.GroupID = inh.GroupID
	}

	// Mirror cascade-resolved opacity scalars onto path so the
	// gradient tessellator (which composes opacity per-vertex rather
	// than baking into Color.A) reads the same values as
	// bakePathOpacity's solid-color path.
	path.Opacity = inh.Opacity
	path.FillOpacity = inh.FillOpacity
	path.StrokeOpacity = inh.StrokeOpacity

	bakePathOpacity(path, inh)
	path.Computed = inh
}

// bakePathOpacity folds the cascade-resolved opacity values into
// FillColor.A and StrokeColor.A. inh.Opacity already includes the
// element's own opacity multiplied through ancestors. Skip flags
// on the inherited style override the corresponding multiplier with
// 1 so an inline SMIL animation can supply that channel at render
// time without being clipped to zero by the static value
// (e.g. fill-opacity="0").
func bakePathOpacity(path *VectorPath, inh ComputedStyle) {
	// visibility:hidden suppresses paint without removing the element
	// from the box tree. Force fill+stroke alpha to zero so tessellate
	// drops the path and the gradient compositor sees zero opacity.
	if inh.Visibility == VisibilityHidden {
		path.FillColor = applyOpacity(path.FillColor, 0)
		path.StrokeColor = applyOpacity(path.StrokeColor, 0)
		path.Opacity = 0
		path.FillOpacity = 0
		path.StrokeOpacity = 0
		return
	}
	combinedOpacity := inh.Opacity
	if inh.SkipOpacity {
		combinedOpacity = 1
	}
	fillOpacity := inh.FillOpacity
	if inh.SkipFillOpacity {
		fillOpacity = 1
	}
	strokeOpacity := inh.StrokeOpacity
	if inh.SkipStrokeOpacity {
		strokeOpacity = 1
	}
	// Sentinel colors (colorInherit, colorCurrent) carry tiny A
	// markers that would multiply to uint8(0) under common static
	// opacities (e.g. 0.083) and cause tessellate to drop the path.
	// Bump to opaque before baking so the final alpha reflects the
	// element's real opacity. Sentinel RGB (255,0,255) survives —
	// render-side tint still replaces RGB wholesale.
	fillCol := path.FillColor
	if isSentinelColor(fillCol) {
		fillCol.A = 255
	}
	strokeCol := path.StrokeColor
	if isSentinelColor(strokeCol) {
		strokeCol.A = 255
	}
	path.FillColor = applyOpacity(fillCol, clampOpacity01(combinedOpacity*fillOpacity))
	path.StrokeColor = applyOpacity(strokeCol, clampOpacity01(combinedOpacity*strokeOpacity))
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

// sanitizeStrokeWidth maps NaN and negative widths to 0. SVG spec
// treats negative stroke-width as an error; NaN poisons tessellation.
func sanitizeStrokeWidth(v float32) float32 {
	if v != v || v < 0 {
		return 0
	}
	return v
}

// isSentinelColor reports whether c is a colorInherit/colorCurrent
// sentinel. Called pre-opacity-bake so the sentinel's marker alpha
// (A=1 or A=2) is intact; exact match avoids collisions with
// translucent user colors like rgba(255,0,255,0.5).
func isSentinelColor(c gui.SvgColor) bool {
	return c == colorInherit || c == colorCurrent
}

func f32Abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
