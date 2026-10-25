// claudehook runs this repo's Claude Code hooks (.claude/settings.json).
//
// It replaces the bash scripts that used to live in .claude/hooks/. Those
// needed bash, jq and shasum. On Windows, Claude Code runs a shell-form hook
// in Git Bash, which has no jq, so every hook exited 0 without checking
// anything. The settings now call this tool in exec form
// (`"command": "go", "args": [...]`), which needs no shell and runs the same
// on every OS; go is already a prerequisite for working on the repo. Since
// Go 1.24, `go run` caches the built binary, so a hook only compiles on its
// first call. Issue #1026 replaced the Makefile's shell gates for the same
// reason.
//
// Usage, from the repo root (the settings pass `go -C ${CLAUDE_PROJECT_DIR}`):
//
//	go run ./tools/claudehook pre     PreToolUse Edit|Write
//	go run ./tools/claudehook post    PostToolUse Edit|Write
//	go run ./tools/claudehook stop    Stop: gate changed packages
//	go run ./tools/claudehook review  Stop: headless correctness review
//
// Claude Code passes the hook input as JSON on stdin. Each mode returns the
// hook contract's exit code: 0 = pass, 2 = block and show the message to the
// model. The process cannot exit 2 itself, though: `go run` reports any
// non-zero exit of the program as "exit status 2" and exits 1, which Claude
// Code treats as a non-blocking error the model never sees. So a block is
// printed as the JSON decision Claude Code reads from stdout on exit 0
// (permissionDecision "deny" for PreToolUse, decision "block" otherwise),
// which has the same effect. A passing mode's messages go to stderr, which
// Claude Code writes to its debug log, as before.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// hookInput holds the fields of the hook input JSON the modes read. Fields
// absent from the input stay zero, as jq's `// empty` and `// false` did.
type hookInput struct {
	SessionID      string `json:"session_id"`
	StopHookActive bool   `json:"stop_hook_active"`
	ToolInput      struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

func main() {
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	raw, _ := io.ReadAll(os.Stdin)
	var in hookInput
	// Unparsable input means no file path and no session: each mode then
	// passes, as the scripts did when jq printed nothing.
	_ = json.Unmarshal(raw, &in)

	dir := projectDir()
	var msg bytes.Buffer
	var code int
	switch mode {
	case "pre":
		code = preEdit(in, &msg)
	case "post":
		code = postEdit(in, dir, &msg)
	case "stop":
		code = stopGate(in, dir, &msg)
	case "review":
		code = reviewGate(in, dir, &msg)
	default:
		fmt.Fprintf(os.Stderr, "claudehook: unknown mode %q (want pre, post, stop or review)\n", mode)
		os.Exit(1)
	}
	os.Exit(report(mode, code, msg.String(), os.Stdout, os.Stderr))
}

// report turns a mode's exit code and message into the hook output and
// returns the process exit code: a block becomes a JSON decision on stdout
// with exit 0 (see the package comment), anything else leaves the message
// on stderr.
func report(mode string, code int, msg string, stdout, stderr io.Writer) int {
	if code != 2 {
		_, _ = io.WriteString(stderr, msg)
		return code
	}
	reason := strings.TrimRight(msg, "\n")
	var out any
	if mode == "pre" {
		out = map[string]any{"hookSpecificOutput": map[string]string{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		}}
	} else {
		out = map[string]string{"decision": "block", "reason": reason}
	}
	b, err := json.Marshal(out)
	if err != nil {
		// Unreachable for maps of strings; fall back to the exit-code
		// contract rather than pass silently.
		_, _ = io.WriteString(stderr, msg)
		return 2
	}
	_, _ = stdout.Write(append(b, '\n'))
	return 0
}

// projectDir is CLAUDE_PROJECT_DIR, or the working directory when it is
// unset (a manual run from the repo root).
func projectDir() string {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
	}
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	return d
}

// lintInstallHint tells how to install the pinned linter when it is not on
// PATH. The hooks call golangci-lint from PATH, not the .bin/ copy make lint
// builds.
const lintInstallHint = "golangci-lint is not on PATH. Install the pinned version " +
	"(tools/lint/go.mod) from the repo root:\n" +
	"  go -C tools/lint install github.com/golangci/golangci-lint/v2/cmd/golangci-lint"

