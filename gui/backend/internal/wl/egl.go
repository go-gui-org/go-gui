//go:build linux && !android && (amd64 || arm64)

package wl

import (
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
)

// libwayland-egl entry points, resolved by LoadEGL. The library is a
// separate file from libwayland-client, and only a GPU client needs it.
var (
	fnEGLWindowCreate  uintptr
	fnEGLWindowResize  uintptr
	fnEGLWindowDestroy uintptr

	eglOnce sync.Once
	eglErr  error
)

// LoadEGL opens libwayland-egl. It runs once; later calls return the first
// result. NewEGLWindow calls it.
func LoadEGL() error {
	eglOnce.Do(func() {
		lib, err := purego.Dlopen("libwayland-egl.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			eglErr = fmt.Errorf("dlopen libwayland-egl.so.1: %w", err)
			return
		}
		for _, s := range []struct {
			fn   *uintptr
			name string
		}{
			{&fnEGLWindowCreate, "wl_egl_window_create"},
			{&fnEGLWindowResize, "wl_egl_window_resize"},
			{&fnEGLWindowDestroy, "wl_egl_window_destroy"},
		} {
			addr, symErr := purego.Dlsym(lib, s.name)
			if symErr != nil || addr == 0 {
				eglErr = fmt.Errorf("libwayland-egl has no %s: %v", s.name, symErr)
				return
			}
			*s.fn = addr
		}
	})
	return eglErr
}

// EGLWindow is a wl_egl_window: the native window EGL renders into. It
// gives a wl_surface a size in buffer pixels, which EGL reads when it
// allocates the next back buffer.
type EGLWindow struct{ ptr uintptr }

// NewEGLWindow creates an EGL window for s, width × height buffer pixels.
func NewEGLWindow(s Surface, width, height int32) (EGLWindow, error) {
	if err := LoadEGL(); err != nil {
		return EGLWindow{}, err
	}
	if width <= 0 || height <= 0 {
		return EGLWindow{}, fmt.Errorf("wl_egl_window_create: bad size %dx%d", width, height)
	}
	r, _, _ := purego.SyscallN(fnEGLWindowCreate, s.ptr, uintptr(width), uintptr(height))
	if r == 0 {
		return EGLWindow{}, fmt.Errorf("wl_egl_window_create(%dx%d) failed", width, height)
	}
	return EGLWindow{ptr: r}, nil
}

// Ptr is the wl_egl_window* to pass to eglCreateWindowSurface.
func (e EGLWindow) Ptr() uintptr { return e.ptr }

// Valid reports whether the window exists.
func (e EGLWindow) Valid() bool { return e.ptr != 0 }

// Resize sets the size of the next buffer EGL allocates. The surface keeps
// its old contents until the next eglSwapBuffers. A size below 1 is raised
// to 1: libwayland-egl ignores a non-positive size and keeps the old one.
func (e EGLWindow) Resize(width, height int32) {
	if e.ptr == 0 {
		return
	}
	_, _, _ = purego.SyscallN(fnEGLWindowResize, e.ptr,
		uintptr(max(width, 1)), uintptr(max(height, 1)), 0, 0)
}

// Destroy frees the window. The EGL surface made from it must be destroyed
// first.
func (e *EGLWindow) Destroy() {
	if e.ptr == 0 {
		return
	}
	_, _, _ = purego.SyscallN(fnEGLWindowDestroy, e.ptr)
	e.ptr = 0
}
