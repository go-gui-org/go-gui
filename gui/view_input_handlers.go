package gui

// Input event handlers: the OnChar/OnKeyDown/OnKeyUp closures Input
// installs, the text-mutation helper they share, and the per-key
// helpers they dispatch to. Split from view_input.go to keep that file
// under the 800-line gate (scripts/large-files.sh).

func inputTextChange(hcfg inputHandlerCfg, layout *Layout, text, ins string, id string, w *Window) (string, bool) {
	mask := hcfg.CompiledMask
	if mask != nil {
		is := inputStateOrDefault(id, w)
		res := inputMaskInsert(text, is.CursorPos, is.selectBeg, is.selectEnd, ins, mask)
		if res.Changed {
			// IME commits are multi-rune; they break the run the way
			// inputInsert does for a paste. Only the ">1 rune"
			// question matters, and ins is caller-sized input that
			// inputMaskInsert bounds, so count with an early exit.
			n := 0
			for range ins {
				n++
				if n > 1 {
					break
				}
			}
			op := inputOpInsert
			if n > 1 {
				op = inputOpNone
			}
			undo := inputPushUndo(is, text, op)
			text = res.Text
			StateMap[string, inputState](w, nsInput, capMany).Set(id, inputState{
				CursorPos: res.CursorPos, Undo: undo, lastEditOp: op,
			})
			return text, true
		}
	} else if hcfg.preTextChange != nil {
		proposed := inputProposedText(text, ins, id, w)
		if adjusted, ok := hcfg.preTextChange(text, proposed); ok {
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

func makeInputOnChar(hcfg inputHandlerCfg) func(EventCtx) {
	return func(ctx EventCtx) {
		// The captured IDs are leaves (Input builds its tree with no
		// Window in hand); ctx.EffID turns them into the identities the
		// focus and state stores hold.
		id := ctx.EffID(hcfg.FocusID)
		if id == "" || !ctx.Window.IsFocus(id) {
			// Not our field: let the character travel on
			return
		}
		// Swallow typed and IME-composed text; the field keeps focus so
		// navigation, selection, and copy still work.
		if hcfg.ReadOnly {
			ctx.Consume()
			return
		}
		ch := ctx.Event.CharCode

		// Control characters are handled by OnKeyDown.
		if ch < charSpace {
			ctx.Consume()
			return
		}

		text := inputTextFromLayout(ctx.Layout)
		ins := ctx.Event.IMEText
		if len(ins) == 0 {
			ins = string(rune(ch))
		}
		text, changed := inputTextChange(hcfg, ctx.Layout, text, ins, id, ctx.Window)

		if changed {
			resetBlinkCursorVisible(ctx.Window)
			hcfg.fireTextChanged(ctx.Layout, text, ctx.Window)
			inputScrollCursorIntoView(
				ctx.EffID(hcfg.scrollID), text, ctx.Layout, ctx.Window,
			)
		}
		ctx.Consume()
	}
}

// inputKeyMutatesText reports whether a key event would change the
// input's text. Read-only fields swallow these while navigation
// (arrows/Home/End), selection (Shift, Ctrl+A), and copy (Ctrl+C) stay
// live. Cut/undo/redo only mutate with a Ctrl/Super modifier; without
// one their handlers decline the key, so it must not be swallowed here.
func inputKeyMutatesText(e *Event, mode inputMode) bool {
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

func makeInputOnKeyDown(hcfg inputHandlerCfg) func(EventCtx) {
	mask := hcfg.CompiledMask
	return func(ctx EventCtx) {
		id := ctx.EffID(hcfg.FocusID)
		if id == "" || !ctx.Window.IsFocus(id) {
			return
		}
		if hcfg.ReadOnly && inputKeyMutatesText(ctx.Event, hcfg.Mode) {
			ctx.Consume()
			return
		}
		imap := StateMap[string, inputState](ctx.Window, nsInput, capMany)
		// Default InputState{}: zero CursorOffset/CursorTrailing seed
		// initial state; both are immediately overwritten below.
		is := imap.GetOr(id, inputState{})
		savedOffset := is.cursorOffset
		savedTrailing := is.cursorTrailing
		is.cursorOffset = -1
		is.cursorTrailing = false
		text := inputTextFromLayout(ctx.Layout)
		runeLen := utf8RuneCount(text)
		pos := is.CursorPos
		pos = min(pos, runeLen)
		isShift := ctx.Event.Modifiers.Has(ModShift)
		isWordMod := ctx.Event.Modifiers.HasAny(ModCtrl, ModAlt, ModSuper)
		handled := true
		textChanged := false

		// Use glyph layout for cursor navigation when available.
		gl, glOK := inputGlyphLayoutWithText(text, ctx.Layout, ctx.Window)

		switch ctx.Event.KeyCode {
		case KeyLeft:
			inputKeyLeft(imap, id, is, text, pos,
				isShift, isWordMod, gl, glOK)
		case KeyRight:
			inputKeyRight(imap, id, is, text, pos,
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
				hcfg, ctx.Layout, text, id, ctx.Event, ctx.Window)
		case KeyEscape:
			inputKeyEscape(imap, id, is)
			handled = false
		case KeyA:
			if ctx.Event.Modifiers.HasAny(ModCtrl, ModSuper) {
				inputSelectAll(text, id, ctx.Window)
			} else {
				handled = false
			}
		case KeyC:
			handled = inputKeyCopy(
				text, id, hcfg.IsPassword, ctx.Event, ctx.Window)
		case KeyV:
			if ctx.Event.Modifiers.HasAny(ModCtrl, ModSuper) {
				text, textChanged = inputKeyPaste(
					text, ctx.Window.GetClipboard(), id,
					mask, hcfg, ctx.Window)
			} else {
				handled = false
			}
		case KeyX:
			text, textChanged, handled = inputKeyCut(
				text, id, hcfg.IsPassword, ctx.Event, ctx.Window)
		case KeyZ:
			text, textChanged, handled = inputKeyUndoRedo(
				text, id, ctx.Event, ctx.Window)
		case KeyBackspace:
			text, textChanged = inputKeyBackspaceOrDelete(
				text, id, false, mask, ctx.Layout, ctx.Window)
		case KeyDelete:
			text, textChanged = inputKeyBackspaceOrDelete(
				text, id, true, mask, ctx.Layout, ctx.Window)
		default:
			handled = false
		}

		if handled {
			resetBlinkCursorVisible(ctx.Window)
			if textChanged {
				hcfg.fireTextChanged(ctx.Layout, text, ctx.Window)
			}
			inputScrollCursorIntoView(
				ctx.EffID(hcfg.scrollID), text, ctx.Layout, ctx.Window,
			)
			ctx.Consume()
		} else if hcfg.OnKeyDown != nil {
			hcfg.OnKeyDown(ctx)
		}
	}
}

func makeInputOnKeyUp(hcfg inputHandlerCfg) func(EventCtx) {
	return func(ctx EventCtx) {
		id := ctx.EffID(hcfg.FocusID)
		if id == "" || !ctx.Window.IsFocus(id) {
			return
		}
		if hcfg.OnKeyUp != nil {
			hcfg.OnKeyUp(ctx)
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
	imap *BoundedMap[string, inputState], id string, is inputState,
) {
	is.selectBeg = 0
	is.selectEnd = 0
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
