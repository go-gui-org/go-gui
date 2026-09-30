//go:build (linux && !js && !android) || (windows && !js)

package gl

import (
	"errors"
	"fmt"
	"os"
)

// exitNoGPUOrPanic is Run/RunApp's error exit: a GPU-context failure
// prints one line (what failed, what to try) and exits non-zero with
// no stack trace, so an RDP/VM user sees advice instead of a panic
// dump. Every other error is a programmer error and still panics.
func exitNoGPUOrPanic(err error) {
	if errors.Is(err, ErrNoGPUContext) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	panic(fmt.Sprintf("gl: %v", err))
}
