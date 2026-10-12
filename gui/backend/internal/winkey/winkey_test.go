//go:build windows

package winkey

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMapVKeyLettersAndDigits(t *testing.T) {
	if got := MapVKey('A'); got != gui.KeyCode('A') {
		t.Errorf("MapVKey('A') = %v, want %v", got, gui.KeyCode('A'))
	}
	if got := MapVKey('Z'); got != gui.KeyCode('Z') {
		t.Errorf("MapVKey('Z') = %v, want %v", got, gui.KeyCode('Z'))
	}
	if got := MapVKey('0'); got != gui.KeyCode('0') {
		t.Errorf("MapVKey('0') = %v, want %v", got, gui.KeyCode('0'))
	}
	if got := MapVKey('9'); got != gui.KeyCode('9') {
		t.Errorf("MapVKey('9') = %v, want %v", got, gui.KeyCode('9'))
	}
}

func TestMapVKeyFunctionKeys(t *testing.T) {
	want := []gui.KeyCode{
		gui.KeyF1, gui.KeyF2, gui.KeyF3, gui.KeyF4,
		gui.KeyF5, gui.KeyF6, gui.KeyF7, gui.KeyF8,
		gui.KeyF9, gui.KeyF10, gui.KeyF11, gui.KeyF12,
	}
	for i, exp := range want {
		vk := uintptr(vkF1 + i)
		if got := MapVKey(vk); got != exp {
			t.Errorf("MapVKey(F%d) = %v, want %v", i+1, got, exp)
		}
	}
}

func TestMapVKeyModifiers(t *testing.T) {
	tests := []struct {
		vk   uintptr
		want gui.KeyCode
	}{
		{vkShift, gui.KeyLeftShift},
		{vkLShift, gui.KeyLeftShift},
		{vkRShift, gui.KeyRightShift},
		{vkControl, gui.KeyLeftControl},
		{vkLControl, gui.KeyLeftControl},
		{vkRControl, gui.KeyRightControl},
		{vkMenu, gui.KeyLeftAlt},
		{vkLMenu, gui.KeyLeftAlt},
		{vkRMenu, gui.KeyRightAlt},
		{vkLWin, gui.KeyLeftSuper},
		{vkRWin, gui.KeyRightSuper},
		{vkCapital, gui.KeyCapsLock},
	}
	for _, tt := range tests {
		if got := MapVKey(tt.vk); got != tt.want {
			t.Errorf("MapVKey(%#x) = %v, want %v", tt.vk, got, tt.want)
		}
	}
}

func TestMapVKeyNavigationAndEditing(t *testing.T) {
	tests := []struct {
		vk   uintptr
		want gui.KeyCode
	}{
		{vkSpace, gui.KeySpace},
		{vkReturn, gui.KeyEnter},
		{vkEscape, gui.KeyEscape},
		{vkTab, gui.KeyTab},
		{vkBack, gui.KeyBackspace},
		{vkDelete, gui.KeyDelete},
		{vkInsert, gui.KeyInsert},
		{vkLeft, gui.KeyLeft},
		{vkRight, gui.KeyRight},
		{vkUp, gui.KeyUp},
		{vkDown, gui.KeyDown},
		{vkPrior, gui.KeyPageUp},
		{vkNext, gui.KeyPageDown},
		{vkHome, gui.KeyHome},
		{vkEnd, gui.KeyEnd},
	}
	for _, tt := range tests {
		if got := MapVKey(tt.vk); got != tt.want {
			t.Errorf("MapVKey(%#x) = %v, want %v", tt.vk, got, tt.want)
		}
	}
}

func TestMapVKeyOEMPunctuation(t *testing.T) {
	tests := []struct {
		vk   uintptr
		want gui.KeyCode
	}{
		{vkOEMComma, gui.KeyComma},
		{vkOEMMinus, gui.KeyMinus},
		{vkOEMPeriod, gui.KeyPeriod},
		{vkOEM2, gui.KeySlash},
		{vkOEM1, gui.KeySemicolon},
		{vkOEMPlus, gui.KeyEqual},
		{vkOEM4, gui.KeyLeftBracket},
		{vkOEM5, gui.KeyBackslash},
		{vkOEM6, gui.KeyRightBracket},
		{vkOEM3, gui.KeyGraveAccent},
	}
	for _, tt := range tests {
		if got := MapVKey(tt.vk); got != tt.want {
			t.Errorf("MapVKey(%#x) = %v, want %v", tt.vk, got, tt.want)
		}
	}
}

func TestMapVKeyUnknown(t *testing.T) {
	// 0x07 is an undefined VK code.
	if got := MapVKey(0x07); got != gui.KeyInvalid {
		t.Errorf("MapVKey(0x07) = %v, want KeyInvalid", got)
	}
	if got := MapVKey(0xFF); got != gui.KeyInvalid {
		t.Errorf("MapVKey(0xFF) = %v, want KeyInvalid", got)
	}
}

