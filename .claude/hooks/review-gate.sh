#!/usr/bin/env bash
# Claude Code Stop hook: a read-only correctness review of the changed Go code,
# run before the turn ends. It is the Review step of the /quality skill, moved to
# the end of every turn that changed Go code.
#
# Why a command hook and not type "prompt" or "agent": a prompt hook sees only
# the hook input JSON, never the diff. An agent hook has no git and starts a
# subagent on every Stop, including turns that changed nothing. This script
# returns at once unless the Go diff changed, and hands the reviewer the diff.
#
# The review runs in a headless `claude -p` with only Read, Grep and Glob, the
# project settings (so CLAUDE.md and gui/CLAUDE.md apply) and no MCP servers.
#
# Loop control: one review per user turn. The review runs only when
# stop_hook_active is false, that is, on the first Stop of a turn. The fixes the
# model makes after a block are then checked by stop-gate.sh, not reviewed again.
#
# Exit 2 blocks the stop and feeds the findings back. A reviewer that fails to
# run (network, auth, timeout) never blocks: infrastructure is not a finding.
set -u

# The child `claude -p` loads project settings, so it would run this hook again.
[ -z "${GOGUI_REVIEW_GATE:-}" ] || exit 0

# opus, not sonnet: on a probe with 3 planted defects sonnet found 1 with a wrong
# line number (3s); opus found all 3 with correct lines (11s).
model=${GOGUI_REVIEW_MODEL:-opus}
maxDiffLines=2500

input=$(cat)
[ "$(jq -r '.stop_hook_active // false' <<<"$input")" = "true" ] && exit 0

cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
gitdir=$(git rev-parse --absolute-git-dir 2>/dev/null) || exit 0

# Tracked changes as a diff, untracked .go files as new-file diffs.
diff=$(
	git diff HEAD -- '*.go'
	git ls-files --others --exclude-standard -- '*.go' | while read -r f; do
		git diff --no-index -- /dev/null "$f"
	done
)
[ -n "$diff" ] || exit 0

# Review each distinct diff once. A Stop fires on every turn, and a turn that
# only talks must not pay for a review of code already reviewed.
stamp="$gitdir/claude-review-gate.reviewed"
fingerprint=$(shasum <<<"$diff" | cut -d' ' -f1)
[ -f "$stamp" ] && [ "$(cat "$stamp")" = "$fingerprint" ] && exit 0

lines=$(wc -l <<<"$diff" | tr -d ' ')
if [ "$lines" -gt "$maxDiffLines" ]; then
	# Too big for a useful single pass. Record it so the note is not repeated.
	echo "$fingerprint" >"$stamp"
	echo "review-gate: Go diff is $lines lines (limit $maxDiffLines); skipped. Run /quality." >&2
	exit 0
fi

read -r -d '' prompt <<'EOF'
You are a correctness reviewer for the go-gui repo. Below is the uncommitted Go
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
- One line per finding: `path/file.go:LINE — [category] defect — fix`, where
  category is one of: correctness, error-path, concurrency, allocation,
  identity, events, theme, tests
- Last line exactly `VERDICT: PASS` (no findings) or `VERDICT: FINDINGS`.
EOF

out=$(
	GOGUI_REVIEW_GATE=1 claude -p \
		--model "$model" \
		--setting-sources project \
		--strict-mcp-config \
		--tools Read,Grep,Glob \
		--no-session-persistence \
		"$prompt"$'\n\n<diff>\n'"$diff"$'\n</diff>' 2>&1
) || {
	echo "review-gate: reviewer did not run; not blocking." >&2
	exit 0
}

verdict=$(grep -E '^VERDICT: (PASS|FINDINGS)$' <<<"$out" | tail -1)
case "$verdict" in
"VERDICT: PASS")
	echo "$fingerprint" >"$stamp"
	exit 0
	;;
"VERDICT: FINDINGS")
	# Stamp now: the fixes change the diff, and one review per diff is the budget.
	echo "$fingerprint" >"$stamp"
	# Log each finding for quality-findings-report.sh. These are reported, not
	# confirmed: a wrong finding the model rejects is still counted.
	logger="$HOME/.claude/scripts/log-finding.sh"
	if [ -x "$logger" ]; then
		re='^([^: ]+\.go):[0-9]+ — \[([a-z-]+)\] (.*)$'
		while IFS= read -r line; do
			[[ "$line" =~ $re ]] &&
				"$logger" review-gate "${BASH_REMATCH[2]}" "${BASH_REMATCH[1]}" "${BASH_REMATCH[3]%% — *}"
		done <<<"$out"
	fi
	{
		echo "Review gate ($model) found possible defects in the changed Go code."
		echo "Fix the real ones. If a finding is wrong, tell the user why; do not"
		echo "work around it silently."
		echo
		grep -v '^VERDICT:' <<<"$out"
	} >&2
	exit 2
	;;
*)
	echo "review-gate: reviewer output had no verdict; not blocking." >&2
	exit 0
	;;
esac
