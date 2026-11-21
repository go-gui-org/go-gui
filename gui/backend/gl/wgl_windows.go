//go:build windows && !js

package gl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	pChoosePixelFormat = gdi32.NewProc("ChoosePixelFormat")
	pSetPixelFormat    = gdi32.NewProc("SetPixelFormat")
	pSwapBuffers       = gdi32.NewProc("SwapBuffers")
)

// openGL32Procs is opengl32.dll and the WGL procedures the backend calls
// by name. glProc looks GL 1.1 names up in the same DLL.
type openGL32Procs struct {
	dll            *windows.LazyDLL
	createContext  *windows.LazyProc
	makeCurrent    *windows.LazyProc
	deleteContext  *windows.LazyProc
	getProcAddress *windows.LazyProc
}

var (
	openGL32Once sync.Once
	openGL32Lib  openGL32Procs
)

// openGL32 picks opengl32.dll on first use and returns it with its WGL
// procedures. It is not a package variable initializer, so a process that
// never asks for GL never loads the DLL: with GOGUI_RENDERER=soft a local
// Mesa is not even mapped (#1310).
func openGL32() *openGL32Procs {
	openGL32Once.Do(func() {
		dll := initOpenGL32()
		openGL32Lib = openGL32Procs{
			dll:            dll,
			createContext:  dll.NewProc("wglCreateContext"),
			makeCurrent:    dll.NewProc("wglMakeCurrent"),
			deleteContext:  dll.NewProc("wglDeleteContext"),
			getProcAddress: dll.NewProc("wglGetProcAddress"),
		}
	})
	return &openGL32Lib
}

// initOpenGL32 loads opengl32.dll.  It first tries the executable's
// directory (so that a Mesa software-rendering DLL placed alongside
// the binary on headless CI runners is discovered), then falls back to
// the restricted System32 search for security.
func initOpenGL32() *windows.LazyDLL {
	if exe, err := os.Executable(); err == nil {
		local := filepath.Join(filepath.Dir(exe), "opengl32.dll")
		dll := windows.NewLazyDLL(local)
		if err := dll.Load(); err == nil {
			return dll
		}
	}
	return windows.NewLazySystemDLL("opengl32.dll")
}

// newGLWin32 is New's GL attempt: the window, then a WGL context on its
// kept DC, the GL function table and the GL resources. The window setup's
// degradation reports go to rep. commit, when not nil, runs once the last
// step that can fail has succeeded, just before the window attaches to w,
// where application code can first run (openAutoWin32).
//
// fallback is true when the error is one the first window may fall back on
// (#1310): the pixel format, context or make-current step, or the binding.
// Up to the binding the attempt publishes nothing to w, so plat.destroy
// discards it whole: no GL resource exists yet, the window was never shown,
// and wndProc routed none of its messages to b, because plat.w is unset.
func newGLWin32(w *gui.Window, rep *windowReports, commit func()) (b *Backend, fallback bool, err error) {
	// Pick opengl32.dll before any pixel-format call, as the package
	// variable initializer did. A Mesa beside the exe that is first loaded
	// after SetPixelFormat gets no context from wglCreateContext.
	openGL32()

	b, err = openWin32Window(w, false, rep)
	if err != nil {
		return nil, false, err
	}
	hwnd := b.plat.hwnd
	cfg := w.Config

	hdc, _, _ := pGetDC.Call(hwnd)
	if hdc == 0 {
		b.plat.destroy()
		return nil, false, errors.New("gl: GetDC failed")
	}
	b.plat.hdc = hdc

	hglrc, cerr := createContextFunc(hdc)
	if cerr != nil {
		b.plat.destroy()
		return nil, true, newGPUContextError("create WGL context", cerr)
	}
	b.plat.hglrc = hglrc

	// A legacy context that createContext kept (GDI Generic 1.1) cannot
	// bind the 3.3 functions: a GPU-context failure, as below (#1310).
	if berr := gogl.InitWithProcAddrFunc(glProc); berr != nil {
		gerr := glBringUpError("bind GL functions", berr)
		b.plat.destroy()
		return nil, true, gerr
	}
	// createContext leaves the context current on hdc, which is what
	// wglSwapIntervalEXT applies to. After the binding, the last step that
	// can fall back, so a discarded attempt reports no swap interval.
	applyVSync(cfg.VSyncOff, wglSetSwapInterval, w.DebugWindowVSync,
		"wglSwapIntervalEXT")

	b.dpiScale = windowScale(hwnd)
	b.physW, b.physH = clientSize(hwnd)
	b.initCaches(cfg)

	if ierr := b.prepareGLResources(); ierr != nil {
		gerr := fmt.Errorf("gl: initGLResources: %w", ierr)
		if perr, ok := errors.AsType[*pipelineInitError](ierr); ok {
			gerr = glBringUpError("build GL pipelines", perr.err)
		}
		b.Destroy()
		return nil, false, gerr
	}
	// Nothing below fails.
	if commit != nil {
		commit()
	}
	b.attachGLWindow(w)

	finishWin32Window(w, b)
	return b, false, nil
}

