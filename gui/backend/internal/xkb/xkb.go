//go:build linux && !android && (amd64 || arm64)

package xkb

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// libxkbcommon entry points, resolved by Load.
var (
	fnContextNew            uintptr
	fnContextUnref          uintptr
	fnKeymapNewFromBuffer   uintptr
	fnKeymapNewFromNames    uintptr
	fnKeymapGetAsString     uintptr
	fnKeymapUnref           uintptr
	fnKeymapModGetIndex     uintptr
	fnKeymapKeyRepeats      uintptr
	fnKeymapKeyGetSymsByLvl uintptr
	fnKeymapNumLayouts      uintptr
	fnKeymapLayoutsForKey   uintptr
	fnKeymapLevelsForKey    uintptr
	fnStateNew              uintptr
	fnStateUnref            uintptr
	fnStateUpdateMask       uintptr
	fnStateKeyGetOneSym     uintptr
	fnStateKeyGetLayout     uintptr
	fnStateKeyGetConsumed   uintptr
	fnStateSerializeMods    uintptr
	fnStateSerializeLayout  uintptr
	fnStateUpdateKey        uintptr
	fnKeymapKeyByName       uintptr
	fnKeysymToUTF32         uintptr

	loadOnce sync.Once
	loadErr  error
)

// libxkbcommon constants (xkbcommon.h).
const (
	keymapFormatTextV1   = 1
	stateModsDepressed   = 1 << 0
	stateModsLatched     = 1 << 1
	stateModsLocked      = 1 << 2
	stateModsEffective   = 1 << 3
	stateLayoutEffective = 1 << 7
	contextNoEnvNames    = 1 << 1 // XKB_CONTEXT_NO_ENVIRONMENT_NAMES
	consumedModeGTK      = 1      // XKB_CONSUMED_MODE_GTK
	keyUp, keyDown       = 0, 1
	modInvalid           = 0xffffffff
	layoutInvalid        = 0xffffffff
	keycodeInvalid       = 0xffffffff
)

// Load opens libxkbcommon. It runs once; later calls return the first
// result. NewKeymap calls it.
func Load() error {
	loadOnce.Do(func() { loadErr = load() })
	return loadErr
}

func load() error {
	lib, err := purego.Dlopen("libxkbcommon.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("dlopen libxkbcommon.so.0: %w", err)
	}
	for _, s := range []struct {
		fn   *uintptr
		name string
	}{
		{&fnContextNew, "xkb_context_new"},
		{&fnContextUnref, "xkb_context_unref"},
		{&fnKeymapNewFromBuffer, "xkb_keymap_new_from_buffer"},
		{&fnKeymapNewFromNames, "xkb_keymap_new_from_names"},
		{&fnKeymapGetAsString, "xkb_keymap_get_as_string"},
		{&fnKeymapUnref, "xkb_keymap_unref"},
		{&fnKeymapModGetIndex, "xkb_keymap_mod_get_index"},
		{&fnKeymapKeyRepeats, "xkb_keymap_key_repeats"},
		{&fnKeymapKeyGetSymsByLvl, "xkb_keymap_key_get_syms_by_level"},
		{&fnKeymapNumLayouts, "xkb_keymap_num_layouts"},
		{&fnKeymapLayoutsForKey, "xkb_keymap_num_layouts_for_key"},
		{&fnKeymapLevelsForKey, "xkb_keymap_num_levels_for_key"},
		{&fnStateNew, "xkb_state_new"},
		{&fnStateUnref, "xkb_state_unref"},
		{&fnStateUpdateMask, "xkb_state_update_mask"},
		{&fnStateKeyGetOneSym, "xkb_state_key_get_one_sym"},
		{&fnStateKeyGetLayout, "xkb_state_key_get_layout"},
		{&fnStateKeyGetConsumed, "xkb_state_key_get_consumed_mods2"},
		{&fnStateSerializeMods, "xkb_state_serialize_mods"},
		{&fnStateSerializeLayout, "xkb_state_serialize_layout"},
		{&fnStateUpdateKey, "xkb_state_update_key"},
		{&fnKeymapKeyByName, "xkb_keymap_key_by_name"},
		{&fnKeysymToUTF32, "xkb_keysym_to_utf32"},
	} {
		addr, symErr := purego.Dlsym(lib, s.name)
		if symErr != nil || addr == 0 {
			return fmt.Errorf("libxkbcommon has no %s: %v", s.name, symErr)
		}
		*s.fn = addr
	}
	return nil
}

