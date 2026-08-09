package gui

import "time"

// window_focus.go — keyboard focus management.

// FocusID returns the current focus ID.
func (w *Window) FocusID() string {
	return w.viewState.focusID
}

// SetFocus sets the focused widget by its string ID and clears
// input selections. Acquires both w.mu (focusID) and w.animMu
// (animations). Use ClearFocus to remove focus.
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
	w.clearInputSelections()
	// Re-focusing the widget that already has focus must leave an
	// in-flight IME composition alone. Consumers legitimately re-assert
	// focus from inside their View function, which runs on every layout
	// rebuild — i.e. after every keystroke — so an unconditional clear
	// here wiped the preedit between each composition update, and the
	// IMEStart below re-activated the platform input context
	// mid-composition. See go-gui-org/go-gui#156.
	if id != prev {
		w.imeClear()
	}
	w.viewState.focusID = id
	if id != "" {
		w.viewState.inputCursorOn.Store(true)
		if !w.hasAnimationLocked(blinkCursorAnimationID) {
			w.animationAddLocked(newBlinkCursorAnimation())
		}
	}
	// Same reasoning: only a real focus *change* touches the platform
	// IME. prev != "" alone suffices for the stop — the enclosing
	// id != prev already rules out a no-op transition.
	if np := w.nativePlatform; np != nil && id != prev {
		if prev != "" {
			np.IMEStop()
		}
		if id != "" {
			np.IMEStart()
		}
	}
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
