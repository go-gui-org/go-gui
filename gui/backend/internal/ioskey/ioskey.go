// Package ioskey turns UIKit keyboard input into go-gui events for the
// iOS backend (issue #806). It mirrors the winkey and x11key packages.
//
// The package has no build tag and no cgo, so its tables run under
// `go test` on any host. The iOS backend itself builds only with
// GOOS=ios, where nothing runs in CI.
package ioskey

import (
	"strings"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
)

// UIKeyboardType values (UIKit/UITextInputTraits.h).
const (
	uiKeyboardTypeDefault      = 0
	uiKeyboardTypeURL          = 3
	uiKeyboardTypeNumberPad    = 4
	uiKeyboardTypePhonePad     = 5
	uiKeyboardTypeEmailAddress = 7
	uiKeyboardTypeDecimalPad   = 8
)

// KeyboardType returns the UIKeyboardType for a field's KeyboardKind.
// KeyboardNone and unknown kinds give the default text keyboard: the
// view hides the keyboard for KeyboardNone, so the type is not shown.
func KeyboardType(kind gui.KeyboardKind) int {
	switch kind {
	case gui.KeyboardNumber:
		return uiKeyboardTypeNumberPad
	case gui.KeyboardDecimal:
		return uiKeyboardTypeDecimalPad
	case gui.KeyboardPhone:
		return uiKeyboardTypePhonePad
	case gui.KeyboardEmail:
		return uiKeyboardTypeEmailAddress
	case gui.KeyboardURL:
		return uiKeyboardTypeURL
	}
	return uiKeyboardTypeDefault
}

// maxTextRunes bounds one insertText: string crossing the cgo boundary.
// It matches the Android and web backends; a real commit is far
// shorter.
const maxTextRunes = 4096

// sanitizeText removes decoding failures and caps the length. It
// returns "" when no text is left to insert.
func sanitizeText(s string) string {
	if !utf8.ValidString(s) || strings.ContainsRune(s, utf8.RuneError) {
		// ToValidUTF8 turns each bad byte run into U+FFFD, then the Map
		// drops every U+FFFD, the ones UIKit sent and the ones just made.
		s = strings.Map(func(r rune) rune {
			if r == utf8.RuneError {
				return -1
			}
			return r
		}, strings.ToValidUTF8(s, "�"))
	}
	if utf8.RuneCountInString(s) > maxTextRunes {
		i := 0
		for range maxTextRunes {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
		}
		s = s[:i]
	}
	return s
}

// TextEvent converts the string UIKit passes to insertText:. A line
// break (the Return key, soft or hardware) becomes a KeyEnter key-down,
// because text fields act on Enter in OnKeyDown, not in OnChar. Other
// text becomes one EventChar that carries the whole string in IMEText,
// the same shape the macOS, Android and web backends send. ok is false
// when nothing is left to send.
func TextEvent(text string) (e gui.Event, ok bool) {
	switch text {
	case "\n", "\r", "\r\n":
		return gui.Event{Type: gui.EventKeyDown, KeyCode: gui.KeyEnter}, true
	}
	text = sanitizeText(text)
	if text == "" {
		return gui.Event{}, false
	}
	r, _ := utf8.DecodeRuneInString(text)
	return gui.Event{Type: gui.EventChar, CharCode: uint32(r), IMEText: text}, true
}

// BackspaceEvent is the event for UIKeyInput's deleteBackward. It is a
// key-down, not a "\b" character: text fields drop control characters
// in OnChar and delete in OnKeyDown.
func BackspaceEvent() gui.Event {
	return gui.Event{Type: gui.EventKeyDown, KeyCode: gui.KeyBackspace}
}

// UIKeyboardHIDUsage values (UIKit/UIKeyConstants.h, USB HID page 7).
const (
	hidA             = 0x04 // through Z at 0x1D
	hid1             = 0x1E // through 9 at 0x26
	hid0             = 0x27
	hidReturn        = 0x28
	hidEscape        = 0x29
	hidBackspace     = 0x2A
	hidTab           = 0x2B
	hidSpace         = 0x2C
	hidMinus         = 0x2D
	hidEqual         = 0x2E
	hidLeftBracket   = 0x2F
	hidRightBracket  = 0x30
	hidBackslash     = 0x31
	hidSemicolon     = 0x33
	hidApostrophe    = 0x34
	hidGrave         = 0x35
	hidComma         = 0x36
	hidPeriod        = 0x37
	hidSlash         = 0x38
	hidF1            = 0x3A // through F12 at 0x45
	hidF12           = 0x45
	hidInsert        = 0x49
	hidHome          = 0x4A
	hidPageUp        = 0x4B
	hidDeleteForward = 0x4C
	hidEnd           = 0x4D
	hidPageDown      = 0x4E
	hidRightArrow    = 0x4F
	hidLeftArrow     = 0x50
	hidDownArrow     = 0x51
	hidUpArrow       = 0x52
)

