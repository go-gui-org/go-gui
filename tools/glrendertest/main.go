// glrendertest runs the GL backend tests that need a real GL 3.3
// context (#823).
//
// It replaces scripts/gl-render-test.sh (issue #1026) with the same
// behavior: compile the gl package's test binary, run each GL test in
// its own process with GOGUI_REQUIRE_GL=1, print each test's output,
// and report fail/skip/crash verdicts as ::error:: annotations. Exit
// code is 1 when any test fails, skips, or crashes.
//
// `go test ./gui/...` skips these tests, because a CI runner has no GPU
// and no display. The CI jobs that call this tool provide Mesa's
// software renderer (llvmpipe): on Linux under Xvfb, on Windows by
// putting Mesa's opengl32.dll next to the test binary. The Xvfb start
// and the DLL copy stay in the CI workflow; only the test selection
// and the environment move here. GOGUI_REQUIRE_GL makes a test fail,
// instead of skip, when the backend still cannot open a context, so a
// broken runner setup shows up as red and not as a silent pass.
//
// One process per test: under Xvfb a second backend in the same process
// gets its X connection reset. There is no retry: a run that crashes
// before reporting a verdict is a failure, like any other.
//
// Usage:
//
//	go run ./tools/glrendertest [-select mesa-gl|soft|system-gl] <dir for the test binary>
//
// On Windows the caller copies opengl32.dll into that directory first:
// the app directory is the only place Windows looks before System32,
// whose opengl32.dll is GDI's OpenGL 1.1. -select picks one of three
// Windows runs (#1310), which differ in that DLL and in GOGUI_RENDERER:
//
//   - mesa-gl, the default: Mesa's DLL; the GL tests, with
//     GOGUI_RENDERER=gl, and the tests that a GL that comes up keeps GL with
//     the variable unset (they unset it themselves).
//   - soft: Mesa's DLL, which the tests require to stay unloaded; the
//     GOGUI_RENDERER=soft present tests, with GOGUI_RENDERER=soft.
//   - system-gl: no DLL in the directory, so the system's own OpenGL; the
//     GDI Generic regression test, then the tests of the fallback to the
//     CPU renderer, with GOGUI_RENDERER unset (each test sets it). The
//     regression test skips, which counts as a failure, when the system
//     OpenGL makes no context, so the run also shows what the runner's
//     OpenGL does.
//
// Linux has one run, the default.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// glPackage is the test binary's source.
const glPackage = "./gui/backend/gl/"

// commonTests run on every OS, in order.
var commonTests = []string{
	"TestPixelSurfaceTextureReuseAndPixels",
	"TestMemImageTextureFollowsRegistry",
	"TestShaderChannelsReadback",
	"TestShaderChannelTextureReuse",
	"TestBackendReportsShaders",
	"TestTriangleEdgesAntialiased",
	"TestProbeSeesAliasedEdges",
	"TestFilterEdgesAntialiased",
	"TestFilterProbeSeesAliasedEdges",
	"TestRefusedResolveFallsBack",
	"TestBackendRenderSmoke",
	"TestGlyphBatchFlushCounts",
	"TestGlyphBatchFlushesBeforeUpload",
	"TestGlyphBatchFlushesBeforeDelete",
	"TestGlyphBatchFlushesBeforeFilledRect",
	"TestGlyphBatchMatchesUnbatched",
	"TestGlyphFillIgnoresBoundTexture",
	"TestGlyphFillsShareABatch",
	"TestGlyphFillTransformed",
	"TestSvgBatchFlushCounts",
	"TestSvgBatchMatchesUnbatched",
}

// x11Tests run everywhere except Windows (#827): the thread-release
// tests are X11-only, and a name that matches nothing reports no
// verdict, which counts as a crash.
var x11Tests = []string{
	"TestNewErrorReleasesThread",
	"TestDestroyReleasesThread",
	// GOGUI_RENDERER=soft present path (#1265): needs an X server, not GL.
	"TestSoftX11NeverTouchesEGL",
	"TestSoftX11PixelsMatchRenderToImage",
	// The X11 soft upload (#1265 step 5): MIT-SHM, the core PutImage
	// fallback, a failed segment attach, and the per-frame heap cost.
	"TestSoftX11CorePutImagePixels",
	"TestSoftX11ShmAttachFailureFallsBack",
	"TestSoftX11PresentAllocs",
	// Auto-pick with GOGUI_RENDERER unset (#1265 step 3): the runner's
	// llvmpipe must choose soft, and so must an EGL failure.
	"TestAutoPickSoftOnSoftwareRasterizer",
	"TestAutoPickSoftOnEGLFailure",
	"TestAutoPickSoftInitFailureFallsBackToGL",
	"TestAutoPickSoftInitFailureAfterEGLFailure",
}

