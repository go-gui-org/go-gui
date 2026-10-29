//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"github.com/go-gui-org/go-gui/gui/backend/internal/x11key"
	"github.com/go-gui-org/go-gui/gui/backend/internal/xkb"
)

// wideKeysymToRune is the character a keysym types, from libxkbcommon's
// table, which knows every script, when libxkbcommon is installed; else
// from x11key's, which knows Latin-1, keypad, Euro and Unicode
// keysyms. The X11 path
// uses it so a Cyrillic or Greek layout types there as on Wayland (#1188).
func wideKeysymToRune(sym uint32) rune {
	if xkb.Load() == nil {
		return xkb.KeysymToRune(sym)
	}
	return x11key.KeysymToRune(sym)
}

// wideKeysymToUpper is the capital of a letter keysym: libxkbcommon's, as
// its Caps Lock rules use, when it is installed; else shiftedKeysym's
// Latin-1 and Unicode conversion.
func wideKeysymToUpper(sym uint32) uint32 {
	if xkb.Load() == nil {
		return xkb.KeysymToUpper(sym)
	}
	return shiftedKeysym(sym)
}
