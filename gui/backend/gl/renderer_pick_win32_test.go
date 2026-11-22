//go:build windows && !js

package gl

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// Tests of the first-window fallback with GOGUI_RENDERER unset (#1310,
// phase 3). Each trigger is forced through a seam, so they run on a machine
// whose GL works too. They open real windows: a runner that cannot create
// one skips, or fails when GOGUI_REQUIRE_GL is set (tools/glrendertest).
//
// The process renderer is process state. Each test starts from an undecided
// one and leaves it undecided (resetProcessRenderer), so they also pass all
// in one process, in any order.

// fallbackMarker is in the fallback's log line and in no other.
const fallbackMarker = "drawing with the CPU renderer"

// errRefused is the refusal the lab VM gives the first context of a process.
var errRefused = errors.New("wglCreateContext: test: the pixel format is invalid")

// setProcessRenderer forgets or sets the process renderer, with no
// bring-up deciding.
func setProcessRenderer(r win32Renderer) {
	processRenderer.mu.Lock()
	processRenderer.chosen = r
	processRenderer.deciding = false
	processRenderer.mu.Unlock()
}

func chosenRenderer() win32Renderer {
	processRenderer.mu.Lock()
	defer processRenderer.mu.Unlock()
	return processRenderer.chosen
}

// resetProcessRenderer makes the next window with GOGUI_RENDERER unset the
// process's first, now and after the test.
// Every New of the test has returned by its cleanup, so none may still be
// deciding: a return path that left it set would keep every later window
// from setting the choice.
func resetProcessRenderer(t *testing.T) {
	t.Helper()
	setProcessRenderer(win32RendererUndecided)
	t.Cleanup(func() {
		processRenderer.mu.Lock()
		deciding := processRenderer.deciding
		processRenderer.mu.Unlock()
		if deciding {
			t.Error("a returned New left the renderer choice deciding")
		}
		setProcessRenderer(win32RendererUndecided)
	})
}

// logCapture is the standard logger's output during a test. Writes may come
// from other goroutines (go-glyph's warm-up), hence the lock.
type logCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logCapture) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logCapture) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *logCapture) reset() {
	l.mu.Lock()
	l.buf.Reset()
	l.mu.Unlock()
}

// fallbackLines returns the logged lines that announce a fallback.
func (l *logCapture) fallbackLines() []string {
	var lines []string
	for line := range strings.SplitSeq(l.String(), "\n") {
		if strings.Contains(line, fallbackMarker) {
			lines = append(lines, line)
		}
	}
	return lines
}

func captureLog(t *testing.T) *logCapture {
	t.Helper()
	l := &logCapture{}
	prev := log.Writer()
	log.SetOutput(l)
	t.Cleanup(func() { log.SetOutput(prev) })
	return l
}

// newAutoProcess sets up a process whose next window is its first with
// GOGUI_RENDERER unset, and captures the log.
func newAutoProcess(t *testing.T) *logCapture {
	t.Helper()
	t.Setenv("GOGUI_RENDERER", "")
	resetProcessRenderer(t)
	return captureLog(t)
}

// refuseContexts makes every WGL context creation fail, as the lab VM fails
// the first, and returns the number of attempts made.
func refuseContexts(t *testing.T) *int {
	t.Helper()
	calls := new(int)
	old := createContextFunc
	createContextFunc = func(uintptr) (uintptr, error) {
		*calls++
		return 0, errRefused
	}
	t.Cleanup(func() { createContextFunc = old })
	return calls
}

// countSoftSetups counts the soft attempts that reach the CPU renderer's
// setup, and returns the count.
func countSoftSetups(t *testing.T) *int {
	t.Helper()
	calls := new(int)
	old := newSoftPresentWin32Func
	newSoftPresentWin32Func = func(hwnd uintptr, w *gui.Window, scale float32) (*softPresentWin32, error) {
		*calls++
		return old(hwnd, w, scale)
	}
	t.Cleanup(func() { newSoftPresentWin32Func = old })
	return calls
}

func fallbackCfg() gui.WindowCfg {
	return gui.WindowCfg{State: new(int), Width: 200, Height: 200}
}

// skipNoWindow skips when err says no window could be created at all: the
// seams are then never reached.
func skipNoWindow(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "CreateWindowExW") {
		softWin32Skip(t, "no window: %v", err)
	}
}

// openFallback runs New on w, which must succeed, and destroys the backend
// after the test.
func openFallback(t *testing.T, w *gui.Window) *Backend {
	t.Helper()
	b, err := New(w)
	skipNoWindow(t, err)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(b.Destroy)
	t.Cleanup(w.WindowCleanup) // runs first, as in Run
	return b
}

