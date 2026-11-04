package gui

import "unicode/utf8"

// Pure edit core for text input (issue #657).
//
// Each function here takes the text and an inputState value and returns the
// result. None of them reads or writes a Window, so an edit can be tested
// with a table and no window. The input* functions in input_state.go load
// the state from the StateMap, call one of these, and store the result.
//
// Offsets are byte offsets (#1187): an insert is a string concatenation
// and a delete a slice, with no []rune round trip, so an edit costs one
// copy of the text whatever its length.
//
// "Pure" means no window, not immutable. inputState.Undo and Redo are
// *BoundedStack values, so a push or pop here changes the stack the caller
// gave. A caller that drops the returned state still sees the stack change.
// A copy-on-write history would remove that, but it allocates on every
// keystroke, so it was rejected.
//
// Functions that return a bool use it to say "store the returned state".
// A false result means the edit did nothing and the caller must keep the
// stored state unchanged, which is what the window-bound functions did
// before the extraction.

// editCommit returns the state after an edit took oldText to newText and
// put the caret at pos, locating the edit as the bytes outside the texts'
// common prefix and suffix. It records the edit for undo (coalescing runs
// of the same op, see inputPushUndo), clears the selection and the redo
// stack, and resets the preferred column. cursorOffset is -1, not 0: 0 is
// a valid column for vertical motion, -1 recomputes it from the caret.
func editCommit(is inputState, oldText, newText string, pos int, op uint8) inputState {
	off, removed, inserted := textDelta(oldText, newText)
	return editCommitDelta(is, oldText, newText, pos, op, off,
		mementoSpan(removed, len(oldText)), mementoSpan(inserted, len(newText)))
}

// editCommitDelta is editCommit for an edit the caller already knows:
// removed replaced by inserted at off. removed and inserted are the
// strings the memento keeps, so a caller passes its own insert string as
// is and a slice of the old text through mementoSpan.
func editCommitDelta(
	is inputState, oldText, newText string, pos int, op uint8,
	off int, removed, inserted string,
) inputState {
	// An app that replaced the text since the last edit leaves steps
	// that describe a text that is gone: historyHolds clears them, and
	// this edit starts the history afresh in the same stack.
	historyHolds(oldText, is)
	return inputState{
		CursorPos:    pos,
		cursorOffset: -1,
		Undo:         inputPushUndo(is, oldText, newText, off, removed, inserted, op),
		lastEditOp:   op,
		undoText:     newText,
	}
}

// editCaretOnly returns the state after an insert that was accepted but
// left the text as it was: a masked digit over the same digit. The caret
// moves to pos and the selection clears, as for any insert, but the undo
// and redo stacks stay as they are, since there is no text to restore.
func editCaretOnly(is inputState, pos int) inputState {
	is.CursorPos = pos
	is.selectBeg, is.selectEnd = 0, 0
	is.cursorOffset = -1
	return is
}

// editSpan returns the byte range an insert at the caret, or over the
// selection, replaces. A negative caret appends. ok is false for a
// selection outside the text.
func editSpan(text string, is inputState) (beg, end int, ok bool) {
	if is.selectBeg != is.selectEnd {
		b, e := u32Sort(is.selectBeg, is.selectEnd)
		if int(b) >= len(text) || int(e) > len(text) {
			return 0, 0, false
		}
		return int(b), int(e), true
	}
	c := is.CursorPos
	if c < 0 {
		c = len(text)
	}
	c = min(c, len(text))
	return c, c, true
}

// editProposedText returns the text that inserting ins at the caret (or
// over the selection) would give, without a state change.
func editProposedText(text, ins string, is inputState) string {
	if len(ins) == 0 {
		return text
	}
	beg, end, ok := editSpan(text, is)
	if !ok {
		return text
	}
	return text[:beg] + ins + text[end:]
}

// editCommitInsert records an insertion whose span and resulting text the
// caller already knows. Keeping this separate lets PreTextChange commit the
// proposal it just validated instead of allocating the same complete string
// again.
func editCommitInsert(
	is inputState, text, newText, ins string, beg, end int,
) inputState {
	op := inputOpInsert
	if isMultiRuneInsert(ins) {
		// Multi-rune inserts (paste, IME commits) are one undo step
		// even after a typing run, so they break the run.
		op = inputOpNone
	}
	return editCommitDelta(is, text, newText, beg+len(ins), op,
		beg, mementoSpan(text[beg:end], len(text)), ins)
}

