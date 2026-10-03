//go:build linux && !android && (amd64 || arm64)

package xkb

import (
	"bytes"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
)

// evdev key codes (linux/input-event-codes.h) plus the XKB offset of 8.
const (
	keyA      = 30 + 8
	keyShiftL = 42 + 8
)

// defaultKeymap compiles the system default keymap (us, from
// xkeyboard-config) and returns it in the text form a compositor sends.
// Skips when libxkbcommon or its data is missing.
func defaultKeymap(t *testing.T) []byte {
	t.Helper()
	if err := Load(); err != nil {
		t.Skip(err)
	}
	ctx, _, _ := purego.SyscallN(fnContextNew, 0)
	defer purego.SyscallN(fnContextUnref, ctx)
	km, _, _ := purego.SyscallN(fnKeymapNewFromNames, ctx, 0, 0)
	if km == 0 {
		t.Skip("no default keymap: xkeyboard-config data missing")
	}
	defer purego.SyscallN(fnKeymapUnref, km)
	str, _, _ := purego.SyscallN(fnKeymapGetAsString, km, keymapFormatTextV1)
	if str == 0 {
		t.Fatal("xkb_keymap_get_as_string failed")
	}
	// The string is malloc'd and leaked: one per test run.
	p := cptr(str)
	n := 0
	for *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
	}
	// Kept with its NUL, as wl_keyboard.keymap delivers it.
	return append([]byte(nil), unsafe.Slice((*byte)(p), n+1)...)
}

func TestKeymapSyms(t *testing.T) {
	k, err := NewKeymap(defaultKeymap(t))
	if err != nil {
		t.Fatal(err)
	}
	defer k.Destroy()

	if got := k.Sym(keyA); got != 'a' {
		t.Errorf("Sym(A) = %#x, want 'a'", got)
	}
	if got := k.State(); got != 0 {
		t.Errorf("State() = %#x with nothing held", got)
	}
	if !k.Repeats(keyA) {
		t.Error("A does not repeat")
	}
	if k.Repeats(keyShiftL) {
		t.Error("Shift_L repeats")
	}

	// Shift is real modifier 0 in every keymap.
	k.UpdateMask(1, 0, 0, 0)
	if got := k.Sym(keyA); got != 'A' {
		t.Errorf("Sym(Shift+A) = %#x, want 'A'", got)
	}
	if got := k.BaseSym(keyA); got != 'a' {
		t.Errorf("BaseSym(Shift+A) = %#x, want 'a'", got)
	}
	if got := k.State(); got != MaskShift {
		t.Errorf("State() = %#x, want MaskShift", got)
	}

	// Control is real modifier 2; Mod1 (Alt) is 3.
	k.UpdateMask(1<<2|1<<3, 0, 0, 0)
	if got := k.State(); got != MaskControl|MaskMod1 {
		t.Errorf("State() = %#x, want MaskControl|MaskMod1", got)
	}
	k.Destroy()
	k.Destroy() // twice is safe
}

func TestNewKeymapRejects(t *testing.T) {
	if err := Load(); err != nil {
		t.Skip(err)
	}
	for name, buf := range map[string][]byte{
		"empty":    nil,
		"only NUL": {0, 0},
		"garbage":  []byte("not a keymap\x00"),
		"too big":  bytes.Repeat([]byte("x"), maxKeymapSize+1),
	} {
		if k, err := NewKeymap(buf); err == nil {
			k.Destroy()
			t.Errorf("%s: NewKeymap succeeded", name)
		}
	}
}

func TestKeysymToRune(t *testing.T) {
	if err := Load(); err != nil {
		t.Skip(err)
	}
	for sym, want := range map[uint32]rune{
		'a':        'a',
		0x6c1:      'а', // Cyrillic_a, outside x11key's Latin-1 range
		0x01000101: 'ā', // direct Unicode keysym
		0xff0d:     '\r',
		0xffbe:     0, // F1
	} {
		if got := KeysymToRune(sym); got != want {
			t.Errorf("KeysymToRune(%#x) = %q, want %q", sym, got, want)
		}
	}
}
