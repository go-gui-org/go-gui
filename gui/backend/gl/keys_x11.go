//go:build linux && !js && !android

package gl

import (
	"strconv"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/x11key"
)

// Key presses on the X11 core keymap (#1188, docs/specs/keyboard-layouts.md).
//
// go-gui reads the core keymap, not XKB, so it applies the core protocol's
// rules itself: the group, NumLock on keypad keysyms, Shift, Caps Lock.
// Xorg publishes each key's columns as the first group's two levels, the
// second group's two levels, then the first group's further levels (3, 4,
// ...) and after them the second group's. ISO level 3 (AltGr) selects
// levels 3 and 4. Where the second group's start depends on how many
// levels the key has in the first, which the core keymap does not say, so
// level 3 is read for the first group only. Which modifier bits NumLock and
// level 3 sit on comes from the modifier mapping, not a fixed Mod2 and Mod5.

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
// own keymap events instead (xkbNotifyEvent). Without the extension the
// core behavior stays.
func useXKBExtension(conn *xgb.Conn) {
	q, err := xproto.QueryExtension(conn, uint16(len("XKEYBOARD")), "XKEYBOARD").Reply()
	if err != nil || q == nil || !q.Present {
		return
	}
	// xgb has no XKB package; requests are raw, little-endian as xgb
	// speaks. UseExtension (minor 0): length 2 words, wanted version 1.0.
	cookie := conn.NewCookie(true, true)
	conn.NewRequest([]byte{q.MajorOpcode, 0, 2, 0, 1, 0, 0, 0}, cookie)
	if reply, rerr := cookie.Reply(); rerr != nil || len(reply) < 2 || reply[1] == 0 {
		return
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

// loadKeyboardInfo reads which modifier bits hold NumLock and ISO level 3,
// and which core groups type Latin letters. It runs after the keymap is
// read, and drops the press records, which may name keys of the old map.
func (p *platformState) loadKeyboardInfo(conn *xgb.Conn) {
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
	for g := range p.latinGroups {
		p.latinGroups[g] = p.groupIsLatin(g)
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
		if s := p.keysym(code, g*coreColumnsPerGroup); s >= 'a' && s <= 'z' || s >= 'A' && s <= 'Z' {
			return true
		}
	}
	return false
}

// group is the core group a press reads: the second when the server
// reports a group in bits 13-14 of the state and the key lists anything
// there. The core keymap holds two groups; a third or fourth reads as the
// second.
func (p *platformState) group(code xproto.Keycode, state uint16) int {
	if (state>>13)&3 == 0 {
		return 0
	}
	if p.keysym(code, 2) == 0 && p.keysym(code, 3) == 0 {
		return 0 // the protocol's rule: an empty second group is the first
	}
	return 1
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
	l1, l2 := p.keysym(code, g*coreColumnsPerGroup), p.keysym(code, g*coreColumnsPerGroup+1)
	in := x11key.KeyInput{Plain: l1, Shifted: l2, Evdev: uint32(code) - 8}
	keypad := isKeypadKeysym(l2)
	if keypad && p.numLockMask != 0 && state&p.numLockMask != 0 {
		in.Plain, in.Shifted = l2, l1
	}
	if !p.latinGroups[g] {
		in.NonLatin = true
		if o := 1 - g; p.latinGroups[o] {
			in.Latin = p.keysym(code, o*coreColumnsPerGroup)
		}
	}
	kc := x11key.KeyCode(in)

	// Shift and level 3 count only on keys they can change: Shift not on a
	// keypad key, level 3 not on a key that lists no level 3 or 4. A key
	// that lists one of them types nothing on the other if it is empty.
	shift := state&xproto.KeyButMaskShift != 0 && !keypad
	var l3, l4 uint32
	level3 := g == 0 && p.level3Mask != 0 && state&p.level3Mask != 0
	if level3 {
		l3, l4 = p.keysym(code, coreLevel3Column), p.keysym(code, coreLevel3Column+1)
		level3 = l3 != 0 || l4 != 0
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
		lo, up := in.Plain, in.Shifted
		if level3 {
			lo, up = l3, l4
		}
		typed = capsLockKeysym(typed, lo, up, shift)
	}

	pk := pressedKey{code: kc, typed: kc, held: true}
	if t := x11key.KeyCode(x11key.KeyInput{Plain: typed, Evdev: in.Evdev}); t != gui.KeyInvalid && t != kc {
		pk.typed, pk.consumed = t, consumed
	}
	return pk, typed
}

// capsLockKeysym applies Caps Lock to sym, the keysym a press of the level
// pair lo, up types with shift. It follows XKB, as Xlib and GTK on the
// same desktop do, not the core protocol (#1188). On a pair of a lowercase
// letter and its capital (an alphabetic key type) Caps Lock inverts Shift,
// so Shift+Caps Lock types the lowercase letter. On any other pair it
// capitalizes the keysym typed, as XKB's Caps Lock transformation does
// when the key type leaves Lock unused: AltGr+S types Đ on Albanian.
func capsLockKeysym(sym, lo, up uint32, shift bool) uint32 {
	if c := wideKeysymToUpper(lo); c != lo && c == up {
		if shift {
			return lo
		}
		return up
	}
	return wideKeysymToUpper(sym)
}
