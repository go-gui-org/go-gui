//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"encoding/xml"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/xkb"
)

// The all-layout sweep of docs/specs/keyboard-layouts.md § Verification
// (#1188), on the harness in keyboard_layouts_linux_test.go.

// xkbRegistry is the part of xkeyboard-config's evdev.xml the sweep reads.
type xkbRegistry struct {
	Layouts []struct {
		Name     string `xml:"configItem>name"`
		Variants []struct {
			Name string `xml:"configItem>name"`
		} `xml:"variantList>variant"`
	} `xml:"layoutList>layout"`
}

// sweepFailures collects invariant violations with a few examples each,
// so one broken rule does not print thousands of lines.
type sweepFailures struct {
	count    map[string]int
	examples map[string][]string
	// x11Skipped counts presses on keys the core keymap cannot describe,
	// where its X11 path is not compared with Wayland (coreRepresentable);
	// level5Skipped those with ISO level 5, compared on neither X11 path.
	x11Skipped, level5Skipped int
	// capsTypeSkipped counts Caps Lock presses on keys whose XKB key type
	// gives Lock a meaning of its own (lockTypeIsAutomatic), which the core
	// keymap does not carry.
	capsTypeSkipped int
	// keyTypeSkipped counts presses on keys whose XKB key type picks
	// another level than the standard types do (standardLevel); its XKB
	// map path is not compared there: the map carries no key types.
	keyTypeSkipped int
}

func (f *sweepFailures) add(rule, format string, args ...any) {
	f.count[rule]++
	if len(f.examples[rule]) < 8 {
		f.examples[rule] = append(f.examples[rule], fmt.Sprintf(format, args...))
	}
}

// Keysym ranges and values the sweep needs (keysymdef.h).
const (
	symLevel3Shift = 0xfe03
	symCapsLockKey = 0xffe5
	symKPBegin     = 0xff9d
	symKPFirst     = 0xff80 // keypad keysyms, X11 protocol § 5
	symKPLast      = 0xffbd
	symDeadFirst   = 0xfe50
	symDeadLast    = 0xfe8f
)

// TestKeyboardAllLayouts is the spec's all-layout sweep. For every layout
// and variant xkeyboard-config lists, every key, plain, with Shift, with
// level 3 and with both, each with and without Caps Lock, it checks the
// invariants of § Verification.
func TestKeyboardAllLayouts(t *testing.T) {
	if testing.Short() {
		t.Skip("all-layout sweep is slow")
	}
	requireXKB(t)
	data, err := os.ReadFile("/usr/share/X11/xkb/rules/evdev.xml")
	if err != nil {
		if os.Getenv("GOGUI_REQUIRE_XKB") == "1" {
			t.Fatalf("GOGUI_REQUIRE_XKB is set but the layout registry is missing: %v", err)
		}
		t.Skip(err)
	}
	var reg xkbRegistry
	if err = xml.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	f := &sweepFailures{count: map[string]int{}, examples: map[string][]string{}}
	// Under -race the sweep takes ~100 s and finds no more: it is one
	// goroutine. Every tenth layout still runs there; plain runs do all.
	stride := 1
	if raceEnabled {
		stride = 10
	}
	layouts, seen := 0, 0
	for _, l := range reg.Layouts {
		if l.Name == "custom" {
			continue // a placeholder for a user's own symbols file
		}
		variants := []string{""}
		for _, v := range l.Variants {
			variants = append(variants, v.Name)
		}
		for _, v := range variants {
			seen++
			if seen%stride != 0 {
				continue
			}
			km, cerr := xkb.NewKeymapFromNames("evdev", "pc105", l.Name, v, "")
			if cerr != nil {
				continue // a registry entry this libxkbcommon cannot build
			}
			km.Destroy()
			layouts++
			// A subtest per layout, so its harness (two keymaps and a
			// window) is freed when it ends rather than with the sweep.
			// Violations still collect in f and the parent reports them.
			t.Run(l.Name+"("+v+")", func(st *testing.T) {
				sweepLayout(st, f, l.Name, v)
			})
		}
	}
	t.Logf("swept %d layouts and variants; X11 not compared on %d level-5 presses and %d Caps Lock presses"+
		" on keys whose XKB type gives Lock its own level; its XKB map path not on %d presses of keys"+
		" whose type picks a level of its own, its core keymap path not on %d of keys that keymap"+
		" cannot describe",
		layouts, f.level5Skipped, f.capsTypeSkipped, f.keyTypeSkipped, f.x11Skipped)
	for rule, n := range f.count {
		t.Errorf("%s: %d violations, e.g.\n  %s", rule, n, strings.Join(f.examples[rule], "\n  "))
	}
}

