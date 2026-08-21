package gui

import "github.com/go-gui-org/go-glyph"

const inputMaxInsertRunes = 65_536

// InputState manages cursor, selection, and undo/redo for
// an input field. Stored in StateRegistry keyed by ID.
type inputState struct {
	Undo           *BoundedStack[inputMemento]
	Redo           *BoundedStack[inputMemento]
	CursorPos      int
	LastClickTime  int64
	selectBeg      uint32
	selectEnd      uint32
	cursorOffset   float32
	cursorTrailing bool  // prefer end-of-previous-line at wrap boundaries
	lastEditOp     uint8 // kind of edit the current undo run holds (issue #328)
}

// InputMemento stores a snapshot for undo/redo.
type inputMemento struct {
	Text         string
	CursorPos    int
	selectBeg    uint32
	selectEnd    uint32
	cursorOffset float32
}

// InputMode selects single-line or multiline behavior.
type inputMode uint8

// InputMode constants.
const (
	inputSingleLine inputMode = iota
	InputMultiline
)

// InputCommitReason identifies why text was committed.
type InputCommitReason uint8

// InputCommitReason constants. Exported because the reason is the
// second parameter of OnTextCommit: without them an app can receive
// the value but cannot branch on it, which is the whole point of
// distinguishing "the user pressed Enter" from "focus moved on".
const (
	// InputCommitEnter is the user finalizing the text: Enter on a
	// single-line field, or an IME finalize.
	// exportaudit:keep — OnTextCommit parameter value for app authors
	InputCommitEnter InputCommitReason = iota
	// InputCommitBlur is the field losing focus with the text as it
	// stands. Not an edit and not necessarily an intent to submit.
	// exportaudit:keep — OnTextCommit parameter value for app authors
	InputCommitBlur
)

const undoMaxSize = 50

// Undo run kinds for inputState.lastEditOp. Consecutive edits of the
// same kind coalesce into one undo step; inputOpNone breaks the run
// and always pushes, so caret motion, paste, and programmatic text
// sets each start a fresh undo step.
const (
	inputOpNone uint8 = iota
	inputOpInsert
	inputOpDelete
)

func inputStateOrDefault(focusID string, w *Window) inputState {
	m := StateMap[string, inputState](w, nsInput, capMany)
	if v, ok := m.Get(focusID); ok {
		return v
	}
	return inputState{}
}

func inputMementoFromState(text string, is inputState) inputMemento {
	return inputMemento{
		Text:         text,
		CursorPos:    is.CursorPos,
		selectBeg:    is.selectBeg,
		selectEnd:    is.selectEnd,
		cursorOffset: is.cursorOffset,
	}
}

// inputPushUndo records a text edit for undo, coalescing it with the
// previous edit when both are the same kind (insert or delete) and no
// selection was involved, so a typing run undoes as one step instead
// of one per character (issue #328). The pre-run state stays at the
// top of the stack for the whole run; the caller stores the op it
// passed as the new lastEditOp.
func inputPushUndo(is inputState, text string, op uint8) *BoundedStack[inputMemento] {
	stack := is.Undo
	if stack == nil {
		stack = newBoundedStack[inputMemento](undoMaxSize)
	}
	if op == inputOpNone || op != is.lastEditOp || is.selectBeg != is.selectEnd {
		stack.Push(inputMementoFromState(text, is))
	}
	return stack
}

func inputStateFromMemento(m inputMemento, undo, redo *BoundedStack[inputMemento]) inputState {
	return inputState{
		CursorPos:    m.CursorPos,
		selectBeg:    m.selectBeg,
		selectEnd:    m.selectEnd,
		cursorOffset: m.cursorOffset,
		Undo:         undo,
		Redo:         redo,
	}
}

// inputProposedText returns the text that would result from
// inserting insertText at the cursor without modifying state.
func inputProposedText(text, insertText string, focusID string, w *Window) string {
	if len(insertText) == 0 {
		return text
	}
	insertRunes := []rune(insertText)
	if len(insertRunes) > inputMaxInsertRunes {
		insertRunes = insertRunes[:inputMaxInsertRunes]
	}
	runes := []rune(text)
	is := inputStateOrDefault(focusID, w)
	cursorPos := min(is.CursorPos, len(runes))
	if cursorPos < 0 {
		return text + string(insertRunes)
	}
	if is.selectBeg != is.selectEnd {
		beg, end := u32Sort(is.selectBeg, is.selectEnd)
		if int(beg) >= len(runes) || int(end) > len(runes) {
			return text
		}
		result := make([]rune, 0, int(beg)+len(insertRunes)+(len(runes)-int(end)))
		result = append(result, runes[:beg]...)
		result = append(result, insertRunes...)
		result = append(result, runes[end:]...)
		return string(result)
	}
	result := make([]rune, 0, len(runes)+len(insertRunes))
	result = append(result, runes[:cursorPos]...)
	result = append(result, insertRunes...)
	result = append(result, runes[cursorPos:]...)
	return string(result)
}

