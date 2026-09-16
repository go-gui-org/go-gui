package gui

import "testing"

// Table tests for the pure edit core (issue #657). No Window: each case
// gives the text and an inputState and checks the returned text, caret,
// selection and store flag.

func TestEditInsert(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		ins      string
		is       inputState
		want     string
		wantPos  int
		wantOK   bool
		wantOpID uint8
	}{
		{"empty insert", "abc", "", inputState{CursorPos: 1}, "abc", 1, false, inputOpNone},
		{"middle", "abc", "x", inputState{CursorPos: 1}, "axbc", 2, true, inputOpInsert},
		{"negative caret appends", "abc", "x", inputState{CursorPos: -1}, "abcx", 4, true, inputOpInsert},
		{"caret past end clamps", "abc", "x", inputState{CursorPos: 9}, "abcx", 4, true, inputOpInsert},
		{"replace selection", "hello", "J", inputState{CursorPos: 1, selectBeg: 1, selectEnd: 0}, "Jello", 1, true, inputOpInsert},
		{"selection outside text", "ab", "x", inputState{CursorPos: 1, selectBeg: 2, selectEnd: 5}, "ab", 1, false, inputOpNone},
		{"multi-rune breaks run", "ab", "xy", inputState{CursorPos: 2}, "abxy", 4, true, inputOpNone},
		{"runes not bytes", "héllo", "!", inputState{CursorPos: 2}, "hé!llo", 3, true, inputOpInsert},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, is, ok := editInsert(tc.text, tc.ins, tc.is)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOK)
			}
			if is.CursorPos != tc.wantPos {
				t.Errorf("CursorPos = %d, want %d", is.CursorPos, tc.wantPos)
			}
			if got != editProposedText(tc.text, tc.ins, tc.is) {
				t.Errorf("editProposedText disagrees with editInsert")
			}
			if !ok {
				return
			}
			if is.selectBeg != is.selectEnd || is.cursorOffset != -1 || is.lastEditOp != tc.wantOpID {
				t.Errorf("state = %+v, want no selection, offset -1, op %d", is, tc.wantOpID)
			}
			if m, _ := is.Undo.Pop(); m.Text != tc.text {
				t.Errorf("undo top = %q, want %q", m.Text, tc.text)
			}
		})
	}
}

func TestEditDelete(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		is      inputState
		forward bool
		want    string
		wantPos int
		wantOK  bool
	}{
		{"backspace at start", "abc", inputState{CursorPos: 0}, false, "abc", 0, false},
		{"delete at end", "abc", inputState{CursorPos: 3}, true, "abc", 3, false},
		{"backspace", "abc", inputState{CursorPos: 2}, false, "ac", 1, true},
		{"forward delete", "abc", inputState{CursorPos: 1}, true, "ac", 1, true},
		{"negative caret is end", "abc", inputState{CursorPos: -1}, false, "ab", 2, true},
		{"selection", "hello", inputState{CursorPos: 4, selectBeg: 4, selectEnd: 1}, true, "ho", 1, true},
		{"selection outside text", "ab", inputState{selectBeg: 1, selectEnd: 7}, false, "ab", 0, false},
		// e + combining acute is one grapheme cluster: two runes, one delete.
		{"whole cluster", "aé", inputState{CursorPos: 3}, false, "a", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, is, ok := editDelete(tc.text, tc.is, tc.forward)
			if got != tc.want || ok != tc.wantOK || is.CursorPos != tc.wantPos {
				t.Fatalf("got (%q, pos %d, %v), want (%q, pos %d, %v)",
					got, is.CursorPos, ok, tc.want, tc.wantPos, tc.wantOK)
			}
			if ok && is.lastEditOp != inputOpDelete {
				t.Errorf("lastEditOp = %d, want inputOpDelete", is.lastEditOp)
			}
		})
	}
}

