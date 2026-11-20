//go:build darwin && arm64 && !ios

package darwinffi

import "unsafe"

// FCaller calls a C function, or sends an Objective-C message, that
// takes or returns floating-point values, with no heap allocation.
//
// purego.SyscallN cannot do this. It puts a copy of the integer
// arguments in the float registers, and it returns only X0 and X1.
// objc.Send and purego.RegisterFunc can do it, but they allocate 4 to 9
// times on each call (measured with purego v0.11.3). That is too much
// for an event path that reads the mouse position and the scroll
// deltas of each event.
//
// Set X and D to the arguments, call Call, then read R0 (an integer or
// pointer result) or RD (a float result, or the members of an NSPoint,
// NSSize or NSRect result). Integer and float arguments are counted on
// their own: for -[NSView convertPoint:fromView:], X is {view, sel,
// fromView} and D is {x, y}.
//
// The same rules as Caller apply. An FCaller must live in heap or
// global memory, it is not safe for concurrent use, and an argument that
// is a pointer must point to memory that stays reachable for the call.
//
// darwin/arm64 only. A struct result of more than four floats, or of
// mixed integers and floats, does not come back in these registers.
type FCaller struct {
	fn uintptr
	X  [8]uintptr
	D  [8]float64
	R0 uintptr
	RD [4]float64
	// S holds the ninth and tenth integer arguments, which go on the
	// stack. Each must be 8 bytes wide.
	S [2]uintptr
}

// fcallABI0 is the address of fcallX in fcall_arm64.s.
var fcallABI0 uintptr

// runtimeCgocall runs fn(arg) on the system stack, as a cgo call does.
// The runtime marks cgocall as a push linkname (go.dev/issue/67401),
// and purego pulls it the same way.
//
//go:linkname runtimeCgocall runtime.cgocall
func runtimeCgocall(fn uintptr, arg unsafe.Pointer) int32

// Call calls fn with the arguments in X and D. The results are in R0
// and RD after it returns. The arguments stay as they are, so a caller
// that sends the same message again sets only what changed.
func (c *FCaller) Call(fn uintptr) {
	c.fn = fn
	runtimeCgocall(fcallABI0, unsafe.Pointer(c))
}

// Send sends sel to obj with no other integer arguments. It sets X[0]
// and X[1] and leaves X[2:] and D as they are, for a message whose
// further arguments the caller set.
func (c *FCaller) Send(obj, sel uintptr) {
	c.X[0] = obj
	c.X[1] = sel
	c.Call(MsgSend())
}
