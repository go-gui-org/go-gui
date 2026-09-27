package glbind

import (
	"syscall"
	"testing"
)

// harmlessProcAddr returns the address of a platform export that can be called
// with any arguments without consequence, so the alloc gate can drive the GL
// wrappers without a GL context.
//
// kernel32!SetLastError takes one DWORD and returns nothing. Under the Win64
// calling convention a callee reads only the parameters it declares, so the
// extra register and stack arguments the wider wrappers pass are ignored, and
// stack space for them belongs to the caller. The only observable effect is the
// thread's last-error value, which no test reads.
func harmlessProcAddr(tb testing.TB) uintptr {
	tb.Helper()
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("SetLastError")
	if err := proc.Find(); err != nil {
		tb.Skipf("kernel32!SetLastError unavailable: %v", err)
	}
	return proc.Addr()
}

// rawCallAllocs is the per-call allocation budget for a wrapper that passes
// integers only. syscall.SyscallN keeps its variadic slice on the stack, so the
// raw path allocates nothing.
const rawCallAllocs = 0.0

// ptrArgAllocs is the per-call allocation budget for a wrapper that hands a Go
// pointer to the driver. syscall.SyscallN carries //go:uintptrkeepalive, which
// keeps the referent alive without forcing it to the heap, so a caller passing
// a function-local array pays nothing.
const ptrArgAllocs = 0.0