func TestEditUndoCoalescing(t *testing.T) {
	// A typing run is one undo step; a selection edit starts a new one.
	text, is := "", inputState{}
	for _, r := range "abc" {
		text, is, _ = editInsert(text, string(r), is)
	}
	if is.Undo.Len() != 1 {
		t.Fatalf("typing run undo depth = %d, want 1", is.Undo.Len())
	}
	is = editSelectAll(text, is)
	text, is, _ = editInsert(text, "z", is)
	if text != "z" || is.Undo.Len() != 2 {
		t.Fatalf("after select+type: %q depth %d, want \"z\" depth 2", text, is.Undo.Len())
	}

	text, is, ok := editUndo(text, is)
	if !ok || text != "abc" || is.selectBeg != 0 || is.selectEnd != 3 {
		t.Fatalf("undo 1 = %q %+v %v, want \"abc\" with selection 0..3", text, is, ok)
	}
	text, is, _ = editUndo(text, is)
	if text != "" {
		t.Fatalf("undo 2 = %q, want empty", text)
	}
	if _, _, ok = editUndo(text, is); ok {
		t.Fatal("undo on empty stack reported ok")
	}
	text, is, _ = editRedo(text, is)
	text, _, _ = editRedo(text, is)
	if text != "z" {
		t.Fatalf("redo twice = %q, want \"z\"", text)
	}
	// An edit after undo drops the redo history.
	text, is, _ = editUndo(text, is)
	if _, is, _ = editInsert(text, "q", is); is.Redo != nil {
		t.Fatal("edit after undo kept the redo stack")
	}
	if _, _, ok = editRedo(text, inputState{}); ok {
		t.Fatal("redo with no stack reported ok")
	}
}

func TestEditCutAndSelectedText(t *testing.T) {
	sel := inputState{CursorPos: 3, selectBeg: 1, selectEnd: 3}
	cases := []struct {
		name       string
		is         inputState
		password   bool
		wantText   string
		wantCopied string
		wantOK     bool
	}{
		{"selection", sel, false, "hlo", "el", true},
		{"password", sel, true, "hello", "", false},
		{"no selection", inputState{CursorPos: 2}, false, "hello", "", false},
		{"selection outside text", inputState{selectBeg: 2, selectEnd: 9}, false, "hello", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copied, ok := editSelectedText("hello", tc.is, tc.password)
			if copied != tc.wantCopied || ok != tc.wantOK {
				t.Fatalf("selected = (%q, %v), want (%q, %v)", copied, ok, tc.wantCopied, tc.wantOK)
			}
			got, cut, is, cutOK := editCut("hello", tc.is, tc.password)
			if got != tc.wantText || cut != tc.wantCopied || cutOK != tc.wantOK {
				t.Fatalf("cut = (%q, %q, %v), want (%q, %q, %v)",
					got, cut, cutOK, tc.wantText, tc.wantCopied, tc.wantOK)
			}
			if cutOK && is.CursorPos != 1 {
				t.Errorf("cut CursorPos = %d, want 1", is.CursorPos)
			}
		})
	}
}

func TestEditSelectAllKeepsHistory(t *testing.T) {
	undo := newBoundedStack[inputMemento](undoMaxSize)
	is := editSelectAll("héllo", inputState{CursorPos: 1, Undo: undo, lastEditOp: inputOpInsert})
	if is.selectBeg != 0 || is.selectEnd != 5 || is.CursorPos != 5 {
		t.Fatalf("select all = %+v, want 0..5 caret 5", is)
	}
	if is.Undo != undo || is.lastEditOp != inputOpInsert {
		t.Fatal("select all dropped the undo stack or run")
	}
}

func TestEditSetTextCursorAtEnd(t *testing.T) {
	is := editSetTextCursorAtEnd("ab", "héllo", inputState{lastEditOp: inputOpInsert})
	if is.CursorPos != 5 || is.lastEditOp != inputOpNone || is.Undo.Len() != 1 {
		t.Fatalf("state = %+v, want caret 5, op none, undo depth 1", is)
	}
}
