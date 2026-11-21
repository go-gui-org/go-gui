//go:build !js && !darwin && !windows

package glbind

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// sysN calls a resolved GL entry point with integer arguments only.
//
// The error results are dropped on purpose: GL reports errors through
// glGetError, which the backend polls separately. sysN1 keeps the result,
// for the few hot entry points that return one (the GL 3.2 fences).
//
// Callers must not pass a Go pointer converted to uintptr: see the rules in
// raw.go. The pointer-carrying entry points below convert inside the
// purego.SyscallN call expression instead, which carries //go:uintptrescapes
// and so keeps the referent alive for the duration of the call.
func sysN(fn uintptr, args ...uintptr) {
	_, _, _ = purego.SyscallN(fn, args...)
}

// sysN1 is sysN for an entry point that returns a value.
func sysN1(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}

// Unlike syscall.SyscallN on Windows, purego.SyscallN allocates twice on every
// call: its variadic slice escapes through //go:uintptrescapes, and it heap-
// allocates the args struct it hands to runtime_cgocall. //go:uintptrescapes
// also forces a pointer argument's referent to the heap, so a caller passing
// &localArray[0] pays a third allocation where Windows pays none. That is
// still below the six allocations per call the purego.RegisterFunc path cost,
// and there is no CGo-free alternative that stays on purego's public API.
// raw_alloc_other_test.go pins these numbers.

func BufferData(target uint32, size int, data unsafe.Pointer, usage uint32) {
	_, _, _ = purego.SyscallN(addrBufferData, uintptr(target), uintptr(size),
		uintptr(data), uintptr(usage))
}

func BufferSubData(target uint32, offset int, size int, data unsafe.Pointer) {
	_, _, _ = purego.SyscallN(addrBufferSubData, uintptr(target), uintptr(offset),
		uintptr(size), uintptr(data))
}

func DrawElements(mode uint32, count int32, xtype uint32, indices unsafe.Pointer) {
	_, _, _ = purego.SyscallN(addrDrawElements, uintptr(mode), uintptr(count),
		uintptr(xtype), uintptr(indices))
}

func TexSubImage2D(target uint32, level int32, xoffset int32, yoffset int32, width int32, height int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	_, _, _ = purego.SyscallN(addrTexSubImage2D, uintptr(target), uintptr(level),
		uintptr(xoffset), uintptr(yoffset), uintptr(width), uintptr(height),
		uintptr(format), uintptr(xtype), uintptr(pixels))
}

func UniformMatrix4fv(location int32, count int32, transpose bool, value *float32) {
	_, _, _ = purego.SyscallN(addrUniformMatrix4fv, uintptr(location), uintptr(count),
		b2u(transpose), uintptr(unsafe.Pointer(value)))
}
