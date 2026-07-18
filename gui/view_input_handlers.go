package gui

// Input event handlers: the OnChar/OnKeyDown/OnKeyUp closures Input
// installs, the text-mutation helper they share, and the per-key
// helpers they dispatch to. Split from view_input.go to keep that file
// under the 800-line gate (scripts/large-files.sh).

func inputTextChange(hcfg inputHandlerCfg, layout *Layout, text, ins string, id string, w *Window) (string, bool) {
	mask := hcfg.CompiledMask
	if mask != nil {
		is := inputStateOrDefault(id, w)
		res := InputMaskInsert(text, is.CursorPos, is.SelectBeg, is.SelectEnd, ins, mask)
		if res.Changed {
			undo := inputPushUndo(is, text)
			text = res.Text
			StateMap[string, InputState](w, nsInput, capMany).Set(id, InputState{
				CursorPos: res.CursorPos, Undo: undo,
			})
			return text, true
		}
	} else if hcfg.PreTextChange != nil {
		proposed := inputProposedText(text, ins, id, w)
		if adjusted, ok := hcfg.PreTextChange(text, proposed); ok {
			if adjusted == proposed {
				text = inputInsert(text, ins, id, w)
			} else {
				inputSetTextAndCursorAtEnd(
					text, adjusted, id, w)
				text = adjusted
			}
			return text, true
		}
	} else {
		text = inputInsert(text, ins, id, w)
		return text, true
	}
	return text, false
}

func makeInputOnChar(hcfg inputHandlerCfg) func(*Layout, *Event, *Window) {
	return func(layout *Layout, e *Event, w *Window) {
		if hcfg.FocusID == "" || !w.IsFocus(hcfg.FocusID) {
			return
		}
		// Swallow typed and IME-composed text; the field keeps focus so
		// navigation, selection, and copy still work.
		if hcfg.ReadOnly {
			e.IsHandled = true
			return
		}
		ch := e.CharCode
		id := hcfg.FocusID

		// Control characters are handled by OnKeyDown.
		if ch < CharSpace {
			e.IsHandled = true
			return
		}

		text := inputTextFromLayout(layout)
		ins := e.IMEText
		if len(ins) == 0 {
			ins = string(rune(ch))
		}
		text, changed := inputTextChange(hcfg, layout, text, ins, id, w)

		if changed {
			resetBlinkCursorVisible(w)
			hcfg.fireTextChanged(layout, text, w)
			inputScrollCursorIntoView(
				hcfg.ScrollID, text, layout, w,
			)
		}
		e.IsHandled = true
	}
}

// inputKeyMutatesText reports whether a key event would change the
// input's text. Read-only fields swallow these while navigation
// (arrows/Home/End), selection (Shift, Ctrl+A), and copy (Ctrl+C) stay
// live. Cut/undo/redo only mutate with a Ctrl/Super modifier; without
// one their handlers decline the key, so it must not be swallowed here.
func inputKeyMutatesText(e *Event, mode InputMode) bool {
	switch e.KeyCode {
	case KeyBackspace, KeyDelete:
		return true
	case KeyEnter:
		// Multiline Enter inserts a newline. Single-line Enter commits
		// and must stay allowed so OnEnter/OnTextCommit still fire; its
		// one edit is PostCommitNormalize, which normalizeOnCommit
		// skips when read-only.
		return mode == InputMultiline
	case KeyV, KeyX, KeyZ:
		return e.Modifiers.HasAny(ModCtrl, ModSuper)
	}
	return false
}

