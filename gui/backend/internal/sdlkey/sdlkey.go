//go:build !js && !ios

package sdlkey

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/veandco/go-sdl2/sdl"
)

// MapMouseButton maps an SDL mouse button to gui.MouseButton.
func MapMouseButton(b uint8) gui.MouseButton {
	switch b {
	case sdl.BUTTON_LEFT:
		return gui.MouseLeft
	case sdl.BUTTON_RIGHT:
		return gui.MouseRight
	case sdl.BUTTON_MIDDLE:
		return gui.MouseMiddle
	default:
		return gui.MouseInvalid
	}
}

// MapMouseButtons merges SDL mouse button state into a
// gui.Modifier bitmask. The state parameter is the button
// mask from SDL mouse-motion events.
func MapMouseButtons(state uint32) gui.Modifier {
	var m gui.Modifier
	if state&sdl.ButtonLMask() != 0 {
		m |= gui.ModLMB
	}
	if state&sdl.ButtonRMask() != 0 {
		m |= gui.ModRMB
	}
	if state&sdl.ButtonMMask() != 0 {
		m |= gui.ModMMB
	}
	return m
}

// MapKeyMod maps SDL key modifiers to gui.Modifier.
func MapKeyMod(mod sdl.Keymod) gui.Modifier {
	var m gui.Modifier
	if mod&sdl.KMOD_SHIFT != 0 {
		m |= gui.ModShift
	}
	if mod&sdl.KMOD_CTRL != 0 {
		m |= gui.ModCtrl
	}
	if mod&sdl.KMOD_ALT != 0 {
		m |= gui.ModAlt
	}
	if mod&sdl.KMOD_GUI != 0 {
		m |= gui.ModSuper
	}
	return m
}

// MapKeyCode maps an SDL keycode to gui.KeyCode.
//
//nolint:gocyclo // key-mapping switch
func MapKeyCode(sym sdl.Keycode) gui.KeyCode {
	// Printable ASCII range (a-z → A-Z for GLFW compat).
	if sym >= 'a' && sym <= 'z' {
		return gui.KeyCode(sym - 32)
	}
	if sym >= '0' && sym <= '9' {
		return gui.KeyCode(sym)
	}

	switch sym {
	case sdl.K_SPACE:
		return gui.KeySpace
	case sdl.K_RETURN, sdl.K_RETURN2, sdl.K_KP_ENTER:
		return gui.KeyEnter
	case sdl.K_ESCAPE:
		return gui.KeyEscape
	case sdl.K_TAB:
		return gui.KeyTab
	case sdl.K_BACKSPACE:
		return gui.KeyBackspace
	case sdl.K_DELETE:
		return gui.KeyDelete
	case sdl.K_INSERT:
		return gui.KeyInsert
	case sdl.K_RIGHT:
		return gui.KeyRight
	case sdl.K_LEFT:
		return gui.KeyLeft
	case sdl.K_DOWN:
		return gui.KeyDown
	case sdl.K_UP:
		return gui.KeyUp
	case sdl.K_PAGEUP:
		return gui.KeyPageUp
	case sdl.K_PAGEDOWN:
		return gui.KeyPageDown
	case sdl.K_HOME:
		return gui.KeyHome
	case sdl.K_END:
		return gui.KeyEnd
	case sdl.K_LSHIFT:
		return gui.KeyLeftShift
	case sdl.K_RSHIFT:
		return gui.KeyRightShift
	case sdl.K_LCTRL:
		return gui.KeyLeftControl
	case sdl.K_RCTRL:
		return gui.KeyRightControl
	case sdl.K_LALT:
		return gui.KeyLeftAlt
	case sdl.K_RALT:
		return gui.KeyRightAlt
	case sdl.K_LGUI:
		return gui.KeyLeftSuper
	case sdl.K_RGUI:
		return gui.KeyRightSuper
	case sdl.K_COMMA:
		return gui.KeyComma
	case sdl.K_MINUS:
		return gui.KeyMinus
	case sdl.K_PERIOD:
		return gui.KeyPeriod
	case sdl.K_SLASH:
		return gui.KeySlash
	case sdl.K_SEMICOLON:
		return gui.KeySemicolon
	case sdl.K_EQUALS:
		return gui.KeyEqual
	case sdl.K_LEFTBRACKET:
		return gui.KeyLeftBracket
	case sdl.K_BACKSLASH:
		return gui.KeyBackslash
	case sdl.K_RIGHTBRACKET:
		return gui.KeyRightBracket
	case sdl.K_BACKQUOTE:
		return gui.KeyGraveAccent
	case sdl.K_CAPSLOCK:
		return gui.KeyCapsLock
	case sdl.K_F1:
		return gui.KeyF1
	case sdl.K_F2:
		return gui.KeyF2
	case sdl.K_F3:
		return gui.KeyF3
	case sdl.K_F4:
		return gui.KeyF4
	case sdl.K_F5:
		return gui.KeyF5
	case sdl.K_F6:
		return gui.KeyF6
	case sdl.K_F7:
		return gui.KeyF7
	case sdl.K_F8:
		return gui.KeyF8
	case sdl.K_F9:
		return gui.KeyF9
	case sdl.K_F10:
		return gui.KeyF10
	case sdl.K_F11:
		return gui.KeyF11
	case sdl.K_F12:
		return gui.KeyF12
	default:
		return gui.KeyInvalid
	}
}
