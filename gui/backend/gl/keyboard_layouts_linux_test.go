//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
	"github.com/go-gui-org/go-gui/gui/backend/internal/xkb"
)

// The tests in this file check docs/specs/keyboard-layouts.md (#1188):
// the key codes, modifiers and text a key press produces on real
// layouts. Keymaps are compiled from layout names with the system's
// xkeyboard-config at run time; nothing from it is checked in.

// requireXKB skips unless libxkbcommon and its layout data are present.
// GOGUI_REQUIRE_XKB=1 (set in CI) turns the skip into a failure.
func requireXKB(t *testing.T) {
	t.Helper()
	err := xkb.Load()
	if err == nil {
		var km *xkb.Keymap
		km, err = xkb.NewKeymapFromNames("evdev", "pc105", "us", "", "")
		km.Destroy()
	}
	if err == nil {
		return
	}
	if os.Getenv("GOGUI_REQUIRE_XKB") == "1" {
		t.Fatalf("GOGUI_REQUIRE_XKB is set but libxkbcommon is not usable: %v", err)
	}
	t.Skip(err)
}

// kbHarness drives the Wayland key path the way a compositor does: the
// compositor's own keymap state takes each key, then the seat gets
// wl_keyboard.key and wl_keyboard.modifiers. It records the key and
// character events the window receives.
type kbHarness struct {
	t      *testing.T
	server *xkb.Keymap // the compositor's state
	w      *gui.Window
	seat   *wlSeat
	events []gui.Event
}

func newKbHarness(t *testing.T, layout, variant string) *kbHarness {
	t.Helper()
	compile := func() *xkb.Keymap {
		km, err := xkb.NewKeymapFromNames("evdev", "pc105", layout, variant, "")
		if err != nil {
			t.Fatal(err)
		}
		return km
	}
	server, client := compile(), compile()
	t.Cleanup(func() {
		server.Destroy()
		client.Destroy()
	})
	h := &kbHarness{t: t, server: server}
	w := gui.NewWindow(gui.WindowCfg{
		State: new(int), Width: 100, Height: 100,
		OnEvent: func(e *gui.Event, _ *gui.Window) {
			switch e.Type {
			case gui.EventKeyDown, gui.EventKeyUp, gui.EventChar:
				h.events = append(h.events, *e)
			}
		},
	})
	w.SetView(func(_ *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{})
	})
	w.FrameFn()
	b := &Backend{}
	b.plat.w = w
	h.w = w
	h.seat = &wlSeat{keymap: client, kbFocus: b}
	return h
}

// code returns the XKB keycode of an XKB key name.
func (h *kbHarness) code(name string) uint32 {
	h.t.Helper()
	c := h.server.KeyByName(name)
	if c == 0 {
		h.t.Fatalf("keymap has no key <%s>", name)
	}
	return c
}

// keyCode sends one key event for an XKB keycode, then the modifiers,
// in the order a compositor sends them.
func (h *kbHarness) keyCode(code uint32, down bool) {
	h.server.UpdateKey(code, down)
	state := uint32(wl.KeyboardKeyStateReleased)
	if down {
		state = wl.KeyboardKeyStatePressed
	}
	h.seat.keyboardKey(0, 0, code-8, state)
	d, l, k, g := h.server.Serialize()
	h.seat.keyboardModifiers(0, d, l, k, g)
}

// setLayout locks the active layout, as a layout switch does, and clears
// every modifier, latched and locked ones included.
func (h *kbHarness) setLayout(idx uint32) {
	h.server.UpdateMask(0, 0, 0, idx)
	d, l, k, g := h.server.Serialize()
	h.seat.keyboardModifiers(0, d, l, k, g)
}

// pressResult is what the last key of a press produced.
type pressResult struct {
	down, up gui.Event
	char     rune
	gotDown  bool
	gotUp    bool
}

// press holds keys[:len-1], taps the last key, and releases the held
// ones in reverse. It returns what the last key produced. An open
// dead-key sequence is dropped afterwards so presses stay independent.
func (h *kbHarness) press(keys ...string) pressResult {
	h.t.Helper()
	codes := make([]uint32, len(keys))
	for i, k := range keys {
		codes[i] = h.code(k)
	}
	return h.pressCodes(codes...)
}

func (h *kbHarness) pressCodes(codes ...uint32) pressResult {
	last := len(codes) - 1
	for _, c := range codes[:last] {
		h.keyCode(c, true)
	}
	h.events = h.events[:0]
	h.keyCode(codes[last], true)
	h.keyCode(codes[last], false)
	var r pressResult
	for _, e := range h.events {
		switch e.Type {
		case gui.EventKeyDown:
			r.down, r.gotDown = e, true
		case gui.EventKeyUp:
			r.up, r.gotUp = e, true
		case gui.EventChar:
			r.char = rune(e.CharCode)
		}
	}
	for _, c := range slices.Backward(codes[:last]) {
		h.keyCode(c, false)
	}
	h.seat.kbFocus.plat.compose.reset()
	return r
}

