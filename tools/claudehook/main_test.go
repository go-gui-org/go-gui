package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestReportTurnsBlockIntoJSONDecision(t *testing.T) {
	for _, tc := range []struct {
		mode, want string
	}{
		{"pre", `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"no \"go.sum\"\nline 2"}}`},
		{"post", `{"decision":"block","reason":"no \"go.sum\"\nline 2"}`},
		{"stop", `{"decision":"block","reason":"no \"go.sum\"\nline 2"}`},
		{"review", `{"decision":"block","reason":"no \"go.sum\"\nline 2"}`},
	} {
		var stdout, stderr strings.Builder
		if code := report(tc.mode, 2, "no \"go.sum\"\nline 2\n", &stdout, &stderr); code != 0 {
			t.Errorf("%s: exit = %d, want 0", tc.mode, code)
		}
		var got, want any
		if err := json.Unmarshal([]byte(stdout.String()), &got); err != nil {
			t.Fatalf("%s: stdout is not one JSON object: %v\n%s", tc.mode, err, stdout.String())
		}
		if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: stdout = %s, want %s", tc.mode, stdout.String(), tc.want)
		}
		if stderr.Len() != 0 {
			t.Errorf("%s: stderr = %q, want empty", tc.mode, stderr.String())
		}
	}

	var stdout, stderr strings.Builder
	if code := report("stop", 0, "stop-gate: note\n", &stdout, &stderr); code != 0 {
		t.Errorf("pass: exit = %d, want 0", code)
	}
	if stdout.Len() != 0 || stderr.String() != "stop-gate: note\n" {
		t.Errorf("pass: stdout %q, stderr %q; want no stdout and the note on stderr",
			stdout.String(), stderr.String())
	}
}

func TestLinesDropsTrailingNewlineAndCR(t *testing.T) {
	got := lines("a\r\nb\n\n")
	if want := []string{"a", "b"}; !slices.Equal(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	if got := lines(""); got != nil {
		t.Fatalf("lines(\"\") = %q, want nil", got)
	}
}

func TestTailKeepsLastLines(t *testing.T) {
	if got := tail("1\n2\n3\n4\n", 2); got != "3\n4" {
		t.Fatalf("tail = %q, want %q", got, "3\n4")
	}
	if got := tail("1\n2\n", 5); got != "1\n2" {
		t.Fatalf("tail = %q, want %q", got, "1\n2")
	}
}

func TestDropNoise(t *testing.T) {
	in := "ok  \tgithub.com/x/a\t0.1s\n?   \tgithub.com/x/b\t[no test files]\n" +
		"ld: warning: ignoring duplicate libraries: '-lobjc'\n--- FAIL: TestX\nFAIL\n"
	if got, want := dropNoise(in), "--- FAIL: TestX\nFAIL"; got != want {
		t.Fatalf("dropNoise = %q, want %q", got, want)
	}
}

func TestStampMatches(t *testing.T) {
	stamp := filepath.Join(t.TempDir(), "stamp")
	if stampMatches(stamp, "abc") {
		t.Fatal("missing stamp matched")
	}
	writeStamp(stamp, "abc")
	if !stampMatches(stamp, "abc") {
		t.Fatal("written stamp did not match")
	}
	if stampMatches(stamp, "abd") {
		t.Fatal("different fingerprint matched")
	}
	// An empty fingerprint must never skip the gate, even against a stamp
	// that holds only a newline (what a failed hash used to write).
	writeStamp(stamp, "")
	if stampMatches(stamp, "") {
		t.Fatal("empty fingerprint matched")
	}
}

func TestPreEditBlocksGoSum(t *testing.T) {
	for _, p := range []string{"go.sum", "/repo/go.sum", filepath.Join("C:", "repo", "tools", "go.sum")} {
		var in hookInput
		in.ToolInput.FilePath = p
		var stderr strings.Builder
		if code := preEdit(in, &stderr); code != 2 {
			t.Errorf("preEdit(%q) = %d, want 2", p, code)
		}
		if !strings.Contains(stderr.String(), "BLOCKED") {
			t.Errorf("preEdit(%q) stderr = %q, want BLOCKED", p, stderr.String())
		}
	}
}

func TestPreEditLineLimit(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.go")
	big := filepath.Join(dir, "big.go")
	writeFile(t, small, "package x\n")
	// gofmt runs before the count, so the padding must survive it: blank
	// lines would collapse to one.
	writeFile(t, big, "package x\n"+strings.Repeat("var _ = 0\n", maxGoFileLines))

	for _, tc := range []struct {
		file string
		want int
	}{
		{small, 0},
		{big, 2},
		{filepath.Join(dir, "new.go"), 0},   // Write creating a file
		{filepath.Join(dir, "notes.md"), 0}, // not Go
		{"", 0},                             // no file path
	} {
		var in hookInput
		in.ToolInput.FilePath = tc.file
		var stderr strings.Builder
		if got := preEdit(in, &stderr); got != tc.want {
			t.Errorf("preEdit(%q) = %d, want %d (stderr %q)", tc.file, got, tc.want, stderr.String())
		}
	}
}

func TestPreEditFormatsGoFile(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not on PATH")
	}
	file := filepath.Join(t.TempDir(), "x.go")
	writeFile(t, file, "package x\nfunc  f( ) {}\n")
	var in hookInput
	in.ToolInput.FilePath = file
	if code := preEdit(in, &strings.Builder{}); code != 0 {
		t.Fatalf("preEdit = %d, want 0", code)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), "package x\n\nfunc f() {}\n"; got != want {
		t.Fatalf("formatted = %q, want %q", got, want)
	}
}

