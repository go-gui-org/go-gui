//go:build !js && !darwin && !windows

package glbind

import (
	"testing"

	"github.com/ebitengine/purego"
)

// harmlessProcAddr returns the address of a platform export that can be called
// with any arguments without consequence, so the alloc gate can drive the GL
// wrappers without a GL context.
//
// libc's getpid takes no arguments and returns the process id. The SysV AMD64
// convention leaves argument registers to the caller, so the arguments the
// wrappers pass are ignored, and the result is discarded. A libc that cannot be
// opened by this name (a static or musl build) skips rather than fails: the
// gate is about allocation counts, not about libc.
func harmlessProcAddr(tb testing.TB) uintptr {
	tb.Helper()
	lib, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		tb.Skipf("dlopen libc.so.6: %v", err)
	}
	proc, err := purego.Dlsym(lib, "getpid")
	if err != nil || proc == 0 {
		tb.Skipf("dlsym getpid: %v", err)
	}
	return proc
}

// ptrArgAllocs is the per-call allocation budget for a wrapper that hands a Go
// pointer to the driver. purego.SyscallN carries //go:uintptrescapes, which
// keeps the referent alive but also forces it to the heap, so a caller passing
// a function-local array pays one allocation for that array — where Windows,
// with //go:uintptrkeepalive, pays none. There is no CGo-free alternative, and
// one allocation still sits far below the six the purego.RegisterFunc path cost
// per call before #812.
const ptrArgAllocs = 1.0
