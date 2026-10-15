// largefiles reports non-test Go source files in gui/ over 800 lines.
//
// It replaces scripts/large-files.sh and the inline find/awk gate of the
// large-files Makefile target (issue #1026). The shell version needed find,
// wc, awk and sort, so it did not run on Windows without extra tools. This
// tool walks the tree with filepath.WalkDir and counts newlines the way
// wc -l does, so the output matches the script on the same tree.
//
// One deliberate difference: the script piped the table through `sort -rn`,
// but every table row starts with "|", which has no leading number, so -n
// compared all rows as zero and -r fell back to reverse whole-line order —
// the report was reverse-alphabetical, not biggest-first. This tool sorts
// by line count descending, then path ascending, which is what the report
// is for.
//
// Usage, from the repo root:
//
//	go run ./tools/largefiles/
//
// Exit code is 0 when every file is within the threshold, 1 with a
// ::error:: annotation when any file exceeds it.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	dir       = "gui"
	threshold = 800
)

type entry struct {
	lines int
	path  string
}

func main() {
	entries, err := collect(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "large-files: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(report(entries))
	if len(entries) != 0 {
		fmt.Fprintf(os.Stderr, "::error::%d Go source files exceed %d lines\n",
			len(entries), threshold)
		os.Exit(1)
	}
}

// collect returns the non-test Go files under root over the threshold,
// sorted by line count descending, then path ascending. Paths use forward
// slashes so the report is identical on every OS.
func collect(root string) ([]entry, error) {
	var out []entry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return nil
		}
		n, err := countLines(path)
		if err != nil {
			return err
		}
		if n > threshold {
			out = append(out, entry{lines: n, path: filepath.ToSlash(path)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b entry) int {
		if a.lines != b.lines {
			return b.lines - a.lines
		}
		return strings.Compare(a.path, b.path)
	})
	return out, nil
}

// countLines counts '\n' bytes, the same definition wc -l uses. A file
// whose last line lacks a trailing newline counts one less than a
// line-scanner would report; Go files in this repo end in a newline, so
// the two agree here.
func countLines(path string) (int, error) {
	// #nosec G304 — path comes from walking the fixed gui dir in collect,
	// not from external input.
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n, nil
}

// report renders the Markdown table the script printed: header, one row
// per file, a blank line, and the threshold reminder.
func report(entries []entry) string {
	var sb strings.Builder
	sb.WriteString("| Lines | File |\n|------:|------|\n")
	for _, e := range entries {
		fmt.Fprintf(&sb, "| %d | %s |\n", e.lines, e.path)
	}
	fmt.Fprintf(&sb, "\nThreshold: >%d lines (non-test Go source in %s/)\n",
		threshold, dir)
	return sb.String()
}
