package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var errCompile = errors.New("compile failed")

// captureStdout runs fn with os.Stdout redirected to a buffer.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		output string
		code   int
		want   verdict
	}{
		{"pass", "=== RUN   TestX\n--- PASS: TestX (0.01s)\nPASS\n", 0, pass},
		{"fail", "--- FAIL: TestX (0.01s)\nFAIL\n", 1, fail},
		// FAIL wins even with a zero exit code; the script checks it first.
		{"fail beats code", "--- FAIL: TestX\n", 0, fail},
		{"skip", "--- SKIP: TestX (0.00s)\n", 0, skip},
		// PASS needs exit code 0 as well.
		{"pass with nonzero exit is a crash", "--- PASS: TestX\n", 1, crash},
		{"empty output is a crash", "", 0, crash},
		{"panic is a crash", "panic: runtime error\n", 2, crash},
		// Matching is name-bound: another test's PASS is no verdict.
		{"other test passing is a crash", "--- PASS: TestY\n", 0, crash},
	}
	for _, tc := range cases {
		if got := classify("TestX", tc.output, tc.code); got != tc.want {
			t.Errorf("classify(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestList(t *testing.T) {
	win := testList(true)
	if len(win) != len(commonTests) {
		t.Errorf("windows list = %d tests, want %d", len(win), len(commonTests))
	}
	for _, x := range x11Tests {
		for _, w := range win {
			if w == x {
				t.Errorf("windows list contains X11-only %s", x)
			}
		}
	}
	full := testList(false)
	if len(full) != len(commonTests)+len(x11Tests) {
		t.Errorf("full list = %d tests, want %d", len(full), len(commonTests)+len(x11Tests))
	}
	for i, c := range commonTests {
		if full[i] != c {
			t.Errorf("full list order changed at %d: %s", i, full[i])
		}
	}
}

func TestBinName(t *testing.T) {
	if got := binName("build", false); got != filepath.Join("build", "gl.test") {
		t.Errorf("binName() = %q", got)
	}
	if got := binName("build", true); got != filepath.Join("build", "gl.test.exe") {
		t.Errorf("binName() = %q", got)
	}
}

func okCompile(string) error { return nil }

func TestRunMixed(t *testing.T) {
	results := map[string]struct {
		out  string
		code int
	}{
		"TestTriangleEdgesAntialiased": {"--- PASS: TestTriangleEdgesAntialiased\n", 0},
		"TestBackendRenderSmoke":       {"--- FAIL: TestBackendRenderSmoke\n", 1},
	}
	runOne := func(_ string, test string) (string, int) {
		r := results[test]
		if r.out == "" {
			// Every other test passes; the stub only scripts the
			// interesting verdicts.
			return "--- PASS: " + test + "\n", 0
		}
		return r.out, r.code
	}
	out := captureStdout(t, func() {
		if code := run(t.TempDir(), false, okCompile, runOne); code != 1 {
			t.Errorf("run() = %d, want 1", code)
		}
	})
	for _, want := range []string{
		"--- PASS: TestTriangleEdgesAntialiased\n",
		"::error::TestBackendRenderSmoke failed\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("run output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestRunAllPass(t *testing.T) {
	out := captureStdout(t, func() {
		if code := run(t.TempDir(), false, okCompile,
			func(_ string, test string) (string, int) {
				return "--- PASS: " + test + "\n", 0
			}); code != 0 {
			t.Errorf("run() = %d, want 0", code)
		}
	})
	if strings.Contains(out, "::error::") {
		t.Errorf("all-pass run emitted annotations:\n%s", out)
	}
}

func TestRunCompileFailure(t *testing.T) {
	out := captureStdout(t, func() {
		if code := run(t.TempDir(), false,
			func(string) error { return errCompile },
			func(_ string, _ string) (string, int) {
				t.Error("runOne called after compile failure")
				return "", 0
			}); code != 1 {
			t.Errorf("run() = %d, want 1", code)
		}
	})
	if out != "" {
		t.Errorf("compile failure printed %q, want nothing", out)
	}
}

// TestHelperProcess is re-executed as a fake gl test binary. Behavior
// follows the requested test name so the real list runs end to end
// through real subprocesses, without a shell or a display.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	target := os.Args[len(os.Args)-1]
	target = strings.TrimSuffix(strings.TrimPrefix(target, "^"), "$")
	switch target {
	case "TestBackendRenderSmoke":
		os.Stdout.WriteString("--- FAIL: " + target + " (0.01s)\n")
		os.Exit(1)
	case "TestSvgBatchMatchesUnbatched":
		os.Stdout.WriteString("--- SKIP: " + target + " (0.00s)\n")
	case "TestGlyphFillTransformed":
		os.Exit(2)
	default:
		os.Stdout.WriteString("--- PASS: " + target + " (0.01s)\n")
	}
	os.Exit(0)
}

func TestRunEndToEnd(t *testing.T) {
	old := startTest
	startTest = func(_, test string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "^"+test+"$")
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { startTest = old }()

	out := captureStdout(t, func() {
		if code := run(t.TempDir(), false, okCompile, runOne); code != 1 {
			t.Errorf("run() = %d, want 1 (one FAIL, one SKIP, one crash)", code)
		}
	})
	for _, want := range []string{
		"::error::TestBackendRenderSmoke failed\n",
		"::error::TestSvgBatchMatchesUnbatched skipped: no usable GL context on this runner\n",
		"::error::TestGlyphFillTransformed crashed without reporting a verdict (exit 2)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("run output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestCompilePkg(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake.test")
	if err := compilePkg("./testdata/fakegl/", bin); err != nil {
		t.Errorf("compilePkg(ok) = %v", err)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Errorf("test binary missing: %v", err)
	}

	// The broken source lives as .txt so the go tool never loads it;
	// copy it into a scratch module as a .go file.
	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.com/dlbroken\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken, err := os.ReadFile("testdata/broken/broken.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "broken.go"), broken, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := compilePkg(mod, filepath.Join(t.TempDir(), "broken.test")); err == nil {
		t.Error("compilePkg(broken) = nil, want an error")
	}
}
