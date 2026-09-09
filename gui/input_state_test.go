package gui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// inputHasSelection returns true if text is selected.
func inputHasSelection(focusID string, w *Window) bool {
	is := StateReadOr(w, nsInput, focusID, inputState{})
	return is.selectBeg != is.selectEnd
}

// Mirrors NewWindow's resting state — see makeWindow in render_test.go.
func newTestWindow() *Window {
	return &Window{focused: true}
}

func setInputState(w *Window, focusID string, is inputState) {
	StateMap[string, inputState](w, nsInput, capMany).Set(focusID, is)
}

func getInputState(w *Window, focusID string) inputState {
	return StateReadOr(w, nsInput, focusID, inputState{})
}

// --- Insert ---

func TestInsertEmojiAtStart(t *testing.T) {
	w := newTestWindow()
	id := "f10001"
	setInputState(w, id, inputState{CursorPos: 0})
	got := inputInsert("abc", "😀", id, w)
	if got != "😀abc" {
		t.Fatalf("got %q, want %q", got, "😀abc")
	}
}

func TestInsertEmojiAtMiddle(t *testing.T) {
	w := newTestWindow()
	id := "f10002"
	setInputState(w, id, inputState{CursorPos: 1})
	got := inputInsert("ab", "😀", id, w)
	if got != "a😀b" {
		t.Fatalf("got %q, want %q", got, "a😀b")
	}
}

func TestInsertCJKString(t *testing.T) {
	w := newTestWindow()
	id := "f10003"
	setInputState(w, id, inputState{CursorPos: 0})
	got := inputInsert("", "日本語", id, w)
	if got != "日本語" {
		t.Fatalf("got %q, want %q", got, "日本語")
	}
	state := getInputState(w, id)
	assertEqual(t, state.CursorPos, 3)
}

func TestInsertCombiningChar(t *testing.T) {
	w := newTestWindow()
	id := "f10004"
	setInputState(w, id, inputState{CursorPos: 1})
	got := inputInsert("e", "\u0301", id, w)
	if got != "e\u0301" {
		t.Fatalf("got %q, want %q", got, "e\u0301")
	}
}

func TestInsertASCIIIntoMultibyte(t *testing.T) {
	w := newTestWindow()
	id := "f10005"
	setInputState(w, id, inputState{CursorPos: 1})
	got := inputInsert("日本", "x", id, w)
	if got != "日x本" {
		t.Fatalf("got %q, want %q", got, "日x本")
	}
}

func TestInsertEmptyString(t *testing.T) {
	w := newTestWindow()
	id := "f10050"
	setInputState(w, id, inputState{CursorPos: 1})
	got := inputInsert("日本", "", id, w)
	if got != "日本" {
		t.Fatalf("got %q, want %q", got, "日本")
	}
}

// --- Delete ---

