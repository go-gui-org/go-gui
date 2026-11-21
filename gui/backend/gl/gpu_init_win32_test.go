//go:build windows && !js

package gl

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
)

// TestNewWrapsGPUContextError drives New with a failing WGL context
// creation: no GPU driver and no GL needed, only window creation,
// which works headless.
func TestNewWrapsGPUContextError(t *testing.T) {
	t.Setenv("GOGUI_RENDERER", "gl")
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

// Pixel format flags from wingdi.h that mark Microsoft's generic
// implementation.
const (
	pfdGenericFormat      = 0x00000040
	pfdGenericAccelerated = 0x00001000
)

var pDescribePixelFormat = gdi32.NewProc("DescribePixelFormat")

// genericPixelFormat returns the first double-buffered RGBA window
// format that hdc offers from Microsoft's generic implementation
// ("GDI Generic", OpenGL 1.1): PFD_GENERIC_FORMAT set and
// PFD_GENERIC_ACCELERATED clear. It is 0 when there is none.
func genericPixelFormat(hdc uintptr) (uintptr, pixelFormatDescriptor) {
	var pfd pixelFormatDescriptor
	size := unsafe.Sizeof(pfd)
	n, _, _ := pDescribePixelFormat.Call(hdc, 1, size,
		uintptr(unsafe.Pointer(&pfd)))
	const want = pfdDrawToWindow | pfdSupportOGL | pfdDoubleBuffer
	for i := uintptr(1); i <= n; i++ {
		if r, _, _ := pDescribePixelFormat.Call(hdc, i, size,
			uintptr(unsafe.Pointer(&pfd))); r == 0 {
			continue
		}
		if pfd.dwFlags&want == want &&
			pfd.dwFlags&pfdGenericFormat != 0 &&
			pfd.dwFlags&pfdGenericAccelerated == 0 &&
			pfd.iPixelType == pfdTypeRGBA && pfd.cColorBits >= 24 {
			return i, pfd
		}
	}
	return 0, pixelFormatDescriptor{}
}

// TestGDIGenericIsGPUContextError is the regression test for #1310: on
// Microsoft's generic OpenGL 1.1, createContext keeps the legacy context
// and succeeds, the GL binding then fails, and New used to return a
// plain error that made Run panic. Selecting the generic pixel format
// gives that context on a machine with a working driver too.
func TestGDIGenericIsGPUContextError(t *testing.T) {
	// A local opengl32.dll (Mesa, as glrendertest copies it) replaces
	// the system DLL for the whole process; it has no GDI Generic.
	if lib := openGL32().dll; !lib.System {
		t.Skipf("%s is loaded, not the system opengl32.dll", lib.Name)
	}
	t.Setenv("GOGUI_RENDERER", "gl")
	reached, found, made := false, false, false
	old := createContextFunc
	createContextFunc = func(hdc uintptr) (uintptr, error) {
		reached = true
		pf, pfd := genericPixelFormat(hdc)
		if pf == 0 {
			return 0, errors.New("test: no generic pixel format")
		}
		found = true
		hglrc, cerr := createContextWithFormat(hdc, pf, &pfd)
		made = cerr == nil
		return hglrc, cerr
	}
	defer func() { createContextFunc = old }()

	newOnGeneric := func() error {
		w := gui.NewWindow(gui.WindowCfg{
			State:  new(int),
			Width:  200,
			Height: 200,
		})
		b, nerr := New(w)
		if nerr == nil {
			b.Destroy()
			t.Fatal("New on GDI Generic returned nil error")
		}
		return nerr
	}
	err := newOnGeneric()
	if !reached {
		t.Skipf("no window, seam not reached: %v", err)
	}
	if !found {
		t.Fatal("the system opengl32.dll offers no generic pixel format")
	}
	// Without a display driver the first WGL context a process asks for
	// can be refused ("The pixel format is invalid"), and the next one is
	// GDI Generic: seen on Windows 11 24H2 in a VM, on the console and
	// over RDP. Only a context that was made can reach the binding.
	if !made {
		err = newOnGeneric()
	}
	if !made {
		t.Skipf("the generic implementation makes no context here: %v", err)
	}
	if !errors.Is(err, ErrNoGPUContext) {
		t.Fatalf("New error = %v, want ErrNoGPUContext", err)
	}
	for _, want := range []string{"GL_VERSION \"1.1", "GL_RENDERER \"GDI Generic\""} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("message %q misses %q", err, want)
		}
	}
	t.Logf("New: %v", err)
}

// newWithFailingInit runs New on the machine's own GL with a seam set to
// fail. It skips where that GL cannot reach the seam (no window, or no GL
// 3.3, as on a CI runner without Mesa).
func newWithFailingInit(t *testing.T, seam *bool) error {
	t.Helper()
	t.Setenv("GOGUI_RENDERER", "gl")
	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	b, err := New(w)
	if err == nil {
		b.Destroy()
		t.Fatal("New with a failing seam returned nil error")
	}
	if !*seam {
		t.Skipf("seam not reached: %v", err)
	}
	return err
}

// TestPipelineFailureIsGPUContextError: a context that cannot build the
// built-in pipelines cannot run the backend, so New classifies the
// failure and names the GL it came up on.
func TestPipelineFailureIsGPUContextError(t *testing.T) {
	cause := errors.New("test: link failed")
	reached := false
	old := initPipelinesFunc
	initPipelinesFunc = func(*Backend) error {
		reached = true
		return cause
	}
	defer func() { initPipelinesFunc = old }()

	err := newWithFailingInit(t, &reached)
	if !errors.Is(err, ErrNoGPUContext) {
		t.Fatalf("New error = %v, want ErrNoGPUContext", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("New error = %v, lost the cause", err)
	}
	// Each string must hold something: read after the teardown they are
	// empty, because no context is current then.
	for _, want := range []string{`GL_VERSION "[^"]+"`, `GL_RENDERER "[^"]+"`} {
		if !regexp.MustCompile(want).MatchString(err.Error()) {
			t.Fatalf("message %q does not match %s", err, want)
		}
	}
	t.Logf("New: %v", err)
}

// TestTextSystemFailureStaysPlain: a text-system failure is not a GPU
// failure, so it keeps the panic that points at the bug.
func TestTextSystemFailureStaysPlain(t *testing.T) {
	cause := errors.New("test: no text system")
	reached := false
	old := newTextSystemFunc
	newTextSystemFunc = func(glyph.DrawBackend) (*glyph.TextSystem, error) {
		reached = true
		return nil, cause
	}
	defer func() { newTextSystemFunc = old }()

	err := newWithFailingInit(t, &reached)
	if errors.Is(err, ErrNoGPUContext) {
		t.Fatalf("New error = %v, must not match ErrNoGPUContext", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("New error = %v, lost the cause", err)
	}
}