// X11 core modifier mask bits (X.h). State returns its mask in this form,
// so x11key.MapModifiers and the X11 shortcut rules read it unchanged.
const (
	MaskShift   = 1 << 0
	MaskLock    = 1 << 1
	MaskControl = 1 << 2
	MaskMod1    = 1 << 3 // Alt
	MaskMod4    = 1 << 6 // Super
	MaskMod5    = 1 << 7 // ISO level 3 (AltGr)
)

// coreMods lists, per X11 core bit, the XKB modifier names that set it. The
// virtual names (Alt, Super, LevelThree) are there for keymaps whose state
// reports the virtual modifier without its real one.
var coreMods = [...]struct {
	bit   uint16
	names []string
}{
	{MaskShift, []string{"Shift"}},
	{MaskLock, []string{"Lock"}},
	{MaskControl, []string{"Control"}},
	{MaskMod1, []string{"Mod1", "Alt"}},
	{MaskMod4, []string{"Mod4", "Super"}},
	{MaskMod5, []string{"Mod5", "LevelThree"}},
}

// Keymap is a compiled keymap and the keyboard state tracked on it.
type Keymap struct {
	ctx, keymap, state uintptr
	// scratch is a second state KeySyms sets freely, so asking what a key
	// types under other modifiers leaves the tracked state alone.
	scratch uintptr
	// coreMask[i] holds the XKB modifier bits that set coreMods[i].bit.
	coreMask [len(coreMods)]uint32
	// shiftMask and numLockMask are the XKB modifier bits of Shift and
	// NumLock; latinLayouts has bit i set when layout i types Latin letters.
	shiftMask, numLockMask, latinLayouts uint32
}

// maxKeymapSize bounds a keymap. A real one is about 60 KB; the bound
// stops a broken compositor from making libxkbcommon parse megabytes.
const maxKeymapSize = 4 << 20

// NewKeymap compiles a keymap in XKB text form, as wl_keyboard.keymap
// sends it. Trailing NUL bytes are ignored. buf is not retained.
func NewKeymap(buf []byte) (*Keymap, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	for len(buf) > 0 && buf[len(buf)-1] == 0 {
		buf = buf[:len(buf)-1]
	}
	if len(buf) == 0 {
		return nil, errors.New("xkb: empty keymap")
	}
	if len(buf) > maxKeymapSize {
		return nil, fmt.Errorf("xkb: keymap of %d bytes is too large", len(buf))
	}
	ctx, _, _ := purego.SyscallN(fnContextNew, 0)
	if ctx == 0 {
		return nil, errors.New("xkb: xkb_context_new failed")
	}
	km, _, _ := purego.SyscallN(fnKeymapNewFromBuffer, ctx,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), keymapFormatTextV1, 0)
	runtime.KeepAlive(buf)
	if km == 0 {
		purego.SyscallN(fnContextUnref, ctx)
		return nil, errors.New("xkb: keymap does not compile")
	}
	return newKeymap(ctx, km)
}