func TestMouseButtons(t *testing.T) {
	tests := []struct {
		wparam uintptr
		want   gui.Modifier
	}{
		{0, 0},
		{mkLButton, gui.ModLMB},
		{mkRButton, gui.ModRMB},
		{mkMButton, gui.ModMMB},
		{mkLButton | mkRButton, gui.ModLMB | gui.ModRMB},
		{mkLButton | mkRButton | mkMButton, gui.ModLMB | gui.ModRMB | gui.ModMMB},
	}
	for _, tt := range tests {
		if got := MouseButtons(tt.wparam); got != tt.want {
			t.Errorf("MouseButtons(%#x) = %v, want %v", tt.wparam, got, tt.want)
		}
	}
}

// keyLParam builds a WM_KEYDOWN lParam from a scancode and the extended-key bit.
func keyLParam(scancode uintptr, extended bool) uintptr {
	lp := uintptr(1) | scancode<<16 // repeat count 1
	if extended {
		lp |= 1 << 24
	}
	return lp
}

func TestMapKeyModifierSides(t *testing.T) {
	tests := []struct {
		name   string
		wparam uintptr
		lparam uintptr
		want   gui.KeyCode
	}{
		{"left shift", vkShift, keyLParam(0x2A, false), gui.KeyLeftShift},
		{"right shift", vkShift, keyLParam(0x36, false), gui.KeyRightShift},
		{"left ctrl", vkControl, keyLParam(0x1D, false), gui.KeyLeftControl},
		{"right ctrl", vkControl, keyLParam(0x1D, true), gui.KeyRightControl},
		{"left alt", vkMenu, keyLParam(0x38, false), gui.KeyLeftAlt},
		{"right alt", vkMenu, keyLParam(0x38, true), gui.KeyRightAlt},
		{"key-up bits ignored", vkShift,
			keyLParam(0x36, false) | 0xC0000000, gui.KeyRightShift},
		{"non-modifier passes through", 'A', keyLParam(0x1E, false), gui.KeyCode('A')},
		{"extended arrow passes through", vkLeft, keyLParam(0x4B, true), gui.KeyLeft},
	}
	for _, tt := range tests {
		if got := MapKey(tt.wparam, tt.lparam); got != tt.want {
			t.Errorf("%s: MapKey(%#x, %#x) = %v, want %v",
				tt.name, tt.wparam, tt.lparam, got, tt.want)
		}
	}
}

// keysOf returns the first n keys as a slice for comparison.
func keysOf(keys [4]gui.KeyCode, n int) []gui.KeyCode {
	return append([]gui.KeyCode(nil), keys[:n]...)
}