func TestBackspaceAfterEmoji(t *testing.T) {
	w := newTestWindow()
	id := "f10010"
	setInputState(w, id, inputState{CursorPos: 1})
	got, ok := inputDelete("😀x", id, false, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "x" {
		t.Fatalf("got %q, want %q", got, "x")
	}
}

func TestBackspaceAfter3Byte(t *testing.T) {
	w := newTestWindow()
	id := "f10011"
	setInputState(w, id, inputState{CursorPos: 1})
	got, ok := inputDelete("€x", id, false, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "x" {
		t.Fatalf("got %q, want %q", got, "x")
	}
}

func TestForwardDeleteOnEmoji(t *testing.T) {
	w := newTestWindow()
	id := "f10012"
	setInputState(w, id, inputState{CursorPos: 0})
	got, ok := inputDelete("😀x", id, true, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "x" {
		t.Fatalf("got %q, want %q", got, "x")
	}
}

func TestBackspaceCombiningChar(t *testing.T) {
	// e + combining acute is one grapheme: Backspace removes the
	// whole cluster, not just the combining mark (issue #330).
	w := newTestWindow()
	id := "f10013"
	setInputState(w, id, inputState{CursorPos: 2})
	got, ok := inputDelete("e\u0301", id, false, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "" {
		t.Fatalf("got %q, want %q", got, "")
	}
}

func TestDeleteEmptyText(t *testing.T) {
	w := newTestWindow()
	id := "f10051"
	setInputState(w, id, inputState{CursorPos: 0})
	got, ok := inputDelete("", id, false, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

// --- Copy ---

func TestCopySingleMultibyteChar(t *testing.T) {
	w := newTestWindow()
	id := "f10020"
	setInputState(w, id, inputState{selectBeg: 0, selectEnd: 1})
	got, ok := inputCopy("€ab", id, false, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "€" {
		t.Fatalf("got %q, want %q", got, "€")
	}
}

func TestCopySpanAcrossMultibyte(t *testing.T) {
	w := newTestWindow()
	id := "f10021"
	setInputState(w, id, inputState{selectBeg: 1, selectEnd: 3})
	got, ok := inputCopy("a€b\u00e9", id, false, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "€b" {
		t.Fatalf("got %q, want %q", got, "€b")
	}
}

func TestCopyEmoji(t *testing.T) {
	w := newTestWindow()
	id := "f10022"
	setInputState(w, id, inputState{selectBeg: 1, selectEnd: 2})
	got, ok := inputCopy("a😀b", id, false, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "😀" {
		t.Fatalf("got %q, want %q", got, "😀")
	}
}

// --- Replace selection ---

func TestReplaceMultibyteSelectionWithASCII(t *testing.T) {
	w := newTestWindow()
	id := "f10030"
	setInputState(w, id, inputState{CursorPos: 1, selectBeg: 1, selectEnd: 2})
	got := inputInsert("a😀b", "x", id, w)
	if got != "axb" {
		t.Fatalf("got %q, want %q", got, "axb")
	}
}

func TestReplaceASCIISelectionWithEmoji(t *testing.T) {
	w := newTestWindow()
	id := "f10031"
	setInputState(w, id, inputState{CursorPos: 1, selectBeg: 1, selectEnd: 3})
	got := inputInsert("abcd", "😀", id, w)
	if got != "a😀d" {
		t.Fatalf("got %q, want %q", got, "a😀d")
	}
}

// --- IME commit ---

func TestIMECommitCJKIntoEmpty(t *testing.T) {
	w := newTestWindow()
	id := "f10040"
	setInputState(w, id, inputState{CursorPos: 0})
	got := inputInsert("", "中文", id, w)
	if got != "中文" {
		t.Fatalf("got %q, want %q", got, "中文")
	}
	state := getInputState(w, id)
	assertEqual(t, state.CursorPos, 2)
}

func TestIMECommitCJKAtCursor(t *testing.T) {
	w := newTestWindow()
	id := "f10041"
	setInputState(w, id, inputState{CursorPos: 2})
	got := inputInsert("abcd", "漢字", id, w)
	if got != "ab漢字cd" {
		t.Fatalf("got %q, want %q", got, "ab漢字cd")
	}
	state := getInputState(w, id)
	assertEqual(t, state.CursorPos, 4)
}

func TestIMECommitReplacingSelection(t *testing.T) {
	w := newTestWindow()
	id := "f10042"
	setInputState(w, id, inputState{CursorPos: 1, selectBeg: 1, selectEnd: 3})
	got := inputInsert("abcd", "日", id, w)
	if got != "a日d" {
		t.Fatalf("got %q, want %q", got, "a日d")
	}
	state := getInputState(w, id)
	assertEqual(t, state.CursorPos, 2)
}

// --- Sequential insert ---

func TestMixedScriptSequentialInsert(t *testing.T) {
	w := newTestWindow()
	id := "f10052"
	setInputState(w, id, inputState{CursorPos: 0})
	text1 := inputInsert("", "abc", id, w)
	if text1 != "abc" {
		t.Fatalf("got %q, want %q", text1, "abc")
	}
	text2 := inputInsert(text1, "日本", id, w)
	if text2 != "abc日本" {
		t.Fatalf("got %q, want %q", text2, "abc日本")
	}
	text3 := inputInsert(text2, "😀", id, w)
	if text3 != "abc日本😀" {
		t.Fatalf("got %q, want %q", text3, "abc日本😀")
	}
}

// --- Undo / Redo ---

func TestUndoRedoBasic(t *testing.T) {
	w := newTestWindow()
	id := "f20001"
	setInputState(w, id, inputState{CursorPos: 0})
	text1 := inputInsert("", "hello", id, w)
	if text1 != "hello" {
		t.Fatalf("got %q, want %q", text1, "hello")
	}
	// Undo should restore empty
	text2 := inputUndo(text1, id, w)
	if text2 != "" {
		t.Fatalf("undo: got %q, want empty", text2)
	}
	// Redo should restore "hello"
	text3 := inputRedo(text2, id, w)
	if text3 != "hello" {
		t.Fatalf("redo: got %q, want %q", text3, "hello")
	}
}

// --- Undo coalescing (issue #328) ---

// typeTypedRun types each rune of typed as its own one-rune insert
// into an empty field and returns the accumulated text.
func typeTypedRun(t *testing.T, typed string, id string, w *Window) string {
	t.Helper()
	text := ""
	for _, ch := range typed {
		text = inputInsert(text, string(ch), id, w)
	}
	return text
}

// Consecutive single-rune inserts must coalesce: typing "hello" then
// one Ctrl+Z restores "".
func TestUndoCoalescesTypingRun(t *testing.T) {
	w := newTestWindow()
	id := "f30001"
	setInputState(w, id, inputState{CursorPos: 0})
	text := typeTypedRun(t, "hello", id, w)
	if got := inputUndo(text, id, w); got != "" {
		t.Fatalf("one undo: got %q, want empty", got)
	}
}

// Caret motion breaks the run: typing "x" after moving the caret is
// its own undo step, and undoing twice restores "".
func TestUndoBreaksRunOnCaretMotion(t *testing.T) {
	w := newTestWindow()
	id := "f30002"
	setInputState(w, id, inputState{CursorPos: 0})
	text := typeTypedRun(t, "hello", id, w)
	imap := StateMap[string, inputState](w, nsInput, capMany)
	updateCursorAndSelection(imap, id, inputStateOrDefault(id, w), 1, false)
	text = inputInsert(text, "x", id, w)
	undo1 := inputUndo(text, id, w)
	if undo1 != "hello" {
		t.Fatalf("undo 1: got %q, want %q", undo1, "hello")
	}
	undo2 := inputUndo(undo1, id, w)
	if undo2 != "" {
		t.Fatalf("undo 2: got %q, want empty", undo2)
	}
}

// A selection-replacing edit pushes its own anchor: select all, type
// "x", one Ctrl+Z restores "hello" in full.
func TestUndoSelectionReplaceIsOneStep(t *testing.T) {
	w := newTestWindow()
	id := "f30003"
	setInputState(w, id, inputState{CursorPos: 0})
	text := typeTypedRun(t, "hello", id, w)
	inputSelectAll(text, id, w)
	text = inputInsert(text, "x", id, w)
	if text != "x" {
		t.Fatalf("replace: got %q, want %q", text, "x")
	}
	if got := inputUndo(text, id, w); got != "hello" {
		t.Fatalf("undo: got %q, want %q", got, "hello")
	}
}

// Masked typing coalesces like plain typing: three digits into a
// phone mask, one Ctrl+Z restores the empty field (issue #328).
func TestUndoCoalescesMaskedTypingRun(t *testing.T) {
	w := newTestWindow()
	id := "f30008"
	setInputState(w, id, inputState{CursorPos: 0})
	compiled, err := compileInputMask(inputMaskFromPreset(MaskPhoneUS), nil)
	if err != nil {
		t.Fatal(err)
	}
	hcfg := inputHandlerCfg{CompiledMask: &compiled}
	text := ""
	for _, ch := range "555" {
		text, _ = inputTextChange(hcfg, nil, text, string(ch), id, w)
	}
	if got := inputUndo(text, id, w); got != "" {
		t.Fatalf("one undo: got %q, want empty", got)
	}
}

// Masked deletes coalesce too: two masked backspaces undo as one step
// back to the pre-delete text (issue #328).
func TestUndoCoalescesMaskedDeleteRun(t *testing.T) {
	w := newTestWindow()
	id := "f30009"
	setInputState(w, id, inputState{CursorPos: 0})
	compiled, err := compileInputMask(inputMaskFromPreset(MaskPhoneUS), nil)
	if err != nil {
		t.Fatal(err)
	}
	hcfg := inputHandlerCfg{CompiledMask: &compiled}
	text := ""
	for _, ch := range "555" {
		text, _ = inputTextChange(hcfg, nil, text, string(ch), id, w)
	}
	pre := text
	text, _ = inputHandleDelete(text, id, false, &compiled, nil, w)
	text, _ = inputHandleDelete(text, id, false, &compiled, nil, w)
	if text == pre {
		t.Fatal("masked deletes changed nothing")
	}
	if got := inputUndo(text, id, w); got != pre {
		t.Fatalf("one undo: got %q, want %q", got, pre)
	}
}

// A multi-rune IME commit through the masked path breaks the run:
// committing "55" then typing "5" takes two Ctrl+Z steps (issue
// #328).
func TestUndoIMEBreaksMaskedRun(t *testing.T) {
	w := newTestWindow()
	id := "f30010"
	setInputState(w, id, inputState{CursorPos: 0})
	compiled, err := compileInputMask(inputMaskFromPreset(MaskPhoneUS), nil)
	if err != nil {
		t.Fatal(err)
	}
	hcfg := inputHandlerCfg{CompiledMask: &compiled}
	text, _ := inputTextChange(hcfg, nil, "", "55", id, w)
	text, _ = inputTextChange(hcfg, nil, text, "5", id, w)
	undo1 := inputUndo(text, id, w)
	if undo1 == "" {
		t.Fatal("undo 1 must restore pre-IME text, not the whole run")
	}
	if got := inputUndo(undo1, id, w); got != "" {
		t.Fatalf("undo 2: got %q, want empty", got)
	}
}

// A programmatic text set breaks the run: typing after a
// PreTextChange-adjusted set undoes back to the set text, not to the
// pre-run state (issue #328).
func TestUndoProgrammaticSetBreaksRun(t *testing.T) {
	w := newTestWindow()
	id := "f30011"
	setInputState(w, id, inputState{CursorPos: 0})
	text := typeTypedRun(t, "hello", id, w)
	inputSetTextAndCursorAtEnd(text, "HELLO", id, w)
	text = inputInsert("HELLO", "!", id, w)
	undo1 := inputUndo(text, id, w)
	if undo1 != "HELLO" {
		t.Fatalf("undo 1: got %q, want %q", undo1, "HELLO")
	}
	undo2 := inputUndo(undo1, id, w)
	if undo2 != "hello" {
		t.Fatalf("undo 2: got %q, want %q", undo2, "hello")
	}
}

// A multi-rune insert (paste) breaks the run: pasting "abc" then
// typing "d" takes two Ctrl+Z steps to get back to "".
func TestUndoPasteBreaksRun(t *testing.T) {
	w := newTestWindow()
	id := "f30004"
	setInputState(w, id, inputState{CursorPos: 0})
	text := inputInsert("", "abc", id, w)
	text = inputInsert(text, "d", id, w)
	undo1 := inputUndo(text, id, w)
	if undo1 != "abc" {
		t.Fatalf("undo 1: got %q, want %q", undo1, "abc")
	}
	undo2 := inputUndo(undo1, id, w)
	if undo2 != "" {
		t.Fatalf("undo 2: got %q, want empty", undo2)
	}
}

// A delete after an insert run is a different edit kind and starts
// its own step: type "ab", backspace, undo restores "ab" then "".
func TestUndoDeleteBreaksInsertRun(t *testing.T) {
	w := newTestWindow()
	id := "f30005"
	setInputState(w, id, inputState{CursorPos: 0})
	text := typeTypedRun(t, "ab", id, w)
	text, _ = inputDelete(text, id, false, w)
	if text != "a" {
		t.Fatalf("delete: got %q, want %q", text, "a")
	}
	undo1 := inputUndo(text, id, w)
	if undo1 != "ab" {
		t.Fatalf("undo 1: got %q, want %q", undo1, "ab")
	}
	undo2 := inputUndo(undo1, id, w)
	if undo2 != "" {
		t.Fatalf("undo 2: got %q, want empty", undo2)
	}
}

// Consecutive deletes coalesce too: three backspaces undo as one.
func TestUndoCoalescesDeleteRun(t *testing.T) {
	w := newTestWindow()
	id := "f30006"
	setInputState(w, id, inputState{CursorPos: 0})
	text := typeTypedRun(t, "abcdef", id, w)
	text, _ = inputDelete(text, id, false, w)
	text, _ = inputDelete(text, id, false, w)
	text, _ = inputDelete(text, id, false, w)
	if text != "abc" {
		t.Fatalf("delete: got %q, want %q", text, "abc")
	}
	undo1 := inputUndo(text, id, w)
	if undo1 != "abcdef" {
		t.Fatalf("undo: got %q, want %q", undo1, "abcdef")
	}
}

// A 60-character run is one undo entry, so previously pushed history
// survives instead of being evicted by the 50-entry cap.
func TestUndoLongRunDoesNotEvictHistory(t *testing.T) {
	w := newTestWindow()
	id := "f30007"
	setInputState(w, id, inputState{CursorPos: 0})
	text := inputInsert("", "seed", id, w)
	text = inputInsert(text, "seed2", id, w)
	for range 60 {
		text = inputInsert(text, "x", id, w)
	}
	undo1 := inputUndo(text, id, w)
	if undo1 != "seedseed2" {
		t.Fatalf("undo 1: got %q, want %q", undo1, "seedseed2")
	}
	undo2 := inputUndo(undo1, id, w)
	if undo2 != "seed" {
		t.Fatalf("undo 2: got %q, want %q", undo2, "seed")
	}
	undo3 := inputUndo(undo2, id, w)
	if undo3 != "" {
		t.Fatalf("undo 3: got %q, want empty", undo3)
	}
}

// --- Select all ---

func TestSelectAll(t *testing.T) {
	w := newTestWindow()
	id := "f20002"
	setInputState(w, id, inputState{CursorPos: 2})
	inputSelectAll("hello", id, w)
	is := getInputState(w, id)
	assertEqual(t, int(is.selectBeg), 0)
	assertEqual(t, int(is.selectEnd), 5)
	assertEqual(t, is.CursorPos, 5)
}

// --- HasSelection ---

func TestHasSelection(t *testing.T) {
	w := newTestWindow()
	id := "f20003"
	setInputState(w, id, inputState{selectBeg: 1, selectEnd: 3})
	if !inputHasSelection(id, w) {
		t.Fatal("expected selection")
	}
	setInputState(w, id, inputState{selectBeg: 0, selectEnd: 0})
	if inputHasSelection(id, w) {
		t.Fatal("expected no selection")
	}
}

// --- Masked insert/delete via InputCfg ---

func TestInputCfgMaskedInsertDelete(t *testing.T) {
	w := newTestWindow()
	id := "f1001"
	setInputState(w, id, inputState{CursorPos: 0})

	// Simulate masked insert.
	pattern := inputMaskFromPreset(MaskPhoneUS)
	compiled, err := compileInputMask(pattern, nil)
	if err != nil {
		t.Fatal(err)
	}
	is := inputStateOrDefault(id, w)
	res := inputMaskInsert("", is.CursorPos, is.selectBeg, is.selectEnd, "555-123-4567", &compiled)
	text := res.Text
	if text != "(555) 123-4567" {
		t.Fatalf("got %q, want %q", text, "(555) 123-4567")
	}

	// Simulate masked backspace.
	setInputState(w, id, inputState{CursorPos: res.CursorPos})
	is2 := inputStateOrDefault(id, w)
	res2 := inputMaskBackspace(text, is2.CursorPos, is2.selectBeg, is2.selectEnd, &compiled)
	if res2.Text != "(555) 123-456" {
		t.Fatalf("got %q, want %q", res2.Text, "(555) 123-456")
	}
}

// --- proposed text (no state mutation) ---

func TestInputProposedText(t *testing.T) {
	w := newTestWindow()
	id := "f-prop"
	setInputState(w, id, inputState{CursorPos: 2})

	if got := inputProposedText("hello", "X", id, w); got != "heXllo" {
		t.Fatalf("insert at cursor = %q, want heXllo", got)
	}
	// Empty insert is a passthrough.
	if got := inputProposedText("hello", "", id, w); got != "hello" {
		t.Fatalf("empty insert = %q, want hello", got)
	}
	// State must be untouched by a propose.
	if is := getInputState(w, id); is.CursorPos != 2 {
		t.Fatalf("state cursor = %d, want 2 (unchanged)", is.CursorPos)
	}
}

func TestInputProposedTextReplacesSelection(t *testing.T) {
	w := newTestWindow()
	id := "f-prop-sel"
	setInputState(w, id, inputState{CursorPos: 3, selectBeg: 1, selectEnd: 3})

	if got := inputProposedText("hello", "XX", id, w); got != "hXXlo" {
		t.Fatalf("insert over selection = %q, want hXXlo", got)
	}
}

func TestInputProposedTextClampsHugeInsert(t *testing.T) {
	w := newTestWindow()
	id := "f-prop-big"
	setInputState(w, id, inputState{CursorPos: 0})

	big := make([]rune, inputMaxInsertRunes+1000)
	for i := range big {
		big[i] = 'x'
	}
	got := inputProposedText("", string(big), id, w)
	if utf8RuneCount(got) != inputMaxInsertRunes {
		t.Fatalf("proposed rune count = %d, want %d (clamped)",
			utf8RuneCount(got), inputMaxInsertRunes)
	}
}

func TestInputSetTextAndCursorAtEnd(t *testing.T) {
	w := newTestWindow()
	id := "f-setend"
	setInputState(w, id, inputState{CursorPos: 0})

	inputSetTextAndCursorAtEnd("old", "新しい", id, w)
	is := getInputState(w, id)
	if is.CursorPos != utf8RuneCount("新しい") {
		t.Fatalf("cursor = %d, want %d (end of new text)",
			is.CursorPos, utf8RuneCount("新しい"))
	}
	if is.Undo == nil {
		t.Fatal("undo stack must be pushed with the old text")
	}
}

func TestTruncateToMaxRunes(t *testing.T) {
	if got := truncateToMaxRunes(""); got != "" {
		t.Errorf("empty = %q, want empty", got)
	}
	if got := truncateToMaxRunes("hello"); got != "hello" {
		t.Errorf("short = %q, want unchanged", got)
	}
	exact := strings.Repeat("x", inputMaxInsertRunes)
	if got := truncateToMaxRunes(exact); got != exact {
		t.Error("exact-budget input must pass through untouched")
	}
	over := strings.Repeat("y", inputMaxInsertRunes+1)
	if got := truncateToMaxRunes(over); utf8RuneCount(got) != inputMaxInsertRunes {
		t.Errorf("over-budget runes = %d, want %d",
			utf8RuneCount(got), inputMaxInsertRunes)
	}
	multi := strings.Repeat("é", inputMaxInsertRunes+10)
	got := truncateToMaxRunes(multi)
	if utf8RuneCount(got) != inputMaxInsertRunes || !utf8.ValidString(got) {
		t.Error("multibyte truncation must keep the budget on a rune boundary")
	}
	big := strings.Repeat("z", inputMaxInsertRunes*4)
	if got := truncateToMaxRunes(big); utf8RuneCount(got) != inputMaxInsertRunes {
		t.Errorf("large runes = %d, want %d",
			utf8RuneCount(got), inputMaxInsertRunes)
	}
	invalid := strings.Repeat("a", inputMaxInsertRunes) + "\xff\xfe"
	got = truncateToMaxRunes(invalid)
	if utf8RuneCount(got) != inputMaxInsertRunes {
		t.Errorf("invalid-tail runes = %d, want %d (bad bytes cap cleanly)",
			utf8RuneCount(got), inputMaxInsertRunes)
	}
}

func TestIsMultiRuneInsert(t *testing.T) {
	if isMultiRuneInsert("") {
		t.Error("empty must be single")
	}
	if isMultiRuneInsert("a") {
		t.Error("one ASCII rune must be single")
	}
	if isMultiRuneInsert("é") {
		t.Error("one multibyte rune must be single")
	}
	if !isMultiRuneInsert("ab") {
		t.Error("two runes must be multi")
	}
	if !isMultiRuneInsert("aé") {
		t.Error("mixed-width pair must be multi")
	}
}

func TestMoveCursorUpDownMultibyte(t *testing.T) {
	// Regression: the column must be counted in runes. With a byte
	// column, a 4-byte rune on the destination line made Up answer an
	// index still on the SOURCE line (the caret never moved up) and
	// made Down answer an index past the end of the text. Both wrong
	// answers land on ordinary grapheme stops, so the caller's
	// closestGraphemeStop snap could not repair either one.

	// Line 0 is one 4-byte rune; line 1 is ASCII. Runes: 0=emoji,
	// 1='\n', 2..7='a'..'f'. Line 0 ends at rune 1.
	up := "\U0001F389\nabcdef"
	assertEqual(t, utf8RuneCount(up), 8)
	// Column 0 of line 1 -> start of line 0.
	assertEqual(t, moveCursorUp(up, 2), 0)
	// Columns past the end of line 0 clamp to its end, never to a
	// position back on line 1.
	assertEqual(t, moveCursorUp(up, 4), 1)
	assertEqual(t, moveCursorUp(up, 7), 1)

	// Line 1 is one 4-byte rune. Runes: 0..5='a'..'f', 6='\n', 7=emoji.
	down := "abcdef\n\U0001F389"
	assertEqual(t, utf8RuneCount(down), 8)
	assertEqual(t, moveCursorDown(down, 0), 7)
	// Column 2 exceeds line 1's single rune, so it clamps to the end
	// of the text (8), not past it.
	assertEqual(t, moveCursorDown(down, 2), 8)
	assertEqual(t, moveCursorDown(down, 5), 8)

	// Moving right on the source line must not move the target left.
	// A byte column made Up(9) answer 1 while Up(8) answered 2.
	cjk := "\u6f22\u5b57\u30c6\u30b9\u30c8\nabcdefghijklmno"
	assertEqual(t, moveCursorUp(cjk, 8), 2)
	assertEqual(t, moveCursorUp(cjk, 9), 3)

	// Line start/end stay exact across multibyte lines.
	assertEqual(t, moveCursorLineStart(cjk, 8), 6)
	assertEqual(t, moveCursorLineEnd(cjk, 2), 5)

	// Every result is a legal cursor position.
	for _, s := range []string{up, down, cjk} {
		n := utf8RuneCount(s)
		for pos := 0; pos <= n; pos++ {
			for _, got := range []int{
				moveCursorUp(s, pos), moveCursorDown(s, pos),
				moveCursorLineStart(s, pos), moveCursorLineEnd(s, pos),
			} {
				if got < 0 || got > n {
					t.Fatalf("%q pos %d: got %d, out of range 0..%d",
						s, pos, got, n)
				}
			}
		}
	}
}

func TestTextViewTruncatesOverlongProgrammaticText(t *testing.T) {
	buf := captureDebugMask(t, DebugGlyphLayoutFallback)
	w := caretTestWindow(400, 60)
	big := strings.Repeat("a", inputMaxInsertRunes+100)
	_, field := arrangeInput(t, w, InputCfg{ID: "big", Text: big})
	if got := utf8RuneCount(inputTextFromLayout(field)); got != inputMaxInsertRunes {
		t.Errorf("input text runes = %d, want %d (clamped)",
			got, inputMaxInsertRunes)
	}
	if buf.Len() == 0 {
		t.Error("expected a truncation finding under DebugGlyphLayoutFallback")
	}
}

func TestStandaloneTextStaysUnbounded(t *testing.T) {
	buf := captureDebugMask(t, DebugGlyphLayoutFallback)
	w := newTestWindow()
	big := strings.Repeat("a", inputMaxInsertRunes+100)
	root := generateViewLayout(Column(ContainerCfg{
		Content: []View{Text(TextCfg{ID: "big", Text: big})},
	}), w)
	ly := findLayoutByID(&root, "big")
	if ly == nil {
		t.Fatal("no layout with ID \"big\" after generate")
	}
	if got := utf8RuneCount(ly.Shape.TC.Text); got != inputMaxInsertRunes+100 {
		t.Errorf("display text runes = %d, want untouched %d",
			got, inputMaxInsertRunes+100)
	}
	if buf.Len() != 0 {
		t.Errorf("display text must stay silent, got %q", buf.String())
	}
}
