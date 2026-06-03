package svg

// xml_text.go — SVG <text>, <tspan>, and <textPath> parsing.

import (
	"html"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// maxTextRunBytes caps a single emitted text run. Defends shaping
// and atlas-cache from hostile CharData that survived file-size cap.
const maxTextRunBytes = 64 << 10

// prepareTextRun trims, length-caps, then HTML-unescapes a raw text
// node body. Cap before unescape so entity expansion can't blow out
// the buffer.
func prepareTextRun(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	if len(t) > maxTextRunBytes {
		t = t[:maxTextRunBytes]
	}
	return html.UnescapeString(t)
}

// parseTextElement extracts text from a <text> element, including
// <tspan> children and <textPath> references. textAncestors must
// already include this element so descendant <tspan>s see <text> as
// a parent during their own cascade.
func parseTextElement(n *xmlNode, computed ComputedStyle, state *parseState,
	textAncestors []css.ElementInfo) {
	elem := n.OpenTag
	x := attrFloat(elem, "x", 0)
	y := attrFloat(elem, "y", 0)
	p := buildTextAttrsFromComputed(elem, computed)
	p.x, p.y = x, y
	parseTextBody(n, p, state, computed, textAncestors)
}

// buildTextAttrsFromComputed projects a cascaded ComputedStyle onto
// the textParentAttrs the body/tspan/textPath emitters consume.
// text-decoration / letter-spacing aren't in the cascade yet so they
// still read raw attr/style on elem.
func buildTextAttrsFromComputed(elem string, computed ComputedStyle) textParentAttrs {
	fontSize := float32(16)
	if computed.FontSize != "" {
		fontSize = parseLength(computed.FontSize)
	}
	fontFamily := cleanFontFamily(computed.FontFamily)

	color := colorBlack
	if computed.FillSet {
		color = computed.Fill
	}
	fillGradientID := computed.FillGradient

	anchor := uint8(0)
	switch computed.TextAnchor {
	case "middle":
		anchor = 1
	case "end":
		anchor = 2
	}

	fontWeight := parseFontWeight(computed.FontWeight)
	bold := fontWeight >= 600
	italic := computed.FontStyle == "italic" || computed.FontStyle == "oblique"

	underline, strikethrough := false, false
	if td, ok := findAttrOrStyle(elem, "text-decoration"); ok {
		if strings.Contains(td, "underline") {
			underline = true
		}
		if strings.Contains(td, "line-through") {
			strikethrough = true
		}
	}
	var letterSpacing float32
	if ls, ok := findAttrOrStyle(elem, "letter-spacing"); ok {
		letterSpacing = parseLength(ls)
	}

	var strokeColor gui.SvgColor
	var strokeWidth float32
	if computed.StrokeSet {
		strokeColor = computed.Stroke
		strokeWidth = sanitizeStrokeWidth(computed.StrokeWidth)
		// stroke="none" cascades as transparent; drop the width so
		// renderers don't emit a zero-alpha hairline.
		if strokeColor.A == 0 {
			strokeWidth = 0
		} else if isSentinelColor(strokeColor) {
			strokeColor = colorBlack
		}
		if strokeColor.A != 0 && strokeWidth == 0 {
			strokeWidth = 1
		}
	} else if rawStroke, ok := findAttrOrStyle(elem, "stroke"); ok {
		// `stroke="inherit"` / `"currentColor"` on <text> with no
		// ancestor stroke leaves StrokeSet=false. Promote to a visible
		// default so the declaration isn't silently a no-op.
		trimmed := strings.TrimSpace(rawStroke)
		if isCSSInheritKeyword(trimmed) ||
			strings.EqualFold(trimmed, "currentcolor") {
			strokeColor = colorBlack
			strokeWidth = 1
		}
	}

	return textParentAttrs{
		fontSize: fontSize, fontFamily: fontFamily,
		color: color, fillGradientID: fillGradientID,
		anchor: anchor, bold: bold, italic: italic,
		fontWeight: fontWeight,
		underline:  underline, strikethrough: strikethrough,
		letterSpacing: letterSpacing,
		strokeColor:   strokeColor, strokeWidth: strokeWidth,
		opacity:        computed.Opacity,
		filterID:       computed.FilterID,
		filterGroupKey: computed.FilterGroupKey,
	}
}

// textParentAttrs holds inherited attributes from a <text> element.
type textParentAttrs struct {
	x, y                     float32
	fontSize                 float32
	fontFamily               string
	color                    gui.SvgColor
	fillGradientID           string
	anchor                   uint8
	bold, italic             bool
	fontWeight               int
	underline, strikethrough bool
	letterSpacing            float32
	strokeColor              gui.SvgColor
	strokeWidth              float32
	opacity                  float32
	filterID                 string
	filterGroupKey           uint32
}

// parseTextBody walks the direct text and <tspan>/<textPath>
// children of a <text> element node. parentComputed and ancestors
// are forwarded so each <tspan> can run its own cascade against the
// <text> element as parent.
func parseTextBody(n *xmlNode, p textParentAttrs, state *parseState,
	parentComputed ComputedStyle, ancestors []css.ElementInfo) {
	curY := p.y

	// Direct text that precedes any child element.
	lead := prepareTextRun(n.Leading)
	if lead != "" {
		state.texts = append(state.texts,
			makeTextFromParent(lead, p.x, curY, p))
	}

	if len(n.Children) == 0 {
		return
	}
	siblings := make([]css.ElementInfo, 0, len(n.Children))
	for i := range n.Children {
		c := &n.Children[i]
		info := makeElementInfo(c.Name, c.OpenTag, i+1, false, c.AttrMap)
		applyPseudoState(&info, state)
		sibsForThis := siblings
		siblings = append(siblings, info)
		switch c.Name {
		case "tspan":
			if !c.SelfClose {
				parseTspan(c, p, &curY, state, parentComputed,
					info, ancestors, sibsForThis)
			}
		case "textPath":
			if !c.SelfClose {
				parseTextPathChild(c, p, state)
			}
		}
		if c.Tail == "" {
			continue
		}
		if tail := prepareTextRun(c.Tail); tail != "" {
			state.texts = append(state.texts,
				makeTextFromParent(tail, p.x, curY, p))
		}
	}
}

// makeTextFromParent creates an SvgText inheriting parent attrs.
func makeTextFromParent(text string, x, y float32, p textParentAttrs) gui.SvgText {
	return gui.SvgText{
		Text:           text,
		X:              x,
		Y:              y,
		FontFamily:     p.fontFamily,
		FontSize:       p.fontSize,
		IsBold:         p.bold,
		IsItalic:       p.italic,
		FontWeight:     p.fontWeight,
		Color:          p.color,
		FillGradientID: p.fillGradientID,
		FilterID:       p.filterID,
		FilterGroupKey: p.filterGroupKey,
		Anchor:         int(p.anchor),
		Opacity:        p.opacity,
		Underline:      p.underline,
		Strikethrough:  p.strikethrough,
		LetterSpacing:  p.letterSpacing,
		StrokeColor:    p.strokeColor,
		StrokeWidth:    p.strokeWidth,
	}
}

// parseTspan parses a <tspan> element, running the CSS cascade so
// author rules and pseudo-state matches reach tspans the same way
// they reach shapes. parentComputed is the <text> element's resolved
// style; ancestors is the chain rooted at <svg> through <text>.
func parseTspan(n *xmlNode, p textParentAttrs, curY *float32, state *parseState,
	parentComputed ComputedStyle, info css.ElementInfo,
	ancestors, siblings []css.ElementInfo) {
	elem := n.OpenTag
	text := prepareTextRun(n.Text)
	if text == "" {
		return
	}

	computed := computeStyle(elem, parentComputed, state, info,
		ancestors, siblings)
	if computed.Display == DisplayNone {
		return
	}

	// Positional attributes are not styleable; read raw markup.
	tx := p.x
	if xv, ok := findAttr(elem, "x"); ok {
		tx = parseF32(xv)
	}
	ty := *curY
	if yv, ok := findAttr(elem, "y"); ok {
		ty = parseF32(yv)
	}
	if dy, ok := findAttr(elem, "dy"); ok {
		ty += parseLength(dy)
	}
	*curY = ty

	fontSize := p.fontSize
	if computed.FontSize != "" {
		fontSize = parseLength(computed.FontSize)
	}
	fontFamily := p.fontFamily
	if computed.FontFamily != "" {
		fontFamily = cleanFontFamily(computed.FontFamily)
	}
	fontWeight := p.fontWeight
	bold := p.bold
	if computed.FontWeight != "" {
		fontWeight = parseFontWeight(computed.FontWeight)
		bold = fontWeight >= 600
	}
	italic := p.italic
	if computed.FontStyle != "" {
		italic = computed.FontStyle == "italic" ||
			computed.FontStyle == "oblique"
	}
	color := p.color
	fillGradientID := p.fillGradientID
	if computed.FillSet {
		color = computed.Fill
		fillGradientID = computed.FillGradient
	}
	strokeColor := p.strokeColor
	strokeWidth := p.strokeWidth
	if computed.StrokeSet {
		strokeColor = computed.Stroke
		strokeWidth = computed.StrokeWidth
	}
	// computeStyle already composes opacity through the cascade.
	// Multiplying by p.opacity here would re-apply ancestor opacity.
	opacity := computed.Opacity

	anchor := p.anchor
	switch computed.TextAnchor {
	case "start":
		anchor = 0
	case "middle":
		anchor = 1
	case "end":
		anchor = 2
	}

	// text-decoration / letter-spacing not yet in cascade.
	underline := p.underline
	strikethrough := p.strikethrough
	if td, ok := findAttrOrStyle(elem, "text-decoration"); ok {
		underline = strings.Contains(td, "underline")
		strikethrough = strings.Contains(td, "line-through")
	}
	letterSpacing := p.letterSpacing
	if ls, ok := findAttrOrStyle(elem, "letter-spacing"); ok {
		letterSpacing = parseLength(ls)
	}

	state.texts = append(state.texts, gui.SvgText{
		Text:           text,
		X:              tx,
		Y:              ty,
		FontFamily:     fontFamily,
		FontSize:       fontSize,
		IsBold:         bold,
		IsItalic:       italic,
		FontWeight:     fontWeight,
		Color:          color,
		FillGradientID: fillGradientID,
		FilterID:       computed.FilterID,
		FilterGroupKey: computed.FilterGroupKey,
		Anchor:         int(anchor),
		Opacity:        opacity,
		Underline:      underline,
		Strikethrough:  strikethrough,
		LetterSpacing:  letterSpacing,
		StrokeColor:    strokeColor,
		StrokeWidth:    strokeWidth,
	})
}

// parseTextPathChild parses a <textPath> child element.
func parseTextPathChild(n *xmlNode, p textParentAttrs, state *parseState) {
	elem := n.OpenTag
	text := prepareTextRun(n.Text)
	if text == "" {
		return
	}

	// Extract href (try href first, then xlink:href).
	pathRef, ok := findAttr(elem, "href")
	if !ok {
		pathRef, ok = findAttr(elem, "xlink:href")
	}
	if !ok || pathRef == "" {
		return
	}
	pathID := strings.TrimPrefix(pathRef, "#")

	// startOffset.
	var startOffset float32
	isPercent := false
	if so, ok := findAttr(elem, "startOffset"); ok {
		trimmed := strings.TrimSpace(so)
		if strings.HasSuffix(trimmed, "%") {
			startOffset = parseF32(trimmed[:len(trimmed)-1])
			isPercent = true
		} else {
			startOffset = parseLength(trimmed)
		}
	}

	// text-anchor on <textPath> overrides parent <text>.
	anchor := p.anchor
	if anc, ok := findAttr(elem, "text-anchor"); ok {
		switch anc {
		case "middle":
			anchor = 1
		case "end":
			anchor = 2
		}
	}

	state.textPaths = append(state.textPaths, gui.SvgTextPath{
		Text:           text,
		PathID:         pathID,
		FontFamily:     p.fontFamily,
		FontSize:       p.fontSize,
		IsBold:         p.bold,
		IsItalic:       p.italic,
		FontWeight:     p.fontWeight,
		Color:          p.color,
		StrokeColor:    p.strokeColor,
		StrokeWidth:    p.strokeWidth,
		FilterID:       p.filterID,
		FilterGroupKey: p.filterGroupKey,
		Anchor:         int(anchor),
		Opacity:        p.opacity,
		LetterSpacing:  p.letterSpacing,
		StartOffset:    startOffset,
		IsPercent:      isPercent,
	})
}

// parseFontWeight converts a CSS font-weight string to a
// numeric value (100-900). Returns 0 for unset/inherit.
func parseFontWeight(fw string) int {
	switch fw {
	case "bold", "bolder":
		return 700
	case "normal", "lighter":
		return 400
	case "":
		return 0
	}
	w := int(parseF32(fw))
	if w >= 100 && w <= 900 {
		return w
	}
	return 0
}

// cleanFontFamily extracts the first font name from a CSS
// font-family list (e.g. "Courier New, monospace" → "Courier New").
func cleanFontFamily(ff string) string {
	if before, _, found := strings.Cut(ff, ","); found {
		return strings.TrimSpace(before)
	}
	return ff
}
