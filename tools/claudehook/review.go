package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// maxDiffLines is the largest Go diff the review gate hands the reviewer.
const maxDiffLines = 2500

// reviewPrompt is the reviewer's instructions. The diff follows on stdin.
const reviewPrompt = `You are a correctness reviewer for the go-gui repo. Below is the uncommitted Go
diff. Review ONLY the changed lines and the code they directly affect. Read
surrounding source with Read/Grep/Glob when a judgement needs it. CLAUDE.md and
gui/CLAUDE.md hold this repo's rules; apply them where they are specific.

Report any defect that could cause incorrect behavior, a panic, a failing test,
a per-frame heap allocation, or a misleading result. Omit style and naming
preferences. Check:
- Correctness: off-by-one, nil/zero-value handling, early returns that skip
  cleanup, wrong boundary conditions.
- Error paths: every returned error handled, or ignored on purpose with a reason.
- Concurrency: w.mu discipline. No window-mutating API (SetFocus, ClearFocus,
  SetView, Window.Lock) reachable from AmendLayout or other code under the frame
  lock; app callbacks raised from that pass go through deferCallback/QueueCommand.
- Allocation: no per-item or per-frame heap allocation in arrange, render or
  event paths (the pipeline after the view phase is 0-alloc).
- Identity: keying sites read shape.idKey() / w.EffID / ctx.EffID, never a bare
  cfg.ID; IDs composed with ScopeID/ScopeIDN.
- Events: a callback that acts on an event calls ctx.Consume().
- Theme: code outside generation with a *Window reads w.Theme().
- Tests: a bug fix without a regression test that would fail before the fix.

Do NOT report: formatting, naming, comment wording, or anything golangci-lint,
go vet or ergonomics-audit already checks. Do not suggest refactors.

Output format, nothing else:
- One line per finding: ` + "`path/file.go:LINE — [category] defect — fix`" + `, where
  category is one of: correctness, error-path, concurrency, allocation,
  identity, events, theme, tests
- Last line exactly ` + "`VERDICT: PASS`" + ` (no findings) or ` + "`VERDICT: FINDINGS`" + `.`

var (
	verdictRe = regexp.MustCompile(`^VERDICT: (PASS|FINDINGS)$`)
	// reviewFindingRe matches one reviewer finding line.
	reviewFindingRe = regexp.MustCompile(`^([^: ]+\.go):[0-9]+ — \[([a-z-]+)\] (.*)$`)
)

