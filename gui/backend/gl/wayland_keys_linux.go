//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/x11key"
)

// Key press records for the Wayland seat (#1188): what a press reports,
// kept so its repeats and release report the same keys.

// readPress is what a press of code reports in the current state. The
// typed key is the KeyCode of the keysym the press produces; when that is
// the same key or none, the typed key is the KeyCode and nothing is used up,
// so Ctrl+Shift+A stays Ctrl+Shift+A.
func (s *wlSeat) readPress(code uint32) pressedKey {
	plain, shifted, latin, nonLatin := s.keymap.KeySyms(code)
	kc := x11key.KeyCode(x11key.KeyInput{
		Plain: plain, Shifted: shifted, Latin: latin, NonLatin: nonLatin, Evdev: code - 8,
	})
	pk := pressedKey{code: kc, typed: kc, held: true}
	typed := x11key.KeyCode(x11key.KeyInput{Plain: s.keymap.Sym(code), Evdev: code - 8})
	if typed != gui.KeyInvalid && typed != kc {
		pk.typed = typed
		pk.consumed = x11key.MapModifiers(s.keymap.Consumed(code))
	}
	return pk
}

// forgetPressed drops the press records. A key held across a focus change
// or a new keymap may be released without a press we saw (wl_keyboard.enter
// lists held keys but sends no press), and must not report an old press.
func (s *wlSeat) forgetPressed() {
	s.pressed = [len(s.pressed)]pressedKey{}
}