// assertFallback requires that b is a soft window that a fallback opened,
// with one log line that names step and cause.
func assertFallback(t *testing.T, b *Backend, w *gui.Window, l *logCapture, step, cause string) {
	t.Helper()
	if !b.plat.softMode || b.plat.soft == nil || b.plat.hglrc != 0 || b.plat.hdc != 0 {
		t.Fatalf("not a soft window: softMode %v, hglrc %#x, hdc %#x",
			b.plat.softMode, b.plat.hglrc, b.plat.hdc)
	}
	if w.RendersShaders() {
		t.Error("the window reports shaders on the CPU renderer")
	}
	if got := chosenRenderer(); got != win32RendererSoft {
		t.Errorf("process renderer = %d, want soft", got)
	}
	lines := l.fallbackLines()
	if len(lines) != 1 {
		t.Fatalf("%d fallback lines, want 1; log:\n%s", len(lines), l)
	}
	for _, want := range []string{step, cause, "GOGUI_RENDERER=gl", "custom shaders are not drawn"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("fallback line %q misses %q", lines[0], want)
		}
	}
	t.Logf("fallback line: %s", lines[0])
}

// TestFallbackOnRefusedContext: the first window's context is refused, as
// on the lab VM, so it opens on the CPU renderer and logs one line.
func TestFallbackOnRefusedContext(t *testing.T) {
	l := newAutoProcess(t)
	calls := refuseContexts(t)
	w := gui.NewWindow(fallbackCfg())
	b := openFallback(t, w)
	if *calls != 1 {
		t.Errorf("%d context attempts, want 1", *calls)
	}
	assertFallback(t, b, w, l, "create WGL context", errRefused.Error())
}

// TestFallbackOnGDIGeneric: the first window gets Microsoft's generic OpenGL
// 1.1, through the pixel format of phase 1's regression test. The binding
// fails, so the window opens on the CPU renderer and the line names the GL.
func TestFallbackOnGDIGeneric(t *testing.T) {
	// A local opengl32.dll (Mesa) replaces the system DLL for the whole
	// process; it has no GDI Generic.
	if lib := openGL32().dll; !lib.System {
		t.Skipf("%s is loaded, not the system opengl32.dll", lib.Name)
	}
	l := newAutoProcess(t)
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
	t.Cleanup(func() { createContextFunc = old })

	open := func() (*Backend, *gui.Window, error) {
		w := gui.NewWindow(fallbackCfg())
		b, err := New(w)
		return b, w, err
	}
	b, w, err := open()
	if !reached {
		skipNoWindow(t, err)
		t.Skipf("seam not reached: %v", err)
	}
	if !found {
		if b != nil {
			b.Destroy()
		}
		t.Fatal("the system opengl32.dll offers no generic pixel format")
	}
	// Without a display driver the first context of a process can be
	// refused, which falls back on the other trigger. The next one is GDI
	// Generic (phase 1's regression test retries for the same reason).
	if !made {
		if b != nil {
			b.Destroy()
		}
		setProcessRenderer(win32RendererUndecided)
		l.reset()
		b, w, err = open()
	}
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(b.Destroy)
	if !made {
		t.Skipf("the generic implementation makes no context here; log:\n%s", l)
	}
	assertFallback(t, b, w, l, "bind GL functions", `GL_RENDERER "GDI Generic"`)
}

// TestPipelineFailureDoesNotFallBack (decision 2): a GL that comes up but
// cannot build the built-in pipelines returns the classified error. It opens
// no soft window, logs no fallback and does not make the process soft. It
// needs a GL that reaches the pipelines, so it skips without one.
func TestPipelineFailureDoesNotFallBack(t *testing.T) {
	l := newAutoProcess(t)
	soft := countSoftSetups(t)
	cause := errors.New("test: link failed")
	reached := false
	old := initPipelinesFunc
	initPipelinesFunc = func(*Backend) error {
		reached = true
		return cause
	}
	t.Cleanup(func() { initPipelinesFunc = old })

	b, err := New(gui.NewWindow(fallbackCfg()))
	if !reached {
		if b != nil {
			b.Destroy()
		}
		skipNoWindow(t, err)
		t.Skipf("no GL here reaches the pipelines: %v; log:\n%s", err, l)
	}
	if err == nil {
		b.Destroy()
		t.Fatal("a pipeline failure opened a window")
	}
	if !errors.Is(err, ErrNoGPUContext) || !errors.Is(err, cause) {
		t.Fatalf("New error = %v, want ErrNoGPUContext with the cause", err)
	}
	if *soft != 0 {
		t.Errorf("%d soft setups after a pipeline failure, want 0", *soft)
	}
	if lines := l.fallbackLines(); len(lines) != 0 {
		t.Errorf("a pipeline failure logged a fallback: %q", lines)
	}
	if chosenRenderer() == win32RendererSoft {
		t.Error("a pipeline failure made the process soft")
	}
}