type pixelFormatDescriptor struct {
	nSize           uint16
	nVersion        uint16
	dwFlags         uint32
	iPixelType      byte
	cColorBits      byte
	cRedBits        byte
	cRedShift       byte
	cGreenBits      byte
	cGreenShift     byte
	cBlueBits       byte
	cBlueShift      byte
	cAlphaBits      byte
	cAlphaShift     byte
	cAccumBits      byte
	cAccumRedBits   byte
	cAccumGreenBits byte
	cAccumBlueBits  byte
	cAccumAlphaBits byte
	cDepthBits      byte
	cStencilBits    byte
	cAuxBuffers     byte
	iLayerType      byte
	bReserved       byte
	dwLayerMask     uint32
	dwVisibleMask   uint32
	dwDamageMask    uint32
}

const (
	pfdDrawToWindow = 0x00000004
	pfdSupportOGL   = 0x00000020
	pfdDoubleBuffer = 0x00000001
	pfdTypeRGBA     = 0

	// wglCreateContextAttribsARB attribute keys/values.
	wglContextMajorVersionARB = 0x2091
	wglContextMinorVersionARB = 0x2092
	wglContextFlagsARB        = 0x2094
	wglContextProfileMaskARB  = 0x9126
	wglContextCoreProfileARB  = 0x00000001
	wglContextFwdCompatARB    = 0x00000002
)

func wglMakeCurrent(hdc, hglrc uintptr) {
	openGL32().makeCurrent.Call(hdc, hglrc)
}

func wglDeleteContext(hglrc uintptr) {
	openGL32().deleteContext.Call(hglrc)
}

func swapBuffers(hdc uintptr) {
	pSwapBuffers.Call(hdc)
}

func basePixelFormat() pixelFormatDescriptor {
	var pfd pixelFormatDescriptor
	pfd.nSize = uint16(unsafe.Sizeof(pfd))
	pfd.nVersion = 1
	pfd.dwFlags = pfdDrawToWindow | pfdSupportOGL | pfdDoubleBuffer
	pfd.iPixelType = pfdTypeRGBA
	pfd.cColorBits = 32
	pfd.cAlphaBits = 8
	pfd.cDepthBits = 24
	pfd.cStencilBits = 8
	return pfd
}

// createContextFunc is the seam the GPU-context error tests
// override: assigning a failing func exercises New's wrap path
// with no GPU driver and no GL.
var createContextFunc = createContext

// createContext sets a pixel format on hdc and creates an OpenGL 3.3
// core-profile context via wglCreateContextAttribsARB, falling back to
// a legacy context if the ARB extension is unavailable. On success the
// returned context is current on hdc.
func createContext(hdc uintptr) (uintptr, error) {
	pfd := basePixelFormat()
	pf, _, err := pChoosePixelFormat.Call(hdc, uintptr(unsafe.Pointer(&pfd)))
	if pf == 0 {
		return 0, fmt.Errorf("ChoosePixelFormat: %w", err)
	}
	return createContextWithFormat(hdc, pf, &pfd)
}

