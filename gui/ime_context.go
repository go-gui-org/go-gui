package gui

// ime_context.go — deciding when the platform input method is live.
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

// findIMEEditTarget reports whether the focused widget's edit target
// is somewhere in this subtree. The walk short-circuits on the first
// match and is skipped entirely when nothing is focused, so the cost
// is one partial tree walk per frame with a focused widget.
func findIMEEditTarget(layout *Layout, w *Window) bool {
	if layout.Shape == nil {
		return false
	}
	if shapeIsIMEEditTarget(layout.Shape) &&
		w.IsFocus(layout.Shape.focusKey()) {
		return true
	}
	for i := range layout.Children {
		if findIMEEditTarget(&layout.Children[i], w) {
			return true
		}
	}
	return false
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
	editing := w.viewState.focusID != "" && findIMEEditTarget(&w.layout, w)
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
