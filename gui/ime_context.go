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
//
// It is gated on the *window* holding OS focus as well. A background
// window receives no key events, so its caret marks an insertion point
// nobody can type into — and the blink is not free: it keeps the 16 ms
// animation ticker alive and wakes the main thread out of its blocking
// event wait twice a second for the whole time the app sits idle.

// shapeIsIMEEditTarget reports whether shape is the editable text
// context an input method would write into — the shape that hosts the
// preedit in renderText.
//
// focusOwner is the discriminator: only input widgets set it
// (view_input.go), so a selection-only focusable Text is excluded. A
// read-only input is excluded too: it stays focusable for its caret
// and selection but can never commit a composition, which is the same
// rule render_text.go applies to preedit rendering.
func shapeIsIMEEditTarget(s *Shape) bool {
	if s == nil {
		return false
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
func shapeDrawsCaret(s *Shape) bool {
	return s != nil && s.TC != nil && s.rendersFocusState()
}

// findEditTargets reports both focus signals — whether the focused
// widget draws a framework caret (the blink gate) and the shape of its
// editable IME context, nil when it has none — in one walk. See
// findEditTargetsIn.
func findEditTargets(layout *Layout, w *Window, depth int) (caret bool, edit *Shape) {
	// Loaded once: focus cannot change mid-walk on the frame
	// goroutine, and IsFocus would reload the same atomic at
	// every node otherwise.
	return findEditTargetsIn(layout, w.FocusID(), depth)
}

// findEditTargetsIn resolves both focus signals against an already
// loaded focus ID. The walk short-circuits as soon as both are found
// and is skipped entirely when nothing is focused, so a frame with a
// focused widget pays one shallow probe and nothing when nothing is
// focused. A focused widget can match through two shapes (Input's
// container and its inner text shape), so once one signal is found
// the walk keeps going for the other. Depth is capped like every
// other tree walk: the tree is not always the app's own, and past
// maxEventDepth the frame drops input rather than the process.
func findEditTargetsIn(layout *Layout, focusID string, depth int) (caret bool, edit *Shape) {
	if layout == nil || layout.Shape == nil || focusID == "" {
		return false, nil
	}
	if overMaxDepth(depth) {
		return false, nil
	}
	if focusID == layout.Shape.focusKey() {
		if shapeDrawsCaret(layout.Shape) {
			caret = true
		}
		if shapeIsIMEEditTarget(layout.Shape) {
			edit = layout.Shape
		}
		if caret && edit != nil {
			return caret, edit
		}
	}
	for i := range layout.Children {
		c, e := findEditTargetsIn(&layout.Children[i], focusID, depth+1)
		caret = caret || c
		if edit == nil {
			edit = e
		}
		if caret && edit != nil {
			return caret, edit
		}
	}
	return caret, edit
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
	_, edit := findEditTargets(&w.layout, w, 0)
	w.applyIMEEditContext(edit)
}

// applyIMEEditContext pushes an edit-context transition to the
// platform. edit is the focused editable text shape, nil when there is
// none. Split from the walk so the combined per-frame sync can share
// one walk between both gates.
//
// The soft keyboard (issue #770) rides the same transitions: it is
// shown right after IMEStart, with the field's kind, and hidden right
// before IMEStop. Moving between two fields therefore hides and shows;
// the Android backend keeps only the last request per frame, so the
// keyboard does not flicker.
func (w *Window) applyIMEEditContext(edit *Shape) {
	focusID := w.FocusID()
	editing := focusID != "" && edit != nil
	id := ""
	var kind KeyboardKind
	var secure bool
	if editing {
		id = focusID
		kind = edit.TC.textKeyboard
		secure = edit.TC.textIsPassword
	}
	if editing == w.viewState.imeEditContext &&
		id == w.viewState.imeEditFocusID {
		// Same field. The app may have changed its Keyboard or
		// IsPassword since the last frame: ask again for the new
		// layout, but leave the input method alone (#156).
		if editing && (kind != w.viewState.imeEditKind ||
			secure != w.viewState.imeEditSecure) {
			w.viewState.imeEditKind = kind
			w.viewState.imeEditSecure = secure
			w.showSoftKeyboard()
		}
		return
	}
	wasEditing := w.viewState.imeEditContext
	w.viewState.imeEditContext = editing
	w.viewState.imeEditFocusID = id
	w.viewState.imeEditKind = kind
	w.viewState.imeEditSecure = secure

	np := w.nativePlatform
	if np == nil {
		return
	}
	if wasEditing {
		np.HideSoftKeyboard()
		np.IMEStop()
	}
	if editing {
		np.IMEStart()
		np.ShowSoftKeyboard(kind, secure)
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
	caret, _ := findEditTargets(&w.layout, w, 0)
	w.applyBlinkCursor(caret)
}

// applyBlinkCursor registers or retires the caret-blink animation
// for a caret gate already resolved by a tree walk.
func (w *Window) applyBlinkCursor(caret bool) {
	// An unfocused window gets no key events, so a blinking caret is a
	// pure idle wakeup — a 16 ms ticker plus a wakeMain twice a second
	// for a caret the user cannot type into. Dropping the animation
	// empties w.animations, which parks the animation ticker outright
	// (animationLoop). renderInputCursor hides the caret to match.
	caret = caret && w.hasFocus()
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

// syncIMECaretState runs both per-frame focus gates — the platform
// input method and the caret-blink animation — off one tree walk.
// The frame calls this instead of the two syncs above.
func (w *Window) syncIMECaretState() {
	caret, edit := findEditTargets(&w.layout, w, 0)
	w.applyIMEEditContext(edit)
	w.applyBlinkCursor(caret)
}
