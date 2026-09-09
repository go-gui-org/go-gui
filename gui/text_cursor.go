package gui

import "unicode/utf8"

// cursorHome returns position 0 (start of text).
func cursorHome() int {
	return 0
}

// cursorEnd returns the rune count (end of text).
func cursorEnd(text string) int {
	return utf8RuneCount(text)
}

// runeToByteIndex converts a rune index to a byte index.
func runeToByteIndex(text string, runeIdx int) int {
	idx := 0
	for i := 0; i < runeIdx && idx < len(text); i++ {
		_, size := utf8.DecodeRuneInString(text[idx:])
		idx += size
	}
	return idx
}

// byteToRuneIndex converts a byte index to a rune index. byteIdx must
// be a rune boundary: slicing mid-rune counts the partial tail as a
// RuneError and the result drifts by one. All callers pass aligned
// indices (glyph layout offsets or runeToByteIndex output).
func byteToRuneIndex(text string, byteIdx int) int {
	if byteIdx <= 0 {
		return 0
	}
	if byteIdx >= len(text) {
		return utf8RuneCount(text)
	}
	return utf8.RuneCountInString(text[:byteIdx])
}

// cursorStartOfParagraph finds the start of the current
// paragraph by searching backward for newline.
func cursorStartOfParagraph(text string, pos int) int {
	if pos < 0 {
		return 0
	}
	byteIdx := runeToByteIndex(text, pos)
	i := byteIdx - 1
	if i >= len(text) {
		i = len(text) - 1
	}
	for i >= 0 {
		if text[i] == '\n' {
			return byteToRuneIndex(text, i+1)
		}
		i--
	}
	return 0
}

// cursorEndOfParagraph finds the end of the current paragraph
// by searching forward for newline.
func cursorEndOfParagraph(text string, pos int) int {
	byteIdx := runeToByteIndex(text, pos)
	for i := byteIdx; i < len(text); i++ {
		if text[i] == '\n' {
			return byteToRuneIndex(text, i)
		}
	}
	return utf8RuneCount(text)
}

// utf8RuneCount returns the number of runes in s.
func utf8RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}

// truncatePreview truncates s to maxRunes runes, appending
// "..." if truncated. Safe for multi-byte UTF-8.
func truncatePreview(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	byteIdx := runeToByteIndex(s, maxRunes)
	return s[:byteIdx] + "..."
}
