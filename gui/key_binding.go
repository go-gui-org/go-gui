package gui

import (
	"os"
	"runtime"
	"strings"
	"sync/atomic"
)

// KeyBindingMode selects which modifier key triggers text shortcuts
// (select all, copy, cut, paste, undo) and which keys move the caret
// (issue #969).
//
// The mode is a fact about the platform, not about one window, so it
// is process-wide. It is chosen at startup from the OS. A backend that
// cannot know the OS at compile time (the web backend runs with
// GOOS=js) sets it with SetKeyBindingMode. The GOGUI_KEY_BINDING_MODE
// environment variable ("command" or "control") overrides both, so a
// developer on Linux can try the macOS bindings.
type KeyBindingMode uint8

const (
	// KeyBindingControl is the Linux, Windows and Android mode. Ctrl
	// triggers shortcuts. Ctrl or Alt with an arrow key moves by word.
	KeyBindingControl KeyBindingMode = iota
	// KeyBindingCommand is the macOS and iOS mode. Cmd (ModSuper)
	// triggers shortcuts. Option (ModAlt) with an arrow key moves by
	// word. Cmd with an arrow key moves to the line or document edge.
	// Ctrl is left free for the Emacs-style keys that every Cocoa text
	// field has: Ctrl+A/E/F/B/N/P/D/H/K/Y.
	KeyBindingCommand
)

// keyBindingEnv is the environment variable that pins the mode.
const keyBindingEnv = "GOGUI_KEY_BINDING_MODE"

var (
	// keyBinding holds the current KeyBindingMode. Atomic because a
	// backend may set it from its own goroutine while event dispatch
	// reads it.
	keyBinding atomic.Uint32
	// keyBindingPinned is true when the environment variable chose the
	// mode. Written once in init, read-only after.
	keyBindingPinned bool
)

func init() {
	mode := defaultKeyBinding(runtime.GOOS)
	if m, ok := parseKeyBinding(os.Getenv(keyBindingEnv)); ok {
		mode = m
		keyBindingPinned = true
	}
	keyBinding.Store(uint32(mode))
}

// defaultKeyBinding returns the mode native to an OS. Apple platforms
// use Cmd. All others use Ctrl.
func defaultKeyBinding(goos string) KeyBindingMode {
	switch goos {
	case "darwin", "ios":
		return KeyBindingCommand
	}
	return KeyBindingControl
}

// parseKeyBinding reads the environment variable value. An empty or
// unknown value returns false, so the platform default stays.
func parseKeyBinding(s string) (KeyBindingMode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "command", "cmd":
		return KeyBindingCommand, true
	case "control", "ctrl":
		return KeyBindingControl, true
	}
	return KeyBindingControl, false
}

// SetKeyBindingMode sets the process-wide key binding mode. Backends
// call it when the OS is known only at run time; the web backend calls
// it when the browser runs on a Mac. Apps do not need it: the default
// already matches the platform. The call has no effect when
// GOGUI_KEY_BINDING_MODE is set, because that variable is a developer
// override and must win.
//
// exportaudit:keep — backend hook (web backend sets it from navigator)
func SetKeyBindingMode(m KeyBindingMode) {
	if keyBindingPinned {
		return
	}
	keyBinding.Store(uint32(m))
}

// currentKeyBinding returns the mode in effect now.
func currentKeyBinding() KeyBindingMode {
	return KeyBindingMode(keyBinding.Load())
}

// ShortcutModifier returns the modifier that triggers text shortcuts on
// this platform: ModSuper (Cmd) on macOS and iOS, ModCtrl elsewhere.
// Subpackages and apps use it to match select all, copy and similar
// keys the same way the built-in text widgets do.
func ShortcutModifier() Modifier {
	if currentKeyBinding() == KeyBindingCommand {
		return ModSuper
	}
	return ModCtrl
}

// isShortcut reports whether m holds the shortcut modifier. Other
// modifiers may be held too, so Ctrl+Shift+Z (redo) still counts.
func isShortcut(m Modifier) bool {
	return m.Has(ShortcutModifier())
}

// isWordMod reports whether an arrow key with m moves by word. On
// macOS that is Option only: Cmd moves to the line edge and Ctrl is
// taken by the system (Mission Control). Elsewhere Ctrl or Alt.
func isWordMod(m Modifier) bool {
	if currentKeyBinding() == KeyBindingCommand {
		return m.Has(ModAlt)
	}
	return m.HasAny(ModCtrl, ModAlt)
}

