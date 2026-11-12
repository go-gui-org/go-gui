package gui

import (
	"strings"
	"unicode/utf8"
)

// singleLineInputDisplayText keeps the field's value byte-for-byte while
// presenting CR and LF as spaces. A single-line editor has no visible second
// row; shaping a pasted line break as one moved the caret onto a clipped row.
// Replacing the one-byte separators preserves every caret/selection offset,
// including around mixed-direction text.
func singleLineInputDisplayText(text string) string {
	i := strings.IndexAny(text, "\r\n")
	if i < 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	b.WriteString(text[:i])
	for j := i; j < len(text); j++ {
		if text[j] == '\r' || text[j] == '\n' {
			b.WriteByte(' ')
		} else {
			b.WriteByte(text[j])
		}
	}
	return b.String()
}

func inputLineBreakCount(text string) int {
	return strings.Count(text, "\r") + strings.Count(text, "\n")
}

// displayOffset maps a byte offset in text to the same caret position in
// display, the text as drawn: text itself for most fields, a password's
// bullets otherwise. The two share rune positions, not byte offsets. An
// offset the app's new text left inside a character counts from that
// character's start, where the edit core snaps it.
func displayOffset(text, display string, off int) int {
	if display == text {
		return off
	}
	off = runeStartAtOrBefore(text, max(0, min(off, len(text))))
	return advanceRunes(display, utf8.RuneCountInString(text[:off]))
}

// textOffset maps a byte offset in display, the text as drawn, back to
// the field's text.
func textOffset(text, display string, off int) int {
	if display == text {
		return off
	}
	off = max(0, min(off, len(display)))
	return advanceRunes(text, utf8.RuneCountInString(display[:off]))
}

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