// TestRendererGLReportsError: with GOGUI_RENDERER=gl, or a value that is
// neither gl nor soft, a refused context is the classified error, with the
// advice to set GOGUI_RENDERER=soft, and no soft window opens.
func TestRendererGLReportsError(t *testing.T) {
	for _, value := range []string{"gl", "Soft"} {
		t.Run(value, func(t *testing.T) {
			l := captureLog(t)
			resetProcessRenderer(t)
			t.Setenv("GOGUI_RENDERER", value)
			calls := refuseContexts(t)
			soft := countSoftSetups(t)
			b, err := New(gui.NewWindow(fallbackCfg()))
			if err == nil {
				b.Destroy()
				t.Fatal("New with a refused context returned no error")
			}
			skipNoWindow(t, err)
			if !errors.Is(err, ErrNoGPUContext) || !errors.Is(err, errRefused) {
				t.Fatalf("New error = %v, want ErrNoGPUContext with the cause", err)
			}
			if !strings.Contains(err.Error(), "GOGUI_RENDERER=soft") {
				t.Errorf("error %q lost the GOGUI_RENDERER=soft advice", err)
			}
			if *calls != 1 || *soft != 0 {
				t.Errorf("%d context attempts and %d soft setups, want 1 and 0", *calls, *soft)
			}
			if lines := l.fallbackLines(); len(lines) != 0 {
				t.Errorf("logged a fallback: %q", lines)
			}
			if got := chosenRenderer(); got != win32RendererUndecided {
				t.Errorf("GOGUI_RENDERER=%s set the process renderer to %d", value, got)
			}
		})
	}
}

// TestFallbackSoftFailure: the context is refused and the CPU renderer's
// setup fails too. The error is the GPU error first, so it matches
// ErrNoGPUContext, with the soft cause after it and no advice to set
// GOGUI_RENDERER=soft; no fallback is logged.
func TestFallbackSoftFailure(t *testing.T) {
	l := newAutoProcess(t)
	refuseContexts(t)
	softErr := errors.New("test: no CPU renderer")
	old := newSoftPresentWin32Func
	newSoftPresentWin32Func = func(uintptr, *gui.Window, float32) (*softPresentWin32, error) {
		return nil, softErr
	}
	t.Cleanup(func() { newSoftPresentWin32Func = old })

	b, err := New(gui.NewWindow(fallbackCfg()))
	if err == nil {
		b.Destroy()
		t.Fatal("New returned no error when neither renderer can run")
	}
	skipNoWindow(t, err)
	if !errors.Is(err, ErrNoGPUContext) || !errors.Is(err, errRefused) || !errors.Is(err, softErr) {
		t.Fatalf("New error = %v, want ErrNoGPUContext with both causes", err)
	}
	msg := err.Error()
	if strings.Index(msg, errRefused.Error()) > strings.Index(msg, softErr.Error()) {
		t.Errorf("error %q puts the soft cause first", msg)
	}
	if strings.Contains(msg, "GOGUI_RENDERER=soft") {
		t.Errorf("error %q advises soft, which just failed", msg)
	}
	if lines := l.fallbackLines(); len(lines) != 0 {
		t.Errorf("logged a fallback that did not happen: %q", lines)
	}
	t.Logf("New: %v", err)
}

// TestFallbackKeepsHiddenStart: a window the app hid before the backend
// started stays hidden across the fallback, and Show brings it up.
func TestFallbackKeepsHiddenStart(t *testing.T) {
	newAutoProcess(t)
	refuseContexts(t)
	w := gui.NewWindow(fallbackCfg())
	w.Hide()
	b := openFallback(t, w)
	if !b.plat.softMode {
		t.Fatal("not on the CPU renderer")
	}
	if windowStyle(b.plat.hwnd)&wsVisible != 0 || w.IsVisible() {
		t.Fatal("a window hidden before the start was shown after the fallback")
	}
	w.Show()
	if windowStyle(b.plat.hwnd)&wsVisible == 0 {
		t.Error("Show did not bring up the fallback window")
	}
}

// TestFallbackKeepsQueuedPlacement: a size and position asked for before
// the backend started reach the window the fallback opened. They are
// replayed by SetNativePlatform, as posted messages, so had the discarded GL
// window taken them, they would be lost with it.
func TestFallbackKeepsQueuedPlacement(t *testing.T) {
	newAutoProcess(t)
	refuseContexts(t)
	const x, y, width, height = 140, 120, 260, 180
	w := gui.NewWindow(fallbackCfg())
	w.SetPosition(x, y)
	w.SetWindowSize(width, height)
	b := openFallback(t, w)
	if !b.plat.softMode {
		t.Fatal("not on the CPU renderer")
	}
	// As Run does: wndProc routes messages to b only once plat.w is set.
	b.plat.w = w
	w.SetView(softWin32View)
	var msg msgW
	pumpMessages(&msg)
	if p := clientOrigin(b.plat.hwnd); p.x != x || p.y != y {
		t.Errorf("client origin %d,%d, want %d,%d", p.x, p.y, x, y)
	}
	cw, ch := clientSize(b.plat.hwnd)
	if ww, wh := clientExtent(width, b.dpiScale), clientExtent(height, b.dpiScale); cw != ww || ch != wh {
		t.Errorf("client size %dx%d, want %dx%d", cw, ch, ww, wh)
	}
}

