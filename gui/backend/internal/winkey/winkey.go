//go:build windows

// Package winkey maps Win32 virtual-key codes and modifier state to
// go-gui key and modifier values. It mirrors the x11key package for
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

// Key-message lParam fields.
const (
	lpScancodeShift = 16
	lpScancodeMask  = 0xFF
	lpExtendedBit   = 1 << 24

	scRShift = 0x36 // right Shift scancode; left is 0x2A
)

// MapKey maps a WM_KEYDOWN / WM_KEYUP message to a gui.KeyCode.
// Key messages carry the generic VK_SHIFT, VK_CONTROL and VK_MENU in
// wparam, never the sided VK_L*/VK_R* codes, so the side comes from
// lparam: the extended-key bit marks right Ctrl and right Alt, and
// the scancode tells the two Shift keys apart (#1007).
func MapKey(wparam, lparam uintptr) gui.KeyCode {
	switch wparam {
	case vkShift:
		if lparam>>lpScancodeShift&lpScancodeMask == scRShift {
			return gui.KeyRightShift
		}
		return gui.KeyLeftShift
	case vkControl:
		if lparam&lpExtendedBit != 0 {
			return gui.KeyRightControl
		}
		return gui.KeyLeftControl
	case vkMenu:
		if lparam&lpExtendedBit != 0 {
			return gui.KeyRightAlt
		}
		return gui.KeyLeftAlt
	}
	return MapVKey(wparam)
}

// Win32 key messages.
const (
	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
)

// IsAltGrCtrl reports whether a key message is the fake left Ctrl that
// Windows sends ahead of AltGr (right Alt) on both press and release.
// next* describe the message queued after it. The fake one has the same
// message time as a right Alt message that follows it. This matches GLFW.
func IsAltGrCtrl(wparam, lparam uintptr, time uint32,
	nextMsg uint32, nextW, nextL uintptr, nextTime uint32) bool {
	if wparam != vkControl || lparam&lpExtendedBit != 0 {
		return false
	}
	switch nextMsg {
	case wmKeyDown, wmSysKeyDown, wmKeyUp, wmSysKeyUp:
	default:
		return false
	}
	return nextW == vkMenu && nextL&lpExtendedBit != 0 && nextTime == time
}

// heldKeys lists the keys HeldKeys tracks, with the sided virtual-key
// code GetKeyState reads each one by. Index i is bit i of HeldKeys.down.
var heldKeys = [...]struct {
	key gui.KeyCode
	vk  uintptr
}{
	{gui.KeyLeftShift, vkLShift},
	{gui.KeyRightShift, vkRShift},
	{gui.KeyLeftSuper, vkLWin},
	{gui.KeyRightSuper, vkRWin},
}

// HeldKeys tracks the keys whose key-up Windows can drop, by which of
// them the app has been told are down. GLFW tracks the same ones.
//
//   - With both Shift keys held, Windows sends no key-up for the first
//     release; one key-up arrives when the second key is released.
//     Release answers it with a key-up for both sides.
//   - After the Win+V hotkey, Windows sends no key-up for the Win key.
//
// ReleaseStale sends a dropped key-up once the message loop finds the
// key physically up.
type HeldKeys struct{ down uint8 }

// heldIndex is k's index in heldKeys, or -1 when k is not tracked.
func heldIndex(k gui.KeyCode) int {
	for i := range heldKeys {
		if heldKeys[i].key == k {
			return i
		}
	}
	return -1
}

// Press records a key-down. Untracked keys are ignored.
func (h *HeldKeys) Press(k gui.KeyCode) {
	if i := heldIndex(k); i >= 0 {
		h.down |= 1 << i
	}
}

// Held reports whether any tracked key is recorded as down.
func (h *HeldKeys) Held() bool { return h.down != 0 }

// Release answers a key-up for key k. It returns k, then for a Shift
// key the other Shift if held, and clears what it returns. k is
// returned even when it was not recorded down, so a key-up whose
// key-down went elsewhere still arrives as it did before.
func (h *HeldKeys) Release(k gui.KeyCode) (keys [len(heldKeys)]gui.KeyCode, n int) {
	keys[0], n = k, 1
	if i := heldIndex(k); i >= 0 {
		h.down &^= 1 << i
	}
	var other gui.KeyCode
	switch k {
	case gui.KeyLeftShift:
		other = gui.KeyRightShift
	case gui.KeyRightShift:
		other = gui.KeyLeftShift
	default:
		return keys, n
	}
	if i := heldIndex(other); h.down&(1<<i) != 0 {
		h.down &^= 1 << i
		keys[1], n = other, 2
	}
	return keys, n
}

// ReleaseAll returns every held key and clears them, for focus loss.
func (h *HeldKeys) ReleaseAll() (keys [len(heldKeys)]gui.KeyCode, n int) {
	return h.releaseStale(func(uintptr) bool { return false })
}

// ReleaseStale returns each held key that is physically up and clears it.
func (h *HeldKeys) ReleaseStale() (keys [len(heldKeys)]gui.KeyCode, n int) {
	return h.releaseStale(keyDown)
}

func (h *HeldKeys) releaseStale(
	isDown func(vk uintptr) bool,
) (keys [len(heldKeys)]gui.KeyCode, n int) {
	for i, hk := range heldKeys {
		if h.down&(1<<i) == 0 || isDown(hk.vk) {
			continue
		}
		h.down &^= 1 << i
		keys[n] = hk.key
		n++
	}
	return keys, n
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
