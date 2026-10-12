// changelogentrycheck is the presence gate for CHANGELOG.md's Unreleased
// block.
//
// Sibling of tools/changelogcheck, which gates the *shape* of released blocks
// (BREAKING placement) and deliberately skips Unreleased. This one gates
// *presence*: a branch that changes the exported surface or the release
// packaging must land its Unreleased entry in the same pass, not as cleanup
// before a tag. PR #538 shipped both-arch packaging with no entry; the text
// was reconstructed a day later in the release PR (#539) from the diff.
//
// Trigger is a heuristic by design (design C, chosen over an exported-surface
// diff that needs a base build, and over a path allowlist that fires on every
// internal refactor). It keys on the two things that empirically went
// undocumented here: exported declarations, and the packaging path set.
//
// Escape hatch: put "changelog: skip" in any commit message on the branch.
//
// Usage:
//
//	go run ./tools/changelogentrycheck/
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

const file = "CHANGELOG.md"

var (
	// Paths whose change reaches users without touching Go: release packaging
	// and the dependency install script CI and downstream builders run.
	packagingPath = regexp.MustCompile(
		`^(Makefile|\.github/workflows/release\.yml|scripts/install-.*\.sh)$`)

	diffBodyLine = regexp.MustCompile(`^[+-][^+-]`)
	diffComment  = regexp.MustCompile(`^[+-][[:space:]]*//`)
	exportedDecl = regexp.MustCompile(`^[+-][[:space:]]*(func [A-Z]|func \([^)]*\) [A-Z]|` +
		`type [A-Z]|const [A-Z]|var [A-Z]|[A-Z][A-Za-z0-9_]*[[:space:]]+[^=])`)

	skipLine = regexp.MustCompile(`(?i)^[[:space:]]*changelog:[[:space:]]*skip`)

	unreleasedHeading = regexp.MustCompile(`^## \[Unreleased\]`)
	versionHeading    = regexp.MustCompile(`^## \[`)
	bulletLine        = regexp.MustCompile(`^[[:space:]]*-[[:space:]]+[^[:space:]]`)
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "changelog-entry-check: %v\n", err)
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
	text, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "changelog-entry-check: %s not found\n", file)
		return nil
	}

	// A branch is compared to where it forked from the default branch. On the
	// default branch itself there is no delta, so the gate is a no-op rather
	// than an error — otherwise `make check` on a clean main would fail.
	baseRef := resolveBaseRef()
	if baseRef == "" {
		fmt.Println("changelog-entry-check: no base ref found, skipping")
		return nil
	}
	base, err := git("merge-base", baseRef, "HEAD")
	if err != nil {
		return err
	}
	base = strings.TrimSpace(base)
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if base == strings.TrimSpace(head) {
		fmt.Printf("changelog-entry-check: no commits since %s, skipping\n", baseRef)
		return nil
	}

	log, err := git("log", "--format=%B", base+"..HEAD")
	if err != nil {
		return err
	}
	if skipRequested(log) {
		fmt.Println("changelog-entry-check: 'changelog: skip' in a commit message, skipping")
		return nil
	}

	names, err := git("diff", "--name-only", base+"..HEAD")
	if err != nil {
		return err
	}
	names = strings.TrimRight(names, "\n")
	if names == "" {
		fmt.Println("changelog-entry-check: no changed files, skipping")
		return nil
	}
	changed := strings.Split(names, "\n")

	var reasons []string
	if pack := packagingChanged(changed); len(pack) != 0 {
		reasons = append(reasons, "release packaging changed:"+strings.Join(pack, " "))
	}
	// Scan added/removed lines in non-test Go files for an exported
	// declaration. Comment lines are excluded so a doc-comment rewrite
	// (PR #533) does not fire.
	if srcs := goSources(changed); len(srcs) != 0 {
		diff, derr := git(append([]string{"diff", base + "..HEAD", "--"}, srcs...)...)
		if derr != nil {
			return derr
		}
		if n := countExported(diff); n > 0 {
			reasons = append(reasons, fmt.Sprintf("%d exported-declaration line(s) changed", n))
		}
	}
	if len(reasons) == 0 {
		fmt.Println("changelog-entry-check: no user-facing trigger, ok")
		return nil
	}
	reason := strings.Join(reasons, "; ")

	// Two conditions, both necessary. Unreleased must be non-empty (at least
	// one list bullet; a bare "### Added" heading with nothing under it is not
	// an entry), AND this branch must have touched CHANGELOG.md. Presence alone
	// is not enough: Unreleased usually already holds an earlier PR's entry,
	// so a branch that adds nothing still finds the block non-empty. That is
	// exactly how #538 shipped undocumented while Unreleased was full of #536
	// and #537.
	touched := slices.Contains(changed, file)
	if touched && unreleasedHasEntry(string(text)) {
		fmt.Printf("changelog-entry-check: ok (%s)\n", reason)
		return nil
	}

	var msg string
	if !touched {
		msg = fmt.Sprintf("this branch is user-facing (%s) but does not touch %s", reason, file)
	} else {
		msg = fmt.Sprintf("%s has no entry under ## [Unreleased], "+
			"but this branch is user-facing (%s)", file, reason)
	}
	fmt.Fprintf(os.Stderr, "::error::changelog-entry-check: %s\n", msg)
	fmt.Fprintf(os.Stderr, `changelog-entry-check: %s

Add the entry in this branch, not in the release PR. Use the existing format:
a Keep a Changelog section (### Added / ### Changed / ### Fixed), a bolded
one-line summary, the issue or PR number, and prose saying what now happens
and why it matters. A breaking change goes under ### Changed as
"- **BREAKING: <what> (#N)**" with the migration spelled out.

If the change genuinely has no user-visible effect, put "changelog: skip" in
a commit message on this branch.
`, msg)
	os.Exit(1)
	return nil
}

// resolveBaseRef returns the ref to diff against, or "" when none exists.
// CHANGELOG_ENTRY_BASE overrides it, for replaying the gate over past commits
// when tuning the heuristic. Not used by CI.
func resolveBaseRef() string {
	if ref := os.Getenv("CHANGELOG_ENTRY_BASE"); ref != "" {
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

// packagingChanged returns the changed paths in the release packaging set.
func packagingChanged(changed []string) []string {
	var out []string
	for _, p := range changed {
		if packagingPath.MatchString(p) {
			out = append(out, p)
		}
	}
	return out
}

// goSources returns the changed non-test Go files.
func goSources(changed []string) []string {
	var out []string
	for _, p := range changed {
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			out = append(out, p)
		}
	}
	return out
}

// countExported counts the added or removed non-comment diff lines that
// declare an exported name.
func countExported(diff string) int {
	n := 0
	for line := range strings.SplitSeq(diff, "\n") {
		if diffBodyLine.MatchString(line) && !diffComment.MatchString(line) &&
			exportedDecl.MatchString(line) {
			n++
		}
	}
	return n
}

// skipRequested reports whether a commit message line opts out of the gate.
func skipRequested(log string) bool {
	for line := range strings.SplitSeq(log, "\n") {
		if skipLine.MatchString(line) {
			return true
		}
	}
	return false
}

// unreleasedHasEntry reports whether the Unreleased block holds a list bullet.
func unreleasedHasEntry(text string) bool {
	in := false
	for line := range strings.SplitSeq(text, "\n") {
		switch {
		case unreleasedHeading.MatchString(line):
			in = true
		case versionHeading.MatchString(line):
			in = false
		case in && bulletLine.MatchString(line):
			return true
		}
	}
	return false
}
