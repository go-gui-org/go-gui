#!/usr/bin/env bash
# Claude Code Stop hook: the turn cannot end while changed Go code fails the gate.
#
# The PostToolUse hook (go-edit-check.sh) checks each Edit/Write. This gate catches
# what that misses: edits made through Bash (sed, gofmt -w, go generate) and the
# repo-wide rules that no per-file check sees (ergonomics-audit).
#
# Scope: packages holding a .go file that differs from HEAD, including untracked
# files. A clean tree exits at once.
#
# Exit 2 blocks the stop and feeds stderr back to the model. To keep a finding
# the model cannot fix from looping forever, the gate blocks at most maxBlocks
# times in a row per session, then lets the stop through with a note.
set -u

maxBlocks=3

# A headless reviewer started by review-gate.sh loads project settings too; it
# edits nothing, so gating its stop only burns time.
[ -z "${GOGUI_REVIEW_GATE:-}" ] || exit 0

input=$(cat)
session=$(jq -r '.session_id // "none"' <<<"$input")
active=$(jq -r '.stop_hook_active // false' <<<"$input")

cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
gitdir=$(git rev-parse --absolute-git-dir 2>/dev/null) || exit 0

# Changed .go files that still exist (a deleted file has no package to check).
files=$(
	{
		git diff --name-only HEAD -- '*.go'
		git ls-files --others --exclude-standard -- '*.go'
	} | sort -u | while read -r f; do [ -f "$f" ] && echo "$f"; done
)
[ -n "$files" ] || exit 0

# Skip the whole run when the Go changes are byte-identical to the last pass. A
# Stop fires on every turn, and most turns after a pass change nothing.
stamp="$gitdir/claude-stop-gate.pass"
fingerprint=$(
	{
		git diff HEAD -- '*.go'
		git ls-files --others --exclude-standard -- '*.go' | xargs cat 2>/dev/null
	} | shasum | cut -d' ' -f1
)
[ -f "$stamp" ] && [ "$(cat "$stamp")" = "$fingerprint" ] && exit 0

pkgs=$(echo "$files" | xargs -n1 dirname | sort -u | sed 's|^|./|')

# Findings go to the shared log (~/.claude/scripts/log-finding.sh) so rules broken
# again and again show up in quality-findings-report.sh. Only the first Stop of a
# turn logs: the re-runs after a block would count the same finding again.
logger="$HOME/.claude/scripts/log-finding.sh"
logFindings=false
[ "$active" != "true" ] && [ -x "$logger" ] && logFindings=true

# log_output <category> <output>: one record per `file.go:line` finding line, or
# one record for the whole check when it names no file (a failing test).
log_output() {
	local category=$1 out=$2 line file kind logged=false
	$logFindings || return 0
	while IFS= read -r line; do
		case "$line" in
		*.go:[0-9]*) ;;
		*) continue ;;
		esac
		file=${line%%:*}
		kind=$category
		# golangci-lint ends each finding with the linter name: "... (unused)".
		if [ "$category" = "golangci-lint" ] && [[ "$line" =~ \(([a-z0-9]+)\)$ ]]; then
			kind=${BASH_REMATCH[1]}
		fi
		"$logger" stop-gate "$kind" "$file" "${line#*: }"
		logged=true
	done <<<"$out"
	$logged || "$logger" stop-gate "$category" "" "$(grep -m1 -E -- '--- FAIL|FAIL|panic' <<<"$out")"
}

report=""
# run <category> <cmd...>: record the tail of a failing command's output.
run() {
	local label=$1 out
	shift
	if ! out=$("$@" 2>&1); then
		out=$(echo "$out" | grep -vE '^ok |no test files|^ld: warning: ignoring duplicate libraries')
		report+="== $label"$'\n'"$(echo "$out" | tail -30)"$'\n\n'
		log_output "$label" "$out"
	fi
}

# shellcheck disable=SC2086 # pkgs is a newline list; splitting is intended.
run govet go vet $pkgs
# shellcheck disable=SC2086
run golangci-lint golangci-lint run $pkgs
# shellcheck disable=SC2086
run test-fail go test -short -count=1 $pkgs

# The modes that fail on findings. focus and callbacks only report, so they are
# left out. The audits scan the whole repo; together they take about a second.
for mode in ids opt literals theme a11y visual deadcfg; do
	run "audit-$mode" go run ./tools/ergonomics-audit/ -mode "$mode" .
done

counter="$gitdir/claude-stop-gate.$session.blocks"
if [ -z "$report" ]; then
	echo "$fingerprint" >"$stamp"
	rm -f "$counter"
	exit 0
fi

blocks=0
[ "$active" = "true" ] && [ -f "$counter" ] && blocks=$(cat "$counter")
if [ "$blocks" -ge "$maxBlocks" ]; then
	rm -f "$counter"
	echo "stop-gate: still failing after $maxBlocks attempts; stop allowed." >&2
	echo "$report" >&2
	exit 0
fi
echo $((blocks + 1)) >"$counter"

{
	echo "Stop gate failed for changed packages: ${pkgs//$'\n'/ }"
	echo "Fix these before ending the turn. If a finding is pre-existing or out of"
	echo "scope, say so to the user instead of working around the check."
	echo
	echo "$report"
} >&2
exit 2
