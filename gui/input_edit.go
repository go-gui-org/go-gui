package gui

// Pure edit core for text input (issue #657).
//
// Each function here takes the text and an inputState value and returns the
// result. None of them reads or writes a Window, so an edit can be tested
// with a table and no window. The input* functions in input_state.go load
// the state from the StateMap, call one of these, and store the result.
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

// editCommit returns the state after a text edit that put the caret at pos.
// It records the pre-edit text for undo (coalescing runs of the same op, see
// inputPushUndo), clears the selection and the redo stack, and resets the
// preferred column. cursorOffset is -1, not 0: 0 is a valid column for
// vertical motion, -1 recomputes it from the caret.
func editCommit(is inputState, oldText string, pos int, op uint8) inputState {
	return inputState{
		CursorPos:    pos,
		cursorOffset: -1,
		Undo:         inputPushUndo(is, oldText, op),
		lastEditOp:   op,
	}
}

// editProposedText returns the text that inserting ins at the caret (or
// over the selection) would give, without a state change.
func editProposedText(text, ins string, is inputState) string {
	if len(ins) == 0 {
		return text
	}
	insertRunes := []rune(truncateToMaxRunes(ins))
	runes := []rune(text)
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

// editInsert inserts ins at the caret, or replaces the selection with it.
// A negative caret appends. It returns false, and the text unchanged, for an
// empty insert or a selection outside the text.
func editInsert(text, ins string, is inputState) (string, inputState, bool) {
	if len(ins) == 0 {
		return text, is, false
	}
	insertRunes := []rune(truncateToMaxRunes(ins))

	runes := []rune(text)
	cursorPos := min(is.CursorPos, len(runes))
	if cursorPos < 0 {
		runes = append(runes, insertRunes...)
		cursorPos = len(runes)
	} else if is.selectBeg != is.selectEnd {
		beg, end := u32Sort(is.selectBeg, is.selectEnd)
		if int(beg) >= len(runes) || int(end) > len(runes) {
			return text, is, false
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

	op := inputOpInsert
	if isMultiRuneInsert(ins) {
		// Multi-rune inserts (paste, IME commits) are one undo step
		// even after a typing run, so they break the run.
		op = inputOpNone
	}
	return string(runes), editCommit(is, text, cursorPos, op), true
}

// editSetTextCursorAtEnd returns the state after the text was replaced with
// newText as a whole, caret at its end. Used when PreTextChange returns
// adjusted text where positional cursor mapping is unreliable. A programmatic
// text set is never part of a typing run.
func editSetTextCursorAtEnd(oldText, newText string, is inputState) inputState {
	return editCommit(is, oldText, utf8RuneCount(newText), inputOpNone)
}

// editDelete removes the selection, or one grapheme cluster at the caret. A
// plain (unselected) delete removes one whole grapheme cluster — the same
// granularity the glyph-backed path gives — so the nil-measurer fallback
// never splits an emoji or combining sequence. forward=true for Delete,
// false for Backspace. An edge delete (Backspace at 0, Delete at end) or a
// selection outside the text changes nothing and returns false. Consuming
// that keystroke is the caller's decision, not this bool's.
func editDelete(text string, is inputState, forward bool) (string, inputState, bool) {
	runes := []rune(text)
	cursorPos := min(is.CursorPos, len(runes))
	if cursorPos < 0 {
		cursorPos = len(runes)
	}

	if is.selectBeg != is.selectEnd {
		beg, end := u32Sort(is.selectBeg, is.selectEnd)
		if int(beg) >= len(runes) || int(end) > len(runes) {
			return text, is, false
		}
		result := make([]rune, 0, int(beg)+(len(runes)-int(end)))
		result = append(result, runes[:beg]...)
		result = append(result, runes[end:]...)
		runes = result
		cursorPos = min(int(beg), len(runes))
	} else {
		if cursorPos == 0 && !forward {
			return text, is, false
		}
		if cursorPos == len(runes) && forward {
			return text, is, false
		}
		// No glyph layout behind this path (nil textMeasurer), so
		// cluster boundaries are recomputed with UAX #29 — the same
		// segmentation shaping produces — keeping Backspace/Delete
		// whole-cluster in the fallback as well.
		stops := graphemeStops(text)
		delPos, delEnd := cursorPos, cursorPos
		if !forward {
			delPos = prevGraphemeStop(stops, cursorPos)
		} else {
			delEnd = nextGraphemeStop(stops, cursorPos)
		}
		if delPos < 0 || delPos >= len(runes) || delEnd > len(runes) {
			return text, is, false
		}
		result := make([]rune, 0, len(runes)-(delEnd-delPos))
		result = append(result, runes[:delPos]...)
		result = append(result, runes[delEnd:]...)
		runes = result
		if !forward {
			cursorPos = delPos
		}
	}

	return string(runes), editCommit(is, text, cursorPos, inputOpDelete), true
}

// editSelectedText returns the selected text. It returns ("", false) with
// no selection, a selection outside the text, or a password field. The
// clipboard write stays with the caller: the core has no platform.
func editSelectedText(text string, is inputState, isPassword bool) (string, bool) {
	if isPassword || is.selectBeg == is.selectEnd {
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

// editUndo restores the top undo memento and pushes the current text and
// caret on the redo stack. It returns false with nothing to undo.
func editUndo(text string, is inputState) (string, inputState, bool) {
	if is.Undo == nil || is.Undo.isEmpty() {
		return text, is, false
	}
	memento, ok := is.Undo.Pop()
	if !ok {
		return text, is, false
	}
	redo := is.Redo
	if redo == nil {
		redo = newBoundedStack[inputMemento](undoMaxSize)
	}
	redo.Push(inputMementoFromState(text, is))
	return memento.Text, inputStateFromMemento(memento, is.Undo, redo), true
}

// editRedo reapplies the top redo memento and pushes the current text and
// caret on the undo stack. It returns false with nothing to redo.
func editRedo(text string, is inputState) (string, inputState, bool) {
	if is.Redo == nil || is.Redo.isEmpty() {
		return text, is, false
	}
	memento, ok := is.Redo.Pop()
	if !ok {
		return text, is, false
	}
	undo := is.Undo
	if undo == nil {
		undo = newBoundedStack[inputMemento](undoMaxSize)
	}
	undo.Push(inputMementoFromState(text, is))
	return memento.Text, inputStateFromMemento(memento, undo, is.Redo), true
}

// editSelectAll selects the whole text and puts the caret at its end. The
// undo stacks and the undo run are kept.
func editSelectAll(text string, is inputState) inputState {
	runeCount := utf8RuneCount(text)
	is.selectBeg = 0
	is.selectEnd = uint32(runeCount)
	is.CursorPos = runeCount
	return is
}
