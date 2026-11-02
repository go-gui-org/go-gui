package gui

import (
	"strings"
	"unicode/utf8"

	"github.com/go-gui-org/go-glyph"
)

// inputMaxRunesDefault is the rune budget of a field that leaves
// InputCfg.MaxLength unset (#1187): 1,048,576 runes, a few megabytes of
// text. An edit that would take the field past its budget is refused
// whole. The bound exists because every edit copies the text once.
const inputMaxRunesDefault = 1 << 20

// inputA11YLabelRunes bounds the accessible name an input's text gives
// its inner text shape. A screen reader is handed a prefix of a large
// field, not megabytes on every tree sync.
const inputA11YLabelRunes = 65_536

// inputBudget resolves InputCfg.MaxLength: a positive value is the
// budget, anything else the default.
func inputBudget(maxLength int) int {
	if maxLength > 0 {
		return maxLength
	}
	return inputMaxRunesDefault
}

// inputState manages the caret, selection and undo history of an input
// field. Stored in the StateRegistry keyed by the field's effective ID.
//
// Offsets are byte offsets into the field's text — the unit every glyph
// layout query takes, so a key event converts nothing and a field of any
// size pays nothing per keystroke for its length. A caret rests on
// grapheme-cluster boundaries, which are rune boundaries.
type inputState struct {
	Undo          *BoundedStack[inputMemento]
	Redo          *BoundedStack[inputMemento]
	CursorPos     int // byte offset
	LastClickTime int64
	// selectBeg and selectEnd are byte offsets; equal means no
	// selection. selectBeg is the anchor, so it may exceed selectEnd.
	selectBeg      uint32
	selectEnd      uint32
	cursorOffset   float32
	cursorTrailing bool  // prefer end-of-previous-line at wrap boundaries
	lastEditOp     uint8 // kind of edit the current undo run holds (issue #328)
	// undoText is the text the history ends at: the text after the last
	// edit, undo or redo. A different text is one the app set, or
	// PostCommitNormalize made, which no step describes, so undo and
	// redo drop the history instead. The app hands back the string the
	// field produced, so the check stops at the shared pointer.
	undoText string
}

// inputMemento is one undo step: the edit as the bytes it replaced at a
// byte offset, and the caret and selection before it. Undo puts removed
// back where inserted now sits; redo is the inverse. A step holds its
// own bytes rather than a copy of the whole text, so the history of a
// large field costs what was edited, not fifty copies of the field
// (#1187).
type inputMemento struct {
	removed      string
	inserted     string
	off          int
	CursorPos    int
	selectBeg    uint32
	selectEnd    uint32
	cursorOffset float32
}

// applies reports whether the memento's inserted bytes still sit at its
// offset in text, as a step must to be undone. It does not tell an app's
// text from the field's: a delete inserted nothing, which is in any
// text. inputState.undoText does.
func (m inputMemento) applies(text string) bool {
	end := m.off + len(m.inserted)
	return m.off >= 0 && end <= len(text) && text[m.off:end] == m.inserted
}

// apply undoes the memento's edit on text.
func (m inputMemento) apply(text string) string {
	return text[:m.off] + m.removed + text[m.off+len(m.inserted):]
}

// inverse is the step that redoes what m undoes, keeping the caret and
// selection in is for the redo to restore.
func (m inputMemento) inverse(is inputState) inputMemento {
	return inputMemento{
		off:          m.off,
		removed:      m.inserted,
		inserted:     m.removed,
		CursorPos:    is.CursorPos,
		selectBeg:    is.selectBeg,
		selectEnd:    is.selectEnd,
		cursorOffset: is.cursorOffset,
	}
}

// merged returns the step that covers m's edit followed by a second edit
// that took oldText (the text after m) to newText by replacing removed at
// off with inserted. The span is the union of the two edits; before and
// after it, newText still reads as the text before m did. m must apply
// to oldText.
func (m inputMemento) merged(
	oldText, newText string, off int, removed, inserted string,
) inputMemento {
	a := min(m.off, off)
	bOld := max(m.off+len(m.inserted), off+len(removed))
	bNew := bOld + len(inserted) - len(removed)
	// The pre-run bytes of [a, bOld): the bytes before m's span, what m
	// removed, and the bytes after its span, all as oldText has them.
	pre := oldText[a:m.off] + m.removed + oldText[m.off+len(m.inserted):bOld]
	return inputMemento{
		off:          a,
		removed:      pre,
		inserted:     mementoSpan(newText[a:bNew], len(newText)),
		CursorPos:    m.CursorPos,
		selectBeg:    m.selectBeg,
		selectEnd:    m.selectEnd,
		cursorOffset: m.cursorOffset,
	}
}

