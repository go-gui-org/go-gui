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
		{0xff95, gui.KeyHome},   // KP_Home: NumLock is already applied
		{0xff9f, gui.KeyDelete}, // KP_Delete
	}
	for _, c := range cases {
		if got := MapKeySym(c.sym); got != c.want {
			t.Errorf("MapKeySym(0x%x) = %v, want %v", c.sym, got, c.want)
		}
	}
}

// TestKeyCodeRules covers each rule of docs/specs/keyboard-layouts.md § 2
// with keysyms only, as the X11 path will feed them.
func TestKeyCodeRules(t *testing.T) {
	cases := []struct {
		name string
		in   KeyInput
		want gui.KeyCode
	}{
		{"1: Num_Lock", KeyInput{Plain: xkNumLock}, gui.KeyNumLock},
		{"1: Menu", KeyInput{Plain: xkMenu}, gui.KeyMenu},
		{"1: F25", KeyInput{Plain: xkF25}, gui.KeyF25},
		{"1: AltGr on right Alt", KeyInput{Plain: xkISOLevel3Shift, Evdev: evdevRightAlt}, gui.KeyRightAlt},
		{"1: AltGr elsewhere", KeyInput{Plain: xkISOLevel3Shift, Evdev: 86}, gui.KeyInvalid},
		{"2: KP_7", KeyInput{Plain: xkKP0 + 7}, gui.KeyKP7},
		{"2: KP_Home", KeyInput{Plain: xkKPHome}, gui.KeyHome},
		{"2: KP_Separator", KeyInput{Plain: xkKPSeparator}, gui.KeyKPDecimal},
		{"2: KP_Begin", KeyInput{Plain: 0xff9d}, gui.KeyInvalid},
		{"2: KP_Enter", KeyInput{Plain: xkKPEnter}, gui.KeyEnter},
		{"3: letter", KeyInput{Plain: 'q', Shifted: 'Q'}, gui.KeyQ},
		{"4: AZERTY digit", KeyInput{Plain: '&', Shifted: '1'}, gui.Key1},
		{"5: Latin layout", KeyInput{Plain: 0x6f3, Latin: 'j', NonLatin: true, Evdev: 46}, gui.KeyJ},
		{"5: US position", KeyInput{Plain: 0x6f3, NonLatin: true, Evdev: 46}, gui.KeyC},
		{"5: not a letter key", KeyInput{Plain: 0x6c8, NonLatin: true, Evdev: 26}, gui.KeyInvalid},
		{"6: punctuation", KeyInput{Plain: '#', Shifted: '\''}, gui.KeyNumberSign},
		{"6: non-Latin punctuation", KeyInput{Plain: '/', NonLatin: true, Evdev: 86}, gui.KeySlash},
		{"7: dead_circumflex", KeyInput{Plain: xkDeadCircumflex}, gui.KeyCircumflex},
		{"7: dead_acute", KeyInput{Plain: 0xfe51}, gui.KeyInvalid},
		{"8: Latin-1 letter", KeyInput{Plain: 0xe4}, gui.KeyInvalid}, // ä
	}
	for _, c := range cases {
		if got := KeyCode(c.in); got != c.want {
			t.Errorf("%s: KeyCode(%+v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

func TestMapModifiers(t *testing.T) {
	if m := MapModifiers(maskShift | maskControl); m&gui.ModShift == 0 || m&gui.ModCtrl == 0 {
		t.Errorf("shift+ctrl not mapped: %v", m)
	}
	if m := MapModifiers(maskMod5); m != gui.ModAltGr {
		t.Errorf("Mod5 mapped to %v, want ModAltGr", m)
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
	// Back and forward must not read as left (#1098).
	if MapButton(8) != gui.MouseInvalid || MapButton(9) != gui.MouseInvalid {
		t.Error("side buttons map to a real button")
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
	for sym, want := range map[uint32]rune{
		0xff80: ' ', // KP_Space
		0xffaa: '*', // KP_Multiply
		0xffac: ',', // KP_Separator: the German numpad comma
		0xffae: '.', // KP_Decimal
		0xffaf: '/', // KP_Divide
		0xffb0: '0', // KP_0
		0xffb9: '9', // KP_9
		0xffbd: '=', // KP_Equal
		0x20ac: '€', // EuroSign: AltGr+E on German
		0x20a0: 0,   // EcuSign: libxkbcommon types no character either
		0xff8d: 0,   // KP_Enter: a key, not text
		0xff89: 0,   // KP_Tab
		0xff95: 0,   // KP_Home: numpad 7 with NumLock off
		0xffbe: 0,   // F1, next to KP_Equal
	} {
		if got := KeysymToRune(sym); got != want {
			t.Errorf("KeysymToRune(%#x) = %q, want %q", sym, got, want)
		}
	}
}