// TestFallbackRunsOnInitOnce: Run and RunApp call OnInit once for a window
// that fell back. OnInit closes the window, so the loop ends at once.
func TestFallbackRunsOnInitOnce(t *testing.T) {
	loops := []struct {
		name string
		run  func(*gui.Window) error
	}{
		{"Run", runE},
		{"RunApp", func(w *gui.Window) error { return runAppE(gui.NewApp(), w) }},
	}
	for _, loop := range loops {
		t.Run(loop.name, func(t *testing.T) {
			l := newAutoProcess(t)
			calls := refuseContexts(t)
			inits := 0
			cfg := fallbackCfg()
			cfg.OnInit = func(w *gui.Window) {
				inits++
				w.SetView(softWin32View)
				w.Close()
			}
			err := loop.run(gui.NewWindow(cfg))
			skipNoWindow(t, err)
			if err != nil {
				t.Fatalf("%s: %v", loop.name, err)
			}
			if inits != 1 {
				t.Errorf("OnInit ran %d times, want 1", inits)
			}
			if *calls != 1 || len(l.fallbackLines()) != 1 {
				t.Errorf("%d context attempts and %d fallback lines, want 1 and 1",
					*calls, len(l.fallbackLines()))
			}
		})
	}
}

// reportsChildEnv marks the subprocess TestFallbackPublishesWinningReportsOnly
// starts, and names its case: "framed" or "frameless".
const reportsChildEnv = "GOGUI_TEST_FALLBACK_REPORTS_CHILD"

// The reports TestFallbackPublishesWinningReportsOnly expects, as gui.Debug
// prints them.
const (
	glAttemptReport   = "test: the GL attempt's transparency"
	glAttemptOpacity  = "test: the GL attempt's opacity"
	softTransparency  = "gui: window: Transparent requested but the CPU renderer on Windows draws a window with a system frame opaque; a frameless one (DecorationNone) is transparent"
	anyTransparency   = "gui: window: Transparent requested"
	transparentFadeNo = "gui: window: opacity requested but the window is Transparent"
	// The part of the soft attempt's fade refusal that names its reason,
	// framed and frameless; GL gives a third (fadeRefusal).
	framedFadeNo    = "the CPU renderer draws it opaque because it has a system frame"
	framelessFadeNo = "the CPU renderer shows it with UpdateLayeredWindow"
)

// TestFallbackPublishesWinningReportsOnly: a Transparent window with a fade
// set before the start falls back. The window must get the reports of the
// soft attempt, and none of the GL attempt, whose window was discarded:
// here a transparency and an opacity report the GL attempt alone makes
// (through the enableWindowTransparencyFunc and applyWindowOpacityFunc
// seams). The two attempts refuse the fade with different reasons
// (fadeRefusal: GL names blur-behind), and the seam gives the GL one a
// text no real report has. With a system frame the soft window is opaque
// and reports that; frameless, it is layered and makes no transparency
// report. The fade is refused on both, each with the CPU renderer's reason
// for that kind of window, checked by its distinctive part.
//
// gui.Debug writes to stderr and keeps no record a test outside gui can
// read, so the window is opened in a subprocess and its stderr is checked.
func TestFallbackPublishesWinningReportsOnly(t *testing.T) {
	if os.Getenv(reportsChildEnv) == "" {
		for _, c := range []struct {
			name         string
			transparency int    // the soft attempt's transparency reports
			fade         string // the reason its fade refusal gives
		}{{"framed", 1, framedFadeNo}, {"frameless", 0, framelessFadeNo}} {
			t.Run(c.name, func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run", "^TestFallbackPublishesWinningReportsOnly$", "-test.v") // #nosec G204 -- this test binary, a fixed test name
				cmd.Env = append(os.Environ(), reportsChildEnv+"="+c.name)
				raw, err := cmd.CombinedOutput()
				out := string(raw)
				t.Logf("subprocess:\n%s", out)
				if err == nil && strings.Contains(out, "--- SKIP: TestFallbackPublishesWinningReportsOnly") {
					t.Skip("the subprocess skipped")
				}
				if err != nil || !strings.Contains(out, "--- PASS: TestFallbackPublishesWinningReportsOnly") {
					t.Fatalf("subprocess did not pass: %v", err)
				}
				for _, gl := range []string{glAttemptReport, glAttemptOpacity} {
					if strings.Contains(out, gl) {
						t.Errorf("the discarded GL attempt's report %q reached the window", gl)
					}
				}
				if n := strings.Count(out, anyTransparency); n != c.transparency ||
					strings.Count(out, softTransparency) != c.transparency {
					t.Errorf("%d transparency reports, want %d %q", n, c.transparency, softTransparency)
				}
				if n := strings.Count(out, transparentFadeNo); n != 1 ||
					strings.Count(out, c.fade) != 1 {
					t.Errorf("%d reports %q, want 1, with %q", n, transparentFadeNo, c.fade)
				}
			})
		}
		return
	}

	gui.DebugCategories(gui.DebugWindowDegraded)
	t.Cleanup(func() { gui.DebugCategories(0) })
	l := newAutoProcess(t)
	refuseContexts(t)
	old := enableWindowTransparencyFunc
	enableWindowTransparencyFunc = func(report func(string), _ uintptr) {
		report(glAttemptReport)
	}
	t.Cleanup(func() { enableWindowTransparencyFunc = old })
	// The GL attempt replays the fade first; the soft attempt gets the
	// real refusal.
	oldOpacity := applyWindowOpacityFunc
	opacityCalls := 0
	applyWindowOpacityFunc = func(warn func(string), hwnd uintptr, refusal string, opacity float32) {
		opacityCalls++
		if opacityCalls == 1 {
			warn(glAttemptOpacity)
			return
		}
		oldOpacity(warn, hwnd, refusal, opacity)
	}
	t.Cleanup(func() { applyWindowOpacityFunc = oldOpacity })
	cfg := fallbackCfg()
	cfg.Transparent = true
	frameless := os.Getenv(reportsChildEnv) == "frameless"
	if frameless {
		cfg.Decorations = gui.DecorationNone
	}
	w := gui.NewWindow(cfg)
	w.SetWindowOpacity(0.5)
	b := openFallback(t, w)
	assertFallback(t, b, w, l, "create WGL context", errRefused.Error())
	if opacityCalls != 2 {
		t.Errorf("%d opacity replays, want 2 (the GL attempt, then the soft one)", opacityCalls)
	}
	ex, _, _ := pGetWindowLongPtrW.Call(b.plat.hwnd, gwlExStyle)
	if layered := ex&wsExLayered != 0; layered != frameless || b.plat.layered != frameless {
		t.Errorf("frameless %v: ex-style %#x, plat.layered %v", frameless, ex, b.plat.layered)
	}
}

