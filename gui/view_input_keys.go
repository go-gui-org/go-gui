package gui

import (
	"github.com/go-gui-org/go-glyph"
)

func inputKeyLeft(
	imap *BoundedMap[string, InputState], id string, is InputState,
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
			newPos = moveCursorWordLeft([]rune(text), pos)
		}
		updateCursorAndSelection(imap, id, is,
			newPos, isShift)
	} else if !isShift && is.SelectBeg != is.SelectEnd {
		beg, _ := u32Sort(is.SelectBeg, is.SelectEnd)
		updateCursorAndSelection(imap, id, is,
			int(beg), false)
	} else {
		var newPos int
		if glOK {
			byteIdx := runeToByteIndex(text, pos)
			newPos = byteToRuneIndex(text,
				gl.MoveCursorLeft(byteIdx))
		} else {
			newPos = pos - 1
			newPos = max(newPos, 0)
		}
		updateCursorAndSelection(imap, id, is,
			newPos, isShift)
	}
}

func inputKeyRight(
	imap *BoundedMap[string, InputState], id string, is InputState,
	text string, pos, runeLen int, isShift, isWordMod bool,
	gl glyph.Layout, glOK bool,
) {
	if isWordMod {
		var newPos int
		if glOK {
			byteIdx := runeToByteIndex(text, pos)
			newPos = byteToRuneIndex(text,
				gl.MoveCursorWordRight(byteIdx))
		} else {
			newPos = moveCursorWordRight([]rune(text), pos)
		}
		updateCursorAndSelection(imap, id, is,
			newPos, isShift)
	} else if !isShift && is.SelectBeg != is.SelectEnd {
		_, end := u32Sort(is.SelectBeg, is.SelectEnd)
		updateCursorAndSelection(imap, id, is,
			int(end), false)
	} else {
		var newPos int
		if glOK {
			byteIdx := runeToByteIndex(text, pos)
			newPos = byteToRuneIndex(text,
				gl.MoveCursorRight(byteIdx))
		} else {
			newPos = pos + 1
			newPos = min(newPos, runeLen)
		}
		updateCursorAndSelection(imap, id, is,
			newPos, isShift)
	}
}

func inputKeyHome(
	imap *BoundedMap[string, InputState], id string, is InputState,
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
	imap *BoundedMap[string, InputState], id string, is InputState,
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
	is.CursorTrailing = trailing
	updateCursorAndSelection(imap, id, is,
		newPos, isShift)
}

// inputKeyVertical handles KeyUp (up=true) and KeyDown (up=false)
// for multiline inputs. Returns false when the key is unhandled
// (single-line mode).
func inputKeyVertical(
	imap *BoundedMap[string, InputState], id string, is InputState,
	text string, pos int, isShift bool,
	savedOffset float32, up bool, mode InputMode,
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
		is.CursorOffset = preferredX
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
		res := InputMaskInsert(text,
			cis.CursorPos,
			cis.SelectBeg,
			cis.SelectEnd, clip, mask)
		if res.Changed {
			undo := inputPushUndo(cis, text)
			StateMap[string, InputState](
				w, nsInput, capMany,
			).Set(id, InputState{
				CursorPos: res.CursorPos,
				Undo:      undo,
			})
			return res.Text, true
		}
		return text, false
	}
	if hcfg.PreTextChange != nil {
		proposed := inputProposedText(text, clip, id, w)
		adjusted, ok := hcfg.PreTextChange(text, proposed)
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
		text, CommitEnter,
	); normalized != text {
		commitText = normalized
		hcfg.fireTextChanged(layout, commitText, w)
	}
	if hcfg.OnTextCommit != nil {
		hcfg.OnTextCommit(
			layout, commitText, CommitEnter, w)
	}
	if hcfg.OnEnter != nil {
		hcfg.OnEnter(layout, e, w)
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
			res = InputMaskDelete(text, is.CursorPos,
				is.SelectBeg, is.SelectEnd, mask)
		} else {
			res = InputMaskBackspace(text, is.CursorPos,
				is.SelectBeg, is.SelectEnd, mask)
		}
		if !res.Changed {
			return text, false
		}
		undo := inputPushUndo(is, text)
		StateMap[string, InputState](
			w, nsInput, capMany,
		).Set(id, InputState{
			CursorPos: res.CursorPos, Undo: undo,
		})
		return res.Text, true
	}
	return inputDeleteGrapheme(text, id, forward, layout, w)
}