func makeInputOnKeyDown(hcfg inputHandlerCfg) func(*Layout, *Event, *Window) {
	mask := hcfg.CompiledMask
	return func(layout *Layout, e *Event, w *Window) {
		if hcfg.FocusID == "" || !w.IsFocus(hcfg.FocusID) {
			return
		}
		if hcfg.ReadOnly && inputKeyMutatesText(e, hcfg.Mode) {
			e.IsHandled = true
			return
		}
		id := hcfg.FocusID
		imap := StateMap[string, InputState](w, nsInput, capMany)
		// Default InputState{}: zero CursorOffset/CursorTrailing seed
		// initial state; both are immediately overwritten below.
		is := imap.GetOr(id, InputState{})
		savedOffset := is.CursorOffset
		savedTrailing := is.CursorTrailing
		is.CursorOffset = -1
		is.CursorTrailing = false
		text := inputTextFromLayout(layout)
		runeLen := utf8RuneCount(text)
		pos := is.CursorPos
		pos = min(pos, runeLen)
		isShift := e.Modifiers.Has(ModShift)
		isWordMod := e.Modifiers.HasAny(ModCtrl, ModAlt, ModSuper)
		handled := true
		textChanged := false

		// Use glyph layout for cursor navigation when available.
		gl, glOK := inputGlyphLayoutWithText(text, layout, w)

		switch e.KeyCode {
		case KeyLeft:
			inputKeyLeft(imap, id, is, text, pos,
				isShift, isWordMod, gl, glOK)
		case KeyRight:
			inputKeyRight(imap, id, is, text, pos, runeLen,
				isShift, isWordMod, gl, glOK)
		case KeyHome:
			inputKeyHome(imap, id, is, text, pos,
				isShift, savedTrailing, gl, glOK)
		case KeyEnd:
			inputKeyEnd(imap, id, is, text, pos,
				isShift, savedTrailing, gl, glOK)
		case KeyUp:
			handled = inputKeyVertical(imap, id, is, text, pos,
				isShift, savedOffset, true, hcfg.Mode, gl, glOK)
		case KeyDown:
			handled = inputKeyVertical(imap, id, is, text, pos,
				isShift, savedOffset, false, hcfg.Mode, gl, glOK)
		case KeyEnter:
			text, textChanged = inputKeyEnter(
				hcfg, layout, text, id, e, w)
		case KeyEscape:
			inputKeyEscape(imap, id, is)
			handled = false
		case KeyA:
			if e.Modifiers.HasAny(ModCtrl, ModSuper) {
				inputSelectAll(text, id, w)
			} else {
				handled = false
			}
		case KeyC:
			handled = inputKeyCopy(
				text, id, hcfg.IsPassword, e, w)
		case KeyV:
			if e.Modifiers.HasAny(ModCtrl, ModSuper) {
				text, textChanged = inputKeyPaste(
					text, w.GetClipboard(), id,
					mask, hcfg, w)
			} else {
				handled = false
			}
		case KeyX:
			text, textChanged, handled = inputKeyCut(
				text, id, hcfg.IsPassword, e, w)
		case KeyZ:
			text, textChanged, handled = inputKeyUndoRedo(
				text, id, e, w)
		case KeyBackspace:
			text, textChanged = inputKeyBackspaceOrDelete(
				text, id, false, mask, layout, w)
		case KeyDelete:
			text, textChanged = inputKeyBackspaceOrDelete(
				text, id, true, mask, layout, w)
		default:
			handled = false
		}

		if handled {
			resetBlinkCursorVisible(w)
			if textChanged {
				hcfg.fireTextChanged(layout, text, w)
			}
			inputScrollCursorIntoView(
				hcfg.ScrollID, text, layout, w,
			)
			e.IsHandled = true
		} else if hcfg.OnKeyDown != nil {
			hcfg.OnKeyDown(layout, e, w)
		}
	}
}

func makeInputOnKeyUp(hcfg inputHandlerCfg) func(*Layout, *Event, *Window) {
	return func(layout *Layout, e *Event, w *Window) {
		if hcfg.FocusID == "" || !w.IsFocus(hcfg.FocusID) {
			return
		}
		if hcfg.OnKeyUp != nil {
			hcfg.OnKeyUp(layout, e, w)
		}
	}
}

func inputKeyEnter(
	hcfg inputHandlerCfg, layout *Layout, text string,
	id string, e *Event, w *Window,
) (string, bool) {
	if hcfg.Mode == InputMultiline {
		return inputInsert(text, "\n", id, w), true
	}
	inputCommitEnter(hcfg, layout, text, e, w)
	return text, false
}

func inputKeyEscape(
	imap *BoundedMap[string, InputState], id string, is InputState,
) {
	is.SelectBeg = 0
	is.SelectEnd = 0
	imap.Set(id, is)
}

func inputKeyCopy(
	text string, id string, isPassword bool, e *Event, w *Window,
) bool {
	if !e.Modifiers.HasAny(ModCtrl, ModSuper) {
		return false
	}
	if copied, ok := inputCopy(text, id, isPassword, w); ok {
		w.SetClipboard(copied)
	}
	return true
}

func inputKeyCut(
	text string, id string, isPassword bool, e *Event, w *Window,
) (string, bool, bool) {
	if !e.Modifiers.HasAny(ModCtrl, ModSuper) {
		return text, false, false
	}
	newText, copied, ok := inputCut(text, id, isPassword, w)
	if ok {
		w.SetClipboard(copied)
		return newText, true, true
	}
	return text, false, true
}

func inputKeyUndoRedo(
	text string, id string, e *Event, w *Window,
) (string, bool, bool) {
	if !e.Modifiers.HasAny(ModCtrl, ModSuper) {
		return text, false, false
	}
	if e.Modifiers.Has(ModShift) {
		if nt := inputRedo(text, id, w); nt != text {
			return nt, true, true
		}
	} else {
		if nt := inputUndo(text, id, w); nt != text {
			return nt, true, true
		}
	}
	return text, false, true
}

func inputKeyBackspaceOrDelete(
	text string, id string, forward bool,
	mask *CompiledInputMask, layout *Layout, w *Window,
) (string, bool) {
	if newText, ok := inputHandleDelete(
		text, id, forward, mask, layout, w,
	); ok {
		return newText, true
	}
	return text, false
}
