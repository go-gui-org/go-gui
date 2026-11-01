package gui

import "unicode/utf8"

// cursorHome returns position 0 (start of text).
func cursorHome() int {
	return 0
}

// cursorEnd returns the byte offset of the end of text.
func cursorEnd(text string) int {
	return len(text)
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

// cursorStartOfParagraph returns the byte offset after the '\n' before
// pos: the start of the paragraph holding pos.
func cursorStartOfParagraph(text string, pos int) int {
	return lineStartBefore(text, pos)
}

// cursorEndOfParagraph returns the byte offset of the '\n' at or after
// pos, or the text end: the end of the paragraph holding pos.
func cursorEndOfParagraph(text string, pos int) int {
	return lineEndAfter(text, pos)
}

// utf8RuneCount returns the number of runes in s.
func utf8RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}

// truncatePreview truncates s to maxRunes runes, appending
// "..." if truncated. Safe for multi-byte UTF-8. A negative budget
// is treated as zero, so empty input still returns empty.
func truncatePreview(s string, maxRunes int) string {
	if maxRunes < 0 {
		maxRunes = 0
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	byteIdx := runeToByteIndex(s, maxRunes)
	return s[:byteIdx] + "..."
}
