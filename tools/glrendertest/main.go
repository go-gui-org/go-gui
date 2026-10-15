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
//	go run ./tools/glrendertest <dir for the test binary>
//
// On Windows the caller copies opengl32.dll into that directory first:
// the app directory is the only place Windows looks before System32,
// whose opengl32.dll is GDI's OpenGL 1.1.
package main

import (
	"errors"
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
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: glrendertest <dir for the test binary>")
		os.Exit(2)
	}
	os.Exit(run(os.Args[1], runtime.GOOS == "windows", compileTest, runOne))
}

// testList returns the tests for this OS, in run order.
func testList(windows bool) []string {
	if windows {
		return commonTests
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

func run(dir string, windows bool, compile compileFunc, runOne runOneFunc) int {
	bin := binName(dir, windows)
	if err := compile(bin); err != nil {
		return 1
	}
	status := 0
	for _, t := range testList(windows) {
		out, code := runOne(bin, t)
		fmt.Println(strings.TrimRight(out, "\n"))
		switch classify(t, out, code) {
		case pass:
		case fail:
			fmt.Printf("::error::%s failed\n", t)
			status = 1
		case skip:
			// A skipped test checked nothing: the runner has no
			// usable GL context.
			fmt.Printf("::error::%s skipped: no usable GL context on this runner\n", t)
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
var startTest = func(bin, test string) *exec.Cmd {
	cmd := exec.Command(bin, "-test.run", "^"+test+"$", "-test.v") // #nosec G204 -- binary this tool just built; test names come from testList
	cmd.Env = append(os.Environ(), "GOGUI_REQUIRE_GL=1")
	return cmd
}

// runOne runs one test in its own process with GOGUI_REQUIRE_GL=1 and
// returns its combined output and exit code.
func runOne(bin, test string) (string, int) {
	out, err := startTest(bin, test).CombinedOutput()
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
