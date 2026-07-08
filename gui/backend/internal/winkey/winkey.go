//go:build windows

// Package winkey maps Win32 virtual-key codes and modifier state to
// go-gui key and modifier values. It mirrors the sdlkey package for
// the native Windows backend.
package winkey

import (
	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	user32       = windows.NewLazySystemDLL("user32.dll")
	pGetKeyState = user32.NewProc("GetKeyState")
)

// Win32 virtual-key codes (subset).
const (
	vkBack    = 0x08
	vkTab     = 0x09
	vkReturn  = 0x0D
	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12 // Alt
	vkCapital = 0x14
	vkEscape  = 0x1B
	vkSpace   = 0x20
	vkPrior   = 0x21 // PageUp
	vkNext    = 0x22 // PageDown
	vkEnd     = 0x23
	vkHome    = 0x24
	vkLeft    = 0x25
	vkUp      = 0x26
	vkRight   = 0x27
	vkDown    = 0x28
	vkInsert  = 0x2D
	vkDelete  = 0x2E

	vkLWin     = 0x5B
	vkRWin     = 0x5C
	vkLShift   = 0xA0
	vkRShift   = 0xA1
	vkLControl = 0xA2
	vkRControl = 0xA3
	vkLMenu    = 0xA4
	vkRMenu    = 0xA5

	vkOEM1      = 0xBA // ;:
	vkOEMPlus   = 0xBB // =+
	vkOEMComma  = 0xBC // ,<
	vkOEMMinus  = 0xBD // -_
	vkOEMPeriod = 0xBE // .>
	vkOEM2      = 0xBF // /?
	vkOEM3      = 0xC0 // `~
	vkOEM4      = 0xDB // [{
	vkOEM5      = 0xDC // \|
	vkOEM6      = 0xDD // ]}

	vkF1  = 0x70
	vkF12 = 0x7B

	// Mouse button flags in WM_MOUSEMOVE / button-event wparam.
	mkLButton = 0x0001
	mkRButton = 0x0002
	mkMButton = 0x0010
)

// MapVKey maps a Win32 virtual-key code to a gui.KeyCode.
//
//nolint:gocyclo // key-mapping switch
func MapVKey(vk uintptr) gui.KeyCode {
	// A-Z and 0-9 share ASCII values with gui.KeyCode.
	if vk >= 'A' && vk <= 'Z' {
		return gui.KeyCode(vk)
	}
	if vk >= '0' && vk <= '9' {
		return gui.KeyCode(vk)
	}
	if vk >= vkF1 && vk <= vkF12 {
		return gui.KeyF1 + gui.KeyCode(vk-vkF1)
	}

	switch vk {
	case vkSpace:
		return gui.KeySpace
	case vkReturn:
		return gui.KeyEnter
	case vkEscape:
		return gui.KeyEscape
	case vkTab:
		return gui.KeyTab
	case vkBack:
		return gui.KeyBackspace
	case vkDelete:
		return gui.KeyDelete
	case vkInsert:
		return gui.KeyInsert
	case vkRight:
		return gui.KeyRight
	case vkLeft:
		return gui.KeyLeft
	case vkDown:
		return gui.KeyDown
	case vkUp:
		return gui.KeyUp
	case vkPrior:
		return gui.KeyPageUp
	case vkNext:
		return gui.KeyPageDown
	case vkHome:
		return gui.KeyHome
	case vkEnd:
		return gui.KeyEnd
	case vkShift, vkLShift:
		return gui.KeyLeftShift
	case vkRShift:
		return gui.KeyRightShift
	case vkControl, vkLControl:
		return gui.KeyLeftControl
	case vkRControl:
		return gui.KeyRightControl
	case vkMenu, vkLMenu:
		return gui.KeyLeftAlt
	case vkRMenu:
		return gui.KeyRightAlt
	case vkLWin:
		return gui.KeyLeftSuper
	case vkRWin:
		return gui.KeyRightSuper
	case vkOEMComma:
		return gui.KeyComma
	case vkOEMMinus:
		return gui.KeyMinus
	case vkOEMPeriod:
		return gui.KeyPeriod
	case vkOEM2:
		return gui.KeySlash
	case vkOEM1:
		return gui.KeySemicolon
	case vkOEMPlus:
		return gui.KeyEqual
	case vkOEM4:
		return gui.KeyLeftBracket
	case vkOEM5:
		return gui.KeyBackslash
	case vkOEM6:
		return gui.KeyRightBracket
	case vkOEM3:
		return gui.KeyGraveAccent
	case vkCapital:
		return gui.KeyCapsLock
	default:
		return gui.KeyInvalid
	}
}

func keyDown(vk uintptr) bool {
	r, _, _ := pGetKeyState.Call(vk)
	return uint16(r)&0x8000 != 0
}

// ModState returns the current keyboard modifier bitmask.
func ModState() gui.Modifier {
	var m gui.Modifier
	if keyDown(vkShift) {
		m |= gui.ModShift
	}
	if keyDown(vkControl) {
		m |= gui.ModCtrl
	}
	if keyDown(vkMenu) {
		m |= gui.ModAlt
	}
	if keyDown(vkLWin) || keyDown(vkRWin) {
		m |= gui.ModSuper
	}
	return m
}

// MouseButtons maps the MK_* button flags from a mouse-message
// wparam into gui mouse-button modifiers.
func MouseButtons(wparam uintptr) gui.Modifier {
	var m gui.Modifier
	if wparam&mkLButton != 0 {
		m |= gui.ModLMB
	}
	if wparam&mkRButton != 0 {
		m |= gui.ModRMB
	}
	if wparam&mkMButton != 0 {
		m |= gui.ModMMB
	}
	return m
}
