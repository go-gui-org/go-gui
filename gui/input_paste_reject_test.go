package gui

import (
	"strings"
	"testing"
)

// Issue #1187: a paste that would leave an Input over its rune budget
// is refused whole, not cut to a prefix, and OnPasteRejected hands the
// app the paste as offered.

// pasteRejectSpy records OnPasteRejected calls.
type pasteRejectSpy struct {
	texts []string
	ctxs  []EventCtx
}

func (s *pasteRejectSpy) record(text string, ctx EventCtx) {
	s.texts = append(s.texts, text)
	s.ctxs = append(s.ctxs, ctx)
}

func TestInputPasteOverBudgetRejectedWhole(t *testing.T) {
	text := "ab"
	var spy pasteRejectSpy
	w := inputMenuTestWindow(t, InputCfg{
		ID:              "name",
		Mode:            InputMultiline,
		OnPasteRejected: spy.record,
	}, &text)
	clip := strings.Repeat("a", 92_359) // the issue's JSON size
	w.SetClipboardGetFn(func() string { return clip })
	if err := w.TestKey("name", KeyV, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if text != "ab" {
		t.Errorf("text has %d bytes, want the field left at %q", len(text), "ab")
	}
	if len(spy.texts) != 1 {
		t.Fatalf("OnPasteRejected calls = %d, want 1", len(spy.texts))
	}
	if spy.texts[0] != clip {
		t.Errorf("rejected text has %d bytes, want the whole paste (%d)",
			len(spy.texts[0]), len(clip))
	}
	ctx := spy.ctxs[0]
	if ctx.Event != nil {
		t.Error("ctx.Event set, want nil")
	}
	if ctx.Window != w {
		t.Error("ctx.Window is not the input's window")
	}
	if got := inputTextFromLayout(ctx.Layout); got != "ab" {
		t.Errorf("ctx.Layout text = %q, want the input's own layout (ab)", got)
	}
}

func TestInputPasteAtBudgetLands(t *testing.T) {
	text := ""
	var spy pasteRejectSpy
	w := inputMenuTestWindow(t, InputCfg{
		ID:              "name",
		OnPasteRejected: spy.record,
	}, &text)
	clip := strings.Repeat("x", inputMaxInsertRunes)
	w.SetClipboardGetFn(func() string { return clip })
	if err := w.TestKey("name", KeyV, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if text != clip {
		t.Errorf("text has %d bytes, want the whole %d-rune paste",
			len(text), inputMaxInsertRunes)
	}
	if len(spy.texts) != 0 {
		t.Errorf("OnPasteRejected calls = %d, want 0", len(spy.texts))
	}
}

// The budget covers the field after the paste, not the paste alone:
// accepting a fitting paste into a nearly full field would leave text
// that generation then truncates.
func TestInputPasteFits(t *testing.T) {
	full := strings.Repeat("a", inputMaxInsertRunes)
	almost := strings.Repeat("a", inputMaxInsertRunes-1)
	tests := []struct {
		name string
		text string
		clip string
		is   inputState
		want bool
	}{
		{"empty field, clip at budget", "", strings.Repeat("x", inputMaxInsertRunes), inputState{}, true},
		{"empty field, clip over budget", "", strings.Repeat("x", inputMaxInsertRunes+1), inputState{}, false},
		{"one rune of room, one rune", almost, "b", inputState{}, true},
		{"one rune of room, two runes", almost, "bc", inputState{}, false},
		{"selection frees room", full, "bcdef", inputState{selectBeg: 0, selectEnd: 5}, true},
		{"selection frees too little", full, "bcdefg", inputState{selectBeg: 5, selectEnd: 0}, false},
		{"multi-byte runes at budget", "", strings.Repeat("🎉", inputMaxInsertRunes), inputState{}, true},
		{"multi-byte runes over budget", "", strings.Repeat("🎉", inputMaxInsertRunes+1), inputState{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := inputPasteFits(tc.text, tc.clip, tc.is); got != tc.want {
				t.Errorf("inputPasteFits = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInputPasteMaskRejectReported(t *testing.T) {
	text := ""
	var spy pasteRejectSpy
	w := inputMenuTestWindow(t, InputCfg{
		ID:              "name",
		Mask:            "999",
		OnPasteRejected: spy.record,
	}, &text)
	w.SetClipboardGetFn(func() string { return "x" })
	if err := w.TestKey("name", KeyV, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if text != "" {
		t.Errorf("text = %q, want the field left empty", text)
	}
	if len(spy.texts) != 1 || spy.texts[0] != "x" {
		t.Errorf("OnPasteRejected texts = %q, want [x]", spy.texts)
	}
}

// The context menu's Paste reports a validator refusal like Ctrl+V.
func TestInputContextMenuPasteRejectReported(t *testing.T) {
	text := "ab"
	var spy pasteRejectSpy
	cfg := InputCfg{
		ID:              "name",
		PreTextChange:   func(_, _ string) (string, bool) { return "", false },
		OnPasteRejected: spy.record,
	}
	w := openMenuWith(t, cfg, &text, "zz", 2, 2)
	if err := w.TestClick(menuItemID(inputMenuPaste)); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if text != "ab" {
		t.Errorf("text = %q, want ab", text)
	}
	if len(spy.texts) != 1 || spy.texts[0] != "zz" {
		t.Fatalf("OnPasteRejected texts = %q, want [zz]", spy.texts)
	}
	if got := inputTextFromLayout(spy.ctxs[0].Layout); got != "ab" {
		t.Errorf("ctx.Layout text = %q, want the input's own layout (ab)", got)
	}
}

func TestSoundInputPasteOverBudgetErrors(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(*InputCfg) {}))
	w.SetClipboardGetFn(func() string {
		return strings.Repeat("a", inputMaxInsertRunes+1)
	})
	if err := w.TestKey("fld", KeyV, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy, SoundError)
}

// A masked paste that the mask accepts but that leaves the text as it
// was (the same digits again) is not a refusal: no reject cue, no
// OnPasteRejected, and the caret moves past it as for any paste.
func TestSoundInputMaskSamePasteNotRejected(t *testing.T) {
	tests := []struct {
		name       string
		text, clip string
		is         inputState
		wantCursor int
	}{
		{"same digits over full selection", "123", "123",
			inputState{CursorPos: 3, selectBeg: 0, selectEnd: 3}, 3},
		{"same digit at caret, field full", "111", "1",
			inputState{CursorPos: 0}, 1},
		// A mask keeps what fits and drops the rest without a cue,
		// as "4567" over "123" gives "456"; the same holds when what
		// fits equals what was there.
		{"same digits plus overflow", "123", "1234",
			inputState{CursorPos: 3, selectBeg: 0, selectEnd: 3}, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			restoreTheme(t)
			var rejected pasteRejectSpy
			w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
				c.Mask = "999"
				c.Text = tc.text
				c.OnPasteRejected = rejected.record
			}))
			w.SetClipboardGetFn(func() string { return tc.clip })
			w.SetFocus("fld")
			setInputState(w, "fld", tc.is)
			if err := w.TestKey("fld", KeyV, ModCtrl); err != nil {
				t.Fatalf("TestKey: %v", err)
			}
			wantCues(t, spy)
			if len(rejected.texts) != 0 {
				t.Errorf("OnPasteRejected texts = %q, want none", rejected.texts)
			}
			is := getInputState(w, "fld")
			if is.CursorPos != tc.wantCursor || is.selectBeg != is.selectEnd {
				t.Errorf("caret %d selection [%d,%d), want caret %d and no selection",
					is.CursorPos, is.selectBeg, is.selectEnd, tc.wantCursor)
			}
		})
	}
}

// A paste is never part of a typing run, even one the mask accepts
// without changing the text. Type "12", select all, paste "12", type
// "3": undo must step back to "12", not past the paste to "".
func TestInputMaskSamePasteBreaksTypingRun(t *testing.T) {
	text := ""
	w := inputMenuTestWindow(t, InputCfg{ID: "name", Mask: "999"}, &text)
	w.SetClipboardGetFn(func() string { return "12" })
	if err := w.TestType("name", "12"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	for _, k := range []KeyCode{KeyA, KeyV} {
		if err := w.TestKey("name", k, ModCtrl); err != nil {
			t.Fatalf("TestKey: %v", err)
		}
	}
	if err := w.TestType("name", "3"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if text != "123" {
		t.Fatalf("text = %q, want 123", text)
	}
	if err := w.TestKey("name", KeyZ, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if text != "12" {
		t.Errorf("after undo text = %q, want 12", text)
	}
}
