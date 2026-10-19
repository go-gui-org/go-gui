package gui

import "testing"

// imeDeleteWindow focuses an Input holding text with the caret at pos and
// returns the window and a pointer to the text OnTextChanged last saw.
func imeDeleteWindow(t *testing.T, cfg InputCfg, pos int) (*Window, *string) {
	t.Helper()
	w := sizedWindow()
	w.focused = true
	got := cfg.Text
	cfg.ID = "f"
	cfg.OnTextChanged = func(s string, _ EventCtx) { got = s }
	w.SetFocus("f")
	setInputState(w, "f", inputState{CursorPos: pos})
	w.layout = generateViewLayout(Input(cfg), w)
	return w, &got
}

// imeDelete sends a deleteSurroundingText request the way the Android
// backend does: a Backspace for the text before the selection, then a
// Delete for the text after it, each carrying its count.
func imeDelete(w *Window, before, after int32, inCodePoints bool) {
	for _, d := range []struct {
		key   KeyCode
		count int32
	}{{KeyBackspace, before}, {KeyDelete, after}} {
		if d.count == 0 {
			continue
		}
		w.EventFn(&Event{
			Type:                  EventKeyDown,
			KeyCode:               d.key,
			IMEDeleteCount:        d.count,
			IMEDeleteInCodePoints: inCodePoints,
		})
	}
}

// TestIMEDeleteSurroundingInput sends the request through EventFn into a
// stock Input: the range and its unit reach the field.
func TestIMEDeleteSurroundingInput(t *testing.T) {
	cases := []struct {
		name          string
		text          string
		pos           int
		before, after int32
		inCodePoints  bool
		want          string
	}{
		// One emoji is two UTF-16 units: a per-unit Backspace would
		// also take the "A".
		{"utf16 emoji", "A😀", 2, 2, 0, false, "A"},
		{"code points", "ab😀cd", 3, 2, 1, true, "ad"},
		{"both directions", "abcdef", 3, 2, 2, true, "af"},
		// A count past the text start stops at the start.
		{"clamped", "abc", 2, 10, 0, true, "c"},
		// A UTF-16 count inside a surrogate pair takes the whole rune.
		{"half emoji", "😀b", 1, 1, 0, false, "b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, got := imeDeleteWindow(t, InputCfg{Text: tc.text}, tc.pos)
			// Both events before any rebuild, as the backend sends them:
			// the Delete must act on the text the Backspace left, not on
			// the stale layout.
			imeDelete(w, tc.before, tc.after, tc.inCodePoints)
			if *got != tc.want {
				t.Fatalf("text = %q, want %q", *got, tc.want)
			}
		})
	}
}

// TestIMEDeleteSurroundingKeepsSelection checks the request deletes next
// to the selection and leaves the selected text, as Android asks.
func TestIMEDeleteSurroundingKeepsSelection(t *testing.T) {
	w, got := imeDeleteWindow(t, InputCfg{Text: "abcdef"}, 4)
	setInputState(w, "f", inputState{CursorPos: 4, selectBeg: 2, selectEnd: 4})
	imeDelete(w, 1, 0, true)
	if *got != "acdef" {
		t.Fatalf("text = %q, want %q", *got, "acdef")
	}
}

// TestIMEDeleteSurroundingMaskFallsBack checks a masked Input takes one
// ordinary Backspace whatever the count: the mask, not the raw range,
// decides what goes. Here that is the slot and the literal before it.
func TestIMEDeleteSurroundingMaskFallsBack(t *testing.T) {
	w, got := imeDeleteWindow(t, InputCfg{Text: "12/3", Mask: "99/99"}, 4)
	imeDelete(w, 3, 0, true)
	if *got != "12" {
		t.Fatalf("masked text = %q, want %q", *got, "12")
	}
}

// TestIMEDeleteSurroundingCustomEditor checks a focused widget that is
// not an Input receives the key-downs with their count and unit, so it
// can delete as asked.
func TestIMEDeleteSurroundingCustomEditor(t *testing.T) {
	w := sizedWindow()
	w.focused = true
	var got []Event
	w.SetFocus("ed")
	w.layout = generateViewLayout(Column(ContainerCfg{
		ID:        "ed",
		Focusable: true,
		OnKeyDown: func(ctx EventCtx) {
			got = append(got, *ctx.Event)
			ctx.Consume()
		},
	}), w)
	imeDelete(w, 3, 2, true)
	if len(got) != 2 {
		t.Fatalf("got %d key-downs, want 2", len(got))
	}
	if e := got[0]; e.KeyCode != KeyBackspace || e.IMEDeleteCount != 3 ||
		!e.IMEDeleteInCodePoints {
		t.Errorf("first = %v count %d codePoints %v, want Backspace 3 true",
			e.KeyCode, e.IMEDeleteCount, e.IMEDeleteInCodePoints)
	}
	if e := got[1]; e.KeyCode != KeyDelete || e.IMEDeleteCount != 2 ||
		!e.IMEDeleteInCodePoints {
		t.Errorf("second = %v count %d codePoints %v, want Delete 2 true",
			e.KeyCode, e.IMEDeleteCount, e.IMEDeleteInCodePoints)
	}
}
