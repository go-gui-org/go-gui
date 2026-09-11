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
