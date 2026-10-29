//go:build linux && !js && !android

package gl

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/jezek/xgb/xproto"
)

// testXKBKey is one key of a hand-built GetMap reply.
type testXKBKey struct {
	groupInfo, width byte
	syms             []uint32
}

// testXKBReply encodes a GetMap reply holding the KeySyms part for keys
// first, first+1, ... on a keyboard of keycodes 8-255.
func testXKBReply(first byte, keys []testXKBKey) []byte {
	le := binary.LittleEndian
	r := make([]byte, xkbGetMapReplyHd)
	r[0], r[10], r[11] = 1, 8, 255
	le.PutUint16(r[12:], xkbKeySymsMask)
	r[17], r[20] = first, byte(len(keys))
	total := 0
	for _, k := range keys {
		r = append(r, 0, 0, 0, 0, k.groupInfo, k.width, 0, 0)
		le.PutUint16(r[len(r)-2:], uint16(len(k.syms)))
		for _, s := range k.syms {
			r = le.AppendUint32(r, s)
		}
		total += len(k.syms)
	}
	le.PutUint16(r[18:], uint16(total))
	le.PutUint32(r[4:], uint32((len(r)-32)/4))
	return r
}

// TestXKBKeySymsGroups: a key's symbols are a groups × width array, and a
// group the key lacks follows its out-of-range rule.
func TestXKBKeySymsGroups(t *testing.T) {
	m, err := parseXKBKeySyms(testXKBReply(9, []testXKBKey{
		{0, 0, nil}, // key 9: no symbols
		{3, 2, []uint32{'c', 'C', 0x6d3, 0x6f3, 'x', 'X'}}, // key 10: three groups
		{1, 1, []uint32{' '}},                              // key 11: one group, wrap
		{2 | xkbClampIntoRange, 1, []uint32{'1', '2'}},     // key 12
		{2 | xkbRedirectIntoRange | 1<<4, 1, []uint32{'1', '2'}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m.maxGroups != 3 {
		t.Errorf("maxGroups %d, want 3", m.maxGroups)
	}
	for _, c := range []struct {
		code         xproto.Keycode
		group, level int
		want         uint32
	}{
		{9, 0, 0, 0},
		{10, 0, 1, 'C'},
		{10, 1, 0, 0x6d3},
		{10, 2, 0, 'x'}, // the third group, where the core keymap cannot say
		{10, 2, 1, 'X'},
		{10, 3, 0, 'c'}, // wrapped: 3 % 3
		{10, 0, 2, 0},   // past the width
		{11, 2, 0, ' '},
		{12, 3, 0, '2'}, // clamped to the last group
		{13, 3, 0, '2'}, // redirected to group 1 (from 0)
	} {
		if got := m.sym(c.code, m.keyGroup(c.code, c.group), c.level); got != c.want {
			t.Errorf("key %d group %d level %d: %#x, want %#x", c.code, c.group, c.level, got, c.want)
		}
	}
}

// TestXKBKeySymsRejects: a reply that breaks any count yields no map.
func TestXKBKeySymsRejects(t *testing.T) {
	good := func() []byte { return testXKBReply(8, []testXKBKey{{1, 2, []uint32{'a', 'A'}}}) }
	le := binary.LittleEndian
	for name, mutate := range map[string]func([]byte) []byte{
		"short":            func(r []byte) []byte { return r[:20] },
		"length past end":  func(r []byte) []byte { le.PutUint32(r[4:], 100); return r },
		"huge length":      func(r []byte) []byte { le.PutUint32(r[4:], 0xfffffff0); return r }, // wraps to n = -32 on 32-bit
		"below header":     func(r []byte) []byte { le.PutUint32(r[4:], 1); return r },
		"no keys":          func(r []byte) []byte { r[20] = 0; le.PutUint16(r[18:], 0); return r },
		"not a reply":      func(r []byte) []byte { r[0] = 0; return r },
		"key types too":    func(r []byte) []byte { le.PutUint16(r[12:], xkbKeySymsMask|1); return r },
		"first below min":  func(r []byte) []byte { r[17] = 7; return r },
		"keys past max":    func(r []byte) []byte { r[11] = 8; r[20] = 2; return r },
		"five groups":      func(r []byte) []byte { r[44] = 5; return r },
		"zero width":       func(r []byte) []byte { r[45] = 0; return r },
		"nSyms mismatch":   func(r []byte) []byte { le.PutUint16(r[46:], 1); return r },
		"total mismatch":   func(r []byte) []byte { le.PutUint16(r[18:], 3); return r },
		"symbols past end": func(r []byte) []byte { le.PutUint32(r[4:], 4); return r[:48] },
	} {
		if m, err := parseXKBKeySyms(mutate(good())); err == nil || m != nil || !errors.Is(err, errXKBMap) {
			t.Errorf("%s: map %v, err %v", name, m, err)
		}
	}
	if _, err := parseXKBKeySyms(good()); err != nil {
		t.Fatalf("unmutated reply: %v", err)
	}
}
