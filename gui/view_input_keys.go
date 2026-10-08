package gui

import (
	"strings"

	"github.com/go-gui-org/go-glyph"
)

func inputKeyLeft(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, isShift, isWordMod bool,
	gl glyph.Layout, glOK bool,
) {
	if isWordMod {
		var newPos int
		if glOK {
			byteIdx := runeToByteIndex(text, pos)
			newPos = byteToRuneIndex(text,
				gl.MoveCursorWordLeft(byteIdx))
		} else {
			newPos = moveCursorWordLeft(text, pos)
		}
		updateCursorAndSelection(imap, id, is,
			newPos, isShift, utf8RuneCount(text))
	} else if !isShift && is.selectBeg != is.selectEnd {
		beg, _ := u32Sort(is.selectBeg, is.selectEnd)
		updateCursorAndSelection(imap, id, is,
			int(beg), false, utf8RuneCount(text))
	} else {
		var newPos int
		if glOK {
			byteIdx := runeToByteIndex(text, pos)
			newPos = byteToRuneIndex(text,
				gl.MoveCursorLeft(byteIdx))
		} else {
			// No glyph layout: snap to the previous UAX #29 grapheme
			// boundary so the caret never rests mid-cluster.
			newPos = prevGraphemeStop(graphemeStops(text), pos)
		}
		updateCursorAndSelection(imap, id, is,
			newPos, isShift, utf8RuneCount(text))
	}
}

func inputKeyRight(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, isShift, isWordMod bool,
	gl glyph.Layout, glOK bool,
) {
	if isWordMod {
		var newPos int
		if glOK {
			byteIdx := runeToByteIndex(text, pos)
			newPos = byteToRuneIndex(text,
				gl.MoveCursorWordRight(byteIdx))
		} else {
			newPos = moveCursorWordRight(text, pos)
		}
		updateCursorAndSelection(imap, id, is,
			newPos, isShift, utf8RuneCount(text))
	} else if !isShift && is.selectBeg != is.selectEnd {
		_, end := u32Sort(is.selectBeg, is.selectEnd)
		updateCursorAndSelection(imap, id, is,
			int(end), false, utf8RuneCount(text))
	} else {
		var newPos int
		if glOK {
			byteIdx := runeToByteIndex(text, pos)
			newPos = byteToRuneIndex(text,
				gl.MoveCursorRight(byteIdx))
		} else {
			// No glyph layout: snap to the next UAX #29 grapheme
			// boundary so the caret never rests mid-cluster.
			newPos = nextGraphemeStop(graphemeStops(text), pos)
		}
		updateCursorAndSelection(imap, id, is,
			newPos, isShift, utf8RuneCount(text))
	}
}

// inputKeyHome moves the caret to the line start. With cycle set (the
// Home key), a press already at the line start steps on to the
// paragraph start, then the text start. macOS Cmd+Left passes
// cycle=false and stays at the line edge, as Cocoa does (#969).
func inputKeyHome(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, isShift, savedTrailing, cycle bool,
	gl glyph.Layout, glOK bool,
) {
	var newPos int
	if glOK {
		byteIdx := runeToByteIndex(text, pos)
		startByte := gl.MoveCursorLineStart(byteIdx)
		if savedTrailing {
			startByte = trailingLineStart(
				gl.Lines, byteIdx, startByte)
		}
		lineStart := byteToRuneIndex(text, startByte)
		if pos != lineStart || !cycle {
			newPos = lineStart
		} else {
			paraStart := cursorStartOfParagraph(text, pos)
			if pos != paraStart {
				newPos = paraStart
			} else {
				newPos = cursorHome()
			}
		}
	} else {
		lineStart := moveCursorLineStart(text, pos)
		if pos != lineStart || !cycle {
			newPos = lineStart
		} else {
			newPos = cursorHome()
		}
	}
	updateCursorAndSelection(imap, id, is,
		newPos, isShift, utf8RuneCount(text))
}

