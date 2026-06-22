//go:build darwin && !ios

package metal

/*
#include "metal_window.h"
*/
import "C"
import (
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
)

// mapMetalEvent converts the current metal event (set by
// metalPollEvent) to a gui.Event. Returns the event and true
// to continue, or false to quit.
func mapMetalEvent() (gui.Event, bool) {
	et := C.metalEventType()
	switch et {
	case C.METAL_EVENT_NONE:
		return gui.Event{}, true

	case C.METAL_EVENT_QUIT:
		return gui.Event{}, false

	case C.METAL_EVENT_MOUSE_DOWN:
		return gui.Event{
			Type:        gui.EventMouseDown,
			MouseX:      float32(C.metalEventMouseX()),
			MouseY:      float32(C.metalEventMouseY()),
			MouseButton: mapMetalMouseButton(int(C.metalEventMouseButton())),
			Modifiers:   mapMetalModifiers(uint32(C.metalEventModifiers())),
		}, true

	case C.METAL_EVENT_MOUSE_UP:
		return gui.Event{
			Type:        gui.EventMouseUp,
			MouseX:      float32(C.metalEventMouseX()),
			MouseY:      float32(C.metalEventMouseY()),
			MouseButton: mapMetalMouseButton(int(C.metalEventMouseButton())),
			Modifiers:   mapMetalModifiers(uint32(C.metalEventModifiers())),
		}, true

	case C.METAL_EVENT_MOUSE_MOVE:
		return gui.Event{
			Type:      gui.EventMouseMove,
			MouseX:    float32(C.metalEventMouseX()),
			MouseY:    float32(C.metalEventMouseY()),
			MouseDX:   float32(C.metalEventMouseDX()),
			MouseDY:   float32(C.metalEventMouseDY()),
			Modifiers: mapMetalModifiers(uint32(C.metalEventModifiers())),
		}, true

	case C.METAL_EVENT_SCROLL_WHEEL:
		return gui.Event{
			Type:      gui.EventMouseScroll,
			MouseX:    float32(C.metalEventMouseX()),
			MouseY:    float32(C.metalEventMouseY()),
			ScrollX:   float32(C.metalEventScrollX()),
			ScrollY:   float32(C.metalEventScrollY()),
			Modifiers: mapMetalModifiers(uint32(C.metalEventModifiers())),
		}, true

	case C.METAL_EVENT_KEY_DOWN:
		kc := mapMacKeyCode(uint16(C.metalEventKeyCode()))
		mods := mapMetalModifiers(uint32(C.metalEventModifiers()))
		// Cmd+Q → quit. The menu key equivalent path (quit: on
		// delegate via performKeyEquivalent:) is the primary path.
		// This check catches Cmd+Q as a fallback when the menu is
		// not wired.
		//
		// When performKeyEquivalent: fires first, quit: sets
		// _quitRequested=1; metalPollEvent consumes it before
		// dequeueing this key-down. On a vetoed quit, this
		// key-down arrives on the next poll and acts as a retry
		// (correct). On an un-vetoed quit, the loop already
		// exited before the second poll (harmless).
		if kc == gui.KeyQ && mods&gui.ModSuper != 0 {
			return gui.Event{}, false
		}
		return gui.Event{
			Type:      gui.EventKeyDown,
			KeyCode:   kc,
			Modifiers: mods,
			KeyRepeat: C.metalEventKeyRepeat() != 0,
		}, true

	case C.METAL_EVENT_KEY_UP:
		return gui.Event{
			Type:      gui.EventKeyUp,
			KeyCode:   mapMacKeyCode(uint16(C.metalEventKeyCode())),
			Modifiers: mapMetalModifiers(uint32(C.metalEventModifiers())),
		}, true

	case C.METAL_EVENT_FLAGS_CHANGED:
		// Modifier-only key change (Shift, Ctrl, Alt, Cmd).
		// Emit as a key event so Go can track modifier state.
		kc := mapMacKeyCode(uint16(C.metalEventKeyCode()))
		mods := mapMetalModifiers(uint32(C.metalEventModifiers()))
		// Determine if the key went down or up by checking
		// whether its modifier flag is set.
		down := (mods & keyCodeModFlag(kc)) != 0
		et2 := gui.EventKeyUp
		if down {
			et2 = gui.EventKeyDown
		}
		// Suppress spurious events with no modifier flag
		// change (e.g., caps lock state changes).
		if kc == gui.KeyInvalid {
			return gui.Event{}, true
		}
		return gui.Event{
			Type:      et2,
			KeyCode:   kc,
			Modifiers: mods,
		}, true

	case C.METAL_EVENT_CHAR:
		text := C.GoString(C.metalEventText())
		if len(text) == 0 {
			return gui.Event{}, true
		}
		r, sz := utf8.DecodeRuneInString(text)
		if r == utf8.RuneError && sz == 1 {
			return gui.Event{}, true
		}
		return gui.Event{
			Type:      gui.EventChar,
			CharCode:  uint32(r),
			IMEText:   text,
			Modifiers: mapMetalModifiers(uint32(C.metalEventModifiers())),
		}, true

	case C.METAL_EVENT_IME_COMP:
		text := C.GoString(C.metalEventText())
		return gui.Event{
			Type:      gui.EventIMEComposition,
			IMEText:   text,
			IMEStart:  int32(C.metalEventIMEStart()),
			IMELength: int32(C.metalEventIMELength()),
		}, true

	default:
		return gui.Event{}, true
	}
}

