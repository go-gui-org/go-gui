package gui

import "testing"

// inputHasSelection returns true if text is selected.
func inputHasSelection(focusID string, w *Window) bool {
	is := StateReadOr(w, nsInput, focusID, InputState{})
	return is.SelectBeg != is.SelectEnd
}

func newTestWindow() *Window {
	return &Window{}
}

func setInputState(w *Window, focusID string, is InputState) {
	StateMap[string, InputState](w, nsInput, capMany).Set(focusID, is)
}

func getInputState(w *Window, focusID string) InputState {
	return StateReadOr(w, nsInput, focusID, InputState{})
}

// --- Insert ---

func TestInsertEmojiAtStart(t *testing.T) {
	w := newTestWindow()
	id := "f10001"
	setInputState(w, id, InputState{CursorPos: 0})
	got := inputInsert("abc", "😀", id, w)
	if got != "😀abc" {
		t.Fatalf("got %q, want %q", got, "😀abc")
	}
}

func TestInsertEmojiAtMiddle(t *testing.T) {
	w := newTestWindow()
	id := "f10002"
	setInputState(w, id, InputState{CursorPos: 1})
	got := inputInsert("ab", "😀", id, w)
	if got != "a😀b" {
		t.Fatalf("got %q, want %q", got, "a😀b")
	}
}

func TestInsertCJKString(t *testing.T) {
	w := newTestWindow()
	id := "f10003"
	setInputState(w, id, InputState{CursorPos: 0})
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
	setInputState(w, id, InputState{CursorPos: 1})
	got := inputInsert("e", "\u0301", id, w)
	if got != "e\u0301" {
		t.Fatalf("got %q, want %q", got, "e\u0301")
	}
}

func TestInsertASCIIIntoMultibyte(t *testing.T) {
	w := newTestWindow()
	id := "f10005"
	setInputState(w, id, InputState{CursorPos: 1})
	got := inputInsert("日本", "x", id, w)
	if got != "日x本" {
		t.Fatalf("got %q, want %q", got, "日x本")
	}
}

func TestInsertEmptyString(t *testing.T) {
	w := newTestWindow()
	id := "f10050"
	setInputState(w, id, InputState{CursorPos: 1})
	got := inputInsert("日本", "", id, w)
	if got != "日本" {
		t.Fatalf("got %q, want %q", got, "日本")
	}
}

// --- Delete ---

func TestBackspaceAfterEmoji(t *testing.T) {
	w := newTestWindow()
	id := "f10010"
	setInputState(w, id, InputState{CursorPos: 1})
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
	setInputState(w, id, InputState{CursorPos: 1})
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
	setInputState(w, id, InputState{CursorPos: 0})
	got, ok := inputDelete("😀x", id, true, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "x" {
		t.Fatalf("got %q, want %q", got, "x")
	}
}

func TestBackspaceCombiningChar(t *testing.T) {
	w := newTestWindow()
	id := "f10013"
	setInputState(w, id, InputState{CursorPos: 2})
	got, ok := inputDelete("e\u0301", id, false, w)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != "e" {
		t.Fatalf("got %q, want %q", got, "e")
	}
}

func TestDeleteEmptyText(t *testing.T) {
	w := newTestWindow()
	id := "f10051"
	setInputState(w, id, InputState{CursorPos: 0})
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
	setInputState(w, id, InputState{SelectBeg: 0, SelectEnd: 1})
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
	setInputState(w, id, InputState{SelectBeg: 1, SelectEnd: 3})
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
	setInputState(w, id, InputState{SelectBeg: 1, SelectEnd: 2})
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
	setInputState(w, id, InputState{CursorPos: 1, SelectBeg: 1, SelectEnd: 2})
	got := inputInsert("a😀b", "x", id, w)
	if got != "axb" {
		t.Fatalf("got %q, want %q", got, "axb")
	}
}

func TestReplaceASCIISelectionWithEmoji(t *testing.T) {
	w := newTestWindow()
	id := "f10031"
	setInputState(w, id, InputState{CursorPos: 1, SelectBeg: 1, SelectEnd: 3})
	got := inputInsert("abcd", "😀", id, w)
	if got != "a😀d" {
		t.Fatalf("got %q, want %q", got, "a😀d")
	}
}

// --- IME commit ---

func TestIMECommitCJKIntoEmpty(t *testing.T) {
	w := newTestWindow()
	id := "f10040"
	setInputState(w, id, InputState{CursorPos: 0})
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
	setInputState(w, id, InputState{CursorPos: 2})
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
	setInputState(w, id, InputState{CursorPos: 1, SelectBeg: 1, SelectEnd: 3})
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
	setInputState(w, id, InputState{CursorPos: 0})
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
	setInputState(w, id, InputState{CursorPos: 0})
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

// --- Select all ---

func TestSelectAll(t *testing.T) {
	w := newTestWindow()
	id := "f20002"
	setInputState(w, id, InputState{CursorPos: 2})
	inputSelectAll("hello", id, w)
	is := getInputState(w, id)
	assertEqual(t, int(is.SelectBeg), 0)
	assertEqual(t, int(is.SelectEnd), 5)
	assertEqual(t, is.CursorPos, 5)
}

// --- HasSelection ---

func TestHasSelection(t *testing.T) {
	w := newTestWindow()
	id := "f20003"
	setInputState(w, id, InputState{SelectBeg: 1, SelectEnd: 3})
	if !inputHasSelection(id, w) {
		t.Fatal("expected selection")
	}
	setInputState(w, id, InputState{SelectBeg: 0, SelectEnd: 0})
	if inputHasSelection(id, w) {
		t.Fatal("expected no selection")
	}
}

// --- Masked insert/delete via InputCfg ---

func TestInputCfgMaskedInsertDelete(t *testing.T) {
	w := newTestWindow()
	id := "f1001"
	setInputState(w, id, InputState{CursorPos: 0})

	// Simulate masked insert.
	pattern := InputMaskFromPreset(MaskPhoneUS)
	compiled, err := CompileInputMask(pattern, nil)
	if err != nil {
		t.Fatal(err)
	}
	is := inputStateOrDefault(id, w)
	res := InputMaskInsert("", is.CursorPos, is.SelectBeg, is.SelectEnd, "555-123-4567", &compiled)
	text := res.Text
	if text != "(555) 123-4567" {
		t.Fatalf("got %q, want %q", text, "(555) 123-4567")
	}

	// Simulate masked backspace.
	setInputState(w, id, InputState{CursorPos: res.CursorPos})
	is2 := inputStateOrDefault(id, w)
	res2 := InputMaskBackspace(text, is2.CursorPos, is2.SelectBeg, is2.SelectEnd, &compiled)
	if res2.Text != "(555) 123-456" {
		t.Fatalf("got %q, want %q", res2.Text, "(555) 123-456")
	}
}