// NewKeymapFromNames compiles the keymap xkeyboard-config builds from rule
// names, as setxkbmap takes them ("evdev", "pc105", "de", "nodeadkeys",
// ""). An empty name takes libxkbcommon's default. Layouts and variants
// may be comma lists ("us,ru").
func NewKeymapFromNames(rules, model, layout, variant, options string) (*Keymap, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	strs := [...]string{rules, model, layout, variant, options}
	var bufs [len(strs)][]byte
	var names [len(strs)]uintptr // struct xkb_rule_names: five const char *
	for i, s := range strs {
		bufs[i] = append([]byte(s), 0)
		names[i] = uintptr(unsafe.Pointer(&bufs[i][0]))
	}
	// XKB_DEFAULT_* must not fill the empty names: the caller asked for
	// exactly this keymap, not the user's.
	ctx, _, _ := purego.SyscallN(fnContextNew, contextNoEnvNames)
	if ctx == 0 {
		return nil, errors.New("xkb: xkb_context_new failed")
	}
	km, _, _ := purego.SyscallN(fnKeymapNewFromNames, ctx, uintptr(unsafe.Pointer(&names[0])), 0)
	runtime.KeepAlive(bufs)
	runtime.KeepAlive(names)
	if km == 0 {
		purego.SyscallN(fnContextUnref, ctx)
		return nil, fmt.Errorf("xkb: no keymap for layout %q variant %q", layout, variant)
	}
	return newKeymap(ctx, km)
}

