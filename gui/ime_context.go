package gui

// ime_context.go — deciding when the platform input method is live,
// and when the caret-blink animation runs.
//
// The input method must be active only while an editable text widget
// holds focus. It is not a "something is focused" signal: on macOS an
// active input context routes every keystroke through
// interpretKeyEvents:, which turns Option+I into a dead circumflex
// instead of the ModAlt shortcut the app asked for (issue #393). The
// Windows backend documents the same contract it was never given
// ("composition only becomes possible once a text widget takes
// focus", gui/backend/gl/ime_win32.go), and on X11 IMEStart is an
// ibus FocusIn that a button has no business claiming.
//
// The caret-blink animation is gated the same way (issue #403): it
// must run only while a widget that renders a framework caret holds
// focus, or the window re-renders every 600 ms for a caret nobody
// draws.

// shapeIsIMEEditTarget reports whether shape is the editable text
// context an input method would write into — the shape that hosts the
// preedit in renderText.
//
// focusOwner is the discriminator: only input widgets set it
// (view_input.go), so a selection-only focusable Text is excluded. A
// read-only input is excluded too: it stays focusable for its caret
// and selection but can never commit a composition, which is the same
// rule render_text.go applies to preedit rendering.
//
// A focusable terminal grid is an edit target as well — it consumes
// typed text directly and has no Text shape of its own.
func shapeIsIMEEditTarget(s *Shape) bool {
	if s == nil {
		return false
	}
	if s.shapeType == shapeTermGrid {
		return s.Focusable
	}
	return s.TC != nil && s.focusOwner != "" && !s.TC.textReadOnly
}

// shapeDrawsCaret reports whether shape can render the framework's
// input caret (renderInputCursor): a text-like shape that renders
// focus state for itself or for a widget it belongs to. It is wider
// than shapeIsIMEEditTarget on purpose (issue #403):
//
//   - a read-only input stays focusable for its caret and selection,
//     so its caret blinks; only the IME gate excludes it.
//   - a selection-only focusable Text draws a caret at its cursor,
//     so it blinks like an input.
//   - a terminal grid is NOT a caret target: its cursor is the
//     consumer's own TermCursor.Visible flag, which the blink
//     animation does not drive.
func shapeDrawsCaret(s *Shape) bool {
	return s != nil && s.TC != nil && s.rendersFocusState()
}

// findEditTargets reports both focus signals — whether the focused
// widget draws a framework caret (the blink gate) and whether it is
// an editable IME context — in one walk. The walk short-circuits as
// soon as both are found and is skipped entirely when nothing is
// focused, so each of its two callers (syncIMEEditContext and
// syncBlinkCursor) pays one shallow probe per frame with a focused
// widget, and nothing when nothing is focused. A focused widget can
// match through two shapes (Input's container and its inner text
// shape), so once one signal is found the walk keeps going for the
// other.
func findEditTargets(layout *Layout, w *Window) (caret, ime bool) {
	if layout.Shape == nil || w.viewState.focusID == "" {
		return false, false
	}
	if w.IsFocus(layout.Shape.focusKey()) {
		if shapeDrawsCaret(layout.Shape) {
			caret = true
		}
		if shapeIsIMEEditTarget(layout.Shape) {
			ime = true
		}
		if caret && ime {
			return true, true
		}
	}
	for i := range layout.Children {
		c, e := findEditTargets(&layout.Children[i], w)
		caret = caret || c
		ime = ime || e
		if caret && ime {
			return true, true
		}
	}
	return caret, ime
}

// syncIMEEditContext starts or stops the platform input method as the
// focused widget gains or loses an editable text context.
//
// Called from the render pass rather than from setFocusLocked: the ID
// alone cannot say whether the widget is editable, and SetFocus is
// legitimately called from inside a View function, where the tree in
// hand is the previous frame's and a newly created input is not in it
// yet. Talking to the platform from the render pass has precedent —
// renderText reports the caret rect the same way.
//
// Only transitions are pushed, so re-asserting focus on the widget
// that already holds it does not re-activate the input method (the
// invariant issue #156 established). Moving between two text fields
// does cycle it: a composition still live inside the engine belongs
// to the field being left, and IMEStop is what cancels it — an ibus
// FocusOut, the IMM context detach, the web hidden input being
// removed.
func (w *Window) syncIMEEditContext() {
	_, ime := findEditTargets(&w.layout, w)
	editing := w.viewState.focusID != "" && ime
	id := ""
	if editing {
		id = w.viewState.focusID
	}
	if editing == w.viewState.imeEditContext &&
		id == w.viewState.imeEditFocusID {
		return
	}
	wasEditing := w.viewState.imeEditContext
	w.viewState.imeEditContext = editing
	w.viewState.imeEditFocusID = id

	np := w.nativePlatform
	if np == nil {
		return
	}
	if wasEditing {
		np.IMEStop()
	}
	if editing {
		np.IMEStart()
	}
}

// syncBlinkCursor starts or stops the caret-blink animation as the
// focused widget gains or loses a framework caret (issue #403).
//
// Called from the render pass for the same reason syncIMEEditContext
// is: setFocusLocked only sees an ID, which cannot say whether the
// widget draws a caret, and SetFocus is legitimately called from
// inside a View function, where the tree in hand is the previous
// frame's and a newly created input is not in it yet. Registering
// here also stops consumers re-asserting focus from View — the
// pattern setFocusLocked's old code re-armed on every layout build —
// from keeping a perpetual blink animation for a widget that draws
// no caret.
//
// A Pulsar registers the blink animation itself and toggles on the
// same inputCursorOn state, so its registration is never removed
// here: it blinks without any focused input.
func (w *Window) syncBlinkCursor() {
	caret, _ := findEditTargets(&w.layout, w)
	w.animMu.Lock()
	defer w.animMu.Unlock()
	_, present := w.animations[blinkCursorAnimationID]
	if caret && !present {
		w.animationAddLocked(newBlinkCursorAnimation())
		return
	}
	if !caret && present && !w.hasAnimationLocked(pulsarAnimationID) {
		delete(w.animations, blinkCursorAnimationID)
		delete(w.animViewBound, blinkCursorAnimationID)
	}
}
