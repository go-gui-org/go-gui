package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// maxBlocks is how many times in a row the stop gate blocks per session
// before it lets the stop through with a note, so a finding the model
// cannot fix does not loop forever.
const maxBlocks = 3

// auditModes are the ergonomics-audit modes that fail on findings. focus and
// callbacks only report, so they are left out. The audits scan the whole
// repo; together they take about a second.
var auditModes = []string{"ids", "opt", "literals", "theme", "a11y", "visual", "deadcfg"}

// stopGate is the Stop hook that keeps the turn from ending while changed Go
// code fails the gate.
//
// The PostToolUse hook checks each Edit/Write. This gate catches what that
// misses: edits made through Bash (sed, gofmt -w, go generate) and the
// repo-wide rules that no per-file check sees (ergonomics-audit).
//
// Scope: packages holding a .go file that differs from HEAD, including
// untracked files. A clean tree passes at once.
func stopGate(in hookInput, project string, stderr io.Writer) int {
	// A headless reviewer started by the review gate loads project
	// settings too; it edits nothing, so gating its stop only burns time.
	if os.Getenv("GOGUI_REVIEW_GATE") != "" {
		return 0
	}
	session := in.SessionID
	if session == "" {
		session = "none"
	}
	gitdir := strings.TrimSpace(gitOutput(project, "rev-parse", "--absolute-git-dir"))
	if gitdir == "" {
		return 0
	}

	files := changedGoFiles(project)
	if len(files) == 0 {
		return 0
	}

	// Skip the whole run when the Go changes are byte-identical to the
	// last pass. A Stop fires on every turn, and most turns after a pass
	// change nothing.
	stamp := filepath.Join(gitdir, "claude-stop-gate.pass")
	fingerprint := stopFingerprint(project)
	if stampMatches(stamp, fingerprint) {
		return 0
	}

	allPkgs := packageDirs(files)

	// Split host packages from target-only ones (web, android, ios
	// backends). The host go vet/lint/test calls fail on a package whose
	// build tags exclude the host with "build constraints exclude all Go
	// files" — a gate error, not a code defect. Web packages are vetted
	// under js/wasm below, which needs no extra toolchain. Android and iOS
	// need the NDK and the Xcode SDK; make prepush cross-compiles and
	// cross-lints them.
	var pkgs, wasmPkgs, skipped []string
	for _, p := range allPkgs {
		switch {
		case hasFiles(project, p):
			pkgs = append(pkgs, p)
		case hasFiles(project, p, "GOOS=js", "GOARCH=wasm"):
			wasmPkgs = append(wasmPkgs, p)
		default:
			skipped = append(skipped, p)
		}
	}

	// Only the first Stop of a turn logs findings: the re-runs after a
	// block would count the same finding again.
	var logger *findingLogger
	if !in.StopHookActive {
		logger = newFindingLogger()
	}

	g := &gate{dir: project, logger: logger}
	if len(pkgs) > 0 {
		g.run("govet", nil, "go", append([]string{"vet"}, pkgs...)...)
		g.run("golangci-lint", nil, "golangci-lint", append([]string{"run"}, pkgs...)...)
		g.run("test-fail", nil, "go", append([]string{"test", "-short", "-count=1"}, pkgs...)...)
	}
	if len(wasmPkgs) > 0 {
		g.run("govet-wasm", []string{"GOOS=js", "GOARCH=wasm"}, "go", append([]string{"vet"}, wasmPkgs...)...)
	}
	for _, mode := range auditModes {
		g.run("audit-"+mode, nil, "go", "run", "./tools/ergonomics-audit/", "-mode", mode, ".")
	}

	counter := filepath.Join(gitdir, "claude-stop-gate."+session+".blocks")
	if g.report.Len() == 0 {
		writeStamp(stamp, fingerprint)
		_ = os.Remove(counter)
		return 0
	}

	blocks := 0
	if in.StopHookActive {
		if b, err := os.ReadFile(counter); err == nil { // #nosec G304 -- block counter under .git, path built by this tool
			blocks, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
	}
	if blocks >= maxBlocks {
		_ = os.Remove(counter)
		_, _ = fmt.Fprintf(stderr, "stop-gate: still failing after %d attempts; stop allowed.\n", maxBlocks)
		_, _ = fmt.Fprintln(stderr, g.report.String())
		return 0
	}
	writeStamp(counter, strconv.Itoa(blocks+1))

	_, _ = fmt.Fprintf(stderr, "Stop gate failed for changed packages: %s\n", strings.Join(allPkgs, " "))
	if len(skipped) > 0 {
		_, _ = fmt.Fprintf(stderr, "Not checked here (no host or js/wasm files; make prepush covers them): %s\n",
			strings.Join(skipped, " "))
	}
	_, _ = fmt.Fprintln(stderr, "Fix these before ending the turn. If a finding is pre-existing or out of")
	_, _ = fmt.Fprintln(stderr, "scope, say so to the user instead of working around the check.")
	_, _ = fmt.Fprintln(stderr)
	_, _ = fmt.Fprintln(stderr, g.report.String())
	return 2
}

// changedGoFiles lists .go files that differ from HEAD or are untracked,
// still exist (a deleted file has no package to check), and sit outside
// testdata/. The go tool ignores testdata for ./..., and an analyzer fixture
// holds code that fails lint on purpose (uncalled functions that only carry
// `// want` findings). make prepush skips it the same way.
func changedGoFiles(project string) []string {
	var files []string
	seen := map[string]bool{}
	for _, f := range append(
		lines(gitOutput(project, "diff", "--name-only", "HEAD", "--", "*.go")),
		lines(gitOutput(project, "ls-files", "--others", "--exclude-standard", "--", "*.go"))...,
	) {
		if f == "" || seen[f] || isTestdata(f) {
			continue
		}
		seen[f] = true
		if fi, err := os.Stat(filepath.Join(project, f)); err == nil && !fi.IsDir() {
			files = append(files, f)
		}
	}
	slices.Sort(files)
	return files
}

// isTestdata reports whether a slash-separated repo path lies in a testdata
// directory.
func isTestdata(f string) bool {
	return strings.HasPrefix(f, "testdata/") || strings.Contains(f, "/testdata/")
}

// stopFingerprint hashes the tracked Go diff plus the contents of untracked
// Go files.
func stopFingerprint(project string) string {
	h := sha256.New()
	_, _ = io.WriteString(h, gitOutput(project, "diff", "HEAD", "--", "*.go"))
	for _, f := range lines(gitOutput(project, "ls-files", "--others", "--exclude-standard", "--", "*.go")) {
		if b, err := os.ReadFile(filepath.Join(project, f)); err == nil { // #nosec G304 -- untracked repo file listed by git
			_, _ = h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// packageDirs returns the distinct directories of files as sorted
// ./-relative package patterns.
func packageDirs(files []string) []string {
	var pkgs []string
	for _, f := range files {
		p := "./" + path.Dir(f)
		if !slices.Contains(pkgs, p) {
			pkgs = append(pkgs, p)
		}
	}
	slices.Sort(pkgs)
	return pkgs
}

// gate runs the checks and collects the failing ones into report.
type gate struct {
	dir    string
	logger *findingLogger
	report strings.Builder
}

// run records the tail of a failing command's output under label.
func (g *gate) run(label string, env []string, name string, args ...string) {
	out, ok := runCmd(g.dir, env, name, args...)
	if ok {
		return
	}
	out = dropNoise(out)
	g.report.WriteString("== " + label + "\n" + tail(out, 30) + "\n\n")
	logFindings(g.logger, label, out)
}

var (
	// findingLineRe matches a line that names a Go file and line number.
	findingLineRe = regexp.MustCompile(`\.go:[0-9]`)
	// linterNameRe matches the linter name golangci-lint puts at the end
	// of each finding: "... (unused)".
	linterNameRe = regexp.MustCompile(`\(([a-z0-9]+)\)$`)
	// failLineRe matches the first line worth logging from a failing check
	// that names no file (a failing test).
	failLineRe = regexp.MustCompile(`--- FAIL|FAIL|panic`)
)

// logFindings logs one record per `file.go:line` finding line in out, or
// one record for the whole check when it names no file.
func logFindings(l *findingLogger, category, out string) {
	if l == nil {
		return
	}
	logged := false
	for _, line := range lines(out) {
		if !findingLineRe.MatchString(line) {
			continue
		}
		file, _, _ := strings.Cut(line, ":")
		kind := category
		if category == "golangci-lint" {
			if m := linterNameRe.FindStringSubmatch(line); m != nil {
				kind = m[1]
			}
		}
		msg := line
		if _, after, found := strings.Cut(line, ": "); found {
			msg = after
		}
		l.log("stop-gate", kind, file, msg)
		logged = true
	}
	if !logged {
		first := ""
		for _, line := range lines(out) {
			if failLineRe.MatchString(line) {
				first = line
				break
			}
		}
		l.log("stop-gate", category, "", first)
	}
}