// hidText maps the HID usages of printable punctuation keys. Letters,
// digits and space are handled as ranges in HIDKey.
var hidText = map[int]gui.KeyCode{
	hidMinus:        gui.KeyMinus,
	hidEqual:        gui.KeyEqual,
	hidLeftBracket:  gui.KeyLeftBracket,
	hidRightBracket: gui.KeyRightBracket,
	hidBackslash:    gui.KeyBackslash,
	hidSemicolon:    gui.KeySemicolon,
	hidApostrophe:   gui.KeyApostrophe,
	hidGrave:        gui.KeyGraveAccent,
	hidComma:        gui.KeyComma,
	hidPeriod:       gui.KeyPeriod,
	hidSlash:        gui.KeySlash,
}

// hidNav maps the HID usages of keys that type no text.
var hidNav = map[int]gui.KeyCode{
	hidEscape:        gui.KeyEscape,
	hidTab:           gui.KeyTab,
	hidInsert:        gui.KeyInsert,
	hidHome:          gui.KeyHome,
	hidPageUp:        gui.KeyPageUp,
	hidDeleteForward: gui.KeyDelete,
	hidEnd:           gui.KeyEnd,
	hidPageDown:      gui.KeyPageDown,
	hidRightArrow:    gui.KeyRight,
	hidLeftArrow:     gui.KeyLeft,
	hidDownArrow:     gui.KeyDown,
	hidUpArrow:       gui.KeyUp,
}

// HIDKey maps a hardware key's HID usage to a gui.KeyCode. text reports
// a key that UIKit types through UIKeyInput while a text field is
// active: a printable key, and also Return (insertText: "\n") and
// Backspace (deleteBackward). KeyEvent leaves those to UIKit, so each
// press arrives once.
func HIDKey(usage int) (key gui.KeyCode, text bool) {
	switch {
	case usage >= hidA && usage < hidA+26:
		return gui.KeyA + gui.KeyCode(usage-hidA), true
	case usage >= hid1 && usage < hid1+9:
		return gui.Key1 + gui.KeyCode(usage-hid1), true
	case usage == hid0:
		return gui.Key0, true
	case usage == hidSpace:
		return gui.KeySpace, true
	case usage == hidReturn:
		return gui.KeyEnter, true
	case usage == hidBackspace:
		return gui.KeyBackspace, true
	case usage >= hidF1 && usage <= hidF12:
		return gui.KeyF1 + gui.KeyCode(usage-hidF1), false
	}
	if k, found := hidText[usage]; found {
		return k, true
	}
	if k, found := hidNav[usage]; found {
		return k, false
	}
	return gui.KeyInvalid, false
}

// UIKeyModifierFlags values (UIKit/UICommand.h).
const (
	uiKeyModifierAlphaShift = 1 << 16 // caps lock
	uiKeyModifierShift      = 1 << 17
	uiKeyModifierControl    = 1 << 18
	uiKeyModifierAlternate  = 1 << 19
	uiKeyModifierCommand    = 1 << 20
	uiKeyModifierNumericPad = 1 << 21
)

// Modifiers maps UIKeyModifierFlags to gui modifiers. Command maps to
// ModSuper, the same as the macOS backend.
func Modifiers(flags int) gui.Modifier {
	var m gui.Modifier
	if flags&uiKeyModifierShift != 0 {
		m |= gui.ModShift
	}
	if flags&uiKeyModifierControl != 0 {
		m |= gui.ModCtrl
	}
	if flags&uiKeyModifierAlternate != 0 {
		m |= gui.ModAlt
	}
	if flags&uiKeyModifierCommand != 0 {
		m |= gui.ModSuper
	}
	return m
}

// KeyEvent converts a hardware key press (down true) or release from
// pressesBegan: / pressesEnded:. editing is true when a text field is
// active and the view, a UIKeyInput, is first responder.
//
// For a press while editing, ok is false when the press must go on to
// UIKit, so that UIKit types it through insertText: or deleteBackward:
// a text key (see HIDKey) with no Command or Control held. A key that
// types no text (arrows, Tab, Escape) and a Command or Control chord
// (Cmd+C, Ctrl+A) come to Go. With no text field active, UIKit types
// nothing, so every mapped key comes to Go, as on the desktop.
//
// For a release, ok is true for every mapped key. The view sends a
// release only for a key whose press Go took: the modifiers can change
// while the key is down (Cmd lifted before C), so they cannot decide.
//
// ok is false for a key this package does not map.
func KeyEvent(usage, flags int, down, editing bool) (e gui.Event, ok bool) {
	key, text := HIDKey(usage)
	if key == gui.KeyInvalid {
		return gui.Event{}, false
	}
	if down && editing && text &&
		flags&(uiKeyModifierCommand|uiKeyModifierControl) == 0 {
		return gui.Event{}, false
	}
	typ := gui.EventKeyUp
	if down {
		typ = gui.EventKeyDown
	}
	return gui.Event{Type: typ, KeyCode: key, Modifiers: Modifiers(flags)}, true
}