// editInsert inserts ins at the caret, or replaces the selection with it.
// A negative caret appends. It returns false, and the text unchanged, for an
// empty insert or a selection outside the text. The field's budget is the
// caller's to check (inputEditFits): the core splices what it is given.
func editInsert(text, ins string, is inputState) (string, inputState, bool) {
	if len(ins) == 0 {
		return text, is, false
	}
	beg, end, ok := editSpan(text, is)
	if !ok {
		return text, is, false
	}
	newText := text[:beg] + ins + text[end:]
	return newText, editCommitInsert(is, text, newText, ins, beg, end), true
}

// editInsertProposed commits proposed without rebuilding it when it is the
// exact insertion ins makes in the current state. PreTextChange runs app code
// between proposing and accepting; if that code indirectly changed the caret
// or selection, the proposal no longer describes the current edit. In that
// rare case, fall back to editInsert and preserve the previous behavior.
func editInsertProposed(
	text, ins, proposed string, is inputState,
) (string, inputState, bool) {
	if len(ins) == 0 {
		return text, is, false
	}
	beg, end, ok := editSpan(text, is)
	if !ok {
		return text, is, false
	}
	insertEnd := beg + len(ins)
	wantLen := len(text) - (end - beg) + len(ins)
	if len(proposed) != wantLen ||
		proposed[:beg] != text[:beg] ||
		proposed[beg:insertEnd] != ins ||
		proposed[insertEnd:] != text[end:] {
		return editInsert(text, ins, is)
	}
	return proposed,
		editCommitInsert(is, text, proposed, ins, beg, end), true
}

// editSetTextCursorAtEnd returns the state after the text was replaced with
// newText as a whole, caret at its end. Used when PreTextChange returns
// adjusted text where positional cursor mapping is unreliable. A programmatic
// text set is never part of a typing run.
func editSetTextCursorAtEnd(oldText, newText string, is inputState) inputState {
	return editCommit(is, oldText, newText, len(newText), inputOpNone)
}

// editDelete removes the selection, or one grapheme cluster at the caret. A
// plain (unselected) delete removes one whole grapheme cluster — the same
// granularity the glyph-backed path gives — so the nil-measurer fallback
// never splits an emoji or combining sequence. forward=true for Delete,
// false for Backspace. An edge delete (Backspace at 0, Delete at end) or a
// selection outside the text changes nothing and returns false. Consuming
// that keystroke is the caller's decision, not this bool's.
func editDelete(text string, is inputState, forward bool) (string, inputState, bool) {
	var delBeg, delEnd int
	if is.selectBeg != is.selectEnd {
		beg, end, ok := editSpan(text, is)
		if !ok {
			return text, is, false
		}
		delBeg, delEnd = beg, end
	} else {
		c := is.CursorPos
		if c < 0 {
			c = len(text)
		}
		c = min(c, len(text))
		if (c == 0 && !forward) || (c == len(text) && forward) {
			return text, is, false
		}
		// No shaped clusters behind this path (see inputDeleteGrapheme:
		// no measurer, a password field, a chunk seam), so cluster
		// boundaries are recomputed with UAX #29 — the same segmentation
		// shaping produces — keeping Backspace/Delete whole-cluster in
		// the fallback as well.
		if forward {
			delBeg, delEnd = c, nextGraphemeBoundary(text, c)
		} else {
			delBeg, delEnd = prevGraphemeBoundary(text, c-1), c
		}
	}
	if delBeg >= delEnd {
		return text, is, false
	}
	newText := text[:delBeg] + text[delEnd:]
	next := editCommitDelta(is, text, newText, delBeg, inputOpDelete,
		delBeg, mementoSpan(text[delBeg:delEnd], len(text)), "")
	return newText, next, true
}