func sweepLayout(t *testing.T, f *sweepFailures, layout, variant string) {
	h := newKbHarness(t, layout, variant)
	name := layout
	if variant != "" {
		name += "(" + variant + ")"
	}
	lfsh := h.code("LFSH")
	core := coreKeymapFor(h.seat.keymap)
	xkbMap := xkbPlatformFor(h.seat.keymap)
	level3 := uint32(0)
	if c := h.server.KeyByName("RALT"); c != 0 && h.seat.keymap.BaseSym(c) == symLevel3Shift {
		level3 = c
	}
	combos := [][]uint32{nil, {lfsh}}
	if level3 != 0 {
		combos = append(combos, []uint32{level3}, []uint32{lfsh, level3})
	}
	// Each combination again with Caps Lock on, where CAPS is Caps Lock.
	caps := h.server.KeyByName("CAPS")
	if caps != 0 && h.seat.keymap.BaseSym(caps) != symCapsLockKey {
		caps = 0
	}
	skip := map[uint32]bool{}
	for _, n := range []string{"CAPS", "NMLK", "LFSH", "RTSH", "RALT", "LCTL", "RCTL", "LALT", "LWIN", "RWIN"} {
		if c := h.server.KeyByName(n); c != 0 {
			skip[c] = true
		}
	}
	for code := uint32(9); code < 256; code++ {
		base := h.seat.keymap.BaseSym(code)
		if base == 0 || skip[code] {
			continue
		}
		levels := make([]uint32, len(combos)) // each combination's level without Caps Lock
		for i := range 2 * len(combos) {
			held, capsOn := combos[i%len(combos)], i >= len(combos)
			if capsOn {
				if caps == 0 {
					break
				}
				h.keyCode(caps, true) // setLayout below unlocks it again
				h.keyCode(caps, false)
			}
			for _, c := range held {
				h.keyCode(c, true)
			}
			// What the press types and whether level 3 is active, from
			// libxkbcommon: real modifier Mod5 is bit 7 in every keymap.
			// Shift+RALT is not level 3 on every layout (de(T3)).
			sym, level := h.server.Sym(code), h.server.Level(code)
			if !capsOn {
				levels[i] = level
			}
			d, l, lk, _ := h.server.Serialize()
			level3On := (d|l|lk)&xkb.MaskMod5 != 0
			for _, c := range slices.Backward(held) {
				h.keyCode(c, false)
			}
			r := h.pressCodes(append(append([]uint32(nil), held...), code)...)
			// Some layouts put ISO_Level3_Latch or a lock on a key (mm,
			// cm, cn(tib)). The key after it then rightly types on level 3
			// with ModAltGr; clear such state so each press starts fresh.
			h.setLayout(0)
			where := fmt.Sprintf("%s key %d held %v caps %v", name, code, held, capsOn)
			if !r.gotDown {
				continue // the window took it (Tab, F12)
			}
			// The text the key types equals libxkbcommon's.
			if want := xkb.KeysymToRune(sym); want >= 0x20 && want != 0x7f &&
				(sym < symDeadFirst || sym > symDeadLast) && r.char != want {
				f.add("char equals libxkbcommon", "%s: got %q, want %q", where, r.char, want)
			}
			// A key whose first level is printable ASCII, or a keypad key,
			// has a KeyCode.
			kp := base >= symKPFirst && base <= symKPLast && base != symKPBegin
			if (base > 0x20 && base < 0x7f || kp) && r.down.KeyCode == gui.KeyInvalid {
				f.add("printable or keypad key has a KeyCode", "%s: base keysym %#x", where, base)
			}
			// Key up repeats key down.
			if r.gotUp && r.up.KeyCode != r.down.KeyCode {
				f.add("key up repeats key down", "%s: up %d, down %d", where, r.up.KeyCode, r.down.KeyCode)
			}
			// ModAltGr is set exactly while level 3 is active.
			if got := r.down.Modifiers.Has(gui.ModAltGr); got != level3On {
				f.add("ModAltGr only with level 3", "%s: ModAltGr %v", where, got)
			}
			// The X11 path reports what the Wayland path does: through the
			// XKB map a server with this layout sends, on every key; through
			// the core keymap Xorg derives from it, on every key that keymap
			// can describe (coreRepresentable). Level 5 (Mod3, de(T3), neo)
			// and a layout's own Caps Lock key type are beyond both.
			if (d|l|lk)&x11Mod3 != 0 {
				f.level5Skipped++
				continue
			}
			if capsOn && !lockTypeIsAutomatic(h.seat.keymap.Levels(code), levels[i-len(combos)], level, sym) {
				f.capsTypeSkipped++
				continue
			}
			compare := func(rule string, p *platformState) {
				// libxkbcommon's real modifiers are the X11 core state bits.
				x, xc := x11PressCode(p, code, uint16(d|l|lk)&0xff)
				if x.KeyCode != r.down.KeyCode || x.TypedKey != r.down.TypedKey ||
					x.Modifiers != r.down.Modifiers || x.TypedModifiers != r.down.TypedModifiers || xc != r.char {
					f.add(rule,
						"%s: x11 KeyCode %d TypedKey %d mods %#x/%#x char %q; wayland %d %d %#x/%#x %q",
						where, x.KeyCode, x.TypedKey, x.Modifiers, x.TypedModifiers, xc,
						r.down.KeyCode, r.down.TypedKey, r.down.Modifiers, r.down.TypedModifiers, r.char)
				}
			}
			lv := h.seat.keymap.Levels(code)
			noLock := level
			if capsOn {
				noLock = levels[i-len(combos)]
			}
			if len(lv) == 1 && !standardType(lv[0], noLock, slices.Contains(held, lfsh), level3On) {
				f.keyTypeSkipped++
			} else {
				compare("x11 (xkb map) agrees with wayland", xkbMap)
			}
			if coreRepresentable(h.seat.keymap.Levels(code)) {
				compare("x11 (core keymap) agrees with wayland", core)
			} else {
				f.x11Skipped++
			}
		}
	}
}

