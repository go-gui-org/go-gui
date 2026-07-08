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
