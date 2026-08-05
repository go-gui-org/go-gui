package gui

// ime holds per-window Input Method Editor composition state.
type ime struct {
	compText   string
	compCursor int
	compSelLen int
	composing  bool
}

// imeUpdate sets composition state from an IME composition event.
//
// The offsets are clamped here rather than in each backend: they cross
// a platform boundary (Cocoa's NSRange, the Android bridge's int64,
// the browser's composition events), a hostile or merely confused
// input method can report anything, and the values feed slice
// arithmetic in the render path, which runs every frame.
func (w *Window) imeUpdate(e *Event) {
	if len(e.IMEText) == 0 {
		w.imeClear()
		return
	}
	n := utf8RuneCount(e.IMEText)
	cursor := min(max(int(e.IMEStart), 0), n)
	w.ime.composing = true
	w.ime.compText = e.IMEText
	w.ime.compCursor = cursor
	w.ime.compSelLen = min(max(int(e.IMELength), 0), n-cursor)
}

// imeClear resets composition state (called on commit or focus
// change).
func (w *Window) imeClear() {
	w.ime.composing = false
	w.ime.compText = ""
	w.ime.compCursor = 0
	w.ime.compSelLen = 0
}

// IMEComposing returns true if an IME composition is in progress.
func (w *Window) IMEComposing() bool {
	return w.ime.composing
}

// IMECompText returns the current IME preedit string.
func (w *Window) IMECompText() string {
	return w.ime.compText
}

// IMECompCursor returns the start of the selected clause within the
// preedit string (in characters), or the cursor offset when no clause
// is selected. See Event.IMEStart.
func (w *Window) IMECompCursor() int {
	return w.ime.compCursor
}

// IMECompSelLen returns the length of the selected clause within
// the preedit string (in characters). Zero means no clause is
// selected.
func (w *Window) IMECompSelLen() int {
	return w.ime.compSelLen
}

// IMESetRect reports the cursor rect to the platform so the
// candidate window positions correctly. No-ops if no backend.
func (w *Window) IMESetRect(x, y, width, height float32) {
	if np := w.nativePlatform; np != nil {
		np.IMESetRect(
			int32(x), int32(y), int32(width), int32(height),
		)
	}
}
