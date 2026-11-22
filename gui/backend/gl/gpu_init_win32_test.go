//go:build windows && !js

package gl

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-glyph"
	"golang.org/x/sys/windows"

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
	// Only a context that was made can reach the binding. Until #1348 this
	// retried: while the system opengl32.dll loaded after SetPixelFormat,
	// the first WGL context of a process was refused on a machine without
	// a display driver ("The pixel format is invalid."), and the next one
	// was GDI Generic.
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

// firstAttemptChildEnv marks the subprocess TestFirstGLAttemptOfProcess
// starts.
const firstAttemptChildEnv = "GOGUI_TEST_FIRST_GL_ATTEMPT_CHILD"

// TestFirstGLAttemptOfProcess is the regression test for #1348: the system
// opengl32.dll was loaded by the first wglCreateContext call, after
// SetPixelFormat, and that first context of the process was refused ("The
// pixel format is invalid."), on a GPU-less VM and in a VirtualBox guest
// whose OpenGL 4.1 works. Later attempts got the context.
//
// It needs a process with no opengl32.dll loaded; in one where an earlier
// test loaded it, it reruns itself alone in a subprocess. It checks first
// that the DLL openGL32 picked is loaded once it returns, without a call
// that would load it: before #1348 the system DLL was not, on any machine.
// Then, with GOGUI_RENDERER=gl, the first GL attempt of the process must end
// where the second does: both come up, or both fail at the same step. Only
// a machine without a vendor driver showed a difference: a vendor driver
// gives a context at either attempt, and a Mesa beside the binary was
// loaded early already. The outcomes are logged, so the system GL selection
// records what the runner's OpenGL does at the first attempt.
func TestFirstGLAttemptOfProcess(t *testing.T) {
	if openGL32Loaded() {
		if os.Getenv(firstAttemptChildEnv) != "" {
			t.Fatal("opengl32.dll was loaded before the test started, in a process that runs only this test")
		}
		cmd := exec.Command(os.Args[0], "-test.run", "^TestFirstGLAttemptOfProcess$", "-test.v") // #nosec G204 -- this test binary, a fixed test name
		cmd.Env = append(os.Environ(), firstAttemptChildEnv+"=1")
		out, err := cmd.CombinedOutput()
		t.Logf("opengl32.dll already loaded in this process; ran in a subprocess:\n%s", out)
		if err == nil && strings.Contains(string(out), "--- SKIP: TestFirstGLAttemptOfProcess") {
			t.Skip("the subprocess skipped")
		}
		if err != nil || !strings.Contains(string(out), "--- PASS: TestFirstGLAttemptOfProcess") {
			t.Fatalf("subprocess did not pass: %v", err)
		}
		return
	}

	lib := openGL32()
	if lib.loadErr != nil {
		t.Fatalf("load %s: %v", lib.dll.Name, lib.loadErr)
	}
	// GetModuleHandleW, not lib.dll.Handle, which would load the DLL.
	if h, _, _ := pGetModuleHandleW.Call(uintptr(unsafe.Pointer(
		windows.StringToUTF16Ptr(lib.dll.Name)))); h == 0 {
		t.Fatalf("%s (system %v) is not loaded after openGL32 returned: the first pixel-format call would run before it",
			lib.dll.Name, lib.dll.System)
	}

	t.Setenv("GOGUI_RENDERER", "gl")
	attempt := func() (string, error) {
		b, err := New(gui.NewWindow(gui.WindowCfg{State: new(int), Width: 200, Height: 200}))
		if err == nil {
			b.Destroy()
			return "came up", nil
		}
		skipNoWindow(t, err)
		gerr, ok := errors.AsType[*gpuContextError](err)
		if !ok {
			t.Fatalf("New: %v, not a GPU-context failure", err)
		}
		return gerr.op + " failed", err
	}
	first, firstErr := attempt()
	second, secondErr := attempt()
	t.Logf("%s; attempt 1: %s (%v); attempt 2: %s (%v)",
		lib.dll.Name, first, firstErr, second, secondErr)
	if first != second {
		t.Fatalf("the first GL attempt of the process %s, the second %s: %v",
			first, second, firstErr)
	}
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
