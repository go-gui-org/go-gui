//go:build linux && !js && !android

package gl

import (
	"testing"

	"github.com/jezek/xgb/xproto"
)

func TestFocusRealChange(t *testing.T) {
	cases := []struct {
		name         string
		mode, detail byte
		want         bool
	}{
		{"normal ancestor", xproto.NotifyModeNormal, xproto.NotifyDetailAncestor, true},
		{"normal nonlinear", xproto.NotifyModeNormal, xproto.NotifyDetailNonlinear, true},
		{"while-grabbed real", xproto.NotifyModeWhileGrabbed, xproto.NotifyDetailNonlinear, true},
		// Grab/ungrab churn from an interactive WM move/resize: must be ignored
		// so EventResized keeps flowing during a resize drag.
		{"grab", xproto.NotifyModeGrab, xproto.NotifyDetailNonlinear, false},
		{"ungrab", xproto.NotifyModeUngrab, xproto.NotifyDetailNonlinear, false},
		// Pointer-driven pseudo-focus: not a real keyboard-focus change.
		{"pointer detail", xproto.NotifyModeNormal, xproto.NotifyDetailPointer, false},
		{"pointer-root detail", xproto.NotifyModeNormal, xproto.NotifyDetailPointerRoot, false},
		{"none detail", xproto.NotifyModeNormal, xproto.NotifyDetailNone, false},
	}
	for _, c := range cases {
		if got := focusRealChange(c.mode, c.detail); got != c.want {
			t.Errorf("%s: focusRealChange(%d,%d)=%v, want %v",
				c.name, c.mode, c.detail, got, c.want)
		}
	}
}

// Issue #948: X servers list most non-printing keys as "Left NoSymbol
// Left", leaving the shifted column empty. The core protocol says an
// empty second column means "same as the first", so Shift+Left must
// still look up as Left. Returning NoSymbol (0) handed IBus a keyval of
// 0, which it swallowed — and with it the Shift+Arrow selection.
func TestKeysymShiftColumnFallsBack(t *testing.T) {
	const (
		minKC   = 8
		per     = 3
		kcA     = 38
		kcLeft  = 113
		symA    = 'a'
		symCapA = 'A'
		symLeft = 0xff51
	)
	syms := make([]xproto.Keysym, (kcLeft-minKC+1)*per)
	// "a A" — a real shifted symbol is kept.
	syms[(kcA-minKC)*per+0] = symA
	syms[(kcA-minKC)*per+1] = symCapA
	// "Left NoSymbol Left" — the shifted column is empty.
	syms[(kcLeft-minKC)*per+0] = symLeft
	syms[(kcLeft-minKC)*per+2] = symLeft
	p := &platformState{
		keymap: &xproto.GetKeyboardMappingReply{
			KeysymsPerKeycode: per,
			Keysyms:           syms,
		},
		minKeycode: minKC,
	}
	cases := []struct {
		name string
		code xproto.Keycode
		col  int
		want uint32
	}{
		{"a unshifted", kcA, 0, symA},
		{"a shifted", kcA, 1, symCapA},
		{"left unshifted", kcLeft, 0, symLeft},
		{"left shifted falls back", kcLeft, 1, symLeft},
	}
	for _, c := range cases {
		if got := p.keysym(c.code, c.col); got != c.want {
			t.Errorf("%s: keysym(%d,%d)=%#x, want %#x",
				c.name, c.code, c.col, got, c.want)
		}
	}
}

