package glbind

import (
	"syscall"
	"unsafe"
)

// sysN calls a resolved GL entry point with integer arguments only.
//
// The return values are dropped on purpose. No hot entry point returns a
// value, and the Errno that syscall.SyscallN reports is the Win32 last-error,
// which GL does not set — GL errors are retrieved with glGetError, which the
// backend polls separately.
//
// Callers must not pass a Go pointer converted to uintptr: see the rules in
// raw.go. The pointer-carrying entry points below convert inside the
// syscall.SyscallN call expression instead, which is the only form the
// compiler keeps the referent alive across (//go:uintptrkeepalive on
// syscall.SyscallN).
func sysN(fn uintptr, args ...uintptr) {
	_, _, _ = syscall.SyscallN(fn, args...)
}

func BufferData(target uint32, size int, data unsafe.Pointer, usage uint32) {
	_, _, _ = syscall.SyscallN(addrBufferData, uintptr(target), uintptr(size),
		uintptr(data), uintptr(usage))
}

func BufferSubData(target uint32, offset int, size int, data unsafe.Pointer) {
	_, _, _ = syscall.SyscallN(addrBufferSubData, uintptr(target), uintptr(offset),
		uintptr(size), uintptr(data))
}

func DrawElements(mode uint32, count int32, xtype uint32, indices unsafe.Pointer) {
	_, _, _ = syscall.SyscallN(addrDrawElements, uintptr(mode), uintptr(count),
		uintptr(xtype), uintptr(indices))
}

func TexSubImage2D(target uint32, level int32, xoffset int32, yoffset int32, width int32, height int32, format uint32, xtype uint32, pixels unsafe.Pointer) {
	_, _, _ = syscall.SyscallN(addrTexSubImage2D, uintptr(target), uintptr(level),
		uintptr(xoffset), uintptr(yoffset), uintptr(width), uintptr(height),
		uintptr(format), uintptr(xtype), uintptr(pixels))
}

func UniformMatrix4fv(location int32, count int32, transpose bool, value *float32) {
	_, _, _ = syscall.SyscallN(addrUniformMatrix4fv, uintptr(location), uintptr(count),
		b2u(transpose), uintptr(unsafe.Pointer(value)))
}