// TestSoftProcessMakesNoGLAttempt: once the first window fell back, a later
// window opens on the CPU renderer with no GL attempt and logs nothing.
func TestSoftProcessMakesNoGLAttempt(t *testing.T) {
	l := newAutoProcess(t)
	calls := refuseContexts(t)
	openFallback(t, gui.NewWindow(fallbackCfg()))
	if *calls != 1 {
		t.Fatalf("%d context attempts for the first window, want 1", *calls)
	}
	second := openFallback(t, gui.NewWindow(fallbackCfg()))
	if !second.plat.softMode || second.plat.soft == nil {
		t.Fatal("the second window is not on the CPU renderer")
	}
	if *calls != 1 {
		t.Errorf("the second window made a GL attempt: %d context attempts", *calls)
	}
	if lines := l.fallbackLines(); len(lines) != 1 {
		t.Errorf("%d fallback lines, want the first window's 1", len(lines))
	}
}

// TestLaterWindowGLFailureReturnsError: in a process whose first window is on
// GL, a later window whose context is refused returns the classified error
// and no soft window opens (no mixed process). It needs a GL that comes up
// for the first window, Mesa's under tools/glrendertest.
func TestLaterWindowGLFailureReturnsError(t *testing.T) {
	l := newAutoProcess(t)
	first, err := New(gui.NewWindow(fallbackCfg()))
	if err != nil {
		t.Skipf("the first window does not come up: %v", err)
	}
	t.Cleanup(first.Destroy)
	if first.plat.softMode {
		t.Skipf("the first window fell back; no GL comes up here: %s", l)
	}
	calls := refuseContexts(t)
	soft := countSoftSetups(t)
	b, err := New(gui.NewWindow(fallbackCfg()))
	if err == nil {
		b.Destroy()
		t.Fatal("a later window with a refused context opened")
	}
	if !errors.Is(err, ErrNoGPUContext) || !errors.Is(err, errRefused) {
		t.Fatalf("New error = %v, want ErrNoGPUContext with the cause", err)
	}
	if *calls != 1 || *soft != 0 {
		t.Errorf("%d context attempts and %d soft setups, want 1 and 0", *calls, *soft)
	}
	if lines := l.fallbackLines(); len(lines) != 0 {
		t.Errorf("logged a fallback: %q", lines)
	}
	if got := chosenRenderer(); got != win32RendererGL {
		t.Errorf("process renderer = %d, want GL", got)
	}
}

// exitDeadlocked ends the test binary when a New that opens a window from
// application code does not return: it prints the test, what waits and
// every goroutine's stack, which shows where the nested New waits, and
// exits with status 1.
func exitDeadlocked(test, waiting string) {
	stacks := make([]byte, 1<<20)
	stacks = stacks[:runtime.Stack(stacks, true)]
	fmt.Fprintf(os.Stderr, "--- FAIL: %s\n"+
		"    %s is still waiting after %s: a window's bring-up waits on "+
		"another's (deadlock)\n\n%s\nFAIL\n",
		test, waiting, hookWindowTimeout, stacks)
	os.Exit(1)
}