// editDeleteSurrounding removes count code points or UTF-16 code units next
// to the selection without deleting the selection itself, matching Android's
// InputConnection deletion contract. A UTF-16 request that lands inside a
// surrogate pair rounds outward to the rune boundary: Go strings cannot hold
// the unpaired surrogate Android's Java buffer could otherwise create. It
// does not round to a grapheme cluster, as Backspace does: the input method
// asks for part of a cluster on purpose (Backspace on "कि" takes only the
// vowel sign), and taking more would leave its view of the text out of step.
func editDeleteSurrounding(
	text string, is inputState, forward bool, count int, inCodePoints bool,
) (string, inputState, bool) {
	if count <= 0 {
		return text, is, false
	}
	c := is.CursorPos
	if c < 0 {
		c = len(text)
	}
	c = min(c, len(text))

	beg, end := c, c
	hasSelection := is.selectBeg != is.selectEnd
	if hasSelection {
		sb, se := u32Sort(is.selectBeg, is.selectEnd)
		if int(sb) > len(text) || int(se) > len(text) {
			return text, is, false
		}
		beg, end = int(sb), int(se)
	}

	units := func(r rune) int {
		if inCodePoints || r <= 0xFFFF {
			return 1
		}
		return 2
	}
	delBeg, delEnd := beg, end
	remaining := count
	if forward {
		delBeg = end
		for delEnd < len(text) && remaining > 0 {
			r, size := utf8.DecodeRuneInString(text[delEnd:])
			remaining -= units(r)
			delEnd += size
		}
	} else {
		delEnd = beg
		for delBeg > 0 && remaining > 0 {
			r, size := utf8.DecodeLastRuneInString(text[:delBeg])
			delBeg -= size
			remaining -= units(r)
		}
	}
	if delBeg == delEnd {
		return text, is, false
	}

	adjust := func(pos int) int {
		switch {
		case pos <= delBeg:
			return pos
		case pos >= delEnd:
			return pos - (delEnd - delBeg)
		default:
			return delBeg
		}
	}
	newText := text[:delBeg] + text[delEnd:]
	next := editCommitDelta(is, text, newText, adjust(c), inputOpDelete,
		delBeg, mementoSpan(text[delBeg:delEnd], len(text)), "")
	if hasSelection {
		next.selectBeg = uint32(adjust(int(is.selectBeg)))
		next.selectEnd = uint32(adjust(int(is.selectEnd)))
	}
	return newText, next, true
}

// editSelectedText returns the selected text. It returns ("", false) with
// no selection, a selection outside the text, or a password field. The
// clipboard write stays with the caller: the core has no platform.
func editSelectedText(text string, is inputState, isPassword bool) (string, bool) {
	if isPassword || is.selectBeg == is.selectEnd {
		return "", false
	}
	beg, end := u32Sort(is.selectBeg, is.selectEnd)
	if int(beg) > len(text) || int(end) > len(text) || beg >= end {
		return "", false
	}
	return text[beg:end], true
}

// editCut returns the text with the selection removed, the removed text, and
// the new state. It returns false, with text and state unchanged, when
// there is nothing to copy (see editSelectedText).
func editCut(text string, is inputState, isPassword bool) (string, string, inputState, bool) {
	copied, ok := editSelectedText(text, is, isPassword)
	if !ok {
		return text, "", is, false
	}
	// A failed delete returns is unchanged, so storing next is safe
	// either way.
	newText, next, _ := editDelete(text, is, false)
	return newText, copied, next, true
}

// editUndo reverts the top undo step and pushes its inverse, with the
// current caret, on the redo stack. It returns false with nothing to
// undo. A step that no longer applies — the app replaced the text under
// the field — empties both stacks and changes nothing: undo is not meant
// to splice stale bytes into text it has never seen.
func editUndo(text string, is inputState) (string, inputState, bool) {
	if is.Undo == nil || is.Undo.isEmpty() {
		return text, is, false
	}
	if !historyHolds(text, is) {
		return text, is, false
	}
	memento, ok := is.Undo.Pop()
	if !ok {
		return text, is, false
	}
	if !memento.applies(text) {
		is.Undo.Clear()
		if is.Redo != nil {
			is.Redo.Clear()
		}
		return text, is, false
	}
	redo := is.Redo
	if redo == nil {
		redo = newBoundedStack[inputMemento](undoMaxSize)
	}
	redo.Push(memento.inverse(is))
	undone := memento.apply(text)
	return undone, inputStateFromMemento(memento, is.Undo, redo, undone), true
}

// editRedo reapplies the top redo step and pushes its inverse, with the
// current caret, on the undo stack. It returns false with nothing to
// redo, and drops the history like editUndo when the step no longer
// applies.
func editRedo(text string, is inputState) (string, inputState, bool) {
	if is.Redo == nil || is.Redo.isEmpty() {
		return text, is, false
	}
	if !historyHolds(text, is) {
		return text, is, false
	}
	memento, ok := is.Redo.Pop()
	if !ok {
		return text, is, false
	}
	if !memento.applies(text) {
		is.Redo.Clear()
		if is.Undo != nil {
			is.Undo.Clear()
		}
		return text, is, false
	}
	undo := is.Undo
	if undo == nil {
		undo = newBoundedStack[inputMemento](undoMaxSize)
	}
	undo.Push(memento.inverse(is))
	redone := memento.apply(text)
	return redone, inputStateFromMemento(memento, undo, is.Redo, redone), true
}

// editSelectAll selects the whole text and puts the caret at its end. The
// undo stacks and the undo run are kept.
func editSelectAll(text string, is inputState) inputState {
	is.selectBeg = 0
	is.selectEnd = uint32(len(text))
	is.CursorPos = len(text)
	return is
}
