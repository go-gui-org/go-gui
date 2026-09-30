//go:build linux && !js && !android

package gl

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestNewWrapsGPUDisplayError drives New with a failing EGL display
// init: no GPU and no GL needed, only an X connection (CI provides
// Xvfb). Without any display the test skips, like the render smoke
// test.
func TestNewWrapsGPUDisplayError(t *testing.T) {
	cause := errors.New("eglInitialize failed (egl error 0x3001)")
	old := eglInitDisplayNFunc
	eglInitDisplayNFunc = func() (uintptr, []eglConfigVisual, error) {
		return 0, nil, cause
	}
	defer func() { eglInitDisplayNFunc = old }()

	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	b, err := New(w)
	if err == nil {
		b.Destroy()
		t.Fatal("New with failing EGL init returned nil error")
	}
	if strings.Contains(err.Error(), "x11 connect") {
		t.Skipf("no X display, seam not reached: %v", err)
	}
	if !errors.Is(err, ErrNoGPUContext) {
		t.Fatalf("New error = %v, want ErrNoGPUContext", err)
	}
	if got := errors.Unwrap(err); got == nil {
		t.Fatalf("wrapped error %v keeps no cause", err)
	}
}

// TestNewWrapsGPUSurfaceError drives New with a failing EGL surface
// creation and a real display init: no GL needed, but an X connection
// and EGL are. Anything short of the injected seam skips — a plain X
// login without EGL must not fail.
func TestNewWrapsGPUSurfaceError(t *testing.T) {
	cause := errors.New("eglCreateWindowSurface failed (egl error 0x3001)")
	old := eglCreateSurfaceContextFunc
	eglCreateSurfaceContextFunc = func(dpy, config uintptr, win uint32) (uintptr, uintptr, error) {
		return 0, 0, cause
	}
	defer func() { eglCreateSurfaceContextFunc = old }()

	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	b, err := New(w)
	if err == nil {
		b.Destroy()
		t.Fatal("New with failing EGL surface returned nil error")
	}
	if strings.Contains(err.Error(), "x11 connect") ||
		strings.Contains(err.Error(), "init EGL display") {
		t.Skipf("injected seam not reached: %v", err)
	}
	if !errors.Is(err, ErrNoGPUContext) {
		t.Fatalf("New error = %v, want ErrNoGPUContext", err)
	}
	if !strings.Contains(err.Error(), "create EGL surface") {
		t.Fatalf("New error = %v, want the surface op named", err)
	}
}
