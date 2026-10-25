//go:build darwin

// Package main is the #1160 spike: an AppKit window with a CAMetalLayer,
// driven through purego/objc with CGO_ENABLED=0. Not for merge.
//
// The file holds the three ways to send an Objective-C message that the
// spike compares for heap allocations:
//
//  1. objc.ID.Send / objc.Send[T]: variadic ...any, built by reflection.
//  2. purego.RegisterFunc: a typed Go func bound once to objc_msgSend.
//  3. purego.SyscallN: integer arguments only, by address.
package main

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

type id = objc.ID

// NSRect and friends mirror the CoreGraphics structs (64-bit).
type NSPoint struct{ X, Y float64 }
type NSSize struct{ Width, Height float64 }
type NSRect struct {
	Origin NSPoint
	Size   NSSize
}

// MTLClearColor is four doubles: a homogeneous float aggregate, passed in
// d0..d3 on arm64.
type MTLClearColor struct{ R, G, B, A float64 }

var (
	libObjC, libMetal uintptr

	msgSendAddr uintptr

	// Typed message functions (option 2). Each is bound once in load.
	msg0        func(obj id, s objc.SEL) id
	msg1        func(obj id, s objc.SEL, a uintptr) id
	msg2        func(obj id, s objc.SEL, a, b uintptr) id
	msgSize     func(obj id, s objc.SEL, sz NSSize)
	msgClear    func(obj id, s objc.SEL, c MTLClearColor)
	msgRect     func(obj id, s objc.SEL) NSRect
	msgFloat    func(obj id, s objc.SEL) float64
	msgSetF     func(obj id, s objc.SEL, v float64)
	msgInitWin  func(obj id, s objc.SEL, r NSRect, style, backing uint, deferFlag bool) id
	msgNextEv   func(app id, s objc.SEL, mask uint64, until, mode id, dequeue bool) id
	msgInitRect func(obj id, s objc.SEL, r NSRect) id
	msgViewport func(obj id, s objc.SEL, v MTLViewport)
	msgKeyEv    func(c id, s objc.SEL, typ uint, loc NSPoint, flags uint, ts float64,
		win int, ctx, chars, charsIgnoring id, repeat bool, keyCode uint16) id

	// Plain C functions, called by address through SyscallN.
	poolPushFn, poolPopFn, createDeviceFn uintptr
)

func mustDlopen(path string) uintptr {
	h, err := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		panic(fmt.Sprintf("spike: load %s: %v", path, err))
	}
	return h
}

func mustDlsym(lib uintptr, name string) uintptr {
	p, err := purego.Dlsym(lib, name)
	if err != nil {
		panic(fmt.Sprintf("spike: %v", err))
	}
	return p
}

// load opens the frameworks and binds the typed message functions. The
// frameworks must be loaded before objc.GetClass can find their classes.
func load() {
	libObjC = mustDlopen("/usr/lib/libobjc.A.dylib")
	// AppKit and QuartzCore are loaded for their classes only.
	mustDlopen("/System/Library/Frameworks/AppKit.framework/AppKit")
	libMetal = mustDlopen("/System/Library/Frameworks/Metal.framework/Metal")
	mustDlopen("/System/Library/Frameworks/QuartzCore.framework/QuartzCore")

	msgSendAddr = mustDlsym(libObjC, "objc_msgSend")
	purego.RegisterFunc(&msg0, msgSendAddr)
	purego.RegisterFunc(&msg1, msgSendAddr)
	purego.RegisterFunc(&msg2, msgSendAddr)
	purego.RegisterFunc(&msgSize, msgSendAddr)
	purego.RegisterFunc(&msgClear, msgSendAddr)
	purego.RegisterFunc(&msgRect, msgSendAddr) // arm64: no _stret entry point
	purego.RegisterFunc(&msgFloat, msgSendAddr)
	purego.RegisterFunc(&msgSetF, msgSendAddr)
	purego.RegisterFunc(&msgInitWin, msgSendAddr)
	purego.RegisterFunc(&msgNextEv, msgSendAddr)
	purego.RegisterFunc(&msgKeyEv, msgSendAddr)
	purego.RegisterFunc(&msgInitRect, msgSendAddr)
	purego.RegisterFunc(&msgViewport, msgSendAddr)

	poolPushFn = mustDlsym(libObjC, "objc_autoreleasePoolPush")
	poolPopFn = mustDlsym(libObjC, "objc_autoreleasePoolPop")
	createDeviceFn = mustDlsym(libMetal, "MTLCreateSystemDefaultDevice")
}

func cls(name string) id { return id(objc.GetClass(name)) }

// sel is for setup paths only: RegisterName takes the runtime lock. Hot
// paths use the selectors cached in sels.
func sel(name string) objc.SEL { return objc.RegisterName(name) }

