package gui

import "time"

// window_focus.go — keyboard focus management.

// FocusID returns the current focus ID.
func (w *Window) FocusID() string {
	return w.viewState.focusID
}

// SetFocus sets the focused widget by its string ID. A real focus
// change clears input selections window-wide; re-asserting the widget
// that already holds focus leaves selections alone. Acquires both w.mu
// (focusID) and w.animMu (animations). Use ClearFocus to remove focus.
func (w *Window) SetFocus(id string) {
	w.mu.Lock()
	w.animMu.Lock()
	defer w.animMu.Unlock()
	defer w.mu.Unlock()
	w.setFocusLocked(id)
}

// ClearFocus removes keyboard focus from any widget.
func (w *Window) ClearFocus() {
	w.SetFocus("")
}

func (w *Window) setFocusLocked(id string) {
	prev := w.viewState.focusID
	// Only a real focus *change* drops selections and clears the IME.
	// Re-asserting focus on the widget that already holds it must leave
	// an in-flight IME composition alone — see #156 — and, since #277,
	// text selections too: consumers legitimately re-assert focus from
	// inside their View function, which runs on every layout rebuild,
	// so an unconditional selection clear wiped every nsInput selection
	// window-wide on each re-assert. Real transitions still clear both.
	if id != prev {
		w.clearInputSelections()
		w.imeClear()
	}
	w.viewState.focusID = id
	if id != "" {
		w.viewState.inputCursorOn.Store(true)
		if !w.hasAnimationLocked(blinkCursorAnimationID) {
			w.animationAddLocked(newBlinkCursorAnimation())
		}
	}
	// The platform IME is not switched here. Whether the new widget is
	// an *editable text* context — the only thing an input method may
	// be activated for — cannot be answered from an ID, and SetFocus is
	// legitimately called from inside a View function, where w.layout
	// still holds the previous frame and a newly created input is not
	// in it yet. syncIMEEditContext decides it from the arranged tree
	// each frame instead (gui/ime_context.go, issue #393).
}

// resetBlinkCursorVisible resets the blink timer so the cursor
// stays visible during typing and cursor movement.
func resetBlinkCursorVisible(w *Window) {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	w.viewState.inputCursorOn.Store(true)
	if a, ok := w.animations[blinkCursorAnimationID]; ok {
		a.SetStart(time.Now())
	}
}

// IsFocus tests if the given focus id equals the window's focus id.
func (w *Window) IsFocus(id string) bool {
	return w.viewState.focusID != "" && w.viewState.focusID == id
}

// hasFocus returns true if the window has focus.
func (w *Window) hasFocus() bool {
	return w.focused
}
