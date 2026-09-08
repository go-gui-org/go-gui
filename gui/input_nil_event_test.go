package gui

import (
	"strings"
	"testing"
)

// The input and combobox event handlers close over window state and
// read ctx.Event unconditionally. A synthetic dispatch with no
// originating event must decline cleanly instead of panicking.

// nilEventCtx builds a context with no event and no layout: the
// least any handler may assume.
func nilEventCtx(w *Window) EventCtx {
	return EventCtx{Layout: nil, Event: nil, Window: w}
}

func TestInputOnCharNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	makeInputOnChar(inputHandlerCfg{})(nilEventCtx(w))
}

func TestInputOnKeyDownNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	makeInputOnKeyDown(inputHandlerCfg{})(nilEventCtx(w))
}

func TestInputOnKeyUpNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	makeInputOnKeyUp(inputHandlerCfg{})(nilEventCtx(w))
}

func TestInputOnClickNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	inputOnClick("leaf", "scroll", true)(nilEventCtx(w))
}

func TestInputOnClickNilLayoutDeclines(t *testing.T) {
	w := newTestWindow()
	e := &Event{Type: EventMouseDown, MouseButton: MouseLeft}
	inputOnClick("leaf", "scroll", true)(EventCtx{nil, e, w})
}

func TestComboboxOnCharNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	makeComboboxOnChar("cb")(nilEventCtx(w))
}

func TestComboboxOnKeyDownNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	makeComboboxOnKeyDown("cb", nil, "cb", nil, "", 0, 0)(
		nilEventCtx(w))
}

// Declining leaves the event unhandled so it travels on.
func TestInputOnCharNoFocusLeavesUnhandled(t *testing.T) {
	w := newTestWindow()
	layout := generateViewLayout(Input(InputCfg{
		ID: "nil-evt", Sizing: FillFit,
	}), w)
	e := &Event{Type: EventChar, CharCode: 'x'}
	layout.Shape.events.OnChar(EventCtx{&layout, e, w})
	if e.IsHandled {
		t.Error("unfocused input must not consume the character")
	}
}

func TestUpdateCursorAndSelectionClampsNegative(t *testing.T) {
	w := newTestWindow()
	imap := StateMap[string, inputState](w, nsInput, capMany)
	updateCursorAndSelection(imap, "neg", inputState{CursorPos: -3},
		-5, true)
	got := imap.GetOr("neg", inputState{CursorPos: -99})
	if got.CursorPos != 0 {
		t.Errorf("CursorPos = %d, want 0", got.CursorPos)
	}
	if got.selectBeg != 0 || got.selectEnd != 0 {
		t.Errorf("selection = (%d, %d), want (0, 0)",
			got.selectBeg, got.selectEnd)
	}
}

func TestPreTextChangeOversizedAdjustedCapped(t *testing.T) {
	w := newTestWindow()
	big := make([]rune, inputMaxInsertRunes+1000)
	for i := range big {
		big[i] = 'x'
	}
	hcfg := inputHandlerCfg{
		preTextChange: func(_, _ string) (string, bool) {
			return string(big), true
		},
	}
	text, changed := inputTextChange(hcfg, nil, "", "a", "cap-pre", w)
	if !changed {
		t.Fatal("expected the adjusted text to apply")
	}
	if n := utf8RuneCount(text); n != inputMaxInsertRunes {
		t.Fatalf("adjusted runes = %d, want %d", n, inputMaxInsertRunes)
	}
}

func TestNormalizeOnCommitOversizedCapped(t *testing.T) {
	hcfg := inputHandlerCfg{
		postCommitNormalize: func(text string, _ InputCommitReason) string {
			return text + string(make([]rune, inputMaxInsertRunes))
		},
	}
	got := hcfg.normalizeOnCommit("hi", InputCommitEnter)
	if n := utf8RuneCount(got); n != inputMaxInsertRunes {
		t.Fatalf("normalized runes = %d, want %d", n, inputMaxInsertRunes)
	}
}

func TestCapCallbackTextBoundaries(t *testing.T) {
	if got := capCallbackText(""); got != "" {
		t.Fatalf("capCallbackText(\"\") = %q, want empty", got)
	}
	// Exactly at budget: passes through untouched, not truncated
	// and re-sliced.
	exact := strings.Repeat("x", inputMaxInsertRunes)
	if got := capCallbackText(exact); got != exact {
		t.Fatalf("at-budget text = %d runes, want %d",
			utf8RuneCount(got), inputMaxInsertRunes)
	}
	over := strings.Repeat("y", inputMaxInsertRunes+1)
	if got := capCallbackText(over); utf8RuneCount(got) != inputMaxInsertRunes {
		t.Fatalf("over-budget text = %d runes, want %d",
			utf8RuneCount(got), inputMaxInsertRunes)
	}
}