// inputKeyEnd is inputKeyHome for the line end; cycle works the same.
func inputKeyEnd(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, isShift, savedTrailing, cycle bool,
	gl glyph.Layout, glOK bool,
) {
	var newPos int
	trailing := false
	if glOK {
		byteIdx := runeToByteIndex(text, pos)
		endByte := gl.MoveCursorLineEnd(byteIdx)
		if savedTrailing {
			endByte = trailingLineEnd(
				gl.Lines, byteIdx, endByte)
		}
		lineEnd := byteToRuneIndex(text, endByte)
		if pos != lineEnd || !cycle {
			newPos = lineEnd
			trailing = true
		} else {
			paraEnd := cursorEndOfParagraph(text, pos)
			if pos != paraEnd {
				newPos = paraEnd
			} else {
				newPos = cursorEnd(text)
			}
		}
	} else {
		lineEnd := moveCursorLineEnd(text, pos)
		if pos != lineEnd || !cycle {
			newPos = lineEnd
			trailing = true
		} else {
			newPos = cursorEnd(text)
		}
	}
	is.cursorTrailing = trailing
	updateCursorAndSelection(imap, id, is,
		newPos, isShift, utf8RuneCount(text))
}

// inputKeyVertical handles KeyUp (up=true) and KeyDown (up=false)
// for multiline inputs. Returns false when the key is unhandled
// (single-line mode).
func inputKeyVertical(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, isShift bool,
	savedOffset float32, up bool, mode inputMode,
	gl glyph.Layout, glOK bool,
) bool {
	if mode != InputMultiline {
		return false
	}
	var newPos int
	if glOK {
		byteIdx := runeToByteIndex(text, pos)
		preferredX := savedOffset
		if preferredX < 0 {
			if cp, ok := gl.GetCursorPos(byteIdx); ok {
				preferredX = cp.X
			}
		}
		is.cursorOffset = preferredX
		if up {
			newPos = byteToRuneIndex(text,
				gl.MoveCursorUp(byteIdx, preferredX))
		} else {
			newPos = byteToRuneIndex(text,
				gl.MoveCursorDown(byteIdx, preferredX))
		}
	} else {
		if up {
			newPos = moveCursorUp(text, pos)
		} else {
			newPos = moveCursorDown(text, pos)
		}
		// The column is counted in runes; snap the result to the
		// nearest cluster boundary so vertical motion cannot park
		// the caret inside a multi-rune grapheme.
		newPos = closestGraphemeStop(graphemeStops(text), newPos)
	}
	updateCursorAndSelection(imap, id, is,
		newPos, isShift, utf8RuneCount(text))
	return true
}

// inputKeyPaste handles Ctrl+V / Cmd+V with mask, PreTextChange,
// and plain-text branches. Returns updated text and whether it
// changed.
func inputKeyPaste(
	text, clip string, id string,
	mask *CompiledInputMask,
	hcfg inputHandlerCfg, w *Window,
) (string, bool) {
	if len(clip) == 0 {
		return text, false
	}
	// Bound before any []rune conversion downstream: clipboard
	// content is platform-sized, not app-sized.
	clip = truncateToMaxRunes(clip)
	if mask != nil {
		cis := inputStateOrDefault(id, w)
		res := inputMaskInsert(text,
			cis.CursorPos,
			cis.selectBeg,
			cis.selectEnd, clip, mask)
		if res.Changed {
			// A paste is never part of a typing run.
			inputStoreState(id, w, editCommit(cis, text, res.CursorPos, inputOpNone))
			return res.Text, true
		}
		// The mask has no room for the pasted text (issue #468).
		playSoundCue(hcfg.cues.reject, w)
		return text, false
	}
	if hcfg.preTextChange != nil {
		proposed := inputProposedText(text, clip, id, w)
		adjusted, ok := hcfg.preTextChange(text, proposed)
		if !ok {
			playSoundCue(hcfg.cues.reject, w)
			return text, false
		}
		if adjusted == proposed {
			return inputInsert(text, clip, id, w), true
		}
		adjusted = capCallbackText(adjusted)
		inputSetTextAndCursorAtEnd(text, adjusted, id, w)
		return adjusted, true
	}
	return inputInsert(text, clip, id, w), true
}

