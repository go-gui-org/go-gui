//go:build !js

package gl

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestGPUContextErrorMatchesSentinel(t *testing.T) {
	cause := errors.New("eglInitialize failed (egl error 0x3001)")
	err := newGPUContextError("init EGL display", cause)
	if !errors.Is(err, ErrNoGPUContext) {
		t.Fatalf("errors.Is(%v, ErrNoGPUContext) = false", err)
	}
	// The sentinel survives New's "gl: %w" wrap layer.
	if !errors.Is(fmt.Errorf("gl: %w", err), ErrNoGPUContext) {
		t.Fatal("sentinel lost through fmt %w wrap")
	}
	if !errors.Is(fmt.Errorf("gl: create window: %w",
		fmt.Errorf("gl: %w", err)), ErrNoGPUContext) {
		t.Fatal("sentinel lost through runAppE's double wrap")
	}
	if got := errors.Unwrap(err); got != cause {
		t.Fatalf("Unwrap() = %v, want the EGL/WGL cause", got)
	}
}

func TestGPUContextErrorMessageNamesFix(t *testing.T) {
	err := newGPUContextError("create WGL context",
		errors.New("wglCreateContext: failed"))
	msg := err.Error()
	for _, want := range []string{"no GPU", "driver", "RDP", "Mesa"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q misses %q", msg, want)
		}
	}
	if strings.Contains(msg, "\n") {
		t.Fatalf("message must stay one line, got %q", msg)
	}
}

func TestNonGPUErrorMissesSentinel(t *testing.T) {
	err := fmt.Errorf("gl: x11 connect: %v", errors.New("no display"))
	if errors.Is(err, ErrNoGPUContext) {
		t.Fatalf("plain init error %v must not match ErrNoGPUContext", err)
	}
}
