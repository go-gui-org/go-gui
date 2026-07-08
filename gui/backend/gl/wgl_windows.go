//go:build windows && !js

package gl

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	opengl32 = windows.NewLazySystemDLL("opengl32.dll")

	pChoosePixelFormat = gdi32.NewProc("ChoosePixelFormat")
	pSetPixelFormat    = gdi32.NewProc("SetPixelFormat")
	pSwapBuffers       = gdi32.NewProc("SwapBuffers")

	pWglCreateContext  = opengl32.NewProc("wglCreateContext")
	pWglMakeCurrent    = opengl32.NewProc("wglMakeCurrent")
	pWglDeleteContext  = opengl32.NewProc("wglDeleteContext")
	pWglGetProcAddress = opengl32.NewProc("wglGetProcAddress")
)

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
	pWglMakeCurrent.Call(hdc, hglrc)
}

func wglDeleteContext(hglrc uintptr) {
	pWglDeleteContext.Call(hglrc)
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
	if r, _, e := pSetPixelFormat.Call(hdc, pf, uintptr(unsafe.Pointer(&pfd))); r == 0 {
		return 0, fmt.Errorf("SetPixelFormat: %w", e)
	}

	// Legacy context first: required both as a fallback and to load
	// the wglCreateContextAttribsARB entry point.
	legacy, _, err := pWglCreateContext.Call(hdc)
	if legacy == 0 {
		return 0, fmt.Errorf("wglCreateContext: %w", err)
	}
	if r, _, e := pWglMakeCurrent.Call(hdc, legacy); r == 0 {
		pWglDeleteContext.Call(legacy)
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
	pWglMakeCurrent.Call(0, 0)
	pWglDeleteContext.Call(legacy)
	if r, _, e := pWglMakeCurrent.Call(hdc, core); r == 0 {
		pWglDeleteContext.Call(core)
		return 0, fmt.Errorf("wglMakeCurrent(core): %w", e)
	}

	if swapInterval := wglProc("wglSwapIntervalEXT"); swapInterval != 0 {
		syscall.SyscallN(swapInterval, 1) // enable vsync
	}
	return core, nil
}

// wglProc resolves a WGL extension entry point. A context must be
// current. Returns 0 if unavailable.
func wglProc(name string) uintptr {
	cname, err := syscall.BytePtrFromString(name)
	if err != nil {
		return 0
	}
	r, _, _ := pWglGetProcAddress.Call(uintptr(unsafe.Pointer(cname)))
	// wglGetProcAddress may return 1,2,3,-1 for error sentinels.
	switch r {
	case 0, 1, 2, 3, ^uintptr(0):
		return 0
	}
	return r
}