// inputKeyKill handles macOS Ctrl+K (#969): cut from the caret to the
// end of its paragraph into the window's kill buffer, as Cocoa does.
// At the paragraph end it cuts the line break, joining the next line.
// The cut goes through inputHandleDelete as a selection delete, so the
// mask and the undo stack see it like any other delete. A password
// field still deletes but keeps nothing, so Ctrl+Y cannot reveal it;
// a masked field does the same (see below).
func inputKeyKill(
	hcfg inputHandlerCfg, layout *Layout,
	text string, id string, pos int, w *Window,
) (string, bool) {
	end := cursorEndOfParagraph(text, pos)
	if end == pos {
		if pos >= utf8RuneCount(text) {
			return text, false
		}
		end = pos + 1
	}
	prev := inputStateOrDefault(id, w)
	is := prev
	is.CursorPos = pos
	is.selectBeg = uint32(pos)
	is.selectEnd = uint32(end)
	inputStoreState(id, w, is)
	newText, ok := inputHandleDelete(
		text, id, true, hcfg.CompiledMask, layout, w)
	if !ok {
		// Nothing deleted (a mask with only literals there): put the
		// old state back, so the range Ctrl+K chose does not stay
		// highlighted as a selection the user never made.
		inputStoreState(id, w, prev)
		return text, false
	}
	// The delete pushed an undo step from the state above, which holds
	// the kill range as a selection. Give that step the caret and
	// selection the user had, so undo does not highlight a range the
	// user never chose. A selection always forces a push
	// (inputPushUndo), so the top entry is this kill's.
	if undo := inputStateOrDefault(id, w).Undo; undo != nil {
		if m, ok := undo.Pop(); ok {
			m.CursorPos = prev.CursorPos
			m.selectBeg, m.selectEnd = prev.selectBeg, prev.selectEnd
			undo.Push(m)
		}
	}
	// A masked delete keeps the literals, so the slice is not what was
	// removed. Keep nothing rather than a wrong kill. Clear the buffer
	// instead of leaving it, so Ctrl+Y does not paste an older kill.
	if hcfg.CompiledMask != nil || hcfg.IsPassword {
		w.killBuffer = ""
		return newText, true
	}
	// Clone: a substring would pin the whole old text in memory for as
	// long as the kill buffer holds it.
	w.killBuffer = strings.Clone(
		text[runeToByteIndex(text, pos):runeToByteIndex(text, end)])
	return newText, true
}

// inputCommitEnter handles single-line Enter: normalize, commit,
// fire OnEnter.
func inputCommitEnter(
	hcfg inputHandlerCfg,
	layout *Layout, text string, e *Event, w *Window,
) {
	// Enter is a deliberate activation, so it sounds like one — before
	// the callbacks and independently of whether they consume, the way
	// dispatch sounds a click. The blur commit stays silent by
	// decision: it is incidental, not an activation (issue #468).
	playSoundCue(hcfg.cues.act, w)
	commitText := text
	if normalized := hcfg.normalizeOnCommit(
		text, InputCommitEnter,
	); normalized != text {
		commitText = normalized
		hcfg.fireTextChanged(layout, commitText, w)
	}
	if hcfg.OnTextCommit != nil {
		hcfg.OnTextCommit(commitText, InputCommitEnter, EventCtx{layout, nil, w})
	}
	if hcfg.OnEnter != nil {
		hcfg.OnEnter(EventCtx{layout, e, w})
	}
}

// inputHandleDelete handles Backspace/Delete for both masked and
// unmasked inputs.
func inputHandleDelete(
	text string, id string, forward bool,
	mask *CompiledInputMask,
	layout *Layout, w *Window,
) (string, bool) {
	if mask != nil {
		is := inputStateOrDefault(id, w)
		var res MaskEditResult
		if forward {
			res = inputMaskDelete(text, is.CursorPos,
				is.selectBeg, is.selectEnd, mask)
		} else {
			res = inputMaskBackspace(text, is.CursorPos,
				is.selectBeg, is.selectEnd, mask)
		}
		if !res.Changed {
			return text, false
		}
		inputStoreState(id, w, editCommit(is, text, res.CursorPos, inputOpDelete))
		return res.Text, true
	}
	return inputDeleteGrapheme(text, id, forward, layout, w)
}
