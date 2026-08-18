package gui

import (
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
			newPos, isShift)
	} else if !isShift && is.selectBeg != is.selectEnd {
		beg, _ := u32Sort(is.selectBeg, is.selectEnd)
		updateCursorAndSelection(imap, id, is,
			int(beg), false)
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
			newPos, isShift)
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
			newPos, isShift)
	} else if !isShift && is.selectBeg != is.selectEnd {
		_, end := u32Sort(is.selectBeg, is.selectEnd)
		updateCursorAndSelection(imap, id, is,
			int(end), false)
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
			newPos, isShift)
	}
}

func inputKeyHome(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, isShift, savedTrailing bool,
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
		if pos != lineStart {
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
		lineStart := moveCursorLineStart([]rune(text), pos)
		if pos != lineStart {
			newPos = lineStart
		} else {
			newPos = cursorHome()
		}
	}
	updateCursorAndSelection(imap, id, is,
		newPos, isShift)
}

func inputKeyEnd(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, isShift, savedTrailing bool,
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
		if pos != lineEnd {
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
		lineEnd := moveCursorLineEnd([]rune(text), pos)
		if pos != lineEnd {
			newPos = lineEnd
			trailing = true
		} else {
			newPos = cursorEnd(text)
		}
	}
	is.cursorTrailing = trailing
	updateCursorAndSelection(imap, id, is,
		newPos, isShift)
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
			newPos = moveCursorUp([]rune(text), pos)
		} else {
			newPos = moveCursorDown([]rune(text), pos)
		}
		// Line-column math is rune-based; snap the result to the
		// nearest cluster boundary so vertical motion cannot park
		// the caret inside a multi-rune grapheme.
		newPos = closestGraphemeStop(graphemeStops(text), newPos)
	}
	updateCursorAndSelection(imap, id, is,
		newPos, isShift)
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
	if mask != nil {
		cis := inputStateOrDefault(id, w)
		res := inputMaskInsert(text,
			cis.CursorPos,
			cis.selectBeg,
			cis.selectEnd, clip, mask)
		if res.Changed {
			// A paste is never part of a typing run.
			undo := inputPushUndo(cis, text, inputOpNone)
			StateMap[string, inputState](
				w, nsInput, capMany,
			).Set(id, inputState{
				CursorPos: res.CursorPos,
				Undo:      undo,
			})
			return res.Text, true
		}
		return text, false
	}
	if hcfg.preTextChange != nil {
		proposed := inputProposedText(text, clip, id, w)
		adjusted, ok := hcfg.preTextChange(text, proposed)
		if !ok {
			return text, false
		}
		if adjusted == proposed {
			return inputInsert(text, clip, id, w), true
		}
		inputSetTextAndCursorAtEnd(text, adjusted, id, w)
		return adjusted, true
	}
	return inputInsert(text, clip, id, w), true
}

// inputCommitEnter handles single-line Enter: normalize, commit,
// fire OnEnter.
func inputCommitEnter(
	hcfg inputHandlerCfg,
	layout *Layout, text string, e *Event, w *Window,
) {
	commitText := text
	if normalized := hcfg.normalizeOnCommit(
		text, commitEnter,
	); normalized != text {
		commitText = normalized
		hcfg.fireTextChanged(layout, commitText, w)
	}
	if hcfg.OnTextCommit != nil {
		hcfg.OnTextCommit(commitText, commitEnter, EventCtx{layout, nil, w})
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
		undo := inputPushUndo(is, text, inputOpDelete)
		StateMap[string, inputState](
			w, nsInput, capMany,
		).Set(id, inputState{
			CursorPos: res.CursorPos, Undo: undo, lastEditOp: inputOpDelete,
		})
		return res.Text, true
	}
	return inputDeleteGrapheme(text, id, forward, layout, w)
}