// createContextWithFormat is createContext after the pixel format is
// chosen: it sets pf on hdc and makes the context. Tests call it with an
// enumerated format, to get the generic implementation on any machine.
func createContextWithFormat(hdc, pf uintptr, pfd *pixelFormatDescriptor) (uintptr, error) {
	if r, _, e := pSetPixelFormat.Call(hdc, pf, uintptr(unsafe.Pointer(pfd))); r == 0 {
		return 0, fmt.Errorf("SetPixelFormat: %w", e)
	}

	wgl := openGL32()
	// Legacy context first: required both as a fallback and to load
	// the wglCreateContextAttribsARB entry point.
	legacy, _, err := wgl.createContext.Call(hdc)
	if legacy == 0 {
		return 0, fmt.Errorf("wglCreateContext: %w", err)
	}
	if r, _, e := wgl.makeCurrent.Call(hdc, legacy); r == 0 {
		wgl.deleteContext.Call(legacy)
		return 0, fmt.Errorf("wglMakeCurrent(legacy): %w", e)
	}

	createAttribs := wglProc("wglCreateContextAttribsARB")
	if createAttribs == 0 {
		// No ARB path; keep the legacy context.
		return legacy, nil
	}

	attribs := []int32{
		wglContextMajorVersionARB, 3,
		wglContextMinorVersionARB, 3,
		wglContextProfileMaskARB, wglContextCoreProfileARB,
		wglContextFlagsARB, wglContextFwdCompatARB,
		0,
	}
	core, _, _ := syscall.SyscallN(createAttribs,
		hdc, 0, uintptr(unsafe.Pointer(&attribs[0])))
	if core == 0 {
		// Core creation failed; fall back to the legacy context.
		return legacy, nil
	}

	// Switch to the core context and drop the legacy one.
	wgl.makeCurrent.Call(0, 0)
	wgl.deleteContext.Call(legacy)
	if r, _, e := wgl.makeCurrent.Call(hdc, core); r == 0 {
		wgl.deleteContext.Call(core)
		return 0, fmt.Errorf("wglMakeCurrent(core): %w", e)
	}
	// The swap interval is set by the caller (applyVSync), which can
	// report a refusal to the window.
	return core, nil
}

// wglSetSwapInterval sets the swap interval of the current context. It
// reports false when the driver lacks WGL_EXT_swap_control or refuses
// the value.
func wglSetSwapInterval(n int32) bool {
	fn := wglProc("wglSwapIntervalEXT")
	if fn == 0 {
		return false
	}
	r, _, _ := syscall.SyscallN(fn, uintptr(n))
	return r != 0
}

// wglProc resolves a WGL extension entry point. A context must be
// current. Returns 0 if unavailable.
func wglProc(name string) uintptr {
	cname, err := syscall.BytePtrFromString(name)
	if err != nil {
		return 0
	}
	r, _, _ := openGL32().getProcAddress.Call(uintptr(unsafe.Pointer(cname)))
	// wglGetProcAddress may return 1,2,3,-1 for error sentinels.
	switch r {
	case 0, 1, 2, 3, ^uintptr(0):
		return 0
	}
	return r
}

// glVersion is GL_VERSION from gl.h.
const glVersion = 0x1F02

// glIdentity reads GL_VERSION and GL_RENDERER from the current context,
// for the message of a GL bring-up failure. glGetString is GL 1.1, so it
// resolves from the opengl32.dll exports even where glbind bound nothing.
// Only failure paths call it: a context that comes up makes no extra call.
func glIdentity() string {
	getString := glProc("glGetString")
	if getString == 0 {
		return "glGetString unavailable"
	}
	read := func(name uintptr) string {
		r, _, _ := syscall.SyscallN(getString, name)
		return cGoString(r)
	}
	return fmt.Sprintf("GL_VERSION %q, GL_RENDERER %q",
		read(glVersion), read(glRenderer))
}

// glBringUpError reports a GL that came up but cannot run the backend as
// a GPU-context failure, naming GL_VERSION and GL_RENDERER. The failed
// window's context must still be current: call it before the destroy.
func glBringUpError(op string, cause error) error {
	return newGPUContextError(op, fmt.Errorf("%w; %s", cause, glIdentity()))
}

// glProc resolves an OpenGL entry point for glbind.InitWithProcAddrFunc.
// A context must be current.
//
// It reproduces what go-gl's C loader did: wglGetProcAddress only returns
// GL 1.2+ and extension entry points, while the GL 1.1 core subset
// (glClear, glViewport, glTexImage2D, ...) is exported directly by
// opengl32.dll and has to be looked up there instead.
func glProc(name string) uintptr {
	if p := wglProc(name); p != 0 {
		return p
	}
	proc := openGL32().dll.NewProc(name)
	if err := proc.Find(); err != nil {
		return 0
	}
	return proc.Addr()
}
