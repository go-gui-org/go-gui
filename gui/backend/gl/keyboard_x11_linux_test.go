//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ibus"
	"github.com/go-gui-org/go-gui/gui/backend/internal/x11key"
	"github.com/go-gui-org/go-gui/gui/backend/internal/xkb"
)

// The X11 path of docs/specs/keyboard-layouts.md (#1188), checked without
// an X server: each test builds the core keymap Xorg publishes for a
// layout, from libxkbcommon's compile of the same layout, and presses keys
// through platformState.readPress.

// X11 core state bits (X.h) and where Xorg puts NumLock and ISO level 3:
// the modifier map of an xkeyboard-config keymap binds Num_Lock to Mod2 and
// ISO_Level3_Shift to Mod5 (checked against Xorg 21 with xgb).
const (
	x11Shift   = xproto.KeyButMaskShift
	x11Lock    = xproto.KeyButMaskLock
	x11Control = xproto.KeyButMaskControl
	x11Mod1    = xproto.KeyButMaskMod1
	x11Mod2    = xproto.KeyButMaskMod2
	x11Mod3    = xproto.KeyButMaskMod3 // ISO level 5
	x11Mod4    = xproto.KeyButMaskMod4
	x11Mod5    = xproto.KeyButMaskMod5
	x11MinKC   = 8
	x11MaxKC   = 255
)

// coreKeymapFor is the platform state an X11 window has for km: the core
// keymap Xorg derives from it and the modifier bits of NumLock and level 3.
// Per key the columns are the first group's two levels, the second group's
// two (a copy of the first's on a key with one group), then the first
// group's further levels and the second's, as Xorg publishes them.
func coreKeymapFor(km *xkb.Keymap) *platformState {
	rows := make([][]uint32, x11MaxKC-x11MinKC+1)
	width := 4
	at := func(s []uint32, i int) uint32 {
		if i < len(s) {
			return s[i]
		}
		return 0
	}
	for code := x11MinKC; code <= x11MaxKC; code++ {
		lv := km.Levels(uint32(code))
		if len(lv) == 0 {
			continue
		}
		g1, g2 := lv[0], lv[0]
		if len(lv) > 1 {
			g2 = lv[1]
		}
		row := []uint32{at(g1, 0), at(g1, 1), at(g2, 0), at(g2, 1)}
		if len(g1) > 2 {
			row = append(row, g1[2:]...)
		}
		if len(g2) > 2 {
			row = append(row, g2[2:]...)
		}
		rows[code-x11MinKC] = row
		width = max(width, len(row))
	}
	syms := make([]xproto.Keysym, len(rows)*width)
	for i, row := range rows {
		for j, s := range row {
			syms[i*width+j] = xproto.Keysym(s)
		}
	}
	p := &platformState{
		keymap: &xproto.GetKeyboardMappingReply{
			KeysymsPerKeycode: byte(width),
			Keysyms:           syms,
		},
		minKeycode:  x11MinKC,
		numLockMask: x11Mod2,
		level3Mask:  x11Mod5,
	}
	for g := range p.latinGroups {
		p.latinGroups[g] = p.groupIsLatin(g)
	}
	return p
}

// coreRepresentable reports whether the core keymap describes a key as
// XKB does, given its levels per layout. Such keys differ on X11 by design
// (spec § 6); it is false when
//   - a level up to the fourth is explicitly NoSymbol: the core keymap's
//     empty column means "absent", but a key types nothing on an XKB
//     NoSymbol level;
//   - a one-level key holds a cased letter: XKB ignores Shift there, the
//     core protocol's rule (#948) gives the capital;
//   - a keypad keysym is on a key other than the plain keypad shape (two
//     levels, both keypad keysyms): keypad types the core keymap has no
//     notion of, where XKB (with its preserved modifiers) and the
//     protocol's NumLock rule disagree.
func coreRepresentable(levels [][]uint32) bool {
	if len(levels) == 0 {
		return true
	}
	g := levels[0]
	keypad := 0
	for i := 0; i < len(g) && i < 4; i++ {
		if g[i] == 0 {
			return false
		}
		if isKeypadKeysym(g[i]) {
			keypad++
		}
	}
	if len(g) == 1 {
		return shiftedKeysym(g[0]) == g[0]
	}
	return keypad == 0 || keypad == 2 && len(g) == 2
}