// win32Tests run on Windows only, where their file is built. They need a
// context that reaches initGLResources, so the plain test run on a
// runner without Mesa skips them (#1310).
var win32Tests = []string{
	"TestPipelineFailureIsGPUContextError",
	"TestTextSystemFailureStaysPlain",
	// With GOGUI_RENDERER unset (#1310, phase 3): a GL that comes up keeps
	// GL, a pipeline failure does not fall back, a later window of a GL
	// process returns the error, a window opened from an appearance hook
	// inside the first window's New gets the first window's renderer, on GL
	// and after a forced fallback, and one opened from the log writer before
	// the choice does not wait for it. Each unsets the variable itself.
	"TestUnsetWithMesaKeepsGL",
	"TestPipelineFailureDoesNotFallBack",
	"TestLaterWindowGLFailureReturnsError",
	"TestWindowFromAppearanceHook",
	"TestWindowFromLogWriter",
}

// win32SoftTests are the GOGUI_RENDERER=soft present tests (#1310). They
// need a desktop session, not GL.
var win32SoftTests = []string{
	"TestSoftWin32LoadsNoGL",
	"TestSoftWin32PixelsMatchRenderToImage",
	"TestSoftWin32ResizeKeepsHandles",
	"TestSoftWin32OpacityAfterPaint",
	"TestSoftWin32PresentAllocs",
}

// win32SystemGLTests run on the system's own OpenGL (#1310): the GDI Generic
// regression test, then the first-window fallback. The fallback tests force
// their trigger through a seam, except TestFallbackOnGDIGeneric, which needs
// the system DLL.
var win32SystemGLTests = []string{
	"TestGDIGenericIsGPUContextError",
	"TestFallbackOnRefusedContext",
	"TestFallbackOnGDIGeneric",
	"TestRendererGLReportsError",
	"TestFallbackSoftFailure",
	"TestFallbackKeepsHiddenStart",
	"TestFallbackKeepsQueuedPlacement",
	"TestFallbackRunsOnInitOnce",
	"TestFallbackPublishesWinningReportsOnly",
	"TestSoftProcessMakesNoGLAttempt",
}

// selection is one run: the tests, in order, and the GOGUI_RENDERER value
// their processes get. An empty renderer removes the variable, so each
// test sets it. skipCause is what a skipped test in this run most likely
// means; it goes into the CI annotation.
type selection struct {
	tests     []string
	renderer  string
	skipCause string
}

// The skip causes. A GL run skips when the runner has no usable GL context.
// The soft run uses no GL: it skips when it cannot open or check the soft
// window, so blaming GL there sends the reader the wrong way.
const (
	causeNoGL         = "no usable GL context on this runner"
	causeNoSoftWindow = "the soft window could not be opened or checked on this runner"
)

// selectionFor returns the run that -select names (see the package doc).
func selectionFor(name string, windows bool) (selection, error) {
	switch {
	case name == "" || name == "mesa-gl":
		// GOGUI_RENDERER=gl keeps the GL tests on GL: the runner's GL is
		// llvmpipe, which auto-pick would replace with the CPU renderer
		// (#1265). The soft and auto-pick tests set the variable themselves.
		return selection{tests: testList(windows), renderer: "gl", skipCause: causeNoGL}, nil
	case !windows:
		return selection{}, fmt.Errorf("-select %s is a Windows run", name)
	case name == "soft":
		return selection{tests: win32SoftTests, renderer: "soft", skipCause: causeNoSoftWindow}, nil
	case name == "system-gl":
		return selection{tests: win32SystemGLTests, skipCause: causeNoGL}, nil
	}
	return selection{}, fmt.Errorf("-select %q: want mesa-gl, soft or system-gl", name)
}

// verdict classifies one test process's output.
type verdict int

const (
	pass verdict = iota
	fail
	skip
	crash
)

