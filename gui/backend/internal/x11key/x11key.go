//go:build linux

// Package x11key maps X11 keysyms and modifier/button state to go-gui
// key, modifier, and mouse-button values. It mirrors the winkey
// package for the native Linux (X11) backend.
package x11key

import "github.com/go-gui-org/go-gui/gui"

// X11 keysyms (subset, from keysymdef.h).
const (
	xkBackSpace      = 0xff08
	xkTab            = 0xff09
	xkReturn         = 0xff0d
	xkPause          = 0xff13
	xkScrollLock     = 0xff14
	xkEscape         = 0xff1b
	xkDelete         = 0xffff
	xkHome           = 0xff50
	xkLeft           = 0xff51
	xkUp             = 0xff52
	xkRight          = 0xff53
	xkDown           = 0xff54
	xkPrior          = 0xff55 // PageUp
	xkNext           = 0xff56 // PageDown
	xkEnd            = 0xff57
	xkPrint          = 0xff61
	xkInsert         = 0xff63
	xkMenu           = 0xff67
	xkNumLock        = 0xff7f
	xkKPSpace        = 0xff80
	xkKPEnter        = 0xff8d
	xkKPHome         = 0xff95
	xkKPLeft         = 0xff96
	xkKPUp           = 0xff97
	xkKPRight        = 0xff98
	xkKPDown         = 0xff99
	xkKPPrior        = 0xff9a
	xkKPNext         = 0xff9b
	xkKPEnd          = 0xff9c
	xkKPInsert       = 0xff9e
	xkKPDelete       = 0xff9f
	xkKPMultiply     = 0xffaa
	xkKPAdd          = 0xffab
	xkKPSeparator    = 0xffac
	xkKPSubtract     = 0xffad
	xkKPDecimal      = 0xffae
	xkKPDivide       = 0xffaf
	xkKP0            = 0xffb0
	xkKP9            = 0xffb9
	xkKPEqual        = 0xffbd
	xkF1             = 0xffbe
	xkF25            = 0xffd6
	xkShiftL         = 0xffe1
	xkShiftR         = 0xffe2
	xkControlL       = 0xffe3
	xkControlR       = 0xffe4
	xkCapsLock       = 0xffe5
	xkAltL           = 0xffe9
	xkAltR           = 0xffea
	xkSuperL         = 0xffeb
	xkSuperR         = 0xffec
	xkISOLevel3Shift = 0xfe03
	xkDeadGrave      = 0xfe50
	xkDeadCircumflex = 0xfe52
	xkDeadTilde      = 0xfe53
	xkEuroSign       = 0x20ac
)

// evdevRightAlt is KEY_RIGHTALT (linux/input-event-codes.h).
const evdevRightAlt = 100

// usPosition is the letter or digit US prints on each evdev key, for
// rule 5's last resort on layouts that type no Latin letters.
var usPosition = [...]gui.KeyCode{
	2: gui.Key1, 3: gui.Key2, 4: gui.Key3, 5: gui.Key4, 6: gui.Key5,
	7: gui.Key6, 8: gui.Key7, 9: gui.Key8, 10: gui.Key9, 11: gui.Key0,
	16: gui.KeyQ, 17: gui.KeyW, 18: gui.KeyE, 19: gui.KeyR, 20: gui.KeyT,
	21: gui.KeyY, 22: gui.KeyU, 23: gui.KeyI, 24: gui.KeyO, 25: gui.KeyP,
	30: gui.KeyA, 31: gui.KeyS, 32: gui.KeyD, 33: gui.KeyF, 34: gui.KeyG,
	35: gui.KeyH, 36: gui.KeyJ, 37: gui.KeyK, 38: gui.KeyL,
	44: gui.KeyZ, 45: gui.KeyX, 46: gui.KeyC, 47: gui.KeyV, 48: gui.KeyB,
	49: gui.KeyN, 50: gui.KeyM,
}

// KeyInput is what the KeyCode rules read about one key press.
type KeyInput struct {
	Plain    uint32 // keysym with NumLock applied and no other modifier
	Shifted  uint32 // keysym with Shift (and NumLock) applied
	Latin    uint32 // Plain in the first Latin layout, when NonLatin
	NonLatin bool   // the active layout types no Latin letters
	Evdev    uint32 // evdev code of the key, XKB keycode - 8; 0 if unknown
}