// reviewGate is the Stop hook that runs a read-only correctness review of
// the changed Go code before the turn ends. It is the Review step of the
// /quality skill, moved to the end of every turn that changed Go code.
//
// Why a command hook and not type "prompt" or "agent": a prompt hook sees
// only the hook input JSON, never the diff. An agent hook has no git and
// starts a subagent on every Stop, including turns that changed nothing.
// This returns at once unless the Go diff changed, and hands the reviewer
// the diff.
//
// The review runs in a headless `claude -p` with only Read, Grep and Glob,
// the project settings (so CLAUDE.md and gui/CLAUDE.md apply) and no MCP
// servers.
//
// Loop control: one review per user turn. The review runs only when
// stop_hook_active is false, that is, on the first Stop of a turn. The fixes
// the model makes after a block are then checked by the stop gate, not
// reviewed again.
//
// It returns 2 to block the stop and feed findings back, otherwise 0.
// Reviewer failures and output without a recognized verdict do not block.
// Diffs over maxDiffLines are stamped as reviewed and skipped; stamp errors
// are ignored.
func reviewGate(in hookInput, project string, stderr io.Writer) int {
	// The child `claude -p` loads project settings, so it would run this
	// hook again.
	if os.Getenv("GOGUI_REVIEW_GATE") != "" || in.StopHookActive {
		return 0
	}
	// opus, not sonnet: on a probe with 3 planted defects sonnet found 1
	// with a wrong line number (3s); opus found all 3 with correct lines
	// (11s).
	model := os.Getenv("GOGUI_REVIEW_MODEL")
	if model == "" {
		model = "opus"
	}

	gitdir := strings.TrimSpace(gitOutput(project, "rev-parse", "--absolute-git-dir"))
	if gitdir == "" {
		return 0
	}
	diff := reviewDiff(project)
	if diff == "" {
		return 0
	}

	// Review each distinct diff once. A Stop fires on every turn, and a
	// turn that only talks must not pay for a review of code already
	// reviewed.
	stamp := filepath.Join(gitdir, "claude-review-gate.reviewed")
	sum := sha256.Sum256([]byte(diff))
	fingerprint := hex.EncodeToString(sum[:])
	if stampMatches(stamp, fingerprint) {
		return 0
	}

	if n := len(lines(diff)); n > maxDiffLines {
		// Too big for a useful single pass. Record it so the note is
		// not repeated.
		writeStamp(stamp, fingerprint)
		_, _ = fmt.Fprintf(stderr, "review-gate: Go diff is %d lines (limit %d); skipped. Run /quality.\n", n, maxDiffLines)
		return 0
	}

	// The diff goes on stdin, not on the command line: Windows caps a
	// command line at 32767 characters, well under a 2500-line diff.
	// #nosec G204 -- fixed binary, no shell; the model name comes from the developer's GOGUI_REVIEW_MODEL
	cmd := exec.Command("claude", "-p",
		"--model", model,
		"--setting-sources", "project",
		"--strict-mcp-config",
		"--tools", "Read,Grep,Glob",
		"--no-session-persistence",
		reviewPrompt)
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "GOGUI_REVIEW_GATE=1")
	cmd.Stdin = strings.NewReader("<diff>\n" + diff + "\n</diff>\n")
	b, err := cmd.CombinedOutput()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "review-gate: reviewer did not run; not blocking.")
		return 0
	}
	out := string(b)

	verdict := ""
	for _, l := range lines(out) {
		if verdictRe.MatchString(l) {
			verdict = l
		}
	}
	switch verdict {
	case "VERDICT: PASS":
		writeStamp(stamp, fingerprint)
		return 0
	case "VERDICT: FINDINGS":
		// Stamp now: the fixes change the diff, and one review per diff
		// is the budget.
		writeStamp(stamp, fingerprint)
		// Log each finding for quality-findings-report.sh. These are
		// reported, not confirmed: a wrong finding the model rejects is
		// still counted.
		if logger := newFindingLogger(); logger != nil {
			for _, l := range lines(out) {
				if m := reviewFindingRe.FindStringSubmatch(l); m != nil {
					msg, _, _ := strings.Cut(m[3], " — ")
					logger.log("review-gate", m[2], m[1], msg)
				}
			}
		}
		_, _ = fmt.Fprintf(stderr, "Review gate (%s) found possible defects in the changed Go code.\n", model)
		_, _ = fmt.Fprintln(stderr, "Fix the real ones. If a finding is wrong, tell the user why; do not")
		_, _ = fmt.Fprintln(stderr, "work around it silently.")
		_, _ = fmt.Fprintln(stderr)
		for _, l := range lines(out) {
			if !strings.HasPrefix(l, "VERDICT:") {
				_, _ = fmt.Fprintln(stderr, l)
			}
		}
		return 2
	default:
		_, _ = fmt.Fprintln(stderr, "review-gate: reviewer output had no verdict; not blocking.")
		return 0
	}
}

// reviewDiff returns the tracked Go changes as a diff, plus each untracked
// .go file as a new-file diff.
func reviewDiff(project string) string {
	var b strings.Builder
	b.WriteString(gitOutput(project, "diff", "HEAD", "--", "*.go"))
	for _, f := range lines(gitOutput(project, "ls-files", "--others", "--exclude-standard", "--", "*.go")) {
		// --no-index exits 1 when the files differ, which they always do
		// here; gitOutput keeps stdout either way.
		b.WriteString(gitOutput(project, "diff", "--no-index", "--", "/dev/null", f))
	}
	return strings.TrimRight(b.String(), "\n")
}