// mapMetalMouseButton maps NSEvent button number to gui.MouseButton.
// NSEvent: 0=left, 1=right, 2=middle.
func mapMetalMouseButton(btn int) gui.MouseButton {
	switch btn {
	case 1:
		return gui.MouseRight
	case 2:
		return gui.MouseMiddle
	default:
		return gui.MouseLeft
	}
}

// macOS virtual key code to gui.KeyCode mapping.
// Maps the UInt16 keyCode from NSEvent to gui.KeyCode.
// Physical-position-based; layout-independent.
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

// keyCodeModFlag returns the modifier flag bit for a given
// modifier key, used by flagsChanged to determine key up/down.
func keyCodeModFlag(kc gui.KeyCode) gui.Modifier {
	switch kc {
	case gui.KeyLeftShift, gui.KeyRightShift:
		return gui.ModShift
	case gui.KeyLeftControl, gui.KeyRightControl:
		return gui.ModCtrl
	case gui.KeyLeftAlt, gui.KeyRightAlt:
		return gui.ModAlt
	case gui.KeyLeftSuper, gui.KeyRightSuper:
		return gui.ModSuper
	case gui.KeyCapsLock:
		return 0 // no gui.CapsLock modifier constant
	}
	return 0
}

// macKeyCodeTable maps macOS virtual key codes (0–127) to gui.KeyCode.
// Layout-independent physical key positions.
// Source: Carbon/HIToolbox/Events.h kVK_* constants.
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
	0x38: gui.KeyLeftAlt,    // Left Option
	0x39: gui.KeyLeftControl,
	0x3A: gui.KeyRightAlt, // Right Option
	0x3B: gui.KeyRightControl,
	0x3C: gui.KeyRightShift,
	0x3E: gui.KeyLeftShift,
	0x3F: gui.KeyF17,
	0x40: gui.KeyF18,
	0x41: gui.KeyKPAdd,     // Keypad .
	0x43: gui.KeyKPDecimal, // Keypad *
	0x45: gui.KeyKPAdd,     // Keypad +
	0x47: gui.KeyNumLock,   // Clear (keypad num lock)
	0x48: gui.KeyInvalid,   // VolumeUp (no gui constant yet)
	0x49: gui.KeyInvalid,   // VolumeDown
	0x4A: gui.KeyInvalid,   // Mute
	0x4B: gui.KeyKPDivide,  // Keypad /
	0x4C: gui.KeyKPEnter,
	0x4E: gui.KeyKPSubtract, // Keypad -
	0x4F: gui.KeyF18,        // (clashes, rare)
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