// tapLock toggles a lock key (NMLK, CAPS).
func (h *kbHarness) tapLock(name string) {
	c := h.code(name)
	h.keyCode(c, true)
	h.keyCode(c, false)
}

// goldenRow is one row of the spec's golden tables. typedKey 0 means the
// press typed the same key: TypedKey and TypedModifiers then equal
// KeyCode and Modifiers.
type goldenRow struct {
	keys      []string
	numLock   bool
	code      gui.KeyCode
	mods      gui.Modifier
	char      rune
	typedKey  gui.KeyCode
	typedMods gui.Modifier
}

func kbKeys(keys ...string) []string { return keys }

// runGolden checks a golden table on both Linux paths: Wayland, through the
// seat a compositor drives, and X11, through the core keymap Xorg derives
// from the same layout (keyboard_x11_linux_test.go).
func runGolden(t *testing.T, layout, variant string, layoutIdx uint32, rows []goldenRow) {
	t.Helper()
	requireXKB(t)
	t.Run("wayland", func(t *testing.T) { runGoldenWayland(t, layout, variant, layoutIdx, rows) })
	t.Run("x11-xkb", func(t *testing.T) { runGoldenX11(t, layout, variant, layoutIdx, rows, xkbPlatformFor) })
	// The core keymap holds two groups, and levels 3 and 4 of the first only.
	if layoutIdx < 2 {
		core := rows
		if layoutIdx > 0 {
			core = slices.DeleteFunc(slices.Clone(rows), func(r goldenRow) bool { return slices.Contains(r.keys, "RALT") })
		}
		t.Run("x11-core", func(t *testing.T) { runGoldenX11(t, layout, variant, layoutIdx, core, coreKeymapFor) })
	}
}

// checkGoldenRow compares what a press reported with its row.
func checkGoldenRow(t *testing.T, name string, row goldenRow, down gui.Event, char rune) {
	t.Helper()
	if down.KeyCode != row.code || down.Modifiers != row.mods || char != row.char {
		t.Errorf("%s: got KeyCode %d Modifiers %#x char %q; want %d %#x %q",
			name, down.KeyCode, down.Modifiers, char, row.code, row.mods, row.char)
	}
	wantKey, wantMods := row.typedKey, row.typedMods
	if wantKey == gui.KeyInvalid {
		wantKey, wantMods = row.code, row.mods
	}
	if down.TypedKey != wantKey || down.TypedModifiers != wantMods {
		t.Errorf("%s: got TypedKey %d TypedModifiers %#x; want %d %#x",
			name, down.TypedKey, down.TypedModifiers, wantKey, wantMods)
	}
}

func goldenRowName(row goldenRow) string {
	name := strings.Join(row.keys, "+")
	if row.numLock {
		name += " (NumLock)"
	}
	return name
}

func runGoldenWayland(t *testing.T, layout, variant string, layoutIdx uint32, rows []goldenRow) {
	h := newKbHarness(t, layout, variant)
	if layoutIdx > 0 {
		h.setLayout(layoutIdx)
	}
	numLock := false
	for _, row := range rows {
		if row.numLock != numLock {
			h.tapLock("NMLK")
			numLock = row.numLock
		}
		name := goldenRowName(row)
		r := h.press(row.keys...)
		if slices.Contains(row.keys, "NMLK") {
			numLock = !numLock // the row itself toggled it
		}
		if !r.gotDown {
			t.Errorf("%s: no key down reached the window", name)
			continue
		}
		checkGoldenRow(t, name, row, r.down, r.char)
		if r.gotUp && (r.up.KeyCode != r.down.KeyCode || r.up.TypedKey != r.down.TypedKey) {
			t.Errorf("%s: key up KeyCode %d TypedKey %d, key down %d %d",
				name, r.up.KeyCode, r.up.TypedKey, r.down.KeyCode, r.down.TypedKey)
		}
	}
}

// NumLock state of a golden row.
const (
	kbNumLockOn  = true
	kbNumLockOff = false
)

