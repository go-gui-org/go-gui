//go:build linux

package x11key

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMapKeySym(t *testing.T) {
	cases := []struct {
		sym  uint32
		want gui.KeyCode
	}{
		{'a', gui.KeyCode('A')}, // lowercase normalized to uppercase
		{'A', gui.KeyCode('A')}, // already uppercase
		{'z', gui.KeyCode('Z')},
		{'5', gui.KeyCode('5')},
		{0xff0d, gui.KeyEnter},  // XK_Return
		{0xff1b, gui.KeyEscape}, // XK_Escape
		{0xff08, gui.KeyBackspace},
		{0xff51, gui.KeyLeft},
		{0xff54, gui.KeyDown},
		{0xffbe, gui.KeyF1},
		{0xffc9, gui.KeyF12},
		{0xffe1, gui.KeyLeftShift},
		{' ', gui.KeySpace},
		{',', gui.KeyComma},
		{0x0, gui.KeyInvalid},
	}
	for _, c := range cases {
		if got := MapKeySym(c.sym); got != c.want {
			t.Errorf("MapKeySym(0x%x) = %v, want %v", c.sym, got, c.want)
		}
	}
}

func TestMapModifiers(t *testing.T) {
	if m := MapModifiers(maskShift | maskControl); m&gui.ModShift == 0 || m&gui.ModCtrl == 0 {
		t.Errorf("shift+ctrl not mapped: %v", m)
	}
	if m := MapModifiers(maskButton1 | maskButton3); m&gui.ModLMB == 0 || m&gui.ModRMB == 0 {
		t.Errorf("button1+button3 not mapped: %v", m)
	}
	if m := MapModifiers(0); m != 0 {
		t.Errorf("empty state mapped to %v", m)
	}
}

func TestMapButton(t *testing.T) {
	if MapButton(1) != gui.MouseLeft || MapButton(2) != gui.MouseMiddle || MapButton(3) != gui.MouseRight {
		t.Error("button mapping wrong")
	}
}

func TestKeysymToRune(t *testing.T) {
	if KeysymToRune('A') != 'A' {
		t.Error("ASCII keysym should map to itself")
	}
	if KeysymToRune(0x010000e9) != 0x00e9 { // direct-unicode é
		t.Error("direct-unicode keysym decode failed")
	}
	if KeysymToRune(0xff0d) != 0 { // Return: non-printable
		t.Error("non-printable keysym should map to 0")
	}
}
