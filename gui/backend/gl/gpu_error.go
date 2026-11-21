//go:build !js

package gl

import (
	"errors"
	"fmt"
	"runtime"
)

// ErrNoGPUContext marks a backend init failure caused by a missing
// GPU context: no EGL display (Linux) or no WGL context (Windows), or
// on Windows a context that cannot run the backend, such as
// Microsoft's generic OpenGL 1.1 ("GDI Generic"), which lacks the GL
// functions or cannot build the shaders (#1310).
// The usual field cases are Windows under RDP or in a VM with no GPU
// driver, and Linux without Mesa or driver GL. On Linux X11 with
// GOGUI_RENDERER unset, an EGL failure opens the window on the CPU
// renderer instead (#1265), so there it is reported only for
// GOGUI_RENDERER=gl, or when the CPU renderer cannot run either. On
// Windows, GOGUI_RENDERER=soft opens the window on the CPU renderer with no
// WGL at all (#1310), so it is not reported then. Use errors.Is to test for
// it; errors.Unwrap returns the EGL/WGL/GL cause.
// exportaudit:keep — matched by errors.Is from app mains, not by import
var ErrNoGPUContext = errors.New("gl: no GPU context")

// gpuContextError wraps an EGL/WGL/GL cause with what to try next. It
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
	// On Linux (#1265) and Windows (#1310) the CPU renderer needs no GPU
	// context at all, so it is the one fix that works without a driver or
	// a package install.
	soft := ""
	if (runtime.GOOS == "linux" || runtime.GOOS == "windows") && !e.noSoft {
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
// remediation. On Windows it also tags a context that came up but
// cannot bind the GL functions or build the built-in pipelines.
// Connection and window failures, the text system, and the Linux
// GL-binding failure are not GPU-context failures and stay plain errors.
func newGPUContextError(op string, cause error) error {
	return &gpuContextError{op: op, cause: cause}
}
