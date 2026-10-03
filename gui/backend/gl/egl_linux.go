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

	// eglPlatformX11KHR selects the X11 platform in eglGetPlatformDisplay
	// (EGL_KHR_platform_x11 / EGL 1.5).
	eglPlatformX11KHR = 0x31D5
	// eglPlatformWaylandKHR selects the Wayland platform
	// (EGL_KHR_platform_wayland); the native display is a wl_display*.
	eglPlatformWaylandKHR = 0x31D8

	eglContextMajorVersion         = 0x3098
	eglContextMinorVersion         = 0x30FB
	eglContextOpenGLProfileMask    = 0x30FD
	eglContextOpenGLCoreProfileBit = 0x00000001
)

var (
	eglOnce    sync.Once
	eglLoadErr error

	eglGetDisplay          func(uintptr) uintptr
	eglGetPlatformDisplay  func(uint32, uintptr, unsafe.Pointer) uintptr // nil before EGL 1.5
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
		// eglGetPlatformDisplay is EGL 1.5. An older libEGL lacks it, and
		// RegisterLibFunc panics on a missing symbol, so it is optional:
		// without it there is no Mesa X11 fallback (#916).
		if sym, symErr := purego.Dlsym(libEGL, "eglGetPlatformDisplay"); symErr == nil && sym != 0 {
			purego.RegisterFunc(&eglGetPlatformDisplay, sym)
		}
	})
	return eglLoadErr
}

// eglProc resolves an OpenGL function pointer for
// glbind.InitWithProcAddrFunc via eglGetProcAddress. EGL 1.5 (Mesa) returns
// core desktop-GL entry points, not only extensions. The result is a driver
// code address, so uintptr — not unsafe.Pointer — is the correct spelling.
func eglProc(name string) uintptr {
	return uintptr(eglGetProcAddress(name))
}

// eglConfigVisual pairs an EGL framebuffer config with the X visual id
// a window using it must be created with.
type eglConfigVisual struct {
	config   uintptr
	visualID uint32
}

// eglMaxConfigs caps how many candidates eglInitDisplayN collects. A
// transparent window needs a config whose native visual has depth 32,
// and drivers list the depth-24 ones first, so one config is not
// enough. Beyond a handful the extras are colour-depth variants that
// pickVisual would reject anyway.
const eglMaxConfigs = 32

// eglInitDisplayNFunc and eglCreateSurfaceContextFunc are the
// seam the GPU-context error test overrides: assigning a failing
// func exercises New's wrap path with no display and no GL.
var (
	eglInitDisplayNFunc         = eglInitDisplayN
	eglCreateSurfaceContextFunc = eglCreateSurfaceContext
)

// errNoDesktopGLConfig marks an EGL display that initializes but offers
// no desktop-OpenGL window config. The usual cause is a driver with
// OpenGL ES only, for example libhybris (Android GPU drivers) on a Linux
// phone (#916). eglInitDisplayN then tries the Mesa X11 platform display.
var errNoDesktopGLConfig = errors.New("no desktop OpenGL config on this EGL display " +
	"(the driver offers OpenGL ES only?) — go-gui needs desktop OpenGL 3.3")

// eglInitDisplayN initializes EGL, binds the desktop-OpenGL API, and
// returns every matching framebuffer config in the driver's own
// preference order, each paired with the X visual id a window using it
// must be created with so the surface matches. EGL opens its own X
// connection from $DISPLAY. The slice is never empty on a nil error.
//
// The default display comes first. When it has no desktop-GL config,
// the X11 platform display is tried next. Under libglvnd these can be
// different vendors: on a libhybris phone the default display is the
// GL ES-only Android driver, and the X11 platform display is Mesa
// (llvmpipe), which does offer desktop GL (#916).
func eglInitDisplayN() (uintptr, []eglConfigVisual, error) {
	if err := loadEGL(); err != nil {
		return 0, nil, err
	}
	openers := []func() (uintptr, []eglConfigVisual, error){
		func() (uintptr, []eglConfigVisual, error) {
			return eglOpenDisplay(eglGetDisplay(eglDefaultDisplay), "eglGetDisplay")
		},
	}
	if eglGetPlatformDisplay != nil {
		openers = append(openers, func() (uintptr, []eglConfigVisual, error) {
			// EGL_DEFAULT_DISPLAY as the native display makes EGL open
			// its own X connection from $DISPLAY, as eglGetDisplay does.
			return eglOpenDisplay(
				eglGetPlatformDisplay(eglPlatformX11KHR, eglDefaultDisplay, nil),
				"eglGetPlatformDisplay(X11)")
		})
	}
	return eglFirstDesktopGL(openers, func(dpy uintptr) { eglTerminate(dpy) })
}

