// Package xkb binds the parts of libxkbcommon the Wayland backend needs to
// turn key events into keysyms (#919), through purego so it stays cgo-free.
//
// A Wayland compositor sends raw evdev key codes plus a keymap in XKB text
// form. libxkbcommon compiles that keymap and tracks the modifier and layout
// state the compositor reports, which gives the keysym a key produces.
// Keysyms are the X11 ones, so the backend reuses the X11 key mapping
// (x11key) and compose tables from there.
//
// Calls go through purego.SyscallN on raw addresses, like package wl, so a
// key press allocates nothing. Not safe for concurrent use; the backend
// calls it from its event loop only.
package xkb