// hookWindowTimeout bounds the tests that open a window from application
// code inside New. A bring-up takes about a second on llvmpipe.
const hookWindowTimeout = 30 * time.Second

// glIsTexture reports whether id names a texture in the current context.
// glbind binds no glIsTexture; the backend never needs it.
func glIsTexture(t *testing.T, id uint32) bool {
	p := glProc("glIsTexture")
	if p == 0 {
		t.Error("no glIsTexture in opengl32.dll")
		return false
	}
	r, _, _ := syscall.SyscallN(p, uintptr(id))
	return byte(r) != 0
}

// TestWindowFromAppearanceHook: a window that follows the system appearance
// runs its OnSystemAppearance hook inside New, when the native platform
// attaches. A hook that opens a second window there must not wait for the
// first window's bring-up, and the second window opens on the renderer the
// first committed: the CPU renderer with no GL attempt after a fallback, and
// otherwise the first window's own outcome (GL under the Mesa GL selection).
// The renderer choice was once locked for the whole bring-up, so the nested
// New blocked on the same thread forever. On GL, the first window's
// transparent texture must also be a texture in its own context: the hook's
// window leaves its context current, and the texture was once created after
// the attach, in that context.
//
// New runs on its own goroutine, which also destroys both windows, because
// their thread ends with it. If it blocks, the test cannot fail the usual
// way: the goroutine never ends, and under a lock held across the bring-up
// the test's cleanup (resetProcessRenderer) would block on it too.
// exitDeadlocked ends the binary instead.
func TestWindowFromAppearanceHook(t *testing.T) {
	for _, refuse := range []bool{true, false} {
		name := "auto"
		if refuse {
			name = "fallback"
		}
		t.Run(name, func(t *testing.T) {
			l := newAutoProcess(t)
			// The OS setting the attach reads, so the hook runs everywhere.
			resetWinAppearance(t, gui.AppearanceDark, true)
			calls := new(int)
			if refuse {
				calls = refuseContexts(t)
			}
			type opened struct {
				err      error
				softMode bool
				hglrc    uintptr
			}
			type result struct {
				first, second opened
				hooks         int
				chosen        win32Renderer // after the hook's New, inside the first's
				firstTex      uint32        // the first window's transparent texture
				firstTexOwn   bool          // a texture in the first window's context
			}
			done := make(chan result, 1)
			go func() {
				var r result
				var second *Backend
				var secondWin *gui.Window
				w := gui.NewWindow(fallbackCfg())
				w.FollowSystemAppearance(gui.ThemeLight, gui.ThemeDark)
				w.OnSystemAppearance(func(gui.Appearance) {
					r.hooks++
					if r.hooks != 1 {
						return
					}
					secondWin = gui.NewWindow(fallbackCfg())
					second, r.second.err = New(secondWin)
					r.chosen = chosenRenderer()
				})
				first, err := New(w)
				r.first.err = err
				if first != nil && !first.plat.softMode {
					first.plat.makeCurrent()
					r.firstTex = first.transparentTex.id
					r.firstTexOwn = glIsTexture(t, r.firstTex)
				}
				for _, o := range []struct {
					b   *Backend
					w   *gui.Window
					rec *opened
				}{{first, w, &r.first}, {second, secondWin, &r.second}} {
					if o.b == nil {
						continue
					}
					o.rec.softMode, o.rec.hglrc = o.b.plat.softMode, o.b.plat.hglrc
					o.w.WindowCleanup()
					o.b.Destroy()
				}
				done <- r
			}()
			var r result
			select {
			case r = <-done:
			case <-time.After(hookWindowTimeout):
				exitDeadlocked("TestWindowFromAppearanceHook/"+name,
					"the window opened from the OnSystemAppearance hook during the first window's New")
			}

			skipNoWindow(t, r.first.err)
			if r.first.err != nil {
				t.Fatalf("first New: %v", r.first.err)
			}
			if r.hooks == 0 {
				t.Fatal("the OnSystemAppearance hook did not run while the window attached")
			}
			if r.second.err != nil {
				t.Fatalf("New from the hook: %v", r.second.err)
			}
			if r.second.softMode != r.first.softMode {
				t.Errorf("the hook's window has softMode %v, the first window %v",
					r.second.softMode, r.first.softMode)
			}
			want := win32RendererGL
			if r.first.softMode {
				want = win32RendererSoft
			}
			if r.chosen != want {
				t.Errorf("process renderer seen from the hook = %d, want %d", r.chosen, want)
			}
			fallbacks := len(l.fallbackLines())
			switch {
			case refuse:
				if !r.first.softMode || *calls != 1 || fallbacks != 1 {
					t.Errorf("softMode %v, %d context attempts and %d fallback lines, "+
						"want true, 1 and 1 (the hook's window makes no GL attempt)",
						r.first.softMode, *calls, fallbacks)
				}
			case r.first.softMode:
				// No GL comes up here; the soft commit is what ran.
				if os.Getenv("GOGUI_REQUIRE_GL") != "" {
					t.Fatalf("GOGUI_REQUIRE_GL is set but the first window fell back; log:\n%s", l)
				}
				t.Logf("no GL comes up here; both windows are on the CPU renderer")
			default:
				if r.first.hglrc == 0 || r.second.hglrc == 0 || fallbacks != 0 {
					t.Errorf("hglrc %#x and %#x, %d fallback lines; want two GL windows and none",
						r.first.hglrc, r.second.hglrc, fallbacks)
				}
				if !r.firstTexOwn {
					t.Errorf("the first window's transparent texture %d is not a texture in "+
						"its own context: it was made in the hook window's", r.firstTex)
				}
			}
		})
	}
}

