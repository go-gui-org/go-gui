package ioskey

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
)

func TestKeyboardType(t *testing.T) {
	cases := []struct {
		kind gui.KeyboardKind
		want int
	}{
		{gui.KeyboardText, uiKeyboardTypeDefault},
		{gui.KeyboardNumber, uiKeyboardTypeNumberPad},
		{gui.KeyboardDecimal, uiKeyboardTypeDecimalPad},
		{gui.KeyboardPhone, uiKeyboardTypePhonePad},
		{gui.KeyboardEmail, uiKeyboardTypeEmailAddress},
		{gui.KeyboardURL, uiKeyboardTypeURL},
		// KeyboardNone never reaches UIKit as a type (the view hides the
		// keyboard instead), and an unknown kind falls back to text.
		{gui.KeyboardNone, uiKeyboardTypeDefault},
		{gui.KeyboardKind(200), uiKeyboardTypeDefault},
	}
	for _, c := range cases {
		if got := KeyboardType(c.kind); got != c.want {
			t.Errorf("KeyboardType(%v) = %d, want %d", c.kind, got, c.want)
		}
	}
}

func TestTextEventPlain(t *testing.T) {
	e, ok := TextEvent("héllo")
	if !ok {
		t.Fatal("TextEvent(plain) not ok")
	}
	if e.Type != gui.EventChar || e.IMEText != "héllo" || e.CharCode != 'h' {
		t.Errorf("got %+v, want EventChar 'h' / %q", e, "héllo")
	}
}

func TestTextEventNewlineIsEnter(t *testing.T) {
	for _, s := range []string{"\n", "\r", "\r\n"} {
		e, ok := TextEvent(s)
		if !ok {
			t.Fatalf("TextEvent(%q) not ok", s)
		}
		if e.Type != gui.EventKeyDown || e.KeyCode != gui.KeyEnter {
			t.Errorf("TextEvent(%q) = %+v, want KeyDown Enter", s, e)
		}
	}
}

func TestTextEventDropsEmptyAndInvalid(t *testing.T) {
	for _, s := range []string{"", "�", string([]byte{0xff, 0xfe})} {
		if e, ok := TextEvent(s); ok {
			t.Errorf("TextEvent(%q) = %+v, want dropped", s, e)
		}
	}
	e, ok := TextEvent("a�b")
	if !ok || e.IMEText != "ab" {
		t.Errorf("TextEvent(a\\uFFFDb) = %+v, %v; want IMEText \"ab\"", e, ok)
	}
}

func TestTextEventCapsLength(t *testing.T) {
	e, ok := TextEvent(strings.Repeat("あ", maxTextRunes+10))
	if !ok {
		t.Fatal("TextEvent(long) not ok")
	}
	if n := utf8.RuneCountInString(e.IMEText); n != maxTextRunes {
		t.Errorf("rune count = %d, want %d", n, maxTextRunes)
	}
}

func TestBackspaceEvent(t *testing.T) {
	e := BackspaceEvent()
	if e.Type != gui.EventKeyDown || e.KeyCode != gui.KeyBackspace {
		t.Errorf("BackspaceEvent() = %+v, want KeyDown Backspace", e)
	}
}

func TestHIDKeyNavigation(t *testing.T) {
	cases := map[int]gui.KeyCode{
		hidEscape:        gui.KeyEscape,
		hidTab:           gui.KeyTab,
		hidInsert:        gui.KeyInsert,
		hidHome:          gui.KeyHome,
		hidPageUp:        gui.KeyPageUp,
		hidDeleteForward: gui.KeyDelete,
		hidEnd:           gui.KeyEnd,
		hidPageDown:      gui.KeyPageDown,
		hidRightArrow:    gui.KeyRight,
		hidLeftArrow:     gui.KeyLeft,
		hidDownArrow:     gui.KeyDown,
		hidUpArrow:       gui.KeyUp,
		hidF1:            gui.KeyF1,
		hidF12:           gui.KeyF12,
	}
	for usage, want := range cases {
		got, text := HIDKey(usage)
		if got != want {
			t.Errorf("HIDKey(0x%x) = %v, want %v", usage, got, want)
		}
		if text {
			t.Errorf("HIDKey(0x%x) reports text, want non-text", usage)
		}
	}
}

