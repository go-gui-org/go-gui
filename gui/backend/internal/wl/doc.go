// Package wl binds the Wayland client library (libwayland-client) through
// purego, so the experimental Wayland backend stays cgo-free (#919).
//
// Why libwayland and not a pure-Go wire protocol: EGL needs a real
// libwayland wl_display* and wl_surface* to render into a Wayland window
// (eglGetPlatformDisplay and wl_egl_window_create). A pure-Go client has no
// such pointers to hand it.
//
// How it binds:
//
//   - Requests go through wl_proxy_marshal_array_flags (libwayland 1.20 or
//     later), which takes the arguments as an array instead of C varargs.
//   - Events arrive through one purego callback installed with
//     wl_proxy_add_dispatcher. libwayland hands it the opcode and the decoded
//     argument array, and the generated SetHandlers code routes them to Go
//     funcs. One callback serves every proxy, which keeps clear of purego's
//     fixed callback limit.
//   - The wl_interface / wl_message tables libwayland reads are generated
//     from the protocol XML in protocols/ by internal/wlgen and built in Go
//     memory the first time Load runs.
//   - Calls go through purego.SyscallN on raw addresses, not
//     purego.RegisterFunc, for the reason given in glbind/raw.go: the
//     RegisterFunc wrapper allocates several times per call.
//
// The package is not safe for concurrent use. Every call, and every
// handler, runs on the goroutine that drives the Conn, which must be locked
// to its OS thread like the rest of the GL backend. One Conn at a time.
//
// wlr-virtual-pointer and input-method-v2 are bound only by tests: they let
// a test drive a pointer, and act as an input method, on its own connection
// under sway, which has neither when headless.
//
// It builds only for little-endian 64-bit Linux (amd64, arm64): a
// wl_argument is read as one 8-byte slot holding its value in the low bytes.
package wl

//go:generate go run ./internal/wlgen . protocols/wayland.xml protocols/xdg-shell.xml protocols/xdg-decoration-unstable-v1.xml protocols/viewporter.xml protocols/fractional-scale-v1.xml protocols/cursor-shape-v1.xml protocols/text-input-unstable-v3.xml protocols/primary-selection-unstable-v1.xml protocols/wlr-virtual-pointer-unstable-v1.xml protocols/input-method-unstable-v2.xml
