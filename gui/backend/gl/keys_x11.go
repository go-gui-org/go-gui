//go:build linux && !js && !android

package gl

import (
	"strconv"
	"unicode"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/x11key"
)

// Key presses on X11 (#1188, docs/specs/keyboard-layouts.md).
//
// go-gui reads key symbols, not XKB's key types or state machine, so it
// applies the level rules itself: the group, NumLock on keypad keysyms,
// Shift, level 3, Caps Lock. The symbols come from the server's XKB map
// (xkbmap_x11.go), every group exact, when the server has the extension;
// else from the core keymap. Xorg publishes its columns as the first
// group's two levels, the second group's two levels, then the first group's
// further levels (3, 4, ...) and after them the second's; how many the
// first has it does not say, so the core keymap gives two groups and level
// 3 for the first only. ISO level 3 (AltGr) selects levels 3 and 4. Which
// modifier bits NumLock and level 3 sit on comes from the modifier mapping,
// not a fixed Mod2 and Mod5.

// pressedKey is what a key press reports: its KeyCode, the key it typed
// and the modifiers that used up (docs/specs/keyboard-layouts.md § 1).
// held tells a recorded press from no record, as a press may report
// KeyInvalid (ä, a dead acute).
type pressedKey struct {
	code, typed gui.KeyCode
	consumed    gui.Modifier
	held        bool
}

// Keysyms the core rules name (keysymdef.h, X11 protocol § 5).
const (
	symNumLock          = 0xff7f
	symISOLevel3Shift   = 0xfe03
	symKeypadFirst      = 0xff80
	symKeypadLast       = 0xffbd
	symVendorKeypadMin  = 0x11000000
	symVendorKeypadMax  = 0x1100ffff
	coreColumnsPerGroup = 2 // unshifted, shifted
	coreLevel3Column    = 4 // the first group's level 3, then its level 4
)

// latinProbeCodes are the X11 keycodes (evdev + 8) of the keys US labels A,
// B, C and D. A group that types an ASCII letter on one of them is Latin.
var latinProbeCodes = [...]xproto.Keycode{38, 56, 54, 40}

// useXKBExtension tells the server this client knows XKB (XkbUseExtension
// 1.0), as Xlib does for every GTK and Qt application. To a client that
// does not, the server reports the active layout as a modifier: groups 2
// to 4 arrive as the bit the keymap's compat map names, Mod5 (AltGr's) on
// a stock keymap, so after a layout switch every key read as AltGr held
// (#1188). After it, key events carry the group in state bits 13-14, which
// readPress reads, and Mod5 only while level 3 is held. The server then
// sends an XKB-aware client no core MappingNotify, so it selects XKB's
// own keymap events instead (xkbNotifyEvent). It returns the extension's
// major opcode, or 0 without it; the core behavior then stays.
func useXKBExtension(conn *xgb.Conn) byte {
	q, err := xproto.QueryExtension(conn, uint16(len("XKEYBOARD")), "XKEYBOARD").Reply()
	if err != nil || q == nil || !q.Present {
		return 0
	}
	// xgb has no XKB package; requests are raw, little-endian as xgb
	// speaks. UseExtension (minor 0): length 2 words, wanted version 1.0.
	cookie := conn.NewCookie(true, true)
	conn.NewRequest([]byte{q.MajorOpcode, 0, 2, 0, 1, 0, 0, 0}, cookie)
	if reply, rerr := cookie.Reply(); rerr != nil || len(reply) < 2 || reply[1] == 0 {
		return 0
	}
	// Every XKB event has the extension's first event code; byte 1 says
	// which. Registered as xgb's extension packages register theirs
	// (randr.Init does the same after connecting).
	xgb.NewEventFuncs[int(q.FirstEvent)] = func(buf []byte) xgb.Event {
		return xkbNotifyEvent{kind: buf[1]}
	}
	// SelectEvents (minor 1) on the core keyboard (0x100): NewKeyboard and
	// Map notify, all of each, every map part, as libX11's XkbSelectEvents
	// sends it. Length 4 words.
	const events = 1<<xkbNewKeyboardNotify | 1<<xkbMapNotify
	conn.NewRequest([]byte{
		q.MajorOpcode, 1, 4, 0,
		0x00, 0x01, // deviceSpec: the core keyboard
		events, 0, // affectWhich
		0, 0, // clear
		events, 0, // selectAll
		0xff, 0, // affectMap: every map component
		0xff, 0, // map
	}, conn.NewCookie(false, false))
	return q.MajorOpcode
}