// The reporter's layout (#1188).
func TestKeyboardGoldenDeNodeadkeys(t *testing.T) {
	runGolden(t, "de", "nodeadkeys", 0, []goldenRow{
		{kbKeys("AE07"), kbNumLockOff, gui.Key7, gui.ModNone, '7', 0, 0},
		{kbKeys("LFSH", "AE07"), kbNumLockOff, gui.Key7, gui.ModShift, '/', gui.KeySlash, gui.ModNone},
		{kbKeys("LCTL", "LFSH", "AE07"), kbNumLockOff, gui.Key7, gui.ModCtrl | gui.ModShift, 0, gui.KeySlash, gui.ModCtrl},
		{kbKeys("RALT", "AE08"), kbNumLockOff, gui.Key8, gui.ModAltGr, '[', gui.KeyLeftBracket, gui.ModNone},
		{kbKeys("RALT", "AD01"), kbNumLockOff, gui.KeyQ, gui.ModAltGr, '@', gui.KeyAt, gui.ModNone},
		{kbKeys("RALT", "AE11"), kbNumLockOff, gui.KeyInvalid, gui.ModAltGr, '\\', gui.KeyBackslash, gui.ModNone},
		{kbKeys("BKSL"), kbNumLockOff, gui.KeyNumberSign, gui.ModNone, '#', 0, 0},
		{kbKeys("LFSH", "BKSL"), kbNumLockOff, gui.KeyNumberSign, gui.ModShift, '\'', gui.KeyApostrophe, gui.ModNone},
		{kbKeys("LFSH", "AB08"), kbNumLockOff, gui.KeyComma, gui.ModShift, ';', gui.KeySemicolon, gui.ModNone},
		{kbKeys("LFSH", "AE10"), kbNumLockOff, gui.Key0, gui.ModShift, '=', gui.KeyEqual, gui.ModNone},
		{kbKeys("LFSH", "AE12"), kbNumLockOff, gui.KeyInvalid, gui.ModShift, '`', gui.KeyGraveAccent, gui.ModNone},
		{kbKeys("AD12"), kbNumLockOff, gui.KeyPlus, gui.ModNone, '+', 0, 0},
		{kbKeys("TLDE"), kbNumLockOff, gui.KeyCircumflex, gui.ModNone, '^', 0, 0},
		{kbKeys("LSGT"), kbNumLockOff, gui.KeyLess, gui.ModNone, '<', 0, 0},
		{kbKeys("AC11"), kbNumLockOff, gui.KeyInvalid, gui.ModNone, 'ä', 0, 0},
		{kbKeys("NMLK"), kbNumLockOff, gui.KeyNumLock, gui.ModNone, 0, 0, 0},
		{kbKeys("RALT"), kbNumLockOff, gui.KeyRightAlt, gui.ModNone, 0, 0, 0},
		{kbKeys("COMP"), kbNumLockOff, gui.KeyMenu, gui.ModNone, 0, 0, 0},
		{kbKeys("KP7"), kbNumLockOff, gui.KeyHome, gui.ModNone, 0, 0, 0},
		{kbKeys("KPDL"), kbNumLockOff, gui.KeyDelete, gui.ModNone, 0, 0, 0},
		{kbKeys("KP7"), kbNumLockOn, gui.KeyKP7, gui.ModNone, '7', 0, 0},
		{kbKeys("KPDL"), kbNumLockOn, gui.KeyKPDecimal, gui.ModNone, ',', 0, 0},
		{kbKeys("KPDV"), kbNumLockOn, gui.KeyKPDivide, gui.ModNone, '/', 0, 0},
		{kbKeys("KPEN"), kbNumLockOn, gui.KeyEnter, gui.ModNone, 0, 0, 0},
	})
}

// German with dead keys: dead_circumflex gives KeyCircumflex (rule 7).
func TestKeyboardGoldenDe(t *testing.T) {
	runGolden(t, "de", "", 0, []goldenRow{
		{kbKeys("TLDE"), kbNumLockOff, gui.KeyCircumflex, gui.ModNone, 0, 0, 0},
		{kbKeys("AE12"), kbNumLockOff, gui.KeyInvalid, gui.ModNone, 0, 0, 0},
		{kbKeys("LFSH", "AE07"), kbNumLockOff, gui.Key7, gui.ModShift, '/', gui.KeySlash, gui.ModNone},
	})
}

// AZERTY: the number row types symbols without Shift (rule 4).
func TestKeyboardGoldenFr(t *testing.T) {
	runGolden(t, "fr", "", 0, []goldenRow{
		{kbKeys("AE01"), kbNumLockOff, gui.Key1, gui.ModNone, '&', gui.KeyAmpersand, gui.ModNone},
		{kbKeys("LFSH", "AE01"), kbNumLockOff, gui.Key1, gui.ModShift, '1', 0, 0},
		{kbKeys("LCTL", "AE01"), kbNumLockOff, gui.Key1, gui.ModCtrl, 0, gui.KeyAmpersand, gui.ModCtrl},
		{kbKeys("AE08"), kbNumLockOff, gui.Key8, gui.ModNone, '_', gui.KeyUnderscore, gui.ModNone},
		{kbKeys("RALT", "AE10"), kbNumLockOff, gui.Key0, gui.ModAltGr, '@', gui.KeyAt, gui.ModNone},
		{kbKeys("AD01"), kbNumLockOff, gui.KeyA, gui.ModNone, 'a', 0, 0},
		{kbKeys("LFSH", "AB09"), kbNumLockOff, gui.KeyColon, gui.ModShift, '/', gui.KeySlash, gui.ModNone},
	})
}