// iconWarningWriter is an application's log writer that opens a window
// from the first write that carries the window-icon warning. Other writes,
// from other goroutines too, only go to l.
type iconWarningWriter struct {
	l      *logCapture
	open   func()
	mu     sync.Mutex
	opened bool
}

func (o *iconWarningWriter) Write(p []byte) (int, error) {
	n, err := o.l.Write(p)
	o.mu.Lock()
	fire := !o.opened && bytes.Contains(p, []byte("gl: window icon:"))
	o.opened = o.opened || fire
	o.mu.Unlock()
	if fire {
		o.open()
	}
	return n, err
}

// TestWindowFromLogWriter: an application's log writer that opens a window
// while it writes a window's warning must not wait on the renderer choice.
// An invalid IconPNG makes the first window log a warning. The warning is
// one of the attempt's reports, so it is logged when New publishes them,
// after the first window chose GL and stopped deciding: the writer's window
// finds the choice made and opens on it. The renderer choice was once
// locked through the first window's bring-up up to its commit, and a
// writer's New blocked on it on the same thread.
//
// It needs a GL that comes up, and checks that first with GOGUI_RENDERER=gl,
// which neither reads nor sets the choice: a writer's window that fell back
// would log its fallback line from inside the write, and the log package,
// which holds its own lock while the writer runs, would block it. The
// goroutine and the timeout are as in TestWindowFromAppearanceHook.
func TestWindowFromLogWriter(t *testing.T) {
	t.Setenv("GOGUI_RENDERER", "gl")
	probeWin := gui.NewWindow(fallbackCfg())
	probe, err := New(probeWin)
	skipNoWindow(t, err)
	if err != nil {
		if os.Getenv("GOGUI_REQUIRE_GL") != "" {
			t.Fatalf("GOGUI_REQUIRE_GL is set but no GL window comes up: %v", err)
		}
		t.Skipf("no GL comes up here: %v", err)
	}
	probeWin.WindowCleanup()
	probe.Destroy()

	l := newAutoProcess(t) // restores the log output after the test
	type opened struct {
		err   error
		hglrc uintptr
	}
	type result struct {
		first, second opened
		wrote         bool          // the writer opened its window
		chosen        win32Renderer // inside the write, after the writer's New
		deciding      bool
	}
	done := make(chan result, 1)
	go func() {
		var r result
		var second *Backend
		var secondWin *gui.Window
		log.SetOutput(&iconWarningWriter{l: l, open: func() {
			r.wrote = true
			secondWin = gui.NewWindow(fallbackCfg())
			second, r.second.err = New(secondWin)
			processRenderer.mu.Lock()
			r.chosen, r.deciding = processRenderer.chosen, processRenderer.deciding
			processRenderer.mu.Unlock()
		}})
		cfg := fallbackCfg()
		cfg.IconPNG = []byte("not a PNG")
		w := gui.NewWindow(cfg)
		first, ferr := New(w)
		r.first.err = ferr
		log.SetOutput(l)
		for _, o := range []struct {
			b   *Backend
			w   *gui.Window
			rec *opened
		}{{first, w, &r.first}, {second, secondWin, &r.second}} {
			if o.b == nil {
				continue
			}
			o.rec.hglrc = o.b.plat.hglrc
			o.w.WindowCleanup()
			o.b.Destroy()
		}
		done <- r
	}()
	var r result
	select {
	case r = <-done:
	case <-time.After(hookWindowTimeout):
		exitDeadlocked("TestWindowFromLogWriter",
			"the window opened from the log writer during the first window's New")
	}

	if r.first.err != nil {
		t.Fatalf("first New: %v", r.first.err)
	}
	if !r.wrote {
		t.Fatalf("the writer opened no window: no icon warning was logged; log:\n%s", l)
	}
	if r.second.err != nil {
		t.Fatalf("New from the log writer: %v", r.second.err)
	}
	if r.chosen != win32RendererGL || r.deciding {
		t.Errorf("inside the write: process renderer %d, deciding %v; want GL and not "+
			"deciding (the warning is published after the choice)", r.chosen, r.deciding)
	}
	if r.first.hglrc == 0 || r.second.hglrc == 0 {
		t.Errorf("hglrc %#x and %#x, want two GL windows", r.first.hglrc, r.second.hglrc)
	}
	if lines := l.fallbackLines(); len(lines) != 0 {
		t.Errorf("logged a fallback: %q", lines)
	}
	if got := chosenRenderer(); got != win32RendererGL {
		t.Errorf("process renderer = %d, want GL, chosen by the first window", got)
	}
}