// mementoSpan returns span for a memento to keep. A substring pins the
// whole text it was cut from for as long as the memento lives, so a span
// that is a small part of its text is copied; a span that is most of the
// text keeps the slice, which then pins at most four times its own size.
func mementoSpan(span string, textLen int) string {
	if len(span) == 0 {
		return ""
	}
	if len(span)*4 < textLen {
		return strings.Clone(span)
	}
	return span
}

// textDelta locates the edit that took old to new as the bytes outside
// their common prefix and suffix.
func textDelta(old, new string) (off int, removed, inserted string) {
	p := commonPrefixLen(old, new)
	s := commonSuffixLen(old, new, p)
	return p, old[p : len(old)-s], new[p : len(new)-s]
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
	return StateReadOr(w, nsInput, focusID, inputState{})
}

// inputPushUndo records the edit that took oldText to newText — removed
// replaced by inserted at off — for undo, coalescing it into the previous
// step when both are the same kind (insert or delete) and no selection
// was involved, so a typing run undoes as one step instead of one per
// character (issue #328). The merged step then spans both edits. removed
// and inserted are the strings the memento keeps (see mementoSpan); the
// caller stores the op it passed as the new lastEditOp.
func inputPushUndo(
	is inputState, oldText, newText string, off int,
	removed, inserted string, op uint8,
) *BoundedStack[inputMemento] {
	if removed == "" && inserted == "" {
		// The edit left the text as it was — a PreTextChange that handed
		// back the current text, say. A step for it would make a Ctrl+Z
		// that changes nothing.
		return is.Undo
	}
	stack := is.Undo
	if stack == nil {
		stack = newBoundedStack[inputMemento](undoMaxSize)
	}
	if op != inputOpNone && op == is.lastEditOp && is.selectBeg == is.selectEnd {
		if top, ok := stack.peek(); ok && top.applies(oldText) {
			stack.replaceTop(top.merged(oldText, newText, off, removed, inserted))
			return stack
		}
	}
	stack.Push(inputMemento{
		off:          off,
		removed:      removed,
		inserted:     inserted,
		CursorPos:    is.CursorPos,
		selectBeg:    is.selectBeg,
		selectEnd:    is.selectEnd,
		cursorOffset: is.cursorOffset,
	})
	return stack
}

func inputStateFromMemento(
	m inputMemento, undo, redo *BoundedStack[inputMemento], text string,
) inputState {
	// Only the text-visible fields restore. lastEditOp resets to
	// inputOpNone so the next edit starts a fresh undo step, and
	// LastClickTime and cursorTrailing drop as transient.
	return inputState{
		CursorPos:    m.CursorPos,
		selectBeg:    m.selectBeg,
		selectEnd:    m.selectEnd,
		cursorOffset: m.cursorOffset,
		Undo:         undo,
		Redo:         redo,
		undoText:     text,
	}
}

// historyHolds reports whether the undo history still describes text:
// the text it ends at is text. On false it clears both stacks.
func historyHolds(text string, is inputState) bool {
	if text == is.undoText {
		return true
	}
	if is.Undo != nil {
		is.Undo.Clear()
	}
	if is.Redo != nil {
		is.Redo.Clear()
	}
	return false
}

// inputStoreState writes is as the input state for focusID.
func inputStoreState(focusID string, w *Window, is inputState) {
	StateMap[string, inputState](w, nsInput, capMany).Set(focusID, is)
}

// inputProposedText returns the text that would result from
// inserting insertText at the cursor without modifying state.
func inputProposedText(text, insertText string, focusID string, w *Window) string {
	return editProposedText(text, insertText, inputStateOrDefault(focusID, w))
}

// inputInsert inserts text at cursor or replaces selection.
// Returns resulting text. See editInsert.
func inputInsert(text string, insertText string, focusID string, w *Window) string {
	next, is, ok := editInsert(text, insertText, inputStateOrDefault(focusID, w))
	if ok {
		inputStoreState(focusID, w, is)
	}
	return next
}

// isMultiRuneInsert reports whether s holds more than one rune,
// stopping at the second. Paste and IME commits (multi-rune) break
// the typing undo run; single keystrokes coalesce into one step.
func isMultiRuneInsert(s string) bool {
	n := 0
	for range s {
		n++
		if n > 1 {
			return true
		}
	}
	return false
}

