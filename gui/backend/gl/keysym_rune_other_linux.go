//go:build linux && !js && !android && !(amd64 || arm64)

package gl

import "github.com/go-gui-org/go-gui/gui/backend/internal/x11key"

// wideKeysymToRune is the character a keysym types. libxkbcommon is
// reached only on amd64 and arm64; elsewhere x11key's table (Latin-1,
// keypad, Euro and Unicode keysyms) is all there is.
func wideKeysymToRune(sym uint32) rune {
	return x11key.KeysymToRune(sym)
}

// wideKeysymToUpper is the capital of a letter keysym, by shiftedKeysym's
// Latin-1 and Unicode conversion.
func wideKeysymToUpper(sym uint32) uint32 {
	return shiftedKeysym(sym)
}
