package gui

// SetClipboardFn sets the function used to copy text to the clipboard.
func (w *Window) SetClipboardFn(fn func(string)) {
	w.clipboardSetFn = fn
}

// SetClipboard copies text to the system clipboard.
func (w *Window) SetClipboard(text string) {
	if w.clipboardSetFn != nil {
		w.clipboardSetFn(text)
	}
}

// SetClipboardGetFn sets the function used to read from the clipboard.
func (w *Window) SetClipboardGetFn(fn func() string) {
	w.clipboardGetFn = fn
}

// GetClipboard returns text from the system clipboard.
func (w *Window) GetClipboard() string {
	if w.clipboardGetFn != nil {
		return w.clipboardGetFn()
	}
	return ""
}

// SetPrimaryFn sets the function used to publish text as the PRIMARY
// selection.
func (w *Window) SetPrimaryFn(fn func(string)) {
	w.primarySetFn = fn
}

// SetPrimary publishes text as the X11 PRIMARY selection — the buffer filled
// by selecting text and pasted with the middle mouse button. It is entirely
// separate from the clipboard, so SetPrimary does not disturb whatever
// SetClipboard last stored. A no-op on platforms without PRIMARY (macOS,
// Windows, web, mobile).
func (w *Window) SetPrimary(text string) {
	if w.primarySetFn != nil {
		w.primarySetFn(text)
	}
}

// SetPrimaryGetFn sets the function used to read the PRIMARY selection.
func (w *Window) SetPrimaryGetFn(fn func() string) {
	w.primaryGetFn = fn
}

// GetPrimary returns the current X11 PRIMARY selection, or "" on platforms
// that have no such concept. Callers wanting a paste that works everywhere
// should fall back to GetClipboard on an empty result.
func (w *Window) GetPrimary() string {
	if w.primaryGetFn != nil {
		return w.primaryGetFn()
	}
	return ""
}
