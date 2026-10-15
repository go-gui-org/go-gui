// themesurfacecheck is the breaking-change gate for the theme surface
// (issue #846).
//
// It replaces scripts/theme-surface-check.sh (issue #1026) with no change
// in behavior: every message, the ::error:: annotation, and the exit codes
// match the script, so the Makefile target and the CI gate keep working
// from `go run` on every OS, including Windows runners without bash.
//
// What it gates: gui/testdata/theme_surface.golden lists every exported
// field of Theme, ThemeCfg and TextStyle, one "Type.Field type" line each.
// A removed golden line is a field that was removed, renamed or retyped,
// and that breaks apps and custom themes outside this repo. Such a branch
// must add a "**BREAKING:" line under ## [Unreleased] in CHANGELOG.md.
//
// Added lines pass silently: adding a role is free (docs/theme-tokens.md).
//
// Usage, from anywhere in the repo:
//
//	go run ./tools/themesurfacecheck/
//
// THEME_SURFACE_BASE overrides the base ref, for replaying the gate over
// past commits. Not used by CI.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	golden = "gui/testdata/theme_surface.golden"
	file   = "CHANGELOG.md"
	marker = "**BREAKING:"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "theme-surface-check: %v\n", err)
		os.Exit(2)
	}
}

func run() error {
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	if err = os.Chdir(strings.TrimSpace(root)); err != nil {
		return err
	}

	// A branch is compared to where it forked from the default branch. On
	// the default branch itself there is no delta, so like the sibling
	// changelogentrycheck gate this is a no-op rather than an error.
	baseRef := resolveBaseRef()
	if baseRef == "" {
		fmt.Println("theme-surface-check: no base ref found, skipping")
		return nil
	}
	base, err := git("merge-base", baseRef, "HEAD")
	if err != nil {
		return err
	}
	base = strings.TrimSpace(base)

	// The golden did not exist at the base (the branch that adds this
	// gate): nothing can have been removed.
	if _, err := git("cat-file", "-e", base+":"+golden); err != nil {
		fmt.Printf("theme-surface-check: %s not at base, ok\n", golden)
		return nil
	}

	// Against the working tree, not HEAD, so the gate also fires on a
	// local run before the rename is committed. In CI the tree is clean
	// and the two are the same.
	diff, err := git("diff", base, "--", golden)
	if err != nil {
		return err
	}
	removed := removedLines(diff)
	if len(removed) == 0 {
		fmt.Println("theme-surface-check: no theme field removed, ok")
		return nil
	}

	// Presence alone is not enough: Unreleased can already hold an
	// earlier PR's BREAKING entry. So take the BREAKING lines this branch
	// added to the changelog and require at least one of them to sit
	// inside Unreleased.
	text, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}
	changelogDiff, err := git("diff", base, "--", file)
	if err != nil {
		return err
	}
	if breakingPresent(unreleasedBlock(string(text)), addedBreaking(changelogDiff)) {
		fmt.Println("theme-surface-check: theme field(s) removed, BREAKING entry present, ok")
		return nil
	}

	fmt.Fprintln(os.Stderr, "::error::theme-surface-check: theme field(s) removed "+
		"without a BREAKING entry")
	fmt.Fprintf(os.Stderr, `theme-surface-check: this branch removes, renames or retypes exported fields
of Theme, ThemeCfg or TextStyle:

%s

Apps and custom themes outside this repo use these names. Either:

1. Keep the old field for one release, mark it "// Deprecated:", and fill it
   next to the new one (docs/theme-tokens.md, "Text roles"). Or
2. Add "- **BREAKING: <what> (#N)**" under ### Changed in ## [Unreleased] of
   %s, with the migration spelled out.
`, indent(removed), file)
	os.Exit(1)
	return nil
}

// resolveBaseRef returns the ref to diff against, or "" when none exists.
func resolveBaseRef() string {
	if ref := os.Getenv("THEME_SURFACE_BASE"); ref != "" {
		return ref
	}
	var cands []string
	if b := os.Getenv("GITHUB_BASE_REF"); b != "" {
		cands = append(cands, "origin/"+b)
	}
	for _, cand := range append(cands, "origin/main", "main") {
		if _, err := git("rev-parse", "--verify", "--quiet", cand); err == nil {
			return cand
		}
	}
	return ""
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output() // #nosec G204 -- fixed binary, no shell; args are flags, refs and git-listed paths
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// removedLines returns the removed content lines of a unified diff: lines
// starting with "-" but not the "---" file header, with the sign stripped.
func removedLines(diff string) []string {
	var out []string
	for line := range strings.SplitSeq(diff, "\n") {
		if strings.HasPrefix(line, "-") && len(line) > 1 && line[1] != '-' {
			out = append(out, line[1:])
		}
	}
	return out
}

// unreleasedBlock returns the lines of text between "## [Unreleased]" and
// the next "## [" heading.
func unreleasedBlock(text string) string {
	var sb strings.Builder
	in := false
	for line := range strings.SplitSeq(text, "\n") {
		switch {
		case strings.HasPrefix(line, "## [Unreleased]"):
			in = true
		case strings.HasPrefix(line, "## ["):
			in = false
		default:
			if in {
				sb.WriteString(line + "\n")
			}
		}
	}
	return sb.String()
}

// addedBreaking returns the added diff lines (excluding the "+++" file
// header, with the sign stripped) that carry the BREAKING marker.
func addedBreaking(diff string) []string {
	var out []string
	for line := range strings.SplitSeq(diff, "\n") {
		if strings.HasPrefix(line, "+") && len(line) > 1 && line[1] != '+' {
			if body := line[1:]; strings.Contains(body, marker) {
				out = append(out, body)
			}
		}
	}
	return out
}

// breakingPresent reports whether any added BREAKING line sits inside the
// Unreleased block as an exact full line (grep -xF in the script).
func breakingPresent(unreleased string, added []string) bool {
	lines := make(map[string]bool)
	for line := range strings.SplitSeq(unreleased, "\n") {
		lines[line] = true
	}
	for _, a := range added {
		if lines[a] {
			return true
		}
	}
	return false
}

func indent(lines []string) string {
	var sb strings.Builder
	for i, l := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("  " + l)
	}
	return sb.String()
}
