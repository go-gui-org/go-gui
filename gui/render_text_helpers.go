package gui

import "strings"

// maskPassword returns a masked version of text, preserving
// newlines when present.
func maskPassword(text string) string {
	if strings.Contains(text, "\n") {
		return passwordMaskKeepNewlines(text)
	}
	return passwordMask(text)
}

// passwordMaskKeepNewlines replaces each rune with a bullet but
// preserves '\n' characters, matching passwordMask's glyph so
// single-line and multiline passwords render identically.
//
// It repeats passwordMask's stack-buffer shape rather than calling it:
// maskPassword routes every multiline password here, and a masked
// field is rebuilt twice per frame (measure in textView.GenerateLayout,
// paint in renderText), so a heap allocation here is per-frame garbage.
// The bullet bytes are U+2022 in UTF-8, the same three passwordMask
// writes.
func passwordMaskKeepNewlines(text string) string {
	n := utf8RuneCount(text)
	if n <= 64 {
		// Worst case is every rune a bullet: n*3 bytes.
		var buf [64 * 3]byte
		return string(buf[:maskKeepNewlinesInto(buf[:], text)])
	}
	var b strings.Builder
	// One bullet (3 bytes) or one '\n' (1 byte) per rune, so the rune
	// count times 3 is an exact upper bound. Sizing off len(text)
	// would under-reserve for ASCII and force a regrow.
	b.Grow(n * 3)
	for _, r := range text {
		if r == '\n' {
			b.WriteByte('\n')
		} else {
			b.WriteString("\u2022")
		}
	}
	return b.String()
}

// maskKeepNewlinesInto writes the mask into dst and returns the byte
// length written. dst must hold 3 bytes per rune of text. Split out
// only so the caller's stack array stays in the caller's frame; it is
// inlined into the one call site above.
func maskKeepNewlinesInto(dst []byte, text string) int {
	i := 0
	for _, r := range text {
		if r == '\n' {
			dst[i] = '\n'
			i++
			continue
		}
		dst[i], dst[i+1], dst[i+2] = 0xe2, 0x80, 0xa2
		i += 3
	}
	return i
}