// vsys0..vsys2 send a message through SyscallN with the arguments listed
// at the call site (option 3, naive form). SyscallN is go:uintptrescapes,
// so the compiler moves the implicit ...uintptr slice to the heap: one
// allocation per call.
func vsys0(obj id, s objc.SEL) id {
	r, _, _ := purego.SyscallN(msgSendAddr, uintptr(obj), uintptr(s))
	return id(r)
}

func vsys1(obj id, s objc.SEL, a uintptr) id {
	r, _, _ := purego.SyscallN(msgSendAddr, uintptr(obj), uintptr(s), a)
	return id(r)
}

// argBuf is the argument array for sys0..sys4 (option 3, buffered form).
// Passing a slice of a package-level array gives SyscallN nothing new to
// move to the heap, so a send makes no allocation. One buffer is enough:
// AppKit and Metal encoding run on the main thread only, and a callback
// into Go that sends again has already finished with the outer call's
// arguments by the time objc_msgSend runs.
//
// The price: an argument is a uintptr by the time it reaches SyscallN, so
// the go:uintptrescapes directive no longer keeps what it points to alive
// or in place.
// A pointer argument must point to heap or global memory (a backend
// scratch field, as the metal backend already uses), never to a stack
// local, which can move when the goroutine stack grows.
var argBuf [15]uintptr

func sys0(obj id, s objc.SEL) id {
	argBuf[0], argBuf[1] = uintptr(obj), uintptr(s)
	r, _, _ := purego.SyscallN(msgSendAddr, argBuf[:2]...)
	return id(r)
}

func sys1(obj id, s objc.SEL, a uintptr) id {
	argBuf[0], argBuf[1], argBuf[2] = uintptr(obj), uintptr(s), a
	r, _, _ := purego.SyscallN(msgSendAddr, argBuf[:3]...)
	return id(r)
}

func sys2(obj id, s objc.SEL, a, b uintptr) id {
	argBuf[0], argBuf[1], argBuf[2], argBuf[3] = uintptr(obj), uintptr(s), a, b
	r, _, _ := purego.SyscallN(msgSendAddr, argBuf[:4]...)
	return id(r)
}

func sys4(obj id, s objc.SEL, a, b, c, d uintptr) id {
	argBuf[0], argBuf[1], argBuf[2], argBuf[3], argBuf[4], argBuf[5] = uintptr(obj), uintptr(s), a, b, c, d
	r, _, _ := purego.SyscallN(msgSendAddr, argBuf[:6]...)
	return id(r)
}

// send48 sends a message whose one argument is a 48-byte struct of six
// words that is not a homogeneous float aggregate: MTLViewport,
// CGAffineTransform. The two ABIs pass it differently, and SyscallN
// can do both:
//
//   - arm64 (AAPCS64): over 16 bytes and not an HFA, so the caller passes
//     a pointer to a copy in the next integer register.
//   - amd64 (System V): over 16 bytes, so class MEMORY: the six words go
//     on the stack and take no register. SyscallN puts arguments 7 and up
//     on the stack, so four zero words fill rdx, rcx, r8 and r9 first.
//
// On arm64 the callee owns the copy and may write to it, so p is copied
// into send48Buf first; the caller's struct is never handed over. The
// buffer is global, so it does not move (see argBuf).
func send48(obj id, s objc.SEL, p *[6]uintptr) id {
	if runtime.GOARCH == "arm64" {
		send48Buf = *p
		return sys1(obj, s, uintptr(unsafe.Pointer(&send48Buf)))
	}
	argBuf[0], argBuf[1] = uintptr(obj), uintptr(s)
	argBuf[2], argBuf[3], argBuf[4], argBuf[5] = 0, 0, 0, 0
	copy(argBuf[6:12], p[:])
	r, _, _ := purego.SyscallN(msgSendAddr, argBuf[:12]...)
	return id(r)
}

var send48Buf [6]uintptr

// callC0 and callC1 call a plain C function by address.
func callC0(fn uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, argBuf[:0]...)
	return r
}

func callC1(fn, a uintptr) uintptr {
	argBuf[0] = a
	r, _, _ := purego.SyscallN(fn, argBuf[:1]...)
	return r
}

func poolPush() uintptr { return callC0(poolPushFn) }

func poolPop(p uintptr) { callC1(poolPopFn, p) }

// nsString returns an autoreleased NSString. Setup paths only.
func nsString(s string) id {
	b := append([]byte(s), 0)
	// A short b lives on the stack. The conversion sits inside the SyscallN
	// call, so go:uintptrescapes moves b to the heap and keeps it alive;
	// sys1 would not (see argBuf). This costs allocations, which is fine
	// off the hot path.
	r, _, _ := purego.SyscallN(msgSendAddr, uintptr(cls("NSString")),
		uintptr(sel("stringWithUTF8String:")), uintptr(unsafe.Pointer(&b[0])))
	return id(r)
}

// cPtr turns an address of C memory into an unsafe.Pointer. The double
// indirection keeps go vet's unsafeptr check quiet: the memory is not
// Go's, so the GC rule it guards does not apply.
func cPtr(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}
