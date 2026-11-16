//go:build linux && !android && (amd64 || arm64)

package wl

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// maxCallArgs is the most arguments any libwayland or libwayland-egl call
// in this package takes: wl_proxy_marshal_array_flags has six.
const maxCallArgs = 6

// callBuf holds one call's arguments while purego.SyscallN runs. A literal
// argument list, purego.SyscallN(fn, a, b), heap-allocates its variadic
// slice on every call: SyscallN lets the slice escape (go build -gcflags=-m
// says so; ebitengine/purego#567). A slice of a package array does not
// allocate, which is what the #1160 spike measured at 0 allocations per
// call. That cost a frame about 9 allocations (#1265 step 5).
//
// Safe for the same reason as marshalBuf: the package is single-threaded
// (doc.go), and the C function has the arguments in registers before
// SyscallN returns, so nothing reads callBuf after the call.
var callBuf [maxCallArgs]uintptr

// call runs the C function fn with args and returns its result. args does
// not escape (it is only copied), so a caller's argument list stays on its
// stack.
//
// An argument must not point into Go stack memory. purego.SyscallN carries
// the go:uintptrescapes directive: for uintptr(unsafe.Pointer(&x)) written
// in its own argument list, the compiler moves x to the heap, so a stack
// copy during the call cannot leave the C side holding a stale address.
// That directive is also why its argument slice escapes. call does not
// carry it, so a stack pointer passed through call could go stale. Pass
// integers, C memory, package variables (marshalBuf) and heap objects
// here; a call that takes the address of a local uses purego.SyscallN
// directly (Connect, Conn.Err), and both of those run once, not per frame.
func call(fn uintptr, args ...uintptr) uintptr {
	if len(args) > maxCallArgs {
		panic(fmt.Sprintf("wl: call with %d arguments, at most %d", len(args), maxCallArgs))
	}
	n := copy(callBuf[:], args)
	r, _, _ := purego.SyscallN(fn, callBuf[:n]...)
	return r
}
