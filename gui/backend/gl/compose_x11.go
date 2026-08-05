//go:build linux && !js && !android

package gl

import "github.com/go-gui-org/go-gui/gui/backend/internal/x11key"

// Dead-key and Multi_key (XCompose) handling for the raw keysym path.
//
// A daemon input method composes for us, but when none is reachable the
// backend decodes keysyms directly (events_x11.go) and dead keys
// (0xfe50 range) and Multi_key sequences (0xff20) used to fall through
// the printable-rune filter and produce nothing. This state machine
// fills that gap with a built-in table of the common Latin sequences,
// mirrored from the X11 en_US.UTF-8 Compose file (the compose.lst Latin
// subset). User ~/.XCompose files and non-Latin sequences are not
// parsed; a healthy IME daemon still takes precedence over this path.
//
// Two-key sequences resolve through deadKeyTable and multiKeyTable
// (compose_table_x11.go, compose_table_multi_x11.go). Longer sequences
// are handled by buffering: after Multi_key a printable key is held
// back as the middle of a three-key sequence ("Multi_key ' e" resolves
// as the pair (' , e)), and two dead keys buffer until a letter
// arrives, which resolves through composeTable3 ("dead_acute
// dead_diaeresis u" -> ǘ). An undefined sequence drops its buffered
// keys and lets the key that broke it fall through on its own, matching
// X11's observable behaviour: the dropped keys produce nothing.

// X11 keysyms used by the compose machine (keysymdef.h).
const (
	multiKey    = 0xff20 // Multi_key, the XCompose prefix key
	deadKeyBase = 0xfe50
	deadKeyEnd  = 0xfe8b // last dead keysym in the range; dead_greek (0xfe8c) sits past it
)

// isDeadKey reports whether sym is a dead-key keysym. The XK_dead_*
// range in keysymdef.h spans 0xfe50-0xfe75 (accents) and 0xfe80-0xfe8b
// (dead letters); dead_greek (0xfe8c) is not one of them.
func isDeadKey(sym uint32) bool {
	return sym >= deadKeyBase && sym <= deadKeyEnd
}

// isComposeKey reports whether sym starts or continues a sequence.
func isComposeKey(sym uint32) bool { return isDeadKey(sym) || sym == multiKey }

// isModifierKey reports whether sym is a modifier-only keysym whose
// press must not disturb a pending sequence. Shift/Alt/Ctrl down
// between a dead key and its target letter is normal typing; only
// printable and compose keys may resolve or cancel the sequence.
func isModifierKey(sym uint32) bool {
	// Lock keys (Num/Scroll) and the ISO level shifts toggle modifier
	// bits too, so a press must be just as transparent.
	switch sym {
	case 0xfe03, // ISO_Level3_Shift (AltGr)
		0xfe11, // ISO_Level5_Shift
		0xff14, // Scroll_Lock
		0xff7f: // Num_Lock
		return true
	}
	// Shift/Ctrl/Caps/Alt/Meta/Super/Hyper, left and right variants
	// (0xffe1-0xffee).
	return sym >= 0xffe1 && sym <= 0xffee
}

// compose is the dead-key / Multi_key state machine for one window.
// The zero value is idle; no allocation happens on the key path.
type compose struct {
	pending [2]uint32 // buffered keysyms of the open sequence
	n       int       // number of buffered keys (0..2)
}

// feed advances the machine with one keysym and returns the rune to
// emit, or 0 when the key must produce no character. The rules mirror
// X11's compose engine:
//
//   - idle + dead key / Multi_key: starts a sequence, nothing emitted
//   - idle + anything else: passthrough of the printable keysym
//   - one buffered key + key: a two-key table hit resolves; otherwise
//     a second compose key or a Multi_key continuation buffers, and any
//     other key falls through with the accent dropped
//   - two buffered keys + key: resolves through the three-key table or
//     the two-key table on the last pair; a miss drops the buffer and
//     lets the key fall through on its own
func (c *compose) feed(sym uint32) rune {
	switch c.n {
	case 0:
		if isComposeKey(sym) {
			c.pending[0], c.n = sym, 1
			return 0
		}
		return x11key.KeysymToRune(sym)

	case 1:
		if isModifierKey(sym) {
			return 0
		}
		if r, ok := lookupPair(c.pending[0], sym); ok {
			c.n = 0
			return r
		}
		switch {
		case isComposeKey(sym) || c.pending[0] == multiKey:
			// A second compose key, or a Multi_key continuation (the
			// middle of a three-key sequence): buffer until the third.
			c.pending[1], c.n = sym, 2
			return 0
		default:
			// Undefined dead-key pair: the accent is dropped and the
			// key falls through on its own.
			c.n = 0
			return x11key.KeysymToRune(sym)
		}

	default: // two buffered keys, resolve on the third
		if isModifierKey(sym) {
			return 0
		}
		r3, ok3 := composeTable3[[3]uint32{c.pending[0], c.pending[1], sym}]
		r2, ok2 := lookupPair(c.pending[1], sym)
		c.n = 0
		switch {
		case ok3:
			return r3
		case ok2:
			return r2
		case isComposeKey(sym):
			c.pending[0], c.n = sym, 1
			return 0
		default:
			return x11key.KeysymToRune(sym)
		}
	}
}

// reset drops any open sequence, e.g. when the window loses focus.
func (c *compose) reset() { c.n = 0 }

// lookupPair resolves a two-key sequence. The two tables are disjoint:
// deadKeyTable only starts with dead keysyms, multiKeyTable only with
// the printable keysyms of classic Multi_key sequences.
func lookupPair(a, b uint32) (rune, bool) {
	if r, ok := deadKeyTable[[2]uint32{a, b}]; ok {
		return r, true
	}
	r, ok := multiKeyTable[[2]uint32{a, b}]
	return r, ok
}
