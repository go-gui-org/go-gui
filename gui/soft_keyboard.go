package gui

// soft_keyboard.go — the OS soft (on-screen) keyboard (issue #770).
//
// Go-GUI does not draw a general soft keyboard. It tells the platform
// what kind of keyboard the focused field wants, and when. The request
// rides the same edit-context gate as the input method
// (applyIMEEditContext in ime_context.go): the keyboard is asked for
// only while an editable text field holds focus, and it is dismissed
// when that ends. An app never has to call ShowSoftKeyboard for the
// normal case.
//
// A custom keypad (kiosk, PIN pad) is ordinary widgets: set the field's
// Keyboard to KeyboardNone so the OS keyboard stays down, and make the
// keys FocusDisabled buttons. A press that a non-focusable widget
// consumes keeps focus where it is (blurUnclaimedPress), so the field
// keeps its caret while the keys type into it.

// KeyboardKind names the soft-keyboard layout a text field asks the
// platform for. It is a hint: a platform without a matching layout
// shows its default text keyboard, and a platform without a soft
// keyboard (desktop) ignores it. It never restricts what can be typed
// — use Mask or PreTextChange for that.
type KeyboardKind uint8

// KeyboardKind values. The zero value is KeyboardText, so an Input that
// does not set Keyboard behaves as before.
const (
	// KeyboardText is the default text keyboard.
	KeyboardText KeyboardKind = iota
	// KeyboardNumber is a digit pad.
	KeyboardNumber
	// KeyboardDecimal is a digit pad with a decimal separator.
	KeyboardDecimal
	// KeyboardPhone is a telephone pad (digits, +, *, #).
	KeyboardPhone
	// KeyboardEmail is a text keyboard with @ and . at hand.
	KeyboardEmail
	// KeyboardURL is a text keyboard with / and . at hand.
	KeyboardURL
	// KeyboardNone keeps the OS soft keyboard hidden while the field
	// is focused. The input method stays live, so a physical keyboard
	// and an app-drawn keypad both still type into the field.
	KeyboardNone
)

// String returns the constant's name without the Keyboard prefix, for
// logs and test failures.
func (k KeyboardKind) String() string {
	switch k {
	case KeyboardText:
		return "Text"
	case KeyboardNumber:
		return "Number"
	case KeyboardDecimal:
		return "Decimal"
	case KeyboardPhone:
		return "Phone"
	case KeyboardEmail:
		return "Email"
	case KeyboardURL:
		return "URL"
	case KeyboardNone:
		return "None"
	}
	return "Unknown"
}

// ShowSoftKeyboard asks the platform to show the soft keyboard for the
// focused text field again, with that field's KeyboardKind. Use it after
// HideSoftKeyboard. The framework already shows the keyboard when a
// field takes focus, and when the user taps the focused field, so most
// apps never call this.
//
// Does nothing when no editable text field holds focus, or on a
// platform without a soft keyboard. Call it from an event callback or a
// QueueCommand, not from a View function or AmendLayout.
func (w *Window) ShowSoftKeyboard() {
	w.lockForAPI("ShowSoftKeyboard")
	defer w.mu.Unlock()
	w.showSoftKeyboard()
}

// HideSoftKeyboard asks the platform to dismiss the soft keyboard. The
// focused field keeps its focus and caret, so a physical keyboard or an
// app keypad can still type into it. To also leave the field, use
// ClearFocus: the keyboard then goes down on its own.
//
// Does nothing when no editable text field holds focus, or on a
// platform without a soft keyboard. Call it from an event callback or a
// QueueCommand, not from a View function or AmendLayout.
func (w *Window) HideSoftKeyboard() {
	w.lockForAPI("HideSoftKeyboard")
	defer w.mu.Unlock()
	if w.nativePlatform == nil || !w.viewState.imeEditContext {
		return
	}
	w.nativePlatform.HideSoftKeyboard()
}

// SoftKeyboardInset returns how many logical pixels at the bottom of the
// window the soft keyboard covers. It is 0 while the keyboard is hidden,
// and on platforms that cannot report it.
//
// The framework does not move layout for the keyboard. Read the inset
// in the View function (for example as bottom padding, or to scroll the
// focused field into view) if the app needs its content above the
// keyboard. The window re-renders when the inset changes, and the
// change also reaches Window.OnEvent as an EventSoftKeyboard.
func (w *Window) SoftKeyboardInset() float32 {
	return w.softKeyboardInset
}

// showSoftKeyboard re-requests the keyboard for the current edit
// target. Caller holds w.mu or runs in EventFn.
func (w *Window) showSoftKeyboard() {
	if w.nativePlatform == nil || !w.viewState.imeEditContext {
		return
	}
	w.nativePlatform.ShowSoftKeyboard(
		w.viewState.imeEditKind, w.viewState.imeEditSecure)
}

// handleSoftKeyboardEvent records the inset a backend reports. The
// value comes from the OS, so it is clamped before any View reads it:
// a negative, NaN or infinite inset becomes 0, and an inset taller than
// the window becomes the window height.
func (w *Window) handleSoftKeyboardEvent(e *Event) {
	inset := e.SoftKeyboardInset
	if !f32IsFinite(inset) || inset < 0 {
		inset = 0
	}
	if h := float32(w.windowHeight); h > 0 && inset > h {
		inset = h
	}
	w.softKeyboardInset = inset
}

// reshowSoftKeyboardOnPress re-opens the soft keyboard when a press
// lands on the field that already held focus before the press. The
// user may have dismissed the keyboard (Android Back) with the field
// still focused; the focus did not change, so the edit-context gate
// sees no transition and would never ask again. A press that moves
// focus into a field is left to the gate.
//
// Runs after the press walk. focusBefore is FocusID from before it. A
// press that a drag-to-scroll claim holds is skipped: it may become a
// pan, and the tap replay runs this again (dragPanOnMouseUp).
func (w *Window) reshowSoftKeyboardOnPress(layout *Layout, e *Event, focusBefore string) {
	if e.MouseButton == MouseRight || focusBefore == "" || layout == nil ||
		w.viewState.dragPan.active ||
		!w.viewState.imeEditContext ||
		w.viewState.imeEditFocusID != focusBefore ||
		w.FocusID() != focusBefore {
		return
	}
	// Only a press on the field itself. A FocusDisabled keypad key
	// keeps focus on the field too, and must not pop the OS keyboard
	// over the keypad.
	ly, ok := findLayoutByFocusID(layout, focusBefore)
	if !ok || !ly.Shape.PointInShape(e.MouseX, e.MouseY) {
		return
	}
	w.showSoftKeyboard()
}
