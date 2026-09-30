//go:build windows && !js

package gl

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestNewWrapsGPUContextError drives New with a failing WGL context
// creation: no GPU driver and no GL needed, only window creation,
// which works headless.
func TestNewWrapsGPUContextError(t *testing.T) {
	cause := errors.New("wglCreateContext: failed")
	old := createContextFunc
	createContextFunc = func(hdc uintptr) (uintptr, error) {
		return 0, cause
	}
	defer func() { createContextFunc = old }()

	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	b, err := New(w)
	if err == nil {
		b.Destroy()
		t.Fatal("New with failing WGL context returned nil error")
	}
	// Headless runners cannot create a window at all (observed on
	// MSYS2: CreateWindowExW finds no window class), so the injected
	// seam is never reached — skip, like the X11 test without a
	// display.
	if strings.Contains(err.Error(), "CreateWindowExW") ||
		strings.Contains(err.Error(), "GetDC") {
		t.Skipf("no window, seam not reached: %v", err)
	}
	if !errors.Is(err, ErrNoGPUContext) {
		t.Fatalf("New error = %v, want ErrNoGPUContext", err)
	}
	if got := errors.Unwrap(err); got == nil {
		t.Fatalf("wrapped error %v keeps no cause", err)
	}
}
