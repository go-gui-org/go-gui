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
	// where X11 is not compared with Wayland (coreRepresentable).
	x11Skipped int
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
	symKPBegin     = 0xff9d
	symKPFirst     = 0xff80 // keypad keysyms, X11 protocol § 5
	symKPLast      = 0xffbd
	symDeadFirst   = 0xfe50
	symDeadLast    = 0xfe8f
)

// TestKeyboardAllLayouts is the spec's all-layout sweep. For every layout
// and variant xkeyboard-config lists, every key, plain, with Shift, with
// level 3 and with both, it checks the invariants of § Verification.
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
	t.Logf("swept %d layouts and variants; X11 not compared on %d presses of keys the core keymap cannot describe",
		layouts, f.x11Skipped)
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
	level3 := uint32(0)
	if c := h.server.KeyByName("RALT"); c != 0 && h.seat.keymap.BaseSym(c) == symLevel3Shift {
		level3 = c
	}
	combos := [][]uint32{nil, {lfsh}}
	if level3 != 0 {
		combos = append(combos, []uint32{level3}, []uint32{lfsh, level3})
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
		for _, held := range combos {
			for _, c := range held {
				h.keyCode(c, true)
			}
			// What the press types and whether level 3 is active, from
			// libxkbcommon: real modifier Mod5 is bit 7 in every keymap.
			// Shift+RALT is not level 3 on every layout (de(T3)).
			sym := h.server.Sym(code)
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
			where := fmt.Sprintf("%s key %d held %v", name, code, held)
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
			// The X11 path, on the core keymap Xorg derives from this
			// layout, reports what the Wayland path does, on every key the
			// core keymap can describe (coreRepresentable).
			// Level 5 (Mod3, de(T3), neo) is beyond the X11 path too.
			if !coreRepresentable(h.seat.keymap.Levels(code)) || (d|l|lk)&x11Mod3 != 0 {
				f.x11Skipped++
				continue
			}
			// libxkbcommon's real modifiers are the X11 core state bits.
			x, xc := x11PressCode(core, code, uint16(d|l|lk)&0xff)
			if x.KeyCode != r.down.KeyCode || x.TypedKey != r.down.TypedKey ||
				x.Modifiers != r.down.Modifiers || x.TypedModifiers != r.down.TypedModifiers || xc != r.char {
				f.add("x11 agrees with wayland",
					"%s: x11 KeyCode %d TypedKey %d mods %#x/%#x char %q; wayland %d %d %#x/%#x %q",
					where, x.KeyCode, x.TypedKey, x.Modifiers, x.TypedModifiers, xc,
					r.down.KeyCode, r.down.TypedKey, r.down.Modifiers, r.down.TypedModifiers, r.char)
			}
		}
	}
}
