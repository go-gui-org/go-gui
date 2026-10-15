package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountLines(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"no trailing newline", "package a", 0},
		{"one line", "package a\n", 1},
		{"three lines", "a\nb\nc\n", 3},
		{"missing final newline", "a\nb\nc", 2},
		{"blank lines count", "a\n\n\n", 3},
		{"crlf", "a\r\nb\r\n", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "f.go")
			if err := os.WriteFile(p, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := countLines(p); err != nil || got != tc.want {
				t.Errorf("countLines() = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
}

func write(t *testing.T, root, name string, lines int) {
	t.Helper()
	var sb strings.Builder
	for range lines {
		sb.WriteString("package a\n")
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCollect(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ok.go", 10)
	write(t, root, "at.go", 800) // at the threshold: excluded
	write(t, root, "over.go", 801)
	write(t, root, "big.go", 900)
	write(t, root, "sub/big2.go", 900) // tie with big.go: path order decides
	write(t, root, "skip_test.go", 2000)
	write(t, root, "notes.txt", 2000)

	got, err := collect(root)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range got {
		paths = append(paths, e.path)
	}
	if len(got) != 3 {
		t.Fatalf("collect() found %d files %q, want 3", len(got), paths)
	}
	if got[0].lines != 900 || got[1].lines != 900 || got[2].lines != 801 {
		t.Errorf("collect() counts = %d, %d, %d; want 900, 900, 801",
			got[0].lines, got[1].lines, got[2].lines)
	}
	for _, p := range paths {
		if strings.Contains(p, "\\") {
			t.Errorf("collect() path %q is not slash-separated", p)
		}
	}
	// Tie broken by ascending path.
	if !strings.HasSuffix(got[0].path, "big.go") ||
		!strings.HasSuffix(got[1].path, "sub/big2.go") {
		t.Errorf("collect() tie order = %q, want big.go before sub/big2.go", paths)
	}
}

func TestCollectMissingDir(t *testing.T) {
	if _, err := collect(filepath.Join(t.TempDir(), "nonexistent")); err == nil {
		t.Error("collect() on a missing dir = nil error, want non-nil")
	}
}

func TestReport(t *testing.T) {
	got := report([]entry{{lines: 801, path: "gui/a.go"}, {lines: 1200, path: "gui/b.go"}})
	// collect() sorts biggest-first; report() keeps that order.
	want := "| Lines | File |\n|------:|------|\n" +
		"| 801 | gui/a.go |\n| 1200 | gui/b.go |\n" +
		"\nThreshold: >800 lines (non-test Go source in gui/)\n"
	if got != want {
		t.Errorf("report() = %q, want %q", got, want)
	}

	empty := report(nil)
	wantEmpty := "| Lines | File |\n|------:|------|\n" +
		"\nThreshold: >800 lines (non-test Go source in gui/)\n"
	if empty != wantEmpty {
		t.Errorf("report(nil) = %q, want %q", empty, wantEmpty)
	}
}