// eglFirstDesktopGL runs open in order and returns the first display
// that has desktop-GL configs. Only errNoDesktopGLConfig moves on to
// the next opener. Any other error (no display, eglInitialize failure)
// returns at once, so a fallback cannot hide it. A display that was
// rejected is released. When every opener fails, the error keeps the
// first cause, which names OpenGL ES, and adds the last fallback
// failure. Separate from eglInitDisplayN so a test can drive it with
// no EGL library.
func eglFirstDesktopGL(open []func() (uintptr, []eglConfigVisual, error),
	release func(uintptr)) (uintptr, []eglConfigVisual, error) {
	var first, last error
	for _, fn := range open {
		dpy, cands, err := fn()
		if err == nil {
			return dpy, cands, nil
		}
		if dpy != 0 {
			release(dpy)
		}
		if first == nil {
			if !errors.Is(err, errNoDesktopGLConfig) {
				return 0, nil, err
			}
			first = err
			continue
		}
		last = err
		if !errors.Is(err, errNoDesktopGLConfig) {
			break
		}
	}
	if last != nil {
		return 0, nil, fmt.Errorf("%w; fallback: %v", first, last)
	}
	if first == nil {
		// No openers: report it, or the caller gets a nil error and an
		// empty config list, which breaks eglInitDisplayN's contract.
		return 0, nil, errors.New("egl: no display to open")
	}
	return 0, nil, first
}

// eglOpenDisplay initializes dpy, binds the desktop-OpenGL API, and
// collects its desktop-GL window configs. getter names the call that
// produced dpy, for the error. On an error after eglInitialize, dpy is
// returned non-zero so the caller can release it.
func eglOpenDisplay(dpy uintptr, getter string) (uintptr, []eglConfigVisual, error) {
	if dpy == 0 {
		return 0, nil, fmt.Errorf("%s: no display", getter)
	}
	var maj, minr int32
	if eglInitialize(dpy, unsafe.Pointer(&maj), unsafe.Pointer(&minr)) == 0 {
		return 0, nil, fmt.Errorf("eglInitialize failed (egl error 0x%x)", eglGetError())
	}
	if eglBindAPI(eglOpenGLAPI) == 0 {
		// A GL ES-only driver can refuse the desktop API here instead
		// of at eglChooseConfig. Both mean the same thing.
		return dpy, nil, fmt.Errorf("%s: eglBindAPI(OpenGL) failed (egl error 0x%x): %w",
			getter, eglGetError(), errNoDesktopGLConfig)
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
	cfgs := make([]uintptr, eglMaxConfigs)
	var n int32
	ok := eglChooseConfig(dpy, unsafe.Pointer(&attribs[0]),
		unsafe.Pointer(&cfgs[0]), int32(len(cfgs)), unsafe.Pointer(&n))
	if ok == 0 {
		return dpy, nil, fmt.Errorf("eglChooseConfig failed (egl error 0x%x)", eglGetError())
	}
	if n <= 0 {
		// The call worked and matched nothing: egl error is
		// EGL_SUCCESS (0x3000). On #916 this was a GL ES-only display.
		return dpy, nil, fmt.Errorf("%s: eglChooseConfig: %w", getter, errNoDesktopGLConfig)
	}
	// A misbehaving driver must not push n past the buffer it was
	// given; cfgs[:n] below would panic.
	if int(n) > len(cfgs) {
		n = int32(len(cfgs))
	}
	cands := make([]eglConfigVisual, 0, n)
	for _, cfg := range cfgs[:n] {
		var vid int32
		eglGetConfigAttrib(dpy, cfg, eglNativeVisualID, unsafe.Pointer(&vid))
		cands = append(cands, eglConfigVisual{config: cfg, visualID: uint32(vid)})
	}
	return dpy, cands, nil
}

// eglCreateSurfaceContext creates a window surface and an OpenGL 3.3
// core context, then makes them current. win is the native window: an
// already-realized X window id, or a wl_egl_window pointer on Wayland.
func eglCreateSurfaceContext(dpy, config, win uintptr) (surface, context uintptr, err error) {
	surface = eglCreateWindowSurface(dpy, config, win, nil)
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