// KeyCode maps a key press to a gui.KeyCode by the rules of
// docs/specs/keyboard-layouts.md § 2. The first rule that applies wins.
func KeyCode(in KeyInput) gui.KeyCode {
	// 1, 2: modifier, named and keypad keys by their own keysym.
	if c := namedKey(in.Plain); c != gui.KeyInvalid {
		return c
	}
	if in.Plain == xkISOLevel3Shift {
		if in.Evdev == evdevRightAlt {
			return gui.KeyRightAlt
		}
		return gui.KeyInvalid
	}
	// 3: an unmodified ASCII letter or digit.
	if c := letterOrDigit(in.Plain); c != gui.KeyInvalid {
		return c
	}
	// 4: a digit with Shift (the AZERTY number row).
	if in.Shifted >= '0' && in.Shifted <= '9' {
		return gui.KeyCode(in.Shifted)
	}
	// 5: on a non-Latin layout, the Latin layout's letter or digit, else
	// the one US prints at this position.
	if in.NonLatin {
		if c := letterOrDigit(in.Latin); c != gui.KeyInvalid {
			return c
		}
		if int(in.Evdev) < len(usPosition) && usPosition[in.Evdev] != gui.KeyInvalid {
			return usPosition[in.Evdev]
		}
	}
	// 6: unmodified printable ASCII; its KeyCode is its character code.
	if in.Plain > ' ' && in.Plain < 0x7f {
		return gui.KeyCode(in.Plain)
	}
	// 7: a dead key with a spacing ASCII form.
	switch in.Plain {
	case xkDeadGrave:
		return gui.KeyGraveAccent
	case xkDeadCircumflex:
		return gui.KeyCircumflex
	case xkDeadTilde:
		return gui.KeyTilde
	}
	return gui.KeyInvalid
}

// MapKeySym maps a single X11 keysym to a gui.KeyCode: KeyCode's rules
// for a key known only by that keysym, such as one an input method
// forwards. Letters are normalized to their uppercase form (gui.KeyCode
// uses ASCII 'A'-'Z').
func MapKeySym(sym uint32) gui.KeyCode {
	return KeyCode(KeyInput{Plain: sym})
}

// letterOrDigit maps an ASCII letter (either case) or digit keysym to its
// KeyCode, KeyInvalid otherwise.
func letterOrDigit(sym uint32) gui.KeyCode {
	switch {
	case sym >= 'a' && sym <= 'z':
		return gui.KeyCode(sym - 0x20)
	case sym >= 'A' && sym <= 'Z', sym >= '0' && sym <= '9':
		return gui.KeyCode(sym)
	}
	return gui.KeyInvalid
}

// namedKey maps the keysyms of modifier, named and keypad keys (rules 1
// and 2). KP_Begin, the keypad 5 without NumLock, has no KeyCode.
//
//nolint:gocyclo // key-mapping switch
func namedKey(sym uint32) gui.KeyCode {
	switch {
	case sym >= xkF1 && sym <= xkF25:
		return gui.KeyF1 + gui.KeyCode(sym-xkF1)
	case sym >= xkKP0 && sym <= xkKP9:
		return gui.KeyKP0 + gui.KeyCode(sym-xkKP0)
	}
	switch sym {
	case ' ':
		return gui.KeySpace
	case xkReturn, xkKPEnter:
		return gui.KeyEnter
	case xkEscape:
		return gui.KeyEscape
	case xkTab:
		return gui.KeyTab
	case xkBackSpace:
		return gui.KeyBackspace
	case xkDelete, xkKPDelete:
		return gui.KeyDelete
	case xkInsert, xkKPInsert:
		return gui.KeyInsert
	case xkRight, xkKPRight:
		return gui.KeyRight
	case xkLeft, xkKPLeft:
		return gui.KeyLeft
	case xkDown, xkKPDown:
		return gui.KeyDown
	case xkUp, xkKPUp:
		return gui.KeyUp
	case xkPrior, xkKPPrior:
		return gui.KeyPageUp
	case xkNext, xkKPNext:
		return gui.KeyPageDown
	case xkHome, xkKPHome:
		return gui.KeyHome
	case xkEnd, xkKPEnd:
		return gui.KeyEnd
	case xkKPDecimal, xkKPSeparator:
		return gui.KeyKPDecimal
	case xkKPDivide:
		return gui.KeyKPDivide
	case xkKPMultiply:
		return gui.KeyKPMultiply
	case xkKPSubtract:
		return gui.KeyKPSubtract
	case xkKPAdd:
		return gui.KeyKPAdd
	case xkKPEqual:
		return gui.KeyKPEqual
	case xkNumLock:
		return gui.KeyNumLock
	case xkScrollLock:
		return gui.KeyScrollLock
	case xkPrint:
		return gui.KeyPrintScreen
	case xkPause:
		return gui.KeyPause
	case xkMenu:
		return gui.KeyMenu
	case xkShiftL:
		return gui.KeyLeftShift
	case xkShiftR:
		return gui.KeyRightShift
	case xkControlL:
		return gui.KeyLeftControl
	case xkControlR:
		return gui.KeyRightControl
	case xkAltL:
		return gui.KeyLeftAlt
	case xkAltR:
		return gui.KeyRightAlt
	case xkSuperL:
		return gui.KeyLeftSuper
	case xkSuperR:
		return gui.KeyRightSuper
	case xkCapsLock:
		return gui.KeyCapsLock
	}
	return gui.KeyInvalid
}