// lockTypeIsAutomatic tells whether Caps Lock acts on a key of the first
// layout, given its levels, as the key type xkeyboard-config picks by
// itself would: noLock is the level without Lock, withLock and sym the
// level and keysym libxkbcommon gives with it. On a pair of a lowercase
// letter and its capital (levels 1 and 2, or 3 and 4) Lock inverts Shift;
// on any other pair it keeps the level and capitalizes the keysym. A
// layout may name a type of its own that gives Lock another meaning
// (Shift+Lock capitals on us(mac), Lock on the digit row of fr(bepo),
// level 5 for ẞ on de, Lock used up on fr(dvorak)'s FOUR_LEVEL_PLUS_LOCK);
// the core keymap carries no key types, so the X11 path cannot follow it.
func lockTypeIsAutomatic(levels [][]uint32, noLock, withLock, sym uint32) bool {
	if len(levels) == 0 {
		return true
	}
	lv := levels[0]
	lo := noLock &^ 1
	if int(lo)+1 >= len(lv) || int(withLock) >= len(lv) {
		return int(withLock) < len(lv) && withLock == noLock && sym == xkb.KeysymToUpper(lv[withLock])
	}
	if up := xkb.KeysymToUpper(lv[lo]); up != lv[lo] && up == lv[lo+1] {
		return withLock == noLock^1 && sym == lv[withLock]
	}
	return withLock == noLock && sym == xkb.KeysymToUpper(lv[withLock])
}

// standardType tells whether libxkbcommon's level on a one-group key with
// levels lv types what standardLevel's does. Shift with level 3 on a keypad
// key never counts: the four-level keypad types disagree on whether that
// uses Shift up (fr(oss_latin9) yes, ir(pes_keypad) no).
func standardType(lv []uint32, level uint32, shift, level3 bool) bool {
	if shift && level3 && len(lv) > 2 && slices.ContainsFunc(lv, isKeypadKeysym) {
		return false
	}
	std := standardLevel(lv, shift, level3)
	return int(level) < len(lv) && lv[level] == lv[std]
}

// standardLevel is the level, from 0, xkeyboard-config's standard key types
// pick on a one-group key with levels lv, NumLock off: Shift the second,
// level 3 the third, both the fourth, each only where the key has it; a
// keypad key ignores Shift. A layout may name a type that orders its
// levels otherwise (cm(azerty)'s digit row, the four-level keypad types of
// fr(bepo)); only the key types say so, and the X11 XKB map path reads
// none.
func standardLevel(lv []uint32, shift, level3 bool) uint32 {
	n := len(lv)
	if n > 1 && (isKeypadKeysym(lv[0]) || isKeypadKeysym(lv[1])) {
		shift = false
	}
	var l int
	if level3 && n > 2 {
		l = 2
	}
	if shift && l+1 < n {
		l++
	}
	return uint32(l)
}
