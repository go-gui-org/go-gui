//go:build linux

// Package x11key maps X11 keysyms and modifier/button state to go-gui
// key, modifier, and mouse-button values. It mirrors the winkey and
// sdlkey packages for the native Linux (X11) backend.
package x11key

import "github.com/go-gui-org/go-gui/gui"

// X11 keysyms (subset, from keysymdef.h).
const (
	xkBackSpace = 0xff08
	xkTab       = 0xff09
	xkReturn    = 0xff0d
	xkEscape    = 0xff1b
	xkDelete    = 0xffff
	xkHome      = 0xff50
	xkLeft      = 0xff51
	xkUp        = 0xff52
	xkRight     = 0xff53
	xkDown      = 0xff54
	xkPrior     = 0xff55 // PageUp
	xkNext      = 0xff56 // PageDown
	xkEnd       = 0xff57
	xkInsert    = 0xff63
	xkKPEnter   = 0xff8d
	xkF1        = 0xffbe
	xkF12       = 0xffc9
	xkShiftL    = 0xffe1
	xkShiftR    = 0xffe2
	xkControlL  = 0xffe3
	xkControlR  = 0xffe4
	xkCapsLock  = 0xffe5
	xkAltL      = 0xffe9
	xkAltR      = 0xffea
	xkSuperL    = 0xffeb
	xkSuperR    = 0xffec
)

// MapKeySym maps an X11 keysym to a gui.KeyCode. Letters are
// normalized to their uppercase form (gui.KeyCode uses ASCII 'A'-'Z').
//
//nolint:gocyclo // key-mapping switch
func MapKeySym(sym uint32) gui.KeyCode {
	// ASCII letters: X sends lowercase (0x61-0x7a) when unshifted and
	// uppercase (0x41-0x5a) when shifted; gui.KeyCode uses uppercase.
	if sym >= 'a' && sym <= 'z' {
		return gui.KeyCode(sym - 0x20)
	}
	if sym >= 'A' && sym <= 'Z' {
		return gui.KeyCode(sym)
	}
	if sym >= '0' && sym <= '9' {
		return gui.KeyCode(sym)
	}
	if sym >= xkF1 && sym <= xkF12 {
		return gui.KeyF1 + gui.KeyCode(sym-xkF1)
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
	case xkDelete:
		return gui.KeyDelete
	case xkInsert:
		return gui.KeyInsert
	case xkRight:
		return gui.KeyRight
	case xkLeft:
		return gui.KeyLeft
	case xkDown:
		return gui.KeyDown
	case xkUp:
		return gui.KeyUp
	case xkPrior:
		return gui.KeyPageUp
	case xkNext:
		return gui.KeyPageDown
	case xkHome:
		return gui.KeyHome
	case xkEnd:
		return gui.KeyEnd
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
	case ',':
		return gui.KeyComma
	case '-':
		return gui.KeyMinus
	case '.':
		return gui.KeyPeriod
	case '/':
		return gui.KeySlash
	case ';':
		return gui.KeySemicolon
	case '=':
		return gui.KeyEqual
	case '[':
		return gui.KeyLeftBracket
	case '\\':
		return gui.KeyBackslash
	case ']':
		return gui.KeyRightBracket
	case '`':
		return gui.KeyGraveAccent
	default:
		return gui.KeyInvalid
	}
}

// X11 KeyButMask bits (from xproto).
const (
	maskShift   = 1
	maskControl = 4
	maskMod1    = 8  // Alt
	maskMod4    = 64 // Super
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
// gui.MouseButton. Returns MouseLeft for unknown buttons.
func MapButton(detail byte) gui.MouseButton {
	switch detail {
	case 2:
		return gui.MouseMiddle
	case 3:
		return gui.MouseRight
	default:
		return gui.MouseLeft
	}
}

// KeysymToRune converts a printable X11 keysym to a Unicode rune,
// returning 0 for non-printable keysyms (function keys, modifiers,
// keypad control, etc.). Handles Latin-1 and the direct-Unicode
// (0x01000000) keysym range.
func KeysymToRune(sym uint32) rune {
	// Direct Unicode keysyms.
	if sym&0xff000000 == 0x01000000 {
		return rune(sym & 0x00ffffff)
	}
	// Latin-1 (ASCII printable + Latin-1 supplement) map 1:1.
	if (sym >= 0x20 && sym <= 0x7e) || (sym >= 0xa0 && sym <= 0xff) {
		return rune(sym)
	}
	return 0
}
