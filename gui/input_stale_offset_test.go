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
