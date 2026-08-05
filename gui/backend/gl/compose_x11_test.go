//go:build linux && !js && !android

package gl

import (
	"slices"
	"testing"
)

// Keysyms used in the tests (keysymdef.h subset; the constants live in
// compose_x11.go but are repeated here so the tests read plainly).
const (
	xkDeadAcute  = 0xfe51
	xkDeadGrave  = 0xfe50
	xkDeadCirc   = 0xfe52
	xkDeadTilde  = 0xfe53
	xkDeadMacron = 0xfe54
	xkDeadDiaer  = 0xfe57
	xkDeadRing   = 0xfe58
	xkDeadDblAc  = 0xfe59
	xkDeadCaron  = 0xfe5a
	xkDeadCed    = 0xfe5b
	xkDeadOgonk  = 0xfe5c
	xkDeadStrok  = 0xfe63
	xkMultiKey   = 0xff20
	xkEscape     = 0xff1b
	xkShiftL     = 0xffe1
	xkHyperR     = 0xffee
	xkNumLock    = 0xff7f
	xkReturn     = 0xff0d
)

// typeSeq feeds keysyms through a fresh compose machine and returns the
// runes it produced.
func typeSeq(syms ...uint32) []rune {
	var c compose
	var out []rune
	for _, s := range syms {
		if r := c.feed(s); r != 0 {
			out = append(out, r)
		}
	}
	return out
}