func main() {
	name := flag.String("select", "", "Windows run: mesa-gl (default), soft or system-gl")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: glrendertest [-select mesa-gl|soft|system-gl] <dir for the test binary>")
		os.Exit(2)
	}
	windows := runtime.GOOS == "windows"
	sel, err := selectionFor(*name, windows)
	if err != nil {
		fmt.Fprintln(os.Stderr, "glrendertest:", err)
		os.Exit(2)
	}
	os.Exit(run(flag.Arg(0), windows, sel.tests, sel.skipCause, compileTest,
		func(bin, test string) (string, int) {
			return runOne(bin, test, sel.renderer)
		}))
}

// testList returns the tests for this OS, in run order.
func testList(windows bool) []string {
	if windows {
		return append(append([]string{}, commonTests...), win32Tests...)
	}
	return append(append([]string{}, commonTests...), x11Tests...)
}

// binName names the compiled test binary in dir.
func binName(dir string, windows bool) string {
	name := "gl.test"
	if windows {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

// compileFunc builds the test binary. runOneFunc runs one test in it.
// Both are variables so tests can stub the subprocess boundary.
type compileFunc func(bin string) error
type runOneFunc func(bin, test string) (output string, code int)

// run compiles the test binary, runs each test and annotates every test that
// did not pass. skipCause names why a skip happened in this selection.
func run(dir string, windows bool, tests []string, skipCause string,
	compile compileFunc, runOne runOneFunc) int {
	bin := binName(dir, windows)
	if err := compile(bin); err != nil {
		return 1
	}
	status := 0
	for _, t := range tests {
		out, code := runOne(bin, t)
		fmt.Println(strings.TrimRight(out, "\n"))
		switch classify(t, out, code) {
		case pass:
		case fail:
			fmt.Printf("::error::%s failed\n", t)
			status = 1
		case skip:
			// A skipped test checked nothing. Its own SKIP line, printed
			// above, says why; skipCause is the usual reason in this run.
			fmt.Printf("::error::%s skipped: %s\n", t, skipCause)
			status = 1
		case crash:
			fmt.Printf("::error::%s crashed without reporting a verdict (exit %d)\n", t, code)
			status = 1
		}
	}
	return status
}

// compileTest builds the gl test binary with a C toolchain disabled,
// like the rest of the desktop build. Build output streams through;
// any failure exits non-zero with go's own message, like the script.
func compileTest(bin string) error {
	return compilePkg(glPackage, bin)
}

// compilePkg builds one package's test binary.
func compilePkg(pkg, bin string) error {
	cmd := exec.Command("go", "test", "-c", "-o", bin, pkg) // #nosec G204 -- fixed binary and flags; paths are the tool's documented inputs
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// startTest builds one test's subprocess. It is a variable so tests can
// point it at the test binary itself (TestHelperProcess) instead of a
// real gl test binary.
var startTest = func(bin, test, renderer string) *exec.Cmd {
	cmd := exec.Command(bin, "-test.run", "^"+test+"$", "-test.v") // #nosec G204 -- binary this tool just built; test names come from the selection
	cmd.Env = testEnv(os.Environ(), renderer)
	return cmd
}

// testEnv is a test process's environment: base with GOGUI_REQUIRE_GL=1,
// and with GOGUI_RENDERER set to renderer, or removed when renderer is
// empty, so a value in the caller's environment cannot reach a test that
// sets its own.
func testEnv(base []string, renderer string) []string {
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if !strings.HasPrefix(kv, "GOGUI_RENDERER=") {
			env = append(env, kv)
		}
	}
	env = append(env, "GOGUI_REQUIRE_GL=1")
	if renderer != "" {
		env = append(env, "GOGUI_RENDERER="+renderer)
	}
	return env
}

// runOne runs one test in its own process with GOGUI_REQUIRE_GL=1 and the
// selection's GOGUI_RENDERER, and returns its combined output and exit code.
func runOne(bin, test, renderer string) (string, int) {
	out, err := startTest(bin, test, renderer).CombinedOutput()
	code := 0
	if err != nil {
		code = 1
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitErr.ExitCode()
		}
	}
	return string(out), code
}

// classify reads the verdict off a test's output, in the script's
// order: a FAIL line wins over everything, then SKIP, then PASS (which
// also needs exit code 0). Anything else is a crash.
func classify(test, output string, code int) verdict {
	if strings.Contains(output, "--- FAIL: "+test) {
		return fail
	}
	if strings.Contains(output, "--- SKIP: "+test) {
		return skip
	}
	if code == 0 && strings.Contains(output, "--- PASS: "+test) {
		return pass
	}
	return crash
}
