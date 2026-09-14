#!/usr/bin/env bash
# run-parallel.sh — run make targets at the same time, then print the
# output of each target as one block and a pass/fail summary.
#
# Used by `make prepush`. The system make on macOS is GNU Make 3.81, which
# has no --output-sync, so `make -j` would mix the output of all targets
# line by line. This script writes each target to its own log instead.
#
# Usage: scripts/run-parallel.sh <target>...
# Exit status is non-zero when one or more targets fail.
# Stays compatible with /bin/bash 3.2 (no `wait -n`, no associative arrays).

set -uo pipefail

make_cmd=${MAKE:-make}
logdir=$(mktemp -d)
trap 'rm -rf "$logdir"' EXIT
# Ctrl-C stops every child target too, not only this script.
trap 'kill 0 2>/dev/null; exit 130' INT TERM

pids=()
for target in "$@"; do
	(
		start=$SECONDS
		"$make_cmd" --no-print-directory "$target" >"$logdir/$target.log" 2>&1
		rc=$?
		echo "$rc $((SECONDS - start))" >"$logdir/$target.status"
		exit "$rc"
	) &
	pids+=("$!")
done

# Wait in the order given. A target that finishes early waits in its log
# until the targets before it have printed.
for pid in "${pids[@]}"; do
	wait "$pid"
done

failed=0
for target in "$@"; do
	echo "==> $target"
	cat "$logdir/$target.log"
done
echo "==> summary"
for target in "$@"; do
	read -r rc secs <"$logdir/$target.status"
	if [ "$rc" -eq 0 ]; then
		printf '  ok    %-18s %4ss\n' "$target" "$secs"
	else
		printf '  FAIL  %-18s %4ss (exit %s)\n' "$target" "$secs" "$rc"
		failed=1
	fi
done
exit "$failed"
