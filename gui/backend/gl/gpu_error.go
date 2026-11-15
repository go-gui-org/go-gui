//go:build !js

package gl

import (
	"errors"
	"fmt"
	"runtime"
)

// ErrNoGPUContext marks a backend init failure caused by a missing
// GPU context: no EGL display (Linux) or no WGL context (Windows).
// The usual field cases are Windows under RDP or in a VM with no GPU
// driver, and Linux without Mesa or driver GL. On Linux X11 with
// GOGUI_RENDERER unset, an EGL failure opens the window on the CPU
// renderer instead (#1265), so there it is reported only for
// GOGUI_RENDERER=gl, or when the CPU renderer cannot run either. Use
// errors.Is to test for it; errors.Unwrap returns the EGL/WGL cause.
// exportaudit:keep — matched by errors.Is from app mains, not by import
var ErrNoGPUContext = errors.New("gl: no GPU context")

// gpuContextError wraps an EGL/WGL cause with what to try next. It
// matches ErrNoGPUContext via Is so errors.Is finds it through any
// number of fmt %w layers.
type gpuContextError struct {
	op    string
	cause error
	// noSoft leaves the GOGUI_RENDERER=soft hint out: the CPU renderer was
	// tried and cannot run on this server either.
	noSoft bool
}

func (e *gpuContextError) Error() string {
	// On Linux the CPU renderer needs no GPU context at all (#1265), so it
	// is the one fix that works without a driver or a package install.
	// Windows has no soft present path yet.
	soft := ""
	if runtime.GOOS == "linux" && !e.noSoft {
		soft = ", or set GOGUI_RENDERER=soft"
	}
	return fmt.Sprintf("gl: %s: %v — no GPU context "+
		"(update the GPU driver, leave RDP/VM, or install Mesa on Linux%s)",
		e.op, e.cause, soft)
}

func (e *gpuContextError) Unwrap() error { return e.cause }

func (e *gpuContextError) Is(target error) bool {
	return target == ErrNoGPUContext
}

// newGPUContextError tags a context-creation failure (EGL display,
// EGL surface/context, WGL context) with the sentinel and the
// remediation. Connection, window, and GL-binding failures are not
// GPU-context failures and stay plain errors.
func newGPUContextError(op string, cause error) error {
	return &gpuContextError{op: op, cause: cause}
}
