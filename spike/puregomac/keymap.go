//go:build darwin

package main

import "github.com/go-gui-org/go-gui/gui"

// Copied from gui/backend/metal/events.go. A port moves these into a file
// without import "C" so both backends can share them.

func mapMacKeyCode(kc uint16) gui.KeyCode {
	if int(kc) < len(macKeyCodeTable) {
		return macKeyCodeTable[kc]
	}
	return gui.KeyInvalid
}

// macOS modifier flags (NSEventModifierFlags) to gui.Modifier.
func mapMetalModifiers(flags uint32) gui.Modifier {
	var m gui.Modifier
	// NSEventModifierFlagCapsLock = 1 << 16
	// NSEventModifierFlagShift    = 1 << 17
	// NSEventModifierFlagControl  = 1 << 18
	// NSEventModifierFlagOption   = 1 << 19
	// NSEventModifierFlagCommand  = 1 << 20
	if flags&(1<<17) != 0 {
		m |= gui.ModShift
	}
	if flags&(1<<18) != 0 {
		m |= gui.ModCtrl
	}
	if flags&(1<<19) != 0 {
		m |= gui.ModAlt
	}
	if flags&(1<<20) != 0 {
		m |= gui.ModSuper
	}
	// CapsLock (1<<16) intentionally ignored — no gui.CapsLock
	// modifier constant.
	return m
}

var macKeyCodeTable = [128]gui.KeyCode{
	0x00: gui.KeyA,
	0x01: gui.KeyS,
	0x02: gui.KeyD,
	0x03: gui.KeyF,
	0x04: gui.KeyH,
	0x05: gui.KeyG,
	0x06: gui.KeyZ,
	0x07: gui.KeyX,
	0x08: gui.KeyC,
	0x09: gui.KeyV,
	0x0A: gui.KeyWorld1, // ISO section (§/± key)
	0x0B: gui.KeyB,
	0x0C: gui.KeyQ,
	0x0D: gui.KeyW,
	0x0E: gui.KeyE,
	0x0F: gui.KeyR,
	0x10: gui.KeyY,
	0x11: gui.KeyT,
	0x12: gui.Key1,
	0x13: gui.Key2,
	0x14: gui.Key3,
	0x15: gui.Key4,
	0x16: gui.Key6,
	0x17: gui.Key5,
	0x18: gui.KeyEqual,
	0x19: gui.Key9,
	0x1A: gui.Key7,
	0x1B: gui.KeyMinus,
	0x1C: gui.Key8,
	0x1D: gui.Key0,
	0x1E: gui.KeyRightBracket,
	0x1F: gui.KeyO,
	0x20: gui.KeyU,
	0x21: gui.KeyLeftBracket,
	0x22: gui.KeyI,
	0x23: gui.KeyP,
	0x24: gui.KeyEnter, // Return
	0x25: gui.KeyL,
	0x26: gui.KeyJ,
	0x27: gui.KeyApostrophe,
	0x28: gui.KeyK,
	0x29: gui.KeySemicolon,
	0x2A: gui.KeyBackslash,
	0x2B: gui.KeyComma,
	0x2C: gui.KeySlash,
	0x2D: gui.KeyN,
	0x2E: gui.KeyM,
	0x2F: gui.KeyPeriod,
	0x30: gui.KeyTab,
	0x31: gui.KeySpace,
	0x32: gui.KeyGraveAccent, // Backtick/tilde
	0x33: gui.KeyBackspace,   // Delete (backspace on Mac)
	0x35: gui.KeyEscape,
	0x36: gui.KeyRightSuper, // Right Cmd
	0x37: gui.KeyLeftSuper,  // Left Cmd
	// The modifier block follows Events.h exactly: left keys are the bare
	// kVK_Shift/Option/Control, right keys are kVK_Right*. It was once
	// shifted by a slot, which reported left Option as right Option.
	0x38: gui.KeyLeftShift,    // kVK_Shift
	0x39: gui.KeyCapsLock,     // kVK_CapsLock
	0x3A: gui.KeyLeftAlt,      // kVK_Option (left)
	0x3B: gui.KeyLeftControl,  // kVK_Control (left)
	0x3C: gui.KeyRightShift,   // kVK_RightShift
	0x3D: gui.KeyRightAlt,     // kVK_RightOption
	0x3E: gui.KeyRightControl, // kVK_RightControl
	0x3F: gui.KeyInvalid,      // kVK_Function (Fn): no gui constant
	0x40: gui.KeyF17,
	0x41: gui.KeyKPDecimal,  // Keypad .
	0x43: gui.KeyKPMultiply, // Keypad *
	0x45: gui.KeyKPAdd,      // Keypad +
	0x47: gui.KeyNumLock,    // Clear (keypad num lock)
	0x48: gui.KeyInvalid,    // VolumeUp (no gui constant yet)
	0x49: gui.KeyInvalid,    // VolumeDown
	0x4A: gui.KeyInvalid,    // Mute
	0x4B: gui.KeyKPDivide,   // Keypad /
	0x4C: gui.KeyKPEnter,
	0x4E: gui.KeyKPSubtract, // Keypad -
	0x4F: gui.KeyF18,
	0x50: gui.KeyF19,
	0x51: gui.KeyKPEqual, // Keypad =
	0x52: gui.KeyKP0,
	0x53: gui.KeyKP1,
	0x54: gui.KeyKP2,
	0x55: gui.KeyKP3,
	0x56: gui.KeyKP4,
	0x57: gui.KeyKP5,
	0x58: gui.KeyKP6,
	0x59: gui.KeyKP7,
	0x5A: gui.KeyF20,
	0x5B: gui.KeyKP8,
	0x5C: gui.KeyKP9,
	0x5D: gui.KeyWorld2,  // JIS Yen key
	0x5E: gui.KeyWorld2,  // JIS Underscore key (closest)
	0x5F: gui.KeyKPEnter, // JIS keypad comma → KPEnter
	0x60: gui.KeyF5,
	0x61: gui.KeyF6,
	0x62: gui.KeyF7,
	0x63: gui.KeyF3,
	0x64: gui.KeyF8,
	0x65: gui.KeyF9,
	0x66: gui.KeyWorld2, // JIS Eisu key
	0x67: gui.KeyF11,
	0x68: gui.KeyWorld2, // JIS Kana key
	0x69: gui.KeyF13,
	0x6A: gui.KeyF16,
	0x6B: gui.KeyF14,
	0x6D: gui.KeyF10,
	0x6E: gui.KeyMenu, // Context menu key (PC keyboard on Mac)
	0x6F: gui.KeyF12,
	0x71: gui.KeyF15,
	0x72: gui.KeyInsert, // Help key (Insert on PC keyboard)
	0x73: gui.KeyHome,
	0x74: gui.KeyPageUp,
	0x75: gui.KeyDelete, // Forward delete (Del on PC keyboard)
	0x76: gui.KeyF4,
	0x77: gui.KeyEnd,
	0x78: gui.KeyF2,
	0x79: gui.KeyPageDown,
	0x7A: gui.KeyF1,
	0x7B: gui.KeyLeft,
	0x7C: gui.KeyRight,
	0x7D: gui.KeyDown,
	0x7E: gui.KeyUp,
}