// X11 KeyButMask bits (from xproto).
const (
	maskShift   = 1
	maskControl = 4
	maskMod1    = 8   // Alt
	maskMod4    = 64  // Super
	maskMod5    = 128 // ISO level 3 (AltGr)
	maskButton1 = 256
	maskButton2 = 512
	maskButton3 = 1024
)

// MapModifiers maps an X11 event state mask to gui modifiers,
// including held mouse buttons.
func MapModifiers(state uint16) gui.Modifier {
	var m gui.Modifier
	if state&maskShift != 0 {
		m |= gui.ModShift
	}
	if state&maskControl != 0 {
		m |= gui.ModCtrl
	}
	if state&maskMod1 != 0 {
		m |= gui.ModAlt
	}
	if state&maskMod4 != 0 {
		m |= gui.ModSuper
	}
	if state&maskMod5 != 0 {
		m |= gui.ModAltGr
	}
	if state&maskButton1 != 0 {
		m |= gui.ModLMB
	}
	if state&maskButton2 != 0 {
		m |= gui.ModMMB
	}
	if state&maskButton3 != 0 {
		m |= gui.ModRMB
	}
	return m
}

// MapButton maps an X11 button detail (1=left, 2=middle, 3=right) to a
// gui.MouseButton. Returns MouseInvalid for other buttons (8 and 9 are
// back and forward): as MouseLeft, their press set the left button held
// with no release to clear it, since only buttons 1-3 send one (#1098).
func MapButton(detail byte) gui.MouseButton {
	switch detail {
	case 1:
		return gui.MouseLeft
	case 2:
		return gui.MouseMiddle
	case 3:
		return gui.MouseRight
	default:
		return gui.MouseInvalid
	}
}

// KeysymToRune converts a printable X11 keysym to a Unicode rune,
// returning 0 for non-printable keysyms (function keys, modifiers,
// keypad control, etc.). Handles Latin-1, the keypad's printable keys,
// EuroSign and the direct-Unicode (0x01000000) keysym range. Without
// libxkbcommon it is all the X11 path has, so a numpad digit and AltGr+E
// type there too (#1188).
func KeysymToRune(sym uint32) rune {
	// Direct Unicode keysyms.
	if sym&0xff000000 == 0x01000000 {
		return rune(sym & 0x00ffffff)
	}
	switch {
	case (sym >= 0x20 && sym <= 0x7e) || (sym >= 0xa0 && sym <= 0xff):
		// Latin-1 (ASCII printable + Latin-1 supplement) map 1:1.
		return rune(sym)
	case sym == xkKPSpace:
		return ' '
	case (sym >= xkKPMultiply && sym <= xkKP9) || sym == xkKPEqual:
		// A printable keypad keysym is 0xff80 plus its ASCII character.
		return rune(sym & 0x7f)
	case sym == xkEuroSign:
		// The one legacy currency keysym libxkbcommon types; it carries
		// its code point.
		return rune(sym)
	}
	return 0
}
