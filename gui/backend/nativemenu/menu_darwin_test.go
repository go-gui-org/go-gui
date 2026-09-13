//go:build darwin && cgo && !ios

package nativemenu

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestEncodeShortcut(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		in    gui.Shortcut
		wantC byte
		wantM int
	}{
		{"letter", gui.Shortcut{Key: gui.KeyA, Modifiers: gui.ModSuper}, 'A', 1},
		{"digit", gui.Shortcut{Key: gui.Key0, Modifiers: gui.ModSuper}, '0', 1},
		{"slash", gui.Shortcut{Key: gui.KeySlash, Modifiers: gui.ModSuper}, '/', 1},
		{"comma", gui.Shortcut{Key: gui.KeyComma, Modifiers: gui.ModSuper}, ',', 1},
		{
			"punctuation with shift",
			gui.Shortcut{Key: gui.KeyEqual, Modifiers: gui.ModSuper | gui.ModShift},
			'=', 3,
		},
		{"unset", gui.Shortcut{}, 0, 0},
		{"escape unsupported", gui.Shortcut{Key: gui.KeyEscape}, 0, 0},
		{"function key unsupported", gui.Shortcut{Key: gui.KeyF1}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotC, gotM := encodeShortcut(tt.in)
			if byte(gotC) != tt.wantC || int(gotM) != tt.wantM {
				t.Errorf("encodeShortcut(%v) = (%q, %d), want (%q, %d)",
					tt.in, byte(gotC), int(gotM), tt.wantC, tt.wantM)
			}
		})
	}
}

// TakeKeyEquivalent must hand out a key-equivalent record exactly once, keep
// key code 0 (kVK_ANSI_A) distinct from "none", and let a mouse-picked item
// clear a stale record.
func TestTakeKeyEquivalent(t *testing.T) {
	// Not parallel: the record is package state.
	TakeKeyEquivalent() // start clean

	if _, ok := TakeKeyEquivalent(); ok {
		t.Fatal("empty record: got ok=true, want false")
	}

	noteKeyEquivalent(0x2C) // kVK_ANSI_Slash
	code, ok := TakeKeyEquivalent()
	if !ok || code != 0x2C {
		t.Fatalf("after key equivalent: got (%#x, %v), want (0x2c, true)", code, ok)
	}
	if _, ok := TakeKeyEquivalent(); ok {
		t.Fatal("second take: got ok=true, want false (record must clear)")
	}

	noteKeyEquivalent(0) // kVK_ANSI_A
	if code, ok := TakeKeyEquivalent(); !ok || code != 0 {
		t.Fatalf("key code 0: got (%#x, %v), want (0, true)", code, ok)
	}

	noteKeyEquivalent(0x2C)
	noteKeyEquivalent(-1) // then a mouse pick
	if _, ok := TakeKeyEquivalent(); ok {
		t.Fatal("mouse pick after key: got ok=true, want record cleared")
	}

	noteKeyEquivalent(0xFFFF) // max uint16 stays valid
	if code, ok := TakeKeyEquivalent(); !ok || code != 0xFFFF {
		t.Fatalf("key code max: got (%#x, %v), want (0xffff, true)", code, ok)
	}

	noteKeyEquivalent(0x10000) // out of uint16 range must clear, not truncate
	if _, ok := TakeKeyEquivalent(); ok {
		t.Fatal("out-of-range key: got ok=true, want false (record cleared)")
	}

	noteKeyEquivalent(0x2C)
	noteKeyEquivalent(-100) // any negative clears, not only -1
	if _, ok := TakeKeyEquivalent(); ok {
		t.Fatal("negative key: got ok=true, want record cleared")
	}
}