// isLineMod reports whether an arrow key with m moves to the line edge
// (Left/Right) or the document edge (Up/Down). Only macOS has this:
// Cmd+Left is Home there.
func isLineMod(m Modifier) bool {
	return currentKeyBinding() == KeyBindingCommand && m.Has(ModSuper)
}

// isCocoaEmacs reports whether a key press with m is a Cocoa Emacs
// key: macOS mode and Ctrl held with no other keyboard modifier. Mouse
// button bits are ignored.
func isCocoaEmacs(m Modifier) bool {
	return currentKeyBinding() == KeyBindingCommand &&
		m&modKeyboard == ModCtrl
}

// cocoaCaretKey maps a Cocoa Emacs letter to the key it acts as.
// Ctrl+B/F/P/N move the caret like the arrow keys; Ctrl+H/D delete
// like Backspace/Delete. Other letters return KeyInvalid. The caller
// checks isCocoaEmacs first.
func cocoaCaretKey(k KeyCode) KeyCode {
	switch k {
	case KeyB:
		return KeyLeft
	case KeyF:
		return KeyRight
	case KeyP:
		return KeyUp
	case KeyN:
		return KeyDown
	case KeyH:
		return KeyBackspace
	case KeyD:
		return KeyDelete
	}
	return KeyInvalid
}

// textKey is a key press resolved for caret movement under the
// current key binding. Input, text selection and RTF selection share
// it, so the three widgets move the caret the same way.
type textKey struct {
	// key is the key the press acts as. A Cocoa Emacs letter becomes
	// its arrow or delete key; Cmd+Left/Right on macOS becomes
	// Home/End.
	key KeyCode
	// emacs is true for a Cocoa Emacs press (Ctrl alone on macOS).
	// Ctrl+A/E/K/Y have no plain-key twin, so the caller handles them
	// when emacs is set.
	emacs bool
	// docEdge is true for Cmd+Up/Down on macOS: go to the start or end
	// of the whole text.
	docEdge bool
	// lineEdge is true for Cmd+Left/Right on macOS. key is then
	// KeyHome/KeyEnd, but the move stops at the line edge instead of
	// cycling on to the paragraph and text edge as Home/End do.
	lineEdge bool
	isShift  bool
	isWord   bool
}

// resolveTextKey reads a key event into a textKey.
func resolveTextKey(e *Event) textKey {
	k := textKey{
		key:     e.KeyCode,
		isShift: e.Modifiers.Has(ModShift),
		isWord:  isWordMod(e.Modifiers),
	}
	if isCocoaEmacs(e.Modifiers) {
		k.emacs = true
		if ck := cocoaCaretKey(e.KeyCode); ck != KeyInvalid {
			k.key = ck
		}
		return k
	}
	if isLineMod(e.Modifiers) {
		switch e.KeyCode {
		case KeyLeft:
			k.key, k.lineEdge = KeyHome, true
		case KeyRight:
			k.key, k.lineEdge = KeyEnd, true
		case KeyUp, KeyDown:
			k.docEdge = true
		}
	}
	return k
}

// textKeyBoundMove handles the caret moves that exist only under a key
// binding mode: macOS Cmd+Up/Down and Cocoa Ctrl+A/E. It returns false
// for any other key, so the caller's own switch handles it.
func textKeyBoundMove(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, tk textKey,
) bool {
	switch {
	case tk.docEdge:
		textKeyDocEdge(imap, id, is, text, tk.key == KeyUp, tk.isShift)
	case tk.emacs && (tk.key == KeyA || tk.key == KeyE):
		textKeyParagraphEdge(imap, id, is, text, pos, tk.key == KeyE)
	default:
		return false
	}
	return true
}

// textKeyParagraphEdge moves the caret to the start (end=false) or end
// of its paragraph. It serves Cocoa Ctrl+A/E, which, unlike Home/End,
// stop at the paragraph edge on a second press.
func textKeyParagraphEdge(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, end bool,
) {
	newPos := cursorStartOfParagraph(text, pos)
	if end {
		newPos = cursorEndOfParagraph(text, pos)
	}
	updateCursorAndSelection(imap, id, is, newPos, false, utf8RuneCount(text))
}

// textKeyDocEdge moves the caret to the start (up) or end of the
// whole text, extending the selection with Shift. It serves macOS
// Cmd+Up/Down.
func textKeyDocEdge(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, up, isShift bool,
) {
	runeLen := utf8RuneCount(text)
	newPos := runeLen
	if up {
		newPos = 0
	}
	updateCursorAndSelection(imap, id, is, newPos, isShift, runeLen)
}