// XKB event kinds (XKBproto.h) the backend selects.
const (
	xkbNewKeyboardNotify = 0
	xkbMapNotify         = 1
)

// xkbNotifyEvent is an XKB event; only its kind is read.
type xkbNotifyEvent struct{ kind byte }

func (e xkbNotifyEvent) Bytes() []byte  { return nil }
func (e xkbNotifyEvent) String() string { return "XkbNotify{" + strconv.Itoa(int(e.kind)) + "}" }

// loadKeyboardInfo reads the XKB key symbols, which modifier bits hold
// NumLock and ISO level 3, and which groups type Latin letters. It runs
// after the core keymap is read, and drops the press records, which may
// name keys of the old map. A GetMap reply that fails validation leaves
// the core keymap in use, never a partial map.
func (p *platformState) loadKeyboardInfo(conn *xgb.Conn) {
	p.xkbMap = nil
	if p.xkbOpcode != 0 {
		if m, err := loadXKBKeySyms(conn, p.xkbOpcode); err == nil {
			p.xkbMap = m
		}
	}
	p.numLockMask, p.level3Mask = 0, 0
	if mm, err := xproto.GetModifierMapping(conn).Reply(); err == nil && mm != nil {
		per := int(mm.KeycodesPerModifier)
		for m := range 8 {
			for i := range per {
				if m*per+i >= len(mm.Keycodes) {
					break
				}
				switch p.keysym(mm.Keycodes[m*per+i], 0) {
				case symNumLock:
					p.numLockMask |= 1 << m
				case symISOLevel3Shift:
					p.level3Mask |= 1 << m
				}
			}
		}
	}
	groups := 2 // the core keymap's
	if p.xkbMap != nil {
		groups = p.xkbMap.maxGroups
	}
	for g := range p.latinGroups {
		p.latinGroups[g] = g < groups && p.groupIsLatin(g)
	}
	p.pressed = [len(p.pressed)]pressedKey{}
}

// reloadKeyboard reads the keymap and the keyboard information again,
// after a MappingNotify: setxkbmap or the desktop settings changed the
// layouts or the modifier mapping while the window was open.
func (p *platformState) reloadKeyboard(conn *xgb.Conn) {
	if km := loadKeymap(conn, xproto.Setup(conn)); km != nil {
		p.keymap = km
	}
	p.loadKeyboardInfo(conn)
	p.compose.reset() // a dead key of the old layout must not combine
}

// modifiers is x11key.MapModifiers with ModAltGr read from the bit ISO
// level 3 sits on in the modifier mapping, as readPress reads it:
// xkeyboard-config puts it on Mod5, a custom mapping may not.
func (p *platformState) modifiers(state uint16) gui.Modifier {
	m := x11key.MapModifiers(state &^ xproto.KeyButMaskMod5)
	if state&p.level3Mask != 0 {
		m |= gui.ModAltGr
	}
	return m
}

func (p *platformState) groupIsLatin(g int) bool {
	for _, code := range latinProbeCodes {
		if s := p.groupSym(code, g, 0); s >= 'a' && s <= 'z' || s >= 'A' && s <= 'Z' {
			return true
		}
	}
	return false
}

