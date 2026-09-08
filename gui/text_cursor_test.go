package gui

import "testing"

// --- cursor_end ---

func TestCursorEnd3Byte(t *testing.T) {
	assertEqual(t, cursorEnd("€€€"), 3)
}

func TestCursorEnd4Byte(t *testing.T) {
	assertEqual(t, cursorEnd("𝐀𝐁𝐂"), 3)
}

func TestCursorEndEmojiOnly(t *testing.T) {
	assertEqual(t, cursorEnd("😀😀"), 2)
}

func TestCursorEndEmpty(t *testing.T) {
	assertEqual(t, cursorEnd(""), 0)
}

func TestCursorEndCombiningChar(t *testing.T) {
	// e + combining acute = 2 runes
	assertEqual(t, cursorEnd("e\u0301"), 2)
}

func TestCursorEndSingle4Byte(t *testing.T) {
	assertEqual(t, cursorEnd("😀"), 1)
}

// --- paragraph navigation ---

func TestCursorStartOfParagraphMidLine(t *testing.T) {
	text := "abc\ndef\nghi"
	// pos 5 is in "def" → start of "def" is after first \n
	assertEqual(t, cursorStartOfParagraph(text, 5), 4)
}

func TestCursorStartOfParagraphBeginning(t *testing.T) {
	text := "hello world"
	assertEqual(t, cursorStartOfParagraph(text, 3), 0)
}

func TestCursorEndOfParagraphMidLine(t *testing.T) {
	text := "abc\ndef\nghi"
	// pos 1 → find \n at byte 3 → rune 3
	assertEqual(t, cursorEndOfParagraph(text, 1), 3)
}

func TestCursorEndOfParagraphLastLine(t *testing.T) {
	text := "abc\ndef"
	assertEqual(t, cursorEndOfParagraph(text, 5), 7)
}

// --- helpers ---

func TestRuneToByteIndex(t *testing.T) {
	text := "a€b" // a=1byte, €=3bytes, b=1byte
	assertEqual(t, runeToByteIndex(text, 0), 0)
	assertEqual(t, runeToByteIndex(text, 1), 1) // after 'a'
	assertEqual(t, runeToByteIndex(text, 2), 4) // after '€'
}

func TestByteToRuneIndex(t *testing.T) {
	text := "a€b"
	assertEqual(t, byteToRuneIndex(text, 0), 0)
	assertEqual(t, byteToRuneIndex(text, 1), 1)
	assertEqual(t, byteToRuneIndex(text, 4), 2)
}

func assertEqual(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}