// iconWarning is in the log line a bad window icon gives, and in no other.
const iconWarning = "gl: window icon:"

// TestFallbackLogsBadIconOnce: a window with an icon that does not decode
// falls back. Both attempts decode the icon, but only the soft attempt's
// window opens, so the warning is logged once, not once for each attempt.
func TestFallbackLogsBadIconOnce(t *testing.T) {
	l := newAutoProcess(t)
	refuseContexts(t)
	cfg := fallbackCfg()
	cfg.IconPNG = []byte("not a png")
	w := gui.NewWindow(cfg)
	b := openFallback(t, w)
	assertFallback(t, b, w, l, "create WGL context", errRefused.Error())
	if n := strings.Count(l.String(), iconWarning); n != 1 {
		t.Errorf("%d icon warnings, want 1; log:\n%s", n, l)
	}
}

// vsyncChildEnv marks the subprocess TestPipelineFailureReportsNoVSync
// starts.
const vsyncChildEnv = "GOGUI_TEST_PIPELINE_VSYNC_CHILD"

// The lines TestPipelineFailureReportsNoVSync looks for, as gui.Debug prints
// them: any vsync finding, and the control finding the child makes itself.
const (
	vsyncFinding  = "gui: window: vsync: "
	vsyncControl  = "test: control vsync finding"
	vsyncWantLine = vsyncFinding + vsyncControl
)

// TestPipelineFailureReportsNoVSync: the driver refuses the swap interval,
// and then the built-in pipelines fail, so New returns an error. The window
// never opened, so it must get no vsync finding. Before #1338's review the
// refusal went to the window at once.
//
// gui.Debug writes to stderr, and TestFindings sees only the findings of its
// own render, so the window is opened in a subprocess and its stderr is
// checked. The child then makes a control finding, which proves the gate is
// on and stderr is read. It needs a GL that reaches the pipelines, so it
// skips without one (tools/glrendertest runs it with GOGUI_REQUIRE_GL).
func TestPipelineFailureReportsNoVSync(t *testing.T) {
	if os.Getenv(vsyncChildEnv) == "" {
		cmd := exec.Command(os.Args[0], "-test.run", "^TestPipelineFailureReportsNoVSync$", "-test.v") // #nosec G204 -- this test binary, a fixed test name
		cmd.Env = append(os.Environ(), vsyncChildEnv+"=1")
		raw, err := cmd.CombinedOutput()
		out := string(raw)
		t.Logf("subprocess:\n%s", out)
		if err == nil && strings.Contains(out, "--- SKIP: TestPipelineFailureReportsNoVSync") {
			t.Skip("the subprocess skipped")
		}
		if err != nil || !strings.Contains(out, "--- PASS: TestPipelineFailureReportsNoVSync") {
			t.Fatalf("subprocess did not pass: %v", err)
		}
		if n := strings.Count(out, vsyncWantLine); n != 1 {
			t.Fatalf("%d control findings, want 1: stderr is not what the test reads", n)
		}
		if n := strings.Count(out, vsyncFinding); n != 1 {
			t.Errorf("%d vsync findings, want only the control: the failed window reported its swap interval", n)
		}
		return
	}

	gui.DebugCategories(gui.DebugWindowDegraded)
	t.Cleanup(func() { gui.DebugCategories(0) })
	oldSwap := swapIntervalFunc
	swapIntervalFunc = func(int32) bool { return false }
	t.Cleanup(func() { swapIntervalFunc = oldSwap })
	reached := false
	oldInit := initPipelinesFunc
	initPipelinesFunc = func(*Backend) error {
		reached = true
		return errors.New("test: link failed")
	}
	t.Cleanup(func() { initPipelinesFunc = oldInit })

	// openWin32Backend is New after the thread lock, so take it as New does.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	w := gui.NewWindow(fallbackCfg())
	b, err := openWin32Backend(w, rendererEnvGL)
	if !reached {
		if b != nil {
			b.Destroy()
		}
		skipNoWindow(t, err)
		if os.Getenv("GOGUI_REQUIRE_GL") != "" {
			t.Fatalf("GOGUI_REQUIRE_GL is set but no GL here reaches the pipelines: %v", err)
		}
		t.Skipf("no GL here reaches the pipelines: %v", err)
	}
	if err == nil {
		b.Destroy()
		t.Fatal("a pipeline failure opened a window")
	}
	w.DebugWindowVSync(vsyncControl)
}
