package svg

import (
	"math"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// isFiniteF32 reports whether v is a finite float32 (not NaN or Inf).
// Local copy to avoid cross-package import; gui has the same helper.
func isFiniteF32(v float32) bool {
	f := float64(v)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// maxStyleBlockBytes caps the total CSS source we'll feed the
// tokenizer per document. Prevents pathological assets that embed
// megabytes of CSS from bloating parse time.
const maxStyleBlockBytes = 64 << 10

// collectStyleBlocks walks the SVG tree, returning the concatenated
// text of every <style> element. Blocks are joined with a newline
// so source-order numbering across blocks stays consistent.
func collectStyleBlocks(root *xmlNode) string {
	var b strings.Builder
	collectStyleBlocksInto(root, &b, 0)
	out := b.String()
	if len(out) > maxStyleBlockBytes {
		out = out[:maxStyleBlockBytes]
	}
	return out
}

// maxStyleScanDepth caps recursion depth when scanning the SVG tree
// for <style> blocks. Defends against pathologically nested input
// (each level pushes a stack frame) — real SVGs nest <20 deep.
const maxStyleScanDepth = 256

func collectStyleBlocksInto(n *xmlNode, b *strings.Builder, depth int) {
	if depth >= maxStyleScanDepth {
		return
	}
	if n.Name == "style" {
		// Concatenated CharData: encoding/xml unwraps CDATA into
		// plain text, so n.Text already holds the rule source.
		text := strings.TrimSpace(n.Text)
		if text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(text)
		}
		return
	}
	if b.Len() >= maxStyleBlockBytes {
		return
	}
	for i := range n.Children {
		collectStyleBlocksInto(&n.Children[i], b, depth+1)
	}
}

func splitClassAttr(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		isWS := c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
		if isWS {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// applyCSSProp folds one CSS declaration into out. Unknown property
// names are silently dropped. The caller orders declarations per the
// cascade (origin, !important, specificity, source order) and applies
// them low-to-high so the last write wins per property. Animation
// properties (animation-*) are routed via applyCSSAnimProp before
// this is reached.
func applyCSSProp(name, value string, out *ComputedStyle) {
	v := strings.TrimSpace(value)
	if v == "" {
		return
	}
	if applyCSSPaintProp(name, v, out) {
		return
	}
	applyCSSLayoutProp(name, v, out)
}

// applyCSSPaintProp handles paint, stroke, and opacity declarations.
// Returns true when name matched and the property was folded.
func applyCSSPaintProp(name, v string, out *ComputedStyle) bool {
	switch name {
	case "fill":
		applyPaintProp(v, &out.Fill, &out.FillSet, &out.FillGradient)
	case "stroke":
		applyPaintProp(v, &out.Stroke, &out.StrokeSet, &out.StrokeGradient)
	case "stroke-width":
		out.StrokeWidth = sanitizeStrokeWidth(parseLength(v))
	case "stroke-linecap":
		out.StrokeCap = parseStrokeCap(v)
	case "stroke-linejoin":
		out.StrokeJoin = parseStrokeJoin(v)
	case "stroke-dasharray":
		out.StrokeDasharray = parseDashList(v)
	case "stroke-dashoffset":
		n := parseFloatTrimmed(v)
		if isFiniteF32(n) {
			out.StrokeDashOffset = n
			out.StrokeDashOffsetSet = true
		}
	case "opacity":
		out.Opacity = clampOpacity01(parseOpacityNumber(v))
	case "fill-opacity":
		out.FillOpacity = clampOpacity01(parseOpacityNumber(v))
	case "stroke-opacity":
		out.StrokeOpacity = clampOpacity01(parseOpacityNumber(v))
	case "fill-rule":
		if v == "evenodd" {
			out.FillRule = FillRuleEvenOdd
		} else {
			out.FillRule = FillRuleNonzero
		}
	default:
		return false
	}
	return true
}

// applyCSSLayoutProp handles font/text, display/visibility, transform
// origin, and the clip-path/filter url() references.
func applyCSSLayoutProp(name, v string, out *ComputedStyle) {
	switch name {
	case "font-family":
		out.FontFamily = v
	case "font-size":
		out.FontSize = v
	case "font-weight":
		out.FontWeight = v
	case "font-style":
		out.FontStyle = v
	case "text-anchor":
		out.TextAnchor = v
	case "transform-origin":
		out.TransformOrigin = v
	case "display":
		if strings.EqualFold(v, "none") {
			out.Display = DisplayNone
		} else {
			out.Display = DisplayInline
		}
	case "visibility":
		switch strings.ToLower(v) {
		case "hidden", "collapse":
			out.Visibility = VisibilityHidden
		default:
			out.Visibility = VisibilityVisible
		}
	case "clip-path":
		if id, ok := parseFillURL(v); ok {
			out.ClipPathID = id
		} else if strings.EqualFold(v, "none") {
			out.ClipPathID = ""
		}
	case "filter":
		if id, ok := parseFillURL(v); ok {
			out.FilterID = id
		} else if strings.EqualFold(v, "none") {
			out.FilterID = ""
		}
	}
}

// applyPaintProp folds one fill/stroke declaration into the supplied
// slots. CSS-wide control keywords are no-ops so the cascade-copied
// parent value survives; unparseable syntax is dropped per CSS
// "invalid → ignore" rather than clobbering with transparent black.
func applyPaintProp(v string, color *gui.SvgColor, set *bool, gradient *string) {
	if isCSSInheritKeyword(v) {
		return
	}
	if gid, ok := parseFillURL(v); ok {
		*gradient = gid
		*color = colorTransparent
		*set = true
		return
	}
	c, parsed := parseSvgColor(v)
	if !parsed {
		return
	}
	*gradient = ""
	*color = c
	*set = true
}

// isCSSInheritKeyword reports whether v is a CSS-wide control
// keyword that preserves the cascaded value. Trimming guards against
// callers that haven't normalized whitespace.
func isCSSInheritKeyword(v string) bool {
	t := strings.TrimSpace(v)
	return strings.EqualFold(t, "inherit") ||
		strings.EqualFold(t, "unset") ||
		strings.EqualFold(t, "revert") ||
		strings.EqualFold(t, "revert-layer")
}

// parseOpacityNumber parses opacity-like values. Accepts a unitless
// number (0..1) or a percentage (0%..100%). The latter is what
// authoring tools emit when sliding an opacity control; svg-spinners
// ships several assets with `opacity: 50%` etc.
func parseOpacityNumber(v string) float32 {
	v = strings.TrimSpace(v)
	if strings.HasSuffix(v, "%") {
		return parseFloatTrimmed(v[:len(v)-1]) / 100
	}
	return parseFloatTrimmed(v)
}

// parseDashList parses a stroke-dasharray value. "none" or an empty
// list returns nil; negative or zero-sum lists also return nil per
// SVG spec ("solid line"). Odd-length lists are doubled so the dash
// pattern is symmetric.
func parseDashList(v string) []float32 {
	if strings.TrimSpace(v) == "none" {
		return nil
	}
	out := make([]float32, 0, 4)
	var sum float32
	for i := 0; i < len(v); {
		for i < len(v) && isFloatListSep(v[i]) {
			i++
		}
		if i >= len(v) {
			break
		}
		end := i
		for end < len(v) && !isFloatListSep(v[end]) {
			end++
		}
		n := parseFloatTrimmed(v[i:end])
		// NaN/Inf reject — propagating non-finite dash lengths
		// poisons stroke geometry. `<0` is also a spec reject.
		if !isFiniteF32(n) || n < 0 {
			return nil
		}
		out = append(out, n)
		sum += n
		i = end
	}
	if sum <= 0 {
		return nil
	}
	if len(out)%2 != 0 {
		out = append(out, out...)
	}
	return out
}

// parseInlineStyle splits a style="" attribute into Decls. Strips
// trailing "!important" (case-insensitive) and tags --foo names as
// custom properties.
func parseInlineStyle(s string) []css.Decl {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	const impTok = "!important"
	var out []css.Decl
	for segment := range strings.SplitSeq(s, ";") {
		seg := strings.TrimSpace(segment)
		if seg == "" {
			continue
		}
		ci := strings.IndexByte(seg, ':')
		if ci <= 0 {
			continue
		}
		rawName := strings.TrimSpace(seg[:ci])
		value := strings.TrimSpace(seg[ci+1:])
		if rawName == "" || value == "" {
			continue
		}
		important := false
		if l := len(value); l >= len(impTok) {
			if strings.EqualFold(value[l-len(impTok):], impTok) {
				important = true
				value = strings.TrimSpace(value[:l-len(impTok)])
			}
		}
		if value == "" {
			continue
		}
		custom := strings.HasPrefix(rawName, "--")
		nm := strings.ToLower(rawName)
		if !custom {
			nm = css.StripVendorPrefix(nm)
		}
		out = append(out, css.Decl{
			Name:       nm,
			Value:      value,
			Important:  important,
			CustomProp: custom,
		})
	}
	return out
}

// resolveVarRefs substitutes var(--name) and var(--name, fallback)
// references in v using the supplied vars map. Undefined references
// without a fallback drop the whole value (returns "") per CSS
// "invalid-at-computed-value-time → initial". With a fallback, the
// fallback is itself var-resolved recursively.
//
// Cyclic (--a → --b → --a) and deeply-nested chains are bounded by
// maxVarRecursion; exceeding the cap drops the value.
func resolveVarRefs(v string, vars map[string]string) string {
	return resolveVarRefsAt(v, vars, 0)
}

const maxVarRecursion = 32

func resolveVarRefsAt(v string, vars map[string]string, depthIn int) string {
	if !strings.Contains(v, "var(") {
		return v
	}
	if depthIn >= maxVarRecursion {
		return ""
	}
	var b strings.Builder
	i := 0
	for i < len(v) {
		idx := strings.Index(v[i:], "var(")
		if idx < 0 {
			b.WriteString(v[i:])
			break
		}
		b.WriteString(v[i : i+idx])
		argStart := i + idx + len("var(")
		j, ok := findClosingParen(v, argStart)
		if !ok {
			return ""
		}
		argText := strings.TrimSpace(v[argStart:j])
		// Split name from fallback at the FIRST top-level comma. The
		// fallback may itself be `var(--x, default)`, so naive Index
		// would split inside the inner var().
		name, fallback, hasFallback := splitVarArgs(argText)
		repl, ok := vars[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			if !hasFallback {
				return ""
			}
			repl = strings.TrimSpace(fallback)
		}
		repl = resolveVarRefsAt(repl, vars, depthIn+1)
		if repl == "" {
			return ""
		}
		b.WriteString(repl)
		i = j + 1
	}
	return b.String()
}

// findClosingParen returns the index of the `)` that matches the
// open paren whose position is start-1. start points one past the
// opening `(`. Used by `var()` and `calc()` resolvers to locate the
// end of a function call within a CSS value string. Returns ok=false
// when the input is not balanced.
func findClosingParen(s string, start int) (int, bool) {
	depth := 1
	for j := start; j < len(s); j++ {
		switch s[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return j, true
			}
		}
	}
	return -1, false
}

// splitVarArgs splits a var() argument list at the first top-level
// comma. Returns (name, fallback, hasFallback). Top-level means
// outside any nested parens, so a fallback containing
// `var(--x, default)` does not get prematurely split.
func splitVarArgs(arg string) (string, string, bool) {
	depth := 0
	for i := 0; i < len(arg); i++ {
		switch arg[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				return arg[:i], arg[i+1:], true
			}
		}
	}
	return arg, "", false
}