func TestHIDKeyText(t *testing.T) {
	cases := map[int]gui.KeyCode{
		hidA:            gui.KeyA,
		hidA + 25:       gui.KeyZ,
		hid1:            gui.Key1,
		hid1 + 8:        gui.Key9,
		hid0:            gui.Key0,
		hidSpace:        gui.KeySpace,
		hidMinus:        gui.KeyMinus,
		hidEqual:        gui.KeyEqual,
		hidLeftBracket:  gui.KeyLeftBracket,
		hidRightBracket: gui.KeyRightBracket,
		hidBackslash:    gui.KeyBackslash,
		hidSemicolon:    gui.KeySemicolon,
		hidApostrophe:   gui.KeyApostrophe,
		hidGrave:        gui.KeyGraveAccent,
		hidComma:        gui.KeyComma,
		hidPeriod:       gui.KeyPeriod,
		hidSlash:        gui.KeySlash,
		// UIKit types these through UIKeyInput while editing.
		hidReturn:    gui.KeyEnter,
		hidBackspace: gui.KeyBackspace,
	}
	for usage, want := range cases {
		got, text := HIDKey(usage)
		if got != want {
			t.Errorf("HIDKey(0x%x) = %v, want %v", usage, got, want)
		}
		if !text {
			t.Errorf("HIDKey(0x%x) reports non-text, want text", usage)
		}
	}
}

func TestHIDKeyUnknown(t *testing.T) {
	for _, usage := range []int{0, -1, 0x99, 0xFFFF} {
		if got, _ := HIDKey(usage); got != gui.KeyInvalid {
			t.Errorf("HIDKey(0x%x) = %v, want KeyInvalid", usage, got)
		}
	}
}

func TestModifiers(t *testing.T) {
	cases := []struct {
		flags int
		want  gui.Modifier
	}{
		{0, gui.ModNone},
		{uiKeyModifierShift, gui.ModShift},
		{uiKeyModifierControl, gui.ModCtrl},
		{uiKeyModifierAlternate, gui.ModAlt},
		{uiKeyModifierCommand, gui.ModSuper},
		{uiKeyModifierShift | uiKeyModifierCommand, gui.ModShift | gui.ModSuper},
		// Caps lock and numeric-pad flags carry no gui modifier.
		{uiKeyModifierAlphaShift | uiKeyModifierNumericPad, gui.ModNone},
	}
	for _, c := range cases {
		if got := Modifiers(c.flags); got != c.want {
			t.Errorf("Modifiers(0x%x) = %v, want %v", c.flags, got, c.want)
		}
	}
}

func TestKeyEventPressWhileEditing(t *testing.T) {
	cases := []struct {
		name   string
		usage  int
		flags  int
		wantGo bool
	}{
		{"arrow", hidLeftArrow, 0, true},
		{"shift arrow", hidLeftArrow, uiKeyModifierShift, true},
		{"tab", hidTab, 0, true},
		{"plain letter", hidA, 0, false},
		{"shift letter", hidA, uiKeyModifierShift, false},
		{"option letter", hidA, uiKeyModifierAlternate, false},
		{"cmd letter", hidA, uiKeyModifierCommand, true},
		{"ctrl letter", hidA, uiKeyModifierControl, true},
		// Return and Backspace arrive through insertText: and
		// deleteBackward; taking them here too would double them.
		{"return", hidReturn, 0, false},
		{"backspace", hidBackspace, 0, false},
		{"cmd backspace", hidBackspace, uiKeyModifierCommand, true},
		{"unknown", 0x99, uiKeyModifierCommand, false},
	}
	for _, c := range cases {
		e, ok := KeyEvent(c.usage, c.flags, true, true)
		if ok != c.wantGo {
			t.Errorf("%s: KeyEvent ok = %v, want %v", c.name, ok, c.wantGo)
			continue
		}
		if ok && (e.Type != gui.EventKeyDown || e.Modifiers != Modifiers(c.flags)) {
			t.Errorf("%s: KeyEvent = %+v", c.name, e)
		}
	}
}

// With no text field active nothing types through UIKeyInput, so every
// mapped key is a key event: Space and Return press a focused button.
func TestKeyEventPressNotEditing(t *testing.T) {
	for _, usage := range []int{hidA, hidSpace, hidReturn, hidBackspace, hidTab} {
		e, ok := KeyEvent(usage, 0, true, false)
		want, _ := HIDKey(usage)
		if !ok || e.Type != gui.EventKeyDown || e.KeyCode != want {
			t.Errorf("KeyEvent(0x%x, not editing) = %+v, %v", usage, e, ok)
		}
	}
	if _, ok := KeyEvent(0x99, 0, true, false); ok {
		t.Error("KeyEvent(unknown, not editing) ok, want not ok")
	}
}

func TestKeyEventUp(t *testing.T) {
	e, ok := KeyEvent(hidUpArrow, 0, false, true)
	if !ok || e.Type != gui.EventKeyUp || e.KeyCode != gui.KeyUp {
		t.Errorf("KeyEvent(up, release) = %+v, %v", e, ok)
	}
	// Cmd+C with Cmd lifted first: the C release carries no modifier and
	// must still reach Go, which took the press.
	e, ok = KeyEvent(hidA+2, 0, false, true)
	if !ok || e.Type != gui.EventKeyUp || e.KeyCode != gui.KeyC {
		t.Errorf("KeyEvent(C, release, no mods) = %+v, %v", e, ok)
	}
}
