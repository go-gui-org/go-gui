package gui

import (
	"os"
	"sync"
)

// GOGUI_EMULATE_CLIPBOARD=1 keeps the clipboard and the PRIMARY selection
// inside each window, so tests and manual runs do not change the system
// clipboard. GOGUI_CLIPBOARD_TEXT is the text the emulated clipboard holds
// at the start (#971).
const (
	emulateClipboardEnv = "GOGUI_EMULATE_CLIPBOARD"
	clipboardTextEnv    = "GOGUI_CLIPBOARD_TEXT"
)

// emulatedClipboard holds a window's clipboard and PRIMARY text when
// GOGUI_EMULATE_CLIPBOARD is set. The Window methods use it before the
// backend functions. So the order in which a backend installs its
// functions does not matter: they are stored but never called.
type emulatedClipboard struct {
	// mu guards both buffers. SetClipboard has no frame-lock contract,
	// so app code can call it from any goroutine.
	mu        sync.Mutex
	clipboard string
	primary   string
}

// emulatedClipboardFromEnv returns the buffers for a new window, or nil
// when emulation is off.
func emulatedClipboardFromEnv() *emulatedClipboard {
	if !envTruthy(emulateClipboardEnv) {
		return nil
	}
	return &emulatedClipboard{clipboard: os.Getenv(clipboardTextEnv)}
}

func (c *emulatedClipboard) set(primary bool, text string) {
	c.mu.Lock()
	if primary {
		c.primary = text
	} else {
		c.clipboard = text
	}
	c.mu.Unlock()
}

func (c *emulatedClipboard) get(primary bool) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if primary {
		return c.primary
	}
	return c.clipboard
}

// SetClipboardFn sets the function used to copy text to the clipboard.
func (w *Window) SetClipboardFn(fn func(string)) {
	w.clipboardSetFn = fn
}

// SetClipboard copies text to the system clipboard.
func (w *Window) SetClipboard(text string) {
	if w.clipEmu != nil {
		w.clipEmu.set(false, text)
		return
	}
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
	if w.clipEmu != nil {
		return w.clipEmu.get(false)
	}
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
	if w.clipEmu != nil {
		w.clipEmu.set(true, text)
		return
	}
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
	if w.clipEmu != nil {
		return w.clipEmu.get(true)
	}
	if w.primaryGetFn != nil {
		return w.primaryGetFn()
	}
	return ""
}