// Non-Latin: Ctrl+C on the key US labels C (rule 5).
func TestKeyboardGoldenRu(t *testing.T) {
	runGolden(t, "ru", "", 0, []goldenRow{
		{kbKeys("LCTL", "AB03"), kbNumLockOff, gui.KeyC, gui.ModCtrl, 0, 0, 0},
		{kbKeys("AB03"), kbNumLockOff, gui.KeyC, gui.ModNone, 'с', 0, 0},
		{kbKeys("LFSH", "AE07"), kbNumLockOff, gui.Key7, gui.ModShift, '?', gui.KeyQuestion, gui.ModNone},
		{kbKeys("LSGT"), kbNumLockOff, gui.KeySlash, gui.ModNone, '/', 0, 0},
		{kbKeys("RALT"), kbNumLockOff, gui.KeyRightAlt, gui.ModNone, 0, 0, 0},
		// Right Alt is plain Alt here, and Alt+key types (only Ctrl and
		// Super make a press a shortcut, #896).
		{kbKeys("RALT", "AB03"), kbNumLockOff, gui.KeyC, gui.ModAlt, 'с', 0, 0},
	})
}

// us,ru with Russian active: the configured Latin layout gives the code.
func TestKeyboardGoldenUsRu(t *testing.T) {
	runGolden(t, "us,ru", "", 1, []goldenRow{
		{kbKeys("LCTL", "AB03"), kbNumLockOff, gui.KeyC, gui.ModCtrl, 0, 0, 0},
		{kbKeys("AB03"), kbNumLockOff, gui.KeyC, gui.ModNone, 'с', 0, 0},
	})
}

// de,ru,fr: the second and the third layout active. On X11 a third layout
// is readable only from the XKB map (the core keymap holds two groups).
func TestKeyboardGoldenDeRuFr(t *testing.T) {
	t.Run("ru", func(t *testing.T) {
		runGolden(t, "de,ru,fr", "", 1, []goldenRow{
			{kbKeys("AB03"), kbNumLockOff, gui.KeyC, gui.ModNone, 'с', 0, 0},
			{kbKeys("LCTL", "AB03"), kbNumLockOff, gui.KeyC, gui.ModCtrl, 0, 0, 0},
		})
	})
	t.Run("fr", func(t *testing.T) {
		runGolden(t, "de,ru,fr", "", 2, []goldenRow{
			{kbKeys("AB03"), kbNumLockOff, gui.KeyC, gui.ModNone, 'c', 0, 0},
			{kbKeys("AE01"), kbNumLockOff, gui.Key1, gui.ModNone, '&', gui.KeyAmpersand, gui.ModNone},
			{kbKeys("LFSH", "AE01"), kbNumLockOff, gui.Key1, gui.ModShift, '1', 0, 0},
			{kbKeys("RALT", "AE10"), kbNumLockOff, gui.Key0, gui.ModAltGr, '@', gui.KeyAt, gui.ModNone},
			{kbKeys("AD01"), kbNumLockOff, gui.KeyA, gui.ModNone, 'a', 0, 0},
		})
	})
}

// us,de,ru,fr: four live groups, each active in turn. Russian takes the
// first Latin layout's keys (D6), here US.
func TestKeyboardGoldenFourGroups(t *testing.T) {
	for _, g := range []struct {
		name string
		idx  uint32
		rows []goldenRow
	}{
		{"us", 0, []goldenRow{
			{kbKeys("AD06"), kbNumLockOff, gui.KeyY, gui.ModNone, 'y', 0, 0},
			{kbKeys("LFSH", "AE07"), kbNumLockOff, gui.Key7, gui.ModShift, '&', gui.KeyAmpersand, gui.ModNone},
		}},
		{"de", 1, []goldenRow{
			{kbKeys("AD06"), kbNumLockOff, gui.KeyZ, gui.ModNone, 'z', 0, 0},
			{kbKeys("LFSH", "AE07"), kbNumLockOff, gui.Key7, gui.ModShift, '/', gui.KeySlash, gui.ModNone},
			{kbKeys("RALT", "AD01"), kbNumLockOff, gui.KeyQ, gui.ModAltGr, '@', gui.KeyAt, gui.ModNone},
		}},
		{"ru", 2, []goldenRow{
			{kbKeys("AD06"), kbNumLockOff, gui.KeyY, gui.ModNone, 'н', 0, 0},
			{kbKeys("LCTL", "AB03"), kbNumLockOff, gui.KeyC, gui.ModCtrl, 0, 0, 0},
		}},
		{"fr", 3, []goldenRow{
			{kbKeys("AE01"), kbNumLockOff, gui.Key1, gui.ModNone, '&', gui.KeyAmpersand, gui.ModNone},
			{kbKeys("AD01"), kbNumLockOff, gui.KeyA, gui.ModNone, 'a', 0, 0},
		}},
	} {
		t.Run(g.name, func(t *testing.T) { runGolden(t, "us,de,ru,fr", "", g.idx, g.rows) })
	}
}