// inputInsert inserts text at cursor or replaces selection.
// Returns resulting text.
func inputInsert(text string, insertText string, focusID string, w *Window) string {
	if len(insertText) == 0 {
		return text
	}
	insertRunes := []rune(insertText)
	if len(insertRunes) > inputMaxInsertRunes {
		insertRunes = insertRunes[:inputMaxInsertRunes]
	}

	runes := []rune(text)
	is := inputStateOrDefault(focusID, w)
	cursorPos := min(is.CursorPos, len(runes))
	if cursorPos < 0 {
		runes = append([]rune(text), insertRunes...)
		cursorPos = len(runes)
	} else if is.selectBeg != is.selectEnd {
		beg, end := u32Sort(is.selectBeg, is.selectEnd)
		if int(beg) >= len(runes) || int(end) > len(runes) {
			return text
		}
		result := make([]rune, 0, int(beg)+len(insertRunes)+(len(runes)-int(end)))
		result = append(result, runes[:beg]...)
		result = append(result, insertRunes...)
		result = append(result, runes[end:]...)
		runes = result
		cursorPos = min(int(beg)+len(insertRunes), len(runes))
	} else {
		result := make([]rune, 0, cursorPos+len(insertRunes)+(len(runes)-cursorPos))
		result = append(result, runes[:cursorPos]...)
		result = append(result, insertRunes...)
		result = append(result, runes[cursorPos:]...)
		runes = result
		cursorPos = min(cursorPos+len(insertRunes), len(runes))
	}

	nextText := string(runes)
	op := inputOpInsert
	if len(insertRunes) > 1 {
		// Multi-rune inserts (paste, IME commits) are one undo step
		// even after a typing run, so they break the run.
		op = inputOpNone
	}
	undo := inputPushUndo(is, text, op)
	imap := StateMap[string, inputState](w, nsInput, capMany)
	imap.Set(focusID, inputState{
		CursorPos:    cursorPos,
		cursorOffset: -1,
		Undo:         undo,
		lastEditOp:   op,
	})
	return nextText
}

// inputSetTextAndCursorAtEnd pushes undo and places cursor at end
// of newText. Used when PreTextChange returns adjusted text where
// positional cursor mapping is unreliable.
func inputSetTextAndCursorAtEnd(oldText, newText string, focusID string, w *Window) {
	is := inputStateOrDefault(focusID, w)
	// A programmatic text set is never part of a typing run.
	undo := inputPushUndo(is, oldText, inputOpNone)
	imap := StateMap[string, inputState](w, nsInput, capMany)
	imap.Set(focusID, inputState{
		CursorPos:    utf8RuneCount(newText),
		cursorOffset: -1,
		Undo:         undo,
	})
}

// inputDelete removes text at cursor or selected range. A plain
// (unselected) delete removes one whole grapheme cluster — the same
// granularity the glyph-backed path gives — so the nil-measurer
// fallback never splits an emoji or combining sequence.
// forwardDelete=true for Delete key, false for Backspace.
func inputDelete(text string, focusID string, forwardDelete bool, w *Window) (string, bool) {
	runes := []rune(text)
	is := inputStateOrDefault(focusID, w)
	cursorPos := min(is.CursorPos, len(runes))
	if cursorPos < 0 {
		cursorPos = len(runes)
	}

	if is.selectBeg != is.selectEnd {
		beg, end := u32Sort(is.selectBeg, is.selectEnd)
		if int(beg) >= len(runes) || int(end) > len(runes) {
			return text, false
		}
		result := make([]rune, 0, int(beg)+(len(runes)-int(end)))
		result = append(result, runes[:beg]...)
		result = append(result, runes[end:]...)
		runes = result
		cursorPos = min(int(beg), len(runes))
	} else {
		if cursorPos == 0 && !forwardDelete {
			return text, true
		}
		if cursorPos == len(runes) && forwardDelete {
			return text, true
		}
		// No glyph layout behind this path (nil textMeasurer), so
		// cluster boundaries are recomputed with UAX #29 — the same
		// segmentation shaping produces — keeping Backspace/Delete
		// whole-cluster in the fallback as well.
		stops := graphemeStops(text)
		delPos, delEnd := cursorPos, cursorPos
		if !forwardDelete {
			delPos = prevGraphemeStop(stops, cursorPos)
		} else {
			delEnd = nextGraphemeStop(stops, cursorPos)
		}
		if delPos < 0 || delPos >= len(runes) || delEnd > len(runes) {
			return text, false
		}
		result := make([]rune, 0, len(runes)-(delEnd-delPos))
		result = append(result, runes[:delPos]...)
		result = append(result, runes[delEnd:]...)
		runes = result
		if !forwardDelete {
			cursorPos = delPos
		}
	}

	nextText := string(runes)
	undo := inputPushUndo(is, text, inputOpDelete)
	imap := StateMap[string, inputState](w, nsInput, capMany)
	imap.Set(focusID, inputState{
		CursorPos:    cursorPos,
		cursorOffset: -1,
		Undo:         undo,
		lastEditOp:   inputOpDelete,
	})
	return nextText, true
}

