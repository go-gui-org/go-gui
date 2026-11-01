package gui

import (
	"strings"
	"testing"
)

// InputCfg.MaxLength (#1187): an edit that would take the field past its
// rune budget is refused whole on every path text can grow by, and text
// the app sets past the budget is shown whole and only shrinks.

func TestInputTypingAtMaxLengthRefused(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
		c.Text = "abc"
		c.MaxLength = 3
	}))
	var changed []string
	w.TestRender(inputSoundView(func(c *InputCfg) {
		c.Text = "abc"
		c.MaxLength = 3
		c.OnTextChanged = func(s string, _ EventCtx) { changed = append(changed, s) }
	}))
	if err := w.TestType("fld", "d"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("OnTextChanged fired with %q, want the field unchanged", changed)
	}
	wantCues(t, spy, SoundError)
}

// An IME commit is one insert: past the budget it is refused whole,
// never committed in part.
func TestInputIMECommitPastMaxLengthRefused(t *testing.T) {
	text := "ab"
	w := inputMenuTestWindow(t, InputCfg{ID: "name", MaxLength: 4}, &text)
	w.SetFocus("name")
	w.TestRender(nil)
	setInputState(w, "name", inputState{CursorPos: 2})
	w.EventFn(&Event{Type: EventChar, CharCode: '日', IMEText: "日本語"})
	if text != "ab" {
		t.Fatalf("text = %q after a 3-rune commit into 2 runes of room, want ab", text)
	}
	w.EventFn(&Event{Type: EventChar, CharCode: '日', IMEText: "日本"})
	if text != "ab日本" {
		t.Fatalf("text = %q after a 2-rune commit into 2 runes of room, want it whole", text)
	}
}

// A field holding app-set text past its budget takes every edit that
// does not grow it: deletes, and replacements no longer than what they
// replace.
func TestInputAppTextOverBudgetEditsShrinkOnly(t *testing.T) {
	text := "abcdefgh"
	w := inputMenuTestWindow(t, InputCfg{ID: "name", MaxLength: 5}, &text)
	render := func() { w.TestRender(nil) }

	setInputState(w, "name", inputState{CursorPos: 8})
	if err := w.TestType("name", "x"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if text != "abcdefgh" {
		t.Fatalf("typing grew an over-budget field to %q", text)
	}

	render()
	if err := w.TestKey("name", KeyBackspace, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if text != "abcdefg" {
		t.Fatalf("Backspace = %q, want abcdefg", text)
	}

	render()
	setInputState(w, "name", inputState{CursorPos: 2, selectBeg: 0, selectEnd: 2})
	if err := w.TestType("name", "z"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if text != "zcdefg" {
		t.Fatalf("shorter replacement = %q, want zcdefg", text)
	}

	render()
	setInputState(w, "name", inputState{CursorPos: 1, selectBeg: 0, selectEnd: 1})
	w.SetClipboardGetFn(func() string { return "PQ" })
	if err := w.TestKey("name", KeyV, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if text != "zcdefg" {
		t.Fatalf("a longer paste over a selection grew the field to %q", text)
	}
}

// An input's text is the accessible name of its text shape; past a
// bound the platform gets a prefix instead of the whole field on every
// tree sync. A standalone Text keeps its whole name.
func TestInputA11YLabelBounded(t *testing.T) {
	w := newTestWindow()
	big := strings.Repeat("a", inputA11YLabelRunes+100)
	field := generateViewLayout(Input(InputCfg{ID: "big", Text: big}), w)
	txt := inputTextShapeFromLayout(&field)
	if txt == nil || txt.a11Y == nil {
		t.Fatal("input text shape has no accessible name")
	}
	if n := utf8RuneCount(txt.a11Y.Label); n != inputA11YLabelRunes {
		t.Errorf("label runes = %d, want %d", n, inputA11YLabelRunes)
	}

	text := generateViewLayout(Text(TextCfg{Text: big}), w)
	if text.Shape.a11Y == nil || text.Shape.a11Y.Label != big {
		t.Error("a standalone Text must keep its whole accessible name")
	}
}

// The platform spell checker re-checks the whole text on each change;
// past spellCheckMaxBytes a field is not checked, and any result it had
// is dropped.
func TestSpellCheckSkipsLargeText(t *testing.T) {
	w := newTestWindow()
	w.SetNativePlatform(&mockSpellPlatform{})
	spellCheckTrigger("f", "helo", w)
	if sm := StateMapRead[string, spellCheckState](w, nsSpellCheck); sm == nil {
		t.Fatal("a small text must be scheduled")
	} else if _, ok := sm.Get("f"); !ok {
		t.Fatal("a small text must be scheduled")
	}

	spellCheckTrigger("f", strings.Repeat("helo ", spellCheckMaxBytes/5+1), w)
	sm := StateMapRead[string, spellCheckState](w, nsSpellCheck)
	if _, ok := sm.Get("f"); ok {
		t.Fatal("a text past spellCheckMaxBytes must not be checked")
	}
	if w.HasAnimation(spellCheckAnimID("f")) {
		t.Fatal("the pending check of the smaller text must be cancelled")
	}
}
