package markdown

import "strings"

// inline.go provides URL safety, image path validation,
// and heading slug generation for the markdown pipeline.

var validImageExts = []string{
	".png", ".jpg", ".jpeg", ".gif", ".svg", ".bmp", ".webp",
}

// IsSafeURL checks that a URL does not use dangerous schemes.
func IsSafeURL(url string) bool {
	trimmed := strings.TrimSpace(url)
	if len(trimmed) == 0 {
		return false
	}
	// Reject control characters outright. An embedded tab or
	// newline inside a scheme name (e.g. "java\tscript:")
	// fails scheme validation below and would otherwise fall
	// through to the plain-relative-path allow rule, while a
	// downstream opener may strip the character and run the
	// scheme. Check after percent-decoding so %0a-style
	// evasions are caught too.
	lower := strings.ToLower(strings.TrimSpace(
		decodePercentPrefix(trimmed)))
	if strings.ContainsFunc(lower, isControlChar) {
		return false
	}
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") {
		return true
	}

	// Safe local references and relative paths.
	if strings.HasPrefix(lower, "#") ||
		strings.HasPrefix(lower, "/") ||
		strings.HasPrefix(lower, "./") ||
		strings.HasPrefix(lower, "../") ||
		strings.HasPrefix(lower, "?") {
		return true
	}

	// Any explicit URI scheme outside the allowlist is blocked.
	if hasURIScheme(lower) {
		return false
	}

	// Plain relative paths without an explicit scheme are safe.
	return true
}

// decodePercentPrefix decodes leading percent-encoded bytes
// (first 40 chars) for scheme detection. Decoding the window slice
// keeps triplets straddling the edge raw: the slice ends where the
// window ends, so no decoded byte overlaps the raw tail.
func decodePercentPrefix(s string) string {
	limit := min(len(s), 40)
	return decodePercentAll(s[:limit]) + s[limit:]
}

// decodePercentAll decodes every valid %XX triplet in s. It feeds
// the traversal check, where partially decoded input would hide
// double-encoded ".." sequences such as %252e.
func decodePercentAll(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	buf := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			hi := hexVal(s[i+1])
			lo := hexVal(s[i+2])
			if hi >= 0 && lo >= 0 {
				buf = append(buf, byte(hi*16+lo))
				i += 3
				continue
			}
		}
		buf = append(buf, s[i])
		i++
	}
	return string(buf)
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// isControlChar reports ASCII control characters and DEL,
// which are never legitimate in a URL and are a known
// scheme-obfuscation vector.
func isControlChar(r rune) bool {
	return r < 0x20 || r == 0x7f
}

func hasURIScheme(s string) bool {
	colon := strings.IndexByte(s, ':')
	if colon <= 0 {
		return false
	}
	for i := range colon {
		c := s[i]
		if c == '/' || c == '?' || c == '#' {
			return false
		}
		if i == 0 {
			if c < 'a' || c > 'z' {
				return false
			}
			continue
		}
		if (c < 'a' || c > 'z') &&
			(c < '0' || c > '9') &&
			c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

// isSafeImagePath validates image paths, blocking traversal
// and absolute paths. Remote http(s) URLs pass the same
// extension allowlist as local paths (checked against the
// path without query or fragment), so a remote non-image
// resource is not fetched as image data.
func isSafeImagePath(path string) bool {
	// Decode repeatedly for this check, so a double-encoded
	// ".." (%252e) cannot hide from it.
	lower := strings.ToLower(path)
	traversal := lower
	for range 3 {
		decoded := decodePercentAll(traversal)
		if decoded == traversal {
			break
		}
		traversal = decoded
	}
	if strings.Contains(traversal, "..") {
		return false
	}
	// Image sources are relative references or http(s) URLs,
	// never filesystem roots: the decoded form catches an
	// encoded leading slash (%2f) too.
	trimmed := strings.TrimSpace(traversal)
	if strings.HasPrefix(trimmed, "/") ||
		strings.HasPrefix(trimmed, "\\") {
		return false
	}
	p := strings.TrimSpace(lower)
	if !IsSafeURL(path) {
		return false
	}
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	for _, ext := range validImageExts {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// HeadingSlug converts heading text to a URL-safe slug.
// Lowercase alphanumeric + dashes, no trailing dashes.
// ASCII-only by design: non-ASCII runes are dropped, so
// non-Latin headings may slug to "".
func headingSlug(text string) string {
	var buf []byte
	prevDash := false
	for _, r := range text {
		switch {
		case r >= 'A' && r <= 'Z':
			buf = append(buf, byte(r+32)) // lowercase
			prevDash = false
		case (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9'):
			buf = append(buf, byte(r))
			prevDash = false
		case r > 127:
			// Drop non-ASCII runes cleanly.
		default:
			if len(buf) > 0 && !prevDash {
				buf = append(buf, '-')
				prevDash = true
			}
		}
	}
	// Trim trailing dash.
	for len(buf) > 0 && buf[len(buf)-1] == '-' {
		buf = buf[:len(buf)-1]
	}
	return string(buf)
}