// group is the group of code a press reads, from the keyboard's group in
// bits 13-14 of the state. The XKB map applies the key's own rule for a
// group it lacks (wrap, clamp, redirect). The core keymap holds two
// groups: any but the first reads the second, if the key lists anything
// there (the protocol's rule: an empty second group is the first).
func (p *platformState) group(code xproto.Keycode, state uint16) int {
	group := int(state>>13) & 3
	if m := p.xkbMap; m != nil {
		return max(m.keyGroup(code, group), 0)
	}
	if group == 0 || p.keysym(code, 2) == 0 && p.keysym(code, 3) == 0 {
		return 0
	}
	return 1
}

// exactLevels is how many levels code has when the XKB map says so
// exactly: on a key with one group its width is that group's type's level
// count. 0 when unknown: groups of a key share the widest one's width,
// padded with NoSymbol, and the core keymap gives no count at all.
func (p *platformState) exactLevels(code xproto.Keycode) int {
	if m := p.xkbMap; m != nil && m.groups(code) == 1 {
		return int(m.keys[code].width)
	}
	return 0
}

// groupSym is the keysym on level (from 0) of code's group g (from 0),
// g wrapped into the key's groups by its own rule. The XKB map knows
// every group and level; the core keymap two groups, and levels 3 and 4
// of the first only.
func (p *platformState) groupSym(code xproto.Keycode, g, level int) uint32 {
	if m := p.xkbMap; m != nil {
		return m.sym(code, m.keyGroup(code, g), level)
	}
	switch {
	case g > 1 || level > 3 || g == 1 && level > 1:
		return 0
	case level < 2:
		return p.keysym(code, g*coreColumnsPerGroup+level)
	}
	return p.keysym(code, coreLevel3Column+level-2)
}

func isKeypadKeysym(sym uint32) bool {
	return sym >= symKeypadFirst && sym <= symKeypadLast ||
		sym >= symVendorKeypadMin && sym <= symVendorKeypadMax
}

// readPress is what a press of code with state reports (spec § 1 and 2),
// and the keysym it types. KeyCode reads the key with only NumLock
// applied; the typed keysym adds Shift, level 3 and Caps Lock.
//
// Where the core protocol and XKB differ, this follows XKB, which every
// Xlib and GTK application on the same desktop uses: on a keypad key only
// NumLock picks the keysym (the protocol would let Shift+keypad 7 type
// 7), and the used-up modifiers are counted as libxkbcommon's GTK mode
// counts them, as on Wayland.
func (p *platformState) readPress(code xproto.Keycode, state uint16) (pressedKey, uint32) {
	g := p.group(code, state)
	n := p.exactLevels(code)
	l1, l2 := p.groupSym(code, g, 0), p.groupSym(code, g, 1)
	if n == 1 || n == 0 && l2 == 0 && p.xkbMap != nil {
		l2 = l1 // a level the key's type lacks: Shift keeps level 1
	}
	in := x11key.KeyInput{Plain: l1, Shifted: l2, Evdev: uint32(code) - 8}
	keypad := isKeypadKeysym(l2)
	if keypad && p.numLockMask != 0 && state&p.numLockMask != 0 {
		in.Plain, in.Shifted = l2, l1
	}
	if !p.latinGroups[g] {
		// As on Wayland: the first group of the first Latin layout.
		in.NonLatin = true
		for lg, latin := range p.latinGroups {
			if latin {
				in.Latin = p.groupSym(code, lg, 0)
				break
			}
		}
	}
	kc := x11key.KeyCode(in)

	// Shift and level 3 count only on keys they can change: Shift not on a
	// keypad key (either of its first two levels a keypad keysym: XKB's
	// keypad types leave Shift out, as on the *_latin9 keypad operators
	// whose level 2 is ·), level 3 not on a key whose type has none. Where the
	// level count is known (exactLevels) that decides, and an empty level 3
	// or 4 types nothing; else a key that lists no level 3 or 4 has none,
	// and one that lists either types nothing on the other if it is empty.
	shift := state&xproto.KeyButMaskShift != 0 && !keypad && !isKeypadKeysym(l1)
	var l3, l4 uint32
	level3 := p.level3Mask != 0 && state&p.level3Mask != 0
	if level3 {
		l3, l4 = p.groupSym(code, g, 2), p.groupSym(code, g, 3)
		switch {
		case n == 3:
			l4 = l3 // no level 4: Shift keeps level 3
		case n > 0:
			level3 = n > 2
		default:
			level3 = l3 != 0 || l4 != 0
		}
	}
	pick := func(s, lv3 bool) uint32 {
		switch {
		case lv3 && s:
			return l4
		case lv3:
			return l3
		case s:
			return in.Shifted
		}
		return in.Plain
	}
	typed := pick(shift, level3)
	// Used up: every active modifier when together they change the
	// keysym, and each one that changes it alone.
	var consumed gui.Modifier
	if typed != in.Plain {
		if shift {
			consumed |= gui.ModShift
		}
		if level3 {
			consumed |= gui.ModAltGr
		}
	}
	if shift && pick(true, false) != in.Plain {
		consumed |= gui.ModShift
	}
	if level3 && pick(false, true) != in.Plain {
		consumed |= gui.ModAltGr
	}
	if state&xproto.KeyButMaskLock != 0 {
		typed = capsLockKeysym(typed, [4]uint32{in.Plain, in.Shifted, l3, l4}, shift, level3)
	}

	pk := pressedKey{code: kc, typed: kc, held: true}
	if t := x11key.KeyCode(x11key.KeyInput{Plain: typed, Evdev: in.Evdev}); t != gui.KeyInvalid && t != kc {
		pk.typed, pk.consumed = t, consumed
	}
	return pk, typed
}

