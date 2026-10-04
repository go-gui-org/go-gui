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
	fnStateNew              uintptr
	fnStateUnref            uintptr
	fnStateUpdateMask       uintptr
	fnStateKeyGetOneSym     uintptr
	fnStateKeyGetLayout     uintptr
	fnStateSerializeMods    uintptr
	fnKeysymToUTF32         uintptr

	loadOnce sync.Once
	loadErr  error
)

// libxkbcommon constants (xkbcommon.h).
const (
	keymapFormatTextV1 = 1
	stateModsEffective = 1 << 3
	modInvalid         = 0xffffffff
	layoutInvalid      = 0xffffffff
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
		{&fnStateNew, "xkb_state_new"},
		{&fnStateUnref, "xkb_state_unref"},
		{&fnStateUpdateMask, "xkb_state_update_mask"},
		{&fnStateKeyGetOneSym, "xkb_state_key_get_one_sym"},
		{&fnStateKeyGetLayout, "xkb_state_key_get_layout"},
		{&fnStateSerializeMods, "xkb_state_serialize_mods"},
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
)

// coreMods lists, per X11 core bit, the XKB modifier names that set it. The
// virtual names (Alt, Super) are there for keymaps whose state reports the
// virtual modifier without its real one.
var coreMods = [...]struct {
	bit   uint16
	names []string
}{
	{MaskShift, []string{"Shift"}},
	{MaskLock, []string{"Lock"}},
	{MaskControl, []string{"Control"}},
	{MaskMod1, []string{"Mod1", "Alt"}},
	{MaskMod4, []string{"Mod4", "Super"}},
}

// Keymap is a compiled keymap and the keyboard state tracked on it.
type Keymap struct {
	ctx, keymap, state uintptr
	// coreMask[i] holds the XKB modifier bits that set coreMods[i].bit.
	coreMask [len(coreMods)]uint32
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

// newKeymap wraps a compiled keymap; it takes over ctx and km.
func newKeymap(ctx, km uintptr) (*Keymap, error) {
	st, _, _ := purego.SyscallN(fnStateNew, km)
	if st == 0 {
		purego.SyscallN(fnKeymapUnref, km)
		purego.SyscallN(fnContextUnref, ctx)
		return nil, errors.New("xkb: xkb_state_new failed")
	}
	k := &Keymap{ctx: ctx, keymap: km, state: st}
	for i, m := range coreMods {
		for _, name := range m.names {
			if idx := k.modIndex(name); idx < 32 {
				k.coreMask[i] |= 1 << idx
			}
		}
	}
	return k, nil
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
	var syms uintptr // const xkb_keysym_t *
	n, _, _ := purego.SyscallN(fnKeymapKeyGetSymsByLvl, k.keymap, uintptr(key),
		layout, 0, uintptr(unsafe.Pointer(&syms)))
	if int32(n) <= 0 || syms == 0 {
		return 0
	}
	return *(*uint32)(cptr(syms))
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
	mods := uint32(r)
	var s uint16
	for i, m := range coreMods {
		if mods&k.coreMask[i] != 0 {
			s |= m.bit
		}
	}
	return s
}

// KeysymToRune is the character a keysym types, or 0 when it types none.
// It covers every script libxkbcommon knows, unlike the Latin-1 and
// direct-Unicode subset x11key.KeysymToRune handles.
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
