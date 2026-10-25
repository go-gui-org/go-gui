package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// maxGoFileLines is the size limit for a Go file being edited. The pre hook
// blocks an edit to a larger file, as the inline settings command did.
const maxGoFileLines = 800

// preEdit is the PreToolUse hook for Edit and Write. It blocks direct go.sum
// edits, then gofmts a Go file and blocks the edit when the file is over
// maxGoFileLines. It returns 2 to block, otherwise 0. Formatting errors are
// ignored; a file that cannot be read passes the size check.
func preEdit(in hookInput, stderr io.Writer) int {
	file := in.ToolInput.FilePath
	if file == "" {
		return 0
	}
	if filepath.Base(file) == "go.sum" {
		_, _ = fmt.Fprintln(stderr, "BLOCKED: do not edit go.sum directly. Run go mod tidy.")
		return 2
	}
	if filepath.Ext(file) != ".go" {
		return 0
	}
	// A Write that creates the file finds nothing to format or count.
	_ = exec.Command("gofmt", "-w", file).Run() // #nosec G204 -- fixed binary, no shell; file is the path Claude Code is editing
	n, err := countLines(file)
	if err != nil {
		return 0
	}
	if n > maxGoFileLines {
		_, _ = fmt.Fprintf(stderr, "WARNING: %s is %d lines (limit %d)\n", file, n, maxGoFileLines)
		return 2
	}
	return 0
}

// countLines counts newlines the way wc -l does.
// It returns 0 and the read error if path cannot be read.
func countLines(path string) (int, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- the file being edited, named by the hook input
	if err != nil {
		return 0, err
	}
	return bytes.Count(b, []byte{'\n'}), nil
}

// postEdit is the PostToolUse hook for Edit and Write: lint-fix and short
// tests on the edited Go file's package. Only that package, not dir/... —
// recursing from gui/ pulls in every subpackage and makes each edit wait on
// the whole tree.
// It skips packages with no host Go files and returns 0 for skipped files or
// successful checks, or 2 if lint or tests fail or cannot start.
func postEdit(in hookInput, project string, stderr io.Writer) int {
	file := in.ToolInput.FilePath
	if file == "" || filepath.Ext(file) != ".go" {
		return 0
	}
	if _, err := os.Stat(file); err != nil {
		return 0
	}
	root, pkg, ok := packageOf(file, project)
	if !ok {
		return 0
	}
	// A package whose build tags exclude the host (web, android, ios, or
	// metal on Windows) fails lint with "build constraints exclude all Go
	// files": a gate error, not a code defect. make prepush cross-lints it.
	if !hasFiles(root, pkg) {
		return 0
	}

	// --fix applies what it can; what is left is a real finding.
	if out, ok := runCmd(root, nil, "golangci-lint", "run", "--fix", pkg); !ok {
		_, _ = fmt.Fprintf(stderr, "golangci-lint findings in %s after --fix:\n", pkg)
		_, _ = fmt.Fprintln(stderr, tail(out, 40))
		return 2
	}
	// Drop the success lines; keep failures only.
	if out, ok := runCmd(root, nil, "go", "test", pkg, "-short", "-count=1"); !ok {
		_, _ = fmt.Fprintf(stderr, "go test %s -short failed:\n", pkg)
		_, _ = fmt.Fprintln(stderr, tail(dropNoise(out), 40))
		return 2
	}
	return 0
}

// packageOf returns the module root holding file and the file's package as
// a ./-relative pattern from that root, with forward slashes. The root is
// the nearest directory with a go.mod, so a file in a nested module such as
// tools/lint, or in a git worktree outside the project, is checked in its
// own module. Relative file paths are resolved from project. ok is false
// when no go.mod is found or the relative package path cannot be resolved
// within the module root.
func packageOf(file, project string) (root, pkg string, ok bool) {
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(project, abs)
	}
	dir := filepath.Dir(abs)
	root, ok = moduleRoot(dir)
	if !ok {
		return "", "", false
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", false
	}
	return root, "./" + filepath.ToSlash(rel), true
}

// moduleRoot walks up from dir to the nearest directory holding go.mod.
// It returns "", false if none is found, ignoring file metadata errors.
func moduleRoot(dir string) (string, bool) {
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
