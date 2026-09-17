#!/usr/bin/env bash
# Claude Code hook for Edit/Write on this repo.
#
# Claude Code passes the tool call as JSON on stdin. It does not set a
# file-path environment variable, so the path must come from
# .tool_input.file_path.
#
# Exit codes follow the hook contract: 0 = pass, 2 = feed stderr back to
# the model. Exit 1 is a non-blocking error the model never sees, so it
# must not be used to report a finding.
#
# Usage: go-edit-check.sh pre   (PreToolUse: block direct go.sum edits)
#        go-edit-check.sh post  (PostToolUse: lint-fix + short tests on the package)
set -u

file=$(jq -r '.tool_input.file_path // empty')
[ -n "$file" ] || exit 0

case "${1:-}" in
pre)
	case "$file" in
	*/go.sum | go.sum)
		echo "BLOCKED: do not edit go.sum directly. Run go mod tidy." >&2
		exit 2
		;;
	esac
	exit 0
	;;
post)
	case "$file" in
	*.go) ;;
	*) exit 0 ;;
	esac
	[ -f "$file" ] || exit 0
	cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0

	# Only the edited package, not dir/... — recursing from gui/ pulls in every
	# subpackage and makes each edit wait on the whole tree.
	pkg="./$(dirname "${file#"$PWD"/}")"

	# --fix applies what it can; what is left is a real finding.
	if ! lint_out=$(golangci-lint run --fix "$pkg" 2>&1); then
		echo "golangci-lint findings in $pkg after --fix:" >&2
		echo "$lint_out" | tail -40 >&2
		exit 2
	fi

	# Drop the success lines; keep failures only.
	if ! test_out=$(go test "$pkg" -short -count=1 2>&1); then
		echo "go test $pkg -short failed:" >&2
		echo "$test_out" | grep -vE '^ok |no test files|^ld: warning: ignoring duplicate libraries' | tail -40 >&2
		exit 2
	fi
	exit 0
	;;
esac
exit 0
