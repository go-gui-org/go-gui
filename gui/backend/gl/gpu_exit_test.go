//go:build (linux && !js && !android) || (windows && !js)

package gl

import (
	"errors"
	"strings"
	"testing"
)

// TestExitNoGPUOrPanicPanicsOnPlainError guards the programmer-error
// branch: anything that is not a GPU-context failure must still
// panic with the historic "gl: ..." shape, never exit silently.
func TestExitNoGPUOrPanicPanicsOnPlainError(t *testing.T) {
	err := errors.New("gl: x11 connect: no display")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("exitNoGPUOrPanic did not panic on a plain error")
		}
		msg, ok := r.(string)
		if !ok || !strings.HasPrefix(msg, "gl: ") {
			t.Fatalf("panic = %v, want \"gl: ...\" shape", r)
		}
	}()
	exitNoGPUOrPanic(err)
}