// x11ModBit is the state bit holding key name down sets.
func x11ModBit(km *xkb.Keymap, name string, group int) uint16 {
	switch name {
	case "LFSH", "RTSH":
		return x11Shift
	case "LCTL", "RCTL":
		return x11Control
	case "LALT":
		return x11Mod1
	case "LWIN", "RWIN":
		return x11Mod4
	case "RALT":
		// What the key does in the active group: Alt_R on us, level 3 on de.
		lv := km.Levels(km.KeyByName("RALT"))
		if g := lv[min(group, len(lv)-1)]; len(g) > 0 && g[0] == symISOLevel3Shift {
			return x11Mod5
		}
		return x11Mod1
	}
	return 0
}

// x11Press presses keys[len-1] with the others held, as the X11 path sees
// it, and returns the key down it reports and the character it types.
func x11Press(t *testing.T, p *platformState, km *xkb.Keymap, keys []string, state uint16) (gui.Event, rune) {
	t.Helper()
	for _, n := range keys[:len(keys)-1] {
		state |= x11ModBit(km, n, int(state>>13)&3)
	}
	code := km.KeyByName(keys[len(keys)-1])
	if code == 0 {
		t.Fatalf("keymap has no key <%s>", keys[len(keys)-1])
	}
	return x11PressCode(p, code, state)
}

// x11PressCode is x11Press for a keycode and a full state.
func x11PressCode(p *platformState, code uint32, state uint16) (gui.Event, rune) {
	pk, sym := p.readPress(xproto.Keycode(code), state)
	r := keyPressCharWide(&p.compose, sym, state, wideKeysymToRune)
	p.compose.reset()
	mods := p.modifiers(state)
	return gui.Event{
		Type: gui.EventKeyDown, KeyCode: pk.code, Modifiers: mods,
		TypedKey: pk.typed, TypedModifiers: mods &^ pk.consumed,
	}, r
}

// xkbReplyFor encodes the GetMap reply (KeySyms part) a server with km
// sends: per key its layouts × its widest layout's levels, padded with
// NoSymbol. libxkbcommon does not say a key's out-of-range rule; every key
// wraps, its default.
func xkbReplyFor(km *xkb.Keymap) []byte {
	keys := make([]testXKBKey, 0, x11MaxKC-x11MinKC+1)
	for code := x11MinKC; code <= x11MaxKC; code++ {
		lv := km.Levels(uint32(code))
		k := testXKBKey{groupInfo: byte(len(lv))}
		for _, g := range lv {
			k.width = max(k.width, byte(len(g)))
		}
		for _, g := range lv {
			k.syms = append(k.syms, g...)
			k.syms = append(k.syms, make([]uint32, int(k.width)-len(g))...)
		}
		keys = append(keys, k)
	}
	return testXKBReply(x11MinKC, keys)
}

// xkbPlatformFor is coreKeymapFor with the XKB map a server with km sends,
// read through the parser the backend uses.
func xkbPlatformFor(km *xkb.Keymap) *platformState {
	p := coreKeymapFor(km)
	m, err := parseXKBKeySyms(xkbReplyFor(km))
	if err != nil {
		panic(err)
	}
	p.xkbMap = m
	for g := range p.latinGroups {
		p.latinGroups[g] = g < m.maxGroups && p.groupIsLatin(g)
	}
	return p
}

func runGoldenX11(t *testing.T, layout, variant string, layoutIdx uint32, rows []goldenRow,
	platformFor func(*xkb.Keymap) *platformState) {
	km, err := xkb.NewKeymapFromNames("evdev", "pc105", layout, variant, "")
	if err != nil {
		t.Fatal(err)
	}
	defer km.Destroy()
	p := platformFor(km)
	base := uint16(layoutIdx) << 13 // the group, as an XKB-aware server reports it
	for _, row := range rows {
		state := base
		if row.numLock {
			state |= x11Mod2
		}
		down, char := x11Press(t, p, km, row.keys, state)
		checkGoldenRow(t, goldenRowName(row), row, down, char)
	}
}