// gr: a second non-Latin script. Shortcuts take the US position.
func TestKeyboardGoldenGr(t *testing.T) {
	runGolden(t, "gr", "", 0, []goldenRow{
		{kbKeys("AB03"), kbNumLockOff, gui.KeyC, gui.ModNone, 'ψ', 0, 0},
		{kbKeys("LCTL", "AB03"), kbNumLockOff, gui.KeyC, gui.ModCtrl, 0, 0, 0},
		{kbKeys("LFSH", "AB03"), kbNumLockOff, gui.KeyC, gui.ModShift, 'Ψ', 0, 0},
	})
}

// pl: AltGr types the national letters on Latin keys, which keep their
// KeyCode; the letter itself has none, so the typed key is the same.
func TestKeyboardGoldenPl(t *testing.T) {
	runGolden(t, "pl", "", 0, []goldenRow{
		{kbKeys("RALT", "AC01"), kbNumLockOff, gui.KeyA, gui.ModAltGr, 'ą', 0, 0},
		{kbKeys("LFSH", "RALT", "AC01"), kbNumLockOff, gui.KeyA, gui.ModShift | gui.ModAltGr, 'Ą', 0, 0},
		{kbKeys("RALT", "AB01"), kbNumLockOff, gui.KeyZ, gui.ModAltGr, 'ż', 0, 0},
		{kbKeys("LCTL", "AC01"), kbNumLockOff, gui.KeyA, gui.ModCtrl, 0, 0, 0},
	})
}

// ru,us and ru,us(dvorak) with Russian active: the first configured Latin
// layout may come after the active one (D6). Dvorak's J there proves the
// later layout is read, not the US position.
func TestKeyboardGoldenRuThenLatin(t *testing.T) {
	t.Run("ru,us", func(t *testing.T) {
		runGolden(t, "ru,us", "", 0, []goldenRow{
			{kbKeys("LCTL", "AB03"), kbNumLockOff, gui.KeyC, gui.ModCtrl, 0, 0, 0},
			{kbKeys("AB03"), kbNumLockOff, gui.KeyC, gui.ModNone, 'с', 0, 0},
		})
	})
	t.Run("ru,us(dvorak)", func(t *testing.T) {
		runGolden(t, "ru,us", ",dvorak", 0, []goldenRow{
			{kbKeys("LCTL", "AB03"), kbNumLockOff, gui.KeyJ, gui.ModCtrl, 0, 0, 0},
		})
	})
}

// Dvorak,ru with Russian active: Dvorak wins over the US position (D6).
func TestKeyboardGoldenDvorakRu(t *testing.T) {
	runGolden(t, "us,ru", "dvorak,", 1, []goldenRow{
		{kbKeys("LCTL", "AB03"), kbNumLockOff, gui.KeyJ, gui.ModCtrl, 0, 0, 0},
	})
}

// Letters follow the layout, not the position.
func TestKeyboardGoldenDvorak(t *testing.T) {
	runGolden(t, "us", "dvorak", 0, []goldenRow{
		{kbKeys("AB03"), kbNumLockOff, gui.KeyJ, gui.ModNone, 'j', 0, 0},
		{kbKeys("AD01"), kbNumLockOff, gui.KeyApostrophe, gui.ModNone, '\'', 0, 0},
	})
}

func TestKeyboardGoldenCh(t *testing.T) {
	runGolden(t, "ch", "", 0, []goldenRow{
		{kbKeys("LFSH", "AE01"), kbNumLockOff, gui.Key1, gui.ModShift, '+', gui.KeyPlus, gui.ModNone},
		{kbKeys("RALT", "AD11"), kbNumLockOff, gui.KeyInvalid, gui.ModAltGr, '[', gui.KeyLeftBracket, gui.ModNone},
		{kbKeys("BKSL"), kbNumLockOff, gui.KeyDollar, gui.ModNone, '$', 0, 0},
	})
}

// ABNT2: the extra / key, and KP_Separator on the numpad.
func TestKeyboardGoldenBr(t *testing.T) {
	runGolden(t, "br", "", 0, []goldenRow{
		{kbKeys("AB11"), kbNumLockOff, gui.KeySlash, gui.ModNone, '/', 0, 0},
		{kbKeys("KPDL"), kbNumLockOn, gui.KeyKPDecimal, gui.ModNone, ',', 0, 0},
	})
}

