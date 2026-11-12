package gui

import (
	"strings"
	"testing"
	"unsafe"
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
	makeInputOnChar(&inputHandlerCfg{})(nilEventCtx(w))
}

func TestInputOnKeyDownNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	makeInputOnKeyDown(&inputHandlerCfg{})(nilEventCtx(w))
}

func TestInputOnKeyUpNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	makeInputOnKeyUp(&inputHandlerCfg{})(nilEventCtx(w))
}

func TestInputOnClickNilEventDeclines(t *testing.T) {
	w := newTestWindow()
	inputOnClick("leaf", "scroll", true, nil)(nilEventCtx(w))
}

func TestInputOnClickNilLayoutDeclines(t *testing.T) {
	w := newTestWindow()
	e := &Event{Type: EventMouseDown, MouseButton: MouseLeft}
	inputOnClick("leaf", "scroll", true, nil)(EventCtx{nil, e, w})
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
		-5, true, 10)
	got := imap.GetOr("neg", inputState{CursorPos: -99})
	if got.CursorPos != 0 {
		t.Errorf("CursorPos = %d, want 0", got.CursorPos)
	}
	if got.selectBeg != 0 || got.selectEnd != 0 {
		t.Errorf("selection = (%d, %d), want (0, 0)",
			got.selectBeg, got.selectEnd)
	}
}

func TestUpdateCursorAndSelectionClampsPastEnd(t *testing.T) {
	w := newTestWindow()
	imap := StateMap[string, inputState](w, nsInput, capMany)
	updateCursorAndSelection(imap, "past", inputState{CursorPos: 999},
		999, false, 3)
	got := imap.GetOr("past", inputState{CursorPos: -99})
	if got.CursorPos != 3 {
		t.Errorf("CursorPos = %d, want 3", got.CursorPos)
	}
	if got.selectBeg != 0 || got.selectEnd != 0 {
		t.Errorf("selection = (%d, %d), want (0, 0)",
			got.selectBeg, got.selectEnd)
	}
}

func TestPreTextChangeOversizedAdjustedCapped(t *testing.T) {
	w := newTestWindow()
	hcfg := inputHandlerCfg{
		maxRunes: 100,
		preTextChange: func(_, _ string) (string, bool) {
			return strings.Repeat("x", 1100), true
		},
	}
	text, changed := inputTextChange(hcfg, nil, "", "a", "cap-pre", w)
	if !changed {
		t.Fatal("expected the adjusted text to apply")
	}
	if n := utf8RuneCount(text); n != 100 {
		t.Fatalf("adjusted runes = %d, want the field's MaxLength 100", n)
	}
}

func TestNormalizeOnCommitOversizedCapped(t *testing.T) {
	hcfg := inputHandlerCfg{
		maxRunes: 100,
		postCommitNormalize: func(text string, _ InputCommitReason) string {
			return text + strings.Repeat("y", 1000)
		},
	}
	got := hcfg.normalizeOnCommit("hi", InputCommitEnter)
	if n := utf8RuneCount(got); n != 100 {
		t.Fatalf("normalized runes = %d, want the field's MaxLength 100", n)
	}
}

func TestCapCallbackTextBoundaries(t *testing.T) {
	const budget = 1000
	if got := capCallbackText("", "", budget); got != "" {
		t.Fatalf("capCallbackText(\"\") = %q, want empty", got)
	}
	// Exactly at budget: passes through untouched, not truncated
	// and re-sliced.
	exact := strings.Repeat("x", budget)
	if got := capCallbackText("", exact, budget); got != exact {
		t.Fatalf("at-budget text = %d runes, want %d",
			utf8RuneCount(got), budget)
	}
	over := strings.Repeat("y", budget+1)
	if got := capCallbackText("", over, budget); utf8RuneCount(got) != budget {
		t.Fatalf("over-budget text = %d runes, want %d",
			utf8RuneCount(got), budget)
	}
	// Text the app set past the budget: handed back as long, it stays
	// whole; grown further, it is cut to its own length, not the budget.
	appSet := strings.Repeat("z", budget+50)
	if got := capCallbackText(appSet, appSet, budget); got != appSet {
		t.Fatalf("unchanged app-set text = %d runes, want %d",
			utf8RuneCount(got), budget+50)
	}
	if got := capCallbackText(appSet, appSet+"zz", budget); utf8RuneCount(got) != budget+50 {
		t.Fatalf("grown app-set text = %d runes, want %d",
			utf8RuneCount(got), budget+50)
	}
}

// PostCommitNormalize that keeps the text, or trims it, must not cut a
// field the app filled past its MaxLength on blur or Enter.
func TestNormalizeOnCommitKeepsAppTextPastBudget(t *testing.T) {
	hcfg := inputHandlerCfg{
		maxRunes: 100,
		postCommitNormalize: func(text string, _ InputCommitReason) string {
			return strings.TrimSpace(text)
		},
	}
	text := strings.Repeat("a", 150) + " "
	if got := hcfg.normalizeOnCommit(text, InputCommitBlur); got != text[:150] {
		t.Fatalf("normalized runes = %d, want 150", utf8RuneCount(got))
	}
}

// An unset MaxLength takes the default budget; a handler config built
// without one, as wrappers and tests build it, gets the same.
func TestInputBudgetDefault(t *testing.T) {
	if got := inputBudget(0); got != inputMaxRunesDefault {
		t.Errorf("inputBudget(0) = %d, want %d", got, inputMaxRunesDefault)
	}
	if got := inputBudget(-5); got != inputMaxRunesDefault {
		t.Errorf("inputBudget(-5) = %d, want the default", got)
	}
	if got := inputBudget(42); got != 42 {
		t.Errorf("inputBudget(42) = %d, want 42", got)
	}
	if got := (&inputHandlerCfg{}).budget(); got != inputMaxRunesDefault {
		t.Errorf("zero handler budget = %d, want the default", got)
	}
}

// A cut callback output is a copy: the field's text, its undo step and
// undoText keep the budget's bytes, not the whole of a runaway return.
func TestCapCallbackTextCopiesTheCut(t *testing.T) {
	const budget = 100
	out := strings.Repeat("y", 1<<20)
	got := capCallbackText("", out, budget)
	if len(got) != budget {
		t.Fatalf("cut output has %d bytes, want %d", len(got), budget)
	}
	if unsafe.StringData(got) == unsafe.StringData(out) {
		t.Error("cut output shares the callback's 1 MiB backing array")
	}
	kept := strings.Repeat("x", budget)
	if got := capCallbackText("", kept, budget); unsafe.StringData(got) != unsafe.StringData(kept) {
		t.Error("output within the budget was copied")
	}
}
