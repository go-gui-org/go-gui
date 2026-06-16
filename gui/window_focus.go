package gui

import "time"

// window_focus.go — keyboard focus management.

// IDFocus returns the current focus ID.
func (w *Window) IDFocus() uint32 {
	return w.viewState.idFocus
}

// SetIDFocus sets the focus id and clears input selections.
// Acquires both w.mu (idFocus) and w.animMu (animations).
func (w *Window) SetIDFocus(id uint32) {
	w.mu.Lock()
	w.animMu.Lock()
	defer w.animMu.Unlock()
	defer w.mu.Unlock()
	w.setIDFocusLocked(id)
}

func (w *Window) setIDFocusLocked(id uint32) {
	prev := w.viewState.idFocus
	w.clearInputSelections()
	w.imeClear()
	w.viewState.idFocus = id
	if id > 0 {
		w.viewState.inputCursorOn.Store(true)
		if !w.hasAnimationLocked(blinkCursorAnimationID) {
			w.animationAddLocked(NewBlinkCursorAnimation())
		}
	}
	if np := w.nativePlatform; np != nil {
		if prev > 0 && id != prev {
			np.IMEStop()
		}
		if id > 0 {
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

// IsFocus tests if the given id_focus equals the window's id_focus.
func (w *Window) IsFocus(idFocus uint32) bool {
	return w.viewState.idFocus > 0 && w.viewState.idFocus == idFocus
}

// hasFocus returns true if the window has focus.
func (w *Window) hasFocus() bool {
	return w.focused
}