func TestPackageOf(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/r\n")
	writeFile(t, filepath.Join(root, "tools", "lint", "go.mod"), "module example.com/lint\n")
	nested := filepath.Join(root, "tools", "lint")

	for _, tc := range []struct {
		name, file, wantRoot, wantPkg string
	}{
		{"root package", filepath.Join(root, "main.go"), root, "./."},
		{"subpackage", filepath.Join(root, "gui", "x.go"), root, "./gui"},
		{"relative path", filepath.Join("gui", "backend", "x.go"), root, "./gui/backend"},
		{"nested module", filepath.Join(nested, "tools.go"), nested, "./."},
	} {
		gotRoot, gotPkg, ok := packageOf(tc.file, root)
		if !ok || gotRoot != tc.wantRoot || gotPkg != tc.wantPkg {
			t.Errorf("%s: packageOf = (%q, %q, %v), want (%q, %q, true)",
				tc.name, gotRoot, gotPkg, ok, tc.wantRoot, tc.wantPkg)
		}
	}

	outside := filepath.Join(t.TempDir(), "x.go")
	if _, _, ok := packageOf(outside, root); ok {
		t.Errorf("packageOf(%q) found a module; want none", outside)
	}
}

func TestPostEditIgnoresNonGoAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"", filepath.Join(dir, "README.md"), filepath.Join(dir, "gone.go")} {
		var in hookInput
		in.ToolInput.FilePath = f
		if code := postEdit(in, dir, &strings.Builder{}); code != 0 {
			t.Errorf("postEdit(%q) = %d, want 0", f, code)
		}
	}
}