// TestKeyboardX11CapsLock: Caps Lock types capitals on X11, Latin and
// Cyrillic, as XKB does: Shift+Caps Lock types the lowercase letter, and
// a level-3 letter is capitalized. It does not change the key a press
// reports.
func TestKeyboardX11CapsLock(t *testing.T) {
	requireXKB(t)
	cases := []struct {
		layout string
		key    string
		state  uint16
		want   rune
		code   gui.KeyCode
	}{
		{"us", "AC01", x11Lock, 'A', gui.KeyA},
		{"us", "AC01", x11Lock | x11Shift, 'a', gui.KeyA}, // XKB, not the protocol's 'A'
		{"us", "AE01", x11Lock, '1', gui.Key1},
		{"us", "AE01", x11Lock | x11Shift, '!', gui.Key1},
		{"de", "AC11", x11Lock, 'Ä', gui.KeyInvalid},
		{"de", "AC11", x11Lock | x11Shift, 'ä', gui.KeyInvalid},
		{"ru", "AB03", x11Lock, 'С', gui.KeyC},
		{"ru", "AB03", x11Lock | x11Shift, 'с', gui.KeyC},
		{"al", "AC02", x11Lock | x11Mod5, 'Đ', gui.KeyS}, // AltGr+S: a legacy keysym, capitalized
		{"de", "AD03", x11Lock | x11Mod5, '€', gui.KeyE},
	}
	for _, c := range cases {
		km, err := xkb.NewKeymapFromNames("evdev", "pc105", c.layout, "", "")
		if err != nil {
			t.Fatal(err)
		}
		p := coreKeymapFor(km)
		down, char := x11Press(t, p, km, []string{c.key}, c.state)
		if char != c.want || down.KeyCode != c.code {
			t.Errorf("%s %s state %#x: char %q KeyCode %d, want %q %d",
				c.layout, c.key, c.state, char, down.KeyCode, c.want, c.code)
		}
		km.Destroy()
	}
}

