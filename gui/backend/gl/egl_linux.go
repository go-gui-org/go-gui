//go:build linux && !js && !android

package gl

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// EGL enum values (from egl.h / eglext.h).
const (
	eglDefaultDisplay = 0
	eglNoContext      = 0

	eglOpenGLAPI = 0x30A2
	eglOpenGLBit = 0x0008

	eglSurfaceType    = 0x3033
	eglWindowBit      = 0x0004
	eglRenderableType = 0x3040
	eglRedSize        = 0x3024
	eglGreenSize      = 0x3023
	eglBlueSize       = 0x3022
	eglAlphaSize      = 0x3021
	eglDepthSize      = 0x3025
	eglStencilSize    = 0x3026
	eglNativeVisualID = 0x302E
	eglNone           = 0x3038

	eglContextMajorVersion         = 0x3098
	eglContextMinorVersion         = 0x30FB
	eglContextOpenGLProfileMask    = 0x30FD
	eglContextOpenGLCoreProfileBit = 0x00000001
)

var (
	eglOnce    sync.Once
	eglLoadErr error

	eglGetDisplay          func(uintptr) uintptr
	eglInitialize          func(uintptr, unsafe.Pointer, unsafe.Pointer) uint32
	eglBindAPI             func(uint32) uint32
	eglChooseConfig        func(uintptr, unsafe.Pointer, unsafe.Pointer, int32, unsafe.Pointer) uint32
	eglGetConfigAttrib     func(uintptr, uintptr, int32, unsafe.Pointer) uint32
	eglCreateWindowSurface func(uintptr, uintptr, uintptr, unsafe.Pointer) uintptr
	eglCreateContext       func(uintptr, uintptr, uintptr, unsafe.Pointer) uintptr
	eglMakeCurrent         func(uintptr, uintptr, uintptr, uintptr) uint32
	eglSwapBuffers         func(uintptr, uintptr) uint32
	eglSwapInterval        func(uintptr, int32) uint32
	eglDestroyContext      func(uintptr, uintptr) uint32
	eglDestroySurface      func(uintptr, uintptr) uint32
	eglTerminate           func(uintptr) uint32
	eglGetProcAddress      func(string) unsafe.Pointer
	eglGetError            func() int32
)

// loadEGL dlopens libEGL (and libGL for desktop-GL symbol fallback) and
// binds the EGL entry points. Pure Go via purego — no cgo. Idempotent.
func loadEGL() error {
	eglOnce.Do(func() {
		libEGL, err := purego.Dlopen("libEGL.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			eglLoadErr = fmt.Errorf("dlopen libEGL.so.1: %w", err)
			return
		}

		reg := func(p any, name string) { purego.RegisterLibFunc(p, libEGL, name) }
		reg(&eglGetDisplay, "eglGetDisplay")
		reg(&eglInitialize, "eglInitialize")
		reg(&eglBindAPI, "eglBindAPI")
		reg(&eglChooseConfig, "eglChooseConfig")
		reg(&eglGetConfigAttrib, "eglGetConfigAttrib")
		reg(&eglCreateWindowSurface, "eglCreateWindowSurface")
		reg(&eglCreateContext, "eglCreateContext")
		reg(&eglMakeCurrent, "eglMakeCurrent")
		reg(&eglSwapBuffers, "eglSwapBuffers")
		reg(&eglSwapInterval, "eglSwapInterval")
		reg(&eglDestroyContext, "eglDestroyContext")
		reg(&eglDestroySurface, "eglDestroySurface")
		reg(&eglTerminate, "eglTerminate")
		reg(&eglGetProcAddress, "eglGetProcAddress")
		reg(&eglGetError, "eglGetError")
	})
	return eglLoadErr
}

// eglProc resolves an OpenGL function pointer for
// gl.InitWithProcAddrFunc via eglGetProcAddress. EGL 1.5 (Mesa) returns
// core desktop-GL entry points, not only extensions.
func eglProc(name string) unsafe.Pointer {
	return eglGetProcAddress(name)
}

// eglInitDisplay initializes EGL, binds the desktop-OpenGL API, and
// chooses a framebuffer config. It returns the display, the config, and
// the X visual id the window must be created with so the surface
// matches. EGL opens its own X connection from $DISPLAY.
func eglInitDisplay() (dpy, config uintptr, visualID uint32, err error) {
	if err = loadEGL(); err != nil {
		return
	}
	dpy = eglGetDisplay(eglDefaultDisplay)
	if dpy == 0 {
		err = errors.New("eglGetDisplay: no display")
		return
	}
	var maj, minr int32
	if eglInitialize(dpy, unsafe.Pointer(&maj), unsafe.Pointer(&minr)) == 0 {
		err = fmt.Errorf("eglInitialize failed (egl error 0x%x)", eglGetError())
		return
	}
	if eglBindAPI(eglOpenGLAPI) == 0 {
		err = fmt.Errorf("eglBindAPI(OpenGL) failed (egl error 0x%x) — "+
			"desktop GL over EGL unsupported by this driver", eglGetError())
		return
	}
	attribs := []int32{
		eglSurfaceType, eglWindowBit,
		eglRenderableType, eglOpenGLBit,
		eglRedSize, 8,
		eglGreenSize, 8,
		eglBlueSize, 8,
		eglAlphaSize, 8,
		eglDepthSize, 24,
		eglStencilSize, 8,
		eglNone,
	}
	var cfg uintptr
	var n int32
	ok := eglChooseConfig(dpy, unsafe.Pointer(&attribs[0]),
		unsafe.Pointer(&cfg), 1, unsafe.Pointer(&n))
	if ok == 0 || n == 0 {
		err = fmt.Errorf("eglChooseConfig: no matching config (egl error 0x%x)", eglGetError())
		return
	}
	var vid int32
	eglGetConfigAttrib(dpy, cfg, eglNativeVisualID, unsafe.Pointer(&vid))
	config = cfg
	visualID = uint32(vid)
	return
}

// eglCreateSurfaceContext creates a window surface for an
// already-realized X window and an OpenGL 3.3 core context, then makes
// them current.
func eglCreateSurfaceContext(dpy, config uintptr, win uint32) (surface, context uintptr, err error) {
	surface = eglCreateWindowSurface(dpy, config, uintptr(win), nil)
	if surface == 0 {
		err = fmt.Errorf("eglCreateWindowSurface failed (egl error 0x%x)", eglGetError())
		return
	}
	ctxAttribs := []int32{
		eglContextMajorVersion, 3,
		eglContextMinorVersion, 3,
		eglContextOpenGLProfileMask, eglContextOpenGLCoreProfileBit,
		eglNone,
	}
	context = eglCreateContext(dpy, config, eglNoContext, unsafe.Pointer(&ctxAttribs[0]))
	if context == 0 {
		err = fmt.Errorf("eglCreateContext(3.3 core) failed (egl error 0x%x)", eglGetError())
		return
	}
	if eglMakeCurrent(dpy, surface, surface, context) == 0 {
		err = fmt.Errorf("eglMakeCurrent failed (egl error 0x%x)", eglGetError())
		return
	}
	eglSwapInterval(dpy, 1)
	return
}