// Two keys type \ on Japanese; both get KeyBackslash.
func TestKeyboardGoldenJp(t *testing.T) {
	runGolden(t, "jp", "", 0, []goldenRow{
		{kbKeys("AE13"), kbNumLockOff, gui.KeyBackslash, gui.ModNone, '\\', 0, 0},
		{kbKeys("AB11"), kbNumLockOff, gui.KeyBackslash, gui.ModNone, '\\', 0, 0},
		{kbKeys("AD11"), kbNumLockOff, gui.KeyAt, gui.ModNone, '@', 0, 0},
	})
}

// TestKeyboardShortcutsFire runs the spec's "Fires" column end to end:
// real key presses through the Wayland path into a window's commands.
func TestKeyboardShortcutsFire(t *testing.T) {
	requireXKB(t)
	cases := []struct {
		name            string
		layout, variant string
		shortcuts       map[string]gui.Shortcut
		keys            []string
		want            string // command ID that runs, "" for none
	}{
		{"de Shift+7 is /", "de", "nodeadkeys",
			map[string]gui.Shortcut{"slash": {Key: gui.KeySlash}},
			kbKeys("LFSH", "AE07"), "slash"},
		{"de Ctrl+Shift+7 is Ctrl+/", "de", "nodeadkeys",
			map[string]gui.Shortcut{"slash": {Key: gui.KeySlash, Modifiers: gui.ModCtrl}},
			kbKeys("LCTL", "LFSH", "AE07"), "slash"},
		{"de Shift+7 bound both ways runs the exact one", "de", "nodeadkeys",
			map[string]gui.Shortcut{
				"shift7": {Key: gui.Key7, Modifiers: gui.ModShift},
				"slash":  {Key: gui.KeySlash},
			},
			kbKeys("LFSH", "AE07"), "shift7"},
		{"de AltGr+8 is not 8", "de", "nodeadkeys",
			map[string]gui.Shortcut{"eight": {Key: gui.Key8}},
			kbKeys("RALT", "AE08"), ""},
		{"de AltGr+8 is [", "de", "nodeadkeys",
			map[string]gui.Shortcut{"bracket": {Key: gui.KeyLeftBracket}},
			kbKeys("RALT", "AE08"), "bracket"},
		{"de Shift+# is '", "de", "nodeadkeys",
			map[string]gui.Shortcut{"apostrophe": {Key: gui.KeyApostrophe}},
			kbKeys("LFSH", "BKSL"), "apostrophe"},
		{"de # is #", "de", "nodeadkeys",
			map[string]gui.Shortcut{"hash": {Key: gui.KeyNumberSign}},
			kbKeys("BKSL"), "hash"},
		{"fr Ctrl+1 on the & key", "fr", "",
			map[string]gui.Shortcut{"one": {Key: gui.Key1, Modifiers: gui.ModCtrl}},
			kbKeys("LCTL", "AE01"), "one"},
		{"ru Ctrl+C", "ru", "",
			map[string]gui.Shortcut{"copy": {Key: gui.KeyC, Modifiers: gui.ModCtrl}},
			kbKeys("LCTL", "AB03"), "copy"},
		{"us Ctrl+Shift+A is not Ctrl+A", "us", "",
			map[string]gui.Shortcut{"all": {Key: gui.KeyA, Modifiers: gui.ModCtrl}},
			kbKeys("LCTL", "LFSH", "AC01"), ""},
		{"us Ctrl+Shift+= runs the exact one over Ctrl++", "us", "",
			map[string]gui.Shortcut{
				"exact": {Key: gui.KeyEqual, Modifiers: gui.ModCtrlShift},
				"plus":  {Key: gui.KeyPlus, Modifiers: gui.ModCtrl},
			},
			kbKeys("LCTL", "LFSH", "AE12"), "exact"},
		{"us Ctrl+Shift+= is Ctrl++", "us", "",
			map[string]gui.Shortcut{"plus": {Key: gui.KeyPlus, Modifiers: gui.ModCtrl}},
			kbKeys("LCTL", "LFSH", "AE12"), "plus"},
	}
	for _, c := range cases {
		h := newKbHarness(t, c.layout, c.variant)
		fired := ""
		for id, s := range c.shortcuts {
			if err := h.w.RegisterCommand(gui.Command{
				ID: id, Shortcut: s, Global: true,
				Execute: func(_ *gui.Event, _ *gui.Window) { fired = id },
			}); err != nil {
				t.Fatal(err)
			}
		}
		h.press(c.keys...)
		if fired != c.want {
			t.Errorf("%s: ran %q, want %q", c.name, fired, c.want)
		}
	}
}