// TestKeyboardX11USRegression is the US regression guard on the X11 path:
// with the US layout active, in any group position (usPositions), every
// key that had a KeyCode on us alone keeps it, with the same Modifiers,
// and the typed pass reaches only constants #1188 added. Through the XKB
// map on every position, through the core keymap on us alone.
func TestKeyboardX11USRegression(t *testing.T) {
	requireXKB(t)
	us, err := xkb.NewKeymapFromNames("evdev", "pc105", "us", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer us.Destroy()
	t.Run("core/us", func(t *testing.T) { usRegressionX11(t, us, coreKeymapFor(us), 0) })
	for _, pos := range usPositions {
		t.Run("xkb/"+pos.layouts, func(t *testing.T) {
			km, kerr := xkb.NewKeymapFromNames("evdev", "pc105", pos.layouts, "", "")
			if kerr != nil {
				t.Fatal(kerr)
			}
			defer km.Destroy()
			usRegressionX11(t, us, xkbPlatformFor(km), pos.group)
		})
	}
}

func usRegressionX11(t *testing.T, us *xkb.Keymap, p *platformState, group uint32) {
	km := us
	held := []struct {
		state uint16
		mods  gui.Modifier
	}{
		{0, gui.ModNone},
		{x11Shift, gui.ModShift},
		{x11Control, gui.ModCtrl},
		{x11Control | x11Shift, gui.ModCtrlShift},
		{x11Mod1, gui.ModAlt},
		{x11Mod4, gui.ModSuper},
	}
	for _, hd := range held {
		if m := x11key.MapModifiers(hd.state); m != hd.mods {
			t.Errorf("state %#x: Modifiers %#x, want %#x", hd.state, m, hd.mods)
		}
	}
	checked := 0
	for code := uint32(x11MinKC + 1); code <= x11MaxKC; code++ {
		want := legacyUSKeyCode(km.BaseSym(code))
		if want == gui.KeyInvalid {
			continue
		}
		for _, hd := range held {
			state := hd.state | uint16(group)<<13 // the group, as XKB reports it
			pk, _ := p.readPress(xproto.Keycode(code), state)
			checked++
			if m := p.modifiers(state); m != hd.mods {
				t.Errorf("key %d state %#x: Modifiers %#x, want %#x", code, state, m, hd.mods)
			}
			if pk.code != want {
				t.Errorf("key %d state %#x: KeyCode %d, want %d", code, hd.state, pk.code, want)
			}
			if pk.typed != pk.code && !newASCIIKeys[pk.typed] {
				t.Errorf("key %d state %#x: TypedKey %d existed before #1188", code, hd.state, pk.typed)
			}
		}
	}
	if checked < 300 {
		t.Fatalf("checked only %d presses", checked)
	}
}

// TestKeyboardX11WithoutXKBTable: without libxkbcommon the X11 path types
// with x11key's table alone; € and the numpad's characters must still
// arrive (#1188).
func TestKeyboardX11WithoutXKBTable(t *testing.T) {
	requireXKB(t) // to build the core keymap, not to type
	km, err := xkb.NewKeymapFromNames("evdev", "pc105", "de", "nodeadkeys", "")
	if err != nil {
		t.Fatal(err)
	}
	defer km.Destroy()
	p := coreKeymapFor(km)
	cases := []struct {
		keys  []string
		state uint16
		want  rune
	}{
		{[]string{"RALT", "AD03"}, 0, '€'},
		{[]string{"KP7"}, x11Mod2, '7'},
		{[]string{"KP0"}, x11Mod2, '0'},
		{[]string{"KPDL"}, x11Mod2, ','},
		{[]string{"KPMU"}, 0, '*'},
		{[]string{"KPAD"}, 0, '+'},
		{[]string{"KPSU"}, 0, '-'},
		{[]string{"KPDV"}, 0, '/'},
		{[]string{"KP7"}, 0, 0}, // NumLock off: Home
		{[]string{"KPEN"}, 0, 0},
	}
	for _, c := range cases {
		state := c.state
		for _, n := range c.keys[:len(c.keys)-1] {
			state |= x11ModBit(km, n, int(state>>13)&3)
		}
		_, sym := p.readPress(xproto.Keycode(km.KeyByName(c.keys[len(c.keys)-1])), state)
		r := keyPressCharWide(&p.compose, sym, state, x11key.KeysymToRune)
		p.compose.reset()
		if r != c.want {
			t.Errorf("%v state %#x: char %q, want %q", c.keys, state, r, c.want)
		}
	}
}

// TestKeysymTablesAgree: every keysym x11key's table types, it types as
// libxkbcommon does, so the X11 path types the same with or without it.
func TestKeysymTablesAgree(t *testing.T) {
	requireXKB(t)
	mapped := 0
	for sym := range uint32(0x10000) {
		r := x11key.KeysymToRune(sym)
		if r == 0 {
			continue
		}
		mapped++
		if want := xkb.KeysymToRune(sym); r != want {
			t.Errorf("keysym %#x: x11key types %q, libxkbcommon %q", sym, r, want)
		}
	}
	if mapped < 200 {
		t.Fatalf("x11key maps only %d keysyms", mapped)
	}
}

// TestKeyboardX11Level3Bit: ModAltGr follows the bit the modifier mapping
// puts ISO level 3 on. xkeyboard-config uses Mod5; with a custom mapping
// on Mod3, AltGr+Q must still be AltGr, or a Q shortcut would win.
func TestKeyboardX11Level3Bit(t *testing.T) {
	requireXKB(t)
	km, err := xkb.NewKeymapFromNames("evdev", "pc105", "de", "nodeadkeys", "")
	if err != nil {
		t.Fatal(err)
	}
	defer km.Destroy()
	p := coreKeymapFor(km)
	p.level3Mask = x11Mod3
	down, char := x11PressCode(p, km.KeyByName("AD01"), x11Mod3)
	if char != '@' || down.KeyCode != gui.KeyQ || down.Modifiers != gui.ModAltGr ||
		down.TypedKey != gui.KeyAt || down.TypedModifiers != gui.ModNone {
		t.Errorf("AltGr+Q on Mod3: %+v char %q", down, char)
	}
	if m := p.modifiers(x11Mod5); m.Has(gui.ModAltGr) {
		t.Errorf("Mod5 is not level 3 here, Modifiers %#x", m)
	}
}

// TestKeyboardX11IMEForwardedKey: a key the input method forwards reports
// what the same press reports without one.
func TestKeyboardX11IMEForwardedKey(t *testing.T) {
	requireXKB(t)
	km, err := xkb.NewKeymapFromNames("evdev", "pc105", "de", "nodeadkeys", "")
	if err != nil {
		t.Fatal(err)
	}
	defer km.Destroy()
	p := coreKeymapFor(km)
	shift7 := ibus.Event{Kind: ibus.KindForwardKey, Keyval: '/', Keycode: km.KeyByName("AE07") - 8, State: x11Shift}
	got := imeEvents(p, []ibus.Event{shift7}, nil)
	if len(got) != 2 || got[0].KeyCode != gui.Key7 || got[0].Modifiers != gui.ModShift ||
		got[0].TypedKey != gui.KeySlash || got[0].TypedModifiers != gui.ModNone || got[1].CharCode != '/' {
		t.Fatalf("forwarded Shift+7: %+v", got)
	}
	// A forwarded Cyrillic key types its letter and keeps its Latin key,
	// as the same press does directly.
	ru, err := xkb.NewKeymapFromNames("evdev", "pc105", "ru", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ru.Destroy()
	es := ibus.Event{Kind: ibus.KindForwardKey, Keyval: ru.BaseSym(ru.KeyByName("AB03")), Keycode: ru.KeyByName("AB03") - 8}
	got = imeEvents(coreKeymapFor(ru), []ibus.Event{es}, nil)
	if len(got) != 2 || got[0].KeyCode != gui.KeyC || got[1].CharCode != 'с' {
		t.Fatalf("forwarded Cyrillic es: %+v", got)
	}
	// A keysym the keymap does not type there, or no keycode: its own key.
	for _, ev := range []ibus.Event{
		{Kind: ibus.KindForwardKey, Keyval: '/', State: x11Shift},
		{Kind: ibus.KindForwardKey, Keyval: '/', Keycode: km.KeyByName("AE08") - 8, State: x11Shift},
	} {
		got = imeEvents(p, []ibus.Event{ev}, nil)
		if got[0].KeyCode != gui.KeySlash || got[0].TypedKey != gui.KeySlash ||
			got[0].TypedModifiers != gui.ModShift {
			t.Errorf("forwarded %+v: %+v", ev, got[0])
		}
	}
}

// TestKeyboardCapsLockTurkish: Turkish i/İ and ı/I under Caps Lock. tr
// declares both keys alphabetic, so Caps Lock picks level 2 (İ, I); az has
// the same pairs with types XKB picks itself, which are not alphabetic.
// Only the key types tell the two apart, and the X11 path reads none: its
// subtests are skipped until it does (docs/specs/keyboard-layouts.md).
func TestKeyboardCapsLockTurkish(t *testing.T) {
	requireXKB(t)
	cases := []struct {
		keys []string
		caps bool
		want rune
	}{
		{[]string{"AC11"}, false, 'i'},
		{[]string{"AC11"}, true, 'İ'},
		{[]string{"LFSH", "AC11"}, true, 'i'},
		{[]string{"RALT", "AC11"}, true, '\''}, // levels 3-4 not a letter pair
		{[]string{"AD08"}, false, 'ı'},
		{[]string{"AD08"}, true, 'I'},
		{[]string{"LFSH", "AD08"}, true, 'ı'},
		{[]string{"RALT", "AD08"}, true, 'Î'}, // FOUR_LEVEL_ALPHABETIC
		{[]string{"LFSH", "RALT", "AD08"}, true, 'î'},
	}
	t.Run("wayland", func(t *testing.T) {
		h := newKbHarness(t, "tr", "")
		for _, c := range cases {
			if c.caps {
				h.tapLock("CAPS")
			}
			r := h.press(c.keys...)
			if c.caps {
				h.tapLock("CAPS")
			}
			if r.char != c.want {
				t.Errorf("%v caps %v: char %q, want %q", c.keys, c.caps, r.char, c.want)
			}
		}
	})
	km, err := xkb.NewKeymapFromNames("evdev", "pc105", "tr", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer km.Destroy()
	for name, platformFor := range map[string]func(*xkb.Keymap) *platformState{
		"x11-core": coreKeymapFor, "x11-xkb": xkbPlatformFor,
	} {
		t.Run(name, func(t *testing.T) {
			t.Skip("Turkish Caps Lock needs XKB key types, which the X11 path does not read")
			p := platformFor(km)
			for _, c := range cases {
				var state uint16
				if c.caps {
					state = x11Lock
				}
				if _, char := x11Press(t, p, km, c.keys, state); char != c.want {
					t.Errorf("%v caps %v: char %q, want %q", c.keys, c.caps, char, c.want)
				}
			}
		})
	}
}
