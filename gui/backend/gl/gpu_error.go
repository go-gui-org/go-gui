//go:build !js

package gl

import (
	"errors"
	"fmt"
)

// ErrNoGPUContext marks a backend init failure caused by a missing
// GPU context: no EGL display (Linux) or no WGL context (Windows).
// The usual field cases are Windows under RDP or in a VM with no GPU
// driver, and Linux without Mesa or driver GL. Use errors.Is to test
// for it; errors.Unwrap returns the EGL/WGL cause.
// exportaudit:keep — matched by errors.Is from app mains, not by import
var ErrNoGPUContext = errors.New("gl: no GPU context")

// gpuContextError wraps an EGL/WGL cause with what to try next. It
// matches ErrNoGPUContext via Is so errors.Is finds it through any
// number of fmt %w layers.
type gpuContextError struct {
	op    string
	cause error
}

func (e *gpuContextError) Error() string {
	return fmt.Sprintf("gl: %s: %v — no GPU context "+
		"(update the GPU driver, leave RDP/VM, or install Mesa on Linux)",
		e.op, e.cause)
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