// truncateRunes caps s at maxRunes runes without converting the whole
// string to []rune first: it counts only as far as the cap.
func truncateRunes(s string, maxRunes int) string {
	if len(s) <= maxRunes {
		return s // byte length bounds rune count from above
	}
	// runeToByteIndex stops at len(s), so a string with fewer runes
	// than the cap slices to itself.
	return s[:runeToByteIndex(s, maxRunes)]
}

// inputEditFits reports whether inserting ins at the caret, or over the
// selection, keeps text within maxRunes runes. An edit that does not fit
// is refused whole rather than cut (#1187). An edit that does not grow
// the text always fits, so a field holding app-set text past its budget
// still takes deletes and replacements by text no longer than the
// selection.
func inputEditFits(text, ins string, is inputState, maxRunes int) bool {
	beg, end, ok := editSpan(text, is)
	if !ok {
		return true // the edit core refuses it; nothing grows
	}
	if len(text)-(end-beg)+len(ins) <= maxRunes {
		return true // byte length bounds rune count from above
	}
	removed := utf8RuneCount(text[beg:end])
	// Count the insert only as far as could ever fit, so an oversize
	// clipboard is refused without scanning all of it.
	limit := maxRunes + removed
	if len(truncateRunes(ins, limit)) < len(ins) {
		return false
	}
	added := utf8RuneCount(ins)
	if added <= removed {
		return true
	}
	return utf8RuneCount(text)-removed+added <= maxRunes
}

// capCallbackText bounds text an app callback returns (PreTextChange
// adjusted, PostCommitNormalize output) at the field's budget. The
// callback is trusted code but not size-checked code: without a bound
// one runaway return pins unbounded memory in per-window state.
func capCallbackText(s string, maxRunes int) string {
	return truncateRunes(s, maxRunes)
}

// inputSetTextAndCursorAtEnd pushes undo and places cursor at end
// of newText. See editSetTextCursorAtEnd.
func inputSetTextAndCursorAtEnd(oldText, newText string, focusID string, w *Window) {
	is := inputStateOrDefault(focusID, w)
	inputStoreState(focusID, w, editSetTextCursorAtEnd(oldText, newText, is))
}

// inputDelete removes text at cursor or selected range. Returns the new
// text and whether it changed. See editDelete.
func inputDelete(text string, focusID string, forwardDelete bool, w *Window) (string, bool) {
	next, is, ok := editDelete(text, inputStateOrDefault(focusID, w), forwardDelete)
	if ok {
		inputStoreState(focusID, w, is)
	}
	return next, ok
}

// inputDeleteSurrounding removes an Android InputConnection range and stores
// the resulting caret/selection state.
func inputDeleteSurrounding(
	text, focusID string, forward bool, count int32,
	inCodePoints bool, w *Window,
) (string, bool) {
	next, is, ok := editDeleteSurrounding(
		text, inputStateOrDefault(focusID, w), forward,
		int(count), inCodePoints,
	)
	if ok {
		inputStoreState(focusID, w, is)
	}
	return next, ok
}

// inputCopy returns the selected text. Returns ("", false) if
// no selection or password mode.
func inputCopy(text string, focusID string, isPassword bool, w *Window) (string, bool) {
	return editSelectedText(text, inputStateOrDefault(focusID, w), isPassword)
}

// inputCut copies selected text then deletes it.
func inputCut(text string, focusID string, isPassword bool, w *Window) (string, string, bool) {
	newText, copied, is, ok := editCut(text, inputStateOrDefault(focusID, w), isPassword)
	if ok {
		inputStoreState(focusID, w, is)
	}
	return newText, copied, ok
}

// inputUndo reverts to previous state. Returns restored text.
func inputUndo(text string, focusID string, w *Window) string {
	// Default InputState{}: zero value means no undo state exists.
	next, is, ok := editUndo(text, inputStateOrDefault(focusID, w))
	if ok {
		inputStoreState(focusID, w, is)
	}
	return next
}

// inputRedo reapplies a previously undone operation.
func inputRedo(text string, focusID string, w *Window) string {
	// Default InputState{}: zero value means no redo state exists.
	next, is, ok := editRedo(text, inputStateOrDefault(focusID, w))
	if ok {
		inputStoreState(focusID, w, is)
	}
	return next
}