func sameKeys(a, b []gui.KeyCode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestHeldKeysRelease pins the GLFW workaround: with both Shift keys
// held, Windows sends one key-up, on the second release. That key-up must
// release both sides. Other keys release only themselves.
func TestHeldKeysRelease(t *testing.T) {
	tests := []struct {
		name    string
		pressed []gui.KeyCode
		up      gui.KeyCode
		want    []gui.KeyCode
		stillUp []gui.KeyCode // held after the release
	}{
		{"left shift only", []gui.KeyCode{gui.KeyLeftShift},
			gui.KeyLeftShift, []gui.KeyCode{gui.KeyLeftShift}, nil},
		{"right shift only", []gui.KeyCode{gui.KeyRightShift},
			gui.KeyRightShift, []gui.KeyCode{gui.KeyRightShift}, nil},
		{"both shifts, right reported",
			[]gui.KeyCode{gui.KeyLeftShift, gui.KeyRightShift},
			gui.KeyRightShift, []gui.KeyCode{gui.KeyRightShift, gui.KeyLeftShift}, nil},
		{"both shifts, left reported",
			[]gui.KeyCode{gui.KeyRightShift, gui.KeyLeftShift},
			gui.KeyLeftShift, []gui.KeyCode{gui.KeyLeftShift, gui.KeyRightShift}, nil},
		{"none held keeps the reported key-up", nil,
			gui.KeyLeftShift, []gui.KeyCode{gui.KeyLeftShift}, nil},
		{"left win", []gui.KeyCode{gui.KeyLeftSuper},
			gui.KeyLeftSuper, []gui.KeyCode{gui.KeyLeftSuper}, nil},
		{"one win key leaves the other held",
			[]gui.KeyCode{gui.KeyLeftSuper, gui.KeyRightSuper},
			gui.KeyRightSuper, []gui.KeyCode{gui.KeyRightSuper},
			[]gui.KeyCode{gui.KeyLeftSuper}},
		{"shift up leaves win held",
			[]gui.KeyCode{gui.KeyLeftShift, gui.KeyLeftSuper},
			gui.KeyLeftShift, []gui.KeyCode{gui.KeyLeftShift},
			[]gui.KeyCode{gui.KeyLeftSuper}},
		{"untracked key", []gui.KeyCode{gui.KeyLeftShift},
			'A', []gui.KeyCode{'A'}, []gui.KeyCode{gui.KeyLeftShift}},
	}
	for _, tt := range tests {
		var h HeldKeys
		for _, k := range tt.pressed {
			h.Press(k)
		}
		keys, n := h.Release(tt.up)
		if got := keysOf(keys, n); !sameKeys(got, tt.want) {
			t.Errorf("%s: Release(%v) = %v, want %v", tt.name, tt.up, got, tt.want)
		}
		keys, n = h.ReleaseAll()
		if got := keysOf(keys, n); !sameKeys(got, tt.stillUp) {
			t.Errorf("%s: held after Release = %v, want %v", tt.name, got, tt.stillUp)
		}
	}
}

func TestHeldKeysPressIgnoresOtherKeys(t *testing.T) {
	var h HeldKeys
	h.Press(gui.KeyLeftControl)
	h.Press('A')
	if keys, n := h.ReleaseAll(); n != 0 {
		t.Errorf("untracked keys recorded: %v", keysOf(keys, n))
	}
}

// TestHeldKeysReleaseStale pins the poll half: a held key that is
// physically up gets its missing key-up. Windows drops the Shift key-up
// when both are held, and the Win key-up after the Win+V hotkey.
func TestHeldKeysReleaseStale(t *testing.T) {
	var h HeldKeys
	h.Press(gui.KeyLeftShift)
	h.Press(gui.KeyRightShift)
	h.Press(gui.KeyLeftSuper)
	h.Press(gui.KeyRightSuper)
	stillDown := func(vk uintptr) bool { return vk == vkRShift || vk == vkRWin }

	keys, n := h.releaseStale(stillDown)
	want := []gui.KeyCode{gui.KeyLeftShift, gui.KeyLeftSuper}
	if got := keysOf(keys, n); !sameKeys(got, want) {
		t.Fatalf("releaseStale = %v, want %v", got, want)
	}
	if keys, n = h.releaseStale(stillDown); n != 0 {
		t.Errorf("second releaseStale = %v, want none", keysOf(keys, n))
	}
	keys, n = h.ReleaseAll()
	want = []gui.KeyCode{gui.KeyRightShift, gui.KeyRightSuper}
	if got := keysOf(keys, n); !sameKeys(got, want) {
		t.Errorf("ReleaseAll = %v, want %v", got, want)
	}
}

func TestHeldKeysHeld(t *testing.T) {
	var h HeldKeys
	if h.Held() {
		t.Error("zero HeldKeys reports held")
	}
	h.Press(gui.KeyRightSuper)
	if !h.Held() {
		t.Error("pressed HeldKeys reports not held")
	}
}

func TestIsAltGrCtrl(t *testing.T) {
	const tm = 1000
	lctrl := keyLParam(0x1D, false)
	ralt := keyLParam(0x38, true)
	lalt := keyLParam(0x38, false)
	tests := []struct {
		name           string
		wparam, lparam uintptr
		nextMsg        uint32
		nextW, nextL   uintptr
		nextTime       uint32
		want           bool
	}{
		{"altgr down", vkControl, lctrl, wmKeyDown, vkMenu, ralt, tm, true},
		{"altgr sys down", vkControl, lctrl, wmSysKeyDown, vkMenu, ralt, tm, true},
		{"altgr up", vkControl, lctrl, wmKeyUp, vkMenu, ralt, tm, true},
		{"altgr sys up", vkControl, lctrl, wmSysKeyUp, vkMenu, ralt, tm, true},
		{"later right alt", vkControl, lctrl, wmKeyDown, vkMenu, ralt, tm + 1, false},
		{"left alt follows", vkControl, lctrl, wmKeyDown, vkMenu, lalt, tm, false},
		{"right ctrl", vkControl, keyLParam(0x1D, true), wmKeyDown, vkMenu, ralt, tm, false},
		{"not ctrl", vkShift, keyLParam(0x2A, false), wmKeyDown, vkMenu, ralt, tm, false},
		{"next not a key", vkControl, lctrl, 0x0200, vkMenu, ralt, tm, false},
		{"next is another key", vkControl, lctrl, wmKeyDown, 'A', ralt, tm, false},
	}
	for _, tt := range tests {
		got := IsAltGrCtrl(tt.wparam, tt.lparam, tm,
			tt.nextMsg, tt.nextW, tt.nextL, tt.nextTime)
		if got != tt.want {
			t.Errorf("%s: IsAltGrCtrl = %v, want %v", tt.name, got, tt.want)
		}
	}
}