// TestKeyboardHeldKeyKeepsCode: a repeat and the release report the
// KeyCode of the press, even when NumLock or the layout changed while the
// key was held — also when that KeyCode is KeyInvalid.
func TestKeyboardHeldKeyKeepsCode(t *testing.T) {
	requireXKB(t)
	cases := []struct {
		name    string
		layout  string
		start   uint32 // layout index at the press
		key     string
		change  func(h *kbHarness)
		pressed gui.KeyCode
	}{
		{"NumLock on while KP7 held", "us", 0, "KP7",
			func(h *kbHarness) { h.tapLock("NMLK") }, gui.KeyHome},
		{"us to ru while ; held", "us,ru", 0, "AC10",
			func(h *kbHarness) { h.setLayout(1) }, gui.KeySemicolon},
		{"ru to us while ж held", "us,ru", 1, "AC10",
			func(h *kbHarness) { h.setLayout(0) }, gui.KeyInvalid},
	}
	for _, c := range cases {
		h := newKbHarness(t, c.layout, "")
		h.setLayout(c.start)
		code := h.code(c.key)
		var got []gui.KeyCode
		collect := func() {
			for _, e := range h.events {
				if e.Type == gui.EventKeyDown || e.Type == gui.EventKeyUp {
					got = append(got, e.KeyCode)
				}
			}
		}
		h.events = h.events[:0]
		h.keyCode(code, true)
		collect()
		c.change(h) // its own key events (NumLock) are not the held key's
		h.events = h.events[:0]
		h.seat.keyPress(code, true) // what the repeat timer does
		h.keyCode(code, false)
		collect()
		want := []gui.KeyCode{c.pressed, c.pressed, c.pressed}
		if !slices.Equal(got, want) {
			t.Errorf("%s: down, repeat, up = %v, want %v", c.name, got, want)
		}
	}
}

// TestKeyboardFocusLossForgetsPress: a key held while the window loses
// focus may come back held (wl_keyboard.enter sends no press) and be
// released then; the release must not report the press from before.
func TestKeyboardFocusLossForgetsPress(t *testing.T) {
	requireXKB(t)
	h := newKbHarness(t, "us,ru", "")
	code := h.code("AC10") // ; on us, ж on ru
	b := h.seat.kbFocus
	h.keyCode(code, true)
	h.seat.keyboardLeave(0, wl.Surface{})
	h.setLayout(1)
	h.seat.kbFocus = b // what keyboardEnter does for our surface
	h.events = h.events[:0]
	h.keyCode(code, false)
	if len(h.events) != 1 || h.events[0].Type != gui.EventKeyUp {
		t.Fatalf("got events %v, want one key up", h.events)
	}
	if got := h.events[0].KeyCode; got != gui.KeyInvalid {
		t.Errorf("release after focus loss reported KeyCode %d from the old press, want KeyInvalid (ж)", got)
	}
}

// legacyUSKeyCode is x11key.MapKeySym as it was before #1188, frozen here
// so the US regression guard keeps comparing against it once MapKeySym
// changes.
//
//nolint:gocyclo // frozen copy of a key-mapping switch
func legacyUSKeyCode(sym uint32) gui.KeyCode {
	switch {
	case sym >= 'a' && sym <= 'z':
		return gui.KeyCode(sym - 0x20)
	case sym >= 'A' && sym <= 'Z', sym >= '0' && sym <= '9':
		return gui.KeyCode(sym)
	case sym >= 0xffbe && sym <= 0xffc9: // F1-F12
		return gui.KeyF1 + gui.KeyCode(sym-0xffbe)
	}
	switch sym {
	case ' ':
		return gui.KeySpace
	case 0xff0d, 0xff8d: // Return, KP_Enter
		return gui.KeyEnter
	case 0xff1b:
		return gui.KeyEscape
	case 0xff09:
		return gui.KeyTab
	case 0xff08:
		return gui.KeyBackspace
	case 0xffff:
		return gui.KeyDelete
	case 0xff63:
		return gui.KeyInsert
	case 0xff53:
		return gui.KeyRight
	case 0xff51:
		return gui.KeyLeft
	case 0xff54:
		return gui.KeyDown
	case 0xff52:
		return gui.KeyUp
	case 0xff55:
		return gui.KeyPageUp
	case 0xff56:
		return gui.KeyPageDown
	case 0xff50:
		return gui.KeyHome
	case 0xff57:
		return gui.KeyEnd
	case 0xffe1:
		return gui.KeyLeftShift
	case 0xffe2:
		return gui.KeyRightShift
	case 0xffe3:
		return gui.KeyLeftControl
	case 0xffe4:
		return gui.KeyRightControl
	case 0xffe9:
		return gui.KeyLeftAlt
	case 0xffea:
		return gui.KeyRightAlt
	case 0xffeb:
		return gui.KeyLeftSuper
	case 0xffec:
		return gui.KeyRightSuper
	case 0xffe5:
		return gui.KeyCapsLock
	case ',':
		return gui.KeyComma
	case '-':
		return gui.KeyMinus
	case '.':
		return gui.KeyPeriod
	case '/':
		return gui.KeySlash
	case ';':
		return gui.KeySemicolon
	case '=':
		return gui.KeyEqual
	case '[':
		return gui.KeyLeftBracket
	case '\\':
		return gui.KeyBackslash
	case ']':
		return gui.KeyRightBracket
	case '`':
		return gui.KeyGraveAccent
	}
	return gui.KeyInvalid
}

