//go:build darwin && !ios

package darwinffi

import (
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// rt holds the Objective-C runtime entry points, bound on first use.
var rt struct {
	once sync.Once

	msgSend  uintptr
	poolPush uintptr
	poolPop  uintptr

	// stringWithUTF8String is objc_msgSend typed for
	// +[NSString stringWithUTF8String:]. purego copies s to a
	// NUL-terminated C string for the length of the call.
	stringWithUTF8String func(cls objc.ID, sel objc.SEL, s string) objc.ID

	nsString                objc.ID
	selStringWithUTF8String objc.SEL
}

func loadRT() {
	rt.once.Do(func() {
		lib := Open(PathObjC)
		// NSString lives in Foundation. Load it before GetClass, or the
		// class is not registered yet.
		Open(PathFoundation)
		rt.msgSend = Func(lib, "objc_msgSend")
		rt.poolPush = Func(lib, "objc_autoreleasePoolPush")
		rt.poolPop = Func(lib, "objc_autoreleasePoolPop")
		purego.RegisterFunc(&rt.stringWithUTF8String, rt.msgSend)
		rt.nsString = Class("NSString")
		rt.selStringWithUTF8String = objc.RegisterName("stringWithUTF8String:")
	})
}

// MsgSend returns the address of objc_msgSend, for Caller.Call.
func MsgSend() uintptr {
	loadRT()
	return rt.msgSend
}

// Class returns the class named name as an object, or 0 when no such
// class is registered. Load the framework that holds it first.
func Class(name string) objc.ID {
	return objc.ID(objc.GetClass(name))
}

// Sel returns the selector for name. objc.RegisterName takes the
// runtime lock, so cache the result outside a hot path.
func Sel(name string) objc.SEL {
	return objc.RegisterName(name)
}

// NSString returns an autoreleased NSString that holds s, or 0 when s
// is not valid UTF-8. Call it inside an autorelease pool (PoolPush).
// As with C.CString, s ends at its first NUL byte.
func NSString(s string) objc.ID {
	loadRT()
	return rt.stringWithUTF8String(rt.nsString, rt.selStringWithUTF8String, s)
}

// PoolPush opens an autorelease pool and locks the goroutine to its
// OS thread until the matching PoolPop. A pool belongs to the thread
// that opened it: if the goroutine moved threads between push and pop,
// the pop would drain another thread's pool. The lock counts, so a
// caller that already holds LockOSThread (the main loop) keeps it.
//
// The pair is for setup paths. It allocates on each call; a per-event
// path uses Caller with PoolPushAddr and PoolPopAddr instead.
func PoolPush() uintptr {
	loadRT()
	runtime.LockOSThread()
	r, _, _ := purego.SyscallN(rt.poolPush)
	return r
}

// PoolPop drains the pool that PoolPush opened and releases the
// thread lock that PoolPush took.
func PoolPop(pool uintptr) {
	purego.SyscallN(rt.poolPop, pool)
	runtime.UnlockOSThread()
}

// PoolPushAddr returns the address of objc_autoreleasePoolPush, for
// Caller. PoolPopAddr gives the matching pop.
func PoolPushAddr() uintptr {
	loadRT()
	return rt.poolPush
}

// PoolPopAddr returns the address of objc_autoreleasePoolPop.
func PoolPopAddr() uintptr {
	loadRT()
	return rt.poolPop
}

// Caller calls a C function or sends a message with no heap
// allocation. purego.SyscallN is go:uintptrescapes, so the compiler
// moves a ...uintptr list built at its call site to the heap on every
// call. Caller copies the arguments into its own array and passes a
// slice of that, which gives SyscallN nothing new to move.
//
// A Caller must live in heap or global memory, and it is not safe for
// concurrent use: guard it with the lock that guards the state it
// works on. Each argument is a plain uintptr by the time it reaches
// SyscallN, so an argument that is a pointer must point to heap or
// global memory that stays reachable for the call, never to a stack
// local (docs/specs/cgo-free-backend-feasibility.md § Investigation).
type Caller struct {
	args [8]uintptr
}

// Call calls fn with args and returns the first result register.
// It panics when there are more than 8 arguments.
func (c *Caller) Call(fn uintptr, args ...uintptr) uintptr {
	n := copy(c.args[:], args)
	if n != len(args) {
		panic("darwinffi: Caller.Call takes at most 8 arguments")
	}
	r, _, _ := purego.SyscallN(fn, c.args[:n]...)
	return r
}