// KeyByName returns the XKB keycode of the key with the given XKB name
// ("AE07", "LFSH", "KP7"), or 0 when the keymap has none.
func (k *Keymap) KeyByName(name string) uint32 {
	b := make([]byte, len(name)+1)
	copy(b, name)
	r, _, _ := purego.SyscallN(fnKeymapKeyByName, k.keymap, uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
	if uint32(r) == keycodeInvalid {
		return 0
	}
	return uint32(r)
}

// UpdateKey presses (down) or releases a key in the keymap's own state,
// the way a compositor tracks the keyboard. Clients get their state from
// wl_keyboard.modifiers instead (UpdateMask).
func (k *Keymap) UpdateKey(key uint32, down bool) {
	dir := uintptr(keyUp)
	if down {
		dir = keyDown
	}
	purego.SyscallN(fnStateUpdateKey, k.state, uintptr(key), dir)
}

// Serialize returns the state as wl_keyboard.modifiers carries it: the
// depressed, latched and locked modifier masks and the effective layout.
func (k *Keymap) Serialize() (depressed, latched, locked, layout uint32) {
	mods := func(component uintptr) uint32 {
		r, _, _ := purego.SyscallN(fnStateSerializeMods, k.state, component)
		return uint32(r)
	}
	lay, _, _ := purego.SyscallN(fnStateSerializeLayout, k.state, stateLayoutEffective)
	return mods(stateModsDepressed), mods(stateModsLatched), mods(stateModsLocked), uint32(lay)
}

// newKeymap wraps a compiled keymap; it takes over ctx and km.
func newKeymap(ctx, km uintptr) (*Keymap, error) {
	st, _, _ := purego.SyscallN(fnStateNew, km)
	if st == 0 {
		purego.SyscallN(fnKeymapUnref, km)
		purego.SyscallN(fnContextUnref, ctx)
		return nil, errors.New("xkb: xkb_state_new failed")
	}
	scratch, _, _ := purego.SyscallN(fnStateNew, km)
	if scratch == 0 {
		purego.SyscallN(fnStateUnref, st)
		purego.SyscallN(fnKeymapUnref, km)
		purego.SyscallN(fnContextUnref, ctx)
		return nil, errors.New("xkb: xkb_state_new failed")
	}
	k := &Keymap{ctx: ctx, keymap: km, state: st, scratch: scratch}
	for i, m := range coreMods {
		k.coreMask[i] = k.modMask(m.names...)
	}
	k.shiftMask = k.modMask("Shift")
	k.numLockMask = k.modMask("Mod2", "NumLock")
	k.latinLayouts = k.findLatinLayouts()
	return k, nil
}

// modMask returns the XKB modifier bits of the named modifiers the keymap
// has.
func (k *Keymap) modMask(names ...string) uint32 {
	var m uint32
	for _, name := range names {
		if idx := k.modIndex(name); idx < 32 {
			m |= 1 << idx
		}
	}
	return m
}

// latinProbeKeys are the keys US labels A, B, C and D. A layout that types
// an ASCII letter on one of them is a Latin layout.
var latinProbeKeys = [...]string{"AC01", "AB05", "AB03", "AC03"}

// findLatinLayouts returns a mask with bit i set when layout i is Latin.
func (k *Keymap) findLatinLayouts() uint32 {
	n, _, _ := purego.SyscallN(fnKeymapNumLayouts, k.keymap)
	var mask uint32
	for layout := range min(uint32(n), 32) {
		for _, name := range latinProbeKeys {
			key := k.KeyByName(name)
			if key == 0 {
				continue
			}
			if s := k.levelSym(key, layout, 0); s >= 'a' && s <= 'z' || s >= 'A' && s <= 'Z' {
				mask |= 1 << layout
				break
			}
		}
	}
	return mask
}

// Levels returns, for each layout key has, the first keysym of each of its
// levels (0 for a level with none). This is the key's whole table, from
// which the core keymap an X server publishes is derived.
func (k *Keymap) Levels(key uint32) [][]uint32 {
	n, _, _ := purego.SyscallN(fnKeymapLayoutsForKey, k.keymap, uintptr(key))
	out := make([][]uint32, uint32(n))
	for layout := range out {
		lv, _, _ := purego.SyscallN(fnKeymapLevelsForKey, k.keymap, uintptr(key), uintptr(layout))
		out[layout] = make([]uint32, uint32(lv))
		for level := range out[layout] {
			out[layout][level] = k.levelSym(key, uint32(layout), uint32(level))
		}
	}
	return out
}

// levelSym is the first keysym of key on one level of one layout, 0 when
// it has none.
func (k *Keymap) levelSym(key, layout, level uint32) uint32 {
	var syms uintptr // const xkb_keysym_t *
	n, _, _ := purego.SyscallN(fnKeymapKeyGetSymsByLvl, k.keymap, uintptr(key),
		uintptr(layout), uintptr(level), uintptr(unsafe.Pointer(&syms)))
	if int32(n) <= 0 || syms == 0 {
		return 0
	}
	return *(*uint32)(cptr(syms))
}

// modIndex looks a modifier up by name; modInvalid when the keymap has none.
func (k *Keymap) modIndex(name string) uint32 {
	b := make([]byte, len(name)+1)
	copy(b, name)
	r, _, _ := purego.SyscallN(fnKeymapModGetIndex, k.keymap, uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
	return uint32(r)
}

// Destroy frees the keymap and its state. Safe to call twice.
func (k *Keymap) Destroy() {
	if k == nil || k.keymap == 0 {
		return
	}
	purego.SyscallN(fnStateUnref, k.state)
	purego.SyscallN(fnStateUnref, k.scratch)
	purego.SyscallN(fnKeymapUnref, k.keymap)
	purego.SyscallN(fnContextUnref, k.ctx)
	*k = Keymap{}
}

// UpdateMask sets the modifier and layout state the compositor reported
// in wl_keyboard.modifiers.
func (k *Keymap) UpdateMask(depressed, latched, locked, group uint32) {
	purego.SyscallN(fnStateUpdateMask, k.state, uintptr(depressed), uintptr(latched),
		uintptr(locked), 0, 0, uintptr(group))
}

// Sym is the keysym key produces in the current state: shift level,
// layout and Caps Lock applied. key is an XKB keycode (evdev code + 8).
// 0 when it produces none or several.
func (k *Keymap) Sym(key uint32) uint32 {
	r, _, _ := purego.SyscallN(fnStateKeyGetOneSym, k.state, uintptr(key))
	return uint32(r)
}

// BaseSym is the keysym on key's first shift level in the current layout,
// the one X11 reports for column 0. It names the key independent of Shift,
// so Shift+a and a are both KeyA.
func (k *Keymap) BaseSym(key uint32) uint32 {
	layout, _, _ := purego.SyscallN(fnStateKeyGetLayout, k.state, uintptr(key))
	if uint32(layout) == layoutInvalid {
		return 0
	}
	return k.levelSym(key, uint32(layout), 0)
}

// KeySyms returns what the KeyCode rules (docs/specs/keyboard-layouts.md
// § 2) read about key in the current layout: the keysym with only NumLock
// applied, and with Shift added. When the active layout types no Latin
// letters, nonLatin is set and latin is the first Latin layout's keysym
// with only NumLock applied (0 if no layout is Latin).
func (k *Keymap) KeySyms(key uint32) (plain, shifted, latin uint32, nonLatin bool) {
	layout, _, _ := purego.SyscallN(fnStateKeyGetLayout, k.state, uintptr(key))
	if uint32(layout) == layoutInvalid {
		return 0, 0, 0, false
	}
	locked, _, _ := purego.SyscallN(fnStateSerializeMods, k.state, stateModsLocked)
	numLock := uint32(locked) & k.numLockMask
	plain = k.symWith(key, 0, numLock, uint32(layout))
	shifted = k.symWith(key, k.shiftMask, numLock, uint32(layout))
	if uint32(layout) < 32 && k.latinLayouts&(1<<uint32(layout)) != 0 {
		return plain, shifted, 0, false
	}
	for l := range uint32(32) {
		if k.latinLayouts&(1<<l) != 0 {
			latin = k.symWith(key, 0, numLock, l)
			break
		}
	}
	return plain, shifted, latin, true
}

// symWith is the keysym key produces with the given depressed and locked
// modifiers in one layout, read from the scratch state.
func (k *Keymap) symWith(key, depressed, locked, layout uint32) uint32 {
	purego.SyscallN(fnStateUpdateMask, k.scratch, uintptr(depressed), 0,
		uintptr(locked), 0, 0, uintptr(layout))
	r, _, _ := purego.SyscallN(fnStateKeyGetOneSym, k.scratch, uintptr(key))
	return uint32(r)
}

// Repeats reports whether key auto-repeats while held.
func (k *Keymap) Repeats(key uint32) bool {
	r, _, _ := purego.SyscallN(fnKeymapKeyRepeats, k.keymap, uintptr(key))
	return int32(r) != 0
}

// State returns the effective modifiers as an X11 core state mask
// (MaskShift, MaskControl, ...). Lock is Caps Lock.
func (k *Keymap) State() uint16 {
	r, _, _ := purego.SyscallN(fnStateSerializeMods, k.state, stateModsEffective)
	return k.coreState(uint32(r))
}

// Consumed returns the modifiers translating key in the current state used
// up, as an X11 core state mask: Shift when Shift+7 types "/". It uses
// libxkbcommon's GTK mode, made for shortcut matching: a modifier counts
// only if it changes what the key produces.
func (k *Keymap) Consumed(key uint32) uint16 {
	r, _, _ := purego.SyscallN(fnStateKeyGetConsumed, k.state, uintptr(key), consumedModeGTK)
	return k.coreState(uint32(r))
}

// coreState converts an XKB modifier mask to an X11 core state mask.
func (k *Keymap) coreState(mods uint32) uint16 {
	var s uint16
	for i, m := range coreMods {
		if mods&k.coreMask[i] != 0 {
			s |= m.bit
		}
	}
	return s
}

// KeysymToRune is the character a keysym types, or 0 when it types none.
// It covers every script libxkbcommon knows, unlike the Latin-1, keypad,
// Euro and direct-Unicode subset x11key.KeysymToRune handles.
func KeysymToRune(sym uint32) rune {
	if fnKeysymToUTF32 == 0 {
		return 0
	}
	r, _, _ := purego.SyscallN(fnKeysymToUTF32, uintptr(sym))
	return rune(uint32(r))
}

// cptr turns an address libxkbcommon handed over (C memory) back into a
// pointer, without vet reading it as a Go pointer kept in a uintptr.
func cptr(u uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&u)) }