// capsLockKeysym applies Caps Lock to sym, the keysym a press of a key
// with levels lv (1 to 4) types with shift and level3. It follows the key
// types XKB picks by itself, as Xlib and GTK on the same desktop do, not
// the core protocol (#1188). A key whose level 1 is a lowercase and level
// 2 an uppercase letter is alphabetic, on levels 3 and 4 too when they are
// such a pair as well (ALPHABETIC, FOUR_LEVEL_ALPHABETIC); there Caps Lock
// inverts Shift, so Shift+Caps Lock types the lowercase letter. The pair
// need not be one letter's two cases: Turkish i and İ are. Anywhere else
// Caps Lock capitalizes the keysym typed, XKB's Caps Lock transformation:
// AltGr+S types Đ on Albanian.
func capsLockKeysym(sym uint32, lv [4]uint32, shift, level3 bool) uint32 {
	alpha, pair := isCasePair(lv[0], lv[1]), 0
	if level3 {
		alpha, pair = alpha && isCasePair(lv[2], lv[3]), 2
	}
	switch {
	case !alpha:
		return wideKeysymToUpper(sym)
	case shift:
		return lv[pair]
	}
	return lv[pair+1]
}

// isCasePair tells whether levels lo and up make a key alphabetic, by
// XKB's own rule for the types it picks: lo is a lowercase and up an
// uppercase letter in libxkbcommon's case table, not necessarily each
// other's (de: ſ and ẞ on AltGr), and not by Unicode alone (ge: პ/P is
// not). A layout that declares a key alphabetic against that rule, as
// Turkish does for i/İ and ı/I, needs the key types, which the X11 path
// does not read: the same pairs on az, typed alike, are not alphabetic.
func isCasePair(lo, up uint32) bool {
	return wideKeysymToUpper(lo) != lo && wideKeysymToLower(up) != up
}

// lowerKeysym is the lowercase of a Latin-1 or Unicode keysym, sym itself
// otherwise: shiftedKeysym's counterpart, without libxkbcommon.
func lowerKeysym(sym uint32) uint32 {
	const unicodeKeysym = 0x01000000
	var r rune
	switch {
	case sym < 0x100:
		r = rune(sym)
	case sym&0xff000000 == unicodeKeysym:
		r = rune(sym & 0x00ffffff)
	default:
		return sym
	}
	switch l := unicode.ToLower(r); {
	case l == r:
		return sym
	case l < 0x100:
		return uint32(l)
	default:
		return unicodeKeysym | uint32(l)
	}
}