// The rest of the core-protocol rule behind #948: a lone letter ("a
// NoSymbol") shifts to its uppercase form rather than repeating the
// lowercase one, and a one-column keymap ("K1" alone) reads as "K1
// NoSymbol" instead of indexing into the next keycode's entry.
func TestKeysymShiftColumnCaseAndWidth(t *testing.T) {
	const minKC = 8
	cases := []struct {
		name string
		per  int
		syms []xproto.Keysym // keycodes minKC, minKC+1, … in order
		code xproto.Keycode
		col  int
		want uint32
	}{
		{"lone ascii letter uppercases", 2, []xproto.Keysym{'a', 0}, minKC, 1, 'A'},
		// 0xe9 é → 0xc9 É: both Latin-1, so the legacy keysym is kept.
		{"lone latin-1 letter uppercases", 2, []xproto.Keysym{0xe9, 0}, minKC, 1, 0xc9},
		// ÿ uppercases to U+0178, outside Latin-1: a Unicode keysym.
		{"lone y-diaeresis to unicode keysym", 2, []xproto.Keysym{0xff, 0}, minKC, 1, 0x01000178},
		// Unicode keysym for ж (U+0436) → Ж (U+0416).
		{"lone unicode letter uppercases", 2, []xproto.Keysym{0x01000436, 0}, minKC, 1, 0x01000416},
		{"lone digit unchanged", 2, []xproto.Keysym{'1', 0}, minKC, 1, '1'},
		// Latin-1 symbols with no Latin-1 capital stay put: µ must not
		// become Greek Μ (a deliberate choice), ß has no simple
		// uppercase, ÷ is not a letter.
		{"lone mu unchanged", 2, []xproto.Keysym{0xb5, 0}, minKC, 1, 0xb5},
		{"lone sharp s unchanged", 2, []xproto.Keysym{0xdf, 0}, minKC, 1, 0xdf},
		{"lone division unchanged", 2, []xproto.Keysym{0xf7, 0}, minKC, 1, 0xf7},
		{"keycode below min", 2, []xproto.Keysym{'a', 0}, minKC - 1, 1, 0},
		{"keycode past keymap", 2, []xproto.Keysym{'a', 0}, minKC + 1, 1, 0},
		{"column past width", 2, []xproto.Keysym{'a', 'A'}, minKC, 2, 0},
		{"empty keymap", 2, nil, minKC, 1, 0},
		{"zero width", 0, []xproto.Keysym{'a'}, minKC, 1, 0},
		// Dotless ı (U+0131) uppercases to ASCII I: the legacy keysym.
		{"unicode to latin-1 capital", 2, []xproto.Keysym{0x01000131, 0}, minKC, 1, 'I'},
		// Past U+10FFFF is no code point; it passes through unchanged.
		{"invalid unicode keysym", 2, []xproto.Keysym{0x01ffffff, 0}, minKC, 1, 0x01ffffff},
		{"one column shifted", 1, []xproto.Keysym{0xff51, 'x'}, minKC, 1, 0xff51},
		{"one column letter shifted", 1, []xproto.Keysym{'b', 'x'}, minKC, 1, 'B'},
		{"one column unshifted", 1, []xproto.Keysym{0xff51, 'x'}, minKC, 0, 0xff51},
	}
	for _, c := range cases {
		p := &platformState{
			keymap: &xproto.GetKeyboardMappingReply{
				KeysymsPerKeycode: byte(c.per),
				Keysyms:           c.syms,
			},
			minKeycode: minKC,
		}
		if got := p.keysym(c.code, c.col); got != c.want {
			t.Errorf("%s: keysym(%d,%d)=%#x, want %#x",
				c.name, c.code, c.col, got, c.want)
		}
	}
}

// Issue #587: only a LeaveNotify that takes the pointer out of the window
// clears hover.
func TestPointerRealExit(t *testing.T) {
	cases := []struct {
		name         string
		mode, detail byte
		want         bool
	}{
		{"normal ancestor", xproto.NotifyModeNormal, xproto.NotifyDetailAncestor, true},
		{"normal nonlinear", xproto.NotifyModeNormal, xproto.NotifyDetailNonlinear, true},
		// Into a child window of ours: the pointer is still inside.
		{"normal inferior", xproto.NotifyModeNormal, xproto.NotifyDetailInferior, false},
		// A grab changing hands: the pointer did not move.
		{"grab", xproto.NotifyModeGrab, xproto.NotifyDetailAncestor, false},
		{"ungrab", xproto.NotifyModeUngrab, xproto.NotifyDetailAncestor, false},
	}
	for _, c := range cases {
		if got := pointerRealExit(c.mode, c.detail); got != c.want {
			t.Errorf("%s: pointerRealExit(%d,%d)=%v, want %v",
				c.name, c.mode, c.detail, got, c.want)
		}
	}
}