func TestChangedGoFilesAndPackages(t *testing.T) {
	repo := gitRepo(t)
	writeFile(t, filepath.Join(repo, "a", "a.go"), "package a\n")
	writeFile(t, filepath.Join(repo, "a", "b.go"), "package a\n")
	writeFile(t, filepath.Join(repo, "c", "c.go"), "package c\n")
	writeFile(t, filepath.Join(repo, "notes.md"), "notes\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "init")

	if got := changedGoFiles(repo); len(got) != 0 {
		t.Fatalf("clean tree: changedGoFiles = %q, want none", got)
	}
	clean := stopFingerprint(repo)

	writeFile(t, filepath.Join(repo, "a", "a.go"), "package a\n\nvar X int\n") // modified
	writeFile(t, filepath.Join(repo, "d", "d.go"), "package d\n")              // untracked
	writeFile(t, filepath.Join(repo, "a", "testdata", "t.go"), "package t\n")  // testdata
	writeFile(t, filepath.Join(repo, "notes.md"), "changed\n")                 // not Go
	if err := os.Remove(filepath.Join(repo, "c", "c.go")); err != nil {        // deleted
		t.Fatal(err)
	}

	got := changedGoFiles(repo)
	if want := []string{"a/a.go", "d/d.go"}; !slices.Equal(got, want) {
		t.Fatalf("changedGoFiles = %q, want %q", got, want)
	}
	if pkgs, want := packageDirs(got), []string{"./a", "./d"}; !slices.Equal(pkgs, want) {
		t.Fatalf("packageDirs = %q, want %q", pkgs, want)
	}

	changed := stopFingerprint(repo)
	if changed == "" || changed == clean {
		t.Fatalf("fingerprint did not change: clean %q, changed %q", clean, changed)
	}
	if again := stopFingerprint(repo); again != changed {
		t.Fatalf("fingerprint not stable: %q then %q", changed, again)
	}
	writeFile(t, filepath.Join(repo, "d", "d.go"), "package d\n\nvar Y int\n")
	if edited := stopFingerprint(repo); edited == changed {
		t.Fatal("editing an untracked file did not change the fingerprint")
	}
}

func TestReviewDiffIncludesUntrackedFiles(t *testing.T) {
	repo := gitRepo(t)
	writeFile(t, filepath.Join(repo, "a.go"), "package a\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "init")
	if got := reviewDiff(repo); got != "" {
		t.Fatalf("clean tree: reviewDiff = %q, want empty", got)
	}

	writeFile(t, filepath.Join(repo, "a.go"), "package a\n\nvar X int\n")
	writeFile(t, filepath.Join(repo, "b.go"), "package a\n\nvar Y int\n")
	got := reviewDiff(repo)
	for _, want := range []string{"+var X int", "+++ b/b.go", "+var Y int"} {
		if !strings.Contains(got, want) {
			t.Errorf("reviewDiff missing %q:\n%s", want, got)
		}
	}
	if strings.HasSuffix(got, "\n") {
		t.Error("reviewDiff ends with a newline; want it trimmed")
	}
}

func TestLogFindings(t *testing.T) {
	type rec struct{ source, kind, file, msg string }
	var got []rec
	l := &findingLogger{record: func(source, kind, file, msg string) {
		got = append(got, rec{source, kind, file, msg})
	}}

	logFindings(l, "golangci-lint",
		"gui/a.go:12:3: unused variable x (unused)\nsummary line\ngui/b.go:7:1: missing doc (revive)")
	logFindings(l, "test-fail", "--- FAIL: TestX (0.00s)\nFAIL\tgithub.com/x\t0.1s")
	logFindings(l, "govet", "")

	want := []rec{
		{"stop-gate", "unused", "gui/a.go", "unused variable x (unused)"},
		{"stop-gate", "revive", "gui/b.go", "missing doc (revive)"},
		{"stop-gate", "test-fail", "", "--- FAIL: TestX (0.00s)"},
		{"stop-gate", "govet", "", ""},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("logged:\n%q\nwant:\n%q", got, want)
	}

	// A nil logger (no script installed) logs nothing and does not panic.
	logFindings(nil, "govet", "gui/a.go:1:1: x")
}

func TestReviewRegexps(t *testing.T) {
	if !verdictRe.MatchString("VERDICT: PASS") || !verdictRe.MatchString("VERDICT: FINDINGS") {
		t.Fatal("verdictRe misses a verdict line")
	}
	if verdictRe.MatchString("VERDICT: PASS.") || verdictRe.MatchString(" VERDICT: PASS") {
		t.Fatal("verdictRe matches a line that is not exactly a verdict")
	}
	m := reviewFindingRe.FindStringSubmatch("gui/a.go:12 — [correctness] nil map write — init it")
	if m == nil || m[1] != "gui/a.go" || m[2] != "correctness" {
		t.Fatalf("reviewFindingRe = %q", m)
	}
	if msg, _, _ := strings.Cut(m[3], " — "); msg != "nil map write" {
		t.Fatalf("finding message = %q, want %q", msg, "nil map write")
	}
}

func TestStopAndReviewSkipInsideReviewer(t *testing.T) {
	t.Setenv("GOGUI_REVIEW_GATE", "1")
	dir := t.TempDir()
	if code := stopGate(hookInput{}, dir, &strings.Builder{}); code != 0 {
		t.Errorf("stopGate = %d, want 0", code)
	}
	if code := reviewGate(hookInput{}, dir, &strings.Builder{}); code != 0 {
		t.Errorf("reviewGate = %d, want 0", code)
	}
}

func TestGatesPassOnCleanTree(t *testing.T) {
	t.Setenv("GOGUI_REVIEW_GATE", "")
	repo := gitRepo(t)
	writeFile(t, filepath.Join(repo, "a.go"), "package a\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "init")
	if code := stopGate(hookInput{}, repo, &strings.Builder{}); code != 0 {
		t.Errorf("stopGate on a clean tree = %d, want 0", code)
	}
	if code := reviewGate(hookInput{}, repo, &strings.Builder{}); code != 0 {
		t.Errorf("reviewGate on a clean tree = %d, want 0", code)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitRepo creates an empty repository in a temp dir, skipping the test when
// git is not installed.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "test")
	git(t, repo, "config", "core.autocrlf", "false")
	return repo
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
