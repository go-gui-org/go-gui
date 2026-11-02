package gui

import "testing"

// Allocation ceilings for one keystroke edit (issue #657). Since #1187 an
// edit is a byte splice of the text plus a delta undo step: the one
// allocation is the new text, and a delete adds the removed bytes it
// keeps for undo. A []rune round trip or a full-text undo copy would
// show here first. Each run resets the
// field to the same text and caret, so the undo stack stays warm and the
// measure is one steady-state keystroke, not stack growth.
func TestInputEditAllocBudget(t *testing.T) {
	const text = "hello world"
	undo := newBoundedStack[inputMemento](undoMaxSize)
	cases := []struct {
		name string
		is   inputState
		edit func(w *Window)
		max  float64
	}{
		{"insert", inputState{CursorPos: 5, Undo: undo, undoText: text},
			func(w *Window) { _ = inputInsert(text, "x", "f", w) }, 1},
		{"insert over selection",
			inputState{CursorPos: 5, selectBeg: 0, selectEnd: 5, Undo: undo, undoText: text},
			func(w *Window) { _ = inputInsert(text, "x", "f", w) }, 1},
		{"backspace", inputState{CursorPos: 5, Undo: undo, undoText: text},
			func(w *Window) { _, _ = inputDelete(text, "f", false, w) }, 2},
		{"forward delete", inputState{CursorPos: 5, Undo: undo, undoText: text},
			func(w *Window) { _, _ = inputDelete(text, "f", true, w) }, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWindow()
			tc.edit(w) // warm the state map
			got := testing.AllocsPerRun(200, func() {
				setInputState(w, "f", tc.is)
				tc.edit(w)
			})
			t.Logf("%s allocs = %v", tc.name, got)
			if got > tc.max {
				t.Fatalf("%s allocs = %v, want <= %v", tc.name, got, tc.max)
			}
		})
	}
}
