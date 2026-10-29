//go:build linux && !js && !android

package gl

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// The server's XKB key symbols (#1188, docs/specs/keyboard-layouts.md § 6).
//
// The core keymap lists a key's third and fourth group after the first two
// groups' further levels, whose count it does not say, so no reading of it
// finds group 3. XkbGetMap with the KeySyms part sends each key's symbols as
// a groups × width array instead. Only that part is read: no key types, so
// a NoSymbol level is either one the key's type lacks or one left empty on
// purpose, and readPress treats both alike.

// XKB protocol values (XKBproto.h) for GetMap.
const (
	xkbGetMapMinor       = 8
	xkbKeySymsMask       = 1 << 1 // XkbKeySymsMask, a map part
	xkbUseCoreKbd        = 0x100
	xkbMaxGroups         = 4
	xkbMaxLevels         = 63 // XkbMaxShiftLevel
	xkbGetMapReplyHd     = 40 // fixed part of the reply, header included
	xkbKeySymMapHd       = 8  // ktIndex[4], groupInfo, width, nSyms
	xkbGroupsMask        = 0x0f
	xkbRangeMask         = 0xc0
	xkbClampIntoRange    = 0x40
	xkbRedirectIntoRange = 0x80
)

// xkbKey is one key of an xkbKeySyms: where its symbols start, and its
// groupInfo (group count, out-of-range rule) and width (levels per group).
type xkbKey struct {
	off              uint32
	groupInfo, width uint8
}

// xkbKeySyms is the symbol map of every key, from one GetMap reply.
type xkbKeySyms struct {
	keys      [256]xkbKey
	syms      []uint32
	maxGroups int // the most groups any key has
}

// groups is how many groups code has.
func (m *xkbKeySyms) groups(code xproto.Keycode) int {
	return int(m.keys[code].groupInfo & xkbGroupsMask)
}

// keyGroup is the group of code a press in the keyboard's group reads:
// group itself when the key has it, else the key's out-of-range rule
// (XKB protocol, "Key Symbol Map"): wrap, clamp, or redirect to a fixed
// group. -1 for a key with no groups.
func (m *xkbKeySyms) keyGroup(code xproto.Keycode, group int) int {
	k := m.keys[code]
	n := int(k.groupInfo & xkbGroupsMask)
	switch {
	case n == 0:
		return -1
	case group < n:
		return group
	}
	switch k.groupInfo & xkbRangeMask {
	case xkbClampIntoRange:
		return n - 1
	case xkbRedirectIntoRange:
		if r := int(k.groupInfo>>4) & 3; r < n {
			return r
		}
		return 0
	}
	return group % n
}

// sym is the keysym on level of code's group g, g already in the key's
// range; 0 (NoSymbol) past its width.
func (m *xkbKeySyms) sym(code xproto.Keycode, g, level int) uint32 {
	k := m.keys[code]
	if g < 0 || g >= int(k.groupInfo&xkbGroupsMask) || level >= int(k.width) {
		return 0
	}
	return m.syms[int(k.off)+g*int(k.width)+level]
}

// loadXKBKeySyms asks the server for the core keyboard's key symbols
// through the XKB extension at major opcode op.
func loadXKBKeySyms(conn *xgb.Conn, op byte) (*xkbKeySyms, error) {
	req := make([]byte, 28) // length 7 words; every other field 0
	req[0], req[1], req[2] = op, xkbGetMapMinor, 7
	binary.LittleEndian.PutUint16(req[4:], xkbUseCoreKbd)
	binary.LittleEndian.PutUint16(req[6:], xkbKeySymsMask) // full
	cookie := conn.NewCookie(true, true)
	conn.NewRequest(req, cookie)
	reply, err := cookie.Reply()
	if err != nil {
		return nil, err
	}
	return parseXKBKeySyms(reply)
}

// errXKBMap is every way a GetMap reply can fail validation.
var errXKBMap = errors.New("xkb: bad GetMap reply")

// parseXKBKeySyms reads the KeySyms part of a GetMap reply, checking every
// count against the reply's length and the protocol's limits. A reply that
// fails any check yields an error and no map, never a partial one.
func parseXKBKeySyms(r []byte) (*xkbKeySyms, error) {
	le := binary.LittleEndian
	if len(r) < xkbGetMapReplyHd || r[0] != 1 {
		return nil, fmt.Errorf("%w: %d bytes", errXKBMap, len(r))
	}
	// The word count is checked as sent, before it becomes an int: on 32-bit
	// Linux 4×count can overflow one.
	words := le.Uint32(r[4:])
	if words > uint32(len(r)-32)/4 || 32+4*int(words) < xkbGetMapReplyHd {
		return nil, fmt.Errorf("%w: length %d words, have %d bytes", errXKBMap, words, len(r))
	}
	r = r[:32+4*int(words)]
	minKC, maxKC := int(r[10]), int(r[11])
	present := le.Uint16(r[12:])
	first, nKeys, total := int(r[17]), int(r[20]), int(le.Uint16(r[18:]))
	// Only the KeySyms part was asked for; another one would come first.
	if present != xkbKeySymsMask {
		return nil, fmt.Errorf("%w: parts %#x", errXKBMap, present)
	}
	// A full KeySyms reply covers every key; none would replace the core
	// keymap with an empty map.
	if nKeys == 0 || minKC < 8 || minKC > maxKC || first < minKC || first+nKeys-1 > maxKC {
		return nil, fmt.Errorf("%w: keys %d+%d outside %d-%d", errXKBMap, first, nKeys, minKC, maxKC)
	}
	m := &xkbKeySyms{syms: make([]uint32, 0, total)}
	at := xkbGetMapReplyHd
	for code := first; code < first+nKeys; code++ {
		if at+xkbKeySymMapHd > len(r) {
			return nil, fmt.Errorf("%w: key %d past the end", errXKBMap, code)
		}
		info, width, nSyms := r[at+4], int(r[at+5]), int(le.Uint16(r[at+6:]))
		groups := int(info & xkbGroupsMask)
		if groups > xkbMaxGroups || width > xkbMaxLevels || groups > 0 && width == 0 ||
			nSyms != groups*width {
			return nil, fmt.Errorf("%w: key %d: %d groups × %d levels, %d symbols",
				errXKBMap, code, groups, width, nSyms)
		}
		at += xkbKeySymMapHd
		if at+4*nSyms > len(r) || len(m.syms)+nSyms > total {
			return nil, fmt.Errorf("%w: key %d: symbols past the end", errXKBMap, code)
		}
		m.keys[code] = xkbKey{off: uint32(len(m.syms)), groupInfo: info, width: uint8(width)}
		for i := range nSyms {
			m.syms = append(m.syms, le.Uint32(r[at+4*i:]))
		}
		at += 4 * nSyms
		m.maxGroups = max(m.maxGroups, groups)
	}
	if len(m.syms) != total {
		return nil, fmt.Errorf("%w: %d symbols, reply says %d", errXKBMap, len(m.syms), total)
	}
	return m, nil
}
