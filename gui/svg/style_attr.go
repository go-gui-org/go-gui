package svg

import "strings"

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