// inputSelectAll selects all text.
func inputSelectAll(text string, focusID string, w *Window) {
	// Default InputState{}: zero value seeds initial select-all state.
	inputStoreState(focusID, w, editSelectAll(text, inputStateOrDefault(focusID, w)))
}

// updateCursorAndSelection moves cursor to newPos, extending
// or resetting selection based on shift modifier. Positions outside
// [0, textLen] (corrupt state, never produced by the cursor
// functions) clamp into range rather than wrapping through the
// uint32 selection casts below.
func updateCursorAndSelection(
	imap *BoundedMap[string, inputState],
	focusID string,
	is inputState,
	newPos int,
	isShift bool,
	textLen int,
) {
	newPos = max(0, min(newPos, textLen))
	anchor := max(0, min(is.CursorPos, textLen))
	if isShift {
		if is.selectBeg == is.selectEnd {
			// Start new selection from current cursor.
			is.selectBeg = uint32(anchor)
			is.selectEnd = uint32(newPos)
		} else {
			// Extend: move the end that matches current cursor.
			if uint32(anchor) == is.selectEnd {
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

// moveCursorWordLeft returns the byte offset of the word start before pos.
func moveCursorWordLeft(text string, pos int) int {
	if pos <= 0 {
		return 0
	}
	return glyph.WordStartLeft(text, pos)
}

// moveCursorWordRight returns the byte offset of the word start after pos.
// Note this lands on the next word's *start*, not past the trailing
// whitespace of the current one — the two agree for space-separated text
// and differ once punctuation forms words of its own.
func moveCursorWordRight(text string, pos int) int {
	return glyph.WordStartRight(text, pos)
}

// wordBoundsAt returns the start and end byte offsets of the word
// surrounding pos. A punctuation run is a word of its own; if pos is on
// whitespace, the whitespace run is selected, which is what double-click
// does on macOS. The exception is whitespace-only text, which has no words
// at all: the range is empty at pos, mirroring the layout path's
// GetWordAtIndex (see glyph.WordBoundsInString).
func wordBoundsAt(text string, pos int) (int, int) {
	if text == "" {
		return 0, 0
	}
	return glyph.WordBoundsInString(text, pos)
}

// Line motion below walks bytes: '\n' never appears inside a multibyte
// sequence, so finding line boundaries needs no decoding. The column
// carried to the other line is counted in RUNES: a byte column onto a
// line whose runes have different widths would land mid-rune.

// lineStartBefore returns the byte offset after the '\n' before pos, or
// 0 on the first line.
func lineStartBefore(text string, pos int) int {
	for i := min(pos, len(text)) - 1; i >= 0; i-- {
		if text[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

// lineEndAfter returns the byte offset of the '\n' at or after pos, or
// len(text) on the last line.
func lineEndAfter(text string, pos int) int {
	for i := max(pos, 0); i < len(text); i++ {
		if text[i] == '\n' {
			return i
		}
	}
	return len(text)
}

// advanceRunes returns the byte offset after n runes of s, or len(s)
// when s is shorter.
func advanceRunes(s string, n int) int {
	i := 0
	for n > 0 && i < len(s) {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		n--
	}
	return i
}

// moveCursorUp moves cursor up one line in multiline text.
func moveCursorUp(text string, pos int) int {
	pos = max(0, min(pos, len(text)))
	lineStart := lineStartBefore(text, pos)
	if lineStart == 0 {
		return 0 // Already on first line.
	}
	col := utf8.RuneCountInString(text[lineStart:pos])
	prevEnd := lineStart - 1
	prevStart := lineStartBefore(text, prevEnd)
	return prevStart + advanceRunes(text[prevStart:prevEnd], col)
}

// moveCursorDown moves cursor down one line in multiline text.
func moveCursorDown(text string, pos int) int {
	pos = max(0, min(pos, len(text)))
	lineStart := lineStartBefore(text, pos)
	lineEnd := lineEndAfter(text, pos)
	if lineEnd >= len(text) {
		return len(text) // Already on last line.
	}
	col := utf8.RuneCountInString(text[lineStart:pos])
	nextStart := lineEnd + 1
	nextEnd := lineEndAfter(text, nextStart)
	return nextStart + advanceRunes(text[nextStart:nextEnd], col)
}

// moveCursorLineStart returns the start of the current line.
func moveCursorLineStart(text string, pos int) int {
	return lineStartBefore(text, pos)
}

// moveCursorLineEnd returns the end of the current line.
func moveCursorLineEnd(text string, pos int) int {
	return lineEndAfter(text, pos)
}