// runCmd runs name in dir with extra environment entries appended, and
// returns its combined stdout and stderr. ok is false when the command
// could not start or exited non-zero. A command that could not start has
// no output of its own, so the reason is returned instead: a missing
// golangci-lint gets lintInstallHint, anything else the start error.
func runCmd(dir string, env []string, name string, args ...string) (out string, ok bool) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	b, err := cmd.CombinedOutput()
	if err == nil {
		return string(b), true
	}
	if _, exited := errors.AsType[*exec.ExitError](err); exited {
		return string(b), false
	}
	if name == "golangci-lint" && errors.Is(err, exec.ErrNotFound) {
		return lintInstallHint, false
	}
	return fmt.Sprintf("%s: %v", name, err), false
}

// gitOutput runs git in dir and returns stdout only. A failing git yields "".
func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	b, _ := cmd.Output()
	return string(b)
}

// lines splits s into lines without the trailing empty element, dropping a
// carriage return so CRLF output from Windows tools compares the same.
func lines(s string) []string {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	for i, p := range parts {
		parts[i] = strings.TrimSuffix(p, "\r")
	}
	return parts
}

// tail returns the last n lines of s, joined by newlines.
func tail(s string, n int) string {
	ls := lines(s)
	if len(ls) > n {
		ls = ls[len(ls)-n:]
	}
	return strings.Join(ls, "\n")
}

// noiseRe matches the success and linker-noise lines dropped from a
// failing command's output before it is reported.
var noiseRe = regexp.MustCompile(`^ok |no test files|^ld: warning: ignoring duplicate libraries`)

// dropNoise removes the lines noiseRe matches.
func dropNoise(s string) string {
	var kept []string
	for _, l := range lines(s) {
		if !noiseRe.MatchString(l) {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n")
}

// hasFiles reports whether pkg (run from dir) has Go files, source or test,
// for the build target env selects. go list -e reports an all-excluded
// package with empty lists instead of failing. Only stdout counts: stderr
// ("go: downloading ...", errors) is not a file count, and a go list that
// fails or cannot start prints none.
func hasFiles(dir, pkg string, env ...string) bool {
	cmd := exec.Command("go", "list", "-e", "-f",
		"{{len .GoFiles}}{{len .CgoFiles}}{{len .TestGoFiles}}{{len .XTestGoFiles}}", pkg)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, _ := cmd.Output()
	n := strings.TrimSpace(string(out))
	return n != "" && n != "0000"
}

// findingLogger is the shared findings log (~/.claude/scripts/log-finding.sh),
// so rules broken again and again show up in quality-findings-report.sh.
// It is optional: nil when the script is absent.
type findingLogger struct {
	// record logs one finding; tests replace it.
	record func(source, kind, file, msg string)
}

// newFindingLogger returns the logger when its script exists, else nil. On
// Unix the script must be executable, as `[ -x ]` required. On Windows a .sh
// file has no executable bit; it runs through bash when bash is on PATH.
func newFindingLogger() *findingLogger {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	script := filepath.Join(home, ".claude", "scripts", "log-finding.sh")
	fi, err := os.Stat(script)
	if err != nil || fi.IsDir() {
		return nil
	}
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("bash"); err != nil {
			return nil
		}
	} else if fi.Mode().Perm()&0o111 == 0 {
		return nil
	}
	return &findingLogger{record: func(source, kind, file, msg string) {
		args := []string{source, kind, file, msg}
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("bash", append([]string{script}, args...)...)
		} else {
			cmd = exec.Command(script, args...)
		}
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		_ = cmd.Run()
	}}
}

// log records one finding. Failures to log never affect the hook's result.
func (l *findingLogger) log(source, kind, file, msg string) {
	if l == nil {
		return
	}
	l.record(source, kind, file, msg)
}

// writeStamp records fingerprint in path, ignoring errors as the scripts'
// `echo >file` did.
func writeStamp(path, fingerprint string) {
	_ = os.WriteFile(path, []byte(fingerprint+"\n"), 0o644)
}

// stampMatches reports whether path holds fingerprint. An empty fingerprint
// never matches: a failed hash must not turn into a permanent skip.
func stampMatches(path, fingerprint string) bool {
	if fingerprint == "" {
		return false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return string(bytes.TrimSpace(b)) == fingerprint
}