// inputCopy returns the selected text. Returns ("", false) if
// no selection or password mode.
func inputCopy(text string, focusID string, isPassword bool, w *Window) (string, bool) {
	if isPassword {
		return "", false
	}
	is := StateReadOr(w, nsInput, focusID, inputState{})
	if is.selectBeg == is.selectEnd {
		return "", false
	}
	beg, end := u32Sort(is.selectBeg, is.selectEnd)
	runeCount := utf8RuneCount(text)
	if int(beg) > runeCount || int(end) > runeCount || beg >= end {
		return "", false
	}
	begByte := runeToByteIndex(text, int(beg))
	endByte := runeToByteIndex(text, int(end))
	return text[begByte:endByte], true
}

// inputCut copies selected text then deletes it.
func inputCut(text string, focusID string, isPassword bool, w *Window) (string, string, bool) {
	if isPassword {
		return text, "", false
	}
	copied, ok := inputCopy(text, focusID, false, w)
	if !ok {
		return text, "", false
	}
	newText, _ := inputDelete(text, focusID, false, w)
	return newText, copied, true
}

// inputUndo reverts to previous state. Returns restored text.
func inputUndo(text string, focusID string, w *Window) string {
	imap := StateMap[string, inputState](w, nsInput, capMany)
	// Default InputState{}: zero value means no undo state exists.
	is := imap.GetOr(focusID, inputState{})
	if is.Undo == nil || is.Undo.isEmpty() {
		return text
	}
	memento, ok := is.Undo.Pop()
	if !ok {
		return text
	}
	redo := is.Redo
	if redo == nil {
		redo = newBoundedStack[inputMemento](undoMaxSize)
	}
	redo.Push(inputMementoFromState(text, is))
	imap.Set(focusID, inputStateFromMemento(memento, is.Undo, redo))
	return memento.Text
}

// inputRedo reapplies a previously undone operation.
func inputRedo(text string, focusID string, w *Window) string {
	imap := StateMap[string, inputState](w, nsInput, capMany)
	// Default InputState{}: zero value means no redo state exists.
	is := imap.GetOr(focusID, inputState{})
	if is.Redo == nil || is.Redo.isEmpty() {
		return text
	}
	memento, ok := is.Redo.Pop()
	if !ok {
		return text
	}
	undo := is.Undo
	if undo == nil {
		undo = newBoundedStack[inputMemento](undoMaxSize)
	}
	undo.Push(inputMementoFromState(text, is))
	imap.Set(focusID, inputStateFromMemento(memento, undo, is.Redo))
	return memento.Text
}

// inputSelectAll selects all text.
func inputSelectAll(text string, focusID string, w *Window) {
	runeCount := utf8RuneCount(text)
	imap := StateMap[string, inputState](w, nsInput, capMany)
	// Default InputState{}: zero value seeds initial select-all state.
	is := imap.GetOr(focusID, inputState{})
	is.selectBeg = 0
	is.selectEnd = uint32(runeCount)
	is.CursorPos = runeCount
	imap.Set(focusID, is)
}