func TestComposeDeadKeySequences(t *testing.T) {
	cases := []struct {
		name string
		keys []uint32
		want []rune
	}{
		// The dead key itself produces nothing; the second key resolves.
		{"dead acute e", []uint32{xkDeadAcute, 'e'}, []rune{'é'}},
		{"dead acute E", []uint32{xkDeadAcute, 'E'}, []rune{'É'}},
		{"dead grave e", []uint32{xkDeadGrave, 'e'}, []rune{'è'}},
		{"dead tilde n", []uint32{xkDeadTilde, 'n'}, []rune{'ñ'}},
		{"dead diaeresis u", []uint32{xkDeadDiaer, 'u'}, []rune{'ü'}},
		{"dead cedilla c", []uint32{xkDeadCed, 'c'}, []rune{'ç'}},
		{"dead ring a", []uint32{xkDeadRing, 'a'}, []rune{'å'}},
		{"dead caron c", []uint32{xkDeadCaron, 'c'}, []rune{'č'}},
		{"dead ogonek a", []uint32{xkDeadOgonk, 'a'}, []rune{'ą'}},
		{"dead stroke o", []uint32{xkDeadStrok, 'o'}, []rune{'ø'}},
		{"dead stroke l", []uint32{xkDeadStrok, 'l'}, []rune{'ł'}},
		{"dead macron a", []uint32{xkDeadMacron, 'a'}, []rune{'ā'}},
		{"dead doubleacute o", []uint32{xkDeadDblAc, 'o'}, []rune{'ő'}},
		// Spacing accents: dead key + space or the dead key repeated.
		{"dead key alone", []uint32{xkDeadAcute}, nil},
		{"dead twice", []uint32{xkDeadAcute, xkDeadAcute}, []rune{'´'}},
		{"dead space", []uint32{xkDeadAcute, ' '}, []rune{'\''}},
		{"diaeresis space", []uint32{xkDeadDiaer, ' '}, []rune{'"'}},
		// An undefined pair drops the accent and emits the key alone.
		{"undefined drops accent", []uint32{xkDeadAcute, 'q'}, []rune{'q'}},
		// A second dead key takes over the pending sequence.
		{"re-pend", []uint32{xkDeadAcute, xkDeadGrave, 'a'}, []rune{'à'}},
	}
	for _, c := range cases {
		if got := typeSeq(c.keys...); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestComposeMultiKeySequences(t *testing.T) {
	cases := []struct {
		name string
		keys []uint32
		want []rune
	}{
		// Multi_key + two keys resolves through the two-key table; an
		// undefined first pair drops Multi_key and re-pends the second.
		{"ae ligature", []uint32{xkMultiKey, 'a', 'e'}, []rune{'æ'}},
		{"AE ligature", []uint32{xkMultiKey, 'A', 'E'}, []rune{'Æ'}},
		{"oe ligature", []uint32{xkMultiKey, 'o', 'e'}, []rune{'œ'}},
		{"sharp s", []uint32{xkMultiKey, 's', 's'}, []rune{'ß'}},
		{"quote a", []uint32{xkMultiKey, '\'', 'a'}, []rune{'á'}},
		{"backtick e", []uint32{xkMultiKey, '`', 'e'}, []rune{'è'}},
		{"caret o", []uint32{xkMultiKey, '^', 'o'}, []rune{'ô'}},
		{"tilde n", []uint32{xkMultiKey, '~', 'n'}, []rune{'ñ'}},
		{"quotedbl u", []uint32{xkMultiKey, '"', 'u'}, []rune{'ü'}},
		{"comma c", []uint32{xkMultiKey, ',', 'c'}, []rune{'ç'}},
		{"slash o", []uint32{xkMultiKey, '/', 'o'}, []rune{'ø'}},
		{"minus a", []uint32{xkMultiKey, '-', 'a'}, []rune{'ā'}},
		{"degree", []uint32{xkMultiKey, 'o', 'o'}, []rune{'°'}},
		{"half", []uint32{xkMultiKey, '1', '2'}, []rune{'½'}},
		{"guillemets", []uint32{xkMultiKey, '<', '<'}, []rune{'«'}},
		{"nobreak space", []uint32{xkMultiKey, ' ', ' '}, []rune{'\u00a0'}},
		{"prefix to dead key", []uint32{xkMultiKey, xkDeadAcute, 'e'}, []rune{'é'}},
		// The first pair is undefined: Multi_key and the second key are
		// dropped, the third falls through on its own.
		{"undefined prefix", []uint32{xkMultiKey, 'z', 'q'}, []rune{'q'}},
		// After a resolution the machine is idle again.
		{"resolves then plain", []uint32{xkMultiKey, '\'', 'a', 'b'}, []rune{'á', 'b'}},
	}
	for _, c := range cases {
		if got := typeSeq(c.keys...); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Two dead keys buffer until a letter arrives, which may resolve as a
// three-key double-accent sequence or fall back to the last pair.
func TestComposeThreeKeySequences(t *testing.T) {
	cases := []struct {
		name string
		keys []uint32
		want []rune
	}{
		{"acute diaeresis u", []uint32{xkDeadAcute, xkDeadDiaer, 'u'}, []rune{'ǘ'}},
		{"grave circumflex a", []uint32{xkDeadGrave, xkDeadCirc, 'a'}, []rune{'ầ'}},
		{"acute diaeresis i", []uint32{xkDeadAcute, xkDeadDiaer, 'i'}, []rune{'ḯ'}},
		{"acute diaeresis e", []uint32{xkDeadAcute, xkDeadDiaer, 'e'}, []rune{'ë'}},
		{"undefined triple drops both", []uint32{xkDeadAcute, xkDeadGrave, 'q'}, []rune{'q'}},
		// The third key is another compose key: it re-pends.
		{"triple then dead then e", []uint32{xkDeadAcute, xkDeadGrave, xkDeadDiaer, 'e'}, []rune{'ë'}},
		// A modifier down between buffered keys must not clear them.
		{"triple across shift", []uint32{xkDeadAcute, xkDeadDiaer, xkShiftL, 'u'}, []rune{'ǘ'}},
		{"multi across shift", []uint32{xkMultiKey, 'a', xkShiftL, 'e'}, []rune{'æ'}},
	}
	for _, c := range cases {
		if got := typeSeq(c.keys...); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// A modifier key down between a dead key and its target letter is
// ordinary typing and must not cancel the pending accent. Lock keys
// and both AltGr levels are modifiers in X11's eyes too.
func TestComposeModifierKeepsSequence(t *testing.T) {
	for _, mod := range []uint32{xkShiftL, xkHyperR, xkNumLock, 0xfe03, 0xfe11} {
		var c compose
		for _, sym := range []uint32{xkDeadAcute, mod} {
			if r := c.feed(sym); r != 0 {
				t.Fatalf("feed(%#x) produced %q, want none", sym, r)
			}
		}
		if r := c.feed('e'); r != 'é' {
			t.Fatalf("dead acute, %#x, e produced %q, want é", mod, r)
		}
	}
	// Two modifier presses in a row must not clear the buffer either.
	var c compose
	c.feed(xkDeadAcute)
	c.feed(xkShiftL)
	c.feed(xkHyperR)
	if r := c.feed('e'); r != 'é' {
		t.Fatalf("dead acute, Shift, Hyper, e produced %q, want é", r)
	}
}

// The dead-key range check: 0xfe50-0xfe8b is dead, everything around
// it (including dead_greek at 0xfe8c) is not.
func TestIsDeadKeyBounds(t *testing.T) {
	cases := []struct {
		sym  uint32
		want bool
	}{
		{0xfe4f, false},
		{0xfe50, true},
		{0xfe51, true},
		{0xfe75, true},
		{0xfe7f, true}, // gap between the accent and letter ranges
		{0xfe80, true},
		{0xfe8b, true},
		{0xfe8c, false}, // dead_greek
		{0xff20, false}, // Multi_key
	}
	for _, c := range cases {
		if got := isDeadKey(c.sym); got != c.want {
			t.Errorf("isDeadKey(%#x) = %v, want %v", c.sym, got, c.want)
		}
	}
}

// The modifier range check must be exact at both edges.
func TestIsModifierKeyBounds(t *testing.T) {
	cases := []struct {
		sym  uint32
		want bool
	}{
		{0xfe02, false},
		{0xfe03, true}, // ISO_Level3_Shift
		{0xfe10, false},
		{0xfe11, true},  // ISO_Level5_Shift
		{0xff13, false}, // Pause
		{0xff14, true},  // Scroll_Lock
		{0xff7e, false},
		{0xff7f, true}, // Num_Lock
		{0xffe0, false},
		{0xffe1, true}, // Shift_L
		{0xffee, true}, // Hyper_R
		{0xffef, false},
	}
	for _, c := range cases {
		if got := isModifierKey(c.sym); got != c.want {
			t.Errorf("isModifierKey(%#x) = %v, want %v", c.sym, got, c.want)
		}
	}
}

// A non-printable, non-modifier key (arrows, Escape, function keys)
// cancels the pending sequence, which then never applies to later keys.
func TestComposeNonPrintableCancels(t *testing.T) {
	var c compose
	c.feed(xkDeadAcute)
	if r := c.feed(xkEscape); r != 0 {
		t.Fatalf("Escape produced %q, want none", r)
	}
	if r := c.feed('e'); r != 'e' {
		t.Fatalf("accent survived Escape: got %q, want plain e", r)
	}
}

func TestComposeReset(t *testing.T) {
	var c compose
	c.feed(xkDeadAcute)
	c.reset()
	if r := c.feed('e'); r != 'e' {
		t.Fatalf("after reset, e produced %q, want plain e", r)
	}
}

// Idle keys pass through unchanged; non-printable keysyms produce
// nothing, exactly like the filter KeysymToRune used to apply.
func TestComposePassthrough(t *testing.T) {
	var c compose
	if r := c.feed('a'); r != 'a' {
		t.Fatalf("idle 'a' produced %q", r)
	}
	if r := c.feed(xkReturn); r != 0 {
		t.Fatalf("idle Return produced %q", r)
	}
	if r := c.feed(xkShiftL); r != 0 {
		t.Fatalf("idle Shift produced %q", r)
	}
}

// The tables carry the well-known Latin-1 and ligature sequences.
func TestComposeTableSanity(t *testing.T) {
	dead := map[[2]uint32]rune{
		{xkDeadAcute, 'e'}:         'é',
		{xkDeadAcute, 'E'}:         'É',
		{xkDeadTilde, 'n'}:         'ñ',
		{xkDeadCed, 'c'}:           'ç',
		{xkDeadRing, 'a'}:          'å',
		{xkDeadStrok, 'o'}:         'ø',
		{xkDeadCaron, 'c'}:         'č',
		{xkDeadOgonk, 'a'}:         'ą',
		{xkDeadDblAc, 'o'}:         'ő',
		{xkDeadDiaer, ' '}:         '"',
		{xkDeadAcute, ' '}:         '\'',
		{xkDeadAcute, xkDeadAcute}: '´',
	}
	for k, want := range dead {
		got, ok := deadKeyTable[k]
		if !ok {
			t.Errorf("deadKeyTable[{%#x, %#x}] missing", k[0], k[1])
			continue
		}
		if got != want {
			t.Errorf("deadKeyTable[{%#x, %#x}] = %q, want %q", k[0], k[1], got, want)
		}
	}
	multi := map[[2]uint32]rune{
		{'a', 'e'}:  'æ',
		{'s', 's'}:  'ß',
		{'\'', 'a'}: 'á',
		{'`', 'e'}:  'è',
		{'^', 'o'}:  'ô',
		{'"', 'u'}:  'ü',
		{',', 'c'}:  'ç',
		{'/', 'o'}:  'ø',
		{'o', 'e'}:  'œ',
		{'o', 'o'}:  '°',
		{'1', '2'}:  '½',
		{'<', '<'}:  '«',
		{' ', ' '}:  '\u00a0',
	}
	for k, want := range multi {
		got, ok := multiKeyTable[k]
		if !ok {
			t.Errorf("multiKeyTable[{%#x, %#x}] missing", k[0], k[1])
			continue
		}
		if got != want {
			t.Errorf("multiKeyTable[{%#x, %#x}] = %q, want %q", k[0], k[1], got, want)
		}
	}
}

// Spot-check the three-key table the same way.
func TestComposeTable3Sanity(t *testing.T) {
	cases := map[[3]uint32]rune{
		{xkDeadAcute, xkDeadDiaer, 'u'}: 'ǘ',
		{xkDeadAcute, xkDeadDiaer, 'U'}: 'Ǘ',
		{xkDeadGrave, xkDeadCirc, 'a'}:  'ầ',
		{xkDeadStrok, xkDeadAcute, 'o'}: 'ǿ',
	}
	for k, want := range cases {
		got, ok := composeTable3[k]
		if !ok {
			t.Errorf("composeTable3[{%#x, %#x, %#x}] missing", k[0], k[1], k[2])
			continue
		}
		if got != want {
			t.Errorf("composeTable3[{%#x, %#x, %#x}] = %q, want %q", k[0], k[1], k[2], got, want)
		}
	}
	// A pair that has no three-key sequence must not be in the table.
	if _, ok := composeTable3[[3]uint32{xkDeadAcute, xkDeadDiaer, 'q'}]; ok {
		t.Error("composeTable3 has a dead_acute dead_diaeresis q entry")
	}
}
