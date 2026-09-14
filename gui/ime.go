package gui

// ime holds per-window Input Method Editor composition state.
type ime struct {
	compText   string
	compCursor int
	compSelLen int
	composing  bool
}

// maxIMEPreeditRunes bounds the preedit stored per window. The
// text arrives from an out-of-process input method with no length
// promised, and it is re-sliced every frame in the render path, so
// an unbounded composition is a memory and per-frame CPU blowup.
// 4096 runes matches the Win32 backend's maxIMECompChars cap on the
// UTF-16 read; a preedit is a phrase, far past any real one.
const maxIMEPreeditRunes = 4096

// maxIMECoord bounds a coordinate handed to the platform for
// candidate placement. float32-to-int32 conversion of an
// out-of-range value is implementation-defined, and a NaN from a
// degenerate style must not propagate into IMM or a D-Bus call.
// Backends with tighter ranges (Win32's int16 caret space) clamp
// further on their side.
const maxIMECoord = 1 << 30

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
	text := e.IMEText
	n := utf8RuneCount(text)
	if n > maxIMEPreeditRunes {
		text = text[:runeToByteIndex(text, maxIMEPreeditRunes)]
		n = maxIMEPreeditRunes
	}
	cursor := min(max(int(e.IMEStart), 0), n)
	w.ime.composing = true
	w.ime.compText = text
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

// imeCoord converts a logical coordinate to the int32 the platform
// takes, rounding to nearest instead of truncating. Non-finite
// values clamp: NaN lands on zero, infinities on the bound.
func imeCoord(v float32) int32 {
	if f32IsFinite(v) {
		if v > maxIMECoord {
			return maxIMECoord
		}
		if v < -maxIMECoord {
			return -maxIMECoord
		}
		if v >= 0 {
			return int32(v + 0.5)
		}
		return -int32(0.5 - v)
	}
	if v > 0 {
		return maxIMECoord
	}
	if v < 0 {
		return -maxIMECoord
	}
	return 0 // NaN
}

// IMESetRect reports the cursor rect to the platform so the
// candidate window positions correctly. No-ops if no backend.
func (w *Window) IMESetRect(x, y, width, height float32) {
	if np := w.nativePlatform; np != nil {
		np.IMESetRect(
			imeCoord(x), imeCoord(y),
			imeCoord(width), imeCoord(height),
		)
	}
}
