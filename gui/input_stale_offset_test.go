package gui

import (
	"testing"
	"unicode/utf8"
)

// Offsets are bytes of the text they were taken in. After the app sets
// another text they can fall inside a character of it; every edit and
// the copy must then act on whole characters, never commit a split one.
func TestEditStaleOffsetsSnapToRuneStart(t *testing.T) {
	const text = "日本語" // three 3-byte runes
	caret := inputState{CursorPos: 4}
	sel := inputState{CursorPos: 5, selectBeg: 1, selectEnd: 5}
	cases := []struct {
		name string
		edit func() (string, bool)
		want string
	}{
		{"insert", func() (string, bool) {
			got, _, ok := editInsert(text, "x", caret)
			return got, ok
		}, "日x本語"},
		{"backspace", func() (string, bool) {
			got, _, ok := editDelete(text, caret, false)
			return got, ok
		}, "本語"},
		{"delete", func() (string, bool) {
			got, _, ok := editDelete(text, caret, true)
			return got, ok
		}, "日語"},
		{"replace selection", func() (string, bool) {
			got, _, ok := editInsert(text, "x", sel)
			return got, ok
		}, "x本語"},
		{"copy", func() (string, bool) {
			return editSelectedText(text, sel, false)
		}, "日"},
		{"delete surrounding", func() (string, bool) {
			got, _, ok := editDeleteSurrounding(text, caret, false, 1, true)
			return got, ok
		}, "本語"},
	}
	for _, c := range cases {
		got, ok := c.edit()
		if !ok || got != c.want || !utf8.ValidString(got) {
			t.Errorf("%s: got (%q, %v), want (%q, true)", c.name, got, ok, c.want)
		}
	}
}

// The reported case end to end: the caret sits past a few ASCII bytes,
// the app sets text of wider characters, and the user types.
func TestInputTypingAfterAppSetTextStaysValid(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 300, Height: 100})
	t.Cleanup(w.WindowCleanup)
	text := "abcd"
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Input(InputCfg{ID: "f", Text: text, Sizing: FillFit,
				OnTextChanged: func(s string, _ EventCtx) { text = s }}),
		}})
	})
	w.SetFocus("f")
	_ = w.TestKey("f", KeyEnd, ModNone) // caret at byte 4
	text = "日本語"
	w.TestRender(nil)
	if err := w.TestType("f", "x"); err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(text) || text != "日x本語" {
		t.Fatalf("typing after the app set %q gave %q", "日本語", text)
	}
}

// The walk back to a rune start is bounded, in text that is not UTF-8 as
// in text that is.
func TestRuneStartAtOrBefore(t *testing.T) {
	cases := []struct {
		text string
		off  int
		want int
	}{
		{"日本", 4, 3},
		{"日本", 3, 3},
		{"日本", 6, 6},                     // the end
		{"日本", 9, 9},                     // past it: left to the caller
		{"a\x80\x80\x80\x80\x80b", 5, 2}, // three bytes back, then stop
		{"a\x80\x80\x80\x80\x80b", 6, 6},
	}
	for _, c := range cases {
		if got := runeStartAtOrBefore(c.text, c.off); got != c.want {
			t.Errorf("runeStartAtOrBefore(%q, %d) = %d, want %d",
				c.text, c.off, got, c.want)
		}
	}
}

// A masked field converts its byte offsets to the mask's rune slots. A
// stale offset inside a character snaps first, or a selection over that
// character converts to an empty one and the insert lands beside it.
func TestMaskRuneArgsSnapStaleOffsets(t *testing.T) {
	is := inputState{CursorPos: 2, selectBeg: 1, selectEnd: 2}
	cursor, beg, end := maskRuneArgs("é1", is)
	if cursor != 1 || beg != 0 || end != 1 {
		t.Errorf("maskRuneArgs = (%d, %d, %d), want (1, 0, 1)", cursor, beg, end)
	}
}

// A password draws bullets. A stale offset inside a character of the
// text maps to the bullet before that character, where the edit core
// snaps the same offset, not to the bullet after it.
func TestDisplayOffsetSnapsStaleOffset(t *testing.T) {
	text := "éa"
	if got := displayOffset(text, maskPassword(text), 1); got != 0 {
		t.Errorf("displayOffset(%q, 1) = %d, want 0", text, got)
	}
}