// updateCursorAndSelection moves cursor to newPos, extending
// or resetting selection based on shift modifier.
func updateCursorAndSelection(
	imap *BoundedMap[string, inputState],
	focusID string,
	is inputState,
	newPos int,
	isShift bool,
) {
	if isShift {
		if is.selectBeg == is.selectEnd {
			// Start new selection from current cursor.
			is.selectBeg = uint32(is.CursorPos)
			is.selectEnd = uint32(newPos)
		} else {
			// Extend: move the end that matches current cursor.
			if uint32(is.CursorPos) == is.selectEnd {
				is.selectEnd = uint32(newPos)
			} else {
				is.selectBeg = uint32(newPos)
			}
		}
	} else {
		is.selectBeg = 0
		is.selectEnd = 0
	}
	is.CursorPos = newPos
	// Any caret motion breaks the current undo run, so edits after
	// navigation form their own undo step (issue #328).
	is.lastEditOp = inputOpNone
	imap.Set(focusID, is)
}

// Word motion with no glyph layout. The rules live in go-glyph
// (layout_words.go) and are the same class-run rules its Layout methods
// apply, so a window with a text measurer and one without segment text
// identically. Never re-implement them here — one copy is the point.
//
// The one divergence go-glyph documents: the string helpers have no
// grapheme-cluster data, so an emoji ZWJ sequence splits at the ZWJ where
// the Layout path keeps it whole. Accepted; the affected path is a
// double-click on an emoji sequence in an input.
//
// All three take the text and a rune index, converting at the go-glyph
// boundary, which is cheaper than the []rune conversion these used to do
// on every Ctrl+arrow keypress.

// moveCursorWordLeft returns the rune index of the word start before pos.
func moveCursorWordLeft(text string, pos int) int {
	if pos <= 0 {
		return 0
	}
	return byteToRuneIndex(text,
		glyph.WordStartLeft(text, runeToByteIndex(text, pos)))
}

// moveCursorWordRight returns the rune index of the word start after pos.
// Note this lands on the next word's *start*, not past the trailing
// whitespace of the current one — the two agree for space-separated text
// and differ once punctuation forms words of its own.
func moveCursorWordRight(text string, pos int) int {
	return byteToRuneIndex(text,
		glyph.WordStartRight(text, runeToByteIndex(text, pos)))
}

// wordBoundsAt returns the start and end rune indices of the word
// surrounding pos. A punctuation run is a word of its own; if pos is on
// whitespace, the whitespace run is selected, which is what double-click
// does on macOS. The exception is whitespace-only text, which has no words
// at all: the range is empty at pos, mirroring the layout path's
// GetWordAtIndex (see glyph.WordBoundsInString).
func wordBoundsAt(text string, pos int) (int, int) {
	if text == "" {
		return 0, 0
	}
	beg, end := glyph.WordBoundsInString(text, runeToByteIndex(text, pos))
	return byteToRuneIndex(text, beg), byteToRuneIndex(text, end)
}

// moveCursorUp moves cursor up one line in multiline text.
func moveCursorUp(runes []rune, pos int) int {
	// Find start of current line.
	lineStart := pos
	for lineStart > 0 && runes[lineStart-1] != '\n' {
		lineStart--
	}
	if lineStart == 0 {
		return 0 // Already on first line.
	}
	col := pos - lineStart
	// Find start of previous line.
	prevLineEnd := lineStart - 1
	prevLineStart := prevLineEnd
	for prevLineStart > 0 && runes[prevLineStart-1] != '\n' {
		prevLineStart--
	}
	prevLineLen := prevLineEnd - prevLineStart
	col = min(col, prevLineLen)
	return prevLineStart + col
}

// moveCursorDown moves cursor down one line in multiline text.
func moveCursorDown(runes []rune, pos int) int {
	n := len(runes)
	// Find start of current line.
	lineStart := pos
	for lineStart > 0 && runes[lineStart-1] != '\n' {
		lineStart--
	}
	col := pos - lineStart
	// Find end of current line (next \n).
	lineEnd := pos
	for lineEnd < n && runes[lineEnd] != '\n' {
		lineEnd++
	}
	if lineEnd >= n {
		return n // Already on last line.
	}
	// Next line starts after \n.
	nextLineStart := lineEnd + 1
	nextLineEnd := nextLineStart
	for nextLineEnd < n && runes[nextLineEnd] != '\n' {
		nextLineEnd++
	}
	nextLineLen := nextLineEnd - nextLineStart
	col = min(col, nextLineLen)
	return nextLineStart + col
}

// moveCursorLineStart returns the start of the current line.
func moveCursorLineStart(runes []rune, pos int) int {
	for pos > 0 && runes[pos-1] != '\n' {
		pos--
	}
	return pos
}

// moveCursorLineEnd returns the end of the current line.
func moveCursorLineEnd(runes []rune, pos int) int {
	n := len(runes)
	for pos < n && runes[pos] != '\n' {
		pos++
	}
	return pos
}