// newASCIIKeys are the KeyCode constants #1188 added (spec § 3).
var newASCIIKeys = map[gui.KeyCode]bool{
	gui.KeyExclamation: true, gui.KeyQuotation: true, gui.KeyNumberSign: true,
	gui.KeyDollar: true, gui.KeyPercent: true, gui.KeyAmpersand: true,
	gui.KeyLeftParen: true, gui.KeyRightParen: true, gui.KeyAsterisk: true,
	gui.KeyPlus: true, gui.KeyColon: true, gui.KeyLess: true,
	gui.KeyGreater: true, gui.KeyQuestion: true, gui.KeyAt: true,
	gui.KeyCircumflex: true, gui.KeyUnderscore: true, gui.KeyLeftBrace: true,
	gui.KeyBar: true, gui.KeyRightBrace: true, gui.KeyTilde: true,
}

// usPositions are keymaps with the US layout active in each group position
// and in both orders next to another layout (#1188): the US regression
// guard holds on each, not only on us alone.
var usPositions = []struct {
	layouts string
	group   uint32
}{
	{"us", 0}, {"us,de", 0}, {"de,us", 1}, {"ru,us", 1}, {"de,ru,us", 2}, {"de,ru,fr,us", 3},
}

// TestKeyboardUSRegression is the spec's US regression guard: with the US
// layout active, in any group position, every key that had a KeyCode on us
// alone keeps it, with the same Modifiers, under every combination of
// Shift, Ctrl, Alt and Super.
func TestKeyboardUSRegression(t *testing.T) {
	requireXKB(t)
	us, err := xkb.NewKeymapFromNames("evdev", "pc105", "us", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer us.Destroy()
	for _, pos := range usPositions {
		t.Run(pos.layouts, func(t *testing.T) { usRegressionWayland(t, us, pos.layouts, pos.group) })
	}
}

func usRegressionWayland(t *testing.T, us *xkb.Keymap, layouts string, group uint32) {
	h := newKbHarness(t, layouts, "")
	h.setLayout(group)
	held := []struct {
		keys []string
		mods gui.Modifier
	}{
		{nil, gui.ModNone},
		{kbKeys("LFSH"), gui.ModShift},
		{kbKeys("LCTL"), gui.ModCtrl},
		{kbKeys("LCTL", "LFSH"), gui.ModCtrl | gui.ModShift},
		{kbKeys("LALT"), gui.ModAlt},
		{kbKeys("LWIN"), gui.ModSuper},
	}
	// Lock keys change the state for the keys after them, and the window
	// itself takes Tab (focus) and F12 (inspector).
	skip := map[uint32]bool{h.code("CAPS"): true, h.code("NMLK"): true}
	checked := 0
	for code := uint32(9); code < 256; code++ {
		if skip[code] {
			continue
		}
		want := legacyUSKeyCode(us.BaseSym(code))
		if want == gui.KeyInvalid || want == gui.KeyTab || want == gui.KeyF12 {
			continue
		}
		for _, hd := range held {
			codes := make([]uint32, 0, len(hd.keys)+1)
			for _, n := range hd.keys {
				codes = append(codes, h.code(n))
			}
			if slices.Contains(codes, code) {
				continue // the key is one of the held modifiers
			}
			r := h.pressCodes(append(codes, code)...)
			checked++
			if !r.gotDown {
				t.Errorf("key %d with %v: no key down reached the window", code, hd.keys)
				continue
			}
			if r.down.KeyCode != want || r.down.Modifiers != hd.mods {
				t.Errorf("key %d with %v: got KeyCode %d Modifiers %#x; want %d %#x",
					code, hd.keys, r.down.KeyCode, r.down.Modifiers, want, hd.mods)
			}
			// The typed pass can only reach a shortcut built from a
			// constant #1188 added, never one apps bound before.
			if r.down.TypedKey != r.down.KeyCode && !newASCIIKeys[r.down.TypedKey] {
				t.Errorf("key %d with %v: TypedKey %d is a key that existed before #1188",
					code, hd.keys, r.down.TypedKey)
			}
		}
	}
	if checked < 300 {
		t.Fatalf("checked only %d presses; the us keymap looks wrong", checked)
	}
}
