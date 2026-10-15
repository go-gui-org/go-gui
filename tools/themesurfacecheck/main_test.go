package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemovedLines(t *testing.T) {
	diff := "diff --git a/golden b/golden\n" +
		"--- a/golden\n" +
		"+++ b/golden\n" +
		"@@ -1,3 +1,2 @@\n" +
		" Theme.A int\n" +
		"-Theme.B string\n" +
		"-Theme.C float32\n" +
		"+Theme.D bool\n"
	got := removedLines(diff)
	if len(got) != 2 || got[0] != "Theme.B string" || got[1] != "Theme.C float32" {
		t.Errorf("removedLines() = %q, want the two removed content lines", got)
	}
}

func TestRemovedLinesEmpty(t *testing.T) {
	for _, diff := range []string{"", "+++ b/f\n--- a/f\n", "+added\n context\n"} {
		if got := removedLines(diff); len(got) != 0 {
			t.Errorf("removedLines(%q) = %q, want empty", diff, got)
		}
	}
}

func TestUnreleasedBlock(t *testing.T) {
	text := "# Changelog\n\n## [Unreleased]\n\n### Changed\n\n- **BREAKING: drop Foo (#1)**\n\n" +
		"## [v1.0.0] - 2026-01-01\n\n- old\n"
	block := unreleasedBlock(text)
	if !strings.Contains(block, "- **BREAKING: drop Foo (#1)**") {
		t.Errorf("unreleasedBlock() misses the BREAKING line:\n%s", block)
	}
	if strings.Contains(block, "old") || strings.Contains(block, "## [Unreleased]") {
		t.Errorf("unreleasedBlock() leaks outside the block:\n%s", block)
	}
}

func TestUnreleasedBlockMissing(t *testing.T) {
	if got := unreleasedBlock("## [v1.0.0]\n\n- old\n"); got != "" {
		t.Errorf("unreleasedBlock() without Unreleased = %q, want empty", got)
	}
}

func TestAddedBreaking(t *testing.T) {
	diff := "--- a/CHANGELOG.md\n" +
		"+++ b/CHANGELOG.md\n" +
		"@@ -1,2 +1,3 @@\n" +
		"+- **BREAKING: drop Foo (#1)**\n" +
		"+- plain line\n" +
		"-removed **BREAKING: stale**\n"
	got := addedBreaking(diff)
	if len(got) != 1 || got[0] != "- **BREAKING: drop Foo (#1)**" {
		t.Errorf("addedBreaking() = %q, want the one added BREAKING line", got)
	}
}

func TestBreakingPresent(t *testing.T) {
	unreleased := "### Changed\n\n- **BREAKING: drop Foo (#1)**\n"
	if !breakingPresent(unreleased, []string{"- **BREAKING: drop Foo (#1)**"}) {
		t.Error("breakingPresent() = false for an exact Unreleased line, want true")
	}
	// Presence elsewhere is not enough: an older PR's entry already in
	// Unreleased does not cover this branch.
	if breakingPresent(unreleased, []string{"- **BREAKING: drop Bar (#2)**"}) {
		t.Error("breakingPresent() = true for a line outside Unreleased, want false")
	}
	// Substring is not enough: grep -xF needs the full line.
	if breakingPresent(unreleased, []string{"**BREAKING: drop Foo (#1)**"}) {
		t.Error("breakingPresent() = true for a partial line, want false")
	}
	if breakingPresent(unreleased, nil) {
		t.Error("breakingPresent() = true with no added lines, want false")
	}
}

func TestResolveBaseRefOverride(t *testing.T) {
	t.Setenv("THEME_SURFACE_BASE", "v1.0.0")
	t.Setenv("GITHUB_BASE_REF", "should-not-be-used")
	if got := resolveBaseRef(); got != "v1.0.0" {
		t.Errorf("resolveBaseRef() = %q, want the THEME_SURFACE_BASE override", got)
	}
}

// TestGoldenHasNoDuplicates guards the input the gate reads: a duplicated
// golden line would hide a removal behind the surviving twin.
func TestGoldenHasNoDuplicates(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "gui", "testdata", "theme_surface.golden"))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
		if seen[line] {
			t.Errorf("golden has duplicate line %q", line)
		}
		seen[line] = true
	}
	if len(seen) == 0 {
		t.Error("golden is empty")
	}
}
